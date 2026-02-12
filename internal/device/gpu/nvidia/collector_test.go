// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package nvidia

import (
	"errors"
	"log/slog"
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

	t.Run("partitioned mode skipped", func(t *testing.T) {
		mockBackend := new(MockNVMLBackend)

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
		}

		result, err := collector.GetProcessPower()

		assert.NoError(t, err)
		assert.Empty(t, result) // Partitioned mode not yet implemented

		mockBackend.AssertExpectations(t)
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

// Ensure interface implementation
var _ gpu.GPUPowerMeter = (*GPUPowerCollector)(nil)
