// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package gpu

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestVendorConstants(t *testing.T) {
	tests := []struct {
		name     string
		vendor   Vendor
		expected string
	}{
		{"nvidia", VendorNVIDIA, "nvidia"},
		{"amd", VendorAMD, "amd"},
		{"intel", VendorIntel, "intel"},
		{"unknown", VendorUnknown, "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, string(tt.vendor))
		})
	}
}

func TestSharingMode_String(t *testing.T) {
	tests := []struct {
		name     string
		mode     SharingMode
		expected string
	}{
		{"exclusive", SharingModeExclusive, "exclusive"},
		{"time-slicing", SharingModeTimeSlicing, "time-slicing"},
		{"partitioned", SharingModePartitioned, "partitioned"},
		{"unknown", SharingModeUnknown, "unknown"},
		{"invalid negative", SharingMode(-1), "unknown"},
		{"invalid large", SharingMode(100), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.mode.String())
		})
	}
}

func TestSharingMode_IotaValues(t *testing.T) {
	// Verify iota ordering
	assert.Equal(t, SharingMode(0), SharingModeUnknown)
	assert.Equal(t, SharingMode(1), SharingModeExclusive)
	assert.Equal(t, SharingMode(2), SharingModeTimeSlicing)
	assert.Equal(t, SharingMode(3), SharingModePartitioned)
}

func TestErrGPUNotFound_Error(t *testing.T) {
	tests := []struct {
		name        string
		deviceIndex int
		expected    string
	}{
		{"device 0", 0, "GPU device not found: index 0"},
		{"device 1", 1, "GPU device not found: index 1"},
		{"device 42", 42, "GPU device not found: index 42"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ErrGPUNotFound{DeviceIndex: tt.deviceIndex}
			assert.Equal(t, tt.expected, err.Error())
		})
	}
}

func TestErrGPUNotInitialized_Error(t *testing.T) {
	err := ErrGPUNotInitialized{}
	assert.Equal(t, "GPU power meter not initialized", err.Error())
}

func TestErrPartitioningNotSupported_Error(t *testing.T) {
	tests := []struct {
		name        string
		deviceIndex int
		expected    string
	}{
		{"device 0", 0, "GPU partitioning not supported on device: index 0"},
		{"device 3", 3, "GPU partitioning not supported on device: index 3"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ErrPartitioningNotSupported{DeviceIndex: tt.deviceIndex}
			assert.Equal(t, tt.expected, err.Error())
		})
	}
}

func TestErrProcessUtilizationUnavailable_Error(t *testing.T) {
	tests := []struct {
		name     string
		reason   string
		expected string
	}{
		{"driver error", "driver not responding", "process utilization unavailable: driver not responding"},
		{"no permission", "insufficient permissions", "process utilization unavailable: insufficient permissions"},
		{"empty reason", "", "process utilization unavailable: "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ErrProcessUtilizationUnavailable{Reason: tt.reason}
			assert.Equal(t, tt.expected, err.Error())
		})
	}
}

func TestErrorTypes_ImplementErrorInterface(t *testing.T) {
	// Compile-time check that all error types implement error interface
	var _ error = ErrGPUNotFound{}
	var _ error = ErrGPUNotInitialized{}
	var _ error = ErrPartitioningNotSupported{}
	var _ error = ErrProcessUtilizationUnavailable{}

	// Runtime check
	errors := []error{
		ErrGPUNotFound{DeviceIndex: 0},
		ErrGPUNotInitialized{},
		ErrPartitioningNotSupported{DeviceIndex: 0},
		ErrProcessUtilizationUnavailable{Reason: "test"},
	}

	for _, err := range errors {
		assert.NotEmpty(t, err.Error())
	}
}

func TestProcessUtilization(t *testing.T) {
	t.Run("zero value", func(t *testing.T) {
		var pu ProcessUtilization
		assert.Equal(t, uint32(0), pu.PID)
		assert.Equal(t, uint32(0), pu.ComputeUtil)
		assert.Equal(t, uint32(0), pu.MemUtil)
		assert.Equal(t, uint32(0), pu.EncUtil)
		assert.Equal(t, uint32(0), pu.DecUtil)
		assert.Equal(t, uint64(0), pu.Timestamp)
	})

	t.Run("populated", func(t *testing.T) {
		pu := ProcessUtilization{
			PID:         1234,
			ComputeUtil: 75,
			MemUtil:     50,
			EncUtil:     25,
			DecUtil:     10,
			Timestamp:   1704067200000000, // microseconds
		}

		assert.Equal(t, uint32(1234), pu.PID)
		assert.Equal(t, uint32(75), pu.ComputeUtil)
		assert.Equal(t, uint32(50), pu.MemUtil)
		assert.Equal(t, uint32(25), pu.EncUtil)
		assert.Equal(t, uint32(10), pu.DecUtil)
		assert.Equal(t, uint64(1704067200000000), pu.Timestamp)
	})

	t.Run("max utilization values", func(t *testing.T) {
		pu := ProcessUtilization{
			PID:         1,
			ComputeUtil: 100,
			MemUtil:     100,
			EncUtil:     100,
			DecUtil:     100,
		}

		assert.Equal(t, uint32(100), pu.ComputeUtil)
		assert.Equal(t, uint32(100), pu.MemUtil)
		assert.Equal(t, uint32(100), pu.EncUtil)
		assert.Equal(t, uint32(100), pu.DecUtil)
	})
}

