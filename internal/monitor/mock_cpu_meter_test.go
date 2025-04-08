/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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
