// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package monitor

import (
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/sustainable-computing-io/kepler/internal/device"
	"github.com/sustainable-computing-io/kepler/internal/resource"

	test_clock "k8s.io/utils/clock/testing"
)

type (
	MockRaplZone = device.MockRaplZone
)

// TestNodePowerCollection tests the PowerMonitor.collectNodePower method
func TestNodePowerCollection(t *testing.T) {
	// Create a logger that writes to nowhere for testing
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	// Create test zones
	pkg := device.NewMockRaplZone(
		"package-0",
		0, "/sys/class/powercap/intel-rapl/intel-rapl:0", 200*Joule)

	core := device.NewMockRaplZone(
		"core-0", 0, "/sys/class/powercap/intel-rapl/intel-rapl:0/intel-rapl:0:0", 150*Joule)

	testZones := []EnergyZone{pkg, core}
	mockCPUPowerMeter := &MockCPUPowerMeter{}

	// Create test scenario
	t.Run("Basic Node Power Collection", func(t *testing.T) {
		startTime := time.Date(2025, 4, 14, 5, 40, 0, 0, time.UTC)
		mockClock := test_clock.NewFakeClock(startTime)

		mockCPUPowerMeter.On("Zones").Return(testZones, nil)

		// Create mock resource informer
		mockResourceInformer := &MockResourceInformer{}
		mockNode := &resource.Node{
			CPUUsageRatio:            0.5,
			ProcessTotalCPUTimeDelta: 100.0,
		}
		mockResourceInformer.On("Node").Return(mockNode)

		// Create a custom PowerMonitor with the mock readers
		pm := NewPowerMonitor(
			mockCPUPowerMeter,
			WithLogger(logger),
			WithClock(mockClock),
			WithResourceInformer(mockResourceInformer))
		assert.NotNil(t, pm)

		// First collection should store the initial values
		t.Run("First Collection", func(t *testing.T) {
			pkg.Inc(20 * Joule)
			core.Inc(10 * Joule)

			// Create PowerData instance
			current := NewSnapshot()

			// Collect node power data
			err := pm.firstNodeRead(current.Node)
			assert.NoError(t, err)

			// Verify mock expectations
			mockCPUPowerMeter.AssertExpectations(t)

			// Check that both zones have data
			assert.Contains(t, current.Node.Zones, pkg)
			assert.Contains(t, current.Node.Zones, core)

			// Check package zone values
			pkgZone := current.Node.Zones[pkg]
			// should equal what package zone returns
			raplPkgEnergy, _ := pkg.Energy()
			assert.Equal(t, raplPkgEnergy.MicroJoules(), pkgZone.EnergyTotal.MicroJoules())
			assert.Equal(t, Power(0), pkgZone.Power) // Should be 0 for first reading

			// Check core zone values
			coreZone := current.Node.Zones[core]
			raplCoreEnergy, _ := core.Energy()
			assert.Equal(t, raplCoreEnergy.MicroJoules(), coreZone.EnergyTotal.MicroJoules())
			assert.Equal(t, Power(0), coreZone.Power)

			pm.snapshot.Store(current)
		})

		// Clear existing mocks set up updated values

		t.Run("Second Collection", func(t *testing.T) {
			// Advance clock by 1 second
			mockClock.Step(1 * time.Second)

			mockCPUPowerMeter.ExpectedCalls = nil

			pkg.Inc(50 * Joule)  // 20 -> 25
			core.Inc(25 * Joule) // 10 -> 12.5
			mockCPUPowerMeter.On("Zones").Return(testZones, nil)

			// Collect node power data again

			prev := pm.snapshot.Load()
			current := NewSnapshot()
			err := pm.calculateNodePower(prev.Node, current.Node)
			assert.NoError(t, err)

			mockCPUPowerMeter.AssertExpectations(t)

			// Check package zone values for second reading
			pkgZone := current.Node.Zones[pkg]
			raplPkgEnergy, _ := pkg.Energy()
			assert.Equal(t, raplPkgEnergy, pkgZone.EnergyTotal) // No difference in Absolute counter
			assert.InDelta(t, 50, pkgZone.Power.Watts(), 0.001) // 50 joules / 1 second = 50 watts

			coreZone := current.Node.Zones[core]
			raplCoreEnergy, _ := core.Energy()
			assert.Equal(t, raplCoreEnergy, coreZone.EnergyTotal) // No difference in Absolute counter
			assert.InDelta(t, 25, coreZone.Power.Watts(), 0.001)  // 25 joules / 1 second = 25 watts

			pm.snapshot.Store(current)
		})

		t.Run("After 3s", func(t *testing.T) {
			// Advance clock by 1 second
			mockClock.Step(3 * time.Second)

			mockCPUPowerMeter.ExpectedCalls = nil

			pkg.Inc(3 * 25 * Joule)
			core.Inc(3 * 15 * Joule)
			mockCPUPowerMeter.On("Zones").Return(testZones, nil)

			// Collect node power data again
			prev := pm.snapshot.Load()
			current := NewSnapshot()
			err := pm.calculateNodePower(prev.Node, current.Node)
			assert.NoError(t, err)

			mockCPUPowerMeter.AssertExpectations(t)

			// Check package zone values for second reading
			pkgZone := current.Node.Zones[pkg]
			raplPkgEnergy, _ := pkg.Energy()
			assert.Equal(t, raplPkgEnergy, pkgZone.EnergyTotal)
			assert.InDelta(t, 25, pkgZone.Power.Watts(), 0.001)

			coreZone := current.Node.Zones[core]
			raplCoreEnergy, _ := core.Energy()
			assert.Equal(t, raplCoreEnergy, coreZone.EnergyTotal)
			assert.InDelta(t, 15, coreZone.Power.Watts(), 0.001)

			pm.snapshot.Store(current)
		})

		t.Run("Counter Wrap Around", func(t *testing.T) {
			pkgE, _ := pkg.Energy()
			assert.Equal(t, uint64(145_000_000), pkgE.MicroJoules())
			assert.Equal(t, float64(145), pkgE.Joules())

			coreE, _ := core.Energy()
			assert.Equal(t, uint64(80_000_000), coreE.MicroJoules())
			assert.Equal(t, float64(80), coreE.Joules())

			mockClock.Step(10 * time.Second)

			mockCPUPowerMeter.ExpectedCalls = nil

			pkg.Inc(10 * 8 * Joule)  // 145 + 80 -> 225 (wraps at 200) -> 25
			core.Inc(10 * 3 * Joule) // 80 + 40 -> 120 (wraps at 100) -> 15
			mockCPUPowerMeter.On("Zones").Return(testZones, nil)

			// Collect node power data again
			prev := pm.snapshot.Load()
			current := NewSnapshot()
			err := pm.calculateNodePower(prev.Node, current.Node)
			assert.NoError(t, err)

			mockCPUPowerMeter.AssertExpectations(t)

			// Check package zone values for second reading
			pkgZone := current.Node.Zones[pkg]
			raplPkgEnergy, _ := pkg.Energy()
			assert.Equal(t, raplPkgEnergy, pkgZone.EnergyTotal)

			assert.InDelta(t, 8, pkgZone.Power.Watts(), 0.001)

			coreZone := current.Node.Zones[core]
			raplCoreEnergy, _ := core.Energy()
			assert.Equal(t, raplCoreEnergy, coreZone.EnergyTotal)
			assert.InDelta(t, 3, coreZone.Power.Watts(), 0.001)

			pm.snapshot.Store(current)
		})

		mockResourceInformer.AssertExpectations(t)
	})
}

