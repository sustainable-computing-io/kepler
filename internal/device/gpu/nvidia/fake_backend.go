// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package nvidia

import (
	"fmt"
	"sync"
	"time"

	"github.com/sustainable-computing-io/kepler/internal/device"
	"github.com/sustainable-computing-io/kepler/internal/device/gpu"
)

// NOTE: This fake backend is not intended for production use. It is for
// development and testing only, similar to fakeRaplMeter for CPU power.

// FakeNVMLBackend implements NVMLBackend for testing without real GPUs.
type FakeNVMLBackend struct {
	mu      sync.RWMutex
	devices []*FakeNVMLDevice
}

// FakeNVMLDevice implements NVMLDevice with configurable behavior.
// Exported fields are set directly for test configuration (same pattern
// as fakeEnergyZone in fake_cpu_power_meter.go).
type FakeNVMLDevice struct {
	// Idx is the device index (0-based).
	Idx int

	// DeviceUUID is the unique identifier for this fake GPU.
	DeviceUUID string

	// DeviceName is the product name for this fake GPU.
	DeviceName string

	// DevicePower is the current power reading returned by GetPowerUsage.
	DevicePower device.Power

	// TotalEnergy is cumulative energy; auto-increments on each GetTotalEnergy call.
	TotalEnergy device.Energy

	// Mode is the compute mode (ComputeModeDefault for time-slicing,
	// ComputeModeExclusiveProcess for exclusive).
	Mode ComputeMode

	// MIG controls whether IsMIGEnabled returns true.
	MIG bool

	// Processes is the list of "running" GPU processes returned by
	// GetComputeRunningProcesses.
	Processes []gpu.ProcessGPUInfo

	// Utilization is the per-process utilization returned by GetProcessUtilization.
	Utilization []gpu.ProcessUtilization

	// UtilError, if set, is returned by GetProcessUtilization instead of Utilization.
	// This allows testing the time-slicing fallback path.
	UtilError error

	mu sync.RWMutex

	// energyIncrement is how much energy to add per GetTotalEnergy call.
	// Computed from DevicePower assuming a 5-second interval.
	energyIncrement device.Energy
}

// Verify interface compliance at compile time.
var (
	_ NVMLBackend = (*FakeNVMLBackend)(nil)
	_ NVMLDevice  = (*FakeNVMLDevice)(nil)
)

// NewFakeNVMLBackend creates a FakeNVMLBackend with the given devices.
func NewFakeNVMLBackend(devices ...*FakeNVMLDevice) *FakeNVMLBackend {
	return &FakeNVMLBackend{devices: devices}
}

func (b *FakeNVMLBackend) Init() error {
	return nil
}

func (b *FakeNVMLBackend) Shutdown() error {
	return nil
}

func (b *FakeNVMLBackend) DeviceCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.devices)
}

func (b *FakeNVMLBackend) GetDevice(index int) (NVMLDevice, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if index < 0 || index >= len(b.devices) {
		return nil, gpu.ErrGPUNotFound{DeviceIndex: index}
	}
	return b.devices[index], nil
}

func (b *FakeNVMLBackend) DiscoverDevices() ([]gpu.GPUDevice, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	devices := make([]gpu.GPUDevice, len(b.devices))
	for i, d := range b.devices {
		devices[i] = gpu.GPUDevice{
			Index:  d.Idx,
			UUID:   d.DeviceUUID,
			Name:   d.DeviceName,
			Vendor: gpu.VendorNVIDIA,
		}
	}
	return devices, nil
}

// FakeNVMLDevice method implementations

func (d *FakeNVMLDevice) Index() int {
	return d.Idx
}

func (d *FakeNVMLDevice) UUID() string {
	return d.DeviceUUID
}

func (d *FakeNVMLDevice) Name() string {
	return d.DeviceName
}

func (d *FakeNVMLDevice) GetPowerUsage() (device.Power, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.DevicePower, nil
}

func (d *FakeNVMLDevice) GetTotalEnergy() (device.Energy, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Auto-increment energy based on power (simulates 5s collection interval).
	if d.energyIncrement == 0 && d.DevicePower > 0 {
		// energy = power * time; 5s * watts -> joules
		d.energyIncrement = device.Energy(d.DevicePower.Watts() * 5 * float64(device.Joule))
	}
	d.TotalEnergy += d.energyIncrement
	return d.TotalEnergy, nil
}

func (d *FakeNVMLDevice) GetComputeRunningProcesses() ([]gpu.ProcessGPUInfo, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	// Return a copy to prevent mutation.
	procs := make([]gpu.ProcessGPUInfo, len(d.Processes))
	copy(procs, d.Processes)

	// Fill in timestamps if not set.
	now := time.Now()
	for i := range procs {
		if procs[i].Timestamp.IsZero() {
			procs[i].Timestamp = now
		}
		if procs[i].DeviceIndex == 0 && d.Idx != 0 {
			procs[i].DeviceIndex = d.Idx
		}
		if procs[i].DeviceUUID == "" {
			procs[i].DeviceUUID = d.DeviceUUID
		}
	}
	return procs, nil
}

func (d *FakeNVMLDevice) GetProcessUtilization(lastSeen uint64) ([]gpu.ProcessUtilization, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if d.UtilError != nil {
		return nil, d.UtilError
	}

	utils := make([]gpu.ProcessUtilization, len(d.Utilization))
	copy(utils, d.Utilization)
	return utils, nil
}

func (d *FakeNVMLDevice) GetComputeMode() (ComputeMode, error) {
	return d.Mode, nil
}

func (d *FakeNVMLDevice) IsMIGEnabled() (bool, error) {
	return d.MIG, nil
}

func (d *FakeNVMLDevice) GetMIGInstances() ([]MIGInstance, error) {
	return nil, nil
}

func (d *FakeNVMLDevice) GetMIGDeviceByInstanceID(gpuInstanceID uint) (NVMLDevice, error) {
	return nil, fmt.Errorf("MIG not supported on fake device")
}

func (d *FakeNVMLDevice) GetMaxMigDeviceCount() (int, error) {
	return 0, nil
}
