// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package monitor

import (
	"context"
	"log"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
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
	mockPowerMeter.On("Init", mock.Anything).Return(nil)

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
			// Create a context with a short timeout, shared by the Init methods
			ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
			defer cancel()

			monitor := NewPowerMonitor(
				powerMeter,
				WithLogger(slog.Default()),
			)

			err := monitor.Init(ctx)
			assert.NoError(t, err)
			assert.Equal(t, monitor.ZoneNames(), zoneNamesFromMeter(powerMeter))
		})
	}
	mockPowerMeter.AssertExpectations(t)
}

func TestPowerMonitor_Shutdown(t *testing.T) {
	mockPowerMeter := &MockCPUPowerMeter{}
	mockPowerMeter.On("Stop").Return(nil)

	monitor := NewPowerMonitor(mockPowerMeter)
	err := monitor.Shutdown()
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

	mockPowerMeter.On("Init").Return(nil)
	mockPowerMeter.On("Zones").Return(energyZones, nil)

	monitor := NewPowerMonitor(mockPowerMeter)

	snapshot, err := monitor.Snapshot()
	assert.NotNil(t, snapshot)
	assert.Nil(t, err)

	// ensure that snapshot is a clone
	assert.NotSame(t, monitor.snapshot, snapshot)
	assert.Equal(t, monitor.snapshot, snapshot)
}