// Test error handling scenarios
func TestNodeErrorHandling(t *testing.T) {
	// Create a logger that writes to nowhere for testing
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	pkg := device.NewMockRaplZone(
		"package-0",
		0, "/sys/class/powercap/intel-rapl/intel-rapl:0", 200*Joule)

	core := device.NewMockRaplZone(
		"core-0", 0, "/sys/class/powercap/intel-rapl/intel-rapl:0/intel-rapl:0:0", 150*Joule)

	testZones := []EnergyZone{pkg, core}

	mockCPUPowerMeter := &MockCPUPowerMeter{}

	// Create a mock clock with a fixed start time
	startTime := time.Date(2023, 4, 15, 9, 0, 0, 0, time.UTC)
	mockClock := test_clock.NewFakeClock(startTime)

	// Create mock resource informer
	mockResourceInformer := &MockResourceInformer{}
	mockNode := &resource.Node{
		CPUUsageRatio:            0.5,
		ProcessTotalCPUTimeDelta: 100.0,
	}
	mockResourceInformer.On("Node").Return(mockNode)

	// Create PowerMonitor with the mock
	pm := NewPowerMonitor(
		mockCPUPowerMeter,
		WithLogger(logger),
		WithClock(mockClock),
		WithResourceInformer(mockResourceInformer),
	)

	t.Run("Zone Listing Error", func(t *testing.T) {
		mockCPUPowerMeter.On("Zones").Return([]EnergyZone(nil), assert.AnError)
		current := NewSnapshot()
		err := pm.firstNodeRead(current.Node)
		assert.Error(t, err, "zone read errors must be propagated")

		prev := NewSnapshot()
		err = pm.calculateNodePower(prev.Node, current.Node)
		assert.Error(t, err, "zone read errors must be propagated")

		mockCPUPowerMeter.AssertExpectations(t)
	})

	t.Run("Zone Energy Read Error", func(t *testing.T) {
		mockCPUPowerMeter.ExpectedCalls = nil
		mockCPUPowerMeter.On("Zones").Return(testZones, nil)
		pkg.OnEnergy(0, assert.AnError)
		core.OnEnergy(10, nil)

		current := NewSnapshot()
		err := pm.firstNodeRead(current.Node)
		assert.Error(t, err, "pkg read error must be propagated")

		prev := NewSnapshot()
		err = pm.calculateNodePower(prev.Node, current.Node)
		assert.Error(t, err, "pkg read error must be propagated")
		mockCPUPowerMeter.AssertExpectations(t)

		// Should have zone info for both
		assert.NotContains(t, current.Node.Zones, pkg)
		assert.Contains(t, current.Node.Zones, core)
	})

	mockResourceInformer.AssertExpectations(t)
}

