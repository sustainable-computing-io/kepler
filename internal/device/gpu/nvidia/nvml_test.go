// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package nvidia

import (
	"log/slog"
	"testing"

	"github.com/NVIDIA/go-nvml/pkg/nvml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/sustainable-computing-io/kepler/internal/device"
)

// mockNvmlLib is a mock implementation of nvmlLib for testing
type mockNvmlLib struct {
	mock.Mock
}

func (m *mockNvmlLib) Init() nvml.Return {
	args := m.Called()
	return args.Get(0).(nvml.Return)
}

func (m *mockNvmlLib) Shutdown() nvml.Return {
	args := m.Called()
	return args.Get(0).(nvml.Return)
}

func (m *mockNvmlLib) DeviceGetCount() (int, nvml.Return) {
	args := m.Called()
	return args.Int(0), args.Get(1).(nvml.Return)
}

func (m *mockNvmlLib) DeviceGetHandleByIndex(index int) (nvmlDeviceHandle, nvml.Return) {
	args := m.Called(index)
	handle := args.Get(0)
	if handle == nil {
		return nil, args.Get(1).(nvml.Return)
	}
	return handle.(nvmlDeviceHandle), args.Get(1).(nvml.Return)
}

func (m *mockNvmlLib) ErrorString(ret nvml.Return) string {
	args := m.Called(ret)
	return args.String(0)
}

// mockDeviceHandle is a mock implementation of nvmlDeviceHandle for testing
type mockDeviceHandle struct {
	mock.Mock
}

func (m *mockDeviceHandle) GetUUID() (string, nvml.Return) {
	args := m.Called()
	return args.String(0), args.Get(1).(nvml.Return)
}

func (m *mockDeviceHandle) GetName() (string, nvml.Return) {
	args := m.Called()
	return args.String(0), args.Get(1).(nvml.Return)
}

func (m *mockDeviceHandle) GetPowerUsage() (uint32, nvml.Return) {
	args := m.Called()
	return args.Get(0).(uint32), args.Get(1).(nvml.Return)
}

func (m *mockDeviceHandle) GetTotalEnergyConsumption() (uint64, nvml.Return) {
	args := m.Called()
	return args.Get(0).(uint64), args.Get(1).(nvml.Return)
}

func (m *mockDeviceHandle) GetComputeRunningProcesses() ([]nvml.ProcessInfo, nvml.Return) {
	args := m.Called()
	procs := args.Get(0)
	if procs == nil {
		return nil, args.Get(1).(nvml.Return)
	}
	return procs.([]nvml.ProcessInfo), args.Get(1).(nvml.Return)
}

func (m *mockDeviceHandle) GetProcessUtilization(lastSeen uint64) ([]nvml.ProcessUtilizationSample, nvml.Return) {
	args := m.Called(lastSeen)
	samples := args.Get(0)
	if samples == nil {
		return nil, args.Get(1).(nvml.Return)
	}
	return samples.([]nvml.ProcessUtilizationSample), args.Get(1).(nvml.Return)
}

func (m *mockDeviceHandle) GetComputeMode() (nvml.ComputeMode, nvml.Return) {
	args := m.Called()
	return args.Get(0).(nvml.ComputeMode), args.Get(1).(nvml.Return)
}

func (m *mockDeviceHandle) GetMigMode() (int, int, nvml.Return) {
	args := m.Called()
	return args.Int(0), args.Int(1), args.Get(2).(nvml.Return)
}

func (m *mockDeviceHandle) GetMigDeviceHandleByIndex(index int) (nvmlDeviceHandle, nvml.Return) {
	args := m.Called(index)
	handle := args.Get(0)
	if handle == nil {
		return nil, args.Get(1).(nvml.Return)
	}
	return handle.(nvmlDeviceHandle), args.Get(1).(nvml.Return)
}

func (m *mockDeviceHandle) GetGpuInstanceId() (int, nvml.Return) {
	args := m.Called()
	return args.Int(0), args.Get(1).(nvml.Return)
}

func (m *mockDeviceHandle) GetMaxMigDeviceCount() (int, nvml.Return) {
	args := m.Called()
	return args.Int(0), args.Get(1).(nvml.Return)
}

func (m *mockDeviceHandle) GetAccountingMode() (nvml.EnableState, nvml.Return) {
	args := m.Called()
	return args.Get(0).(nvml.EnableState), args.Get(1).(nvml.Return)
}

