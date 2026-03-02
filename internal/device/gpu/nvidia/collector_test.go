// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package nvidia

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/sustainable-computing-io/kepler/internal/device"
	"github.com/sustainable-computing-io/kepler/internal/device/gpu"
)

func TestNewGPUPowerCollector(t *testing.T) {
	t.Run("with logger", func(t *testing.T) {
		collector, err := NewGPUPowerCollector(slog.Default())
		assert.NoError(t, err)
		assert.NotNil(t, collector)
		assert.NotNil(t, collector.nvml)
		assert.NotNil(t, collector.minObservedPower)
		assert.NotNil(t, collector.idleObserved)
		assert.NotNil(t, collector.sharingModes)
	})

	t.Run("with nil logger uses default", func(t *testing.T) {
		collector, err := NewGPUPowerCollector(nil)
		assert.NoError(t, err)
		assert.NotNil(t, collector)
		assert.NotNil(t, collector.logger)
	})
}

func TestGPUPowerCollector_Name(t *testing.T) {
	collector := &GPUPowerCollector{}
	assert.Equal(t, "nvidia-gpu-power-collector", collector.Name())
}

func TestGPUPowerCollector_Vendor(t *testing.T) {
	collector := &GPUPowerCollector{}
	assert.Equal(t, gpu.VendorNVIDIA, collector.Vendor())
}

func TestGPUPowerCollector_Init(t *testing.T) {
	t.Run("successful init", func(t *testing.T) {
		mockBackend := new(MockNVMLBackend)
		mockDevice := new(MockNVMLDevice)

		mockBackend.On("Init").Return(nil)
		mockBackend.On("DiscoverDevices").Return([]gpu.GPUDevice{
			{Index: 0, UUID: "GPU-123", Name: "Test GPU", Vendor: gpu.VendorNVIDIA},
		}, nil)
		mockBackend.On("DeviceCount").Return(1)
		mockBackend.On("GetDevice", 0).Return(mockDevice, nil)
		mockDevice.On("IsMIGEnabled").Return(false, nil)
		mockDevice.On("GetComputeMode").Return(ComputeModeDefault, nil)

		collector := &GPUPowerCollector{
			logger:           slog.Default(),
			nvml:             mockBackend,
			minObservedPower: make(map[string]float64),
			idleObserved:     make(map[string]bool),
			sharingModes:     make(map[int]gpu.SharingMode),
		}

		err := collector.Init()

		assert.NoError(t, err)
		assert.Len(t, collector.devices, 1)
		assert.Equal(t, gpu.SharingModeTimeSlicing, collector.sharingModes[0])

		mockBackend.AssertExpectations(t)
		mockDevice.AssertExpectations(t)
	})

	t.Run("init failure", func(t *testing.T) {
		mockBackend := new(MockNVMLBackend)
		mockBackend.On("Init").Return(gpu.ErrGPUNotInitialized{})

		collector := &GPUPowerCollector{
			nvml:             mockBackend,
			minObservedPower: make(map[string]float64),
			idleObserved:     make(map[string]bool),
			sharingModes:     make(map[int]gpu.SharingMode),
		}

		err := collector.Init()

		assert.Error(t, err)

		mockBackend.AssertExpectations(t)
	})

	t.Run("discover devices failure", func(t *testing.T) {
		mockBackend := new(MockNVMLBackend)
		mockBackend.On("Init").Return(nil)
		mockBackend.On("DiscoverDevices").Return(nil, gpu.ErrGPUNotInitialized{})

		collector := &GPUPowerCollector{
			logger:           slog.Default(),
			nvml:             mockBackend,
			minObservedPower: make(map[string]float64),
			idleObserved:     make(map[string]bool),
			sharingModes:     make(map[int]gpu.SharingMode),
		}

		err := collector.Init()

		assert.Error(t, err)

		mockBackend.AssertExpectations(t)
	})

	t.Run("detect modes failure is non-fatal", func(t *testing.T) {
		mockBackend := new(MockNVMLBackend)

		mockBackend.On("Init").Return(nil)
		mockBackend.On("DiscoverDevices").Return([]gpu.GPUDevice{
			{Index: 0, UUID: "GPU-123", Name: "Test GPU", Vendor: gpu.VendorNVIDIA},
		}, nil)
		mockBackend.On("DeviceCount").Return(1)
		mockBackend.On("GetDevice", 0).Return(nil, gpu.ErrGPUNotFound{DeviceIndex: 0})

		collector := &GPUPowerCollector{
			logger:           slog.Default(),
			nvml:             mockBackend,
			minObservedPower: make(map[string]float64),
			idleObserved:     make(map[string]bool),
			sharingModes:     make(map[int]gpu.SharingMode),
		}

		err := collector.Init()

		// Init should succeed even if mode detection fails
		assert.NoError(t, err)
		assert.Len(t, collector.devices, 1)

		mockBackend.AssertExpectations(t)
	})

	t.Run("idempotent - second Init is no-op", func(t *testing.T) {
		mockBackend := new(MockNVMLBackend)
		mockDevice := new(MockNVMLDevice)

		// Only expect one call to each — second Init should be a no-op
		mockBackend.On("Init").Return(nil).Once()
		mockBackend.On("DiscoverDevices").Return([]gpu.GPUDevice{
			{Index: 0, UUID: "GPU-123", Name: "Test GPU", Vendor: gpu.VendorNVIDIA},
		}, nil).Once()
		mockBackend.On("DeviceCount").Return(1).Once()
		mockBackend.On("GetDevice", 0).Return(mockDevice, nil).Once()
		mockDevice.On("IsMIGEnabled").Return(false, nil).Once()
		mockDevice.On("GetComputeMode").Return(ComputeModeDefault, nil).Once()

		collector := &GPUPowerCollector{
			logger:           slog.Default(),
			nvml:             mockBackend,
			minObservedPower: make(map[string]float64),
			idleObserved:     make(map[string]bool),
			sharingModes:     make(map[int]gpu.SharingMode),
		}

		err := collector.Init()
		assert.NoError(t, err)
		assert.True(t, collector.initialized)

		// Second call should be a no-op
		err = collector.Init()
		assert.NoError(t, err)

		mockBackend.AssertExpectations(t)
		mockDevice.AssertExpectations(t)
	})
}

func TestGPUPowerCollector_Shutdown(t *testing.T) {
	mockBackend := new(MockNVMLBackend)
	mockBackend.On("Shutdown").Return(nil)

	collector := &GPUPowerCollector{
		nvml: mockBackend,
	}

	err := collector.Shutdown()

	assert.NoError(t, err)
	mockBackend.AssertExpectations(t)
}

func TestGPUPowerCollector_Devices(t *testing.T) {
	devices := []gpu.GPUDevice{
		{Index: 0, UUID: "GPU-123", Name: "Test GPU 0", Vendor: gpu.VendorNVIDIA},
		{Index: 1, UUID: "GPU-456", Name: "Test GPU 1", Vendor: gpu.VendorNVIDIA},
	}

	collector := &GPUPowerCollector{
		devices: devices,
	}

	result := collector.Devices()

	assert.Equal(t, devices, result)
}

func TestGPUPowerCollector_GetPowerUsage(t *testing.T) {
	t.Run("successful power reading", func(t *testing.T) {
		mockBackend := new(MockNVMLBackend)
		mockDevice := new(MockNVMLDevice)

		expectedPower := device.Power(100 * device.Watt)

		mockBackend.On("GetDevice", 0).Return(mockDevice, nil)
		mockDevice.On("GetPowerUsage").Return(expectedPower, nil)

		collector := &GPUPowerCollector{
			nvml: mockBackend,
		}

		power, err := collector.GetPowerUsage(0)

		assert.NoError(t, err)
		assert.Equal(t, expectedPower, power)

		mockBackend.AssertExpectations(t)
		mockDevice.AssertExpectations(t)
	})

	t.Run("device not found", func(t *testing.T) {
		mockBackend := new(MockNVMLBackend)
		mockBackend.On("GetDevice", 99).Return(nil, gpu.ErrGPUNotFound{DeviceIndex: 99})

		collector := &GPUPowerCollector{
			nvml: mockBackend,
		}

		_, err := collector.GetPowerUsage(99)

		assert.Error(t, err)

		mockBackend.AssertExpectations(t)
	})

	t.Run("power reading error", func(t *testing.T) {
		mockBackend := new(MockNVMLBackend)
		mockDevice := new(MockNVMLDevice)

		mockBackend.On("GetDevice", 0).Return(mockDevice, nil)
		mockDevice.On("GetPowerUsage").Return(device.Power(0), errors.New("NVML error"))

		collector := &GPUPowerCollector{
			nvml: mockBackend,
		}

		_, err := collector.GetPowerUsage(0)

		assert.Error(t, err)

		mockBackend.AssertExpectations(t)
		mockDevice.AssertExpectations(t)
	})
}

func TestGPUPowerCollector_GetTotalEnergy(t *testing.T) {
	t.Run("successful energy reading", func(t *testing.T) {
		mockBackend := new(MockNVMLBackend)
		mockDevice := new(MockNVMLDevice)

		expectedEnergy := device.Energy(1000 * device.Joule)

		mockBackend.On("GetDevice", 0).Return(mockDevice, nil)
		mockDevice.On("GetTotalEnergy").Return(expectedEnergy, nil)

		collector := &GPUPowerCollector{
			nvml: mockBackend,
		}

		energy, err := collector.GetTotalEnergy(0)

		assert.NoError(t, err)
		assert.Equal(t, expectedEnergy, energy)

		mockBackend.AssertExpectations(t)
		mockDevice.AssertExpectations(t)
	})

	t.Run("device not found", func(t *testing.T) {
		mockBackend := new(MockNVMLBackend)
		mockBackend.On("GetDevice", 99).Return(nil, gpu.ErrGPUNotFound{DeviceIndex: 99})

		collector := &GPUPowerCollector{
			nvml: mockBackend,
		}

		_, err := collector.GetTotalEnergy(99)

		assert.Error(t, err)

		mockBackend.AssertExpectations(t)
	})
}

