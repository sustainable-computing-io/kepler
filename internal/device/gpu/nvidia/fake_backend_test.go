// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package nvidia

import (
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/sustainable-computing-io/kepler/internal/device"
	"github.com/sustainable-computing-io/kepler/internal/device/gpu"
)

// stubPIDSource is a test PIDSource that tests can flip between idle and load.
type stubPIDSource struct {
	mu   sync.Mutex
	pids []uint32
}

func (s *stubPIDSource) PIDs() []uint32 {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]uint32, len(s.pids))
	copy(out, s.pids)
	return out
}

func (s *stubPIDSource) set(pids ...uint32) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pids = append([]uint32(nil), pids...)
}

func TestFakeNVMLBackend_DiscoverDevices(t *testing.T) {
	backend := NewFakeNVMLBackend(
		&FakeNVMLDevice{Idx: 0, DeviceUUID: "FAKE-GPU-0000", DeviceName: "Fake GPU 0"},
		&FakeNVMLDevice{Idx: 1, DeviceUUID: "FAKE-GPU-0001", DeviceName: "Fake GPU 1"},
	)

	assert.NoError(t, backend.Init())
	assert.Equal(t, 2, backend.DeviceCount())

	devices, err := backend.DiscoverDevices()
	require.NoError(t, err)
	assert.Len(t, devices, 2)
	assert.Equal(t, "FAKE-GPU-0000", devices[0].UUID)
	assert.Equal(t, gpu.VendorNVIDIA, devices[0].Vendor)

	assert.NoError(t, backend.Shutdown())
}

func TestFakeNVMLBackend_GetDevice(t *testing.T) {
	backend := NewFakeNVMLBackend(
		&FakeNVMLDevice{Idx: 0, DeviceUUID: "FAKE-GPU-0000"},
	)

	dev, err := backend.GetDevice(0)
	require.NoError(t, err)
	assert.Equal(t, "FAKE-GPU-0000", dev.UUID())

	_, err = backend.GetDevice(99)
	assert.Error(t, err)
	_, err = backend.GetDevice(-1)
	assert.Error(t, err)
}

func TestFakeNVMLDeviceMetadataAndPower(t *testing.T) {
	dev := &FakeNVMLDevice{
		Idx:         1,
		DeviceUUID:  "FAKE-GPU-0001",
		DeviceName:  "Fake GPU 1",
		DevicePower: 225 * device.Watt,
		Mode:        ComputeModeExclusiveProcess,
		MIG:         true,
	}

	assert.Equal(t, 1, dev.Index())
	assert.Equal(t, "FAKE-GPU-0001", dev.UUID())
	assert.Equal(t, "Fake GPU 1", dev.Name())
	power, err := dev.GetPowerUsage()
	require.NoError(t, err)
	assert.Equal(t, 225*device.Watt, power)

	mode, err := dev.GetComputeMode()
	require.NoError(t, err)
	assert.Equal(t, ComputeModeExclusiveProcess, mode)
	migEnabled, err := dev.IsMIGEnabled()
	require.NoError(t, err)
	assert.True(t, migEnabled)
	instances, err := dev.GetMIGInstances()
	require.NoError(t, err)
	assert.Nil(t, instances)
	_, err = dev.GetMIGDeviceByInstanceID(0)
	assert.ErrorContains(t, err, "MIG not supported")
	maxDevices, err := dev.GetMaxMigDeviceCount()
	require.NoError(t, err)
	assert.Zero(t, maxDevices)
}

func TestFakeNVMLDeviceTotalEnergy(t *testing.T) {
	now := time.Unix(1_000, 0)
	dev := &FakeNVMLDevice{
		DevicePower: 225 * device.Watt,
		TotalEnergy: 10 * device.Joule,
		now:         func() time.Time { return now },
	}

	energy, err := dev.GetTotalEnergy()
	require.NoError(t, err)
	assert.Equal(t, 10*device.Joule, energy, "first read establishes the energy baseline")
	now = now.Add(3 * time.Second)
	energy, err = dev.GetTotalEnergy()
	require.NoError(t, err)
	assert.Equal(t, 685*device.Joule, energy, "225 W over 3 s should add 675 J")
}

func TestFakeNVMLDeviceComputeRunningProcesses(t *testing.T) {
	now := time.Unix(1_000, 0)
	dev := &FakeNVMLDevice{
		Idx:        1,
		DeviceUUID: "FAKE-GPU-0001",
		Processes:  []gpu.ProcessGPUInfo{{PID: 1001}},
		now:        func() time.Time { return now },
	}

	processes, err := dev.GetComputeRunningProcesses()
	require.NoError(t, err)
	require.Len(t, processes, 1)
	assert.Equal(t, 1, processes[0].DeviceIndex)
	assert.Equal(t, "FAKE-GPU-0001", processes[0].DeviceUUID)
	assert.Equal(t, now, processes[0].Timestamp)
	processes[0].PID = 9999
	assert.Equal(t, uint32(1001), dev.Processes[0].PID, "returned processes must be a copy")
}