func TestNewNVMLBackend(t *testing.T) {
	t.Run("with logger", func(t *testing.T) {
		logger := slog.Default()
		backend := NewNVMLBackend(logger)

		assert.NotNil(t, backend)
		b := backend.(*nvmlBackend)
		assert.False(t, b.initialized)
		assert.Nil(t, b.devices)
	})

	t.Run("with nil logger uses default", func(t *testing.T) {
		backend := NewNVMLBackend(nil)

		assert.NotNil(t, backend)
		b := backend.(*nvmlBackend)
		assert.NotNil(t, b.logger)
	})
}

func TestNVMLBackend_Init(t *testing.T) {
	t.Run("successful init with devices", func(t *testing.T) {
		mockLib := new(mockNvmlLib)
		mockHandle := new(mockDeviceHandle)

		mockLib.On("Init").Return(nvml.SUCCESS)
		mockLib.On("DeviceGetCount").Return(1, nvml.SUCCESS)
		mockLib.On("DeviceGetHandleByIndex", 0).Return(mockHandle, nvml.SUCCESS)
		mockHandle.On("GetUUID").Return("GPU-123", nvml.SUCCESS)
		mockHandle.On("GetName").Return("Test GPU", nvml.SUCCESS)

		backend := newNVMLBackendWithLib(slog.Default(), mockLib)
		err := backend.Init()

		assert.NoError(t, err)
		assert.True(t, backend.initialized)
		assert.Len(t, backend.devices, 1)
		assert.Equal(t, "GPU-123", backend.devices[0].uuid)
		assert.Equal(t, "Test GPU", backend.devices[0].name)

		mockLib.AssertExpectations(t)
		mockHandle.AssertExpectations(t)
	})

	t.Run("already initialized", func(t *testing.T) {
		mockLib := new(mockNvmlLib)

		backend := newNVMLBackendWithLib(slog.Default(), mockLib)
		backend.initialized = true

		err := backend.Init()

		assert.NoError(t, err)
		mockLib.AssertNotCalled(t, "Init")
	})

	t.Run("init failure", func(t *testing.T) {
		mockLib := new(mockNvmlLib)

		mockLib.On("Init").Return(nvml.ERROR_UNKNOWN)
		mockLib.On("ErrorString", nvml.ERROR_UNKNOWN).Return("Unknown error")

		backend := newNVMLBackendWithLib(slog.Default(), mockLib)
		err := backend.Init()

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "NVML init failed")
		assert.False(t, backend.initialized)

		mockLib.AssertExpectations(t)
	})

	t.Run("device count failure", func(t *testing.T) {
		mockLib := new(mockNvmlLib)

		mockLib.On("Init").Return(nvml.SUCCESS)
		mockLib.On("DeviceGetCount").Return(0, nvml.ERROR_UNKNOWN)
		mockLib.On("Shutdown").Return(nvml.SUCCESS)
		mockLib.On("ErrorString", nvml.ERROR_UNKNOWN).Return("Unknown error")

		backend := newNVMLBackendWithLib(slog.Default(), mockLib)
		err := backend.Init()

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get device count")
		assert.False(t, backend.initialized)

		mockLib.AssertExpectations(t)
	})

	t.Run("device handle failure skips device", func(t *testing.T) {
		mockLib := new(mockNvmlLib)
		mockHandle := new(mockDeviceHandle)

		mockLib.On("Init").Return(nvml.SUCCESS)
		mockLib.On("DeviceGetCount").Return(2, nvml.SUCCESS)
		mockLib.On("DeviceGetHandleByIndex", 0).Return(nil, nvml.ERROR_UNKNOWN)
		mockLib.On("DeviceGetHandleByIndex", 1).Return(mockHandle, nvml.SUCCESS)
		mockLib.On("ErrorString", nvml.ERROR_UNKNOWN).Return("Unknown error")
		mockHandle.On("GetUUID").Return("GPU-456", nvml.SUCCESS)
		mockHandle.On("GetName").Return("Test GPU 1", nvml.SUCCESS)

		backend := newNVMLBackendWithLib(slog.Default(), mockLib)
		err := backend.Init()

		assert.NoError(t, err)
		assert.Len(t, backend.devices, 1)
		assert.Equal(t, "GPU-456", backend.devices[0].uuid)

		mockLib.AssertExpectations(t)
		mockHandle.AssertExpectations(t)
	})

	t.Run("UUID failure uses fallback", func(t *testing.T) {
		mockLib := new(mockNvmlLib)
		mockHandle := new(mockDeviceHandle)

		mockLib.On("Init").Return(nvml.SUCCESS)
		mockLib.On("DeviceGetCount").Return(1, nvml.SUCCESS)
		mockLib.On("DeviceGetHandleByIndex", 0).Return(mockHandle, nvml.SUCCESS)
		mockHandle.On("GetUUID").Return("", nvml.ERROR_UNKNOWN)
		mockHandle.On("GetName").Return("Test GPU", nvml.SUCCESS)

		backend := newNVMLBackendWithLib(slog.Default(), mockLib)
		err := backend.Init()

		assert.NoError(t, err)
		assert.Equal(t, "gpu-0", backend.devices[0].uuid)

		mockLib.AssertExpectations(t)
		mockHandle.AssertExpectations(t)
	})

	t.Run("name failure uses fallback", func(t *testing.T) {
		mockLib := new(mockNvmlLib)
		mockHandle := new(mockDeviceHandle)

		mockLib.On("Init").Return(nvml.SUCCESS)
		mockLib.On("DeviceGetCount").Return(1, nvml.SUCCESS)
		mockLib.On("DeviceGetHandleByIndex", 0).Return(mockHandle, nvml.SUCCESS)
		mockHandle.On("GetUUID").Return("GPU-123", nvml.SUCCESS)
		mockHandle.On("GetName").Return("", nvml.ERROR_UNKNOWN)

		backend := newNVMLBackendWithLib(slog.Default(), mockLib)
		err := backend.Init()

		assert.NoError(t, err)
		assert.Equal(t, "Unknown NVIDIA GPU", backend.devices[0].name)

		mockLib.AssertExpectations(t)
		mockHandle.AssertExpectations(t)
	})
}

