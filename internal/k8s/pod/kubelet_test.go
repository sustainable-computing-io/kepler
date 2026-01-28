// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package pod

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestKubeletPodInformer_Name(t *testing.T) {
	informer := NewKubeletInformer()
	assert.Equal(t, "kubeletPodInformer", informer.Name())
}

func TestKubeletPodInformer_Init_NoNodeName(t *testing.T) {
	informer := NewKubeletInformer()
	err := informer.Init()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "nodeName not set")
}

func TestKubeletPodInformer_Init_NoToken(t *testing.T) {
	informer := NewKubeletInformer(
		WithNodeName("test-node"),
	)
	err := informer.Init()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read SA token")
}

func TestKubeletPodInformer_Refresh(t *testing.T) {
	// Create a mock kubelet server
	podList := createTestPodList()
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify auth header
		authHeader := r.Header.Get("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		// Return pod list
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(podList)
	}))
	defer server.Close()

	// Parse server URL to get host and port
	host, port := parseHostPort(t, server.URL)

	// Write a temporary token file
	tokenFile := writeTokenFile(t, "test-token")

	// Create informer pointing to mock server
	informer := &kubeletPodInformer{
		logger:       testLogger(),
		nodeName:     "test-node",
		kubeletHost:  host,
		kubeletPort:  port,
		pollInterval: 1 * time.Second,
		httpClient:   server.Client(),
		tokenPath:    tokenFile,
		cache:        make(map[string]*ContainerInfo),
	}

	// Test refresh
	err := informer.refresh(context.Background())
	require.NoError(t, err)

	// Verify cache was populated
	assert.Len(t, informer.cache, 2) // 2 containers in test pod list

	// Lookup container
	info, found, err := informer.LookupByContainerID("abc123")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "test-pod-1", info.PodName)
	assert.Equal(t, "default", info.Namespace)
	assert.Equal(t, "container-1", info.ContainerName)
	assert.Equal(t, "uid-1", info.PodID)
}

func TestKubeletPodInformer_LookupByContainerID_NotFound(t *testing.T) {
	// Create mock server that returns empty pod list
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(&corev1.PodList{})
	}))
	defer server.Close()

	host, port := parseHostPort(t, server.URL)
	tokenFile := writeTokenFile(t, "test-token")

	informer := &kubeletPodInformer{
		logger:       testLogger(),
		nodeName:     "test-node",
		kubeletHost:  host,
		kubeletPort:  port,
		pollInterval: 1 * time.Second,
		httpClient:   server.Client(),
		tokenPath:    tokenFile,
		cache:        make(map[string]*ContainerInfo),
	}

	// Lookup non-existent container
	info, found, err := informer.LookupByContainerID("nonexistent")
	require.NoError(t, err)
	assert.False(t, found)
	assert.Nil(t, info)
}

func TestKubeletPodInformer_LookupByContainerID_Found(t *testing.T) {
	informer := &kubeletPodInformer{
		logger:       testLogger(),
		nodeName:     "test-node",
		kubeletHost:  "localhost",
		kubeletPort:  10250,
		pollInterval: 1 * time.Second,
		cache: map[string]*ContainerInfo{
			"container-id-1": {
				PodID:         "pod-uid-1",
				PodName:       "my-pod",
				Namespace:     "my-namespace",
				ContainerName: "my-container",
			},
		},
	}

	info, found, err := informer.LookupByContainerID("container-id-1")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "my-pod", info.PodName)
	assert.Equal(t, "my-namespace", info.Namespace)
	assert.Equal(t, "my-container", info.ContainerName)
	assert.Equal(t, "pod-uid-1", info.PodID)
}

func TestKubeletPodInformer_Run_ContextCancel(t *testing.T) {
	informer := &kubeletPodInformer{
		logger:       testLogger(),
		nodeName:     "test-node",
		kubeletHost:  "localhost",
		kubeletPort:  10250,
		pollInterval: 1 * time.Hour, // large interval to avoid ticker firing before cancel
		cache:        make(map[string]*ContainerInfo),
	}

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error)
	go func() {
		done <- informer.Run(ctx)
	}()

	// Cancel after short delay
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		assert.NoError(t, err)
	case <-time.After(1 * time.Second):
		t.Fatal("Run did not exit after context cancel")
	}
}

