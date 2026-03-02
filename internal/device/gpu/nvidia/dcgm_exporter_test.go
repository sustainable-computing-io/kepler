// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package nvidia

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/iotest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

const sampleDCGMMetrics = `# HELP DCGM_FI_PROF_GR_ENGINE_ACTIVE GPU compute engine activity (0.0-1.0)
# TYPE DCGM_FI_PROF_GR_ENGINE_ACTIVE gauge
DCGM_FI_PROF_GR_ENGINE_ACTIVE{gpu="0",UUID="GPU-abc-123",GPU_I_ID="1",GPU_I_PROFILE="3g.20gb",device="nvidia0"} 0.75
DCGM_FI_PROF_GR_ENGINE_ACTIVE{gpu="0",UUID="GPU-abc-123",GPU_I_ID="2",GPU_I_PROFILE="3g.20gb",device="nvidia0"} 0.25
DCGM_FI_PROF_GR_ENGINE_ACTIVE{gpu="1",UUID="GPU-def-456",GPU_I_ID="1",GPU_I_PROFILE="1g.5gb",device="nvidia1"} 0.0
# HELP DCGM_FI_DEV_POWER_USAGE Power usage in watts
# TYPE DCGM_FI_DEV_POWER_USAGE gauge
DCGM_FI_DEV_POWER_USAGE{gpu="0",UUID="GPU-abc-123",GPU_I_ID="1",GPU_I_PROFILE="3g.20gb",device="nvidia0"} 120.5
DCGM_FI_DEV_POWER_USAGE{gpu="0",UUID="GPU-abc-123",GPU_I_ID="2",GPU_I_PROFILE="3g.20gb",device="nvidia0"} 85.3
`

func TestParseMetrics(t *testing.T) {
	backend := NewDCGMExporterBackend(slog.Default())

	metrics, err := backend.parseMetrics(strings.NewReader(sampleDCGMMetrics))
	require.NoError(t, err)

	// Should have 3 MIG instances across 2 GPUs
	assert.Len(t, metrics.instances, 3)

	// Check GPU 0, instance 1
	key01 := migKey{gpuIndex: 0, gpuInstanceID: 1}
	assert.Contains(t, metrics.instances, key01)
	inst01 := metrics.instances[key01]
	assert.Equal(t, 0.75, inst01.activity)
	assert.Equal(t, "3g.20gb", inst01.profile)

	// Check GPU 0, instance 2
	key02 := migKey{gpuIndex: 0, gpuInstanceID: 2}
	assert.Contains(t, metrics.instances, key02)
	inst02 := metrics.instances[key02]
	assert.Equal(t, 0.25, inst02.activity)

	// Check GPU 1, instance 1
	key11 := migKey{gpuIndex: 1, gpuInstanceID: 1}
	assert.Contains(t, metrics.instances, key11)
	inst11 := metrics.instances[key11]
	assert.Equal(t, 0.0, inst11.activity)
	assert.Equal(t, "1g.5gb", inst11.profile)
}

func TestParseMetrics_empty(t *testing.T) {
	backend := NewDCGMExporterBackend(slog.Default())

	metrics, err := backend.parseMetrics(strings.NewReader(""))
	require.NoError(t, err)
	assert.Empty(t, metrics.instances)
}

func TestParseMetrics_nonMIG(t *testing.T) {
	// Metrics without GPU_I_ID should be skipped
	input := `DCGM_FI_DEV_GPU_UTIL{gpu="0",UUID="GPU-abc-123",device="nvidia0"} 50.0`
	backend := NewDCGMExporterBackend(slog.Default())

	metrics, err := backend.parseMetrics(strings.NewReader(input))
	require.NoError(t, err)
	assert.Empty(t, metrics.instances)
}

func TestParseLabels(t *testing.T) {
	labels := parseLabels(`gpu="0",UUID="GPU-abc-123",GPU_I_ID="1",GPU_I_PROFILE="3g.20gb"`)

	assert.Equal(t, "0", labels["gpu"])
	assert.Equal(t, "GPU-abc-123", labels["UUID"])
	assert.Equal(t, "1", labels["GPU_I_ID"])
	assert.Equal(t, "3g.20gb", labels["GPU_I_PROFILE"])
}

