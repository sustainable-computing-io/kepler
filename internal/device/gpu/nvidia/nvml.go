// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package nvidia

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/NVIDIA/go-nvml/pkg/nvml"
	"github.com/sustainable-computing-io/kepler/internal/device"
	"github.com/sustainable-computing-io/kepler/internal/device/gpu"
)

// MIGInstance represents a Multi-Instance GPU partition (NVIDIA-specific)
type MIGInstance struct {
	// EntityID is the MIG device index within the parent GPU
	EntityID uint

	// GPUInstanceID is the GPU Instance ID
	GPUInstanceID uint

	// ComputeInstances are the compute instances within this GPU instance
	ComputeInstances []ComputeInstance

	// ProfileSlices is the number of GPU slices allocated to this instance
	ProfileSlices uint
}

// ComputeInstance represents a compute instance within a MIG GPU instance
type ComputeInstance struct {
	// EntityID is the compute instance entity ID
	EntityID uint

	// ComputeInstanceID is the compute instance ID
	ComputeInstanceID uint
}

// NVMLBackend provides access to NVIDIA GPUs via the NVML library.
// It is used for:
//   - Device discovery and power readings (all scenarios)
//   - Per-process utilization via GetProcessUtilization() (time-slicing scenario)
//   - MIG mode detection
//
// Thread-safety: All methods are safe for concurrent use.
type NVMLBackend interface {
	Init() error
	Shutdown() error
	DeviceCount() int
	GetDevice(index int) (NVMLDevice, error)
	DiscoverDevices() ([]gpu.GPUDevice, error)
}

// NVMLDevice wraps operations on a single NVIDIA GPU device
type NVMLDevice interface {
	Index() int
	UUID() string
	Name() string
	GetPowerUsage() (device.Power, error)
	GetTotalEnergy() (device.Energy, error)
	GetComputeRunningProcesses() ([]gpu.ProcessGPUInfo, error)
	GetProcessUtilization(lastSeen uint64) ([]gpu.ProcessUtilization, error)
	GetComputeMode() (ComputeMode, error)
	IsMIGEnabled() (bool, error)
	GetMIGInstances() ([]MIGInstance, error)
	GetMIGDeviceByInstanceID(gpuInstanceID uint) (NVMLDevice, error)
	GetMaxMigDeviceCount() (int, error)
}

// nvmlBackend is the concrete implementation of NVMLBackend
type nvmlBackend struct {
	logger      *slog.Logger
	lib         nvmlLib
	devices     []nvmlDevice
	initialized bool
	mu          sync.RWMutex
}

// nvmlDevice wraps a single NVML device handle
type nvmlDevice struct {
	index  int
	handle nvmlDeviceHandle
	lib    nvmlLib
	uuid   string
	name   string
}

// NewNVMLBackend creates a new NVML backend instance
func NewNVMLBackend(logger *slog.Logger) NVMLBackend {
	return newNVMLBackendWithLib(logger, newRealNvmlLib())
}

// newNVMLBackendWithLib creates an NVML backend with a specific library implementation.
// This is used for testing with mock implementations.
func newNVMLBackendWithLib(logger *slog.Logger, lib nvmlLib) *nvmlBackend {
	if logger == nil {
		logger = slog.Default()
	}
	return &nvmlBackend{
		logger: logger.With("component", "nvml"),
		lib:    lib,
	}
}

// Init initializes the NVML library and discovers all GPU devices
func (n *nvmlBackend) Init() error {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.initialized {
		return nil
	}

	ret := n.lib.Init()
	if ret != nvml.SUCCESS {
		return fmt.Errorf("NVML init failed: %s", n.lib.ErrorString(ret))
	}

	count, ret := n.lib.DeviceGetCount()
	if ret != nvml.SUCCESS {
		_ = n.lib.Shutdown()
		return fmt.Errorf("failed to get device count: %s", n.lib.ErrorString(ret))
	}

	n.devices = make([]nvmlDevice, 0, count)
	for i := 0; i < count; i++ {
		handle, ret := n.lib.DeviceGetHandleByIndex(i)
		if ret != nvml.SUCCESS {
			n.logger.Warn("failed to get device handle", "index", i, "error", n.lib.ErrorString(ret))
			continue
		}

		uuid, ret := handle.GetUUID()
		if ret != nvml.SUCCESS {
			uuid = fmt.Sprintf("gpu-%d", i)
		}

		name, ret := handle.GetName()
		if ret != nvml.SUCCESS {
			name = "Unknown NVIDIA GPU"
		}

		n.devices = append(n.devices, nvmlDevice{
			index:  i,
			handle: handle,
			lib:    n.lib,
			uuid:   uuid,
			name:   name,
		})

		n.logger.Info("discovered GPU", "index", i, "uuid", uuid, "name", name)
	}

	n.initialized = true
	n.logger.Info("NVML initialized", "device_count", len(n.devices))
	return nil
}