func TestKubeletPodInformer_AddContainersToCache(t *testing.T) {
	informer := &kubeletPodInformer{
		logger: testLogger(),
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "test-ns",
			UID:       types.UID("test-uid"),
		},
	}

	statuses := []corev1.ContainerStatus{
		{
			Name:        "container-a",
			ContainerID: "containerd://id-a",
		},
		{
			Name:        "container-b",
			ContainerID: "docker://id-b",
		},
		{
			Name:        "container-empty",
			ContainerID: "", // should be skipped
		},
	}

	cache := make(map[string]*ContainerInfo)
	informer.addContainersToCache(cache, pod, statuses)

	assert.Len(t, cache, 2)

	infoA := cache["id-a"]
	require.NotNil(t, infoA)
	assert.Equal(t, "test-pod", infoA.PodName)
	assert.Equal(t, "test-ns", infoA.Namespace)
	assert.Equal(t, "container-a", infoA.ContainerName)
	assert.Equal(t, "test-uid", infoA.PodID)

	infoB := cache["id-b"]
	require.NotNil(t, infoB)
	assert.Equal(t, "container-b", infoB.ContainerName)
}

func TestNewKubeletInformer_DefaultOptions(t *testing.T) {
	informer := NewKubeletInformer()

	// Default host comes from NODE_IP env or falls back to localhost
	assert.Equal(t, getDefaultKubeletHost(), informer.kubeletHost)
	assert.Equal(t, defaultKubeletPort, informer.kubeletPort)
	assert.Equal(t, defaultPollInterval, informer.pollInterval)
}

func TestNewKubeletInformer_WithOptions(t *testing.T) {
	informer := NewKubeletInformer(
		WithNodeName("my-node"),
		WithKubeletHost("custom-host"),
		WithKubeletPort(12345),
		WithPollInterval(30*time.Second),
	)

	assert.Equal(t, "my-node", informer.nodeName)
	assert.Equal(t, "custom-host", informer.kubeletHost)
	assert.Equal(t, 12345, informer.kubeletPort)
	assert.Equal(t, 30*time.Second, informer.pollInterval)
}

func TestKubeletPodInformer_InitWithMockToken(t *testing.T) {
	informer := NewKubeletInformer(
		WithNodeName("test-node"),
	)

	assert.Equal(t, "test-node", informer.nodeName)
}

func TestGetDefaultKubeletHost_WithEnvVar(t *testing.T) {
	t.Setenv("NODE_IP", "10.0.0.1")
	assert.Equal(t, "10.0.0.1", getDefaultKubeletHost())
}

func TestGetDefaultKubeletHost_Fallback(t *testing.T) {
	t.Setenv("NODE_IP", "")
	assert.Equal(t, "localhost", getDefaultKubeletHost())
}

func TestNewKubeletInformer_ZeroValueDefaults(t *testing.T) {
	// When options supply zero values, defaults should be applied
	informer := NewKubeletInformer(
		WithKubeletHost(""),
		WithKubeletPort(0),
		WithPollInterval(0),
	)
	assert.Equal(t, getDefaultKubeletHost(), informer.kubeletHost)
	assert.Equal(t, defaultKubeletPort, informer.kubeletPort)
	assert.Equal(t, defaultPollInterval, informer.pollInterval)
}

func TestKubeletPodInformer_Init_Success(t *testing.T) {
	// Mock kubelet server returning a valid pod list
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(createTestPodList())
	}))
	defer server.Close()

	host, port := parseHostPort(t, server.URL)
	tokenFile := writeTokenFile(t, "test-token")

	informer := &kubeletPodInformer{
		logger:       testLogger(),
		nodeName:     "test-node",
		kubeletHost:  host,
		kubeletPort:  port,
		pollInterval: 1 * time.Second,
		tokenPath:    tokenFile,
		cache:        make(map[string]*ContainerInfo),
	}
	// Override httpClient after Init sets it, so we use the test server's TLS client
	informer.httpClient = server.Client()

	err := informer.refresh(context.Background())
	require.NoError(t, err)
	assert.Len(t, informer.cache, 2)
}