func TestDCGMExporterBackend_GetMIGInstanceActivity(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, sampleDCGMMetrics)
	}))
	defer server.Close()

	ctx := context.Background()
	backend := NewDCGMExporterBackend(slog.Default())
	backend.SetEndpoint(server.URL)

	err := backend.Init(ctx)
	require.NoError(t, err)

	// Should return activity for GPU 0, instance 1
	activity, err := backend.GetMIGInstanceActivity(ctx, 0, 1)
	require.NoError(t, err)
	assert.Equal(t, 0.75, activity)

	// Should return activity for GPU 0, instance 2
	activity, err = backend.GetMIGInstanceActivity(ctx, 0, 2)
	require.NoError(t, err)
	assert.Equal(t, 0.25, activity)

	// Should return error for nonexistent instance
	_, err = backend.GetMIGInstanceActivity(ctx, 0, 99)
	assert.Error(t, err)
}

func TestDCGMExporterBackend_caching(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount++
		_, _ = fmt.Fprint(w, sampleDCGMMetrics)
	}))
	defer server.Close()

	ctx := context.Background()
	backend := NewDCGMExporterBackend(slog.Default())
	backend.SetEndpoint(server.URL)

	err := backend.Init(ctx)
	require.NoError(t, err)

	// Init makes a test request
	initCalls := callCount

	// First query fetches fresh
	_, err = backend.GetMIGInstanceActivity(ctx, 0, 1)
	require.NoError(t, err)
	assert.Equal(t, initCalls+1, callCount)

	// Second query within cache TTL should not make another HTTP request
	_, err = backend.GetMIGInstanceActivity(ctx, 0, 2)
	require.NoError(t, err)
	assert.Equal(t, initCalls+1, callCount, "should use cached metrics")
}

func TestDCGMExporterBackend_notInitialized(t *testing.T) {
	backend := NewDCGMExporterBackend(slog.Default())

	_, err := backend.GetMIGInstanceActivity(context.Background(), 0, 1)
	assert.Error(t, err)
}

func TestDCGMExporterBackend_SetEndpoint(t *testing.T) {
	backend := NewDCGMExporterBackend(slog.Default())

	// Should append /metrics
	backend.SetEndpoint("http://localhost:9400")
	assert.Equal(t, "http://localhost:9400/metrics", backend.endpoint)

	// Should not double-append
	backend.SetEndpoint("http://localhost:9400/metrics")
	assert.Equal(t, "http://localhost:9400/metrics", backend.endpoint)

	// Should handle trailing slash without doubling /metrics
	backend.SetEndpoint("http://localhost:9400/metrics/")
	assert.Equal(t, "http://localhost:9400/metrics", backend.endpoint)

	// Should handle trailing slash on base URL
	backend.SetEndpoint("http://localhost:9400/")
	assert.Equal(t, "http://localhost:9400/metrics", backend.endpoint)

	// Should handle multiple trailing slashes
	backend.SetEndpoint("http://localhost:9400///")
	assert.Equal(t, "http://localhost:9400/metrics", backend.endpoint)

	// Should handle whitespace
	backend.SetEndpoint("  http://localhost:9400  ")
	assert.Equal(t, "http://localhost:9400/metrics", backend.endpoint)
}

func TestDCGMExporterBackend_cacheExpiry(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount++
		_, _ = fmt.Fprint(w, sampleDCGMMetrics)
	}))
	defer server.Close()

	ctx := context.Background()
	backend := NewDCGMExporterBackend(slog.Default())
	backend.SetEndpoint(server.URL)

	err := backend.Init(ctx)
	require.NoError(t, err)
	initCalls := callCount

	// First fetch
	_, _ = backend.GetMIGInstanceActivity(ctx, 0, 1)
	assert.Equal(t, initCalls+1, callCount)

	// Expire the cache by backdating timestamp
	backend.mu.Lock()
	backend.cachedMetrics.timestamp = time.Now().Add(-3 * time.Second)
	backend.mu.Unlock()

	// Next fetch should refetch
	_, _ = backend.GetMIGInstanceActivity(ctx, 0, 1)
	assert.Equal(t, initCalls+2, callCount, "should refetch after cache expiry")
}