func TestGPUPowerCollector_GetDevicePowerStats(t *testing.T) {
	t.Run("calculates idle and active power when idle observed", func(t *testing.T) {
		mockBackend := new(MockNVMLBackend)
		mockDevice := new(MockNVMLDevice)

		// Pre-populate minimum observed power and mark idle as observed
		collector := &GPUPowerCollector{
			logger: slog.Default(),
			nvml:   mockBackend,
			minObservedPower: map[string]float64{
				"GPU-123": 40.0, // 40W idle
			},
			idleObserved: map[string]bool{
				"GPU-123": true,
			},
		}

		mockBackend.On("GetDevice", 0).Return(mockDevice, nil)
		mockDevice.On("GetPowerUsage").Return(device.Power(100*device.Watt), nil)
		mockDevice.On("UUID").Return("GPU-123")
		// Processes running — min should not update
		mockDevice.On("GetComputeRunningProcesses").Return([]gpu.ProcessGPUInfo{
			{PID: 1234},
		}, nil)

		stats, err := collector.GetDevicePowerStats(0)

		assert.NoError(t, err)
		assert.Equal(t, 100.0, stats.TotalPower)
		assert.Equal(t, 40.0, stats.IdlePower)
		assert.Equal(t, 60.0, stats.ActivePower)

		mockBackend.AssertExpectations(t)
		mockDevice.AssertExpectations(t)
	})

	t.Run("idle power only updated when no processes running", func(t *testing.T) {
		mockBackend := new(MockNVMLBackend)
		mockDevice := new(MockNVMLDevice)

		collector := &GPUPowerCollector{
			logger:           slog.Default(),
			nvml:             mockBackend,
			minObservedPower: make(map[string]float64),
			idleObserved:     make(map[string]bool),
		}

		mockBackend.On("GetDevice", 0).Return(mockDevice, nil)
		mockDevice.On("UUID").Return("GPU-123")

		// First call: 100W with processes running — should NOT set idle
		mockDevice.On("GetPowerUsage").Return(device.Power(100*device.Watt), nil).Once()
		mockDevice.On("GetComputeRunningProcesses").Return([]gpu.ProcessGPUInfo{
			{PID: 1234},
		}, nil).Once()

		stats, err := collector.GetDevicePowerStats(0)
		assert.NoError(t, err)
		assert.Equal(t, 100.0, stats.TotalPower)
		assert.Equal(t, 0.0, stats.IdlePower) // No idle observed yet
		assert.Equal(t, 100.0, stats.ActivePower)
		assert.False(t, collector.idleObserved["GPU-123"])

		// Second call: 50W with NO processes — should set idle baseline
		mockDevice.On("GetPowerUsage").Return(device.Power(50*device.Watt), nil).Once()
		mockDevice.On("GetComputeRunningProcesses").Return([]gpu.ProcessGPUInfo{}, nil).Once()

		stats, err = collector.GetDevicePowerStats(0)
		assert.NoError(t, err)
		assert.Equal(t, 50.0, stats.TotalPower)
		assert.Equal(t, 50.0, stats.IdlePower)
		assert.Equal(t, 0.0, stats.ActivePower)
		assert.True(t, collector.idleObserved["GPU-123"])
		assert.Equal(t, 50.0, collector.minObservedPower["GPU-123"])

		// Third call: 120W with processes — idle baseline stays at 50W
		mockDevice.On("GetPowerUsage").Return(device.Power(120*device.Watt), nil).Once()
		mockDevice.On("GetComputeRunningProcesses").Return([]gpu.ProcessGPUInfo{
			{PID: 5678},
		}, nil).Once()

		stats, err = collector.GetDevicePowerStats(0)
		assert.NoError(t, err)
		assert.Equal(t, 120.0, stats.TotalPower)
		assert.Equal(t, 50.0, stats.IdlePower)
		assert.Equal(t, 70.0, stats.ActivePower)

		mockBackend.AssertExpectations(t)
		mockDevice.AssertExpectations(t)
	})

	t.Run("default idle power used before true idle observed", func(t *testing.T) {
		mockBackend := new(MockNVMLBackend)
		mockDevice := new(MockNVMLDevice)

		collector := &GPUPowerCollector{
			logger:           slog.Default(),
			nvml:             mockBackend,
			minObservedPower: make(map[string]float64),
			idleObserved:     make(map[string]bool),
			idlePower:        30.0, // User configured 30W default
		}

		mockBackend.On("GetDevice", 0).Return(mockDevice, nil)
		mockDevice.On("GetPowerUsage").Return(device.Power(100*device.Watt), nil)
		mockDevice.On("UUID").Return("GPU-123")
		mockDevice.On("GetComputeRunningProcesses").Return([]gpu.ProcessGPUInfo{
			{PID: 1234},
		}, nil)

		stats, err := collector.GetDevicePowerStats(0)

		assert.NoError(t, err)
		assert.Equal(t, 100.0, stats.TotalPower)
		assert.Equal(t, 30.0, stats.IdlePower) // Uses configured default
		assert.Equal(t, 70.0, stats.ActivePower)

		mockBackend.AssertExpectations(t)
		mockDevice.AssertExpectations(t)
	})

	t.Run("default idle power takes precedence over observed", func(t *testing.T) {
		mockBackend := new(MockNVMLBackend)
		mockDevice := new(MockNVMLDevice)

		collector := &GPUPowerCollector{
			logger: slog.Default(),
			nvml:   mockBackend,
			minObservedPower: map[string]float64{
				"GPU-123": 40.0,
			},
			idleObserved: map[string]bool{
				"GPU-123": true,
			},
			idlePower: 30.0, // User configured default takes precedence
		}

		mockBackend.On("GetDevice", 0).Return(mockDevice, nil)
		mockDevice.On("GetPowerUsage").Return(device.Power(100*device.Watt), nil)
		mockDevice.On("UUID").Return("GPU-123")
		mockDevice.On("GetComputeRunningProcesses").Return([]gpu.ProcessGPUInfo{
			{PID: 1234},
		}, nil)

		stats, err := collector.GetDevicePowerStats(0)

		assert.NoError(t, err)
		assert.Equal(t, 30.0, stats.IdlePower) // Configured default, not observed 40W

		mockBackend.AssertExpectations(t)
		mockDevice.AssertExpectations(t)
	})

	t.Run("GetComputeRunningProcesses error is non-fatal", func(t *testing.T) {
		mockBackend := new(MockNVMLBackend)
		mockDevice := new(MockNVMLDevice)

		collector := &GPUPowerCollector{
			logger: slog.Default(),
			nvml:   mockBackend,
			minObservedPower: map[string]float64{
				"GPU-123": 40.0,
			},
			idleObserved: map[string]bool{
				"GPU-123": true,
			},
		}

		mockBackend.On("GetDevice", 0).Return(mockDevice, nil)
		mockDevice.On("GetPowerUsage").Return(device.Power(100*device.Watt), nil)
		mockDevice.On("UUID").Return("GPU-123")
		mockDevice.On("GetComputeRunningProcesses").Return(nil, errors.New("NVML error"))

		stats, err := collector.GetDevicePowerStats(0)

		// Should still succeed — just skip idle detection this round
		assert.NoError(t, err)
		assert.Equal(t, 100.0, stats.TotalPower)
		assert.Equal(t, 40.0, stats.IdlePower) // Uses previously observed
		assert.Equal(t, 60.0, stats.ActivePower)

		mockBackend.AssertExpectations(t)
		mockDevice.AssertExpectations(t)
	})

	t.Run("active power clamped to zero", func(t *testing.T) {
		mockBackend := new(MockNVMLBackend)
		mockDevice := new(MockNVMLDevice)

		collector := &GPUPowerCollector{
			logger:           slog.Default(),
			nvml:             mockBackend,
			minObservedPower: make(map[string]float64),
			idleObserved:     make(map[string]bool),
			idlePower:        100.0, // Default higher than actual reading
		}

		mockBackend.On("GetDevice", 0).Return(mockDevice, nil)
		mockDevice.On("GetPowerUsage").Return(device.Power(80*device.Watt), nil)
		mockDevice.On("UUID").Return("GPU-123")
		mockDevice.On("GetComputeRunningProcesses").Return([]gpu.ProcessGPUInfo{
			{PID: 1234},
		}, nil)

		stats, err := collector.GetDevicePowerStats(0)

		assert.NoError(t, err)
		assert.Equal(t, 80.0, stats.TotalPower)
		assert.Equal(t, 100.0, stats.IdlePower)
		assert.Equal(t, 0.0, stats.ActivePower) // Clamped to 0

		mockBackend.AssertExpectations(t)
		mockDevice.AssertExpectations(t)
	})

	t.Run("device not found", func(t *testing.T) {
		mockBackend := new(MockNVMLBackend)
		mockBackend.On("GetDevice", 99).Return(nil, gpu.ErrGPUNotFound{DeviceIndex: 99})

		collector := &GPUPowerCollector{
			nvml:             mockBackend,
			minObservedPower: make(map[string]float64),
			idleObserved:     make(map[string]bool),
		}

		_, err := collector.GetDevicePowerStats(99)

		assert.Error(t, err)

		mockBackend.AssertExpectations(t)
	})
}

