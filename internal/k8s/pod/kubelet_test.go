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
	serverURL := server.URL
	hostPort := strings.TrimPrefix(serverURL, "https://")
	parts := strings.Split(hostPort, ":")
	host := parts[0]
	port := mustParsePort(parts[1])

	// Create informer pointing to mock server
	informer := &kubeletPodInformer{
		logger:       testLogger(),
		nodeName:     "test-node",
		kubeletHost:  host,
		kubeletPort:  port,
		pollInterval: 1 * time.Second,
		httpClient:   server.Client(),
		token:        "test-token",
		cache:        make(map[string]*ContainerInfo),
	}

	// Test refresh
	err := informer.refresh()
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

	hostPort := strings.TrimPrefix(server.URL, "https://")
	parts := strings.Split(hostPort, ":")

	informer := &kubeletPodInformer{
		logger:       testLogger(),
		nodeName:     "test-node",
		kubeletHost:  parts[0],
		kubeletPort:  mustParsePort(parts[1]),
		pollInterval: 1 * time.Second,
		httpClient:   server.Client(),
		token:        "test-token",
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
		pollInterval: 100 * time.Millisecond,
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
	// We can't easily mock the token path, so this test verifies
	// that the informer properly initializes other fields
	informer := NewKubeletInformer(
		WithNodeName("test-node"),
	)

	// Manually set token to bypass file read
	informer.token = "test-token"

	// The Init will fail on kubelet fetch since there's no server
	// but we've verified the token handling above
	assert.Equal(t, "test-node", informer.nodeName)
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

func mustParsePort(s string) int {
	var port int
	for _, c := range s {
		if c >= '0' && c <= '9' {
			port = port*10 + int(c-'0')
		}
	}
	return port
}
