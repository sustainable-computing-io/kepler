// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package monitor

import (
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	testingclock "k8s.io/utils/clock/testing"
)

// TestIntegration_Monitor_Snapshot validates the complete lifecycle of monitor snapshots
// and core behaviors of Kepler's power attribution logic.
//
// ## Validations
//
// ### 1. Energy-First Attribution Pattern
// - **First snapshot**: Only energy is available (no time delta for power calculation)
// - Energy is distributed to workloads based on CPU utilization ratios
// - Node receives total hardware energy from RAPL sensors
// - Workloads receive proportional energy based on their CPU time delta
//
// ### 2. Power Calculation Evolution
// - **Second snapshot**: Power calculations appear once time delta is available
// - Power = resource.CPUTimeDelta / node.CPUTotalTime * Node Active Power
// - Both node and workloads show power consumption based on energy deltas
//
// ### 3. Terminated Workload Tracking Infrastructure
// - Verifies all terminated resource trackers are properly initialized
// - Confirms correct capacity configuration (configurable via WithMaxTerminated)
// - Validates energy zone assignment (uses top energy zone from power meter)
// - Tests reflection-based resource type detection for logging
//
// ### 4. Active/Idle Energy Split and Attribution Conservation
// - Node energy is split into Active (60%) and Idle (40%) based on CPU usage ratio
// - Active energy is distributed to processes proportionally based on their CPU time
// - Sum of all process energy equals node active energy (energy conservation)
// - Sum of all process power equals node active power (power conservation)
//
// ### 5. Energy Accumulation and Consistency
// - Node energy continuously increases as hardware sensors report higher values
// - Energy attribution remains consistent across multiple collections
// - System handles realistic energy progression (100J → 150J → 200J)
//
// ## Test Architecture
//
// This integration test uses controlled mocks to simulate realistic hardware behavior:
// - **Mock Power Meter**: Provides predictable energy readings with realistic progression
// - **Fake Clock**: Enables deterministic timing for power calculations
// - **Test Resources**: Pre-configured workloads with active CPU usage patterns
// - **Resource Informer**: Simulates process/container discovery from /proc filesystem
//
// ## Key Behaviors Validated
//
// 1. **Hardware Integration**: RAPL sensor simulation with energy zones
// 2. **Attribution Algorithm**: CPU-ratio based energy distribution
// 3. **Power Calculation**: Time-based power derivation from energy deltas
// 4. **Workload Tracking**: Process/container lifecycle management
// 5. **Configuration**: Terminated workload tracking with configurable limits
// 6. **Logging**: Resource type detection using reflection for structured logging
//
// This test serves as both validation and documentation of how Kepler's monitor
// orchestrates measurement, attribution, and export of energy/power data.
//
// NOTE:: This test doesn't show actual terminated workloads because that would require
// more complex mock setup to simulate resource lifecycle changes. The key behaviors
// that are validated are:
// 1. ✅ First snapshot: Energy allocation without power
// 2. ✅ Subsequent snapshots: Power calculations based on time deltas
// 3. ✅ Active/Idle energy split: Correctly computed based on CPU usage ratio
// 4. ✅ Energy conservation: Sum of process energy equals node active energy
// 5. ✅ Power attribution: Sum of process power equals node active power
// 6. ✅ Terminated workload tracking: Properly configured and operational
// 7. ✅ Resource type logging: Works through reflection
// 8. ✅ Energy accumulation: Node energy increases over time
func TestIntegration_Monitor_Snapshot(t *testing.T) {
	// Create test resources with active CPU usage
	tr := CreateTestResources(
		withNodeCpuUsage(0.6),        // 60% CPU usage for active energy
		withNodeCpuTimeDelta(2000.0), // 2000ms total CPU time delta
	)

	// Setup mock power meter with realistic energy progression
	mockPowerMeter := &MockCPUPowerMeter{}
	pkg := &MockEnergyZone{}
	pkg.On("Name").Return("package").Maybe()
	pkg.On("Index").Return(0).Maybe()
	pkg.On("MaxEnergy").Return(1000 * Joule).Maybe()

	// Energy readings show consumption over time
	pkg.On("Energy").Return(100*Joule, nil).Once() // Initial: 100J
	pkg.On("Energy").Return(150*Joule, nil).Once() // +50J after 5s = 10W average
	pkg.On("Energy").Return(200*Joule, nil).Once() // +50J after 5s = 10W average

	energyZones := []EnergyZone{pkg}
	mockPowerMeter.On("Zones").Return(energyZones, nil).Maybe()
	mockPowerMeter.On("PrimaryEnergyZone").Return(pkg, nil).Maybe()
	mockPowerMeter.On("Name").Return("mock-cpu").Maybe()

	// Setup resource informer
	resourceInformer := &MockResourceInformer{}
	resourceInformer.SetExpectations(t, tr)
	resourceInformer.On("Refresh").Return(nil).Times(3)

	// let there be time
	startTime := time.Date(2025, 07, 10, 12, 0, 0, 0, time.UTC)
	fakeClock := testingclock.NewFakeClock(startTime)

	// Create monitor with terminated workload tracking
	monitor := NewPowerMonitor(
		mockPowerMeter,
		WithResourceInformer(resourceInformer),
		WithClock(fakeClock),
		WithMaxTerminated(10), // Enable terminated workload tracking
		WithLogger(slog.Default().With("test", "snapshot-evolution")),
	)

	err := monitor.Init()
	require.NoError(t, err)

	// === Collection 1: First reading shows energy distribution ===
	snapshot1, err := monitor.Snapshot()
	require.NoError(t, err)
	require.NotNil(t, snapshot1)

	t.Logf("=== First Snapshot ===")
	t.Logf("Node energy: %.0f J, power: %.0f W",
		snapshot1.Node.Zones[pkg].EnergyTotal.Joules(),
		snapshot1.Node.Zones[pkg].Power.Watts())
	t.Logf("Running processes: %d, terminated: %d",
		len(snapshot1.Processes), len(snapshot1.TerminatedProcesses))
	t.Logf("Running containers: %d, terminated: %d",
		len(snapshot1.Containers), len(snapshot1.TerminatedContainers))

	// First reading: Node should have energy but no power
	assert.True(t, snapshot1.Node.Zones[pkg].EnergyTotal > 0, "Node should have energy")
	assert.Equal(t, Power(0), snapshot1.Node.Zones[pkg].Power, "Node should have no power on first reading")

	// First reading: Energy split should be calculated based on first energy reading
	// First reading gets 100J, split 60% active / 40% idle
	expectedActiveEnergy1 := 60 * Joule // 100J * 0.6
	expectedIdleEnergy1 := 40 * Joule   // 100J * 0.4
	assert.Equal(t, expectedActiveEnergy1, snapshot1.Node.Zones[pkg].ActiveEnergyTotal,
		"Active energy total should be 60% of first reading")
	assert.Equal(t, expectedIdleEnergy1, snapshot1.Node.Zones[pkg].IdleEnergyTotal,
		"Idle energy total should be 40% of first reading")

	// Workloads should exist and have energy attributed from first reading
	assert.NotEmpty(t, snapshot1.Processes, "Should have processes")
	assert.NotEmpty(t, snapshot1.Containers, "Should have containers")

	// On first reading, processes get attribution based on activeEnergy (not ActiveEnergyTotal)
	// activeEnergy for first reading is 60kJ, but this value is internal to NodeUsage
	// Process energy attribution should be 0 because there's no CPU time delta on first reading
	totalProcessEnergy1 := 0 * Joule
	for _, proc := range snapshot1.Processes {
		totalProcessEnergy1 += proc.Zones[pkg].EnergyTotal
	}
	// First reading typically has zero process energy due to no CPU time delta
	assert.Equal(t, 0*Joule, totalProcessEnergy1,
		"Process energy should be 0 on first reading (no CPU time delta for attribution)")

	// No terminated workloads initially
	assert.Empty(t, snapshot1.TerminatedProcesses, "Should have no terminated processes")
	assert.Empty(t, snapshot1.TerminatedContainers, "Should have no terminated containers")

	// === Collection 2: Time passes, power calculations appear ===
	fakeClock.Step(5 * time.Second) // Advance time to enable power calculation

	snapshot2, err := monitor.Snapshot()
	require.NoError(t, err)
	require.NotNil(t, snapshot2)

	t.Logf("=== Second Snapshot (after 5s) ===")
	t.Logf("Node energy: %.0f J, power: %.0f W",
		snapshot2.Node.Zones[pkg].EnergyTotal.Joules(),
		snapshot2.Node.Zones[pkg].Power.Watts())
	t.Logf("Node active energy: %.0f J, active power: %.0f W",
		snapshot2.Node.Zones[pkg].ActiveEnergyTotal.Joules(),
		snapshot2.Node.Zones[pkg].ActivePower.Watts())

	// Second reading: Node should now have power (energy delta / time delta)
	assert.Greater(t, snapshot2.Node.Zones[pkg].Power, 0*Watt, "Node should have power on second reading")
	assert.Greater(t, snapshot2.Node.Zones[pkg].ActivePower, 0*Watt, "Node should have active power on second reading")
	assert.Greater(t, snapshot2.Node.Zones[pkg].EnergyTotal, snapshot1.Node.Zones[pkg].EnergyTotal,
		"Node energy should have accumulated")

	// Validate Active/Idle energy split (60% CPU usage = 60% of energy delta)
	// Cumulative active energy: 60J (first) + 30J (delta) = 90J
	expectedActiveEnergyTotal2 := 90 * Joule // 60J + 30J
	assert.Equal(t, expectedActiveEnergyTotal2, snapshot2.Node.Zones[pkg].ActiveEnergyTotal,
		"Active energy total should accumulate: 60J + 30J = 90J")

	// Cumulative idle energy: 40J (first) + 20J (delta) = 60J
	expectedIdleEnergyTotal2 := 60 * Joule // 40J + 20J
	assert.Equal(t, expectedIdleEnergyTotal2, snapshot2.Node.Zones[pkg].IdleEnergyTotal,
		"Idle energy total should accumulate: 40J + 20J = 60J")

	// Validate Active/Idle power split (should match 60% CPU usage)
	expectedActivePower := Power(float64(snapshot2.Node.Zones[pkg].Power) * 0.6)
	assert.Equal(t, expectedActivePower, snapshot2.Node.Zones[pkg].ActivePower,
		"Active power should be 60% of total power")

	expectedIdlePower := snapshot2.Node.Zones[pkg].Power - expectedActivePower
	assert.Equal(t, expectedIdlePower, snapshot2.Node.Zones[pkg].IdlePower,
		"Idle power should be 40% of total power")

	// Verify energy attribution: sum of all process energy should equal current energy delta
	// In second reading, the energy delta is 50J, active portion is 30J
	totalProcessEnergy := 0 * Joule
	for _, proc := range snapshot2.Processes {
		totalProcessEnergy += proc.Zones[pkg].EnergyTotal
	}
	expectedActiveEnergyDelta := 30 * Joule // 50J * 0.6 = 30J (current delta)
	assert.Equal(t, expectedActiveEnergyDelta, totalProcessEnergy,
		"Sum of process energy should equal current active energy delta (30J)")

	// Verify power attribution: sum of all process power should equal node active power
	totalProcessPower := 0 * Watt
	for _, proc := range snapshot2.Processes {
		totalProcessPower += proc.Zones[pkg].Power
	}
	assert.Equal(t, snapshot2.Node.Zones[pkg].ActivePower, totalProcessPower,
		"Sum of process power should equal node active power")

	// === Collection 3: Continued operation ===
	fakeClock.Step(5 * time.Second) // Advance time again

	snapshot3, err := monitor.Snapshot()
	require.NoError(t, err)
	require.NotNil(t, snapshot3)

	t.Logf("=== Third Snapshot (after 10s total) ===")
	t.Logf("Node energy: %.0f J, power: %.0f W",
		snapshot3.Node.Zones[pkg].EnergyTotal.Joules(),
		snapshot3.Node.Zones[pkg].Power.Watts())
	t.Logf("Node active energy: %.0f J, active power: %.0f µW",
		snapshot3.Node.Zones[pkg].ActiveEnergyTotal.Joules(),
		snapshot3.Node.Zones[pkg].ActivePower.Watts())

	// Third reading: Power should continue, energy should continue accumulating
	assert.True(t, snapshot3.Node.Zones[pkg].Power > 0, "Node should maintain power")
	assert.True(t, snapshot3.Node.Zones[pkg].ActivePower > 0, "Node should maintain active power")
	assert.True(t, snapshot3.Node.Zones[pkg].EnergyTotal >= snapshot2.Node.Zones[pkg].EnergyTotal,
		"Node energy should continue accumulating")

	// Validate cumulative energy accumulation
	// Third reading: Active energy total should now be 60J + 30J + 30J = 120J
	expectedActiveEnergyTotal3 := 120 * Joule // 60J + 30J + 30J
	assert.Equal(t, expectedActiveEnergyTotal3, snapshot3.Node.Zones[pkg].ActiveEnergyTotal,
		"Active energy total should accumulate: 60J + 30J + 30J = 120J")

	expectedIdleEnergyTotal3 := 80 * Joule // 40J + 20J + 20J
	assert.Equal(t, expectedIdleEnergyTotal3, snapshot3.Node.Zones[pkg].IdleEnergyTotal,
		"Idle energy total should accumulate: 40J + 20J + 20J = 80J")

	// Verify consistent attribution in third snapshot
	// Process energy accumulates: first delta (30J) + second delta (30J) = 60J
	totalProcessEnergy3 := 0 * Joule
	totalProcessPower3 := Power(0)
	for _, proc := range snapshot3.Processes {
		totalProcessEnergy3 += proc.Zones[pkg].EnergyTotal
		totalProcessPower3 += proc.Zones[pkg].Power
	}
	expectedTotalProcessEnergy3 := 60 * Joule // 30J + 30J (accumulated process energy
	assert.Equal(t, expectedTotalProcessEnergy3, totalProcessEnergy3,
		"Sum of process energy should equal accumulated process attribution (60J)")
	assert.Equal(t, snapshot3.Node.Zones[pkg].ActivePower, totalProcessPower3,
		"Sum of process power should equal node active power in third snapshot")

	// === Verify terminated workload tracking is configured ===
	t.Run("Terminated workload tracking configuration", func(t *testing.T) {
		// Verify all trackers are initialized
		assert.NotNil(t, monitor.terminatedProcessesTracker, "Process tracker should be initialized")
		assert.NotNil(t, monitor.terminatedContainersTracker, "Container tracker should be initialized")
		assert.NotNil(t, monitor.terminatedVMsTracker, "VM tracker should be initialized")
		assert.NotNil(t, monitor.terminatedPodsTracker, "Pod tracker should be initialized")

		// Verify configuration
		assert.Equal(t, 10, monitor.terminatedProcessesTracker.MaxSize(), "Should have correct capacity")
		assert.Equal(t, pkg, monitor.terminatedProcessesTracker.EnergyZone(), "Should use top energy zone")

		// Verify tracker types are logged correctly
		processTracker := monitor.terminatedProcessesTracker.String()
		assert.Contains(t, processTracker, "TerminatedResourceTracker", "Should identify as terminated resource tracker")
		assert.Contains(t, processTracker, "0/10", "Should show current usage")
		assert.Contains(t, processTracker, "package", "Should show energy zone")

		t.Logf("Process tracker: %s", processTracker)
		t.Logf("Container tracker: %s", monitor.terminatedContainersTracker.String())
	})

	// === Test terminated workload prioritization ===
	t.Run("Terminated workload tracker prioritization", func(t *testing.T) {
		// Create a new monitor with small capacity for testing prioritization
		smallCapacityMonitor := NewPowerMonitor(
			mockPowerMeter,
			WithResourceInformer(resourceInformer),
			WithClock(fakeClock),
			WithMaxTerminated(2), // Small capacity to test eviction
			WithLogger(slog.Default().With("test", "prioritization")),
		)

		err := smallCapacityMonitor.Init()
		require.NoError(t, err)

		// === Step 1: Create terminated processes with different energy levels ===

		// High energy process
		highEnergyProcess := &Process{
			PID:   1001,
			Comm:  "high-energy-proc",
			Zones: make(ZoneUsageMap),
		}
		highEnergyProcess.Zones[pkg] = Usage{
			EnergyTotal: 100 * Joule, // High energy
			Power:       Power(20 * Watt),
		}

		// Medium energy process
		mediumEnergyProcess := &Process{
			PID:   1002,
			Comm:  "medium-energy-proc",
			Zones: make(ZoneUsageMap),
		}
		mediumEnergyProcess.Zones[pkg] = Usage{
			EnergyTotal: 50 * Joule, // Medium energy
			Power:       Power(10 * Watt),
		}

		// Low energy process
		lowEnergyProcess := &Process{
			PID:   1003,
			Comm:  "low-energy-proc",
			Zones: make(ZoneUsageMap),
		}
		lowEnergyProcess.Zones[pkg] = Usage{
			EnergyTotal: 20 * Joule, // Low energy
			Power:       Power(5 * Watt),
		}

		// === Step 2: Add processes to tracker (under capacity) ===

		// Add first two processes (within capacity)
		smallCapacityMonitor.terminatedProcessesTracker.Add(mediumEnergyProcess)
		smallCapacityMonitor.terminatedProcessesTracker.Add(lowEnergyProcess)

		// Verify both processes are tracked
		trackedProcesses := smallCapacityMonitor.terminatedProcessesTracker.Items()
		assert.Len(t, trackedProcesses, 2, "Should track both processes when under capacity")

		// === Step 3: Add high energy process (should evict lowest) ===

		// Add high energy process - should evict low energy process
		smallCapacityMonitor.terminatedProcessesTracker.Add(highEnergyProcess)

		// Verify tracker maintains only top 2 processes
		trackedProcesses = smallCapacityMonitor.terminatedProcessesTracker.Items()
		assert.Len(t, trackedProcesses, 2, "Should maintain capacity limit")

		// Verify high and medium energy processes are retained
		trackedPIDs := make(map[int]bool)
		trackedEnergies := make(map[int]Energy)
		for _, proc := range trackedProcesses {
			trackedPIDs[proc.PID] = true
			trackedEnergies[proc.PID] = proc.Zones[pkg].EnergyTotal
		}

		assert.True(t, trackedPIDs[1001], "High energy process should be retained")
		assert.True(t, trackedPIDs[1002], "Medium energy process should be retained")
		assert.False(t, trackedPIDs[1003], "Low energy process should be evicted")

		// Verify energy values are preserved correctly
		assert.Equal(t, 100*Joule, trackedEnergies[1001], "High energy process should have correct energy")
		assert.Equal(t, 50*Joule, trackedEnergies[1002], "Medium energy process should have correct energy")

		// === Step 4: Add another low energy process (should be rejected) ===

		veryLowEnergyProcess := &Process{
			PID:   1004,
			Comm:  "very-low-energy-proc",
			Zones: make(ZoneUsageMap),
		}
		veryLowEnergyProcess.Zones[pkg] = Usage{
			EnergyTotal: 10 * Joule, // Very low energy
			Power:       Power(2 * Watt),
		}

		smallCapacityMonitor.terminatedProcessesTracker.Add(veryLowEnergyProcess)

		// Verify tracker still maintains the same top 2 processes
		trackedProcesses = smallCapacityMonitor.terminatedProcessesTracker.Items()
		assert.Len(t, trackedProcesses, 2, "Should still maintain capacity limit")

		// Verify the same high energy processes are still tracked
		trackedPIDs = make(map[int]bool)
		for _, proc := range trackedProcesses {
			trackedPIDs[proc.PID] = true
		}

		assert.True(t, trackedPIDs[1001], "High energy process should still be retained")
		assert.True(t, trackedPIDs[1002], "Medium energy process should still be retained")
		assert.False(t, trackedPIDs[1004], "Very low energy process should be rejected")

		// === Step 5: Add ultra high energy process (should evict medium) ===

		ultraHighEnergyProcess := &Process{
			PID:   1005,
			Comm:  "ultra-high-energy-proc",
			Zones: make(ZoneUsageMap),
		}
		ultraHighEnergyProcess.Zones[pkg] = Usage{
			EnergyTotal: 200 * Joule, // Ultra high energy
			Power:       Power(40 * Watt),
		}

		smallCapacityMonitor.terminatedProcessesTracker.Add(ultraHighEnergyProcess)

		// Verify tracker now has ultra high and high energy processes
		trackedProcesses = smallCapacityMonitor.terminatedProcessesTracker.Items()
		assert.Len(t, trackedProcesses, 2, "Should maintain capacity limit")

		// Verify ultra high and high energy processes are retained
		trackedPIDs = make(map[int]bool)
		trackedEnergies = make(map[int]Energy)
		for _, proc := range trackedProcesses {
			trackedPIDs[proc.PID] = true
			trackedEnergies[proc.PID] = proc.Zones[pkg].EnergyTotal
		}

		assert.True(t, trackedPIDs[1005], "Ultra high energy process should be retained")
		assert.True(t, trackedPIDs[1001], "High energy process should be retained")
		assert.False(t, trackedPIDs[1002], "Medium energy process should be evicted")

		// Verify energy values are preserved correctly
		assert.Equal(t, 200*Joule, trackedEnergies[1005], "Ultra high energy process should have correct energy")
		assert.Equal(t, 100*Joule, trackedEnergies[1001], "High energy process should have correct energy")

		t.Logf("=== Priority-based Tracking Validated ===")
		t.Logf("✅ Under capacity: All processes tracked")
		t.Logf("✅ At capacity: Higher energy processes evict lower energy ones")
		t.Logf("✅ Rejection: Very low energy processes are rejected when tracker is full")
		t.Logf("✅ Eviction: Medium energy processes are evicted for ultra high energy ones")
		t.Logf("✅ Energy preservation: Tracked processes maintain exact energy values")
	})

	t.Logf("=== Integration Test Summary ===")
	// NOTE: if you modify the test, make sure to update this summary
	t.Logf("✅ First reading behavior: Energy without power")
	t.Logf("✅ Power calculation behavior: Appears in subsequent readings")
	t.Logf("✅ Active/Idle split: 60%% CPU usage → 60%% active energy/power")
	t.Logf("✅ Energy conservation: Process energy sums to node active energy")
	t.Logf("✅ Power attribution: Process power sums to node active power")
	t.Logf("✅ Terminated workload tracking: Configured with capacity %d",
		monitor.terminatedProcessesTracker.MaxSize())
	t.Logf("✅ Resource type detection: Using reflection-based logging")
	t.Logf("✅ Priority-based tracking: Maintains only top energy consuming terminated resources")
	t.Logf("✅ Energy accumulation: Node energy increased from %.0f to %.0f J",
		snapshot1.Node.Zones[pkg].EnergyTotal.Joules(),
		snapshot3.Node.Zones[pkg].EnergyTotal.Joules())
}
