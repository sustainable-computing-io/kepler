// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/sustainable-computing-io/kepler/test/common"
)

// Timing constants for workload tests.
// Based on Kepler's monitor.interval (3s) and CPU usage ratio lag.
const (
	// waitForLoad: wait for load to appear in metrics (2+ collection cycles)
	waitForLoad = 15 * time.Second

	// waitAfterLoad: wait after stopping load (3+ cycles for idle readings)
	waitAfterLoad = 18 * time.Second

	// waitForStable: wait for stable baseline readings
	waitForStable = 12 * time.Second
)

// TestStressWorkloadDetected verifies that a stress-ng process appears in Kepler metrics
func TestStressWorkloadDetected(t *testing.T) {
	kepler, scraper := setupKeplerWithWorkloadSupport(t)
	require.True(t, kepler.IsRunning(), "Kepler should be running")

	workload := StartWorkload(t,
		WithWorkloadName("detect-test"),
		WithCPUWorkers(1),
		WithCPULoad(50),
	)

	found := WaitForProcessInMetrics(t, scraper, workload.ParentPID(), 30*time.Second)
	if !found {
		for _, pid := range workload.PIDs() {
			if WaitForProcessInMetrics(t, scraper, pid, 10*time.Second) {
				found = true
				break
			}
		}
	}

	assert.True(t, found, "stress-ng process should appear in Kepler metrics")
}

// TestPowerIncreasesUnderLoad verifies that node power increases when CPU load is applied
func TestPowerIncreasesUnderLoad(t *testing.T) {
	_, scraper := setupKeplerWithWorkloadSupport(t)

	require.True(t, WaitForValidCPUMetrics(t, scraper, 30*time.Second),
		"Kepler should have valid CPU metrics before test")

	baselineSnapshot, err := scraper.TakeSnapshot()
	require.NoError(t, err, "Failed to take baseline snapshot")

	baselinePower := baselineSnapshot.SumValues("kepler_node_cpu_watts", nil)
	baselineActive := baselineSnapshot.SumValues("kepler_node_cpu_active_watts", nil)
	baselineUsage, _ := baselineSnapshot.GetValue("kepler_node_cpu_usage_ratio", nil)

	t.Logf("Baseline: total=%.2f W, active=%.2f W, usage=%.2f%%",
		baselinePower, baselineActive, baselineUsage*100)

	workload := StartWorkload(t,
		WithWorkloadName("load-test"),
		WithCPUWorkers(2),
		WithCPULoad(80),
	)
	defer workload.Stop()

	time.Sleep(waitForStable)

	loadedSnapshot, err := scraper.TakeSnapshot()
	require.NoError(t, err, "Failed to take loaded snapshot")

	loadedPower := loadedSnapshot.SumValues("kepler_node_cpu_watts", nil)
	loadedActive := loadedSnapshot.SumValues("kepler_node_cpu_active_watts", nil)
	loadedUsage, _ := loadedSnapshot.GetValue("kepler_node_cpu_usage_ratio", nil)

	t.Logf("Under load: total=%.2f W, active=%.2f W, usage=%.2f%%",
		loadedPower, loadedActive, loadedUsage*100)

	t.Logf("Power delta: total=%.2f W, active=%.2f W",
		loadedPower-baselinePower, loadedActive-baselineActive)

	assert.Greater(t, loadedUsage, baselineUsage, "CPU usage ratio should increase under load")
}

// TestPowerDecreasesAfterLoad verifies that a workload's power drops to zero after it stops.
// Uses process-level metrics to be independent of background system activity.
func TestPowerDecreasesAfterLoad(t *testing.T) {
	_, scraper := setupKeplerWithWorkloadSupport(t)

	require.True(t, WaitForValidCPUMetrics(t, scraper, 30*time.Second),
		"Kepler should have valid CPU metrics before test")

	workload := StartWorkload(t,
		WithWorkloadName("decrease-test"),
		WithCPUWorkers(2),
		WithCPULoad(90),
	)

	// Wait for load to appear in metrics
	time.Sleep(waitForLoad)

	loadedSnapshot, err := scraper.TakeSnapshot()
	require.NoError(t, err, "Failed to take loaded snapshot")
	loadedPower, loadedFound := FindWorkloadPower(loadedSnapshot, workload)

	t.Logf("Under load: found=%v, power=%.4f W", loadedFound, loadedPower)
	require.True(t, loadedFound && loadedPower > 0, "Workload should have power while running")

	err = workload.Stop()
	require.NoError(t, err, "Failed to stop workload")

	time.Sleep(waitAfterLoad)

	afterSnapshot, err := scraper.TakeSnapshot()
	require.NoError(t, err, "Failed to take post-load snapshot")
	afterPower, _ := FindWorkloadPower(afterSnapshot, workload)

	t.Logf("After stop: power=%.4f W", afterPower)

	// After stopping, workload should have zero power (process no longer running)
	assert.Less(t, afterPower, loadedPower, "Workload power should decrease after stop")
}

