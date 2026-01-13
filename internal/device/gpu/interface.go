// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package gpu

import (
	"time"

	"github.com/sustainable-computing-io/kepler/internal/device"
	"github.com/sustainable-computing-io/kepler/internal/service"
)

// GPUDevice represents a single GPU device with its properties.
// This struct is vendor-agnostic - vendor-specific details (like NVIDIA MIG)
// are handled internally by each vendor's implementation.
type GPUDevice struct {
	// Index is the device index as reported by the GPU driver (0-based).
	// For NVIDIA: corresponds to NVML device index from nvmlDeviceGetHandleByIndex()
	// For AMD: corresponds to ROCm device index
	Index int

	// UUID is the globally unique identifier for this GPU
	UUID string

	// Name is the product name (e.g., "NVIDIA A100-SXM4-40GB", "AMD MI250X")
	Name string

	// Vendor identifies the GPU manufacturer
	Vendor Vendor
}

// GPUPowerStats contains power statistics for a GPU device
type GPUPowerStats struct {
	// TotalPower is the current total power consumption in Watts
	TotalPower float64

	// IdlePower is the detected or configured idle power in Watts
	IdlePower float64

	// ActivePower is the power attributed to workloads (TotalPower - IdlePower) in Watts
	ActivePower float64
}

// GPUPowerMeter is the interface for GPU power measurement and process attribution.
// Implementations must be thread-safe for concurrent access.
type GPUPowerMeter interface {
	service.Service     // Name()
	service.Initializer // Init()
	service.Shutdowner  // Shutdown()

	// Vendor returns the GPU vendor (nvidia, amd, intel)
	Vendor() Vendor

	// Devices returns all discovered GPU devices
	Devices() []GPUDevice

	// GetPowerUsage returns the current power consumption for a specific device in Watts
	GetPowerUsage(deviceIndex int) (device.Power, error)

	// GetTotalEnergy returns the cumulative energy consumption for a device in Joules
	GetTotalEnergy(deviceIndex int) (device.Energy, error)

	// GetDevicePowerStats returns power statistics including idle power detection
	GetDevicePowerStats(deviceIndex int) (GPUPowerStats, error)

	// GetProcessPower returns power attribution per process.
	// The map key is PID and value is power in Watts.
	GetProcessPower() (map[uint32]float64, error)

	// GetProcessInfo returns detailed GPU metrics per process
	GetProcessInfo() ([]ProcessGPUInfo, error)
}

// ProcessGPUInfo contains per-process GPU metrics collected from the device.
// This struct is vendor-agnostic.
type ProcessGPUInfo struct {
	// PID is the process ID using the GPU
	PID uint32

	// DeviceIndex is the GPU device index (0-based)
	DeviceIndex int

	// DeviceUUID is the unique identifier of the GPU device
	DeviceUUID string

	// ComputeUtil is the compute utilization ratio (0.0-1.0)
	// For NVIDIA: SM (Streaming Multiprocessor) utilization
	// For AMD: CU (Compute Unit) utilization
	ComputeUtil float64

	// MemoryUsed is the GPU memory used by this process in bytes
	MemoryUsed uint64

	// Timestamp is when this measurement was taken
	Timestamp time.Time
}