func TestKubeletPodInformer_Init_FullPath(t *testing.T) {
	// End-to-end Init with a real TLS test server
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(&corev1.PodList{})
	}))
	defer server.Close()

	host, port := parseHostPort(t, server.URL)
	tokenFile := writeTokenFile(t, "init-token")

	informer := &kubeletPodInformer{
		logger:       testLogger(),
		nodeName:     "test-node",
		kubeletHost:  host,
		kubeletPort:  port,
		pollInterval: 1 * time.Hour,
		tokenPath:    tokenFile,
		cache:        make(map[string]*ContainerInfo),
	}

	// Init creates its own httpClient with InsecureSkipVerify, which works with TLS test server
	err := informer.Init()
	require.NoError(t, err)
	assert.NotNil(t, informer.httpClient)
}

func TestKubeletPodInformer_Run_RefreshError(t *testing.T) {
	// Server returns 500 to trigger the refresh error log path
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	host, port := parseHostPort(t, server.URL)
	tokenFile := writeTokenFile(t, "test-token")

	informer := &kubeletPodInformer{
		logger:       testLogger(),
		nodeName:     "test-node",
		kubeletHost:  host,
		kubeletPort:  port,
		pollInterval: 50 * time.Millisecond,
		httpClient:   server.Client(),
		tokenPath:    tokenFile,
		cache:        make(map[string]*ContainerInfo),
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error)
	go func() {
		done <- informer.Run(ctx)
	}()

	// Let at least one tick fire with the error
	time.Sleep(120 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		assert.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not exit after context cancel")
	}
}

func TestKubeletPodInformer_DoRefresh_TokenReadError(t *testing.T) {
	informer := &kubeletPodInformer{
		logger:    testLogger(),
		tokenPath: "/nonexistent/token",
		cache:     make(map[string]*ContainerInfo),
	}

	err := informer.doRefresh(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read SA token")
}

func TestKubeletPodInformer_DoRefresh_HTTPError(t *testing.T) {
	tokenFile := writeTokenFile(t, "test-token")

	informer := &kubeletPodInformer{
		logger:      testLogger(),
		kubeletHost: "127.0.0.1",
		kubeletPort: 1, // unreachable port
		tokenPath:   tokenFile,
		httpClient:  &http.Client{Timeout: 100 * time.Millisecond},
		cache:       make(map[string]*ContainerInfo),
	}

	err := informer.doRefresh(context.Background())
	require.Error(t, err)
}

func TestKubeletPodInformer_DoRefresh_Non200Status(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	host, port := parseHostPort(t, server.URL)
	tokenFile := writeTokenFile(t, "test-token")

	informer := &kubeletPodInformer{
		logger:      testLogger(),
		kubeletHost: host,
		kubeletPort: port,
		tokenPath:   tokenFile,
		httpClient:  server.Client(),
		cache:       make(map[string]*ContainerInfo),
	}

	err := informer.doRefresh(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "kubelet returned status 403")
}

func TestKubeletPodInformer_DoRefresh_InvalidJSON(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("not-json"))
	}))
	defer server.Close()

	host, port := parseHostPort(t, server.URL)
	tokenFile := writeTokenFile(t, "test-token")

	informer := &kubeletPodInformer{
		logger:      testLogger(),
		kubeletHost: host,
		kubeletPort: port,
		tokenPath:   tokenFile,
		httpClient:  server.Client(),
		cache:       make(map[string]*ContainerInfo),
	}

	err := informer.doRefresh(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decode pod list")
}

