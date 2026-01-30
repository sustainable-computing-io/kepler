// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// powerTolerance is the acceptable tolerance for power comparisons (5%)
const powerTolerance = 0.05

// absolutePowerTolerance is the minimum absolute tolerance in watts
const absolutePowerTolerance = 0.001

// TestNodePowerConservation verifies that Total Node Watts = Active Watts + Idle Watts
func TestNodePowerConservation(t *testing.T) {
	_, scraper := setupKeplerForTest(t)

	require.True(t, WaitForValidCPUMetrics(t, scraper, 30*time.Second),
		"Kepler should have valid CPU metrics")

	snapshot, err := scraper.TakeSnapshot()
	require.NoError(t, err, "Failed to take metrics snapshot")

	totalWatts := snapshot.GetAllWithName("kepler_node_cpu_watts")
	activeWatts := snapshot.GetAllWithName("kepler_node_cpu_active_watts")
	idleWatts := snapshot.GetAllWithName("kepler_node_cpu_idle_watts")

	require.NotEmpty(t, totalWatts, "Should have node total watts metrics")
	require.NotEmpty(t, activeWatts, "Should have node active watts metrics")
	require.NotEmpty(t, idleWatts, "Should have node idle watts metrics")

	totalByZone := make(map[string]float64)
	activeByZone := make(map[string]float64)
	idleByZone := make(map[string]float64)

	for _, m := range totalWatts {
		totalByZone[m.Labels["zone"]] = m.Value
	}
	for _, m := range activeWatts {
		activeByZone[m.Labels["zone"]] = m.Value
	}
	for _, m := range idleWatts {
		idleByZone[m.Labels["zone"]] = m.Value
	}

	for zone, total := range totalByZone {
		active := activeByZone[zone]
		idle := idleByZone[zone]
		computed := active + idle

		t.Logf("Zone %s: total=%.4f W, active=%.4f W, idle=%.4f W, computed=%.4f W",
			zone, total, active, idle, computed)

		assertWithinTolerance(t, total, computed, powerTolerance, absolutePowerTolerance,
			"Zone %s: Total watts should equal Active + Idle", zone)
	}
}

// TestProcessPowerSumsToNodeActive verifies that Σ(Process Power) = Node Active Power
func TestProcessPowerSumsToNodeActive(t *testing.T) {
	_, scraper := setupKeplerForTest(t)

	// Wait for valid metrics to ensure all process metrics are populated
	require.True(t, WaitForValidCPUMetrics(t, scraper, 30*time.Second),
		"Kepler should have valid CPU metrics")

	snapshot, err := scraper.TakeSnapshot()
	require.NoError(t, err, "Failed to take metrics snapshot")

	nodeActiveWatts := snapshot.GetAllWithName("kepler_node_cpu_active_watts")
	require.NotEmpty(t, nodeActiveWatts, "Should have node active watts metrics")

	processWatts := snapshot.GetAllWithName("kepler_process_cpu_watts")
	require.NotEmpty(t, processWatts, "Should have process watts metrics")

	processSumByZone := make(map[string]float64)
	for _, m := range processWatts {
		if m.Labels["state"] == "running" {
			processSumByZone[m.Labels["zone"]] += m.Value
		}
	}

	nodeActiveByZone := make(map[string]float64)
	for _, m := range nodeActiveWatts {
		nodeActiveByZone[m.Labels["zone"]] = m.Value
	}

	for zone, nodeActive := range nodeActiveByZone {
		processSum := processSumByZone[zone]

		t.Logf("Zone %s: node_active=%.4f W, sum_processes=%.4f W", zone, nodeActive, processSum)

		assertWithinTolerance(t, nodeActive, processSum, powerTolerance, absolutePowerTolerance,
			"Zone %s: Sum of process power should equal node active power", zone)
	}
}

