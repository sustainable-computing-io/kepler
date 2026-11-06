// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package e2e_k8s

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/sustainable-computing-io/kepler/test/common"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

var (
	containerGPULabels = []string{"container_id", "container_name", "runtime", "state", "pod_id"}
	podGPULabels       = []string{"pod_id", "pod_name", "pod_namespace", "state"}
)

// gpuAttributionTolerance is 1% for same-snapshot process→container→pod GPU
// watt identity checks. Leave powerTolerance (10%) for CPU / timing-sensitive tests.
const gpuAttributionTolerance = 0.01

// TestFakeGPUContainerPodAttribution waits for the fake GPU idle reading,
// starts tagged stress-ng workers, and checks container and pod GPU series.
//
// Requires Kepler from manifests/overlays/e2e (dev.fake-gpu-meter plus
// KEPLER_FAKE_GPU_WORKLOAD_DIR=/fake-gpu-workload). Missing node GPU series
// is a failure, not a skip. Attribution depends on /host/proc/<pid>/cwd
// (readlink) equaling that path after Abs+EvalSymlinks in the Kepler container.
func TestFakeGPUContainerPodAttribution(t *testing.T) {
	feature := features.New("fake-gpu-container-pod-attribution").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			deleteFakeGPUStressWorkload(ctx, t, cfg)
			requireFakeGPUIdle(ctx, t)
			deployFakeGPUStressWorkload(ctx, t, cfg)
			return ctx
		}).
		Assess("container and pod GPU watts and joules", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			ok := waitForMetricCondition(ctx, t, fakeGPUWorkloadAttributed, waitForGPUMetrics)
			require.True(t, ok, "kepler_container_gpu_watts / kepler_pod_gpu_watts (and joules) should appear for the fake-GPU stress workload")

			snapshot := takeSnapshot(t)
			assertFakeGPUContainerSeries(t, snapshot)
			assertFakeGPUPodSeries(t, snapshot)
			assertFakeGPUWattsAttribution(t, snapshot)
			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			deleteFakeGPUStressWorkload(ctx, t, cfg)
			return ctx
		}).
		Feature()

	testenv.Test(t, feature)
}

func requireFakeGPUIdle(ctx context.Context, t *testing.T) {
	t.Helper()

	if !waitForMetricCondition(ctx, t, func(s *common.MetricsSnapshot) bool {
		return s.HasMetric("kepler_node_gpu_watts")
	}, 30*time.Second) {
		require.Fail(t, "kepler_node_gpu_watts is required; deploy Kepler with the e2e overlay "+
			"(KUSTOMIZE_DIR=manifests/overlays/e2e KUSTOMIZE_FLAGS=--load-restrictor=LoadRestrictionsNone)")
	}

	ok := waitForMetricCondition(ctx, t, func(s *common.MetricsSnapshot) bool {
		total := s.SumValues("kepler_node_gpu_watts", nil)
		active := s.SumValues("kepler_node_gpu_active_watts", nil)
		return math.Abs(active) <= absolutePowerTolerance &&
			math.Abs(total-fakeGPUIdleWatts) <= math.Max(fakeGPUIdleWatts*powerTolerance, absolutePowerTolerance)
	}, waitForGPUMetrics)
	require.True(t, ok, "Kepler never reported the fake GPU as idle (~0 W active, ~%.0f W total)", fakeGPUIdleWatts)
}

func fakeGPUWorkloadAttributed(s *common.MetricsSnapshot) bool {
	containerWatts := gpuStressContainerSum(s, "kepler_container_gpu_watts")
	podWatts := gpuStressPodSum(s, "kepler_pod_gpu_watts")
	containerJoules := gpuStressContainerSum(s, "kepler_container_gpu_joules_total")
	podJoules := gpuStressPodSum(s, "kepler_pod_gpu_joules_total")
	if containerWatts <= 0 || podWatts <= 0 || containerJoules <= 0 || podJoules <= 0 {
		return false
	}
	ids := gpuStressContainerIDs(s)
	return len(ids) > 0 && sumProcessGPUWatts(s, ids) > 0
}