func TestGPUPowerCollector_GetProcessPower(t *testing.T) {
	t.Run("exclusive mode attribution", func(t *testing.T) {
		mockBackend := new(MockNVMLBackend)
		mockDevice := new(MockNVMLDevice)

		collector := &GPUPowerCollector{
			logger: slog.Default(),
			nvml:   mockBackend,
			devices: []gpu.GPUDevice{
				{Index: 0, UUID: "GPU-123"},
			},
			sharingModes: map[int]gpu.SharingMode{
				0: gpu.SharingModeExclusive,
			},
			minObservedPower: map[string]float64{
				"GPU-123": 40.0,
			},
			idleObserved: map[string]bool{
				"GPU-123": true,
			},
		}

		mockBackend.On("GetDevice", 0).Return(mockDevice, nil)
		mockDevice.On("GetPowerUsage").Return(device.Power(100*device.Watt), nil)
		mockDevice.On("UUID").Return("GPU-123")
		mockDevice.On("GetComputeRunningProcesses").Return([]gpu.ProcessGPUInfo{
			{PID: 1234, DeviceIndex: 0},
		}, nil)

		result, err := collector.GetProcessPower()

		assert.NoError(t, err)
		assert.Len(t, result, 1)
		assert.Equal(t, 60.0, result[1234]) // 100W - 40W idle = 60W active

		mockBackend.AssertExpectations(t)
		mockDevice.AssertExpectations(t)
	})

	t.Run("time slicing mode attribution", func(t *testing.T) {
		mockBackend := new(MockNVMLBackend)
		mockDevice := new(MockNVMLDevice)

		collector := &GPUPowerCollector{
			logger: slog.Default(),
			nvml:   mockBackend,
			devices: []gpu.GPUDevice{
				{Index: 0, UUID: "GPU-123"},
			},
			sharingModes: map[int]gpu.SharingMode{
				0: gpu.SharingModeTimeSlicing,
			},
			minObservedPower: map[string]float64{
				"GPU-123": 40.0,
			},
			idleObserved: map[string]bool{
				"GPU-123": true,
			},
		}

		mockBackend.On("GetDevice", 0).Return(mockDevice, nil)
		mockDevice.On("GetPowerUsage").Return(device.Power(100*device.Watt), nil)
		mockDevice.On("UUID").Return("GPU-123")
		mockDevice.On("GetComputeRunningProcesses").Return([]gpu.ProcessGPUInfo{
			{PID: 1001},
			{PID: 1002},
		}, nil)
		mockDevice.On("GetProcessUtilization", mock.Anything).Return([]gpu.ProcessUtilization{
			{PID: 1001, ComputeUtil: 60, Timestamp: 100}, // 60% SM util
			{PID: 1002, ComputeUtil: 40, Timestamp: 100}, // 40% SM util
		}, nil)

		result, err := collector.GetProcessPower()

		assert.NoError(t, err)
		assert.Len(t, result, 2)
		// Active power = 60W, distributed by SM utilization
		assert.InDelta(t, 36.0, result[1001], 0.01) // 60% of 60W = 36W
		assert.InDelta(t, 24.0, result[1002], 0.01) // 40% of 60W = 24W

		mockBackend.AssertExpectations(t)
		mockDevice.AssertExpectations(t)
	})

	t.Run("time slicing fallback to equal distribution", func(t *testing.T) {
		mockBackend := new(MockNVMLBackend)
		mockDevice := new(MockNVMLDevice)

		collector := &GPUPowerCollector{
			logger: slog.Default(),
			nvml:   mockBackend,
			devices: []gpu.GPUDevice{
				{Index: 0, UUID: "GPU-123"},
			},
			sharingModes: map[int]gpu.SharingMode{
				0: gpu.SharingModeTimeSlicing,
			},
			minObservedPower: map[string]float64{
				"GPU-123": 40.0,
			},
			idleObserved: map[string]bool{
				"GPU-123": true,
			},
		}

		mockBackend.On("GetDevice", 0).Return(mockDevice, nil)
		mockDevice.On("GetPowerUsage").Return(device.Power(100*device.Watt), nil)
		mockDevice.On("UUID").Return("GPU-123")
		// GetProcessUtilization fails, falls back to equal distribution
		mockDevice.On("GetProcessUtilization", mock.Anything).Return(nil, gpu.ErrProcessUtilizationUnavailable{})
		mockDevice.On("GetComputeRunningProcesses").Return([]gpu.ProcessGPUInfo{
			{PID: 1001, DeviceIndex: 0},
			{PID: 1002, DeviceIndex: 0},
		}, nil)

		result, err := collector.GetProcessPower()

		assert.NoError(t, err)
		assert.Len(t, result, 2)
		// Active power = 60W, split equally
		assert.Equal(t, 30.0, result[1001])
		assert.Equal(t, 30.0, result[1002])

		mockBackend.AssertExpectations(t)
		mockDevice.AssertExpectations(t)
	})

	t.Run("no active power returns empty result when no processes", func(t *testing.T) {
		mockBackend := new(MockNVMLBackend)
		mockDevice := new(MockNVMLDevice)

		// Idle power equals total power - no active work
		collector := &GPUPowerCollector{
			logger: slog.Default(),
			nvml:   mockBackend,
			devices: []gpu.GPUDevice{
				{Index: 0, UUID: "GPU-123"},
			},
			sharingModes: map[int]gpu.SharingMode{
				0: gpu.SharingModeTimeSlicing,
			},
			minObservedPower: map[string]float64{
				"GPU-123": 100.0, // Same as total
			},
			idleObserved: map[string]bool{
				"GPU-123": true,
			},
		}

		mockBackend.On("GetDevice", 0).Return(mockDevice, nil)
		mockDevice.On("GetPowerUsage").Return(device.Power(100*device.Watt), nil)
		mockDevice.On("UUID").Return("GPU-123")
		// Even with activePower=0, we still check for running processes
		mockDevice.On("GetComputeRunningProcesses").Return([]gpu.ProcessGPUInfo{}, nil)

		result, err := collector.GetProcessPower()

		assert.NoError(t, err)
		assert.Empty(t, result) // No processes to attribute power to

		mockBackend.AssertExpectations(t)
		mockDevice.AssertExpectations(t)
	})

	t.Run("exclusive mode with no running processes", func(t *testing.T) {
		mockBackend := new(MockNVMLBackend)
		mockDevice := new(MockNVMLDevice)

		collector := &GPUPowerCollector{
			logger: slog.Default(),
			nvml:   mockBackend,
			devices: []gpu.GPUDevice{
				{Index: 0, UUID: "GPU-123"},
			},
			sharingModes: map[int]gpu.SharingMode{
				0: gpu.SharingModeExclusive,
			},
			minObservedPower: map[string]float64{
				"GPU-123": 40.0,
			},
			idleObserved: map[string]bool{
				"GPU-123": true,
			},
		}

		mockBackend.On("GetDevice", 0).Return(mockDevice, nil)
		mockDevice.On("GetPowerUsage").Return(device.Power(100*device.Watt), nil)
		mockDevice.On("UUID").Return("GPU-123")
		mockDevice.On("GetComputeRunningProcesses").Return([]gpu.ProcessGPUInfo{}, nil)

		result, err := collector.GetProcessPower()

		assert.NoError(t, err)
		assert.Empty(t, result) // No processes to attribute power to

		mockBackend.AssertExpectations(t)
		mockDevice.AssertExpectations(t)
	})

	t.Run("time slicing with zero total compute utilization", func(t *testing.T) {
		mockBackend := new(MockNVMLBackend)
		mockDevice := new(MockNVMLDevice)

		collector := &GPUPowerCollector{
			logger: slog.Default(),
			nvml:   mockBackend,
			devices: []gpu.GPUDevice{
				{Index: 0, UUID: "GPU-123"},
			},
			sharingModes: map[int]gpu.SharingMode{
				0: gpu.SharingModeTimeSlicing,
			},
			minObservedPower: map[string]float64{
				"GPU-123": 40.0,
			},
			idleObserved: map[string]bool{
				"GPU-123": true,
			},
		}

		mockBackend.On("GetDevice", 0).Return(mockDevice, nil)
		mockDevice.On("GetPowerUsage").Return(device.Power(100*device.Watt), nil)
		mockDevice.On("UUID").Return("GPU-123")
		mockDevice.On("GetComputeRunningProcesses").Return([]gpu.ProcessGPUInfo{
			{PID: 1001},
			{PID: 1002},
		}, nil)
		mockDevice.On("GetProcessUtilization", mock.Anything).Return([]gpu.ProcessUtilization{
			{PID: 1001, ComputeUtil: 0, Timestamp: 100}, // 0% SM util
			{PID: 1002, ComputeUtil: 0, Timestamp: 100}, // 0% SM util
		}, nil)

		result, err := collector.GetProcessPower()

		assert.NoError(t, err)
		// Zero utilization falls back to equal distribution: 60W / 2 = 30W each
		assert.Len(t, result, 2)
		assert.Equal(t, 30.0, result[1001])
		assert.Equal(t, 30.0, result[1002])

		mockBackend.AssertExpectations(t)
		mockDevice.AssertExpectations(t)
	})

	t.Run("time slicing with empty process utilization", func(t *testing.T) {
		mockBackend := new(MockNVMLBackend)
		mockDevice := new(MockNVMLDevice)

		collector := &GPUPowerCollector{
			logger: slog.Default(),
			nvml:   mockBackend,
			devices: []gpu.GPUDevice{
				{Index: 0, UUID: "GPU-123"},
			},
			sharingModes: map[int]gpu.SharingMode{
				0: gpu.SharingModeTimeSlicing,
			},
			minObservedPower: map[string]float64{
				"GPU-123": 40.0,
			},
			idleObserved: map[string]bool{
				"GPU-123": true,
			},
		}

		mockBackend.On("GetDevice", 0).Return(mockDevice, nil)
		mockDevice.On("GetPowerUsage").Return(device.Power(100*device.Watt), nil)
		mockDevice.On("UUID").Return("GPU-123")
		mockDevice.On("GetComputeRunningProcesses").Return([]gpu.ProcessGPUInfo{
			{PID: 1001},
			{PID: 1002},
		}, nil)
		mockDevice.On("GetProcessUtilization", mock.Anything).Return([]gpu.ProcessUtilization{}, nil)

		result, err := collector.GetProcessPower()

		assert.NoError(t, err)
		// Empty utilization falls back to equal distribution: 60W / 2 = 30W each
		assert.Len(t, result, 2)
		assert.Equal(t, 30.0, result[1001])
		assert.Equal(t, 30.0, result[1002])

		mockBackend.AssertExpectations(t)
		mockDevice.AssertExpectations(t)
	})

	t.Run("partitioned mode with DCGM", func(t *testing.T) {
		mockBackend := new(MockNVMLBackend)
		mockDevice := new(MockNVMLDevice)
		mockDCGM := new(MockDCGMBackend)
		mockMIGDev1 := new(MockNVMLDevice)
		mockMIGDev2 := new(MockNVMLDevice)

		collector := &GPUPowerCollector{
			logger: slog.Default(),
			nvml:   mockBackend,
			devices: []gpu.GPUDevice{
				{Index: 0, UUID: "GPU-123"},
			},
			sharingModes: map[int]gpu.SharingMode{
				0: gpu.SharingModePartitioned,
			},
			minObservedPower: map[string]float64{
				"GPU-123": 40.0,
			},
			idleObserved: map[string]bool{
				"GPU-123": true,
			},
			dcgm: mockDCGM,
			migInstancesByDevice: map[int][]MIGGPUInstance{
				0: {
					{ParentGPUIndex: 0, GPUInstanceID: 1},
					{ParentGPUIndex: 0, GPUInstanceID: 2},
				},
			},
		}

		mockDCGM.On("IsInitialized").Return(true)

		mockBackend.On("GetDevice", 0).Return(mockDevice, nil)
		mockDevice.On("GetPowerUsage").Return(device.Power(100*device.Watt), nil)
		mockDevice.On("UUID").Return("GPU-123")
		mockDevice.On("GetComputeRunningProcesses").Return([]gpu.ProcessGPUInfo{
			{PID: 1001},
		}, nil)

		// Instance 1: activity 0.6, has process 1001 with SmUtil 80
		mockDCGM.On("GetMIGInstanceActivity", 0, uint(1)).Return(0.6, nil)
		mockDevice.On("GetMIGDeviceByInstanceID", uint(1)).Return(mockMIGDev1, nil)
		mockMIGDev1.On("GetProcessUtilization", uint64(0)).Return([]gpu.ProcessUtilization{
			{PID: 1001, ComputeUtil: 80},
		}, nil)

		// Instance 2: activity 0.4, has process 2001 with SmUtil 50
		mockDCGM.On("GetMIGInstanceActivity", 0, uint(2)).Return(0.4, nil)
		mockDevice.On("GetMIGDeviceByInstanceID", uint(2)).Return(mockMIGDev2, nil)
		mockMIGDev2.On("GetProcessUtilization", uint64(0)).Return([]gpu.ProcessUtilization{
			{PID: 2001, ComputeUtil: 50},
		}, nil)

		result, err := collector.GetProcessPower()

		assert.NoError(t, err)
		assert.Len(t, result, 2)
		// Active power = 100 - 40 = 60W
		// Instance 1 gets 60 * (0.6/1.0) = 36W → PID 1001 gets 36W
		// Instance 2 gets 60 * (0.4/1.0) = 24W → PID 2001 gets 24W
		assert.InDelta(t, 36.0, result[1001], 0.01)
		assert.InDelta(t, 24.0, result[2001], 0.01)

		mockBackend.AssertExpectations(t)
		mockDevice.AssertExpectations(t)
		mockDCGM.AssertExpectations(t)
	})

	t.Run("partitioned mode fallback without DCGM", func(t *testing.T) {
		mockBackend := new(MockNVMLBackend)
		mockDevice := new(MockNVMLDevice)

		collector := &GPUPowerCollector{
			logger: slog.Default(),
			nvml:   mockBackend,
			devices: []gpu.GPUDevice{
				{Index: 0, UUID: "GPU-123"},
			},
			sharingModes: map[int]gpu.SharingMode{
				0: gpu.SharingModePartitioned,
			},
			minObservedPower: map[string]float64{
				"GPU-123": 40.0,
			},
			idleObserved: map[string]bool{
				"GPU-123": true,
			},
			dcgm:                 nil, // No DCGM
			migInstancesByDevice: map[int][]MIGGPUInstance{},
		}

		mockBackend.On("GetDevice", 0).Return(mockDevice, nil)
		mockDevice.On("GetPowerUsage").Return(device.Power(100*device.Watt), nil)
		mockDevice.On("UUID").Return("GPU-123")
		mockDevice.On("GetComputeRunningProcesses").Return([]gpu.ProcessGPUInfo{
			{PID: 1001},
			{PID: 2001},
		}, nil)

		result, err := collector.GetProcessPower()

		assert.NoError(t, err)
		assert.Len(t, result, 2)
		// Fallback: equal distribution of 60W among 2 processes
		assert.Equal(t, 30.0, result[1001])
		assert.Equal(t, 30.0, result[2001])

		mockBackend.AssertExpectations(t)
		mockDevice.AssertExpectations(t)
	})
}