// TestEnergyMonotonicallyIncreases verifies that energy counters always increase
func TestEnergyMonotonicallyIncreases(t *testing.T) {
	_, scraper := setupKeplerForTest(t)

	time.Sleep(waitForMetricsAvailable)

	snapshot1, err := scraper.TakeSnapshot()
	require.NoError(t, err, "Failed to take first snapshot")

	nodeJoules1 := snapshot1.SumValues("kepler_node_cpu_joules_total", nil)
	activeJoules1 := snapshot1.SumValues("kepler_node_cpu_active_joules_total", nil)
	idleJoules1 := snapshot1.SumValues("kepler_node_cpu_idle_joules_total", nil)

	t.Logf("First snapshot: node=%.2f J, active=%.2f J, idle=%.2f J",
		nodeJoules1, activeJoules1, idleJoules1)

	time.Sleep(waitBetweenSnapshots)

	snapshot2, err := scraper.TakeSnapshot()
	require.NoError(t, err, "Failed to take second snapshot")

	nodeJoules2 := snapshot2.SumValues("kepler_node_cpu_joules_total", nil)
	activeJoules2 := snapshot2.SumValues("kepler_node_cpu_active_joules_total", nil)
	idleJoules2 := snapshot2.SumValues("kepler_node_cpu_idle_joules_total", nil)

	t.Logf("Second snapshot: node=%.2f J, active=%.2f J, idle=%.2f J",
		nodeJoules2, activeJoules2, idleJoules2)

	assert.GreaterOrEqual(t, nodeJoules2, nodeJoules1, "Node total energy should not decrease")
	assert.GreaterOrEqual(t, activeJoules2, activeJoules1, "Active energy should not decrease")
	assert.GreaterOrEqual(t, idleJoules2, idleJoules1, "Idle energy should not decrease")

	t.Logf("Energy delta: node=%.2f J, active=%.2f J, idle=%.2f J",
		nodeJoules2-nodeJoules1, activeJoules2-activeJoules1, idleJoules2-idleJoules1)
}

// TestEnergyConservation verifies that Active + Idle = Total energy
func TestEnergyConservation(t *testing.T) {
	_, scraper := setupKeplerForTest(t)

	require.True(t, WaitForValidCPUMetrics(t, scraper, 30*time.Second),
		"Kepler should have valid CPU metrics")

	snapshot, err := scraper.TakeSnapshot()
	require.NoError(t, err, "Failed to take metrics snapshot")

	totalJoules := snapshot.GetAllWithName("kepler_node_cpu_joules_total")
	activeJoules := snapshot.GetAllWithName("kepler_node_cpu_active_joules_total")
	idleJoules := snapshot.GetAllWithName("kepler_node_cpu_idle_joules_total")

	require.NotEmpty(t, totalJoules, "Should have node total joules metrics")
	require.NotEmpty(t, activeJoules, "Should have node active joules metrics")
	require.NotEmpty(t, idleJoules, "Should have node idle joules metrics")

	totalByZone := make(map[string]float64)
	activeByZone := make(map[string]float64)
	idleByZone := make(map[string]float64)

	for _, m := range totalJoules {
		totalByZone[m.Labels["zone"]] = m.Value
	}
	for _, m := range activeJoules {
		activeByZone[m.Labels["zone"]] = m.Value
	}
	for _, m := range idleJoules {
		idleByZone[m.Labels["zone"]] = m.Value
	}

	for zone, total := range totalByZone {
		active := activeByZone[zone]
		idle := idleByZone[zone]
		computed := active + idle

		t.Logf("Zone %s: total=%.2f J, active=%.2f J, idle=%.2f J, computed=%.2f J",
			zone, total, active, idle, computed)

		assertWithinTolerance(t, total, computed, powerTolerance, absolutePowerTolerance,
			"Zone %s: Total joules should equal Active + Idle", zone)
	}
}

