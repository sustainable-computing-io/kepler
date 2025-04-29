// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package monitor

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/sustainable-computing-io/kepler/internal/device"
	test_clock "k8s.io/utils/clock/testing"
)

func TestNewPowerMonitor(t *testing.T) {
	tests := []struct {
		name string
		opts []OptionFn
		want string
	}{{
		name: "default options",
		opts: []OptionFn{},
		want: "monitor",
	}, {
		name: "with logger",
		opts: []OptionFn{
			WithLogger(slog.Default().With("test", "custom")),
		},
		want: "monitor",
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockPowerMeter := &MockCPUPowerMeter{}
			mockPowerMeter.On("Name").Return("mock-cpu")
			monitor := NewPowerMonitor(mockPowerMeter, tt.opts...)

			// Check if monitor is correctly initialized
			assert.NotNil(t, monitor)
			assert.Equal(t, tt.want, monitor.Name())
			assert.NotNil(t, monitor.dataCh)
			assert.Nil(t, monitor.snapshot.Load())
			assert.NotNil(t, monitor.logger)
			assert.NotNil(t, monitor.cpu)
		})
	}
}

var _ device.EnergyZone = (*MockEnergyZone)(nil)

// TestPowerMonitor_Init tests the Init method that initializes the monitor and
// has Zone names calculated
func TestPowerMonitor_Init(t *testing.T) {
	mockPowerMeter := &MockCPUPowerMeter{}
	pkg := &MockEnergyZone{}
	pkg.On("Name").Return("package")
	pkg.On("Index").Return(0)
	pkg.On("Path").Return("")
	pkg.On("Energy").Return(Energy(100_000), nil)
	pkg.On("MaxEnergy").Return(Energy(1_000_000))
	energyZones := []device.EnergyZone{
		pkg,
	}
	mockPowerMeter.On("Zones").Return(energyZones, nil)
	mockPowerMeter.On("Name").Return("mock-cpu")

	fakePowerMeter, err := device.NewFakeCPUMeter(nil)
	require.NoError(t, err)

	powerMeters := []device.CPUPowerMeter{
		mockPowerMeter,
		fakePowerMeter,
	}

	zoneNamesFromMeter := func(meter device.CPUPowerMeter) []string {
		energyZones, err := meter.Zones()
		if err != nil {
			log.Fatal(err)
		}
		var zoneNames []string
		for _, zone := range energyZones {
			zoneNames = append(zoneNames, zone.Name())
		}
		return zoneNames
	}

	for _, powerMeter := range powerMeters {
		t.Run(powerMeter.Name(), func(t *testing.T) {
			monitor := NewPowerMonitor(
				powerMeter,
				WithLogger(slog.Default()),
			)

			err := monitor.Init()
			assert.NoError(t, err)
			assert.Equal(t, monitor.ZoneNames(), zoneNamesFromMeter(powerMeter))
		})
	}
	mockPowerMeter.AssertExpectations(t)
}

func TestPowerMonitor_DataChannel(t *testing.T) {
	mockPowerMeter := &MockCPUPowerMeter{}
	monitor := NewPowerMonitor(mockPowerMeter)

	dataCh := monitor.DataChannel()
	assert.NotNil(t, dataCh)
}

func TestPowerMonitor_ZoneNames(t *testing.T) {
	mockPowerMeter := &MockCPUPowerMeter{}
	monitor := NewPowerMonitor(mockPowerMeter)
	// TODO: Implement zone names validation

	names := monitor.ZoneNames()
	assert.Empty(t, names)
}

func TestPowerMonitor_Snapshot(t *testing.T) {
	mockPowerMeter := &MockCPUPowerMeter{}
	pkg := &MockEnergyZone{}
	pkg.On("Name").Return("package")
	pkg.On("Index").Return(0)
	pkg.On("Path").Return("")
	pkg.On("Energy").Return(Energy(100_000), nil)
	pkg.On("MaxEnergy").Return(Energy(1_000_000))

	energyZones := []device.EnergyZone{
		pkg,
	}

	mockPowerMeter.On("Init").Return(nil)
	mockPowerMeter.On("Zones").Return(energyZones, nil)

	monitor := NewPowerMonitor(mockPowerMeter)

	snapshot, err := monitor.Snapshot()
	assert.NotNil(t, snapshot)
	assert.Nil(t, err)

	// ensure that snapshot is a clone
	assert.NotSame(t, monitor.snapshot.Load(), snapshot)
	assert.Equal(t, monitor.snapshot.Load(), snapshot)
}

func TestPowerMonitor_InitZones(t *testing.T) {
	fakePowerMeter, err := device.NewFakeCPUMeter(nil)
	require.NoError(t, err, "failed to create fake power meter")
	monitor := NewPowerMonitor(fakePowerMeter)

	err = monitor.Init()
	assert.NoError(t, err)
	assert.NotEmpty(t, monitor.zonesNames)
}

