// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package monitor

import (
	"context"

	"github.com/stretchr/testify/mock"
	"github.com/sustainable-computing-io/kepler/internal/device"
)

// MockCPUPowerMeter is a mock implementation of device.CPUPowerMeter
type MockCPUPowerMeter struct {
	mock.Mock
}

func (m *MockCPUPowerMeter) Zones() ([]device.EnergyZone, error) {
	args := m.Called()
	return args.Get(0).([]device.EnergyZone), args.Error(1)
}

func (m *MockCPUPowerMeter) Name() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockCPUPowerMeter) Start(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockCPUPowerMeter) Stop() error {
	args := m.Called()
	return args.Error(0)
}

type MockEnergyZone struct {
	mock.Mock
}

func (m *MockEnergyZone) Name() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockEnergyZone) Index() int {
	args := m.Called()
	return args.Int(0)
}

func (m *MockEnergyZone) Path() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockEnergyZone) Energy() device.Energy {
	args := m.Called()
	return args.Get(0).(device.Energy)
}

func (m *MockEnergyZone) MaxEnergy() device.Energy {
	args := m.Called()
	return args.Get(0).(device.Energy)
}