func TestGPUPowerCollector_GetProcessInfo(t *testing.T) {
	t.Run("returns processes from all devices", func(t *testing.T) {
		mockBackend := new(MockNVMLBackend)
		mockDevice0 := new(MockNVMLDevice)
		mockDevice1 := new(MockNVMLDevice)

		collector := &GPUPowerCollector{
			nvml: mockBackend,
			devices: []gpu.GPUDevice{
				{Index: 0, UUID: "GPU-0"},
				{Index: 1, UUID: "GPU-1"},
			},
		}

		mockBackend.On("GetDevice", 0).Return(mockDevice0, nil)
		mockBackend.On("GetDevice", 1).Return(mockDevice1, nil)
		mockDevice0.On("GetComputeRunningProcesses").Return([]gpu.ProcessGPUInfo{
			{PID: 1001, DeviceIndex: 0},
		}, nil)
		mockDevice1.On("GetComputeRunningProcesses").Return([]gpu.ProcessGPUInfo{
			{PID: 2001, DeviceIndex: 1},
			{PID: 2002, DeviceIndex: 1},
		}, nil)

		result, err := collector.GetProcessInfo()

		assert.NoError(t, err)
		assert.Len(t, result, 3)

		mockBackend.AssertExpectations(t)
		mockDevice0.AssertExpectations(t)
		mockDevice1.AssertExpectations(t)
	})

	t.Run("continues on device error", func(t *testing.T) {
		mockBackend := new(MockNVMLBackend)
		mockDevice1 := new(MockNVMLDevice)

		collector := &GPUPowerCollector{
			nvml: mockBackend,
			devices: []gpu.GPUDevice{
				{Index: 0, UUID: "GPU-0"},
				{Index: 1, UUID: "GPU-1"},
			},
		}

		mockBackend.On("GetDevice", 0).Return(nil, gpu.ErrGPUNotFound{DeviceIndex: 0})
		mockBackend.On("GetDevice", 1).Return(mockDevice1, nil)
		mockDevice1.On("GetComputeRunningProcesses").Return([]gpu.ProcessGPUInfo{
			{PID: 2001, DeviceIndex: 1},
		}, nil)

		result, err := collector.GetProcessInfo()

		assert.NoError(t, err)
		assert.Len(t, result, 1)
		assert.Equal(t, uint32(2001), result[0].PID)

		mockBackend.AssertExpectations(t)
		mockDevice1.AssertExpectations(t)
	})

	t.Run("continues on GetComputeRunningProcesses error", func(t *testing.T) {
		mockBackend := new(MockNVMLBackend)
		mockDevice0 := new(MockNVMLDevice)
		mockDevice1 := new(MockNVMLDevice)

		collector := &GPUPowerCollector{
			nvml: mockBackend,
			devices: []gpu.GPUDevice{
				{Index: 0, UUID: "GPU-0"},
				{Index: 1, UUID: "GPU-1"},
			},
		}

		mockBackend.On("GetDevice", 0).Return(mockDevice0, nil)
		mockBackend.On("GetDevice", 1).Return(mockDevice1, nil)
		mockDevice0.On("GetComputeRunningProcesses").Return(nil, errors.New("NVML error"))
		mockDevice1.On("GetComputeRunningProcesses").Return([]gpu.ProcessGPUInfo{
			{PID: 2001, DeviceIndex: 1},
		}, nil)

		result, err := collector.GetProcessInfo()

		assert.NoError(t, err)
		assert.Len(t, result, 1)
		assert.Equal(t, uint32(2001), result[0].PID)

		mockBackend.AssertExpectations(t)
		mockDevice0.AssertExpectations(t)
		mockDevice1.AssertExpectations(t)
	})

	t.Run("empty result when no devices", func(t *testing.T) {
		mockBackend := new(MockNVMLBackend)

		collector := &GPUPowerCollector{
			nvml:    mockBackend,
			devices: []gpu.GPUDevice{},
		}

		result, err := collector.GetProcessInfo()

		assert.NoError(t, err)
		assert.Empty(t, result)
	})
}

func TestGPUPowerCollector_GetProcessPower_ErrorPaths(t *testing.T) {
	t.Run("exclusive mode GetDevice error", func(t *testing.T) {
		mockBackend := new(MockNVMLBackend)

		collector := &GPUPowerCollector{
			logger: slog.Default(),
			nvml:   mockBackend,
			devices: []gpu.GPUDevice{
				{Index: 0, UUID: "GPU-123"},
			},
			sharingModes: map[int]gpu.SharingMode{
				0: gpu.SharingModeExclusive,
			},
			minObservedPower: make(map[string]float64),
			idleObserved:     make(map[string]bool),
		}

		mockBackend.On("GetDevice", 0).Return(nil, gpu.ErrGPUNotFound{DeviceIndex: 0})

		result, err := collector.GetProcessPower()

		// GetProcessPower returns no error but logs warning
		assert.NoError(t, err)
		assert.Empty(t, result)

		mockBackend.AssertExpectations(t)
	})

	t.Run("exclusive mode GetComputeRunningProcesses error", func(t *testing.T) {
		mockBackend := new(MockNVMLBackend)
		mockDevice := new(MockNVMLDevice)

		collector := &GPUPowerCollector{
			logger: slog.Default(),
			nvml:   mockBackend,
			devices: []gpu.GPUDevice{
				{Index: 0, UUID: "GPU-123"},
			},
			sharingModes: map[int]gpu.SharingMode{
				0: gpu.SharingModeExclusive,
			},
			minObservedPower: map[string]float64{
				"GPU-123": 40.0,
			},
			idleObserved: map[string]bool{
				"GPU-123": true,
			},
		}

		mockBackend.On("GetDevice", 0).Return(mockDevice, nil)
		mockDevice.On("GetPowerUsage").Return(device.Power(100*device.Watt), nil)
		mockDevice.On("UUID").Return("GPU-123")
		mockDevice.On("GetComputeRunningProcesses").Return(nil, errors.New("NVML error"))

		result, err := collector.GetProcessPower()

		// GetProcessPower returns no error but logs warning
		assert.NoError(t, err)
		assert.Empty(t, result)

		mockBackend.AssertExpectations(t)
		mockDevice.AssertExpectations(t)
	})

	t.Run("exclusive mode GetPowerUsage error in attributeExclusive", func(t *testing.T) {
		mockBackend := new(MockNVMLBackend)
		mockDevice := new(MockNVMLDevice)

		collector := &GPUPowerCollector{
			logger: slog.Default(),
			nvml:   mockBackend,
			devices: []gpu.GPUDevice{
				{Index: 0, UUID: "GPU-123"},
			},
			sharingModes: map[int]gpu.SharingMode{
				0: gpu.SharingModeExclusive,
			},
			minObservedPower: make(map[string]float64),
			idleObserved:     make(map[string]bool),
		}

		// GetDevice succeeds but GetPowerUsage fails
		mockBackend.On("GetDevice", 0).Return(mockDevice, nil)
		mockDevice.On("GetPowerUsage").Return(device.Power(0), errors.New("power reading failed"))

		result, err := collector.GetProcessPower()

		// GetProcessPower returns no error but logs warning
		assert.NoError(t, err)
		assert.Empty(t, result)

		mockBackend.AssertExpectations(t)
		mockDevice.AssertExpectations(t)
	})

	t.Run("time slicing mode GetDevice error", func(t *testing.T) {
		mockBackend := new(MockNVMLBackend)

		collector := &GPUPowerCollector{
			logger: slog.Default(),
			nvml:   mockBackend,
			devices: []gpu.GPUDevice{
				{Index: 0, UUID: "GPU-123"},
			},
			sharingModes: map[int]gpu.SharingMode{
				0: gpu.SharingModeTimeSlicing,
			},
			minObservedPower: make(map[string]float64),
			idleObserved:     make(map[string]bool),
		}

		mockBackend.On("GetDevice", 0).Return(nil, gpu.ErrGPUNotFound{DeviceIndex: 0})

		result, err := collector.GetProcessPower()

		assert.NoError(t, err)
		assert.Empty(t, result)

		mockBackend.AssertExpectations(t)
	})

	t.Run("time slicing mode GetPowerUsage error", func(t *testing.T) {
		mockBackend := new(MockNVMLBackend)
		mockDevice := new(MockNVMLDevice)

		collector := &GPUPowerCollector{
			logger: slog.Default(),
			nvml:   mockBackend,
			devices: []gpu.GPUDevice{
				{Index: 0, UUID: "GPU-123"},
			},
			sharingModes: map[int]gpu.SharingMode{
				0: gpu.SharingModeTimeSlicing,
			},
			minObservedPower: make(map[string]float64),
			idleObserved:     make(map[string]bool),
		}

		mockBackend.On("GetDevice", 0).Return(mockDevice, nil)
		mockDevice.On("GetPowerUsage").Return(device.Power(0), errors.New("NVML error"))

		result, err := collector.GetProcessPower()

		assert.NoError(t, err)
		assert.Empty(t, result)

		mockBackend.AssertExpectations(t)
		mockDevice.AssertExpectations(t)
	})

	t.Run("multiple devices with mixed errors", func(t *testing.T) {
		mockBackend := new(MockNVMLBackend)
		mockDevice1 := new(MockNVMLDevice)

		collector := &GPUPowerCollector{
			logger: slog.Default(),
			nvml:   mockBackend,
			devices: []gpu.GPUDevice{
				{Index: 0, UUID: "GPU-0"},
				{Index: 1, UUID: "GPU-1"},
			},
			sharingModes: map[int]gpu.SharingMode{
				0: gpu.SharingModeExclusive, // Will fail
				1: gpu.SharingModeExclusive, // Will succeed
			},
			minObservedPower: map[string]float64{
				"GPU-0": 40.0,
				"GPU-1": 50.0,
			},
			idleObserved: map[string]bool{
				"GPU-0": true,
				"GPU-1": true,
			},
		}

		// Device 0 fails
		mockBackend.On("GetDevice", 0).Return(nil, gpu.ErrGPUNotFound{DeviceIndex: 0})

		// Device 1 succeeds
		mockBackend.On("GetDevice", 1).Return(mockDevice1, nil)
		mockDevice1.On("GetPowerUsage").Return(device.Power(100*device.Watt), nil)
		mockDevice1.On("UUID").Return("GPU-1")
		mockDevice1.On("GetComputeRunningProcesses").Return([]gpu.ProcessGPUInfo{
			{PID: 2001, DeviceIndex: 1},
		}, nil)

		result, err := collector.GetProcessPower()

		assert.NoError(t, err)
		assert.Len(t, result, 1)
		assert.Equal(t, 50.0, result[2001]) // 100W - 50W idle = 50W

		mockBackend.AssertExpectations(t)
		mockDevice1.AssertExpectations(t)
	})

	t.Run("power accumulates across same PID on multiple devices", func(t *testing.T) {
		mockBackend := new(MockNVMLBackend)
		mockDevice0 := new(MockNVMLDevice)
		mockDevice1 := new(MockNVMLDevice)

		collector := &GPUPowerCollector{
			logger: slog.Default(),
			nvml:   mockBackend,
			devices: []gpu.GPUDevice{
				{Index: 0, UUID: "GPU-0"},
				{Index: 1, UUID: "GPU-1"},
			},
			sharingModes: map[int]gpu.SharingMode{
				0: gpu.SharingModeExclusive,
				1: gpu.SharingModeExclusive,
			},
			minObservedPower: map[string]float64{
				"GPU-0": 40.0,
				"GPU-1": 40.0,
			},
			idleObserved: map[string]bool{
				"GPU-0": true,
				"GPU-1": true,
			},
		}

		// Both devices have same PID (process using multiple GPUs)
		mockBackend.On("GetDevice", 0).Return(mockDevice0, nil)
		mockDevice0.On("GetPowerUsage").Return(device.Power(100*device.Watt), nil)
		mockDevice0.On("UUID").Return("GPU-0")
		mockDevice0.On("GetComputeRunningProcesses").Return([]gpu.ProcessGPUInfo{
			{PID: 1234, DeviceIndex: 0},
		}, nil)

		mockBackend.On("GetDevice", 1).Return(mockDevice1, nil)
		mockDevice1.On("GetPowerUsage").Return(device.Power(100*device.Watt), nil)
		mockDevice1.On("UUID").Return("GPU-1")
		mockDevice1.On("GetComputeRunningProcesses").Return([]gpu.ProcessGPUInfo{
			{PID: 1234, DeviceIndex: 1}, // Same PID
		}, nil)

		result, err := collector.GetProcessPower()

		assert.NoError(t, err)
		assert.Len(t, result, 1)
		// 60W from GPU-0 + 60W from GPU-1 = 120W
		assert.Equal(t, 120.0, result[1234])

		mockBackend.AssertExpectations(t)
		mockDevice0.AssertExpectations(t)
		mockDevice1.AssertExpectations(t)
	})
}