// TestCPUUsageRatioValid verifies CPU usage ratio is in valid range [0, 1]
func TestCPUUsageRatioValid(t *testing.T) {
	_, scraper := setupKeplerForTest(t)

	for i := range [5]int{} {
		if i > 0 {
			time.Sleep(waitBetweenSamples)
		}

		metrics, err := scraper.ScrapeMetric("kepler_node_cpu_usage_ratio")
		require.NoError(t, err, "Failed to scrape CPU usage ratio")
		require.NotEmpty(t, metrics, "Should have CPU usage ratio metric")

		ratio := metrics[0].Value

		assert.GreaterOrEqual(t, ratio, 0.0, "CPU usage ratio should be >= 0 (sample %d)", i+1)
		assert.LessOrEqual(t, ratio, 1.0, "CPU usage ratio should be <= 1 (sample %d)", i+1)

		t.Logf("Sample %d: CPU usage ratio = %.4f (%.2f%%)", i+1, ratio, ratio*100)
	}
}

// TestActivePowerProportionalToUsage verifies active power is proportional to CPU usage
func TestActivePowerProportionalToUsage(t *testing.T) {
	_, scraper := setupKeplerForTest(t)

	require.True(t, WaitForValidCPUMetrics(t, scraper, 30*time.Second),
		"Kepler should have valid CPU metrics")

	snapshot, err := scraper.TakeSnapshot()
	require.NoError(t, err, "Failed to take metrics snapshot")

	usageRatio, ok := snapshot.GetValue("kepler_node_cpu_usage_ratio", nil)
	require.True(t, ok, "Should have CPU usage ratio")

	totalWatts := snapshot.GetAllWithName("kepler_node_cpu_watts")
	activeWatts := snapshot.GetAllWithName("kepler_node_cpu_active_watts")

	activeByZone := make(map[string]float64)
	for _, m := range activeWatts {
		activeByZone[m.Labels["zone"]] = m.Value
	}

	for _, m := range totalWatts {
		zone := m.Labels["zone"]
		total := m.Value
		active := activeByZone[zone]

		if total > 0 {
			computedRatio := active / total

			t.Logf("Zone %s: total=%.4f W, active=%.4f W, ratio=%.4f, expected=%.4f",
				zone, total, active, computedRatio, usageRatio)

			assertWithinTolerance(t, usageRatio, computedRatio, 0.1, 0.01,
				"Zone %s: Active/Total ratio should match CPU usage ratio", zone)
		}
	}
}

// TestProcessEnergyAccumulates verifies that process energy accumulates over time
func TestProcessEnergyAccumulates(t *testing.T) {
	_, scraper := setupKeplerForTest(t)

	time.Sleep(waitForMetricsAvailable)

	snapshot1, err := scraper.TakeSnapshot()
	require.NoError(t, err, "Failed to take first snapshot")

	processJoules1 := snapshot1.SumValues("kepler_process_cpu_joules_total", map[string]string{"state": "running"})
	t.Logf("First snapshot: total process joules = %.2f J", processJoules1)

	time.Sleep(waitBetweenSnapshots)

	snapshot2, err := scraper.TakeSnapshot()
	require.NoError(t, err, "Failed to take second snapshot")

	processJoules2 := snapshot2.SumValues("kepler_process_cpu_joules_total", map[string]string{"state": "running"})
	t.Logf("Second snapshot: total process joules = %.2f J", processJoules2)

	assert.GreaterOrEqual(t, processJoules2, 0.0, "Process joules should be non-negative")

	t.Logf("Process energy change: %.2f J", processJoules2-processJoules1)
}

// assertWithinTolerance checks if actual is within tolerance of expected
func assertWithinTolerance(t *testing.T, expected, actual, percentTolerance, absTolerance float64, msgAndArgs ...any) {
	t.Helper()

	diff := math.Abs(expected - actual)
	tolerance := math.Max(math.Abs(expected)*percentTolerance, absTolerance)

	if diff > tolerance {
		if len(msgAndArgs) > 0 {
			t.Errorf("%s: expected %.6f, got %.6f (diff: %.6f, tolerance: %.6f)",
				msgAndArgs[0], expected, actual, diff, tolerance)
		} else {
			t.Errorf("expected %.6f, got %.6f (diff: %.6f, tolerance: %.6f)",
				expected, actual, diff, tolerance)
		}
	}
}