func TestNVMLBackend_Shutdown(t *testing.T) {
	t.Run("successful shutdown", func(t *testing.T) {
		mockLib := new(mockNvmlLib)
		mockLib.On("Shutdown").Return(nvml.SUCCESS)

		backend := newNVMLBackendWithLib(slog.Default(), mockLib)
		backend.initialized = true
		backend.devices = []nvmlDevice{{index: 0}}

		err := backend.Shutdown()

		assert.NoError(t, err)
		assert.False(t, backend.initialized)
		assert.Nil(t, backend.devices)

		mockLib.AssertExpectations(t)
	})

	t.Run("not initialized is no-op", func(t *testing.T) {
		mockLib := new(mockNvmlLib)

		backend := newNVMLBackendWithLib(slog.Default(), mockLib)
		backend.initialized = false

		err := backend.Shutdown()

		assert.NoError(t, err)
		mockLib.AssertNotCalled(t, "Shutdown")
	})

	t.Run("shutdown failure", func(t *testing.T) {
		mockLib := new(mockNvmlLib)
		mockLib.On("Shutdown").Return(nvml.ERROR_UNKNOWN)
		mockLib.On("ErrorString", nvml.ERROR_UNKNOWN).Return("Unknown error")

		backend := newNVMLBackendWithLib(slog.Default(), mockLib)
		backend.initialized = true

		err := backend.Shutdown()

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "NVML shutdown failed")

		mockLib.AssertExpectations(t)
	})
}

func TestNVMLBackend_DeviceCount(t *testing.T) {
	mockLib := new(mockNvmlLib)
	backend := newNVMLBackendWithLib(slog.Default(), mockLib)
	backend.devices = []nvmlDevice{{}, {}, {}}

	assert.Equal(t, 3, backend.DeviceCount())
}

func TestNVMLBackend_GetDevice(t *testing.T) {
	t.Run("not initialized", func(t *testing.T) {
		mockLib := new(mockNvmlLib)
		backend := newNVMLBackendWithLib(slog.Default(), mockLib)
		backend.initialized = false

		_, err := backend.GetDevice(0)
		assert.Error(t, err)
	})

	t.Run("invalid index", func(t *testing.T) {
		mockLib := new(mockNvmlLib)
		backend := newNVMLBackendWithLib(slog.Default(), mockLib)
		backend.initialized = true
		backend.devices = []nvmlDevice{}

		_, err := backend.GetDevice(0)
		assert.Error(t, err)

		_, err = backend.GetDevice(-1)
		assert.Error(t, err)
	})

	t.Run("valid index", func(t *testing.T) {
		mockLib := new(mockNvmlLib)
		mockHandle := new(mockDeviceHandle)
		backend := newNVMLBackendWithLib(slog.Default(), mockLib)
		backend.initialized = true
		backend.devices = []nvmlDevice{
			{index: 0, handle: mockHandle, lib: mockLib, uuid: "GPU-123", name: "Test"},
		}

		dev, err := backend.GetDevice(0)
		assert.NoError(t, err)
		assert.Equal(t, "GPU-123", dev.UUID())
	})
}

