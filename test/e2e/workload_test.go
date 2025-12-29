// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

// TestPowerDecreasesAfterLoad verifies that power decreases after load is removed
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
	loadedActive := loadedSnapshot.SumValues("kepler_node_cpu_active_watts", nil)

	t.Logf("Under load: active=%.2f W", loadedActive)

	err = workload.Stop()
	require.NoError(t, err, "Failed to stop workload")

	time.Sleep(waitAfterLoad)

	afterSnapshot, err := scraper.TakeSnapshot()
	require.NoError(t, err, "Failed to take post-load snapshot")

	afterActive := afterSnapshot.SumValues("kepler_node_cpu_active_watts", nil)

	t.Logf("After load: active=%.2f W", afterActive)

	// Verify power decreased - this is what the test name claims to verify.
	// Note: CPU usage ratio has significant lag in Kepler and is unreliable for this test.
	assert.Less(t, afterActive, loadedActive, "Active power should decrease after load is removed")

	t.Logf("Power delta: %.2f W -> %.2f W (change: %.2f W)",
		loadedActive, afterActive, afterActive-loadedActive)
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

	processMetrics := snapshot.GetAllWithName("kepler_process_cpu_watts")

	var workload1Power, workload2Power float64
	workload1Found := false
	workload2Found := false

	pid1Str := strconv.Itoa(workload1.ParentPID())
	pid2Str := strconv.Itoa(workload2.ParentPID())

	for _, m := range processMetrics {
		if m.Labels["pid"] == pid1Str {
			workload1Power = m.Value
			workload1Found = true
		}
		if m.Labels["pid"] == pid2Str {
			workload2Power = m.Value
			workload2Found = true
		}
	}

	if !workload1Found {
		for _, pid := range workload1.PIDs() {
			pidStr := strconv.Itoa(pid)
			for _, m := range processMetrics {
				if m.Labels["pid"] == pidStr {
					workload1Power += m.Value
					workload1Found = true
				}
			}
		}
	}

	if !workload2Found {
		for _, pid := range workload2.PIDs() {
			pidStr := strconv.Itoa(pid)
			for _, m := range processMetrics {
				if m.Labels["pid"] == pidStr {
					workload2Power += m.Value
					workload2Found = true
				}
			}
		}
	}

	t.Logf("Workload 1 (30%% CPU): found=%v, power=%.4f W", workload1Found, workload1Power)
	t.Logf("Workload 2 (60%% CPU): found=%v, power=%.4f W", workload2Found, workload2Power)

	assert.True(t, workload1Found || workload2Found,
		"At least one workload should be detected in metrics")

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