func TestGPUPowerCollector_GetDevicePowerStats_ErrorPaths(t *testing.T) {
	t.Run("GetPowerUsage error", func(t *testing.T) {
		mockBackend := new(MockNVMLBackend)
		mockDevice := new(MockNVMLDevice)

		collector := &GPUPowerCollector{
			nvml:             mockBackend,
			minObservedPower: make(map[string]float64),
			idleObserved:     make(map[string]bool),
		}

		mockBackend.On("GetDevice", 0).Return(mockDevice, nil)
		mockDevice.On("GetPowerUsage").Return(device.Power(0), errors.New("NVML error"))

		_, err := collector.GetDevicePowerStats(0)

		assert.Error(t, err)

		mockBackend.AssertExpectations(t)
		mockDevice.AssertExpectations(t)
	})

	t.Run("first idle observation sets baseline", func(t *testing.T) {
		mockBackend := new(MockNVMLBackend)
		mockDevice := new(MockNVMLDevice)

		collector := &GPUPowerCollector{
			logger:           slog.Default(),
			nvml:             mockBackend,
			minObservedPower: make(map[string]float64),
			idleObserved:     make(map[string]bool),
		}

		mockBackend.On("GetDevice", 0).Return(mockDevice, nil)
		mockDevice.On("GetPowerUsage").Return(device.Power(100*device.Watt), nil)
		mockDevice.On("UUID").Return("GPU-123")
		// No processes running — truly idle
		mockDevice.On("GetComputeRunningProcesses").Return([]gpu.ProcessGPUInfo{}, nil)

		stats, err := collector.GetDevicePowerStats(0)

		assert.NoError(t, err)
		assert.Equal(t, 100.0, stats.TotalPower)
		assert.Equal(t, 100.0, stats.IdlePower) // First idle reading becomes baseline
		assert.Equal(t, 0.0, stats.ActivePower)

		// Verify idle state
		assert.Equal(t, 100.0, collector.minObservedPower["GPU-123"])
		assert.True(t, collector.idleObserved["GPU-123"])

		mockBackend.AssertExpectations(t)
		mockDevice.AssertExpectations(t)
	})

	t.Run("first reading under load does not set idle baseline", func(t *testing.T) {
		mockBackend := new(MockNVMLBackend)
		mockDevice := new(MockNVMLDevice)

		collector := &GPUPowerCollector{
			logger:           slog.Default(),
			nvml:             mockBackend,
			minObservedPower: make(map[string]float64),
			idleObserved:     make(map[string]bool),
		}

		mockBackend.On("GetDevice", 0).Return(mockDevice, nil)
		mockDevice.On("GetPowerUsage").Return(device.Power(200*device.Watt), nil)
		mockDevice.On("UUID").Return("GPU-123")
		// Processes running — NOT idle
		mockDevice.On("GetComputeRunningProcesses").Return([]gpu.ProcessGPUInfo{
			{PID: 1234},
		}, nil)

		stats, err := collector.GetDevicePowerStats(0)

		assert.NoError(t, err)
		assert.Equal(t, 200.0, stats.TotalPower)
		assert.Equal(t, 0.0, stats.IdlePower) // No idle observed, no default
		assert.Equal(t, 200.0, stats.ActivePower)

		// Verify idle NOT set
		assert.Empty(t, collector.minObservedPower)
		assert.False(t, collector.idleObserved["GPU-123"])

		mockBackend.AssertExpectations(t)
		mockDevice.AssertExpectations(t)
	})
}

func TestGPUPowerCollector_SetIdlePower(t *testing.T) {
	collector := &GPUPowerCollector{
		minObservedPower: make(map[string]float64),
		idleObserved:     make(map[string]bool),
	}

	collector.SetIdlePower(50.0)
	assert.Equal(t, 50.0, collector.idlePower)

	collector.SetIdlePower(0)
	assert.Equal(t, 0.0, collector.idlePower)

	// Negative values are clamped to 0
	collector.SetIdlePower(-10.0)
	assert.Equal(t, 0.0, collector.idlePower)
}

// Verify IdlePowerConfigurable interface implementation
var _ gpu.IdlePowerConfigurable = (*GPUPowerCollector)(nil)

func TestGPUPowerCollector_GetTotalEnergy_ErrorPaths(t *testing.T) {
	t.Run("GetTotalEnergy error", func(t *testing.T) {
		mockBackend := new(MockNVMLBackend)
		mockDevice := new(MockNVMLDevice)

		mockBackend.On("GetDevice", 0).Return(mockDevice, nil)
		mockDevice.On("GetTotalEnergy").Return(device.Energy(0), errors.New("NVML error"))

		collector := &GPUPowerCollector{
			nvml: mockBackend,
		}

		_, err := collector.GetTotalEnergy(0)

		assert.Error(t, err)

		mockBackend.AssertExpectations(t)
		mockDevice.AssertExpectations(t)
	})
}

func TestGPUPowerCollector_Init_DetectAllModesErrorPath(t *testing.T) {
	// Test the error path where DetectAllModes fails
	// Note: Current implementation of DetectAllModes never returns an error,
	// it handles device errors internally. This test documents that behavior.
	mockBackend := new(MockNVMLBackend)
	mockDevice := new(MockNVMLDevice)

	mockBackend.On("Init").Return(nil)
	mockBackend.On("DiscoverDevices").Return([]gpu.GPUDevice{
		{Index: 0, UUID: "GPU-123", Name: "Test GPU", Vendor: gpu.VendorNVIDIA},
	}, nil)
	mockBackend.On("DeviceCount").Return(1)
	// GetDevice fails, which causes DetectAllModes to set mode to Unknown
	mockBackend.On("GetDevice", 0).Return(nil, errors.New("device error"))

	collector := &GPUPowerCollector{
		logger:           slog.Default(),
		nvml:             mockBackend,
		minObservedPower: make(map[string]float64),
		idleObserved:     make(map[string]bool),
		sharingModes:     make(map[int]gpu.SharingMode),
	}

	err := collector.Init()

	// Init succeeds even with device errors in mode detection
	assert.NoError(t, err)
	// Mode defaults to Unknown when detection fails
	assert.Equal(t, gpu.SharingModeUnknown, collector.sharingModes[0])

	mockBackend.AssertExpectations(t)
	mockDevice.AssertExpectations(t)
}

func TestRegistration(t *testing.T) {
	// Verify that the nvidia package registers itself on import
	vendors := gpu.RegisteredVendors()

	found := false
	for _, v := range vendors {
		if v == gpu.VendorNVIDIA {
			found = true
			break
		}
	}

	assert.True(t, found, "NVIDIA vendor should be registered")
}

func TestDistributeByUtilization(t *testing.T) {
	t.Run("proportional distribution", func(t *testing.T) {
		result := make(map[uint32]float64)
		processUtil := map[uint32]uint32{
			1001: 60,
			1002: 40,
		}
		distributeByUtilization(result, processUtil, 100.0)

		assert.InDelta(t, 60.0, result[1001], 0.01)
		assert.InDelta(t, 40.0, result[1002], 0.01)
	})

	t.Run("equal distribution when zero utilization", func(t *testing.T) {
		result := make(map[uint32]float64)
		processUtil := map[uint32]uint32{
			1001: 0,
			1002: 0,
		}
		distributeByUtilization(result, processUtil, 100.0)

		assert.InDelta(t, 50.0, result[1001], 0.01)
		assert.InDelta(t, 50.0, result[1002], 0.01)
	})

	t.Run("single process gets all power", func(t *testing.T) {
		result := make(map[uint32]float64)
		processUtil := map[uint32]uint32{
			1001: 80,
		}
		distributeByUtilization(result, processUtil, 100.0)

		assert.Equal(t, 100.0, result[1001])
	})

	t.Run("accumulates across calls", func(t *testing.T) {
		result := map[uint32]float64{
			1001: 10.0, // pre-existing
		}
		processUtil := map[uint32]uint32{
			1001: 100,
		}
		distributeByUtilization(result, processUtil, 50.0)

		assert.Equal(t, 60.0, result[1001]) // 10 + 50
	})
}

func TestGPUPowerCollector_cacheMIGHierarchy(t *testing.T) {
	t.Run("caches MIG instances from NVML", func(t *testing.T) {
		mockBackend := new(MockNVMLBackend)
		mockDevice := new(MockNVMLDevice)

		collector := &GPUPowerCollector{
			logger: slog.Default(),
			nvml:   mockBackend,
			devices: []gpu.GPUDevice{
				{Index: 0, UUID: "GPU-123"},
			},
			sharingModes: map[int]gpu.SharingMode{
				0: gpu.SharingModePartitioned,
			},
		}

		mockBackend.On("GetDevice", 0).Return(mockDevice, nil)
		mockDevice.On("GetMIGInstances").Return([]MIGInstance{
			{GPUInstanceID: 1, ProfileSlices: 3},
			{GPUInstanceID: 2, ProfileSlices: 3},
		}, nil)

		err := collector.cacheMIGHierarchy()
		assert.NoError(t, err)

		assert.Len(t, collector.migInstancesByDevice[0], 2)

		inst := collector.migInstancesByDevice[0][0]
		assert.Equal(t, 0, inst.ParentGPUIndex)
		assert.Equal(t, uint(1), inst.GPUInstanceID)

		mockBackend.AssertExpectations(t)
		mockDevice.AssertExpectations(t)
	})

	t.Run("skips non-MIG devices", func(t *testing.T) {
		mockBackend := new(MockNVMLBackend)

		collector := &GPUPowerCollector{
			logger: slog.Default(),
			nvml:   mockBackend,
			devices: []gpu.GPUDevice{
				{Index: 0, UUID: "GPU-123"},
			},
			sharingModes: map[int]gpu.SharingMode{
				0: gpu.SharingModeTimeSlicing,
			},
		}

		err := collector.cacheMIGHierarchy()
		assert.NoError(t, err)

		assert.Empty(t, collector.migInstancesByDevice)
	})

	t.Run("handles GetMIGInstances failure", func(t *testing.T) {
		mockBackend := new(MockNVMLBackend)
		mockDevice := new(MockNVMLDevice)

		collector := &GPUPowerCollector{
			logger: slog.Default(),
			nvml:   mockBackend,
			sharingModes: map[int]gpu.SharingMode{
				0: gpu.SharingModePartitioned,
			},
		}

		mockBackend.On("GetDevice", 0).Return(mockDevice, nil)
		mockDevice.On("GetMIGInstances").Return([]MIGInstance(nil), errors.New("unsupported"))

		err := collector.cacheMIGHierarchy()
		assert.NoError(t, err)

		// Device skipped, no instances cached
		assert.Empty(t, collector.migInstancesByDevice[0])

		mockBackend.AssertExpectations(t)
		mockDevice.AssertExpectations(t)
	})
}