func gpuStressContainerSum(s *common.MetricsSnapshot, name string) float64 {
	var total float64
	for _, m := range s.GetAllWithName(name) {
		if m.Labels["container_name"] == gpuStressContainerName && m.Labels["state"] == "running" {
			total += m.Value
		}
	}
	return total
}

func gpuStressPodSum(s *common.MetricsSnapshot, name string) float64 {
	var total float64
	for _, m := range s.GetAllWithName(name) {
		if isFakeGPUStressPodMetric(m, "running") {
			total += m.Value
		}
	}
	return total
}

func assertFakeGPUContainerSeries(t *testing.T, snapshot *common.MetricsSnapshot) {
	t.Helper()

	watts := gpuStressContainerMetrics(snapshot, "kepler_container_gpu_watts")
	joules := gpuStressContainerMetrics(snapshot, "kepler_container_gpu_joules_total")
	require.NotEmpty(t, watts, "kepler_container_gpu_watts should exist for %s", gpuStressContainerName)
	require.NotEmpty(t, joules, "kepler_container_gpu_joules_total should exist for %s", gpuStressContainerName)

	for _, m := range watts {
		assertRequiredLabels(t, m, containerGPULabels, "kepler_container_gpu_watts")
		assert.Equal(t, gpuStressContainerName, m.Labels["container_name"])
		assert.Equal(t, "running", m.Labels["state"])
		assert.NotEmpty(t, m.Labels["container_id"])
		assert.NotEmpty(t, m.Labels["runtime"])
		assert.NotEmpty(t, m.Labels["pod_id"])
		assert.Greater(t, m.Value, float64(0), "container GPU watts should be > 0")
	}
	for _, m := range joules {
		assertRequiredLabels(t, m, containerGPULabels, "kepler_container_gpu_joules_total")
		assert.Greater(t, m.Value, float64(0), "container GPU joules should be > 0")
	}

	t.Logf("fake GPU container watts=%.4f W joules=%.4f J",
		gpuStressContainerSum(snapshot, "kepler_container_gpu_watts"),
		gpuStressContainerSum(snapshot, "kepler_container_gpu_joules_total"))
}

func assertFakeGPUPodSeries(t *testing.T, snapshot *common.MetricsSnapshot) {
	t.Helper()

	watts := gpuStressPodMetrics(snapshot, "kepler_pod_gpu_watts")
	joules := gpuStressPodMetrics(snapshot, "kepler_pod_gpu_joules_total")
	require.NotEmpty(t, watts, "kepler_pod_gpu_watts should exist for %s", gpuStressDaemonSetName)
	require.NotEmpty(t, joules, "kepler_pod_gpu_joules_total should exist for %s", gpuStressDaemonSetName)

	for _, m := range watts {
		assertRequiredLabels(t, m, podGPULabels, "kepler_pod_gpu_watts")
		assert.Equal(t, "running", m.Labels["state"])
		assert.Equal(t, testNamespace, m.Labels["pod_namespace"])
		assert.NotEmpty(t, m.Labels["pod_id"])
		assert.Greater(t, m.Value, float64(0), "pod GPU watts should be > 0")
	}
	for _, m := range joules {
		assertRequiredLabels(t, m, podGPULabels, "kepler_pod_gpu_joules_total")
		assert.Greater(t, m.Value, float64(0), "pod GPU joules should be > 0")
	}

	t.Logf("fake GPU pod watts=%.4f W joules=%.4f J",
		gpuStressPodSum(snapshot, "kepler_pod_gpu_watts"),
		gpuStressPodSum(snapshot, "kepler_pod_gpu_joules_total"))
}

