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
	"github.com/stretchr/testify/require"
	"github.com/sustainable-computing-io/kepler/internal/device"
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
	monitor := NewPowerMonitor(mockMeter)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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
}

func TestPowerMonitor_Run_WithTimeout(t *testing.T) {
	mockMeter := &MockCPUPowerMeter{}
	monitor := NewPowerMonitor(mockMeter)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	startTime := time.Now()
	err := monitor.Run(ctx)
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