func TestGPUPowerCollector_SetDCGMEndpoint(t *testing.T) {
	t.Run("before init stores endpoint", func(t *testing.T) {
		collector := &GPUPowerCollector{}

		collector.SetDCGMEndpoint("http://localhost:9400/metrics")
		assert.Equal(t, "http://localhost:9400/metrics", collector.dcgmEndpoint)

		collector.SetDCGMEndpoint("")
		assert.Equal(t, "", collector.dcgmEndpoint)
	})

	t.Run("after init with MIG reinitializes DCGM", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprint(w, "# HELP\nDCGM_FI_PROF_GR_ENGINE_ACTIVE{gpu=\"0\",GPU_I_ID=\"1\",GPU_I_PROFILE=\"1g.5gb\"} 0.5\n")
		}))
		defer ts.Close()

		collector := &GPUPowerCollector{
			logger:       slog.Default(),
			initialized:  true,
			sharingModes: map[int]gpu.SharingMode{0: gpu.SharingModePartitioned},
		}

		// SetDCGMEndpoint after Init triggers DCGM reinit
		collector.SetDCGMEndpoint(ts.URL + "/metrics")
		assert.NotNil(t, collector.dcgm, "DCGM should be initialized after SetDCGMEndpoint")
		assert.True(t, collector.dcgm.IsInitialized())
	})

	t.Run("after init without MIG does not init DCGM", func(t *testing.T) {
		collector := &GPUPowerCollector{
			logger:       slog.Default(),
			initialized:  true,
			sharingModes: map[int]gpu.SharingMode{0: gpu.SharingModeTimeSlicing},
		}

		collector.SetDCGMEndpoint("http://localhost:9400/metrics")
		assert.Nil(t, collector.dcgm, "DCGM should not be initialized for non-MIG GPUs")
	})
}

// Verify DCGMEndpointConfigurable interface implementation
var _ gpu.DCGMEndpointConfigurable = (*GPUPowerCollector)(nil)

func TestGPUPowerCollector_attributePartitioned_idleInstances(t *testing.T) {
	mockBackend := new(MockNVMLBackend)
	mockDevice := new(MockNVMLDevice)
	mockDCGM := new(MockDCGMBackend)
	mockMIGDev1 := new(MockNVMLDevice)

	collector := &GPUPowerCollector{
		logger: slog.Default(),
		nvml:   mockBackend,
		devices: []gpu.GPUDevice{
			{Index: 0, UUID: "GPU-123"},
		},
		sharingModes: map[int]gpu.SharingMode{
			0: gpu.SharingModePartitioned,
		},
		minObservedPower: map[string]float64{
			"GPU-123": 40.0,
		},
		idleObserved: map[string]bool{
			"GPU-123": true,
		},
		dcgm: mockDCGM,
		migInstancesByDevice: map[int][]MIGGPUInstance{
			0: {
				{ParentGPUIndex: 0, GPUInstanceID: 1},
				{ParentGPUIndex: 0, GPUInstanceID: 2},
			},
		},
	}

	mockDCGM.On("IsInitialized").Return(true)

	mockBackend.On("GetDevice", 0).Return(mockDevice, nil)
	mockDevice.On("GetPowerUsage").Return(device.Power(100*device.Watt), nil)
	mockDevice.On("UUID").Return("GPU-123")
	mockDevice.On("GetComputeRunningProcesses").Return([]gpu.ProcessGPUInfo{
		{PID: 1001},
	}, nil)

	// Instance 1: active with process
	mockDCGM.On("GetMIGInstanceActivity", 0, uint(1)).Return(0.5, nil)
	mockDevice.On("GetMIGDeviceByInstanceID", uint(1)).Return(mockMIGDev1, nil)
	mockMIGDev1.On("GetProcessUtilization", uint64(0)).Return([]gpu.ProcessUtilization{
		{PID: 1001, ComputeUtil: 80},
	}, nil)

	// Instance 2: idle (activity = 0) — should be skipped entirely
	mockDCGM.On("GetMIGInstanceActivity", 0, uint(2)).Return(0.0, nil)
	// No GetMIGDeviceByInstanceID call expected for idle instance

	result, err := collector.GetProcessPower()

	assert.NoError(t, err)
	assert.Len(t, result, 1)
	// Active power = 60W, only instance 1 is active
	// Instance 1 gets 60 * (0.5/0.5) = 60W → PID 1001 gets 60W
	assert.Equal(t, 60.0, result[1001])

	mockBackend.AssertExpectations(t)
	mockDevice.AssertExpectations(t)
	mockDCGM.AssertExpectations(t)
	mockMIGDev1.AssertExpectations(t)
}

func TestGPUPowerCollector_attributePartitioned_multiGPU(t *testing.T) {
	// Two physical GPUs both in MIG mode, each with their own instances and processes.
	// Verifies power is attributed independently per GPU.
	mockBackend := new(MockNVMLBackend)
	mockDevice0 := new(MockNVMLDevice)
	mockDevice1 := new(MockNVMLDevice)
	mockDCGM := new(MockDCGMBackend)
	mockMIG0_1 := new(MockNVMLDevice) // GPU 0, instance 1
	mockMIG0_2 := new(MockNVMLDevice) // GPU 0, instance 2
	mockMIG1_1 := new(MockNVMLDevice) // GPU 1, instance 1

	collector := &GPUPowerCollector{
		logger: slog.Default(),
		nvml:   mockBackend,
		devices: []gpu.GPUDevice{
			{Index: 0, UUID: "GPU-AAA"},
			{Index: 1, UUID: "GPU-BBB"},
		},
		sharingModes: map[int]gpu.SharingMode{
			0: gpu.SharingModePartitioned,
			1: gpu.SharingModePartitioned,
		},
		minObservedPower: map[string]float64{
			"GPU-AAA": 50.0,
			"GPU-BBB": 30.0,
		},
		idleObserved: map[string]bool{
			"GPU-AAA": true,
			"GPU-BBB": true,
		},
		dcgm: mockDCGM,
		migInstancesByDevice: map[int][]MIGGPUInstance{
			0: {
				{ParentGPUIndex: 0, GPUInstanceID: 1},
				{ParentGPUIndex: 0, GPUInstanceID: 2},
			},
			1: {
				{ParentGPUIndex: 1, GPUInstanceID: 1},
			},
		},
	}

	mockDCGM.On("IsInitialized").Return(true)

	// --- GPU 0: 200W total, 50W idle → 150W active ---
	mockBackend.On("GetDevice", 0).Return(mockDevice0, nil)
	mockDevice0.On("GetPowerUsage").Return(device.Power(200*device.Watt), nil)
	mockDevice0.On("UUID").Return("GPU-AAA")
	mockDevice0.On("GetComputeRunningProcesses").Return([]gpu.ProcessGPUInfo{
		{PID: 1001}, {PID: 1002},
	}, nil)

	// GPU 0, instance 1: activity 0.6, PID 1001
	mockDCGM.On("GetMIGInstanceActivity", 0, uint(1)).Return(0.6, nil)
	mockDevice0.On("GetMIGDeviceByInstanceID", uint(1)).Return(mockMIG0_1, nil)
	mockMIG0_1.On("GetProcessUtilization", uint64(0)).Return([]gpu.ProcessUtilization{
		{PID: 1001, ComputeUtil: 100},
	}, nil)

	// GPU 0, instance 2: activity 0.4, PID 1002
	mockDCGM.On("GetMIGInstanceActivity", 0, uint(2)).Return(0.4, nil)
	mockDevice0.On("GetMIGDeviceByInstanceID", uint(2)).Return(mockMIG0_2, nil)
	mockMIG0_2.On("GetProcessUtilization", uint64(0)).Return([]gpu.ProcessUtilization{
		{PID: 1002, ComputeUtil: 100},
	}, nil)

	// --- GPU 1: 100W total, 30W idle → 70W active ---
	mockBackend.On("GetDevice", 1).Return(mockDevice1, nil)
	mockDevice1.On("GetPowerUsage").Return(device.Power(100*device.Watt), nil)
	mockDevice1.On("UUID").Return("GPU-BBB")
	mockDevice1.On("GetComputeRunningProcesses").Return([]gpu.ProcessGPUInfo{
		{PID: 2001},
	}, nil)

	// GPU 1, instance 1: activity 1.0, PID 2001
	mockDCGM.On("GetMIGInstanceActivity", 1, uint(1)).Return(1.0, nil)
	mockDevice1.On("GetMIGDeviceByInstanceID", uint(1)).Return(mockMIG1_1, nil)
	mockMIG1_1.On("GetProcessUtilization", uint64(0)).Return([]gpu.ProcessUtilization{
		{PID: 2001, ComputeUtil: 80},
	}, nil)

	result, err := collector.GetProcessPower()
	assert.NoError(t, err)
	assert.Len(t, result, 3)

	// GPU 0 active = 150W, total activity = 0.6+0.4 = 1.0
	// PID 1001: 150 * (0.6/1.0) = 90W
	// PID 1002: 150 * (0.4/1.0) = 60W
	assert.InDelta(t, 90.0, result[1001], 0.01)
	assert.InDelta(t, 60.0, result[1002], 0.01)

	// GPU 1 active = 70W, total activity = 1.0
	// PID 2001: 70 * (1.0/1.0) = 70W
	assert.InDelta(t, 70.0, result[2001], 0.01)

	mockBackend.AssertExpectations(t)
	mockDevice0.AssertExpectations(t)
	mockDevice1.AssertExpectations(t)
	mockDCGM.AssertExpectations(t)
	mockMIG0_1.AssertExpectations(t)
	mockMIG0_2.AssertExpectations(t)
	mockMIG1_1.AssertExpectations(t)
}

func TestGPUPowerCollector_Shutdown_withDCGM(t *testing.T) {
	mockBackend := new(MockNVMLBackend)
	mockDCGM := new(MockDCGMBackend)

	mockBackend.On("Shutdown").Return(nil)
	mockDCGM.On("IsInitialized").Return(true)
	mockDCGM.On("Shutdown").Return(nil)

	collector := &GPUPowerCollector{
		nvml: mockBackend,
		dcgm: mockDCGM,
	}

	err := collector.Shutdown()

	assert.NoError(t, err)
	mockBackend.AssertExpectations(t)
	mockDCGM.AssertExpectations(t)
}

