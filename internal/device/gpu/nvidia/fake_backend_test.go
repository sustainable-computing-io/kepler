// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package nvidia

import (
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/sustainable-computing-io/kepler/internal/device"
	"github.com/sustainable-computing-io/kepler/internal/device/gpu"
)

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

// TestFakeNVMLBackend_ThroughRealCollector verifies the fake backend wires
// correctly through the real GPUPowerCollector (Init + Devices + GetProcessPower).
func TestFakeNVMLBackend_ThroughRealCollector(t *testing.T) {
	backend := NewFakeNVMLBackend(&FakeNVMLDevice{
		Idx:         0,
		DeviceUUID:  "FAKE-GPU-0000",
		DeviceName:  "Fake NVIDIA GPU",
		DevicePower: 225 * device.Watt,
		Mode:        ComputeModeDefault,
		Processes: []gpu.ProcessGPUInfo{
			{PID: 1001},
		},
		Utilization: []gpu.ProcessUtilization{
			{PID: 1001, ComputeUtil: 80},
		},
	})

	collector, err := NewGPUPowerCollector(slog.Default(), WithNVMLBackend(backend))
	require.NoError(t, err)

	err = collector.Init()
	require.NoError(t, err)

	// Verify devices discovered
	assert.Len(t, collector.Devices(), 1)
	assert.Equal(t, "FAKE-GPU-0000", collector.Devices()[0].UUID)

	// Verify GetProcessPower returns data through the real collector logic
	power, err := collector.GetProcessPower()
	require.NoError(t, err)
	assert.NotEmpty(t, power, "GetProcessPower should return data for fake process")
	assert.Contains(t, power, uint32(1001))

	assert.NoError(t, collector.Shutdown())
}