func TestNVMLBackend_DiscoverDevices(t *testing.T) {
	t.Run("not initialized", func(t *testing.T) {
		mockLib := new(mockNvmlLib)
		backend := newNVMLBackendWithLib(slog.Default(), mockLib)
		backend.initialized = false

		_, err := backend.DiscoverDevices()
		assert.Error(t, err)
	})

	t.Run("returns devices", func(t *testing.T) {
		mockLib := new(mockNvmlLib)
		backend := newNVMLBackendWithLib(slog.Default(), mockLib)
		backend.initialized = true
		backend.devices = []nvmlDevice{
			{index: 0, uuid: "GPU-0", name: "GPU 0"},
			{index: 1, uuid: "GPU-1", name: "GPU 1"},
		}

		devices, err := backend.DiscoverDevices()
		assert.NoError(t, err)
		assert.Len(t, devices, 2)
		assert.Equal(t, "GPU-0", devices[0].UUID)
		assert.Equal(t, "GPU-1", devices[1].UUID)
	})
}

func TestNVMLDevice_GetPowerUsage(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mockLib := new(mockNvmlLib)
		mockHandle := new(mockDeviceHandle)

		mockHandle.On("GetPowerUsage").Return(uint32(100000), nvml.SUCCESS) // 100W in mW

		dev := &nvmlDevice{handle: mockHandle, lib: mockLib}
		power, err := dev.GetPowerUsage()

		assert.NoError(t, err)
		assert.Equal(t, device.Power(100000)*device.MilliWatt, power)

		mockHandle.AssertExpectations(t)
	})

	t.Run("error", func(t *testing.T) {
		mockLib := new(mockNvmlLib)
		mockHandle := new(mockDeviceHandle)

		mockHandle.On("GetPowerUsage").Return(uint32(0), nvml.ERROR_UNKNOWN)
		mockLib.On("ErrorString", nvml.ERROR_UNKNOWN).Return("Unknown error")

		dev := &nvmlDevice{handle: mockHandle, lib: mockLib}
		_, err := dev.GetPowerUsage()

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get power usage")

		mockHandle.AssertExpectations(t)
		mockLib.AssertExpectations(t)
	})
}

func TestNVMLDevice_GetTotalEnergy(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mockLib := new(mockNvmlLib)
		mockHandle := new(mockDeviceHandle)

		mockHandle.On("GetTotalEnergyConsumption").Return(uint64(5000000), nvml.SUCCESS) // 5000J in mJ

		dev := &nvmlDevice{handle: mockHandle, lib: mockLib}
		energy, err := dev.GetTotalEnergy()

		assert.NoError(t, err)
		assert.Equal(t, device.Energy(5000000)*device.MilliJoule, energy)

		mockHandle.AssertExpectations(t)
	})

	t.Run("error", func(t *testing.T) {
		mockLib := new(mockNvmlLib)
		mockHandle := new(mockDeviceHandle)

		mockHandle.On("GetTotalEnergyConsumption").Return(uint64(0), nvml.ERROR_UNKNOWN)
		mockLib.On("ErrorString", nvml.ERROR_UNKNOWN).Return("Unknown error")

		dev := &nvmlDevice{handle: mockHandle, lib: mockLib}
		_, err := dev.GetTotalEnergy()

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get total energy")

		mockHandle.AssertExpectations(t)
		mockLib.AssertExpectations(t)
	})
}

func TestNVMLDevice_GetComputeRunningProcesses(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mockLib := new(mockNvmlLib)
		mockHandle := new(mockDeviceHandle)

		procs := []nvml.ProcessInfo{
			{Pid: 1234, UsedGpuMemory: 1024},
			{Pid: 5678, UsedGpuMemory: 2048},
		}
		mockHandle.On("GetComputeRunningProcesses").Return(procs, nvml.SUCCESS)

		dev := &nvmlDevice{index: 0, handle: mockHandle, lib: mockLib, uuid: "GPU-123"}
		result, err := dev.GetComputeRunningProcesses()

		assert.NoError(t, err)
		assert.Len(t, result, 2)
		assert.Equal(t, uint32(1234), result[0].PID)
		assert.Equal(t, uint64(1024), result[0].MemoryUsed)

		mockHandle.AssertExpectations(t)
	})

	t.Run("error", func(t *testing.T) {
		mockLib := new(mockNvmlLib)
		mockHandle := new(mockDeviceHandle)

		mockHandle.On("GetComputeRunningProcesses").Return(nil, nvml.ERROR_UNKNOWN)
		mockLib.On("ErrorString", nvml.ERROR_UNKNOWN).Return("Unknown error")

		dev := &nvmlDevice{handle: mockHandle, lib: mockLib}
		_, err := dev.GetComputeRunningProcesses()

		assert.Error(t, err)

		mockHandle.AssertExpectations(t)
		mockLib.AssertExpectations(t)
	})
}