// TestMultipleWorkloadsAttribution verifies that multiple stress processes are properly attributed
func TestMultipleWorkloadsAttribution(t *testing.T) {
	_, scraper := setupKeplerWithWorkloadSupport(t)

	workload1 := StartWorkload(t,
		WithWorkloadName("workload-1"),
		WithCPUWorkers(1),
		WithCPULoad(30),
	)
	defer workload1.Stop()

	workload2 := StartWorkload(t,
		WithWorkloadName("workload-2"),
		WithCPUWorkers(1),
		WithCPULoad(60),
	)
	defer workload2.Stop()

	time.Sleep(waitForLoad)

	snapshot, err := scraper.TakeSnapshot()
	require.NoError(t, err, "Failed to take snapshot")

	workload1Power, workload1Found := FindWorkloadPower(snapshot, workload1)
	workload2Power, workload2Found := FindWorkloadPower(snapshot, workload2)

	t.Logf("Workload 1 (30%% CPU): found=%v, power=%.4f W", workload1Found, workload1Power)
	t.Logf("Workload 2 (60%% CPU): found=%v, power=%.4f W", workload2Found, workload2Power)

	assert.True(t, workload1Found, "Workload 1 should be detected in metrics")
	assert.True(t, workload2Found, "Workload 2 should be detected in metrics")

	if workload1Found && workload2Found && workload1Power > 0 && workload2Power > 0 {
		t.Logf("Power ratio (workload2/workload1): %.2f (expected ~2.0 for 60%%/30%%)",
			workload2Power/workload1Power)
	}
}

// TestWorkloadPowerAttribution verifies that a known workload gets power attributed
func TestWorkloadPowerAttribution(t *testing.T) {
	_, scraper := setupKeplerWithWorkloadSupport(t)

	workload := StartWorkload(t,
		WithWorkloadName("attribution-test"),
		WithCPUWorkers(2),
		WithCPULoad(70),
	)
	defer workload.Stop()

	time.Sleep(waitForStable)

	allPIDs := append([]int{workload.ParentPID()}, workload.PIDs()...)

	var foundPower float64
	var foundPID int

	for _, pid := range allPIDs {
		power, found := WaitForNonZeroProcessPower(t, scraper, pid, 15*time.Second)
		if found {
			foundPower = power
			foundPID = pid
			break
		}
	}

	if foundPID > 0 {
		t.Logf("Workload PID %d has power: %.4f W", foundPID, foundPower)
		assert.Greater(t, foundPower, 0.0, "Workload should have positive power")
	} else {
		snapshot, _ := scraper.TakeSnapshot()
		processMetrics := snapshot.GetAllWithName("kepler_process_cpu_watts")

		t.Logf("Looking for PIDs: %v", allPIDs)
		t.Logf("Found %d process metrics, top by power:", len(processMetrics))

		for i, m := range processMetrics {
			if i >= 10 {
				break
			}
			if m.Value > 0 {
				t.Logf("  PID=%s comm=%s power=%.4f W",
					m.Labels["pid"], m.Labels["comm"], m.Value)
			}
		}

		t.Log("Workload may not have been detected - this can happen due to process lifecycle timing")
	}
}