func TestNewDCGMExporterBackend(t *testing.T) {
	t.Run("with nil logger", func(t *testing.T) {
		backend := NewDCGMExporterBackend(nil)
		assert.NotNil(t, backend)
		assert.NotNil(t, backend.logger)
		assert.NotNil(t, backend.client)
	})

	t.Run("with logger", func(t *testing.T) {
		logger := slog.Default()
		backend := NewDCGMExporterBackend(logger)
		assert.NotNil(t, backend)
	})
}

func TestDCGMExporterBackend_Shutdown(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, sampleDCGMMetrics)
	}))
	defer server.Close()

	ctx := context.Background()
	backend := NewDCGMExporterBackend(slog.Default())
	backend.SetEndpoint(server.URL)

	err := backend.Init(ctx)
	require.NoError(t, err)
	assert.True(t, backend.IsInitialized())

	err = backend.Shutdown()
	assert.NoError(t, err)
	assert.False(t, backend.IsInitialized())

	// After shutdown, GetMIGInstanceActivity should fail
	_, err = backend.GetMIGInstanceActivity(ctx, 0, 1)
	assert.Error(t, err)
}

func TestDCGMExporterBackend_IsInitialized(t *testing.T) {
	backend := NewDCGMExporterBackend(slog.Default())
	assert.False(t, backend.IsInitialized())
}

func TestDCGMExporterBackend_Init_errors(t *testing.T) {
	ctx := context.Background()

	t.Run("idempotent init", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = fmt.Fprint(w, sampleDCGMMetrics)
		}))
		defer server.Close()

		backend := NewDCGMExporterBackend(slog.Default())
		backend.SetEndpoint(server.URL)

		err := backend.Init(ctx)
		require.NoError(t, err)

		// Second init should be a no-op
		err = backend.Init(ctx)
		assert.NoError(t, err)
		assert.True(t, backend.IsInitialized())
	})

	t.Run("configured endpoint not reachable", func(t *testing.T) {
		backend := NewDCGMExporterBackend(slog.Default())
		backend.SetEndpoint("http://127.0.0.1:1/metrics")

		err := backend.Init(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not reachable")
		assert.False(t, backend.IsInitialized())
	})

	t.Run("all endpoints fail", func(t *testing.T) {
		backend := NewDCGMExporterBackend(slog.Default())
		backend.discoverEndpoint = func() string { return "" }
		backend.fallbackEndpoints = []string{"http://127.0.0.1:1/metrics"}

		err := backend.Init(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no reachable dcgm-exporter endpoint found")
	})

	t.Run("discovery succeeds", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = fmt.Fprint(w, sampleDCGMMetrics)
		}))
		defer server.Close()

		backend := NewDCGMExporterBackend(slog.Default())
		backend.discoverEndpoint = func() string { return server.URL + "/metrics" }

		err := backend.Init(ctx)
		assert.NoError(t, err)
		assert.True(t, backend.IsInitialized())
		assert.Equal(t, server.URL+"/metrics", backend.endpoint)
	})

	t.Run("discovery returns unreachable endpoint falls to fallback", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = fmt.Fprint(w, sampleDCGMMetrics)
		}))
		defer server.Close()

		backend := NewDCGMExporterBackend(slog.Default())
		backend.discoverEndpoint = func() string { return "http://127.0.0.1:1/metrics" }
		backend.fallbackEndpoints = []string{server.URL + "/metrics"}

		err := backend.Init(ctx)
		assert.NoError(t, err)
		assert.True(t, backend.IsInitialized())
		assert.Equal(t, server.URL+"/metrics", backend.endpoint)
	})

	t.Run("fallback endpoint succeeds", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = fmt.Fprint(w, sampleDCGMMetrics)
		}))
		defer server.Close()

		backend := NewDCGMExporterBackend(slog.Default())
		backend.discoverEndpoint = func() string { return "" }
		backend.fallbackEndpoints = []string{server.URL + "/metrics"}

		err := backend.Init(ctx)
		assert.NoError(t, err)
		assert.True(t, backend.IsInitialized())
	})

	t.Run("configured endpoint returns 500", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		backend := NewDCGMExporterBackend(slog.Default())
		backend.SetEndpoint(server.URL)

		err := backend.Init(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not reachable")
	})
}

