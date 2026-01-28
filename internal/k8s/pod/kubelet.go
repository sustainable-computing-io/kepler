// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package pod

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
	corev1 "k8s.io/api/core/v1"
)

const (
	defaultKubeletPort      = 10250
	defaultPollInterval     = 15 * time.Second
	defaultRequestTimeout   = 10 * time.Second
	serviceAccountTokenPath = "/var/run/secrets/kubernetes.io/serviceaccount/token"
)

// getDefaultKubeletHost returns the kubelet host from NODE_IP env var,
// falling back to "localhost" if not set. The NODE_IP should be set via
// downward API (status.hostIP) in the pod spec.
func getDefaultKubeletHost() string {
	if nodeIP := os.Getenv("NODE_IP"); nodeIP != "" {
		return nodeIP
	}
	return "localhost"
}

type kubeletPodInformer struct {
	logger       *slog.Logger
	nodeName     string
	kubeletHost  string
	kubeletPort  int
	pollInterval time.Duration
	tokenPath    string

	httpClient   *http.Client
	refreshGroup singleflight.Group

	mu    sync.RWMutex
	cache map[string]*ContainerInfo // containerID -> ContainerInfo
}

// NewKubeletInformer creates a new kubelet-based pod informer that polls
// the local kubelet /pods endpoint instead of watching the API server.
func NewKubeletInformer(opts ...OptFn) *kubeletPodInformer {
	opt := DefaultOpts()
	for _, fn := range opts {
		fn(&opt)
	}

	host := opt.kubeletHost
	if host == "" {
		host = getDefaultKubeletHost()
	}
	port := opt.kubeletPort
	if port == 0 {
		port = defaultKubeletPort
	}
	interval := opt.pollInterval
	if interval == 0 {
		interval = defaultPollInterval
	}

	return &kubeletPodInformer{
		logger:       opt.logger.With("service", "kubeletPodInformer"),
		nodeName:     opt.nodeName,
		kubeletHost:  host,
		kubeletPort:  port,
		pollInterval: interval,
		tokenPath:    serviceAccountTokenPath,
		cache:        make(map[string]*ContainerInfo),
	}
}

func (i *kubeletPodInformer) Name() string {
	return "kubeletPodInformer"
}

func (i *kubeletPodInformer) Init() error {
	if i.nodeName == "" {
		return fmt.Errorf("nodeName not set")
	}

	// Verify token file is readable
	if _, err := i.readToken(); err != nil {
		return fmt.Errorf("failed to read SA token: %w", err)
	}

	// Setup HTTP client with TLS (kubelet uses self-signed cert)
	i.httpClient = &http.Client{
		Timeout: defaultRequestTimeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, // kubelet uses self-signed cert
			},
		},
	}

	// Do initial fetch
	if err := i.refresh(context.Background()); err != nil {
		return fmt.Errorf("initial kubelet fetch failed: %w", err)
	}

	i.logger.Info("kubelet pod informer initialized",
		"nodeName", i.nodeName,
		"kubeletHost", i.kubeletHost,
		"kubeletPort", i.kubeletPort,
		"pollInterval", i.pollInterval)

	return nil
}

// readToken reads the service account token from the token file.
// This is called on each refresh to handle projected token rotation.
func (i *kubeletPodInformer) readToken() (string, error) {
	tokenBytes, err := os.ReadFile(i.tokenPath)
	if err != nil {
		return "", err
	}
	return string(tokenBytes), nil
}

func (i *kubeletPodInformer) Run(ctx context.Context) error {
	i.logger.Info("starting kubelet pod informer")
	ticker := time.NewTicker(i.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			i.logger.Info("kubelet pod informer stopped")
			return nil
		case <-ticker.C:
			if err := i.refresh(ctx); err != nil {
				i.logger.Warn("failed to refresh pods from kubelet", "error", err)
			}
		}
	}
}

func (i *kubeletPodInformer) refresh(ctx context.Context) error {
	_, err, _ := i.refreshGroup.Do("refresh", func() (interface{}, error) {
		return nil, i.doRefresh(ctx)
	})
	return err
}

func (i *kubeletPodInformer) doRefresh(ctx context.Context) error {
	token, err := i.readToken()
	if err != nil {
		return fmt.Errorf("failed to read SA token: %w", err)
	}

	url := fmt.Sprintf("https://%s:%d/pods", i.kubeletHost, i.kubeletPort)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := i.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("kubelet returned status %d", resp.StatusCode)
	}

	var podList corev1.PodList
	if err := json.NewDecoder(resp.Body).Decode(&podList); err != nil {
		return fmt.Errorf("failed to decode pod list: %w", err)
	}

	// Build container ID -> pod info cache
	newCache := make(map[string]*ContainerInfo)
	for _, pod := range podList.Items {
		i.addContainersToCache(newCache, &pod, pod.Status.ContainerStatuses)
		i.addContainersToCache(newCache, &pod, pod.Status.InitContainerStatuses)
		i.addContainersToCache(newCache, &pod, pod.Status.EphemeralContainerStatuses)
	}

	i.mu.Lock()
	i.cache = newCache
	i.mu.Unlock()

	i.logger.Debug("refreshed pod cache from kubelet",
		"podCount", len(podList.Items),
		"containerCount", len(newCache))
	return nil
}

func (i *kubeletPodInformer) addContainersToCache(cache map[string]*ContainerInfo, pod *corev1.Pod, statuses []corev1.ContainerStatus) {
	for _, status := range statuses {
		if status.ContainerID == "" {
			continue
		}
		containerID := extractContainerID(status.ContainerID)
		cache[containerID] = &ContainerInfo{
			PodID:         string(pod.UID),
			PodName:       pod.Name,
			Namespace:     pod.Namespace,
			ContainerName: status.Name,
		}
	}
}

// LookupByContainerID retrieves pod details and container name given a containerID.
// If the containerID is not found in cache, it triggers an immediate refresh.
func (i *kubeletPodInformer) LookupByContainerID(containerID string) (*ContainerInfo, bool, error) {
	i.mu.RLock()
	info, found := i.cache[containerID]
	i.mu.RUnlock()

	if found {
		return info, true, nil
	}

	// Trigger immediate refresh for unknown container (coalesced via singleflight)
	if err := i.refresh(context.Background()); err != nil {
		i.logger.Warn("on-demand refresh failed", "error", err)
	}

	i.mu.RLock()
	info, found = i.cache[containerID]
	i.mu.RUnlock()

	if !found {
		return nil, false, nil
	}
	return info, true, nil
}
