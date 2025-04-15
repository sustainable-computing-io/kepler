// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package monitor

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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
			mockPowerMeter := new(MockCPUPowerMeter)
			mockPowerMeter.On("Name").Return("mock-cpu")
			monitor := NewPowerMonitor(mockPowerMeter, tt.opts...)

			// Check if monitor is correctly initialized
			assert.NotNil(t, monitor)
			assert.Equal(t, tt.want, monitor.Name())
			assert.NotNil(t, monitor.dataCh)
			assert.NotNil(t, monitor.snapshot)
			assert.NotNil(t, monitor.logger)
			assert.NotNil(t, monitor.cpu)
		})
	}
}

var _ device.EnergyZone = (*MockEnergyZone)(nil)

func TestPowerMonitor_Start(t *testing.T) {
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

	// Create a context with a short timeout, shared by the Start methods
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	mockPowerMeter.On("Start", ctx).Return(nil)
	mockPowerMeter.On("Zones").Return(energyZones, nil)

	monitor := NewPowerMonitor(
		mockPowerMeter,
		WithLogger(slog.Default()),
	)

	// Start the monitor and verify it blocks until context cancellation
	startTime := time.Now()
	err := monitor.Start(ctx)
	duration := time.Since(startTime)

	// Verify the method returns when context is done and there's no error
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, duration, 50*time.Millisecond, "Start should block until context is done")

	mockPowerMeter.AssertExpectations(t)
}

func TestPowerMonitor_Stop(t *testing.T) {
	mockPowerMeter := &MockCPUPowerMeter{}
	mockPowerMeter.On("Stop").Return(nil)

	monitor := NewPowerMonitor(mockPowerMeter)
	err := monitor.Stop()
	assert.NoError(t, err)
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

	mockPowerMeter.On("Start").Return(nil)
	mockPowerMeter.On("Zones").Return(energyZones, nil)

	monitor := NewPowerMonitor(mockPowerMeter)

	snapshot, err := monitor.Snapshot()
	assert.NotNil(t, snapshot)
	assert.Nil(t, err)

	// ensure that snapshot is a clone
	assert.NotSame(t, monitor.snapshot, snapshot)
	assert.Equal(t, monitor.snapshot, snapshot)
}