func TestDCGMExporterBackend_fetchMetrics_errors(t *testing.T) {
	ctx := context.Background()

	t.Run("server returns 500", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		backend := NewDCGMExporterBackend(slog.Default())
		backend.endpoint = server.URL + "/metrics"
		backend.initialized = true

		_, err := backend.GetMIGInstanceActivity(ctx, 0, 1)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unexpected status: 500")
	})

	t.Run("server unreachable", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = fmt.Fprint(w, sampleDCGMMetrics)
		}))
		endpoint := server.URL + "/metrics"
		server.Close() // Close immediately

		backend := NewDCGMExporterBackend(slog.Default())
		backend.endpoint = endpoint
		backend.initialized = true

		_, err := backend.GetMIGInstanceActivity(ctx, 0, 1)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to fetch metrics")
	})
}

func TestParseMetrics_malformed(t *testing.T) {
	t.Run("malformed float value skipped by regex", func(t *testing.T) {
		// Regex requires numeric value — non-numeric values don't match
		input := `DCGM_FI_PROF_GR_ENGINE_ACTIVE{gpu="0",UUID="GPU-abc",GPU_I_ID="1",GPU_I_PROFILE="3g.20gb"} not_a_number`
		backend := NewDCGMExporterBackend(slog.Default())

		metrics, err := backend.parseMetrics(strings.NewReader(input))
		require.NoError(t, err)
		assert.Empty(t, metrics.instances)
	})

	t.Run("value with scientific notation", func(t *testing.T) {
		input := `DCGM_FI_PROF_GR_ENGINE_ACTIVE{gpu="0",UUID="GPU-abc",GPU_I_ID="1",GPU_I_PROFILE="3g.20gb"} 1.5e-2`
		backend := NewDCGMExporterBackend(slog.Default())

		metrics, err := backend.parseMetrics(strings.NewReader(input))
		require.NoError(t, err)
		key := migKey{gpuIndex: 0, gpuInstanceID: 1}
		assert.Contains(t, metrics.instances, key)
		assert.InDelta(t, 0.015, metrics.instances[key].activity, 0.001)
	})

	t.Run("scanner error", func(t *testing.T) {
		backend := NewDCGMExporterBackend(slog.Default())

		_, err := backend.parseMetrics(iotest.ErrReader(fmt.Errorf("read error")))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "error reading metrics")
	})
}

func TestDCGMExporterBackend_GetMIGInstanceActivity_fetchError(t *testing.T) {
	ctx := context.Background()

	// Backend is initialized but server is gone
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, sampleDCGMMetrics)
	}))

	backend := NewDCGMExporterBackend(slog.Default())
	backend.SetEndpoint(server.URL)

	err := backend.Init(ctx)
	require.NoError(t, err)

	// Fetch once to populate cache
	_, err = backend.GetMIGInstanceActivity(ctx, 0, 1)
	require.NoError(t, err)

	// Expire cache and close server
	backend.mu.Lock()
	backend.cachedMetrics.timestamp = time.Now().Add(-3 * time.Second)
	backend.mu.Unlock()
	server.Close()

	_, err = backend.GetMIGInstanceActivity(ctx, 0, 1)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to fetch metrics")
}

func TestParseLabels_edgeCases(t *testing.T) {
	t.Run("empty string", func(t *testing.T) {
		labels := parseLabels("")
		assert.Empty(t, labels)
	})

	t.Run("single label", func(t *testing.T) {
		labels := parseLabels(`gpu="0"`)
		assert.Len(t, labels, 1)
		assert.Equal(t, "0", labels["gpu"])
	})
}