// TestCalculateEnergyDelta tests the CalculateEnergyDelta function directly
func TestCalculateEnergyDelta(t *testing.T) {
	testCases := []struct {
		name      string
		current   Energy
		previous  Energy
		maxJoules Energy
		expected  Energy
	}{{
		name:      "Normal",
		current:   25 * Joule,
		previous:  20 * Joule,
		maxJoules: 100 * Joule,
		expected:  5 * Joule,
	}, {
		name:      "Wrap around",
		current:   10 * Joule,
		previous:  90 * Joule,
		maxJoules: 100 * Joule,
		expected:  20 * Joule, // 100-90 + 10J
	}, {
		name:      "Zero values",
		current:   0 * Joule,
		previous:  0 * Joule,
		maxJoules: 100 * Joule,
		expected:  0 * Joule,
	}, {
		name:      "Max value is zero",
		current:   10 * Joule,
		previous:  20 * Joule,
		maxJoules: 0 * Joule,
		expected:  0 * Joule, // returns 0 if there is no max and there is a wrap
	}, {
		name:      "Negative diff but max is negative",
		current:   2 * Joule,
		previous:  8 * Joule,
		maxJoules: 10 * Joule,
		expected:  4 * Joule, // No wrap correction with negative max
	}, {
		name:      "Current equals max",
		current:   100 * Joule,
		previous:  90 * Joule,
		maxJoules: 100 * Joule,
		expected:  10 * Joule,
	}, {
		name:      "Previous equals max",
		current:   10 * Joule,
		previous:  100 * Joule,
		maxJoules: 100 * Joule,
		expected:  10 * Joule,
	}, {
		name:      "Exact wrap",
		current:   0 * Joule,
		previous:  100 * Joule,
		maxJoules: 100 * Joule,
		expected:  0 * Joule,
	}}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := calculateEnergyDelta(tc.current, tc.previous, tc.maxJoules)
			assert.Equal(t, tc.expected, result, "Diff should match expected value")
		})
	}
}

