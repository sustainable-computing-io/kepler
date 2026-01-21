// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package nvidia

import (
	"github.com/NVIDIA/go-nvml/pkg/nvml"
)

// nvmlLib abstracts the NVML library functions for testability.
// This allows mocking NVML calls in unit tests.
type nvmlLib interface {
	Init() nvml.Return
	Shutdown() nvml.Return
	DeviceGetCount() (int, nvml.Return)
	DeviceGetHandleByIndex(index int) (nvmlDeviceHandle, nvml.Return)
	ErrorString(ret nvml.Return) string
}

// nvmlDeviceHandle abstracts operations on an NVML device handle.
type nvmlDeviceHandle interface {
	GetUUID() (string, nvml.Return)
	GetName() (string, nvml.Return)
	GetPowerUsage() (uint32, nvml.Return)
	GetTotalEnergyConsumption() (uint64, nvml.Return)
	GetComputeRunningProcesses() ([]nvml.ProcessInfo, nvml.Return)
	GetProcessUtilization(lastSeen uint64) ([]nvml.ProcessUtilizationSample, nvml.Return)
	GetComputeMode() (nvml.ComputeMode, nvml.Return)
	GetMigMode() (int, int, nvml.Return)
	GetMigDeviceHandleByIndex(index int) (nvmlDeviceHandle, nvml.Return)
	GetGpuInstanceId() (int, nvml.Return)
	GetMaxMigDeviceCount() (int, nvml.Return)
	GetAccountingMode() (nvml.EnableState, nvml.Return)
}

// realNvmlLib is the production implementation that calls the actual NVML library.
type realNvmlLib struct{}

// realDeviceHandle wraps an actual nvml.Device
type realDeviceHandle struct {
	device nvml.Device
}

// newRealNvmlLib creates a new real NVML library wrapper.
func newRealNvmlLib() nvmlLib {
	return &realNvmlLib{}
}

func (r *realNvmlLib) Init() nvml.Return {
	return nvml.Init()
}

func (r *realNvmlLib) Shutdown() nvml.Return {
	return nvml.Shutdown()
}

func (r *realNvmlLib) DeviceGetCount() (int, nvml.Return) {
	return nvml.DeviceGetCount()
}

func (r *realNvmlLib) DeviceGetHandleByIndex(index int) (nvmlDeviceHandle, nvml.Return) {
	handle, ret := nvml.DeviceGetHandleByIndex(index)
	if ret != nvml.SUCCESS {
		return nil, ret
	}
	return &realDeviceHandle{device: handle}, ret
}

func (r *realNvmlLib) ErrorString(ret nvml.Return) string {
	return nvml.ErrorString(ret)
}

func (h *realDeviceHandle) GetUUID() (string, nvml.Return) {
	return h.device.GetUUID()
}

func (h *realDeviceHandle) GetName() (string, nvml.Return) {
	return h.device.GetName()
}

func (h *realDeviceHandle) GetPowerUsage() (uint32, nvml.Return) {
	return h.device.GetPowerUsage()
}

func (h *realDeviceHandle) GetTotalEnergyConsumption() (uint64, nvml.Return) {
	return h.device.GetTotalEnergyConsumption()
}

func (h *realDeviceHandle) GetComputeRunningProcesses() ([]nvml.ProcessInfo, nvml.Return) {
	return h.device.GetComputeRunningProcesses()
}

func (h *realDeviceHandle) GetProcessUtilization(lastSeen uint64) ([]nvml.ProcessUtilizationSample, nvml.Return) {
	return h.device.GetProcessUtilization(lastSeen)
}

func (h *realDeviceHandle) GetComputeMode() (nvml.ComputeMode, nvml.Return) {
	return h.device.GetComputeMode()
}

func (h *realDeviceHandle) GetMigMode() (int, int, nvml.Return) {
	return h.device.GetMigMode()
}

func (h *realDeviceHandle) GetMigDeviceHandleByIndex(index int) (nvmlDeviceHandle, nvml.Return) {
	handle, ret := h.device.GetMigDeviceHandleByIndex(index)
	if ret != nvml.SUCCESS {
		return nil, ret
	}
	return &realDeviceHandle{device: handle}, ret
}

func (h *realDeviceHandle) GetGpuInstanceId() (int, nvml.Return) {
	return h.device.GetGpuInstanceId()
}

func (h *realDeviceHandle) GetMaxMigDeviceCount() (int, nvml.Return) {
	return h.device.GetMaxMigDeviceCount()
}

func (h *realDeviceHandle) GetAccountingMode() (nvml.EnableState, nvml.Return) {
	return h.device.GetAccountingMode()
}