// Shutdown cleans up NVML resources
func (n *nvmlBackend) Shutdown() error {
	n.mu.Lock()
	defer n.mu.Unlock()

	if !n.initialized {
		return nil
	}

	ret := n.lib.Shutdown()
	if ret != nvml.SUCCESS {
		return fmt.Errorf("NVML shutdown failed: %s", n.lib.ErrorString(ret))
	}

	n.devices = nil
	n.initialized = false
	n.logger.Info("NVML shutdown complete")
	return nil
}

// DeviceCount returns the number of discovered GPU devices
func (n *nvmlBackend) DeviceCount() int {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return len(n.devices)
}

// GetDevice returns an NVMLDevice for the given index
func (n *nvmlBackend) GetDevice(index int) (NVMLDevice, error) {
	n.mu.RLock()
	defer n.mu.RUnlock()

	if !n.initialized {
		return nil, gpu.ErrGPUNotInitialized{}
	}

	if index < 0 || index >= len(n.devices) {
		return nil, gpu.ErrGPUNotFound{DeviceIndex: index}
	}

	return &n.devices[index], nil
}

// DiscoverDevices returns GPU device information for all discovered devices
func (n *nvmlBackend) DiscoverDevices() ([]gpu.GPUDevice, error) {
	n.mu.RLock()
	defer n.mu.RUnlock()

	if !n.initialized {
		return nil, gpu.ErrGPUNotInitialized{}
	}

	devices := make([]gpu.GPUDevice, len(n.devices))
	for i, dev := range n.devices {
		devices[i] = gpu.GPUDevice{
			Index:  dev.index,
			UUID:   dev.uuid,
			Name:   dev.name,
			Vendor: gpu.VendorNVIDIA,
		}
	}

	return devices, nil
}

// Index returns the device index
func (d *nvmlDevice) Index() int {
	return d.index
}

// UUID returns the device UUID
func (d *nvmlDevice) UUID() string {
	return d.uuid
}

// Name returns the device name
func (d *nvmlDevice) Name() string {
	return d.name
}

// GetPowerUsage returns the current power consumption in Watts
func (d *nvmlDevice) GetPowerUsage() (device.Power, error) {
	// NVML returns power in milliwatts
	powerMW, ret := d.handle.GetPowerUsage()
	if ret != nvml.SUCCESS {
		return 0, fmt.Errorf("failed to get power usage: %s", d.lib.ErrorString(ret))
	}

	// Convert milliwatts to device.Power (which is in microwatts)
	return device.Power(powerMW) * device.MilliWatt, nil
}

// GetTotalEnergy returns cumulative energy consumption in Joules
func (d *nvmlDevice) GetTotalEnergy() (device.Energy, error) {
	// NVML returns energy in millijoules
	energyMJ, ret := d.handle.GetTotalEnergyConsumption()
	if ret != nvml.SUCCESS {
		return 0, fmt.Errorf("failed to get total energy: %s", d.lib.ErrorString(ret))
	}

	// Convert millijoules to device.Energy (which is in microjoules)
	return device.Energy(energyMJ) * device.MilliJoule, nil
}

// GetComputeRunningProcesses returns processes currently using the GPU for compute
func (d *nvmlDevice) GetComputeRunningProcesses() ([]gpu.ProcessGPUInfo, error) {
	procs, ret := d.handle.GetComputeRunningProcesses()
	if ret != nvml.SUCCESS {
		return nil, fmt.Errorf("failed to get running processes: %s", d.lib.ErrorString(ret))
	}

	now := time.Now()
	result := make([]gpu.ProcessGPUInfo, len(procs))
	for i, p := range procs {
		result[i] = gpu.ProcessGPUInfo{
			PID:         p.Pid,
			DeviceIndex: d.index,
			DeviceUUID:  d.uuid,
			MemoryUsed:  p.UsedGpuMemory,
			Timestamp:   now,
		}
	}

	return result, nil
}

// GetProcessUtilization returns per-process SM and memory utilization.
// This is the key API for time-slicing power attribution.
//
// Parameters:
//   - lastSeen: timestamp (microseconds) from previous call, or 0 for first call
//
// Returns slice of ProcessUtilization with SmUtil normalized across all processes.
func (d *nvmlDevice) GetProcessUtilization(lastSeen uint64) ([]gpu.ProcessUtilization, error) {
	samples, ret := d.handle.GetProcessUtilization(lastSeen)
	if ret == nvml.SUCCESS {
		result := make([]gpu.ProcessUtilization, len(samples))
		for i, s := range samples {
			result[i] = gpu.ProcessUtilization{
				PID:         s.Pid,
				ComputeUtil: s.SmUtil,
				MemUtil:     s.MemUtil,
				EncUtil:     s.EncUtil,
				DecUtil:     s.DecUtil,
				Timestamp:   s.TimeStamp,
			}
		}
		return result, nil
	}

	// Check if accounting mode is the issue
	mode, accRet := d.handle.GetAccountingMode()
	if accRet == nvml.SUCCESS && mode == nvml.FEATURE_DISABLED {
		return nil, gpu.ErrProcessUtilizationUnavailable{
			Reason: "process utilization requires accounting mode or driver 450+; accounting mode is disabled",
		}
	}

	return nil, gpu.ErrProcessUtilizationUnavailable{
		Reason: fmt.Sprintf("GetProcessUtilization failed: %s", d.lib.ErrorString(ret)),
	}
}