// TestNodeActiveEnergyCounterBehavior verifies that ActiveEnergy and IdleEnergy represent interval-based energy attribution,
// correctly splitting the delta energy between active and idle portions rather than acting as cumulative counters
func TestNodeActiveEnergyCounterBehavior(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	pkg := device.NewMockRaplZone(
		"package-0",
		0, "/sys/class/powercap/intel-rapl/intel-rapl:0", 1000*Joule)

	testZones := []EnergyZone{pkg}
	mockCPUPowerMeter := &MockCPUPowerMeter{}

	startTime := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	mockClock := test_clock.NewFakeClock(startTime)

	mockResourceInformer := &MockResourceInformer{}
	mockNode := &resource.Node{
		CPUUsageRatio:            0.4, // 40% CPU usage
		ProcessTotalCPUTimeDelta: 80.0,
	}
	mockResourceInformer.On("Node").Return(mockNode)

	pm := NewPowerMonitor(
		mockCPUPowerMeter,
		WithLogger(logger),
		WithClock(mockClock),
		WithResourceInformer(mockResourceInformer))

	t.Run("Initial measurement", func(t *testing.T) {
		mockCPUPowerMeter.On("Zones").Return(testZones, nil)
		pkg.Inc(100 * Joule) // Start at 100J

		snapshot := NewSnapshot()
		err := pm.firstNodeRead(snapshot.Node)
		assert.NoError(t, err)

		pkgZone := snapshot.Node.Zones[pkg]
		// firstNodeRead now calculates initial values based on absolute energy and CPU usage ratio
		expectedActive := Energy(float64(100*Joule) * 0.4) // 40J (40% CPU usage)
		expectedIdle := Energy(100*Joule) - expectedActive // 60J

		assert.Equal(t, expectedActive, pkgZone.activeEnergy, "First reading activeEnergy should be portion of absolute energy")
		assert.Equal(t, expectedActive, pkgZone.ActiveEnergyTotal, "First reading ActiveEnergyTotal should equal activeEnergy")
		assert.Equal(t, expectedIdle, pkgZone.IdleEnergyTotal, "First reading IdleEnergyTotal should be remaining portion")

		pm.snapshot.Store(snapshot)
		mockCPUPowerMeter.AssertExpectations(t)
	})

	t.Run("Second measurement - verify interval-based attribution", func(t *testing.T) {
		mockClock.Step(2 * time.Second)
		mockCPUPowerMeter.ExpectedCalls = nil
		mockCPUPowerMeter.On("Zones").Return(testZones, nil)

		pkg.Inc(50 * Joule) // Total: 150J, Delta: 50J

		prev := pm.snapshot.Load()
		current := NewSnapshot()
		err := pm.calculateNodePower(prev.Node, current.Node)
		assert.NoError(t, err)

		pkgZone := current.Node.Zones[pkg]

		// ActiveEnergy and IdleEnergy should represent the portion of delta energy for this interval
		deltaEnergy := Energy(50 * Joule)
		expectedActiveEnergy := Energy(float64(deltaEnergy) * mockNode.CPUUsageRatio) // 50J * 0.4 = 20J
		expectedIdleEnergy := deltaEnergy - expectedActiveEnergy                      // 50J - 20J = 30J

		assert.Equal(t, expectedActiveEnergy, pkgZone.activeEnergy,
			"activeEnergy should be 20J (40% of 50J delta energy)")

		// ActiveEnergyTotal should accumulate: previous (40J from firstNodeRead) + current interval (20J)
		expectedActiveTotal := Energy(40*Joule) + expectedActiveEnergy // 40J + 20J = 60J
		expectedIdleTotal := Energy(60*Joule) + expectedIdleEnergy     // 60J + 30J = 90J

		assert.Equal(t, expectedActiveTotal, pkgZone.ActiveEnergyTotal,
			"ActiveEnergyTotal should accumulate from previous + current")
		assert.Equal(t, expectedIdleTotal, pkgZone.IdleEnergyTotal,
			"IdleEnergyTotal should accumulate from previous + current")
		// Total energy attribution should equal delta energy
		expectedTotal := expectedActiveEnergy + expectedIdleEnergy
		assert.Equal(t, deltaEnergy, expectedTotal,
			"ActiveEnergy + IdleEnergy should equal delta energy for this interval")

		pm.snapshot.Store(current)
		mockCPUPowerMeter.AssertExpectations(t)
	})

	t.Run("Third measurement - verify values reset per interval", func(t *testing.T) {
		mockClock.Step(3 * time.Second)
		mockCPUPowerMeter.ExpectedCalls = nil
		mockCPUPowerMeter.On("Zones").Return(testZones, nil)

		pkg.Inc(60 * Joule) // Total: 210J, Delta: 60J

		prev := pm.snapshot.Load()
		current := NewSnapshot()
		err := pm.calculateNodePower(prev.Node, current.Node)
		assert.NoError(t, err)

		pkgZone := current.Node.Zones[pkg]

		// ActiveEnergy and IdleEnergy should represent only this interval's attribution
		deltaEnergy := Energy(60 * Joule)
		expectedActiveEnergy := Energy(float64(deltaEnergy) * mockNode.CPUUsageRatio) // 60J * 0.4 = 24J
		expectedIdleEnergy := deltaEnergy - expectedActiveEnergy                      // 60J - 24J = 36J

		assert.Equal(t, expectedActiveEnergy, pkgZone.activeEnergy,
			"activeEnergy should be 24J (not cumulative from previous measurement)")
		// Validate cumulative totals: previous values + current interval
		prevPkgZone := prev.Node.Zones[pkg]
		expectedActiveTotal := prevPkgZone.ActiveEnergyTotal + expectedActiveEnergy
		expectedIdleTotal := prevPkgZone.IdleEnergyTotal + expectedIdleEnergy
		assert.Equal(t, expectedActiveTotal, pkgZone.ActiveEnergyTotal,
			"ActiveEnergyTotal should accumulate previous total + current activeEnergy")
		assert.Equal(t, expectedIdleTotal, pkgZone.IdleEnergyTotal,
			"IdleEnergyTotal should accumulate previous total + current idle energy")
		// Total energy attribution should equal delta energy
		expectedTotal := expectedActiveEnergy + expectedIdleEnergy
		assert.Equal(t, deltaEnergy, expectedTotal,
			"ActiveEnergy + IdleEnergy should equal delta energy for this interval")

		pm.snapshot.Store(current)
		mockCPUPowerMeter.AssertExpectations(t)
	})

	t.Run("Fourth measurement - verify dynamic attribution with CPU usage change", func(t *testing.T) {
		// Change CPU usage ratio to test attribution logic
		mockNode.CPUUsageRatio = 0.8 // 80% CPU usage

		mockClock.Step(1 * time.Second)
		mockCPUPowerMeter.ExpectedCalls = nil
		mockCPUPowerMeter.On("Zones").Return(testZones, nil)

		pkg.Inc(40 * Joule) // Total: 250J, Delta: 40J

		prev := pm.snapshot.Load()
		current := NewSnapshot()
		err := pm.calculateNodePower(prev.Node, current.Node)
		assert.NoError(t, err)

		pkgZone := current.Node.Zones[pkg]

		// ActiveEnergy and IdleEnergy should reflect new CPU usage ratio for this interval
		deltaEnergy := Energy(40 * Joule)
		expectedActiveEnergy := Energy(float64(deltaEnergy) * mockNode.CPUUsageRatio) // 40J * 0.8 = 32J
		expectedIdleEnergy := deltaEnergy - expectedActiveEnergy                      // 40J - 32J = 8J

		assert.Equal(t, expectedActiveEnergy, pkgZone.activeEnergy,
			"activeEnergy should be 32J (80% of 40J delta with new CPU usage)")
		// Validate cumulative totals: previous values + current interval
		prevPkgZone := prev.Node.Zones[pkg]
		expectedActiveTotal := prevPkgZone.ActiveEnergyTotal + expectedActiveEnergy
		expectedIdleTotal := prevPkgZone.IdleEnergyTotal + expectedIdleEnergy
		assert.Equal(t, expectedActiveTotal, pkgZone.ActiveEnergyTotal,
			"ActiveEnergyTotal should accumulate previous total + current activeEnergy")
		assert.Equal(t, expectedIdleTotal, pkgZone.IdleEnergyTotal,
			"IdleEnergyTotal should accumulate previous total + current idle energy")

		// Verify the relationship holds: totals should be greater than current interval
		assert.Greater(t, pkgZone.ActiveEnergyTotal.MicroJoules(), expectedActiveEnergy.MicroJoules(),
			"ActiveEnergyTotal should be greater than current interval activeEnergy")
		assert.Greater(t, pkgZone.IdleEnergyTotal.MicroJoules(), expectedIdleEnergy.MicroJoules(),
			"IdleEnergyTotal should be greater than current interval idle energy")

		// Total energy attribution should equal delta energy
		expectedTotal := expectedActiveEnergy + expectedIdleEnergy
		assert.Equal(t, deltaEnergy, expectedTotal,
			"ActiveEnergy + IdleEnergy should equal delta energy for this interval")

		mockCPUPowerMeter.AssertExpectations(t)
	})

	mockResourceInformer.AssertExpectations(t)
}