// dcgmExporterPod creates a pod that looks like a dcgm-exporter pod.
func dcgmExporterPod(name, namespace, nodeName, podIP string, labels map[string]string, ports ...corev1.ContainerPort) *corev1.Pod {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: corev1.PodSpec{
			NodeName: nodeName,
		},
		Status: corev1.PodStatus{
			PodIP: podIP,
		},
	}
	if len(ports) > 0 {
		pod.Spec.Containers = []corev1.Container{{
			Name:  "dcgm-exporter",
			Ports: ports,
		}}
	}
	return pod
}

func TestDiscoverLocalDCGMExporter(t *testing.T) {
	const nodeName = "gpu-node-1"

	t.Run("finds GPU Operator pod on same node", func(t *testing.T) {
		client := fake.NewSimpleClientset(
			dcgmExporterPod("dcgm-exporter-abc", "nvidia-gpu-operator", nodeName, "10.0.0.1",
				map[string]string{"app": "nvidia-dcgm-exporter"}),
		)

		backend := NewDCGMExporterBackend(slog.Default())
		backend.kubeClient = client
		t.Setenv("NODE_NAME", nodeName)

		endpoint := backend.discoverLocalDCGMExporter()
		assert.Equal(t, "http://10.0.0.1:9400/metrics", endpoint)
	})

	t.Run("finds standalone Helm chart pod via second label", func(t *testing.T) {
		client := fake.NewSimpleClientset(
			dcgmExporterPod("dcgm-exporter-xyz", "monitoring", nodeName, "10.0.0.2",
				map[string]string{"app.kubernetes.io/name": "dcgm-exporter"}),
		)

		backend := NewDCGMExporterBackend(slog.Default())
		backend.kubeClient = client
		t.Setenv("NODE_NAME", nodeName)

		endpoint := backend.discoverLocalDCGMExporter()
		assert.Equal(t, "http://10.0.0.2:9400/metrics", endpoint)
	})

	t.Run("finds pod in custom namespace", func(t *testing.T) {
		client := fake.NewSimpleClientset(
			dcgmExporterPod("dcgm-custom", "my-gpu-stack", nodeName, "10.0.0.3",
				map[string]string{"app": "nvidia-dcgm-exporter"}),
		)

		backend := NewDCGMExporterBackend(slog.Default())
		backend.kubeClient = client
		t.Setenv("NODE_NAME", nodeName)

		endpoint := backend.discoverLocalDCGMExporter()
		assert.Equal(t, "http://10.0.0.3:9400/metrics", endpoint)
	})

	t.Run("ignores pod on different node", func(t *testing.T) {
		// Note: fake clientset doesn't support fieldSelector filtering,
		// so we test this by having no pods with the correct labels on any node.
		// The real K8s API filters by spec.nodeName via fieldSelector.
		client := fake.NewSimpleClientset(
			dcgmExporterPod("dcgm-other", "nvidia-gpu-operator", "other-node", "10.0.0.4",
				map[string]string{"app": "unrelated-app"}),
		)

		backend := NewDCGMExporterBackend(slog.Default())
		backend.kubeClient = client
		t.Setenv("NODE_NAME", nodeName)

		endpoint := backend.discoverLocalDCGMExporter()
		assert.Empty(t, endpoint)
	})

	t.Run("skips pod with no IP", func(t *testing.T) {
		client := fake.NewSimpleClientset(
			dcgmExporterPod("dcgm-no-ip", "nvidia-gpu-operator", nodeName, "",
				map[string]string{"app": "nvidia-dcgm-exporter"}),
		)

		backend := NewDCGMExporterBackend(slog.Default())
		backend.kubeClient = client
		t.Setenv("NODE_NAME", nodeName)

		endpoint := backend.discoverLocalDCGMExporter()
		assert.Empty(t, endpoint)
	})

	t.Run("no matching pods", func(t *testing.T) {
		client := fake.NewSimpleClientset(
			dcgmExporterPod("unrelated-pod", "default", nodeName, "10.0.0.5",
				map[string]string{"app": "something-else"}),
		)

		backend := NewDCGMExporterBackend(slog.Default())
		backend.kubeClient = client
		t.Setenv("NODE_NAME", nodeName)

		endpoint := backend.discoverLocalDCGMExporter()
		assert.Empty(t, endpoint)
	})

	t.Run("NODE_NAME not set", func(t *testing.T) {
		backend := NewDCGMExporterBackend(slog.Default())
		t.Setenv("NODE_NAME", "")

		endpoint := backend.discoverLocalDCGMExporter()
		assert.Empty(t, endpoint)
	})

	t.Run("prefers first label match", func(t *testing.T) {
		// Both GPU Operator and standalone pods exist — should find GPU Operator first
		client := fake.NewSimpleClientset(
			dcgmExporterPod("dcgm-operator", "nvidia-gpu-operator", nodeName, "10.0.0.10",
				map[string]string{"app": "nvidia-dcgm-exporter"}),
			dcgmExporterPod("dcgm-standalone", "monitoring", nodeName, "10.0.0.11",
				map[string]string{"app.kubernetes.io/name": "dcgm-exporter"}),
		)

		backend := NewDCGMExporterBackend(slog.Default())
		backend.kubeClient = client
		t.Setenv("NODE_NAME", nodeName)

		endpoint := backend.discoverLocalDCGMExporter()
		assert.Equal(t, "http://10.0.0.10:9400/metrics", endpoint)
	})

	t.Run("uses named metrics port", func(t *testing.T) {
		client := fake.NewSimpleClientset(
			dcgmExporterPod("dcgm-custom-port", "nvidia-gpu-operator", nodeName, "10.0.0.20",
				map[string]string{"app": "nvidia-dcgm-exporter"},
				corev1.ContainerPort{Name: "metrics", ContainerPort: 9500}),
		)

		backend := NewDCGMExporterBackend(slog.Default())
		backend.kubeClient = client
		t.Setenv("NODE_NAME", nodeName)

		endpoint := backend.discoverLocalDCGMExporter()
		assert.Equal(t, "http://10.0.0.20:9500/metrics", endpoint)
	})

	t.Run("uses single container port", func(t *testing.T) {
		client := fake.NewSimpleClientset(
			dcgmExporterPod("dcgm-single-port", "nvidia-gpu-operator", nodeName, "10.0.0.21",
				map[string]string{"app": "nvidia-dcgm-exporter"},
				corev1.ContainerPort{Name: "http", ContainerPort: 8080}),
		)

		backend := NewDCGMExporterBackend(slog.Default())
		backend.kubeClient = client
		t.Setenv("NODE_NAME", nodeName)

		endpoint := backend.discoverLocalDCGMExporter()
		assert.Equal(t, "http://10.0.0.21:8080/metrics", endpoint)
	})

	t.Run("falls back to 9400 with no ports", func(t *testing.T) {
		client := fake.NewSimpleClientset(
			dcgmExporterPod("dcgm-no-ports", "nvidia-gpu-operator", nodeName, "10.0.0.22",
				map[string]string{"app": "nvidia-dcgm-exporter"}),
		)

		backend := NewDCGMExporterBackend(slog.Default())
		backend.kubeClient = client
		t.Setenv("NODE_NAME", nodeName)

		endpoint := backend.discoverLocalDCGMExporter()
		assert.Equal(t, "http://10.0.0.22:9400/metrics", endpoint)
	})
}

