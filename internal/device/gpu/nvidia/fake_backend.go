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

// PIDSource supplies the PIDs the fake GPU should treat as compute processes.
// An empty or nil slice means the GPU is idle. Implementations must be safe
// for concurrent PIDs() calls.
type PIDSource interface {
	PIDs() []uint32
}

// FakeNVMLBackend implements NVMLBackend for testing without real GPUs.
type FakeNVMLBackend struct {
	mu      sync.RWMutex
	devices []*FakeNVMLDevice
}

// FakeNVMLDevice implements NVMLDevice with configurable behavior.
// Set exported fields before concurrent use. Callers must not read or write
// them directly while device methods are running. Methods synchronize internal
// updates to cumulative energy and, when PIDSource is set, to power and process
// lists.
//
// When PIDSource is nil, DevicePower, Processes, and Utilization are used as
// static values (the original fake-device behaviour). When PIDSource is set,
// each power/process/utilization read refreshes those fields from IdlePower or
// LoadPower and UtilizationTemplate.
type FakeNVMLDevice struct {
	// Idx is the device index (0-based).
	Idx int

	// DeviceUUID is the unique identifier for this fake GPU.
	DeviceUUID string

	// DeviceName is the product name for this fake GPU.
	DeviceName string

	// DevicePower is the current power reading returned by GetPowerUsage.
	DevicePower device.Power

	// TotalEnergy is cumulative energy; GetTotalEnergy integrates DevicePower
	// over the elapsed time since its previous call.
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

	// PIDSource, if set, is consulted on each power/process/utilization read.
	// Empty PIDs select IdlePower and empty process lists; non-empty PIDs select
	// LoadPower and per-PID process/utilization samples.
	PIDSource PIDSource

	// IdlePower is reported while PIDSource returns no PIDs.
	IdlePower device.Power

	// LoadPower is reported while PIDSource returns one or more PIDs.
	LoadPower device.Power

	// UtilizationTemplate is applied to PIDSource PIDs in order when building
	// GetProcessUtilization samples. Extra PIDs beyond the template get PID-only
	// samples (zero utilization).
	UtilizationTemplate []gpu.ProcessUtilization

	mu sync.RWMutex

	lastEnergyRead time.Time
	now            func() time.Time
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

func (d *FakeNVMLDevice) currentTimeLocked() time.Time {
	if d.now != nil {
		return d.now()
	}
	return time.Now()
}

// collectPIDs reads PIDSource under a short lock, then calls PIDs() without
// holding d.mu so procfs scans do not block other device methods.
func (d *FakeNVMLDevice) collectPIDs() (hasSource bool, pids []uint32) {
	d.mu.Lock()
	src := d.PIDSource
	d.mu.Unlock()
	if src == nil {
		return false, nil
	}
	return true, src.PIDs()
}

// integratePendingLocked adds elapsed*DevicePower since lastEnergyRead.
// Caller must hold d.mu.
func (d *FakeNVMLDevice) integratePendingLocked(now time.Time) {
	if !d.lastEnergyRead.IsZero() {
		elapsed := now.Sub(d.lastEnergyRead).Seconds()
		if elapsed > 0 && d.DevicePower > 0 {
			d.TotalEnergy += device.Energy(d.DevicePower.Watts() * elapsed * float64(device.Joule))
		}
	}
	d.lastEnergyRead = now
}

// applyPIDsLocked updates power and process samples from a PID snapshot.
// Caller must hold d.mu. hasSource false leaves DevicePower, Processes, and
// Utilization unchanged. Pending energy is integrated at the pre-refresh power
// before DevicePower changes.
func (d *FakeNVMLDevice) applyPIDsLocked(hasSource bool, pids []uint32) {
	if !hasSource {
		return
	}

	d.integratePendingLocked(d.currentTimeLocked())

	if len(pids) == 0 {
		d.DevicePower = d.IdlePower
		d.Processes = nil
		d.Utilization = nil
		return
	}

	d.DevicePower = d.LoadPower
	procs := make([]gpu.ProcessGPUInfo, len(pids))
	utils := make([]gpu.ProcessUtilization, len(pids))
	for i, pid := range pids {
		procs[i] = gpu.ProcessGPUInfo{
			PID:         pid,
			DeviceIndex: d.Idx,
			DeviceUUID:  d.DeviceUUID,
		}
		utils[i] = gpu.ProcessUtilization{PID: pid}
		if i < len(d.UtilizationTemplate) {
			tmpl := d.UtilizationTemplate[i]
			utils[i].ComputeUtil = tmpl.ComputeUtil
			utils[i].MemUtil = tmpl.MemUtil
			utils[i].EncUtil = tmpl.EncUtil
			utils[i].DecUtil = tmpl.DecUtil
			utils[i].Timestamp = tmpl.Timestamp
		}
	}
	d.Processes = procs
	d.Utilization = utils
}

func (d *FakeNVMLDevice) GetPowerUsage() (device.Power, error) {
	hasSource, pids := d.collectPIDs()
	d.mu.Lock()
	defer d.mu.Unlock()
	d.applyPIDsLocked(hasSource, pids)
	return d.DevicePower, nil
}

func (d *FakeNVMLDevice) GetTotalEnergy() (device.Energy, error) {
	hasSource, pids := d.collectPIDs()

	d.mu.Lock()
	defer d.mu.Unlock()

	now := d.currentTimeLocked()
	// Integrate the elapsed interval at the power that was in effect before this
	// refresh. Idle time stays idle energy even if PIDSource now reports load.
	d.integratePendingLocked(now)
	d.applyPIDsLocked(hasSource, pids)
	return d.TotalEnergy, nil
}

func (d *FakeNVMLDevice) GetComputeRunningProcesses() ([]gpu.ProcessGPUInfo, error) {
	hasSource, pids := d.collectPIDs()
	d.mu.Lock()
	defer d.mu.Unlock()
	d.applyPIDsLocked(hasSource, pids)

	// Return a copy to prevent mutation.
	procs := make([]gpu.ProcessGPUInfo, len(d.Processes))
	copy(procs, d.Processes)

	now := d.currentTimeLocked()
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
	hasSource, pids := d.collectPIDs()
	d.mu.Lock()
	defer d.mu.Unlock()
	d.applyPIDsLocked(hasSource, pids)

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