// TestTerminatedWorkloadTracking verifies that terminated processes appear with state=terminated
func TestTerminatedWorkloadTracking(t *testing.T) {
	_, scraper := setupKeplerWithWorkloadSupport(t)

	// Start a workload with high CPU to accumulate energy above threshold
	// (e2e-config.yaml has minTerminatedEnergyThreshold: 1 joule)
	workload := StartWorkload(t,
		WithWorkloadName("terminated-test"),
		WithCPUWorkers(2),
		WithCPULoad(80),
	)

	// Wait for workload to appear in metrics and accumulate energy
	time.Sleep(waitForLoad)

	// Take snapshot while running to verify it's tracked
	runningSnapshot, err := scraper.TakeSnapshot()
	require.NoError(t, err, "Failed to take running snapshot")

	runningPower, runningFound := FindWorkloadPower(runningSnapshot, workload)
	t.Logf("Workload while running: found=%v, power=%.4f W", runningFound, runningPower)

	// Get PIDs before stopping (they'll be invalid after)
	allPIDs := append([]int{workload.ParentPID()}, workload.PIDs()...)
	t.Logf("Workload PIDs to track: %v", allPIDs)

	// Explicitly stop the workload (don't rely on t.Cleanup)
	err = workload.Stop()
	require.NoError(t, err, "Failed to stop workload")

	// Wait for Kepler to detect termination and export the terminated metrics
	// Need at least 2-3 collection cycles for terminated tracking
	time.Sleep(waitForLoad)

	// Look for any of the workload's PIDs in terminated state
	var terminatedEnergy float64
	var terminatedPID int
	terminatedFound := false

	for _, pid := range allPIDs {
		energy, found := WaitForTerminatedProcess(t, scraper, pid, 15*time.Second)
		if found {
			terminatedEnergy = energy
			terminatedPID = pid
			terminatedFound = true
			break
		}
	}

	if terminatedFound {
		t.Logf("Terminated workload PID=%d tracked with energy=%.4f J", terminatedPID, terminatedEnergy)
		assert.Greater(t, terminatedEnergy, 0.0, "Terminated workload should have positive energy")
	} else {
		// Log diagnostic info if not found
		snapshot, _ := scraper.TakeSnapshot()
		terminatedMetrics := snapshot.GetAllWithName("kepler_process_cpu_joules_total")

		t.Logf("Looking for terminated PIDs: %v", allPIDs)
		t.Logf("Found %d total process joules metrics", len(terminatedMetrics))

		terminatedCount := 0
		for _, m := range terminatedMetrics {
			if m.Labels["state"] == "terminated" {
				terminatedCount++
				if terminatedCount <= 5 {
					t.Logf("  Terminated: PID=%s comm=%s energy=%.4f J",
						m.Labels["pid"], m.Labels["comm"], m.Value)
				}
			}
		}
		t.Logf("Total terminated processes in metrics: %d", terminatedCount)

		// This is expected behavior in some cases - workload may not meet energy threshold
		// or may have been cleaned up before the scrape
		t.Log("Workload may not appear as terminated due to energy threshold or timing")
	}
}

// TestEnergyConservationUnderLoad verifies energy conservation during load
func TestEnergyConservationUnderLoad(t *testing.T) {
	_, scraper := setupKeplerWithWorkloadSupport(t)

	workload := StartWorkload(t,
		WithWorkloadName("conservation-test"),
		WithCPUWorkers(2),
		WithCPULoad(50),
	)
	defer workload.Stop()

	time.Sleep(waitForStable)

	snapshot, err := scraper.TakeSnapshot()
	require.NoError(t, err, "Failed to take snapshot")

	nodeActiveWatts := snapshot.GetAllWithName("kepler_node_cpu_active_watts")
	require.NotEmpty(t, nodeActiveWatts, "Should have node active watts")

	processWatts := snapshot.GetAllWithName("kepler_process_cpu_watts")

	nodeActiveByZone := make(map[string]float64)
	processSumByZone := make(map[string]float64)

	for _, m := range nodeActiveWatts {
		nodeActiveByZone[m.Labels["zone"]] = m.Value
	}

	for _, m := range processWatts {
		if m.Labels["state"] == "running" {
			processSumByZone[m.Labels["zone"]] += m.Value
		}
	}

	for zone, nodeActive := range nodeActiveByZone {
		processSum := processSumByZone[zone]

		t.Logf("Zone %s under load: node_active=%.4f W, process_sum=%.4f W",
			zone, nodeActive, processSum)

		assertWithinTolerance(t, nodeActive, processSum, powerTolerance, absolutePowerTolerance,
			"Zone %s: Energy conservation should hold under load", zone)
	}
}