func TestDCGMExporterPort(t *testing.T) {
	tests := []struct {
		name string
		pod  *corev1.Pod
		want int32
	}{
		{
			name: "named metrics port",
			pod:  dcgmExporterPod("p", "ns", "n", "1.2.3.4", nil, corev1.ContainerPort{Name: "metrics", ContainerPort: 9500}),
			want: 9500,
		},
		{
			name: "default port 9400",
			pod:  dcgmExporterPod("p", "ns", "n", "1.2.3.4", nil, corev1.ContainerPort{Name: "http", ContainerPort: 9400}),
			want: 9400,
		},
		{
			name: "single non-standard port",
			pod:  dcgmExporterPod("p", "ns", "n", "1.2.3.4", nil, corev1.ContainerPort{Name: "custom", ContainerPort: 8080}),
			want: 8080,
		},
		{
			name: "metrics port preferred over others",
			pod: dcgmExporterPod("p", "ns", "n", "1.2.3.4", nil,
				corev1.ContainerPort{Name: "health", ContainerPort: 8080},
				corev1.ContainerPort{Name: "metrics", ContainerPort: 9500},
			),
			want: 9500,
		},
		{
			name: "no containers - fallback",
			pod:  dcgmExporterPod("p", "ns", "n", "1.2.3.4", nil),
			want: 9400,
		},
		{
			name: "multiple ports no match - fallback",
			pod: dcgmExporterPod("p", "ns", "n", "1.2.3.4", nil,
				corev1.ContainerPort{Name: "health", ContainerPort: 8080},
				corev1.ContainerPort{Name: "grpc", ContainerPort: 50051},
			),
			want: 9400,
		},
		{
			name: "metrics port in second container wins over single port in first",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "sidecar", Ports: []corev1.ContainerPort{{Name: "health", ContainerPort: 8080}}},
						{Name: "dcgm-exporter", Ports: []corev1.ContainerPort{{Name: "metrics", ContainerPort: 9500}}},
					},
				},
			},
			want: 9500,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, dcgmExporterPort(tc.pod))
		})
	}
}