func TestFakeNVMLDeviceProcessUtilization(t *testing.T) {
	utilErr := errors.New("utilization unavailable")
	dev := &FakeNVMLDevice{
		Utilization: []gpu.ProcessUtilization{{PID: 1001, ComputeUtil: 80}},
	}

	utilization, err := dev.GetProcessUtilization(0)
	require.NoError(t, err)
	require.Len(t, utilization, 1)
	utilization[0].PID = 9999
	assert.Equal(t, uint32(1001), dev.Utilization[0].PID, "returned utilization must be a copy")

	dev.UtilError = utilErr
	_, err = dev.GetProcessUtilization(0)
	assert.ErrorIs(t, err, utilErr)
}

func TestFakeNVMLDevice_PIDSourceIdleThenLoad(t *testing.T) {
	src := &stubPIDSource{}
	dev := &FakeNVMLDevice{
		Idx:        1,
		DeviceUUID: "FAKE-GPU-0001",
		IdlePower:  40 * device.Watt,
		LoadPower:  225 * device.Watt,
		PIDSource:  src,
		UtilizationTemplate: []gpu.ProcessUtilization{
			{ComputeUtil: 80},
			{ComputeUtil: 20},
		},
	}

	power, err := dev.GetPowerUsage()
	require.NoError(t, err)
	assert.Equal(t, 40*device.Watt, power)

	procs, err := dev.GetComputeRunningProcesses()
	require.NoError(t, err)
	assert.Empty(t, procs)
	utils, err := dev.GetProcessUtilization(0)
	require.NoError(t, err)
	assert.Empty(t, utils)

	src.set(1001, 1002)

	power, err = dev.GetPowerUsage()
	require.NoError(t, err)
	assert.Equal(t, 225*device.Watt, power)

	procs, err = dev.GetComputeRunningProcesses()
	require.NoError(t, err)
	require.Len(t, procs, 2)
	assert.Equal(t, uint32(1001), procs[0].PID)
	assert.Equal(t, uint32(1002), procs[1].PID)
	assert.Equal(t, 1, procs[0].DeviceIndex)
	assert.Equal(t, "FAKE-GPU-0001", procs[0].DeviceUUID)
	procs[0].PID = 9999
	procs[1].PID = 8888

	again, err := dev.GetComputeRunningProcesses()
	require.NoError(t, err)
	assert.Equal(t, uint32(1001), again[0].PID, "returned processes must be a copy")
	assert.Equal(t, uint32(1002), again[1].PID)

	utils, err = dev.GetProcessUtilization(0)
	require.NoError(t, err)
	require.Len(t, utils, 2)
	assert.Equal(t, uint32(80), utils[0].ComputeUtil)
	assert.Equal(t, uint32(20), utils[1].ComputeUtil)
	utils[0].PID = 9999
	utils[0].ComputeUtil = 1

	utilsAgain, err := dev.GetProcessUtilization(0)
	require.NoError(t, err)
	assert.Equal(t, uint32(1001), utilsAgain[0].PID, "returned utilization must be a copy")
	assert.Equal(t, uint32(80), utilsAgain[0].ComputeUtil)
}

func TestFakeNVMLDevice_EnergyAcrossIdleToLoad(t *testing.T) {
	now := time.Unix(1_000, 0)
	src := &stubPIDSource{}
	dev := &FakeNVMLDevice{
		IdlePower: 40 * device.Watt,
		LoadPower: 225 * device.Watt,
		PIDSource: src,
		now:       func() time.Time { return now },
	}

	baseline, err := dev.GetTotalEnergy()
	require.NoError(t, err)
	assert.Equal(t, device.Energy(0), baseline, "first read establishes the energy baseline")

	now = now.Add(2 * time.Second)
	src.set(1001, 1002)

	afterIdle, err := dev.GetTotalEnergy()
	require.NoError(t, err)
	assert.Equal(t, 80*device.Joule, afterIdle-baseline, "idle interval must be integrated at 40 W")
	assert.Equal(t, 225*device.Watt, dev.DevicePower, "refresh after integrate must switch to load")

	now = now.Add(3 * time.Second)
	afterLoad, err := dev.GetTotalEnergy()
	require.NoError(t, err)
	assert.Equal(t, 675*device.Joule, afterLoad-afterIdle, "load interval must be integrated at 225 W")
}

func TestFakeNVMLDevice_EnergyMonitorOrder(t *testing.T) {
	now := time.Unix(1_000, 0)
	src := &stubPIDSource{}
	dev := &FakeNVMLDevice{
		IdlePower: 40 * device.Watt,
		LoadPower: 225 * device.Watt,
		PIDSource: src,
		now:       func() time.Time { return now },
	}

	baseline, err := dev.GetTotalEnergy()
	require.NoError(t, err)
	assert.Equal(t, device.Energy(0), baseline, "first read establishes the energy baseline")

	now = now.Add(2 * time.Second)
	src.set(1001, 1002)

	_, err = dev.GetPowerUsage()
	require.NoError(t, err)
	_, err = dev.GetComputeRunningProcesses()
	require.NoError(t, err)
	_, err = dev.GetProcessUtilization(0)
	require.NoError(t, err)

	afterIdle, err := dev.GetTotalEnergy()
	require.NoError(t, err)
	assert.Equal(t, 80*device.Joule, afterIdle-baseline, "idle interval must be integrated at 40 W")
	assert.Equal(t, 225*device.Watt, dev.DevicePower, "refresh after integrate must switch to load")

	now = now.Add(3 * time.Second)
	afterLoad, err := dev.GetTotalEnergy()
	require.NoError(t, err)
	assert.Equal(t, 675*device.Joule, afterLoad-afterIdle, "load interval must be integrated at 225 W")
}

