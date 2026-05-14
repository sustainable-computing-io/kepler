// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package pod

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// PodMeta holds the pod identity resolved from an IP.
type PodMeta struct {
	Name      string
	Namespace string
}

// IPResolver resolves pod IPs to pod names/namespaces by periodically
// listing pods from the Kubernetes API server. Safe for concurrent use.
type IPResolver struct {
	client   kubernetes.Interface
	logger   *slog.Logger
	interval time.Duration

	mu    sync.RWMutex
	ipMap map[string]PodMeta

	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewIPResolver creates an IPResolver that lists pods every interval.
// If kubeconfig is empty, in-cluster config is used.
func NewIPResolver(kubeconfig string, interval time.Duration, logger *slog.Logger) (*IPResolver, error) {
	cfg, err := buildConfig(kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("k8s config: %w", err)
	}

	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("k8s client: %w", err)
	}

	if interval <= 0 {
		interval = 30 * time.Second
	}

	return &IPResolver{
		client:   client,
		logger:   logger.With("component", "pod-ip-resolver"),
		interval: interval,
		ipMap:    make(map[string]PodMeta),
	}, nil
}

func buildConfig(kubeconfig string) (*rest.Config, error) {
	if kubeconfig != "" {
		return clientcmd.BuildConfigFromFlags("", kubeconfig)
	}
	return rest.InClusterConfig()
}

// NodePodCIDRs returns the pod CIDR(s) assigned to the given node. Most
// clusters use a single PodCIDR per node; multi-family clusters (IPv4+IPv6)
// set PodCIDRs. Both are returned as strings.
func NodePodCIDRs(kubeconfig, nodeName string) ([]string, error) {
	cfg, err := buildConfig(kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("k8s config: %w", err)
	}
	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("k8s client: %w", err)
	}

	node, err := client.CoreV1().Nodes().Get(context.Background(), nodeName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get node %q: %w", nodeName, err)
	}

	// Prefer PodCIDRs (plural, supports dual-stack) over PodCIDR (single).
	if len(node.Spec.PodCIDRs) > 0 {
		return node.Spec.PodCIDRs, nil
	}
	if node.Spec.PodCIDR != "" {
		return []string{node.Spec.PodCIDR}, nil
	}
	return nil, fmt.Errorf("node %q has no PodCIDR assigned", nodeName)
}

// Start begins the background refresh loop. Stops when ctx is cancelled.
func (r *IPResolver) Start(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	r.cancel = cancel

	if err := r.refresh(ctx); err != nil {
		r.logger.Warn("initial pod IP refresh failed", "error", err)
	}

	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		t := time.NewTicker(r.interval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				if err := r.refresh(ctx); err != nil {
					r.logger.Warn("pod IP refresh failed", "error", err)
				}
			}
		}
	}()
}

// Stop cancels the background loop and waits for it to exit.
func (r *IPResolver) Stop() {
	if r.cancel != nil {
		r.cancel()
	}
	r.wg.Wait()
}

// Resolve returns the pod metadata for the given IP, if known.
func (r *IPResolver) Resolve(ip string) (PodMeta, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	m, ok := r.ipMap[ip]
	return m, ok
}

func (r *IPResolver) refresh(ctx context.Context) error {
	list, err := r.client.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	newMap := make(map[string]PodMeta, len(list.Items))
	for i := range list.Items {
		p := &list.Items[i]
		if p.Status.PodIP == "" {
			continue
		}
		// Skip host-network pods — their IP is the node IP, not useful for attribution.
		if p.Spec.HostNetwork {
			continue
		}
		meta := PodMeta{Name: p.Name, Namespace: p.Namespace}
		newMap[p.Status.PodIP] = meta
		// Also index additional pod IPs (dual-stack)
		for _, ip := range p.Status.PodIPs {
			if ip.IP != "" && ip.IP != p.Status.PodIP {
				newMap[ip.IP] = meta
			}
		}
	}

	r.mu.Lock()
	r.ipMap = newMap
	r.mu.Unlock()

	r.logger.Debug("pod IP resolver refreshed", "pods", len(newMap))
	return nil
}

var _ = corev1.Pod{} // keep import
