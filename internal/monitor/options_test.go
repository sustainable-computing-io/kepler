// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package monitor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/sustainable-computing-io/kepler/internal/device"
	"github.com/sustainable-computing-io/kepler/internal/device/gpu"
)

// MockGPUPowerMeter is a mock implementation of gpu.GPUPowerMeter
type MockGPUPowerMeter struct {
	mock.Mock
}

func (m *MockGPUPowerMeter) Name() string {
	return "mock-gpu"
}

func (m *MockGPUPowerMeter) Init() error {
	return nil
}

func (m *MockGPUPowerMeter) Shutdown() error {
	return nil
}

func (m *MockGPUPowerMeter) Vendor() gpu.Vendor {
	args := m.Called()
	return args.Get(0).(gpu.Vendor)
}

func (m *MockGPUPowerMeter) Devices() []gpu.GPUDevice {
	args := m.Called()
	return args.Get(0).([]gpu.GPUDevice)
}

func (m *MockGPUPowerMeter) GetPowerUsage(deviceIndex int) (device.Power, error) {
	args := m.Called(deviceIndex)
	return args.Get(0).(device.Power), args.Error(1)
}

func (m *MockGPUPowerMeter) GetTotalEnergy(deviceIndex int) (device.Energy, error) {
	args := m.Called(deviceIndex)
	return args.Get(0).(device.Energy), args.Error(1)
}

func (m *MockGPUPowerMeter) GetProcessPower() (map[uint32]float64, error) {
	args := m.Called()
	return args.Get(0).(map[uint32]float64), args.Error(1)
}

func (m *MockGPUPowerMeter) GetDevicePowerStats(deviceIndex int) (gpu.GPUPowerStats, error) {
	args := m.Called(deviceIndex)
	return args.Get(0).(gpu.GPUPowerStats), args.Error(1)
}

func (m *MockGPUPowerMeter) GetProcessInfo() ([]gpu.ProcessGPUInfo, error) {
	args := m.Called()
	return args.Get(0).([]gpu.ProcessGPUInfo), args.Error(1)
}

// TestWithMaxTerminated tests the WithMaxTerminated option function
func TestWithMaxTerminated(t *testing.T) {
	tests := []struct {
		name     string
		maxValue int
	}{
		{"zero", 0},
		{"positive", 100},
		{"unlimited", -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := DefaultOpts()
			option := WithMaxTerminated(tt.maxValue)
			option(&opts)
			assert.Equal(t, tt.maxValue, opts.maxTerminated)
		})
	}
}

// TestWithGPUPowerMeters tests the WithGPUPowerMeters option function
// with 0, 1, and N GPU meters to support multi-vendor scenarios
func TestWithGPUPowerMeters(t *testing.T) {
	t.Run("zero meters (no GPUs on node)", func(t *testing.T) {
		opts := DefaultOpts()
		option := WithGPUPowerMeters(nil)
		option(&opts)
		assert.Nil(t, opts.gpuMeters)
		assert.Len(t, opts.gpuMeters, 0)
	})

	t.Run("empty slice (no GPUs on node)", func(t *testing.T) {
		opts := DefaultOpts()
		option := WithGPUPowerMeters([]gpu.GPUPowerMeter{})
		option(&opts)
		assert.NotNil(t, opts.gpuMeters)
		assert.Len(t, opts.gpuMeters, 0)
	})

	t.Run("one meter (single vendor)", func(t *testing.T) {
		mockMeter := new(MockGPUPowerMeter)

		opts := DefaultOpts()
		option := WithGPUPowerMeters([]gpu.GPUPowerMeter{mockMeter})
		option(&opts)
		assert.Len(t, opts.gpuMeters, 1)
		assert.Same(t, mockMeter, opts.gpuMeters[0])
	})

	t.Run("multiple meters (multi-vendor: NVIDIA + AMD)", func(t *testing.T) {
		nvidiaMeter := new(MockGPUPowerMeter)
		amdMeter := new(MockGPUPowerMeter)

		opts := DefaultOpts()
		option := WithGPUPowerMeters([]gpu.GPUPowerMeter{nvidiaMeter, amdMeter})
		option(&opts)
		assert.Len(t, opts.gpuMeters, 2)
		assert.Same(t, nvidiaMeter, opts.gpuMeters[0])
		assert.Same(t, amdMeter, opts.gpuMeters[1])
	})

	t.Run("three meters (multi-vendor: NVIDIA + AMD + Intel)", func(t *testing.T) {
		nvidiaMeter := new(MockGPUPowerMeter)
		amdMeter := new(MockGPUPowerMeter)
		intelMeter := new(MockGPUPowerMeter)

		opts := DefaultOpts()
		option := WithGPUPowerMeters([]gpu.GPUPowerMeter{nvidiaMeter, amdMeter, intelMeter})
		option(&opts)
		assert.Len(t, opts.gpuMeters, 3)
	})
}