func TestGPUPowerCollector_Init_withMIG(t *testing.T) {
	mockBackend := new(MockNVMLBackend)
	mockDevice := new(MockNVMLDevice)

	mockBackend.On("Init").Return(nil)
	mockBackend.On("DiscoverDevices").Return([]gpu.GPUDevice{
		{Index: 0, UUID: "GPU-123", Name: "A100"},
	}, nil)
	mockBackend.On("DeviceCount").Return(1)
	mockBackend.On("GetDevice", 0).Return(mockDevice, nil)
	mockDevice.On("IsMIGEnabled").Return(true, nil)
	// MIG enumeration calls
	mockDevice.On("GetMIGInstances").Return([]MIGInstance{
		{GPUInstanceID: 1, ProfileSlices: 3},
	}, nil)

	collector := &GPUPowerCollector{
		logger:           slog.Default(),
		nvml:             mockBackend,
		minObservedPower: make(map[string]float64),
		idleObserved:     make(map[string]bool),
		sharingModes:     make(map[int]gpu.SharingMode),
	}

	// Init will detect MIG, try to create a real DCGMExporterBackend (which will fail
	// to init since there's no real dcgm-exporter), and cache MIG hierarchy
	err := collector.Init()

	assert.NoError(t, err)
	assert.Equal(t, gpu.SharingModePartitioned, collector.sharingModes[0])
	// MIG hierarchy should be cached even if DCGM init fails
	assert.Len(t, collector.migInstancesByDevice[0], 1)

	mockBackend.AssertExpectations(t)
	mockDevice.AssertExpectations(t)
}

func TestGPUPowerCollector_attributePartitioned_errorPaths(t *testing.T) {
	t.Run("GetDevice error", func(t *testing.T) {
		mockBackend := new(MockNVMLBackend)
		mockDCGM := new(MockDCGMBackend)

		collector := &GPUPowerCollector{
			logger: slog.Default(),
			nvml:   mockBackend,
			devices: []gpu.GPUDevice{
				{Index: 0, UUID: "GPU-123"},
			},
			sharingModes: map[int]gpu.SharingMode{
				0: gpu.SharingModePartitioned,
			},
			minObservedPower: make(map[string]float64),
			idleObserved:     make(map[string]bool),
			dcgm:             mockDCGM,
			migInstancesByDevice: map[int][]MIGGPUInstance{
				0: {{ParentGPUIndex: 0, GPUInstanceID: 1}},
			},
		}

		mockBackend.On("GetDevice", 0).Return(nil, errors.New("device error"))

		result, err := collector.GetProcessPower()
		assert.NoError(t, err) // error is logged, not returned
		assert.Empty(t, result)
		mockBackend.AssertExpectations(t)
	})

	t.Run("power stats error", func(t *testing.T) {
		mockBackend := new(MockNVMLBackend)
		mockDevice := new(MockNVMLDevice)
		mockDCGM := new(MockDCGMBackend)

		collector := &GPUPowerCollector{
			logger: slog.Default(),
			nvml:   mockBackend,
			devices: []gpu.GPUDevice{
				{Index: 0, UUID: "GPU-123"},
			},
			sharingModes: map[int]gpu.SharingMode{
				0: gpu.SharingModePartitioned,
			},
			minObservedPower: make(map[string]float64),
			idleObserved:     make(map[string]bool),
			dcgm:             mockDCGM,
			migInstancesByDevice: map[int][]MIGGPUInstance{
				0: {{ParentGPUIndex: 0, GPUInstanceID: 1}},
			},
		}

		mockBackend.On("GetDevice", 0).Return(mockDevice, nil)
		mockDevice.On("GetPowerUsage").Return(device.Power(0), errors.New("power error"))

		result, err := collector.GetProcessPower()
		assert.NoError(t, err)
		assert.Empty(t, result)
		mockBackend.AssertExpectations(t)
		mockDevice.AssertExpectations(t)
	})

	t.Run("DCGM activity error skips instance", func(t *testing.T) {
		mockBackend := new(MockNVMLBackend)
		mockDevice := new(MockNVMLDevice)
		mockDCGM := new(MockDCGMBackend)
		mockMIGDev := new(MockNVMLDevice)

		collector := &GPUPowerCollector{
			logger: slog.Default(),
			nvml:   mockBackend,
			devices: []gpu.GPUDevice{
				{Index: 0, UUID: "GPU-123"},
			},
			sharingModes: map[int]gpu.SharingMode{
				0: gpu.SharingModePartitioned,
			},
			minObservedPower: map[string]float64{"GPU-123": 40.0},
			idleObserved:     map[string]bool{"GPU-123": true},
			dcgm:             mockDCGM,
			migInstancesByDevice: map[int][]MIGGPUInstance{
				0: {
					{ParentGPUIndex: 0, GPUInstanceID: 1},
					{ParentGPUIndex: 0, GPUInstanceID: 2},
				},
			},
		}

		mockDCGM.On("IsInitialized").Return(true)
		mockBackend.On("GetDevice", 0).Return(mockDevice, nil)
		mockDevice.On("GetPowerUsage").Return(device.Power(100*device.Watt), nil)
		mockDevice.On("UUID").Return("GPU-123")
		mockDevice.On("GetComputeRunningProcesses").Return([]gpu.ProcessGPUInfo{{PID: 1001}}, nil)

		// Instance 1: DCGM error → skipped
		mockDCGM.On("GetMIGInstanceActivity", 0, uint(1)).Return(0.0, errors.New("dcgm error"))
		// Instance 2: succeeds
		mockDCGM.On("GetMIGInstanceActivity", 0, uint(2)).Return(0.5, nil)
		mockDevice.On("GetMIGDeviceByInstanceID", uint(2)).Return(mockMIGDev, nil)
		mockMIGDev.On("GetProcessUtilization", uint64(0)).Return([]gpu.ProcessUtilization{
			{PID: 2001, ComputeUtil: 80},
		}, nil)

		result, err := collector.GetProcessPower()
		assert.NoError(t, err)
		assert.Len(t, result, 1)
		assert.InDelta(t, 60.0, result[2001], 0.01) // 100-40=60W, only instance 2 active
		mockDCGM.AssertExpectations(t)
	})

	t.Run("MIG device error skips instance", func(t *testing.T) {
		mockBackend := new(MockNVMLBackend)
		mockDevice := new(MockNVMLDevice)
		mockDCGM := new(MockDCGMBackend)
		mockMIGDev := new(MockNVMLDevice)

		collector := &GPUPowerCollector{
			logger: slog.Default(),
			nvml:   mockBackend,
			devices: []gpu.GPUDevice{
				{Index: 0, UUID: "GPU-123"},
			},
			sharingModes: map[int]gpu.SharingMode{
				0: gpu.SharingModePartitioned,
			},
			minObservedPower: map[string]float64{"GPU-123": 40.0},
			idleObserved:     map[string]bool{"GPU-123": true},
			dcgm:             mockDCGM,
			migInstancesByDevice: map[int][]MIGGPUInstance{
				0: {
					{ParentGPUIndex: 0, GPUInstanceID: 1},
					{ParentGPUIndex: 0, GPUInstanceID: 2},
				},
			},
		}

		mockDCGM.On("IsInitialized").Return(true)
		mockBackend.On("GetDevice", 0).Return(mockDevice, nil)
		mockDevice.On("GetPowerUsage").Return(device.Power(100*device.Watt), nil)
		mockDevice.On("UUID").Return("GPU-123")
		mockDevice.On("GetComputeRunningProcesses").Return([]gpu.ProcessGPUInfo{{PID: 1001}}, nil)

		// Instance 1: active but GetMIGDeviceByInstanceID fails → skipped
		mockDCGM.On("GetMIGInstanceActivity", 0, uint(1)).Return(0.8, nil)
		mockDevice.On("GetMIGDeviceByInstanceID", uint(1)).Return(nil, errors.New("mig device error"))
		// Instance 2: succeeds
		mockDCGM.On("GetMIGInstanceActivity", 0, uint(2)).Return(0.5, nil)
		mockDevice.On("GetMIGDeviceByInstanceID", uint(2)).Return(mockMIGDev, nil)
		mockMIGDev.On("GetProcessUtilization", uint64(0)).Return([]gpu.ProcessUtilization{
			{PID: 2001, ComputeUtil: 50},
		}, nil)

		result, err := collector.GetProcessPower()
		assert.NoError(t, err)
		assert.Len(t, result, 1)
		assert.InDelta(t, 60.0, result[2001], 0.01) // Only instance 2 in data
		mockDCGM.AssertExpectations(t)
	})

	t.Run("process util fallback to GetComputeRunningProcesses", func(t *testing.T) {
		mockBackend := new(MockNVMLBackend)
		mockDevice := new(MockNVMLDevice)
		mockDCGM := new(MockDCGMBackend)
		mockMIGDev := new(MockNVMLDevice)

		collector := &GPUPowerCollector{
			logger: slog.Default(),
			nvml:   mockBackend,
			devices: []gpu.GPUDevice{
				{Index: 0, UUID: "GPU-123"},
			},
			sharingModes: map[int]gpu.SharingMode{
				0: gpu.SharingModePartitioned,
			},
			minObservedPower: map[string]float64{"GPU-123": 40.0},
			idleObserved:     map[string]bool{"GPU-123": true},
			dcgm:             mockDCGM,
			migInstancesByDevice: map[int][]MIGGPUInstance{
				0: {{ParentGPUIndex: 0, GPUInstanceID: 1}},
			},
		}

		mockDCGM.On("IsInitialized").Return(true)
		mockBackend.On("GetDevice", 0).Return(mockDevice, nil)
		mockDevice.On("GetPowerUsage").Return(device.Power(100*device.Watt), nil)
		mockDevice.On("UUID").Return("GPU-123")
		mockDevice.On("GetComputeRunningProcesses").Return([]gpu.ProcessGPUInfo{{PID: 1001}}, nil)

		mockDCGM.On("GetMIGInstanceActivity", 0, uint(1)).Return(0.5, nil)
		mockDevice.On("GetMIGDeviceByInstanceID", uint(1)).Return(mockMIGDev, nil)
		// GetProcessUtilization fails → falls back to GetComputeRunningProcesses
		mockMIGDev.On("GetProcessUtilization", uint64(0)).Return(nil, errors.New("not supported"))
		mockMIGDev.On("GetComputeRunningProcesses").Return([]gpu.ProcessGPUInfo{
			{PID: 3001},
			{PID: 3002},
		}, nil)

		result, err := collector.GetProcessPower()
		assert.NoError(t, err)
		assert.Len(t, result, 2)
		// Equal distribution: 60W / 2 = 30W each (utilization is 0 for both)
		assert.InDelta(t, 30.0, result[3001], 0.01)
		assert.InDelta(t, 30.0, result[3002], 0.01)
		mockMIGDev.AssertExpectations(t)
	})

	t.Run("GetComputeRunningProcesses error in fallback skips instance", func(t *testing.T) {
		mockBackend := new(MockNVMLBackend)
		mockDevice := new(MockNVMLDevice)
		mockDCGM := new(MockDCGMBackend)
		mockMIGDev := new(MockNVMLDevice)

		collector := &GPUPowerCollector{
			logger: slog.Default(),
			nvml:   mockBackend,
			devices: []gpu.GPUDevice{
				{Index: 0, UUID: "GPU-123"},
			},
			sharingModes: map[int]gpu.SharingMode{
				0: gpu.SharingModePartitioned,
			},
			minObservedPower: map[string]float64{"GPU-123": 40.0},
			idleObserved:     map[string]bool{"GPU-123": true},
			dcgm:             mockDCGM,
			migInstancesByDevice: map[int][]MIGGPUInstance{
				0: {{ParentGPUIndex: 0, GPUInstanceID: 1}},
			},
		}

		mockDCGM.On("IsInitialized").Return(true)
		mockBackend.On("GetDevice", 0).Return(mockDevice, nil)
		mockDevice.On("GetPowerUsage").Return(device.Power(100*device.Watt), nil)
		mockDevice.On("UUID").Return("GPU-123")
		mockDevice.On("GetComputeRunningProcesses").Return([]gpu.ProcessGPUInfo{{PID: 1001}}, nil)

		mockDCGM.On("GetMIGInstanceActivity", 0, uint(1)).Return(0.5, nil)
		mockDevice.On("GetMIGDeviceByInstanceID", uint(1)).Return(mockMIGDev, nil)
		mockMIGDev.On("GetProcessUtilization", uint64(0)).Return(nil, errors.New("not supported"))
		mockMIGDev.On("GetComputeRunningProcesses").Return(nil, errors.New("proc error"))

		result, err := collector.GetProcessPower()
		assert.NoError(t, err)
		assert.Empty(t, result) // Instance skipped, no data
		mockMIGDev.AssertExpectations(t)
	})

	t.Run("all DCGM instances fail returns empty", func(t *testing.T) {
		mockBackend := new(MockNVMLBackend)
		mockDevice := new(MockNVMLDevice)
		mockDCGM := new(MockDCGMBackend)

		collector := &GPUPowerCollector{
			logger: slog.Default(),
			nvml:   mockBackend,
			devices: []gpu.GPUDevice{
				{Index: 0, UUID: "GPU-123"},
			},
			sharingModes: map[int]gpu.SharingMode{
				0: gpu.SharingModePartitioned,
			},
			minObservedPower: map[string]float64{"GPU-123": 40.0},
			idleObserved:     map[string]bool{"GPU-123": true},
			dcgm:             mockDCGM,
			migInstancesByDevice: map[int][]MIGGPUInstance{
				0: {
					{ParentGPUIndex: 0, GPUInstanceID: 1},
					{ParentGPUIndex: 0, GPUInstanceID: 2},
				},
			},
		}

		mockDCGM.On("IsInitialized").Return(true)
		mockBackend.On("GetDevice", 0).Return(mockDevice, nil)
		mockDevice.On("GetPowerUsage").Return(device.Power(100*device.Watt), nil)
		mockDevice.On("UUID").Return("GPU-123")
		mockDevice.On("GetComputeRunningProcesses").Return([]gpu.ProcessGPUInfo{{PID: 1001}}, nil)

		// Both instances fail
		mockDCGM.On("GetMIGInstanceActivity", 0, uint(1)).Return(0.0, errors.New("error"))
		mockDCGM.On("GetMIGInstanceActivity", 0, uint(2)).Return(0.0, errors.New("error"))

		result, err := collector.GetProcessPower()
		assert.NoError(t, err)
		assert.Empty(t, result)
		mockDCGM.AssertExpectations(t)
	})
}

