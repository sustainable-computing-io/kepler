// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNodeMetricsHaveZoneLabels verifies that node metrics have zone labels
func TestNodeMetricsHaveZoneLabels(t *testing.T) {
	_, scraper := setupKeplerForTest(t)

	time.Sleep(5 * time.Second)

	metrics, err := scraper.ScrapeMetric("kepler_node_cpu_watts")
	require.NoError(t, err, "Failed to scrape node CPU watts")
	require.NotEmpty(t, metrics, "Should have node CPU watts metrics")

	for _, m := range metrics {
		assert.Contains(t, m.Labels, "zone", "Node metric should have zone label")
		assert.NotEmpty(t, m.Labels["zone"], "Zone label should not be empty")
	}

	zones := make(map[string]bool)
	for _, m := range metrics {
		zones[m.Labels["zone"]] = true
	}
	t.Logf("Found RAPL zones: %v", zones)
}

// TestProcessMetricsHaveRequiredLabels verifies that process metrics have required labels
func TestProcessMetricsHaveRequiredLabels(t *testing.T) {
	_, scraper := setupKeplerForTest(t)

	time.Sleep(5 * time.Second)

	metrics, err := scraper.ScrapeMetric("kepler_process_cpu_watts")
	require.NoError(t, err, "Failed to scrape process CPU watts")
	require.NotEmpty(t, metrics, "Should have process CPU watts metrics")

	requiredLabels := []string{"pid", "comm", "exe", "type", "state", "zone"}

	for _, m := range metrics {
		for _, label := range requiredLabels {
			assert.Contains(t, m.Labels, label, "Process metric should have %s label", label)
		}
	}

	t.Logf("Sample processes found:")
	count := 0
	for _, m := range metrics {
		if count >= 5 {
			break
		}
		t.Logf("  PID=%s comm=%s power=%.4f W", m.Labels["pid"], m.Labels["comm"], m.Value)
		count++
	}
}

// TestMetricsArePositiveOrZero verifies that power metrics are non-negative
func TestMetricsArePositiveOrZero(t *testing.T) {
	_, scraper := setupKeplerForTest(t)

	time.Sleep(5 * time.Second)

	snapshot, err := scraper.TakeSnapshot()
	require.NoError(t, err, "Failed to take snapshot")

	nodeWatts := snapshot.GetAllWithName("kepler_node_cpu_watts")
	for _, m := range nodeWatts {
		assert.GreaterOrEqual(t, m.Value, float64(0), "Node CPU watts should be >= 0")
	}

	processWatts := snapshot.GetAllWithName("kepler_process_cpu_watts")
	for _, m := range processWatts {
		assert.GreaterOrEqual(t, m.Value, float64(0), "Process CPU watts should be >= 0")
	}

	nodeJoules := snapshot.GetAllWithName("kepler_node_cpu_joules_total")
	for _, m := range nodeJoules {
		assert.GreaterOrEqual(t, m.Value, float64(0), "Node CPU joules should be >= 0")
	}

	t.Logf("Verified %d node watts, %d process watts, %d node joules metrics are non-negative",
		len(nodeWatts), len(processWatts), len(nodeJoules))
}

// TestMetricsCountersScrapeableMultipleTimes verifies we can scrape repeatedly
func TestMetricsCountersScrapeableMultipleTimes(t *testing.T) {
	_, scraper := setupKeplerForTest(t)

	time.Sleep(5 * time.Second)

	var snapshots [3]*struct {
		nodeJoules   float64
		nodeWatts    float64
		processWatts float64
		processCount int
		usageRatio   float64
	}

	for i := range [3]int{} {
		snapshot, err := scraper.TakeSnapshot()
		require.NoError(t, err, "Failed to take snapshot %d", i+1)

		nodeJoules := snapshot.SumValues("kepler_node_cpu_joules_total", nil)
		nodeWatts := snapshot.SumValues("kepler_node_cpu_watts", nil)
		processWatts := snapshot.SumValues("kepler_process_cpu_watts", nil)
		processMetrics := snapshot.GetAllWithName("kepler_process_cpu_watts")
		usageRatio, _ := snapshot.GetValue("kepler_node_cpu_usage_ratio", nil)

		snapshots[i] = &struct {
			nodeJoules   float64
			nodeWatts    float64
			processWatts float64
			processCount int
			usageRatio   float64
		}{
			nodeJoules:   nodeJoules,
			nodeWatts:    nodeWatts,
			processWatts: processWatts,
			processCount: len(processMetrics),
			usageRatio:   usageRatio,
		}

		t.Logf("Snapshot %d: nodeJoules=%.2f nodeWatts=%.2f processWatts=%.2f processes=%d usage=%.2f%%",
			i+1, nodeJoules, nodeWatts, processWatts, len(processMetrics), usageRatio*100)

		if i < 2 {
			time.Sleep(3 * time.Second)
		}
	}

	for i, snap := range snapshots {
		assert.GreaterOrEqual(t, snap.nodeJoules, float64(0), "Snapshot %d: node joules should be >= 0", i+1)
		assert.GreaterOrEqual(t, snap.nodeWatts, float64(0), "Snapshot %d: node watts should be >= 0", i+1)
		assert.Greater(t, snap.processCount, 0, "Snapshot %d: should have processes", i+1)
	}
}
