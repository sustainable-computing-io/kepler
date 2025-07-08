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
	"github.com/sustainable-computing-io/kepler/internal/resource"
	testingclock "k8s.io/utils/clock/testing"
)

func TestNewPowerMonitor(t *testing.T) {
	tr := CreateTestResources()
	resourceInformer := &MockResourceInformer{}
	resourceInformer.SetExpectations(t, tr)
	resourceInformer.On("Refresh").Return(nil)

	tt := []struct {
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
			WithResourceInformer(resourceInformer),
		},
		want: "monitor",
	}}

	for _, tt := range tt {
		t.Run(tt.name, func(t *testing.T) {
			mockPowerMeter := &MockCPUPowerMeter{}
			mockPowerMeter.On("Name").Return("mock-cpu")
			monitor := NewPowerMonitor(
				mockPowerMeter,
				tt.opts...)

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
	mockPowerMeter.On("PrimaryEnergyZone").Return(pkg, nil)
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
	mockPowerMeter.On("PrimaryEnergyZone").Return(pkg, nil)

	tr := CreateTestResources()
	resourceInformer := &MockResourceInformer{}
	resourceInformer.SetExpectations(t, tr)
	resourceInformer.On("Refresh").Return(nil)

	monitor := NewPowerMonitor(mockPowerMeter, WithResourceInformer(resourceInformer))

	err := monitor.Init()
	require.NoError(t, err)

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
	mockMeter.On("PrimaryEnergyZone").Return(pkg, nil)

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
	mockMeter.On("PrimaryEnergyZone").Return(pkg, nil)

	tr := CreateTestResources()
	resourceInformer := &MockResourceInformer{}
	resourceInformer.SetExpectations(t, tr)
	resourceInformer.On("Refresh").Return(nil)

	monitor := NewPowerMonitor(mockMeter, WithResourceInformer(resourceInformer))

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
	mockMeter.On("PrimaryEnergyZone").Return(pkg, nil)

	tr := CreateTestResources()
	resourceInformer := &MockResourceInformer{}
	resourceInformer.SetExpectations(t, tr)
	resourceInformer.On("Refresh").Return(nil)

	monitor := NewPowerMonitor(mockMeter, WithResourceInformer(resourceInformer))

	err := monitor.Init()
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	startTime := time.Now()
	err = monitor.Run(ctx) // will block until context times out
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
	mockMeter.On("PrimaryEnergyZone").Return(zone, nil)

	tr := CreateTestResources()
	resourceInformer := &MockResourceInformer{}
	resourceInformer.SetExpectations(t, tr)
	resourceInformer.On("Refresh").Return(nil)

	monitor := NewPowerMonitor(mockMeter, WithResourceInformer(resourceInformer))

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
	mockCPUPowerMeter.On("Zones").Return(testZones, nil)
	mockCPUPowerMeter.On("PrimaryEnergyZone").Return(pkg, nil)

	tr := CreateTestResources()
	resourceInformer := &MockResourceInformer{}
	resourceInformer.SetExpectations(t, tr)
	resourceInformer.On("Refresh").Return(nil)

	t.Run("Basic", func(t *testing.T) {
		startTime := time.Date(2025, 4, 29, 11, 20, 0, 0, time.UTC)
		mockClock := testingclock.NewFakeClock(startTime)

		// Create a custom PowerMonitor with the mock readers
		pm := NewPowerMonitor(
			mockCPUPowerMeter,
			WithLogger(logger),
			WithClock(mockClock),
			WithResourceInformer(resourceInformer),
			WithInterval(0),
		)
		assert.NotNil(t, pm)

		err := pm.Init()
		require.NoError(t, err)

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
			assert.Equal(t, raplPkgEnergy.MicroJoules(), pkgZone.EnergyTotal.MicroJoules())
			assert.Equal(t, Power(0), pkgZone.Power) // Should be 0 for first reading

			// Check core zone values
			coreZone := current.Node.Zones[core]
			raplCoreEnergy, _ := core.Energy()
			assert.Equal(t, raplCoreEnergy.MicroJoules(), coreZone.EnergyTotal.MicroJoules())
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
			mockCPUPowerMeter.On("PrimaryEnergyZone").Return(pkg, nil)

			// Collect node power data again

			err := pm.refreshSnapshot()
			assert.NoError(t, err)

			mockCPUPowerMeter.AssertExpectations(t)

			// Check package zone values for second reading
			current := pm.snapshot.Load()
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
			mockClock.Step(3 * time.Second)

			mockCPUPowerMeter.ExpectedCalls = nil

			pkg.Inc(3 * 25 * Joule)
			core.Inc(3 * 15 * Joule)
			mockCPUPowerMeter.On("Zones").Return(testZones, nil)
			mockCPUPowerMeter.On("PrimaryEnergyZone").Return(pkg, nil)

			// Collect node power data again
			err := pm.refreshSnapshot()
			assert.NoError(t, err)

			mockCPUPowerMeter.AssertExpectations(t)

			// Check package zone values for second reading
			current := pm.snapshot.Load()
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
			mockCPUPowerMeter.On("PrimaryEnergyZone").Return(pkg, nil)

			// Collect node power data again
			err := pm.refreshSnapshot()
			assert.NoError(t, err)

			mockCPUPowerMeter.AssertExpectations(t)

			// Check package zone values for second reading
			current := pm.snapshot.Load()
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
	})
}

func TestRefreshSnapshotError(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	mockCPUPowerMeter := &MockCPUPowerMeter{}

	// Create a mock clock with a fixed start time
	startTime := time.Date(2023, 4, 15, 9, 0, 0, 0, time.UTC)
	mockClock := testingclock.NewFakeClock(startTime)

	tr := CreateTestResources()
	resourceInformer := &MockResourceInformer{}
	resourceInformer.SetExpectations(t, tr)
	resourceInformer.On("Refresh").Return(nil)

	// Create PowerMonitor with the mock
	pm := NewPowerMonitor(
		mockCPUPowerMeter,
		WithLogger(logger),
		WithClock(mockClock),
		WithResourceInformer(resourceInformer),
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
		mockCPUPowerMeter.On("PrimaryEnergyZone").Return(pkg, nil)
		mockClock.Step(10 * time.Second)

		err := pm.Init()
		require.NoError(t, err)
		err = pm.refreshSnapshot()
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
		mockCPUPowerMeter.On("PrimaryEnergyZone").Return(pkg, nil)
		mockClock.Step(30 * time.Second)

		err := pm.Init()
		require.NoError(t, err)
		err = pm.refreshSnapshot()
		assert.NoError(t, err)
		snapshot := pm.snapshot.Load()
		assert.Equal(t, mockClock.Now(), snapshot.Timestamp)
		assert.Contains(t, snapshot.Node.Zones, pkg)
		assert.Contains(t, snapshot.Node.Zones, core)
	})
}

// TestTerminatedWorkloadsClearedAfterSnapshot validates that terminated workloads
// (processes, containers, VMs, pods) are cleared in the first calculation after
// the Snapshot function is called.
func TestTerminatedWorkloadsClearedAfterSnapshot(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	fakeClock := testingclock.NewFakeClock(time.Now())

	// Create mock CPU meter with test zones
	zones := CreateTestZones()
	mockMeter := &MockCPUPowerMeter{}
	mockMeter.On("Zones").Return(zones, nil)
	mockMeter.On("PrimaryEnergyZone").Return(zones[0], nil)

	// Create test resources with processes, containers, VMs, and pods
	tr := CreateTestResources()
	resourceInformer := &MockResourceInformer{}
	resourceInformer.SetExpectations(t, tr)
	resourceInformer.On("Refresh").Return(nil)

	// Create monitor
	monitor := &PowerMonitor{
		logger:        logger,
		cpu:           mockMeter,
		clock:         fakeClock,
		resources:     resourceInformer,
		maxTerminated: 500,
	}

	err := monitor.Init()
	require.NoError(t, err)

	// Step 1: Create initial snapshot with some workloads using refreshSnapshot
	err = monitor.refreshSnapshot()
	require.NoError(t, err)

	// Get the initial snapshot and add some energy to workloads so they can be terminated with non-zero energy
	snapshot1 := monitor.snapshot.Load()
	require.NotNil(t, snapshot1)

	// Add some energy to processes so they can be terminated with non-zero energy
	for _, zone := range zones {
		if proc, exists := snapshot1.Processes["456"]; exists {
			proc.Zones[zone] = Usage{
				EnergyTotal: 100 * Joule,
				Power:       10 * Watt,
			}
		}
		if proc, exists := snapshot1.Processes["789"]; exists {
			proc.Zones[zone] = Usage{
				EnergyTotal: 150 * Joule,
				Power:       15 * Watt,
			}
		}
	}

	// Add some energy to containers
	for _, zone := range zones {
		if container, exists := snapshot1.Containers["container-2"]; exists {
			container.Zones[zone] = Usage{
				EnergyTotal: 200 * Joule,
				Power:       20 * Watt,
			}
		}
	}

	// Add some energy to VMs
	for _, zone := range zones {
		if vm, exists := snapshot1.VirtualMachines["vm-2"]; exists {
			vm.Zones[zone] = Usage{
				EnergyTotal: 300 * Joule,
				Power:       30 * Watt,
			}
		}
	}

	// Step 2: Advance time and create test resources with terminated workloads
	fakeClock.Step(5 * time.Second)

	// Create new test resources where some workloads are now terminated
	trWithTerminated := &TestResource{
		Node: tr.Node, // Keep the same node
		Processes: &resource.Processes{
			Running: map[int]*resource.Process{
				123: tr.Processes.Running[123], // Keep process 123 running
				// Process 456 and 789 are now terminated
			},
			Terminated: map[int]*resource.Process{
				456: tr.Processes.Running[456], // Process 456 terminated
				789: tr.Processes.Running[789], // Process 789 terminated
			},
		},
		Containers: &resource.Containers{
			Running: map[string]*resource.Container{
				"container-1": tr.Containers.Running["container-1"], // Keep container-1 running
				// container-2 is now terminated
			},
			Terminated: map[string]*resource.Container{
				"container-2": tr.Containers.Running["container-2"], // container-2 terminated
			},
		},
		VirtualMachines: &resource.VirtualMachines{
			Running: map[string]*resource.VirtualMachine{
				"vm-1": tr.VirtualMachines.Running["vm-1"], // Keep vm-1 running
				// vm-2 is now terminated
			},
			Terminated: map[string]*resource.VirtualMachine{
				"vm-2": tr.VirtualMachines.Running["vm-2"], // vm-2 terminated
			},
		},
		Pods: &resource.Pods{
			Running: map[string]*resource.Pod{
				"pod-id-1": tr.Pods.Running["pod-id-1"], // Keep pod-id-1 running
			},
			Terminated: map[string]*resource.Pod{},
		},
	}

	// Update mock expectations with terminated workloads
	resourceInformer.ExpectedCalls = nil
	resourceInformer.SetExpectations(t, trWithTerminated)
	resourceInformer.On("Refresh").Return(nil)

	// Step 3: Calculate power with terminated workloads (first calculation after snapshot not exported)
	err = monitor.refreshSnapshot()
	require.NoError(t, err)

	snapshot2 := monitor.snapshot.Load()
	require.NotNil(t, snapshot2)

	// Step 4: Verify terminated workloads are present before Snapshot() is called
	assert.NotEmpty(t, snapshot2.TerminatedProcesses, "Terminated processes should be present before Snapshot() call")
	assert.NotEmpty(t, snapshot2.TerminatedContainers, "Terminated containers should be present before Snapshot() call")
	assert.NotEmpty(t, snapshot2.TerminatedVirtualMachines, "Terminated VMs should be present before Snapshot() call")

	// Verify specific terminated workloads are present
	assert.Contains(t, snapshot2.TerminatedProcesses, "456", "Process 456 should be in terminated processes")
	assert.Contains(t, snapshot2.TerminatedProcesses, "789", "Process 789 should be in terminated processes")
	assert.Contains(t, snapshot2.TerminatedContainers, "container-2", "Container-2 should be in terminated containers")
	assert.Contains(t, snapshot2.TerminatedVirtualMachines, "vm-2", "VM-2 should be in terminated VMs")

	// Step 5: Call Snapshot() to mark it as exported
	exportedSnapshot, err := monitor.Snapshot()
	require.NoError(t, err)
	require.NotNil(t, exportedSnapshot)

	// Verify exported flag is set
	assert.True(t, monitor.exported.Load(), "Exported flag should be true after Snapshot() call")

	// Step 6: Advance time again and perform another calculation
	fakeClock.Step(5 * time.Second)

	// Keep the same resource state (no new terminations)
	resourceInformer.ExpectedCalls = nil
	resourceInformer.SetExpectations(t, trWithTerminated)
	resourceInformer.On("Refresh").Return(nil)

	// Step 7: Calculate power again (first calculation after export)
	err = monitor.refreshSnapshot()
	require.NoError(t, err)

	snapshot3 := monitor.snapshot.Load()
	require.NotNil(t, snapshot3)

	// Step 8: Verify terminated workloads are cleared after Snapshot() was called
	assert.Empty(t, snapshot3.TerminatedProcesses, "Terminated processes should be cleared after Snapshot() call")
	assert.Empty(t, snapshot3.TerminatedContainers, "Terminated containers should be cleared after Snapshot() call")
	assert.Empty(t, snapshot3.TerminatedVirtualMachines, "Terminated VMs should be cleared after Snapshot() call")
	assert.Empty(t, snapshot3.TerminatedPods, "Terminated pods should be cleared after Snapshot() call")

	// Step 9: Verify running workloads are still present
	assert.Contains(t, snapshot3.Processes, "123", "Running process 123 should still be present")
	assert.Contains(t, snapshot3.Containers, "container-1", "Running container-1 should still be present")
	assert.Contains(t, snapshot3.VirtualMachines, "vm-1", "Running vm-1 should still be present")
	assert.Contains(t, snapshot3.Pods, "pod-id-1", "Running pod-id-1 should still be present")

	// Step 10: Test that if Snapshot() is not called, terminated workloads persist
	fakeClock.Step(5 * time.Second)

	// Add new workloads to snapshot3 first so they can be terminated later
	snapshot3.Processes["999"] = &Process{
		PID:          999,
		Comm:         "new-process",
		Exe:          "/usr/bin/new-process",
		CPUTotalTime: 50.0,
		Zones:        make(ZoneUsageMap, len(zones)),
	}
	snapshot3.Containers["container-3"] = &Container{
		ID:           "container-3",
		Name:         "new-container-3",
		Runtime:      resource.ContainerDRuntime,
		CPUTotalTime: 50.0,
		Zones:        make(ZoneUsageMap, len(zones)),
	}
	snapshot3.VirtualMachines["vm-3"] = &VirtualMachine{
		ID:           "vm-3",
		Name:         "new-vm-3",
		Hypervisor:   resource.KVMHypervisor,
		CPUTotalTime: 50.0,
		Zones:        make(ZoneUsageMap, len(zones)),
	}
	snapshot3.Pods["pod-id-3"] = &Pod{
		ID:           "pod-id-3",
		Name:         "new-pod-3",
		Namespace:    "default",
		CPUTotalTime: 50.0,
		Zones:        make(ZoneUsageMap, len(zones)),
	}

	// Add some energy to these new workloads
	for _, zone := range zones {
		snapshot3.Processes["999"].Zones[zone] = Usage{
			EnergyTotal: 75 * Joule,
			Power:       7 * Watt,
		}
		snapshot3.Containers["container-3"].Zones[zone] = Usage{
			EnergyTotal: 125 * Joule,
			Power:       12 * Watt,
		}
		snapshot3.VirtualMachines["vm-3"].Zones[zone] = Usage{
			EnergyTotal: 175 * Joule,
			Power:       17 * Watt,
		}
		snapshot3.Pods["pod-id-3"].Zones[zone] = Usage{
			EnergyTotal: 225 * Joule,
			Power:       22 * Watt,
		}
	}

	// Update the stored snapshot with the new workloads
	monitor.snapshot.Store(snapshot3)

	// Create more terminated workloads
	trWithMoreTerminated := &TestResource{
		Node: tr.Node,
		Processes: &resource.Processes{
			Running: map[int]*resource.Process{
				123: tr.Processes.Running[123], // Still running
			},
			Terminated: map[int]*resource.Process{
				999: {PID: 999, Comm: "new-terminated", Exe: "/usr/bin/new-terminated", CPUTotalTime: 50.0}, // Now terminated process
			},
		},
		Containers: &resource.Containers{
			Running: map[string]*resource.Container{
				"container-1": tr.Containers.Running["container-1"], // Still running
			},
			Terminated: map[string]*resource.Container{
				"container-3": {ID: "container-3", Name: "terminated-container-3", Runtime: resource.ContainerDRuntime}, // Now terminated container
			},
		},
		VirtualMachines: &resource.VirtualMachines{
			Running: map[string]*resource.VirtualMachine{
				"vm-1": tr.VirtualMachines.Running["vm-1"], // Still running
			},
			Terminated: map[string]*resource.VirtualMachine{
				"vm-3": {ID: "vm-3", Name: "terminated-vm-3", Hypervisor: resource.KVMHypervisor}, // Now terminated VM
			},
		},
		Pods: &resource.Pods{
			Running: map[string]*resource.Pod{
				"pod-id-1": tr.Pods.Running["pod-id-1"], // Still running
			},
			Terminated: map[string]*resource.Pod{
				"pod-id-3": {ID: "pod-id-3", Name: "terminated-pod-3", Namespace: "default", CPUTotalTime: 50.0}, // Now terminated pod
			},
		},
	}

	resourceInformer.ExpectedCalls = nil
	resourceInformer.SetExpectations(t, trWithMoreTerminated)
	resourceInformer.On("Refresh").Return(nil)

	// Calculate power without calling Snapshot() first (exported flag should still be false)
	err = monitor.refreshSnapshot()
	require.NoError(t, err)

	snapshot4 := monitor.snapshot.Load()
	require.NotNil(t, snapshot4)

	// Step 11: Verify new terminated workloads are present since Snapshot() wasn't called
	assert.Contains(t, snapshot4.TerminatedProcesses, "999", "New terminated process should be present")
	assert.Contains(t, snapshot4.TerminatedContainers, "container-3", "New terminated container should be present")
	assert.Contains(t, snapshot4.TerminatedVirtualMachines, "vm-3", "New terminated VM should be present")
	assert.Contains(t, snapshot4.TerminatedPods, "pod-id-3", "New terminated pod should be present")

	resourceInformer.AssertExpectations(t)
	mockMeter.AssertExpectations(t)
}

// TestSnapshotFreshnessAndCloning validates that:
// 1. When data is fresh, Snapshot() returns a copy of the existing snapshot without triggering new calculations
// 2. When data is stale, Snapshot() triggers new calculations and returns a copy of the updated snapshot
// 3. The returned snapshot is always a clone (different object) with the same content
func TestSnapshotFreshnessAndCloning(t *testing.T) {
	fakeClock := testingclock.NewFakeClock(time.Now())

	// Create mock CPU meter with test zones
	zones := CreateTestZones()
	mockMeter := &MockCPUPowerMeter{}
	mockMeter.On("Zones").Return(zones, nil)
	mockMeter.On("PrimaryEnergyZone").Return(zones[0], nil)

	// Create test resources
	tr := CreateTestResources()
	resourceInformer := &MockResourceInformer{}
	resourceInformer.SetExpectations(t, tr)
	resourceInformer.On("Refresh").Return(nil)

	// Create monitor with a short max staleness for testing
	maxStaleness := 1 * time.Second
	monitor := NewPowerMonitor(
		mockMeter,
		WithClock(fakeClock),
		WithMaxStaleness(maxStaleness),
		WithResourceInformer(resourceInformer),
	)

	err := monitor.Init()
	require.NoError(t, err)

	// Step 1: Create initial snapshot
	err = monitor.refreshSnapshot()
	require.NoError(t, err)

	initialSnapshot := monitor.snapshot.Load()
	require.NotNil(t, initialSnapshot)
	initialTimestamp := initialSnapshot.Timestamp

	// Step 2: Test fresh data behavior - call Snapshot() immediately (data is fresh)
	resourceInformer.ExpectedCalls = nil // Clear expectations since no new calls should be made

	snapshot1, err := monitor.Snapshot()
	require.NoError(t, err)
	require.NotNil(t, snapshot1)

	// Verify it's a clone (different object, same content)
	assert.NotSame(t, initialSnapshot, snapshot1, "Snapshot should return a clone, not the same object")
	assert.Equal(t, initialSnapshot, snapshot1, "Snapshot content should be equal to the original")

	// Verify no new computation happened (timestamp should be the same)
	currentSnapshot := monitor.snapshot.Load()
	assert.Equal(t, initialTimestamp, currentSnapshot.Timestamp, "No new computation should have occurred when data is fresh")

	// Step 3: Test stale data behavior - advance time beyond staleness threshold
	fakeClock.Step(maxStaleness + 100*time.Millisecond)

	// Expect new calls since data is now stale and will trigger recomputation
	resourceInformer.SetExpectations(t, tr)
	resourceInformer.On("Refresh").Return(nil)

	snapshot2, err := monitor.Snapshot()
	require.NoError(t, err)
	require.NotNil(t, snapshot2)

	// Verify it's a clone with updated content
	newSnapshot := monitor.snapshot.Load()
	assert.NotSame(t, newSnapshot, snapshot2, "Snapshot should return a clone, not the same object")
	assert.Equal(t, newSnapshot, snapshot2, "Snapshot content should be equal to the updated snapshot")

	// Verify new computation happened (timestamp should be updated)
	assert.True(t, newSnapshot.Timestamp.After(initialTimestamp), "New computation should have occurred when data is stale")
	assert.Equal(t, fakeClock.Now(), newSnapshot.Timestamp, "Timestamp should be updated to current time")

	// Step 4: Test fresh data again - call Snapshot() immediately after the previous call
	resourceInformer.ExpectedCalls = nil // Clear expectations since no new calls should be made

	snapshot3, err := monitor.Snapshot()
	require.NoError(t, err)
	require.NotNil(t, snapshot3)

	// Verify it's a clone of the same fresh data
	stillCurrentSnapshot := monitor.snapshot.Load()
	assert.NotSame(t, stillCurrentSnapshot, snapshot3, "Snapshot should return a clone, not the same object")
	assert.Equal(t, stillCurrentSnapshot, snapshot3, "Snapshot content should be equal to the current snapshot")

	// Verify no new computation happened (timestamp should remain the same)
	assert.Equal(t, newSnapshot.Timestamp, stillCurrentSnapshot.Timestamp, "No new computation should have occurred when data is fresh")

	// Step 5: Test that exported flag is set correctly
	assert.True(t, monitor.exported.Load(), "Exported flag should be true after Snapshot() calls")

	resourceInformer.AssertExpectations(t)
	mockMeter.AssertExpectations(t)
}