func TestNVMLDevice_GetProcessUtilization(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mockLib := new(mockNvmlLib)
		mockHandle := new(mockDeviceHandle)

		samples := []nvml.ProcessUtilizationSample{
			{Pid: 1234, SmUtil: 50, MemUtil: 30, EncUtil: 0, DecUtil: 0, TimeStamp: 12345},
		}
		mockHandle.On("GetProcessUtilization", uint64(0)).Return(samples, nvml.SUCCESS)

		dev := &nvmlDevice{handle: mockHandle, lib: mockLib}
		result, err := dev.GetProcessUtilization(0)

		assert.NoError(t, err)
		assert.Len(t, result, 1)
		assert.Equal(t, uint32(1234), result[0].PID)
		assert.Equal(t, uint32(50), result[0].ComputeUtil)

		mockHandle.AssertExpectations(t)
	})

	t.Run("accounting mode disabled", func(t *testing.T) {
		mockLib := new(mockNvmlLib)
		mockHandle := new(mockDeviceHandle)

		mockHandle.On("GetProcessUtilization", uint64(0)).Return(nil, nvml.ERROR_NOT_SUPPORTED)
		mockHandle.On("GetAccountingMode").Return(nvml.FEATURE_DISABLED, nvml.SUCCESS)

		dev := &nvmlDevice{handle: mockHandle, lib: mockLib}
		_, err := dev.GetProcessUtilization(0)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "accounting mode is disabled")

		mockHandle.AssertExpectations(t)
	})

	t.Run("generic error", func(t *testing.T) {
		mockLib := new(mockNvmlLib)
		mockHandle := new(mockDeviceHandle)

		mockHandle.On("GetProcessUtilization", uint64(0)).Return(nil, nvml.ERROR_UNKNOWN)
		mockHandle.On("GetAccountingMode").Return(nvml.FEATURE_ENABLED, nvml.SUCCESS)
		mockLib.On("ErrorString", nvml.ERROR_UNKNOWN).Return("Unknown error")

		dev := &nvmlDevice{handle: mockHandle, lib: mockLib}
		_, err := dev.GetProcessUtilization(0)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "GetProcessUtilization failed")

		mockHandle.AssertExpectations(t)
		mockLib.AssertExpectations(t)
	})
}