// assertFakeGPUWattsAttribution checks container GPU watts ≈ Σ process GPU watts
// for the same container_id, and pod GPU watts ≈ Σ container GPU watts for the
// same pod_id. Process GPU series have container_id, not pod_id.
func assertFakeGPUWattsAttribution(t *testing.T, snapshot *common.MetricsSnapshot) {
	t.Helper()

	processByContainer := processGPUWattsByContainer(snapshot)
	require.NotEmpty(t, processByContainer, "kepler_process_gpu_watts should exist for the fake-GPU stress containers")

	for _, container := range gpuStressContainerMetrics(snapshot, "kepler_container_gpu_watts") {
		id := container.Labels["container_id"]
		processSum, ok := processByContainer[id]
		require.True(t, ok && processSum > 0,
			"kepler_process_gpu_watts should exist for container_id=%s", id)
		t.Logf("container_id=%s: container=%.4f W process_sum=%.4f W", id, container.Value, processSum)
		assertWithinTolerance(t, processSum, container.Value, gpuAttributionTolerance, absolutePowerTolerance,
			"container %s GPU watts should equal Σ process GPU watts", truncateID(id))
	}

	containerByPod := map[string]float64{}
	for _, container := range gpuStressContainerMetrics(snapshot, "kepler_container_gpu_watts") {
		if podID := container.Labels["pod_id"]; podID != "" {
			containerByPod[podID] += container.Value
		}
	}

	for _, pod := range gpuStressPodMetrics(snapshot, "kepler_pod_gpu_watts") {
		id := pod.Labels["pod_id"]
		containerSum, ok := containerByPod[id]
		require.True(t, ok && containerSum > 0,
			"kepler_container_gpu_watts should exist for pod_id=%s", id)
		t.Logf("pod_id=%s: pod=%.4f W container_sum=%.4f W", id, pod.Value, containerSum)
		assertWithinTolerance(t, containerSum, pod.Value, gpuAttributionTolerance, absolutePowerTolerance,
			"pod %s GPU watts should equal Σ container GPU watts", truncateID(id))
	}
}

func gpuStressContainerIDs(s *common.MetricsSnapshot) map[string]struct{} {
	ids := make(map[string]struct{})
	for _, m := range gpuStressContainerMetrics(s, "kepler_container_gpu_watts") {
		if id := m.Labels["container_id"]; id != "" {
			ids[id] = struct{}{}
		}
	}
	return ids
}

func processGPUWattsByContainer(s *common.MetricsSnapshot) map[string]float64 {
	byContainer := make(map[string]float64)
	for _, m := range s.GetAllWithName("kepler_process_gpu_watts") {
		if m.Labels["state"] != "running" {
			continue
		}
		id := m.Labels["container_id"]
		if id == "" {
			continue
		}
		byContainer[id] += m.Value
	}
	return byContainer
}

func sumProcessGPUWatts(s *common.MetricsSnapshot, containerIDs map[string]struct{}) float64 {
	var total float64
	byContainer := processGPUWattsByContainer(s)
	for id := range containerIDs {
		total += byContainer[id]
	}
	return total
}

func gpuStressContainerMetrics(s *common.MetricsSnapshot, name string) []common.Metric {
	var out []common.Metric
	for _, m := range s.GetAllWithName(name) {
		if m.Labels["container_name"] == gpuStressContainerName && m.Labels["state"] == "running" {
			out = append(out, m)
		}
	}
	return out
}

func gpuStressPodMetrics(s *common.MetricsSnapshot, name string) []common.Metric {
	var out []common.Metric
	for _, m := range s.GetAllWithName(name) {
		if isFakeGPUStressPodMetric(m, "running") {
			out = append(out, m)
		}
	}
	return out
}

func assertRequiredLabels(t *testing.T, m common.Metric, labels []string, metricName string) {
	t.Helper()
	for _, label := range labels {
		assert.Contains(t, m.Labels, label, "%s should have %s label", metricName, label)
	}
}
