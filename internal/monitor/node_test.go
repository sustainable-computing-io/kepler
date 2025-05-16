// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package monitor

import (
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/sustainable-computing-io/kepler/internal/device"

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

		mockCPUPowerMeter.On("Zones").Return(testZones, nil).Once()

		// Create a custom PowerMonitor with the mock readers
		pm := NewPowerMonitor(
			mockCPUPowerMeter,
			WithLogger(logger),
			WithClock(mockClock))
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
			assert.Equal(t, raplPkgEnergy.MicroJoules(), pkgZone.Absolute.MicroJoules())
			assert.Equal(t, Energy(0), pkgZone.Delta) // First reading has 0 diff
			assert.Equal(t, Power(0), pkgZone.Power)  // Should be 0 for first reading

			// Check core zone values
			coreZone := current.Node.Zones[core]
			raplCoreEnergy, _ := core.Energy()
			assert.Equal(t, raplCoreEnergy.MicroJoules(), coreZone.Absolute.MicroJoules())
			assert.Equal(t, Energy(0), coreZone.Delta)
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
			assert.Equal(t, raplPkgEnergy, pkgZone.Absolute)     // No difference in Absolute counter
			assert.InDelta(t, 50, pkgZone.Delta.Joules(), 0.001) // Should see 50 joules difference
			assert.InDelta(t, 50, pkgZone.Power.Watts(), 0.001)  // 50 joules / 1 second = 50 watts

			coreZone := current.Node.Zones[core]
			raplCoreEnergy, _ := core.Energy()
			assert.Equal(t, raplCoreEnergy, coreZone.Absolute)    // No difference in Absolute counter
			assert.InDelta(t, 25, coreZone.Delta.Joules(), 0.001) // Should see 25 joules difference
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
			assert.Equal(t, raplPkgEnergy, pkgZone.Absolute)
			assert.InDelta(t, 75, pkgZone.Delta.Joules(), 0.001)
			assert.InDelta(t, 25, pkgZone.Power.Watts(), 0.001)

			coreZone := current.Node.Zones[core]
			raplCoreEnergy, _ := core.Energy()
			assert.Equal(t, raplCoreEnergy, coreZone.Absolute)
			assert.InDelta(t, 45, coreZone.Delta.Joules(), 0.001)
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
			assert.Equal(t, raplPkgEnergy, pkgZone.Absolute)

			assert.InDelta(t, 80, pkgZone.Delta.Joules(), 0.001)
			assert.InDelta(t, 8, pkgZone.Power.Watts(), 0.001)

			coreZone := current.Node.Zones[core]
			raplCoreEnergy, _ := core.Energy()
			assert.Equal(t, raplCoreEnergy, coreZone.Absolute)
			assert.InDelta(t, 30, coreZone.Delta.Joules(), 0.001)
			assert.InDelta(t, 3, coreZone.Power.Watts(), 0.001)

			pm.snapshot.Store(current)
		})
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

	// Create PowerMonitor with the mock
	pm := NewPowerMonitor(
		mockCPUPowerMeter,
		WithLogger(logger),
		WithClock(mockClock),
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