func TestNVMLDevice_GetComputeMode(t *testing.T) {
	t.Run("default mode", func(t *testing.T) {
		mockLib := new(mockNvmlLib)
		mockHandle := new(mockDeviceHandle)

		mockHandle.On("GetComputeMode").Return(nvml.COMPUTEMODE_DEFAULT, nvml.SUCCESS)

		dev := &nvmlDevice{handle: mockHandle, lib: mockLib}
		mode, err := dev.GetComputeMode()

		assert.NoError(t, err)
		assert.Equal(t, ComputeModeDefault, mode)

		mockHandle.AssertExpectations(t)
	})

	t.Run("exclusive thread mode", func(t *testing.T) {
		mockLib := new(mockNvmlLib)
		mockHandle := new(mockDeviceHandle)

		mockHandle.On("GetComputeMode").Return(nvml.COMPUTEMODE_EXCLUSIVE_THREAD, nvml.SUCCESS)

		dev := &nvmlDevice{handle: mockHandle, lib: mockLib}
		mode, err := dev.GetComputeMode()

		assert.NoError(t, err)
		assert.Equal(t, ComputeModeExclusiveThread, mode)

		mockHandle.AssertExpectations(t)
	})

	t.Run("exclusive process mode", func(t *testing.T) {
		mockLib := new(mockNvmlLib)
		mockHandle := new(mockDeviceHandle)

		mockHandle.On("GetComputeMode").Return(nvml.COMPUTEMODE_EXCLUSIVE_PROCESS, nvml.SUCCESS)

		dev := &nvmlDevice{handle: mockHandle, lib: mockLib}
		mode, err := dev.GetComputeMode()

		assert.NoError(t, err)
		assert.Equal(t, ComputeModeExclusiveProcess, mode)

		mockHandle.AssertExpectations(t)
	})

	t.Run("prohibited mode", func(t *testing.T) {
		mockLib := new(mockNvmlLib)
		mockHandle := new(mockDeviceHandle)

		mockHandle.On("GetComputeMode").Return(nvml.COMPUTEMODE_PROHIBITED, nvml.SUCCESS)

		dev := &nvmlDevice{handle: mockHandle, lib: mockLib}
		mode, err := dev.GetComputeMode()

		assert.NoError(t, err)
		assert.Equal(t, ComputeModeProhibited, mode)

		mockHandle.AssertExpectations(t)
	})

	t.Run("unknown mode defaults", func(t *testing.T) {
		mockLib := new(mockNvmlLib)
		mockHandle := new(mockDeviceHandle)

		mockHandle.On("GetComputeMode").Return(nvml.ComputeMode(999), nvml.SUCCESS)

		dev := &nvmlDevice{handle: mockHandle, lib: mockLib}
		mode, err := dev.GetComputeMode()

		assert.NoError(t, err)
		assert.Equal(t, ComputeModeDefault, mode)

		mockHandle.AssertExpectations(t)
	})

	t.Run("error", func(t *testing.T) {
		mockLib := new(mockNvmlLib)
		mockHandle := new(mockDeviceHandle)

		mockHandle.On("GetComputeMode").Return(nvml.COMPUTEMODE_DEFAULT, nvml.ERROR_UNKNOWN)
		mockLib.On("ErrorString", nvml.ERROR_UNKNOWN).Return("Unknown error")

		dev := &nvmlDevice{handle: mockHandle, lib: mockLib}
		_, err := dev.GetComputeMode()

		assert.Error(t, err)

		mockHandle.AssertExpectations(t)
		mockLib.AssertExpectations(t)
	})
}

func TestNVMLDevice_IsMIGEnabled(t *testing.T) {
	t.Run("MIG enabled", func(t *testing.T) {
		mockLib := new(mockNvmlLib)
		mockHandle := new(mockDeviceHandle)

		mockHandle.On("GetMigMode").Return(nvml.DEVICE_MIG_ENABLE, 0, nvml.SUCCESS)

		dev := &nvmlDevice{handle: mockHandle, lib: mockLib}
		enabled, err := dev.IsMIGEnabled()

		assert.NoError(t, err)
		assert.True(t, enabled)

		mockHandle.AssertExpectations(t)
	})

	t.Run("MIG disabled", func(t *testing.T) {
		mockLib := new(mockNvmlLib)
		mockHandle := new(mockDeviceHandle)

		mockHandle.On("GetMigMode").Return(nvml.DEVICE_MIG_DISABLE, 0, nvml.SUCCESS)

		dev := &nvmlDevice{handle: mockHandle, lib: mockLib}
		enabled, err := dev.IsMIGEnabled()

		assert.NoError(t, err)
		assert.False(t, enabled)

		mockHandle.AssertExpectations(t)
	})

	t.Run("not supported", func(t *testing.T) {
		mockLib := new(mockNvmlLib)
		mockHandle := new(mockDeviceHandle)

		mockHandle.On("GetMigMode").Return(0, 0, nvml.ERROR_NOT_SUPPORTED)

		dev := &nvmlDevice{handle: mockHandle, lib: mockLib}
		enabled, err := dev.IsMIGEnabled()

		assert.NoError(t, err)
		assert.False(t, enabled)

		mockHandle.AssertExpectations(t)
	})

	t.Run("error", func(t *testing.T) {
		mockLib := new(mockNvmlLib)
		mockHandle := new(mockDeviceHandle)

		mockHandle.On("GetMigMode").Return(0, 0, nvml.ERROR_UNKNOWN)
		mockLib.On("ErrorString", nvml.ERROR_UNKNOWN).Return("Unknown error")

		dev := &nvmlDevice{handle: mockHandle, lib: mockLib}
		_, err := dev.IsMIGEnabled()

		assert.Error(t, err)

		mockHandle.AssertExpectations(t)
		mockLib.AssertExpectations(t)
	})
}