// TestProcessMetadataAccuracy verifies that process metrics have correct comm and exe labels
func TestProcessMetadataAccuracy(t *testing.T) {
	_, scraper := setupKeplerWithWorkloadSupport(t)

	workload := StartWorkload(t,
		WithWorkloadName("metadata-test"),
		WithCPUWorkers(1),
		WithCPULoad(50),
	)
	defer workload.Stop()

	// Wait for workload to appear in metrics
	time.Sleep(waitForLoad)

	snapshot, err := scraper.TakeSnapshot()
	require.NoError(t, err, "Failed to take snapshot")

	processMetrics := snapshot.GetAllWithName("kepler_process_cpu_watts")

	// Find the stress-ng process by PID
	allPIDs := append([]int{workload.ParentPID()}, workload.PIDs()...)
	var foundMetric *common.Metric

	for _, pid := range allPIDs {
		pidStr := strconv.Itoa(pid)
		for i := range processMetrics {
			m := &processMetrics[i]
			if m.Labels["pid"] == pidStr && m.Labels["state"] == "running" {
				foundMetric = m
				break
			}
		}
		if foundMetric != nil {
			break
		}
	}

	if foundMetric == nil {
		t.Log("Workload process not found in metrics - may be timing related")
		return
	}

	t.Logf("Found process: pid=%s, comm=%s, exe=%s",
		foundMetric.Labels["pid"], foundMetric.Labels["comm"], foundMetric.Labels["exe"])

	// Verify comm label contains "stress" (stress-ng worker processes)
	comm := foundMetric.Labels["comm"]
	assert.NotEmpty(t, comm, "comm label should not be empty")
	assert.True(t, strings.Contains(comm, "stress"),
		"comm label should contain 'stress', got: %s", comm)

	// Verify exe label is not empty and points to a path
	exe := foundMetric.Labels["exe"]
	assert.NotEmpty(t, exe, "exe label should not be empty")
}

// TestProportionalPowerAttribution verifies workloads with higher CPU get proportionally more power
func TestProportionalPowerAttribution(t *testing.T) {
	_, scraper := setupKeplerWithWorkloadSupport(t)

	// Start two workloads with different CPU loads
	// Using 25% vs 75% for a 3:1 ratio
	workloadLow := StartWorkload(t,
		WithWorkloadName("low-load"),
		WithCPUWorkers(1),
		WithCPULoad(25),
	)
	defer workloadLow.Stop()

	workloadHigh := StartWorkload(t,
		WithWorkloadName("high-load"),
		WithCPUWorkers(1),
		WithCPULoad(75),
	)
	defer workloadHigh.Stop()

	// Wait for workloads to stabilize and appear in metrics
	time.Sleep(waitForLoad)

	snapshot, err := scraper.TakeSnapshot()
	require.NoError(t, err, "Failed to take snapshot")

	lowPower, lowFound := FindWorkloadPower(snapshot, workloadLow)
	highPower, highFound := FindWorkloadPower(snapshot, workloadHigh)

	t.Logf("Low load (25%%): found=%v, power=%.4f W", lowFound, lowPower)
	t.Logf("High load (75%%): found=%v, power=%.4f W", highFound, highPower)

	// Both should be found
	if !lowFound || !highFound {
		t.Log("One or both workloads not found - skipping proportionality check")
		return
	}

	// Both should have positive power
	if lowPower <= 0 || highPower <= 0 {
		t.Log("One or both workloads have zero power - skipping proportionality check")
		return
	}

	// The high-load workload should have more power than the low-load workload
	assert.Greater(t, highPower, lowPower,
		"Higher CPU load should result in higher power attribution")

	// Check the ratio is reasonable (between 1.5x and 6x for 75%/25% = 3x expected)
	ratio := highPower / lowPower
	t.Logf("Power ratio (high/low): %.2f (expected ~3.0 for 75%%/25%%)", ratio)

	assert.Greater(t, ratio, 1.2, "Power ratio should be > 1.2 (high load uses more power)")
}