func TestDCGMExporterBackend_Init_discoveryPriority(t *testing.T) {
	// Test the 3-level discovery priority:
	// 1. K8s API discovery (local pod on same node)
	// 2. localhost:9400 (hostNetwork)
	// 3. ClusterIP service (last resort)

	t.Run("priority 1: K8s discovery succeeds", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = fmt.Fprint(w, sampleDCGMMetrics)
		}))
		defer server.Close()

		backend := NewDCGMExporterBackend(slog.Default())
		backend.discoverEndpoint = func() string { return server.URL + "/metrics" }
		// Set fallbacks to unreachable — should never be tried
		backend.fallbackEndpoints = []string{"http://127.0.0.1:1/metrics"}

		err := backend.Init(context.Background())
		require.NoError(t, err)
		assert.Equal(t, server.URL+"/metrics", backend.endpoint)
	})

	t.Run("priority 2: discovery fails, localhost fallback succeeds", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = fmt.Fprint(w, sampleDCGMMetrics)
		}))
		defer server.Close()

		backend := NewDCGMExporterBackend(slog.Default())
		backend.discoverEndpoint = func() string { return "" } // K8s discovery fails
		backend.fallbackEndpoints = []string{
			server.URL + "/metrics", // simulates localhost:9400
			"http://127.0.0.1:1/metrics",
		}

		err := backend.Init(context.Background())
		require.NoError(t, err)
		assert.Equal(t, server.URL+"/metrics", backend.endpoint)
	})

	t.Run("priority 3: discovery and localhost fail, ClusterIP succeeds", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = fmt.Fprint(w, sampleDCGMMetrics)
		}))
		defer server.Close()

		backend := NewDCGMExporterBackend(slog.Default())
		backend.discoverEndpoint = func() string { return "" }
		backend.fallbackEndpoints = []string{
			"http://127.0.0.1:1/metrics", // localhost fails
			server.URL + "/metrics",      // ClusterIP succeeds
		}

		err := backend.Init(context.Background())
		require.NoError(t, err)
		assert.Equal(t, server.URL+"/metrics", backend.endpoint)
	})

	t.Run("all 3 priorities fail", func(t *testing.T) {
		backend := NewDCGMExporterBackend(slog.Default())
		backend.discoverEndpoint = func() string { return "" }
		backend.fallbackEndpoints = []string{
			"http://127.0.0.1:1/metrics",
			"http://127.0.0.1:2/metrics",
		}

		err := backend.Init(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no reachable dcgm-exporter endpoint found")
	})

	t.Run("discovery returns unreachable, falls through to fallback", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = fmt.Fprint(w, sampleDCGMMetrics)
		}))
		defer server.Close()

		backend := NewDCGMExporterBackend(slog.Default())
		backend.discoverEndpoint = func() string { return "http://127.0.0.1:1/metrics" }
		backend.fallbackEndpoints = []string{server.URL + "/metrics"}

		err := backend.Init(context.Background())
		require.NoError(t, err)
		assert.Equal(t, server.URL+"/metrics", backend.endpoint)
	})
}

