// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"io"
	"net/http"
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

// TestExpectedMetricsPresent verifies that expected node and process metrics are exported
func TestExpectedMetricsPresent(t *testing.T) {
	_, scraper := setupKeplerForTest(t)

	require.True(t, WaitForValidCPUMetrics(t, scraper, 30*time.Second),
		"Kepler should have valid CPU metrics")

	snapshot, err := scraper.TakeSnapshot()
	require.NoError(t, err, "Failed to take metrics snapshot")

	tests := []struct {
		name    string
		metrics []string
	}{
		{
			name: "node level",
			metrics: []string{
				"kepler_node_cpu_joules_total",
				"kepler_node_cpu_watts",
				"kepler_node_cpu_active_joules_total",
				"kepler_node_cpu_active_watts",
				"kepler_node_cpu_idle_joules_total",
				"kepler_node_cpu_idle_watts",
				"kepler_node_cpu_usage_ratio",
			},
		},
		{
			name: "process level",
			metrics: []string{
				"kepler_process_cpu_joules_total",
				"kepler_process_cpu_watts",
				"kepler_process_cpu_seconds_total",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			for _, metricName := range tc.metrics {
				assert.True(t, snapshot.HasMetric(metricName),
					"Expected metric %s to be present", metricName)
			}
			t.Logf("All expected %s metrics are present", tc.name)
		})
	}

	// Additional check: verify we have actual process data
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

	time.Sleep(waitForProcessStabilization)

	assert.False(t, kepler.IsRunning(), "Kepler should not be running after stop")
}

// TestLandingPageAvailable verifies that the root endpoint returns the service landing page
func TestLandingPageAvailable(t *testing.T) {
	kepler, _ := setupKeplerForTest(t)

	resp, err := http.Get(kepler.BaseURL() + "/")
	require.NoError(t, err, "Failed to fetch landing page")
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "Landing page should return 200 OK")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "Failed to read response body")

	bodyStr := string(body)
	assert.Contains(t, bodyStr, "Kepler", "Landing page should mention Kepler")
	assert.Contains(t, bodyStr, "/metrics", "Landing page should link to /metrics endpoint")

	t.Logf("Landing page returned %d bytes", len(body))
}