func TestPowerMonitor_Init_Success(t *testing.T) {
	mockMeter := &MockCPUPowerMeter{}

	pkg := &MockEnergyZone{}
	pkg.On("Name").Return("package")
	core := &MockEnergyZone{}
	core.On("Name").Return("core")
	mockMeter.On("Zones").Return([]EnergyZone{pkg, core}, nil)

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	monitor := NewPowerMonitor(mockMeter, WithLogger(logger))

	err := monitor.Init()
	assert.NoError(t, err)
	assert.Equal(t, []string{"package", "core"}, monitor.ZoneNames())

	select {
	case <-monitor.dataCh:
		// Signal received as expected
	default:
		t.Error("Expected signal in data channel but none received")
	}

	// Verify mocks
	mockMeter.AssertExpectations(t)
	pkg.AssertExpectations(t)
	core.AssertExpectations(t)
}

func TestPowerMonitor_Init_CPUInitFailure(t *testing.T) {
	mockMeter := &MockCPUPowerMeter{}

	cpuInitError := errors.New("cpu init failed")
	mockMeter.On("Zones").Return([]EnergyZone{}, cpuInitError)

	monitor := NewPowerMonitor(mockMeter)

	err := monitor.Init()
	assert.Error(t, err)
	assert.ErrorContains(t, err, "zone initialization failed")
	assert.ErrorIs(t, errors.Unwrap(err), cpuInitError)

	assert.Empty(t, monitor.ZoneNames())

	// Verify no signal was sent (dataCh should be empty)
	select {
	case <-monitor.dataCh:
		t.Error("No signal expected in data channel")
	default:
		// No signal as expected
	}

	mockMeter.AssertExpectations(t)
}

func TestPowerMonitor_Init_ZonesFailure(t *testing.T) {
	mockMeter := &MockCPUPowerMeter{}

	zonesError := errors.New("failed to retrieve zones")
	mockMeter.On("Zones").Return([]EnergyZone{}, zonesError)

	monitor := NewPowerMonitor(mockMeter)

	err := monitor.Init()

	assert.Error(t, err)
	assert.ErrorContains(t, err, "zone initialization failed")
	assert.ErrorIs(t, errors.Unwrap(err), zonesError)

	// Zones should not be initialized
	assert.Empty(t, monitor.ZoneNames())

	// Verify no signal was sent
	select {
	case <-monitor.dataCh:
		t.Error("No signal expected in data channel")
	default:
		// No signal as expected
	}

	// Verify mocks
	mockMeter.AssertExpectations(t)
}

func TestPowerMonitor_Run(t *testing.T) {
	mockMeter := &MockCPUPowerMeter{}
	// Set up pkg mock
	pkg := &MockEnergyZone{}
	pkg.On("Name").Return("package")

	pkg.On("Energy").Return(Energy(100*Joule), nil)

	mockMeter.On("Zones").Return([]EnergyZone{pkg}, nil)
	monitor := NewPowerMonitor(mockMeter)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err := monitor.Init()
	require.NoError(t, err)

	// Start Run in a go routine since it  should blocks until context cancellation
	runComplete := make(chan struct{})
	go func() {
		err := monitor.Run(ctx)
		assert.NoError(t, err)
		close(runComplete)
	}()

	time.Sleep(10 * time.Millisecond)

	// cancel to stop Running
	cancel()

	// Wait for Run to complete
	select {
	case <-runComplete:
		// Run completed as expected
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Run didn't exit after context cancellation")
	}

	// Verify mocks
	mockMeter.AssertExpectations(t)
	pkg.AssertExpectations(t)
}

func TestPowerMonitor_Run_WithTimeout(t *testing.T) {
	mockMeter := &MockCPUPowerMeter{}
	// Set up pkg mock
	pkg := &MockEnergyZone{}
	pkg.On("Name").Return("package")
	pkg.On("MaxEnergy").Return(Energy(1000 * Joule))

	pkg.On("Energy").Return(Energy(100*Joule), nil)

	mockMeter.On("Init", mock.Anything).Return(nil).Once()
	mockMeter.On("Zones").Return([]EnergyZone{pkg}, nil)
	monitor := NewPowerMonitor(mockMeter)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	startTime := time.Now()
	err := monitor.Run(ctx) // will block until context times out
	duration := time.Since(startTime)

	assert.NoError(t, err)
	assert.GreaterOrEqual(t, duration, 50*time.Millisecond)
	assert.Less(t, duration, 100*time.Millisecond, "Run should exit promptly after context expires")
}

// TestPowerMonitor_FullInitRunShutdownCycle tests the complete lifecycle
func TestPowerMonitor_FullInitRunShutdownCycle(t *testing.T) {
	mockMeter := &MockCPUPowerMeter{}

	zone := &MockEnergyZone{}
	zone.On("Name").Return("test-zone")
	zone.On("Energy").Return(Energy(100*Joule), nil)
	mockMeter.On("Zones").Return([]EnergyZone{zone}, nil)

	monitor := NewPowerMonitor(mockMeter)
	err := monitor.Init()
	require.NoError(t, err)

	// Start Run in goroutine
	ctx, cancel := context.WithCancel(context.Background())
	runComplete := make(chan struct{})
	go func() {
		err := monitor.Run(ctx)
		assert.NoError(t, err)
		close(runComplete)
	}()

	// short delay to ensure Run is executing
	time.Sleep(10 * time.Millisecond)

	// cancel context to stop Run
	cancel()

	// wait for Run to complete
	select {
	case <-runComplete:
		// Success
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Run didn't exit after context cancellation")
	}

	mockMeter.AssertExpectations(t)
	zone.AssertExpectations(t)
}