func TestGPUPowerCollector_attributePartitionedFallback_errorPaths(t *testing.T) {
	t.Run("GetComputeRunningProcesses error", func(t *testing.T) {
		mockBackend := new(MockNVMLBackend)
		mockDevice := new(MockNVMLDevice)

		collector := &GPUPowerCollector{
			logger: slog.Default(),
			nvml:   mockBackend,
			devices: []gpu.GPUDevice{
				{Index: 0, UUID: "GPU-123"},
			},
			sharingModes: map[int]gpu.SharingMode{
				0: gpu.SharingModePartitioned,
			},
			minObservedPower:     map[string]float64{"GPU-123": 40.0},
			idleObserved:         map[string]bool{"GPU-123": true},
			dcgm:                 nil, // no DCGM → uses fallback
			migInstancesByDevice: map[int][]MIGGPUInstance{},
		}

		mockBackend.On("GetDevice", 0).Return(mockDevice, nil)
		mockDevice.On("GetPowerUsage").Return(device.Power(100*device.Watt), nil)
		mockDevice.On("UUID").Return("GPU-123")
		// First call in getDevicePowerStatsLocked
		mockDevice.On("GetComputeRunningProcesses").Return(nil, errors.New("proc error"))

		result, err := collector.GetProcessPower()
		assert.NoError(t, err)
		assert.Empty(t, result)
		mockBackend.AssertExpectations(t)
	})

	t.Run("no processes returns empty", func(t *testing.T) {
		mockBackend := new(MockNVMLBackend)
		mockDevice := new(MockNVMLDevice)

		collector := &GPUPowerCollector{
			logger: slog.Default(),
			nvml:   mockBackend,
			devices: []gpu.GPUDevice{
				{Index: 0, UUID: "GPU-123"},
			},
			sharingModes: map[int]gpu.SharingMode{
				0: gpu.SharingModePartitioned,
			},
			minObservedPower:     map[string]float64{"GPU-123": 40.0},
			idleObserved:         map[string]bool{"GPU-123": true},
			dcgm:                 nil, // no DCGM → uses fallback
			migInstancesByDevice: map[int][]MIGGPUInstance{},
		}

		mockBackend.On("GetDevice", 0).Return(mockDevice, nil)
		mockDevice.On("GetPowerUsage").Return(device.Power(100*device.Watt), nil)
		mockDevice.On("UUID").Return("GPU-123")
		mockDevice.On("GetComputeRunningProcesses").Return([]gpu.ProcessGPUInfo{}, nil)

		result, err := collector.GetProcessPower()
		assert.NoError(t, err)
		assert.Empty(t, result)
		mockBackend.AssertExpectations(t)
	})
}

func TestGPUPowerCollector_cacheMIGHierarchy_errorPaths(t *testing.T) {
	t.Run("GetDevice error continues to next device", func(t *testing.T) {
		mockBackend := new(MockNVMLBackend)

		collector := &GPUPowerCollector{
			logger: slog.Default(),
			nvml:   mockBackend,
			sharingModes: map[int]gpu.SharingMode{
				0: gpu.SharingModePartitioned,
			},
		}

		mockBackend.On("GetDevice", 0).Return(nil, errors.New("device gone"))

		err := collector.cacheMIGHierarchy()
		assert.NoError(t, err)
		assert.Empty(t, collector.migInstancesByDevice)
		mockBackend.AssertExpectations(t)
	})

	t.Run("GetMIGInstances error continues to next device", func(t *testing.T) {
		mockBackend := new(MockNVMLBackend)
		mockDevice := new(MockNVMLDevice)

		collector := &GPUPowerCollector{
			logger: slog.Default(),
			nvml:   mockBackend,
			sharingModes: map[int]gpu.SharingMode{
				0: gpu.SharingModePartitioned,
			},
		}

		mockBackend.On("GetDevice", 0).Return(mockDevice, nil)
		mockDevice.On("GetMIGInstances").Return(nil, errors.New("mig error"))

		err := collector.cacheMIGHierarchy()
		assert.NoError(t, err)
		assert.Empty(t, collector.migInstancesByDevice)
		mockBackend.AssertExpectations(t)
		mockDevice.AssertExpectations(t)
	})
}

func TestGPUPowerCollector_Init_dcgmEndpoint(t *testing.T) {
	t.Run("pre-configured endpoint unreachable", func(t *testing.T) {
		mockBackend := new(MockNVMLBackend)
		mockDevice := new(MockNVMLDevice)

		mockBackend.On("Init").Return(nil)
		mockBackend.On("DiscoverDevices").Return([]gpu.GPUDevice{
			{Index: 0, UUID: "GPU-123", Name: "A100"},
		}, nil)
		mockBackend.On("DeviceCount").Return(1)
		mockBackend.On("GetDevice", 0).Return(mockDevice, nil)
		mockDevice.On("IsMIGEnabled").Return(true, nil)
		mockDevice.On("GetMIGInstances").Return([]MIGInstance{
			{GPUInstanceID: 1, ProfileSlices: 3},
		}, nil)

		collector := &GPUPowerCollector{
			logger:           slog.Default(),
			nvml:             mockBackend,
			minObservedPower: make(map[string]float64),
			idleObserved:     make(map[string]bool),
			sharingModes:     make(map[int]gpu.SharingMode),
			dcgmEndpoint:     "http://10.0.0.1:9400/metrics", // pre-configured
		}

		err := collector.Init()
		assert.NoError(t, err)
		assert.Equal(t, gpu.SharingModePartitioned, collector.sharingModes[0])
		assert.Nil(t, collector.dcgm) // DCGM failed to init
		assert.Len(t, collector.migInstancesByDevice[0], 1)

		mockBackend.AssertExpectations(t)
		mockDevice.AssertExpectations(t)
	})

	t.Run("pre-configured endpoint reachable", func(t *testing.T) {
		// Set up a real HTTP server that serves DCGM metrics
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprint(w, "# HELP DCGM_FI_PROF_GR_ENGINE_ACTIVE\nDCGM_FI_PROF_GR_ENGINE_ACTIVE{gpu=\"0\",GPU_I_ID=\"1\",GPU_I_PROFILE=\"1g.5gb\"} 0.5\n")
		}))
		defer ts.Close()

		mockBackend := new(MockNVMLBackend)
		mockDevice := new(MockNVMLDevice)

		mockBackend.On("Init").Return(nil)
		mockBackend.On("DiscoverDevices").Return([]gpu.GPUDevice{
			{Index: 0, UUID: "GPU-123", Name: "A100"},
		}, nil)
		mockBackend.On("DeviceCount").Return(1)
		mockBackend.On("GetDevice", 0).Return(mockDevice, nil)
		mockDevice.On("IsMIGEnabled").Return(true, nil)
		mockDevice.On("GetMIGInstances").Return([]MIGInstance{
			{GPUInstanceID: 1, ProfileSlices: 3},
		}, nil)

		collector := &GPUPowerCollector{
			logger:           slog.Default(),
			nvml:             mockBackend,
			minObservedPower: make(map[string]float64),
			idleObserved:     make(map[string]bool),
			sharingModes:     make(map[int]gpu.SharingMode),
			dcgmEndpoint:     ts.URL + "/metrics", // reachable endpoint
		}

		err := collector.Init()
		assert.NoError(t, err)
		assert.Equal(t, gpu.SharingModePartitioned, collector.sharingModes[0])
		assert.NotNil(t, collector.dcgm) // DCGM init succeeds
		assert.True(t, collector.dcgm.IsInitialized())
		assert.Len(t, collector.migInstancesByDevice[0], 1)

		mockBackend.AssertExpectations(t)
		mockDevice.AssertExpectations(t)
	})
}

func TestGPUPowerCollector_Shutdown_dcgmError(t *testing.T) {
	mockBackend := new(MockNVMLBackend)
	mockDCGM := new(MockDCGMBackend)

	mockBackend.On("Shutdown").Return(nil)
	mockDCGM.On("IsInitialized").Return(true)
	mockDCGM.On("Shutdown").Return(errors.New("dcgm shutdown error"))

	collector := &GPUPowerCollector{
		logger: slog.Default(),
		nvml:   mockBackend,
		dcgm:   mockDCGM,
	}

	err := collector.Shutdown()
	// DCGM error is logged, not returned; NVML shutdown succeeds
	assert.NoError(t, err)
	mockBackend.AssertExpectations(t)
	mockDCGM.AssertExpectations(t)
}

// Ensure interface implementation
var _ gpu.GPUPowerMeter = (*GPUPowerCollector)(nil)