func TestNodeActiveEnergyTotalAccumulation(t *testing.T) {
	// Test to explicitly verify that ActiveEnergyTotal and IdleEnergyTotal correctly accumulate
	// over multiple measurements, and that activeEnergy represents only the current interval
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	pkg := device.NewMockRaplZone(
		"package-0",
		0, "/sys/class/powercap/intel-rapl/intel-rapl:0", 1000*Joule)

	mockCPUPowerMeter := &MockCPUPowerMeter{}
	testZones := []EnergyZone{pkg}
	mockCPUPowerMeter.On("Zones").Return(testZones, nil)
	mockCPUPowerMeter.On("PrimaryEnergyZone").Return(testZones[0], nil)

	mockResourceInformer := &MockResourceInformer{}
	mockNode := &resource.Node{
		CPUUsageRatio:            0.6, // 60% CPU usage
		ProcessTotalCPUTimeDelta: 100.0,
	}
	mockResourceInformer.On("Node").Return(mockNode, nil)

	mockClock := test_clock.NewFakeClock(time.Now())

	pm := &PowerMonitor{
		logger:    logger,
		cpu:       mockCPUPowerMeter,
		clock:     mockClock,
		resources: mockResourceInformer,
	}

	err := pm.Init()
	require.NoError(t, err)

	t.Run("measurement 1 - initial", func(t *testing.T) {
		pkg.Inc(100 * Joule) // Initial value

		snapshot := NewSnapshot()
		err := pm.firstNodeRead(snapshot.Node)
		assert.NoError(t, err)

		pkgZone := snapshot.Node.Zones[pkg]
		// firstNodeRead now calculates initial values based on absolute energy and CPU usage ratio
		expectedActive := Energy(float64(100*Joule) * 0.6) // 60J (60% CPU usage)
		expectedIdle := Energy(100*Joule) - expectedActive // 40J

		assert.Equal(t, expectedActive, pkgZone.activeEnergy, "Initial activeEnergy should be portion of absolute energy")
		assert.Equal(t, expectedActive, pkgZone.ActiveEnergyTotal, "Initial ActiveEnergyTotal should equal activeEnergy")
		assert.Equal(t, expectedIdle, pkgZone.IdleEnergyTotal, "Initial IdleEnergyTotal should be remaining portion")

		pm.snapshot.Store(snapshot)
	})

	t.Run("measurement 2", func(t *testing.T) {
		pkg.Inc(50 * Joule) // Total: 150J, Delta: 50J

		prev := pm.snapshot.Load()
		current := NewSnapshot()
		err := pm.calculateNodePower(prev.Node, current.Node)
		assert.NoError(t, err)

		pkgZone := current.Node.Zones[pkg]
		deltaEnergy := Energy(50 * Joule)
		expectedActive := Energy(float64(deltaEnergy) * 0.6) // 30J
		expectedIdle := deltaEnergy - expectedActive         // 20J

		assert.Equal(t, expectedActive, pkgZone.activeEnergy, "activeEnergy should be interval-based")

		// ActiveEnergyTotal should accumulate: previous (60J from firstNodeRead) + current interval (30J)
		expectedActiveTotal := Energy(60*Joule) + expectedActive // 60J + 30J = 90J
		expectedIdleTotal := Energy(40*Joule) + expectedIdle     // 40J + 20J = 60J

		assert.Equal(t, expectedActiveTotal, pkgZone.ActiveEnergyTotal, "ActiveEnergyTotal should accumulate from previous + current")
		assert.Equal(t, expectedIdleTotal, pkgZone.IdleEnergyTotal, "IdleEnergyTotal should accumulate from previous + current")

		pm.snapshot.Store(current)
	})

	t.Run("measurement 3", func(t *testing.T) {
		pkg.Inc(40 * Joule) // Total: 190J, Delta: 40J

		prev := pm.snapshot.Load()
		current := NewSnapshot()
		err := pm.calculateNodePower(prev.Node, current.Node)
		assert.NoError(t, err)

		pkgZone := current.Node.Zones[pkg]
		deltaEnergy := Energy(40 * Joule)
		expectedActive := Energy(float64(deltaEnergy) * 0.6) // 24J
		expectedIdle := deltaEnergy - expectedActive         // 16J

		// Validate interval values (should not be cumulative)
		assert.Equal(t, expectedActive, pkgZone.activeEnergy, "activeEnergy should be interval-based (not cumulative)")

		// Validate cumulative totals
		// Previous: 90J active + 60J idle (from measurement 2)
		// Current interval: 24J active + 16J idle
		expectedActiveTotal := Energy(90*Joule) + expectedActive // 90J + 24J = 114J
		expectedIdleTotal := Energy(60*Joule) + expectedIdle     // 60J + 16J = 76J

		assert.Equal(t, expectedActiveTotal, pkgZone.ActiveEnergyTotal, "ActiveEnergyTotal should accumulate")
		assert.Equal(t, expectedIdleTotal, pkgZone.IdleEnergyTotal, "IdleEnergyTotal should accumulate")

		pm.snapshot.Store(current)
	})

	t.Run("measurement 4", func(t *testing.T) {
		pkg.Inc(30 * Joule) // Total: 220J, Delta: 30J

		prev := pm.snapshot.Load()
		current := NewSnapshot()
		err := pm.calculateNodePower(prev.Node, current.Node)
		assert.NoError(t, err)

		pkgZone := current.Node.Zones[pkg]
		deltaEnergy := Energy(30 * Joule)
		expectedActive := Energy(float64(deltaEnergy) * 0.6) // 18J
		expectedIdle := deltaEnergy - expectedActive         // 12J

		// Validate interval values
		assert.Equal(t, expectedActive, pkgZone.activeEnergy, "activeEnergy should be interval-based")

		// Validate cumulative totals
		// Previous: 114J active + 76J idle (from measurement 3)
		// Current interval: 18J active + 12J idle
		expectedActiveTotal := Energy(114*Joule) + expectedActive // 114J + 18J = 132J
		expectedIdleTotal := Energy(76*Joule) + expectedIdle      // 76J + 12J = 88J

		assert.Equal(t, expectedActiveTotal, pkgZone.ActiveEnergyTotal, "ActiveEnergyTotal should accumulate")
		assert.Equal(t, expectedIdleTotal, pkgZone.IdleEnergyTotal, "IdleEnergyTotal should accumulate")

		// Final validation: verify total accumulated energy equals initial + sum of all interval deltas
		initialEnergy := Energy(100 * Joule)                                    // Initial energy from firstNodeRead: 100J
		totalIntervalDeltas := Energy(50*Joule + 40*Joule + 30*Joule)           // Sum of interval deltas: 120J
		totalAccumulated := pkgZone.ActiveEnergyTotal + pkgZone.IdleEnergyTotal // 132J + 88J = 220J
		expectedTotal := initialEnergy + totalIntervalDeltas                    // 100J + 120J = 220J
		assert.Equal(t, expectedTotal, totalAccumulated, "Total accumulated energy should equal initial + sum of interval deltas")

		pm.snapshot.Store(current)
	})

	mockCPUPowerMeter.AssertExpectations(t)
	mockResourceInformer.AssertExpectations(t)
}