func TestNVMLDevice_GetMIGInstances(t *testing.T) {
	t.Run("MIG not enabled", func(t *testing.T) {
		mockLib := new(mockNvmlLib)
		mockHandle := new(mockDeviceHandle)

		mockHandle.On("GetMigMode").Return(nvml.DEVICE_MIG_DISABLE, 0, nvml.SUCCESS)

		dev := &nvmlDevice{index: 0, handle: mockHandle, lib: mockLib}
		_, err := dev.GetMIGInstances()

		assert.Error(t, err)

		mockHandle.AssertExpectations(t)
	})

	t.Run("MIG enabled with instances", func(t *testing.T) {
		mockLib := new(mockNvmlLib)
		mockHandle := new(mockDeviceHandle)
		mockMigHandle := new(mockDeviceHandle)

		mockHandle.On("GetMigMode").Return(nvml.DEVICE_MIG_ENABLE, 0, nvml.SUCCESS)
		mockHandle.On("GetMaxMigDeviceCount").Return(7, nvml.SUCCESS)
		mockHandle.On("GetMigDeviceHandleByIndex", 0).Return(mockMigHandle, nvml.SUCCESS)
		mockHandle.On("GetMigDeviceHandleByIndex", 1).Return(nil, nvml.ERROR_NOT_FOUND)
		mockHandle.On("GetMigDeviceHandleByIndex", 2).Return(nil, nvml.ERROR_NOT_FOUND)
		mockHandle.On("GetMigDeviceHandleByIndex", 3).Return(nil, nvml.ERROR_NOT_FOUND)
		mockHandle.On("GetMigDeviceHandleByIndex", 4).Return(nil, nvml.ERROR_NOT_FOUND)
		mockHandle.On("GetMigDeviceHandleByIndex", 5).Return(nil, nvml.ERROR_NOT_FOUND)
		mockHandle.On("GetMigDeviceHandleByIndex", 6).Return(nil, nvml.ERROR_NOT_FOUND)
		mockMigHandle.On("GetGpuInstanceId").Return(1, nvml.SUCCESS)

		dev := &nvmlDevice{index: 0, handle: mockHandle, lib: mockLib}
		instances, err := dev.GetMIGInstances()

		assert.NoError(t, err)
		assert.Len(t, instances, 1)
		assert.Equal(t, uint(1), instances[0].GPUInstanceID)

		mockHandle.AssertExpectations(t)
		mockMigHandle.AssertExpectations(t)
	})

	t.Run("no MIG instances found", func(t *testing.T) {
		mockLib := new(mockNvmlLib)
		mockHandle := new(mockDeviceHandle)

		mockHandle.On("GetMigMode").Return(nvml.DEVICE_MIG_ENABLE, 0, nvml.SUCCESS)
		mockHandle.On("GetMaxMigDeviceCount").Return(7, nvml.SUCCESS)
		for i := 0; i < 7; i++ {
			mockHandle.On("GetMigDeviceHandleByIndex", i).Return(nil, nvml.ERROR_NOT_FOUND)
		}

		dev := &nvmlDevice{index: 0, handle: mockHandle, lib: mockLib}
		_, err := dev.GetMIGInstances()

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no MIG instances found")

		mockHandle.AssertExpectations(t)
	})
}

