// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestKeplerStarts verifies that Kepler starts successfully and is running
func TestKeplerStarts(t *testing.T) {
	kepler, _ := setupKeplerForTest(t)

	assert.True(t, kepler.IsRunning(), "Kepler should be running")
	assert.Greater(t, kepler.PID(), 0, "Kepler should have a valid PID")

	t.Logf("Kepler is running with PID %d", kepler.PID())
}

// TestMetricsEndpointAvailable verifies the /metrics endpoint returns successfully
func TestMetricsEndpointAvailable(t *testing.T) {
	_, scraper := setupKeplerForTest(t)

	families, err := scraper.Scrape()
	require.NoError(t, err, "Failed to scrape metrics")
	require.NotEmpty(t, families, "Metrics response should not be empty")

	t.Logf("Successfully scraped %d metric families", len(families))
}

// TestBuildInfoPresent verifies that kepler_build_info metric exists
func TestBuildInfoPresent(t *testing.T) {
	_, scraper := setupKeplerForTest(t)

	metrics, err := scraper.ScrapeMetric("kepler_build_info")
	require.NoError(t, err, "kepler_build_info metric should exist")
	require.NotEmpty(t, metrics, "kepler_build_info should have at least one metric")

	m := metrics[0]
	assert.Contains(t, m.Labels, "version", "build_info should have version label")
	assert.Contains(t, m.Labels, "revision", "build_info should have revision label")
	assert.Equal(t, float64(1), m.Value, "build_info value should be 1")

	t.Logf("Kepler version: %s, revision: %s", m.Labels["version"], m.Labels["revision"])
}

// TestNodeMetricsPresent verifies that node-level metrics are exported
func TestNodeMetricsPresent(t *testing.T) {
	_, scraper := setupKeplerForTest(t)

	time.Sleep(5 * time.Second)

	expectedMetrics := []string{
		"kepler_node_cpu_joules_total",
		"kepler_node_cpu_watts",
		"kepler_node_cpu_active_joules_total",
		"kepler_node_cpu_active_watts",
		"kepler_node_cpu_idle_joules_total",
		"kepler_node_cpu_idle_watts",
		"kepler_node_cpu_usage_ratio",
	}

	snapshot, err := scraper.TakeSnapshot()
	require.NoError(t, err, "Failed to take metrics snapshot")

	for _, metricName := range expectedMetrics {
		assert.True(t, snapshot.HasMetric(metricName), "Expected metric %s to be present", metricName)
	}

	t.Logf("All expected node metrics are present")
}

// TestProcessMetricsPresent verifies that process-level metrics are exported
func TestProcessMetricsPresent(t *testing.T) {
	_, scraper := setupKeplerForTest(t)

	time.Sleep(5 * time.Second)

	expectedMetrics := []string{
		"kepler_process_cpu_joules_total",
		"kepler_process_cpu_watts",
		"kepler_process_cpu_seconds_total",
	}

	snapshot, err := scraper.TakeSnapshot()
	require.NoError(t, err, "Failed to take metrics snapshot")

	for _, metricName := range expectedMetrics {
		assert.True(t, snapshot.HasMetric(metricName), "Expected metric %s to be present", metricName)
	}

	processMetrics := snapshot.GetAllWithName("kepler_process_cpu_watts")
	assert.NotEmpty(t, processMetrics, "Should have at least one process metric")

	t.Logf("Found %d process metrics", len(processMetrics))
}

// TestCPUInfoPresent verifies that CPU info metric is exported
func TestCPUInfoPresent(t *testing.T) {
	_, scraper := setupKeplerForTest(t)

	metrics, err := scraper.ScrapeMetric("kepler_node_cpu_info")
	require.NoError(t, err, "kepler_node_cpu_info metric should exist")
	require.NotEmpty(t, metrics, "Should have CPU info for at least one processor")

	m := metrics[0]
	expectedLabels := []string{"processor", "vendor_id", "model_name", "physical_id", "core_id"}
	for _, label := range expectedLabels {
		assert.Contains(t, m.Labels, label, "CPU info should have %s label", label)
	}

	t.Logf("Found %d CPU(s) with info", len(metrics))
	if len(metrics) > 0 {
		t.Logf("CPU 0: %s", metrics[0].Labels["model_name"])
	}
}

// TestKeplerGracefulShutdown verifies that Kepler shuts down cleanly
func TestKeplerGracefulShutdown(t *testing.T) {
	kepler, _ := setupKeplerForTest(t)

	require.True(t, kepler.IsRunning(), "Kepler should be running initially")

	err := kepler.Stop()
	require.NoError(t, err, "Kepler should stop without error")

	time.Sleep(500 * time.Millisecond)

	assert.False(t, kepler.IsRunning(), "Kepler should not be running after stop")
}