// TestMonitorRefreshSnapshot tests the PowerMonitor.refreshSnapshot
func TestMonitorRefreshSnapshot(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	pkg := device.NewMockRaplZone(
		"package-0",
		0, "/sys/class/powercap/intel-rapl/intel-rapl:0", 200*Joule)

	core := device.NewMockRaplZone(
		"core-0", 0, "/sys/class/powercap/intel-rapl/intel-rapl:0/intel-rapl:0:0", 150*Joule)

	testZones := []EnergyZone{pkg, core}
	mockCPUPowerMeter := &MockCPUPowerMeter{}

	t.Run("Basic", func(t *testing.T) {
		startTime := time.Date(2025, 4, 29, 11, 20, 0, 0, time.UTC)
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

			err := pm.refreshSnapshot()
			assert.NoError(t, err)

			// Verify mock expectations
			mockCPUPowerMeter.AssertExpectations(t)

			// Check that both zones have data
			current := pm.snapshot.Load()
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

			err := pm.refreshSnapshot()
			assert.NoError(t, err)

			mockCPUPowerMeter.AssertExpectations(t)

			// Check package zone values for second reading
			current := pm.snapshot.Load()
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
			mockClock.Step(3 * time.Second)

			mockCPUPowerMeter.ExpectedCalls = nil

			pkg.Inc(3 * 25 * Joule)
			core.Inc(3 * 15 * Joule)
			mockCPUPowerMeter.On("Zones").Return(testZones, nil)

			// Collect node power data again
			err := pm.refreshSnapshot()
			assert.NoError(t, err)

			mockCPUPowerMeter.AssertExpectations(t)

			// Check package zone values for second reading
			current := pm.snapshot.Load()
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
			err := pm.refreshSnapshot()
			assert.NoError(t, err)

			mockCPUPowerMeter.AssertExpectations(t)

			// Check package zone values for second reading
			current := pm.snapshot.Load()
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

func TestRefreshSnapshotError(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

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
		err := pm.refreshSnapshot()
		assert.Error(t, err, "zone read errors must be propagated")
		snapshot := pm.snapshot.Load()
		assert.Empty(t, snapshot)
		mockCPUPowerMeter.AssertExpectations(t)
	})

	t.Run("Fix first read", func(t *testing.T) {
		mockCPUPowerMeter.ExpectedCalls = nil
		pkg := device.NewMockRaplZone(
			"package-0",
			0, "/sys/class/powercap/intel-rapl/intel-rapl:0", 200*Joule)

		core := device.NewMockRaplZone(
			"core-0", 0, "/sys/class/powercap/intel-rapl/intel-rapl:0/intel-rapl:0:0", 150*Joule)

		testZones := []EnergyZone{pkg, core}
		mockCPUPowerMeter.On("Zones").Return(testZones, nil)
		mockClock.Step(10 * time.Second)

		err := pm.refreshSnapshot()
		assert.NoError(t, err)
		snapshot := pm.snapshot.Load()
		assert.Equal(t, mockClock.Now(), snapshot.Timestamp)
		assert.Contains(t, snapshot.Node.Zones, pkg)
		assert.Contains(t, snapshot.Node.Zones, core)

		mockCPUPowerMeter.AssertExpectations(t)
	})

	t.Run("Error on computePower", func(t *testing.T) {
		mockCPUPowerMeter.ExpectedCalls = nil
		mockCPUPowerMeter.On("Zones").Return([]EnergyZone(nil), assert.AnError)
		mockClock.Step(30 * time.Second)
		err := pm.refreshSnapshot()
		assert.Error(t, err, "zone read errors must be propagated")
		snapshot := pm.snapshot.Load()
		assert.NotEqual(t, mockClock.Now(), snapshot.Timestamp)
	})

	t.Run("Fix computePower", func(t *testing.T) {
		mockCPUPowerMeter.ExpectedCalls = nil
		pkg := device.NewMockRaplZone(
			"package-0",
			0, "/sys/class/powercap/intel-rapl/intel-rapl:0", 200*Joule)

		core := device.NewMockRaplZone(
			"core-0", 0, "/sys/class/powercap/intel-rapl/intel-rapl:0/intel-rapl:0:0", 150*Joule)

		testZones := []EnergyZone{pkg, core}
		mockCPUPowerMeter.On("Zones").Return(testZones, nil)
		mockClock.Step(30 * time.Second)

		err := pm.refreshSnapshot()
		assert.NoError(t, err)
		snapshot := pm.snapshot.Load()
		assert.Equal(t, mockClock.Now(), snapshot.Timestamp)
		assert.Contains(t, snapshot.Node.Zones, pkg)
		assert.Contains(t, snapshot.Node.Zones, core)
	})
}