func fakeCollectorDevice(mode ComputeMode, src PIDSource) *FakeNVMLDevice {
	return &FakeNVMLDevice{
		Idx:        0,
		DeviceUUID: "FAKE-GPU-0000",
		DeviceName: "Fake NVIDIA GPU",
		IdlePower:  40 * device.Watt,
		LoadPower:  225 * device.Watt,
		Mode:       mode,
		PIDSource:  src,
		UtilizationTemplate: []gpu.ProcessUtilization{
			{ComputeUtil: 80},
			{ComputeUtil: 20},
		},
	}
}

func observeIdleThenLoad(t *testing.T, collector *GPUPowerCollector, src *stubPIDSource) map[uint32]float64 {
	t.Helper()

	stats, err := collector.GetDevicePowerStats(0)
	require.NoError(t, err)
	assert.Equal(t, 40.0, stats.TotalPower)
	assert.Equal(t, 40.0, stats.IdlePower)
	assert.Equal(t, 0.0, stats.ActivePower)

	src.set(1001, 1002)

	power, err := collector.GetProcessPower()
	require.NoError(t, err)
	return power
}

// TestFakeNVMLBackend_ThroughRealCollector verifies the fake backend wires
// correctly through the real GPUPowerCollector (Init + Devices + GetProcessPower).
func TestFakeNVMLBackend_ThroughRealCollector(t *testing.T) {
	t.Run("time-slicing unequal utilization", func(t *testing.T) {
		src := &stubPIDSource{}
		collector, err := NewGPUPowerCollector(slog.Default(), WithNVMLBackend(
			NewFakeNVMLBackend(fakeCollectorDevice(ComputeModeDefault, src)),
		))
		require.NoError(t, err)
		require.NoError(t, collector.Init())
		t.Cleanup(func() { assert.NoError(t, collector.Shutdown()) })

		assert.Len(t, collector.Devices(), 1)
		assert.Equal(t, "FAKE-GPU-0000", collector.Devices()[0].UUID)

		power := observeIdleThenLoad(t, collector, src)
		require.Len(t, power, 2)
		assert.InDelta(t, 148.0, power[1001], 0.01) // 80% of 185 W active
		assert.InDelta(t, 37.0, power[1002], 0.01)  // 20% of 185 W active

		stats, err := collector.GetDevicePowerStats(0)
		require.NoError(t, err)
		assert.InDelta(t, stats.ActivePower, power[1001]+power[1002], 0.01)
	})

	t.Run("exclusive equal split", func(t *testing.T) {
		src := &stubPIDSource{}
		collector, err := NewGPUPowerCollector(slog.Default(), WithNVMLBackend(
			NewFakeNVMLBackend(fakeCollectorDevice(ComputeModeExclusiveProcess, src)),
		))
		require.NoError(t, err)
		require.NoError(t, collector.Init())
		t.Cleanup(func() { assert.NoError(t, collector.Shutdown()) })

		power := observeIdleThenLoad(t, collector, src)
		require.Len(t, power, 2)
		assert.InDelta(t, 92.5, power[1001], 0.01) // half of 185 W active
		assert.InDelta(t, 92.5, power[1002], 0.01)

		stats, err := collector.GetDevicePowerStats(0)
		require.NoError(t, err)
		assert.InDelta(t, stats.ActivePower, power[1001]+power[1002], 0.01)
	})
}

func TestFakeNVMLDevice_ConcurrentPIDSourceFlip(t *testing.T) {
	src := &stubPIDSource{}
	dev := &FakeNVMLDevice{
		IdlePower: 40 * device.Watt,
		LoadPower: 225 * device.Watt,
		PIDSource: src,
		UtilizationTemplate: []gpu.ProcessUtilization{
			{ComputeUtil: 80},
			{ComputeUtil: 20},
		},
		now: func() time.Time { return time.Now() },
	}

	var stop atomic.Bool
	var wg sync.WaitGroup
	run := func(fn func()) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for !stop.Load() {
				fn()
			}
		}()
	}

	run(func() { _, _ = dev.GetPowerUsage() })
	run(func() { _, _ = dev.GetTotalEnergy() })
	run(func() { _, _ = dev.GetComputeRunningProcesses() })
	run(func() { _, _ = dev.GetProcessUtilization(0) })
	run(func() {
		src.set(1001, 1002)
		src.set()
	})

	time.Sleep(100 * time.Millisecond)
	stop.Store(true)
	wg.Wait()
}