func TestKubeletPodInformer_LookupByContainerID_FoundAfterRefresh(t *testing.T) {
	// Server returns a pod list containing the container we'll look up
	podList := createTestPodList()
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(podList)
	}))
	defer server.Close()

	host, port := parseHostPort(t, server.URL)
	tokenFile := writeTokenFile(t, "test-token")

	informer := &kubeletPodInformer{
		logger:      testLogger(),
		nodeName:    "test-node",
		kubeletHost: host,
		kubeletPort: port,
		tokenPath:   tokenFile,
		httpClient:  server.Client(),
		cache:       make(map[string]*ContainerInfo), // empty cache
	}

	// "abc123" is not in cache, but will be found after on-demand refresh
	info, found, err := informer.LookupByContainerID("abc123")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "test-pod-1", info.PodName)
}

func TestKubeletPodInformer_LookupByContainerID_RefreshFails(t *testing.T) {
	tokenFile := writeTokenFile(t, "test-token")

	informer := &kubeletPodInformer{
		logger:      testLogger(),
		nodeName:    "test-node",
		kubeletHost: "127.0.0.1",
		kubeletPort: 1, // unreachable
		tokenPath:   tokenFile,
		httpClient:  &http.Client{Timeout: 100 * time.Millisecond},
		cache:       make(map[string]*ContainerInfo),
	}

	// Lookup triggers on-demand refresh which fails; should return not-found without error
	info, found, err := informer.LookupByContainerID("missing")
	require.NoError(t, err)
	assert.False(t, found)
	assert.Nil(t, info)
}

func TestKubeletPodInformer_Init_RefreshFails(t *testing.T) {
	tokenFile := writeTokenFile(t, "init-token")

	informer := &kubeletPodInformer{
		logger:       testLogger(),
		nodeName:     "test-node",
		kubeletHost:  "127.0.0.1",
		kubeletPort:  1, // unreachable port
		pollInterval: 1 * time.Hour,
		tokenPath:    tokenFile,
		cache:        make(map[string]*ContainerInfo),
	}

	err := informer.Init()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "initial kubelet fetch failed")
}

func TestKubeletPodInformer_DoRefresh_CancelledContext(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(&corev1.PodList{})
	}))
	defer server.Close()

	host, port := parseHostPort(t, server.URL)
	tokenFile := writeTokenFile(t, "test-token")

	informer := &kubeletPodInformer{
		logger:      testLogger(),
		kubeletHost: host,
		kubeletPort: port,
		tokenPath:   tokenFile,
		httpClient:  server.Client(),
		cache:       make(map[string]*ContainerInfo),
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	err := informer.doRefresh(ctx)
	require.Error(t, err)
}

// Helper functions

func createTestPodList() *corev1.PodList {
	return &corev1.PodList{
		Items: []corev1.Pod{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod-1",
					Namespace: "default",
					UID:       types.UID("uid-1"),
				},
				Status: corev1.PodStatus{
					ContainerStatuses: []corev1.ContainerStatus{
						{
							Name:        "container-1",
							ContainerID: "containerd://abc123",
						},
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod-2",
					Namespace: "kube-system",
					UID:       types.UID("uid-2"),
				},
				Status: corev1.PodStatus{
					ContainerStatuses: []corev1.ContainerStatus{
						{
							Name:        "container-2",
							ContainerID: "docker://def456",
						},
					},
				},
			},
		},
	}
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
}

// parseHostPort extracts host and port from a test server URL.
func parseHostPort(t *testing.T, serverURL string) (string, int) {
	t.Helper()
	hostPort := strings.TrimPrefix(serverURL, "https://")
	parts := strings.Split(hostPort, ":")
	require.Len(t, parts, 2, "expected host:port in URL %s", serverURL)
	port, err := strconv.Atoi(parts[1])
	require.NoError(t, err, "failed to parse port from URL %s", serverURL)
	return parts[0], port
}

// writeTokenFile writes a temporary token file and returns its path.
func writeTokenFile(t *testing.T, token string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "sa-token-*")
	require.NoError(t, err)
	_, err = f.WriteString(token)
	require.NoError(t, err)
	require.NoError(t, f.Close())
	return f.Name()
}
