// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/sustainable-computing-io/kepler/test/common"
)

// waitForGPUMetrics waits until Kepler exposes GPU node metrics.
// This indicates the fake GPU backend has initialized and the monitor
// has completed at least one refresh cycle with GPU data.
func waitForGPUMetrics(t *testing.T, scraper *common.MetricsScraper, timeout time.Duration) bool {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	err := common.WaitForCondition(ctx, 2*time.Second, func() bool {
		metrics, err := scraper.ScrapeMetric("kepler_node_gpu_watts")
		return err == nil && len(metrics) > 0
	})

	return err == nil
}

// TestGPUNodeMetricsPresent verifies that GPU node-level metrics exist when
// the fake GPU meter is enabled in e2e-config.yaml.
func TestGPUNodeMetricsPresent(t *testing.T) {
	_, scraper := setupKeplerForTest(t)

	require.True(t, waitForGPUMetrics(t, scraper, 30*time.Second),
		"Kepler should have GPU metrics when fake GPU meter is enabled")

	snapshot, err := scraper.TakeSnapshot()
	require.NoError(t, err, "Failed to take metrics snapshot")

	gpuMetrics := []string{
		"kepler_node_gpu_watts",
		"kepler_node_gpu_idle_watts",
		"kepler_node_gpu_active_watts",
	}

	for _, metric := range gpuMetrics {
		assert.True(t, snapshot.HasMetric(metric),
			"metric %s should exist when fake GPU meter is enabled", metric)
	}

	// GPU info metric should also be present
	assert.True(t, snapshot.HasMetric("kepler_node_gpu_info"),
		"kepler_node_gpu_info should exist when fake GPU meter is enabled")
}

// TestGPUNodeMetricsLabels verifies GPU node metrics have required labels.
func TestGPUNodeMetricsLabels(t *testing.T) {
	_, scraper := setupKeplerForTest(t)

	if !waitForGPUMetrics(t, scraper, 30*time.Second) {
		t.Skip("Skipping: no GPU metrics present (fake GPU meter may not be enabled)")
	}

	snapshot, err := scraper.TakeSnapshot()
	require.NoError(t, err)

	requiredLabels := []string{"gpu", "gpu_uuid", "gpu_name", "vendor"}

	metrics := snapshot.GetAllWithName("kepler_node_gpu_watts")
	require.NotEmpty(t, metrics)

	for _, m := range metrics {
		for _, label := range requiredLabels {
			assert.Contains(t, m.Labels, label,
				"kepler_node_gpu_watts should have %s label", label)
		}
	}
}

// TestGPUNodeMetricsNonNegative verifies all GPU power metric values are >= 0.
func TestGPUNodeMetricsNonNegative(t *testing.T) {
	_, scraper := setupKeplerForTest(t)

	if !waitForGPUMetrics(t, scraper, 30*time.Second) {
		t.Skip("Skipping: no GPU metrics present (fake GPU meter may not be enabled)")
	}

	snapshot, err := scraper.TakeSnapshot()
	require.NoError(t, err)

	gpuMetrics := []string{
		"kepler_node_gpu_watts",
		"kepler_node_gpu_idle_watts",
		"kepler_node_gpu_active_watts",
	}

	for _, metricName := range gpuMetrics {
		metrics := snapshot.GetAllWithName(metricName)
		for _, m := range metrics {
			assert.GreaterOrEqual(t, m.Value, float64(0),
				"%s should be >= 0 (gpu=%s)", metricName, m.Labels["gpu"])
		}
	}
}

// TestGPUNodePowerConservation verifies: Total GPU Watts = Active + Idle.
func TestGPUNodePowerConservation(t *testing.T) {
	_, scraper := setupKeplerForTest(t)

	if !waitForGPUMetrics(t, scraper, 30*time.Second) {
		t.Skip("Skipping: no GPU metrics present (fake GPU meter may not be enabled)")
	}

	snapshot, err := scraper.TakeSnapshot()
	require.NoError(t, err)

	totalMetrics := snapshot.GetAllWithName("kepler_node_gpu_watts")
	require.NotEmpty(t, totalMetrics)

	activeMetrics := snapshot.GetAllWithName("kepler_node_gpu_active_watts")
	idleMetrics := snapshot.GetAllWithName("kepler_node_gpu_idle_watts")

	// Build maps by gpu label for matching
	activeByGPU := make(map[string]float64)
	for _, m := range activeMetrics {
		activeByGPU[m.Labels["gpu"]] = m.Value
	}
	idleByGPU := make(map[string]float64)
	for _, m := range idleMetrics {
		idleByGPU[m.Labels["gpu"]] = m.Value
	}

	verified := 0
	for _, m := range totalMetrics {
		gpuLabel := m.Labels["gpu"]
		active := activeByGPU[gpuLabel]
		idle := idleByGPU[gpuLabel]
		computed := active + idle

		t.Logf("GPU %s: total=%.4f W, active=%.4f W, idle=%.4f W, computed=%.4f W",
			gpuLabel, m.Value, active, idle, computed)

		diff := math.Abs(m.Value - computed)
		tolerance := math.Max(m.Value*powerTolerance, absolutePowerTolerance)

		assert.LessOrEqual(t, diff, tolerance,
			"GPU %s: total (%.4f) should equal active + idle (%.4f)", gpuLabel, m.Value, computed)

		verified++
	}

	assert.Greater(t, verified, 0, "Should verify at least one GPU power conservation")
}