// GetComputeMode returns the GPU's compute mode configuration.
func (d *nvmlDevice) GetComputeMode() (ComputeMode, error) {
	mode, ret := d.handle.GetComputeMode()
	if ret != nvml.SUCCESS {
		return ComputeModeDefault, fmt.Errorf("failed to get compute mode: %s", d.lib.ErrorString(ret))
	}

	switch mode {
	case nvml.COMPUTEMODE_DEFAULT:
		return ComputeModeDefault, nil
	case nvml.COMPUTEMODE_EXCLUSIVE_THREAD:
		return ComputeModeExclusiveThread, nil
	case nvml.COMPUTEMODE_EXCLUSIVE_PROCESS:
		return ComputeModeExclusiveProcess, nil
	case nvml.COMPUTEMODE_PROHIBITED:
		return ComputeModeProhibited, nil
	default:
		return ComputeModeDefault, nil
	}
}

// IsMIGEnabled checks if Multi-Instance GPU mode is enabled on this device
func (d *nvmlDevice) IsMIGEnabled() (bool, error) {
	currentMode, _, ret := d.handle.GetMigMode()
	if ret == nvml.ERROR_NOT_SUPPORTED {
		return false, nil
	}
	if ret != nvml.SUCCESS {
		return false, fmt.Errorf("failed to get MIG mode: %s", d.lib.ErrorString(ret))
	}

	return currentMode == nvml.DEVICE_MIG_ENABLE, nil
}

// GetMIGInstances returns all MIG GPU instances on this device
func (d *nvmlDevice) GetMIGInstances() ([]MIGInstance, error) {
	migEnabled, err := d.IsMIGEnabled()
	if err != nil {
		return nil, err
	}
	if !migEnabled {
		return nil, gpu.ErrPartitioningNotSupported{DeviceIndex: d.index}
	}

	return d.enumerateMIGInstances()
}

// enumerateMIGInstances discovers MIG instances by iterating through possible indices
func (d *nvmlDevice) enumerateMIGInstances() ([]MIGInstance, error) {
	maxCount, err := d.GetMaxMigDeviceCount()
	if err != nil {
		return nil, fmt.Errorf("failed to get max MIG device count: %w", err)
	}
	if maxCount == 0 {
		return nil, fmt.Errorf("MIG not supported on this device")
	}

	var instances []MIGInstance
	for i := 0; i < maxCount; i++ {
		migDevice, ret := d.handle.GetMigDeviceHandleByIndex(i)
		if ret != nvml.SUCCESS {
			continue
		}

		giID, ret := migDevice.GetGpuInstanceId()
		if ret != nvml.SUCCESS {
			continue
		}

		instances = append(instances, MIGInstance{
			GPUInstanceID: uint(giID),
			EntityID:      uint(i),
		})
	}

	if len(instances) == 0 {
		return nil, fmt.Errorf("no MIG instances found")
	}

	return instances, nil
}

// GetMIGDeviceByInstanceID returns a MIG device by its GPU Instance ID.
func (d *nvmlDevice) GetMIGDeviceByInstanceID(gpuInstanceID uint) (NVMLDevice, error) {
	migEnabled, err := d.IsMIGEnabled()
	if err != nil {
		return nil, err
	}
	if !migEnabled {
		return nil, gpu.ErrPartitioningNotSupported{DeviceIndex: d.index}
	}

	// Iterate through MIG devices to find the one with matching GPU Instance ID
	maxCount, err := d.GetMaxMigDeviceCount()
	if err != nil {
		return nil, fmt.Errorf("failed to get max MIG device count: %w", err)
	}

	for i := 0; i < maxCount; i++ {
		migHandle, ret := d.handle.GetMigDeviceHandleByIndex(i)
		if ret != nvml.SUCCESS {
			continue
		}

		giID, ret := migHandle.GetGpuInstanceId()
		if ret != nvml.SUCCESS {
			continue
		}

		if uint(giID) == gpuInstanceID {
			uuid, _ := migHandle.GetUUID()
			name, _ := migHandle.GetName()
			if name == "" {
				name = fmt.Sprintf("MIG-%d-%d", d.index, gpuInstanceID)
			}

			return &nvmlDevice{
				index:  d.index,
				handle: migHandle,
				lib:    d.lib,
				uuid:   uuid,
				name:   name,
			}, nil
		}
	}

	return nil, fmt.Errorf("MIG instance with GPU Instance ID %d not found", gpuInstanceID)
}

// GetMaxMigDeviceCount returns the maximum number of MIG devices for this GPU.
// Returns 0 if MIG is not supported.
func (d *nvmlDevice) GetMaxMigDeviceCount() (int, error) {
	count, ret := d.handle.GetMaxMigDeviceCount()
	if ret == nvml.ERROR_NOT_SUPPORTED {
		return 0, nil
	}
	if ret != nvml.SUCCESS {
		return 0, fmt.Errorf("failed to get max MIG device count: %s", d.lib.ErrorString(ret))
	}
	return count, nil
}