func TestGPUDevice(t *testing.T) {
	tests := []struct {
		name   string
		device GPUDevice
	}{
		{
			name: "nvidia device",
			device: GPUDevice{
				Index:  0,
				UUID:   "GPU-12345678-abcd-1234-abcd-123456789012",
				Name:   "NVIDIA A100-SXM4-40GB",
				Vendor: VendorNVIDIA,
			},
		},
		{
			name: "amd device",
			device: GPUDevice{
				Index:  1,
				UUID:   "AMD-GPU-001",
				Name:   "AMD MI250X",
				Vendor: VendorAMD,
			},
		},
		{
			name: "intel device",
			device: GPUDevice{
				Index:  0,
				UUID:   "INTEL-GPU-001",
				Name:   "Intel Data Center GPU Max 1550",
				Vendor: VendorIntel,
			},
		},
		{
			name: "unknown vendor",
			device: GPUDevice{
				Index:  0,
				UUID:   "UNKNOWN-001",
				Name:   "Generic GPU",
				Vendor: VendorUnknown,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.GreaterOrEqual(t, tt.device.Index, 0)
			assert.NotEmpty(t, tt.device.UUID)
			assert.NotEmpty(t, tt.device.Name)
			assert.NotEmpty(t, string(tt.device.Vendor))
		})
	}
}

func TestGPUPowerStats(t *testing.T) {
	t.Run("zero power", func(t *testing.T) {
		stats := GPUPowerStats{
			TotalPower:  0,
			IdlePower:   0,
			ActivePower: 0,
		}

		assert.Equal(t, 0.0, stats.TotalPower)
		assert.Equal(t, 0.0, stats.IdlePower)
		assert.Equal(t, 0.0, stats.ActivePower)
	})

	t.Run("typical power values", func(t *testing.T) {
		stats := GPUPowerStats{
			TotalPower:  250.0, // 250W total
			IdlePower:   50.0,  // 50W idle
			ActivePower: 200.0, // 200W active (250 - 50)
		}

		assert.Equal(t, 250.0, stats.TotalPower)
		assert.Equal(t, 50.0, stats.IdlePower)
		assert.Equal(t, 200.0, stats.ActivePower)
		// Verify ActivePower = TotalPower - IdlePower
		assert.Equal(t, stats.TotalPower-stats.IdlePower, stats.ActivePower)
	})

	t.Run("idle only", func(t *testing.T) {
		stats := GPUPowerStats{
			TotalPower:  50.0,
			IdlePower:   50.0,
			ActivePower: 0.0,
		}

		assert.Equal(t, stats.IdlePower, stats.TotalPower)
		assert.Equal(t, 0.0, stats.ActivePower)
	})
}

func TestProcessGPUInfo(t *testing.T) {
	t.Run("zero value", func(t *testing.T) {
		var info ProcessGPUInfo
		assert.Equal(t, uint32(0), info.PID)
		assert.Equal(t, 0, info.DeviceIndex)
		assert.Empty(t, info.DeviceUUID)
		assert.Equal(t, 0.0, info.ComputeUtil)
		assert.Equal(t, uint64(0), info.MemoryUsed)
		assert.True(t, info.Timestamp.IsZero())
	})

	t.Run("populated", func(t *testing.T) {
		now := time.Now()
		info := ProcessGPUInfo{
			PID:         5678,
			DeviceIndex: 0,
			DeviceUUID:  "GPU-12345678",
			ComputeUtil: 0.85,                   // 85%
			MemoryUsed:  4 * 1024 * 1024 * 1024, // 4GB
			Timestamp:   now,
		}

		assert.Equal(t, uint32(5678), info.PID)
		assert.Equal(t, 0, info.DeviceIndex)
		assert.Equal(t, "GPU-12345678", info.DeviceUUID)
		assert.Equal(t, 0.85, info.ComputeUtil)
		assert.Equal(t, uint64(4*1024*1024*1024), info.MemoryUsed)
		assert.Equal(t, now, info.Timestamp)
	})

	t.Run("compute util range", func(t *testing.T) {
		// ComputeUtil is a ratio from 0.0 to 1.0
		tests := []struct {
			name string
			util float64
		}{
			{"zero", 0.0},
			{"half", 0.5},
			{"full", 1.0},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				info := ProcessGPUInfo{ComputeUtil: tt.util}
				assert.GreaterOrEqual(t, info.ComputeUtil, 0.0)
				assert.LessOrEqual(t, info.ComputeUtil, 1.0)
			})
		}
	})
}