func TestDCGMExporterBackend_CircuitBreaker(t *testing.T) {
	t.Run("trips after consecutive failures", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		ctx := context.Background()
		backend := NewDCGMExporterBackend(slog.Default())
		backend.endpoint = server.URL + "/metrics"
		backend.initialized = true

		// Make maxConsecutiveFailures calls — should trip on the last one
		for i := 0; i < maxConsecutiveFailures; i++ {
			_, err := backend.GetMIGInstanceActivity(ctx, 0, 1)
			assert.Error(t, err)
		}

		assert.False(t, backend.IsInitialized(), "circuit breaker should trip")
		assert.False(t, backend.circuitOpenTime.IsZero(), "circuitOpenTime should be set")
	})

	t.Run("resets on success", func(t *testing.T) {
		callCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			callCount++
			if callCount <= 2 {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			_, _ = fmt.Fprint(w, sampleDCGMMetrics)
		}))
		defer server.Close()

		ctx := context.Background()
		backend := NewDCGMExporterBackend(slog.Default())
		backend.endpoint = server.URL + "/metrics"
		backend.initialized = true

		// Two failures (below threshold)
		_, _ = backend.GetMIGInstanceActivity(ctx, 0, 1)
		_, _ = backend.GetMIGInstanceActivity(ctx, 0, 1)
		assert.True(t, backend.IsInitialized(), "should still be initialized")
		assert.Equal(t, 2, backend.consecutiveFailures)

		// Success resets counter
		activity, err := backend.GetMIGInstanceActivity(ctx, 0, 1)
		require.NoError(t, err)
		assert.Equal(t, 0.75, activity)
		assert.Equal(t, 0, backend.consecutiveFailures)
	})

	t.Run("re-init after interval", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = fmt.Fprint(w, sampleDCGMMetrics)
		}))
		defer server.Close()

		ctx := context.Background()
		backend := NewDCGMExporterBackend(slog.Default())
		backend.endpoint = server.URL + "/metrics"
		// Simulate tripped circuit breaker
		backend.initialized = false
		backend.circuitOpenTime = time.Now().Add(-reInitInterval - time.Second)

		// Should attempt re-init and succeed
		activity, err := backend.GetMIGInstanceActivity(ctx, 0, 1)
		require.NoError(t, err)
		assert.Equal(t, 0.75, activity)
		assert.True(t, backend.IsInitialized(), "should be re-initialized")
		assert.Equal(t, 0, backend.consecutiveFailures)
	})

	t.Run("no re-init before interval", func(t *testing.T) {
		ctx := context.Background()
		backend := NewDCGMExporterBackend(slog.Default())
		backend.endpoint = "http://127.0.0.1:1/metrics"
		// Simulate recently tripped circuit breaker
		backend.initialized = false
		backend.circuitOpenTime = time.Now()

		_, err := backend.GetMIGInstanceActivity(ctx, 0, 1)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "circuit breaker open")
		assert.False(t, backend.IsInitialized())
	})

	t.Run("re-init fails resets timer", func(t *testing.T) {
		ctx := context.Background()
		backend := NewDCGMExporterBackend(slog.Default())
		backend.endpoint = "http://127.0.0.1:1/metrics"
		backend.initialized = false
		backend.circuitOpenTime = time.Now().Add(-reInitInterval - time.Second)

		_, err := backend.GetMIGInstanceActivity(ctx, 0, 1)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "re-init failed")
		assert.False(t, backend.IsInitialized())
		// Timer should be reset
		assert.True(t, time.Since(backend.circuitOpenTime) < time.Second)
	})

	t.Run("shutdown resets circuit breaker", func(t *testing.T) {
		backend := NewDCGMExporterBackend(slog.Default())
		backend.initialized = true
		backend.consecutiveFailures = 5
		backend.circuitOpenTime = time.Now()

		err := backend.Shutdown()
		assert.NoError(t, err)
		assert.Equal(t, 0, backend.consecutiveFailures)
		assert.True(t, backend.circuitOpenTime.IsZero())
	})
}