func TestNVMLDevice_GetMIGDeviceByInstanceID(t *testing.T) {
	t.Run("MIG not enabled", func(t *testing.T) {
		mockLib := new(mockNvmlLib)
		mockHandle := new(mockDeviceHandle)

		mockHandle.On("GetMigMode").Return(nvml.DEVICE_MIG_DISABLE, 0, nvml.SUCCESS)

		dev := &nvmlDevice{index: 0, handle: mockHandle, lib: mockLib}
		_, err := dev.GetMIGDeviceByInstanceID(1)

		assert.Error(t, err)

		mockHandle.AssertExpectations(t)
	})

	t.Run("found instance", func(t *testing.T) {
		mockLib := new(mockNvmlLib)
		mockHandle := new(mockDeviceHandle)
		mockMigHandle := new(mockDeviceHandle)

		mockHandle.On("GetMigMode").Return(nvml.DEVICE_MIG_ENABLE, 0, nvml.SUCCESS)
		mockHandle.On("GetMaxMigDeviceCount").Return(7, nvml.SUCCESS)
		mockHandle.On("GetMigDeviceHandleByIndex", 0).Return(mockMigHandle, nvml.SUCCESS)
		mockMigHandle.On("GetGpuInstanceId").Return(5, nvml.SUCCESS)
		mockMigHandle.On("GetUUID").Return("MIG-UUID", nvml.SUCCESS)
		mockMigHandle.On("GetName").Return("MIG-Name", nvml.SUCCESS)

		dev := &nvmlDevice{index: 0, handle: mockHandle, lib: mockLib}
		migDev, err := dev.GetMIGDeviceByInstanceID(5)

		assert.NoError(t, err)
		assert.Equal(t, "MIG-UUID", migDev.UUID())
		assert.Equal(t, "MIG-Name", migDev.Name())

		mockHandle.AssertExpectations(t)
		mockMigHandle.AssertExpectations(t)
	})

	t.Run("not found", func(t *testing.T) {
		mockLib := new(mockNvmlLib)
		mockHandle := new(mockDeviceHandle)

		mockHandle.On("GetMigMode").Return(nvml.DEVICE_MIG_ENABLE, 0, nvml.SUCCESS)
		mockHandle.On("GetMaxMigDeviceCount").Return(7, nvml.SUCCESS)
		for i := 0; i < 7; i++ {
			mockHandle.On("GetMigDeviceHandleByIndex", i).Return(nil, nvml.ERROR_NOT_FOUND)
		}

		dev := &nvmlDevice{index: 0, handle: mockHandle, lib: mockLib}
		_, err := dev.GetMIGDeviceByInstanceID(99)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")

		mockHandle.AssertExpectations(t)
	})

	t.Run("empty name uses fallback", func(t *testing.T) {
		mockLib := new(mockNvmlLib)
		mockHandle := new(mockDeviceHandle)
		mockMigHandle := new(mockDeviceHandle)

		mockHandle.On("GetMigMode").Return(nvml.DEVICE_MIG_ENABLE, 0, nvml.SUCCESS)
		mockHandle.On("GetMaxMigDeviceCount").Return(7, nvml.SUCCESS)
		mockHandle.On("GetMigDeviceHandleByIndex", 0).Return(mockMigHandle, nvml.SUCCESS)
		mockMigHandle.On("GetGpuInstanceId").Return(1, nvml.SUCCESS)
		mockMigHandle.On("GetUUID").Return("", nvml.ERROR_UNKNOWN)
		mockMigHandle.On("GetName").Return("", nvml.ERROR_UNKNOWN)

		dev := &nvmlDevice{index: 0, handle: mockHandle, lib: mockLib}
		migDev, err := dev.GetMIGDeviceByInstanceID(1)

		assert.NoError(t, err)
		assert.Equal(t, "MIG-0-1", migDev.Name())

		mockHandle.AssertExpectations(t)
		mockMigHandle.AssertExpectations(t)
	})
}

func TestNVMLDevice_GetMaxMigDeviceCount(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mockLib := new(mockNvmlLib)
		mockHandle := new(mockDeviceHandle)

		mockHandle.On("GetMaxMigDeviceCount").Return(7, nvml.SUCCESS)

		dev := &nvmlDevice{handle: mockHandle, lib: mockLib}
		count, err := dev.GetMaxMigDeviceCount()

		assert.NoError(t, err)
		assert.Equal(t, 7, count)

		mockHandle.AssertExpectations(t)
	})

	t.Run("not supported", func(t *testing.T) {
		mockLib := new(mockNvmlLib)
		mockHandle := new(mockDeviceHandle)

		mockHandle.On("GetMaxMigDeviceCount").Return(0, nvml.ERROR_NOT_SUPPORTED)

		dev := &nvmlDevice{handle: mockHandle, lib: mockLib}
		count, err := dev.GetMaxMigDeviceCount()

		assert.NoError(t, err)
		assert.Equal(t, 0, count)

		mockHandle.AssertExpectations(t)
	})

	t.Run("error", func(t *testing.T) {
		mockLib := new(mockNvmlLib)
		mockHandle := new(mockDeviceHandle)

		mockHandle.On("GetMaxMigDeviceCount").Return(0, nvml.ERROR_UNKNOWN)
		mockLib.On("ErrorString", nvml.ERROR_UNKNOWN).Return("Unknown error")

		dev := &nvmlDevice{handle: mockHandle, lib: mockLib}
		_, err := dev.GetMaxMigDeviceCount()

		assert.Error(t, err)

		mockHandle.AssertExpectations(t)
		mockLib.AssertExpectations(t)
	})
}

func TestNVMLDevice_SimpleGetters(t *testing.T) {
	mockLib := new(mockNvmlLib)
	mockHandle := new(mockDeviceHandle)

	dev := &nvmlDevice{
		index:  5,
		handle: mockHandle,
		lib:    mockLib,
		uuid:   "GPU-TEST-UUID",
		name:   "Test GPU Name",
	}

	assert.Equal(t, 5, dev.Index())
	assert.Equal(t, "GPU-TEST-UUID", dev.UUID())
	assert.Equal(t, "Test GPU Name", dev.Name())
}
