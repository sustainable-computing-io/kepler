// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package nvidia

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/sustainable-computing-io/kepler/internal/device"
	"github.com/sustainable-computing-io/kepler/internal/device/gpu"
)

func TestFakeNVMLBackend_InterfaceCompliance(t *testing.T) {
	// Compile-time checks are in fake_backend.go; this verifies at runtime.
	var _ NVMLBackend = (*FakeNVMLBackend)(nil)
	var _ NVMLDevice = (*FakeNVMLDevice)(nil)
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
