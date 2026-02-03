// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package nvidia

import (
	"github.com/stretchr/testify/mock"
	"github.com/sustainable-computing-io/kepler/internal/device"
	"github.com/sustainable-computing-io/kepler/internal/device/gpu"
)

// MockNVMLBackend is a mock implementation of NVMLBackend for testing
type MockNVMLBackend struct {
	mock.Mock
}

func (m *MockNVMLBackend) Init() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockNVMLBackend) Shutdown() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockNVMLBackend) DeviceCount() int {
	args := m.Called()
	return args.Int(0)
}

func (m *MockNVMLBackend) GetDevice(index int) (NVMLDevice, error) {
	args := m.Called(index)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(NVMLDevice), args.Error(1)
}

func (m *MockNVMLBackend) DiscoverDevices() ([]gpu.GPUDevice, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]gpu.GPUDevice), args.Error(1)
}

// MockNVMLDevice is a mock implementation of NVMLDevice for testing
type MockNVMLDevice struct {
	mock.Mock
}

func (m *MockNVMLDevice) Index() int {
	args := m.Called()
	return args.Int(0)
}

func (m *MockNVMLDevice) UUID() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockNVMLDevice) Name() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockNVMLDevice) GetPowerUsage() (device.Power, error) {
	args := m.Called()
	return args.Get(0).(device.Power), args.Error(1)
}

func (m *MockNVMLDevice) GetTotalEnergy() (device.Energy, error) {
	args := m.Called()
	return args.Get(0).(device.Energy), args.Error(1)
}

func (m *MockNVMLDevice) GetComputeRunningProcesses() ([]gpu.ProcessGPUInfo, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]gpu.ProcessGPUInfo), args.Error(1)
}

func (m *MockNVMLDevice) GetProcessUtilization(lastSeen uint64) ([]gpu.ProcessUtilization, error) {
	args := m.Called(lastSeen)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]gpu.ProcessUtilization), args.Error(1)
}

func (m *MockNVMLDevice) GetComputeMode() (ComputeMode, error) {
	args := m.Called()
	return args.Get(0).(ComputeMode), args.Error(1)
}

func (m *MockNVMLDevice) IsMIGEnabled() (bool, error) {
	args := m.Called()
	return args.Bool(0), args.Error(1)
}

func (m *MockNVMLDevice) GetMIGInstances() ([]MIGInstance, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]MIGInstance), args.Error(1)
}

func (m *MockNVMLDevice) GetMIGDeviceByInstanceID(gpuInstanceID uint) (NVMLDevice, error) {
	args := m.Called(gpuInstanceID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(NVMLDevice), args.Error(1)
}

func (m *MockNVMLDevice) GetMaxMigDeviceCount() (int, error) {
	args := m.Called()
	return args.Int(0), args.Error(1)
}

// Verify interface implementations
var _ NVMLBackend = (*MockNVMLBackend)(nil)
var _ NVMLDevice = (*MockNVMLDevice)(nil)
