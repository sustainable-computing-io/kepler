// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package monitor

import (
	"context"

	"github.com/stretchr/testify/mock"
)

// MockCPUPowerMeter is a mock implementation of CPUPowerMeter
type MockCPUPowerMeter struct {
	mock.Mock
}

func (m *MockCPUPowerMeter) Zones() ([]EnergyZone, error) {
	args := m.Called()
	return args.Get(0).([]EnergyZone), args.Error(1)
}

func (m *MockCPUPowerMeter) Name() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockCPUPowerMeter) Run(ctx context.Context) error {
	args := m.Called(ctx)
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

func (m *MockEnergyZone) Energy() (Energy, error) {
	args := m.Called()
	return args.Get(0).(Energy), args.Error(1)
}

func (m *MockEnergyZone) MaxEnergy() Energy {
	args := m.Called()
	return args.Get(0).(Energy)
}
