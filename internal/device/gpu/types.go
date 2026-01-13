// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package gpu

import "fmt"

// Vendor represents the GPU manufacturer
type Vendor string

const (
	VendorNVIDIA  Vendor = "nvidia"
	VendorAMD     Vendor = "amd"
	VendorIntel   Vendor = "intel"
	VendorUnknown Vendor = "unknown"
)

// SharingMode represents how a GPU is shared among processes
type SharingMode int

const (
	// SharingModeUnknown indicates the sharing mode could not be determined
	SharingModeUnknown SharingMode = iota

	// SharingModeExclusive indicates a single process has exclusive access to the GPU.
	// Power attribution: 100% to the single process.
	SharingModeExclusive

	// SharingModeTimeSlicing indicates multiple processes share the GPU via time-slicing.
	// Power attribution: Proportional to compute utilization.
	SharingModeTimeSlicing

	// SharingModePartitioned indicates the GPU is partitioned into isolated instances.
	// For NVIDIA: Multi-Instance GPU (MIG)
	// Power attribution: Proportional to partition size and activity within each instance.
	SharingModePartitioned
)

// String returns a human-readable name for the sharing mode
func (m SharingMode) String() string {
	switch m {
	case SharingModeExclusive:
		return "exclusive"
	case SharingModeTimeSlicing:
		return "time-slicing"
	case SharingModePartitioned:
		return "partitioned"
	default:
		return "unknown"
	}
}

// ProcessUtilization holds per-process GPU utilization metrics
type ProcessUtilization struct {
	// PID is the process ID
	PID uint32

	// ComputeUtil is the compute unit utilization percentage (0-100)
	ComputeUtil uint32

	// MemUtil is the memory utilization percentage (0-100)
	MemUtil uint32

	// EncUtil is the encoder utilization percentage (0-100)
	EncUtil uint32

	// DecUtil is the decoder utilization percentage (0-100)
	DecUtil uint32

	// Timestamp is the measurement time in microseconds
	Timestamp uint64
}

// ErrGPUNotFound is returned when a GPU device is not found
type ErrGPUNotFound struct {
	DeviceIndex int
}

func (e ErrGPUNotFound) Error() string {
	return fmt.Sprintf("GPU device not found: index %d", e.DeviceIndex)
}

// ErrGPUNotInitialized is returned when GPU operations are attempted before initialization
type ErrGPUNotInitialized struct{}

func (e ErrGPUNotInitialized) Error() string {
	return "GPU power meter not initialized"
}

// ErrPartitioningNotSupported is returned when partitioning operations are attempted on unsupported hardware
type ErrPartitioningNotSupported struct {
	DeviceIndex int
}

func (e ErrPartitioningNotSupported) Error() string {
	return fmt.Sprintf("GPU partitioning not supported on device: index %d", e.DeviceIndex)
}

// ErrProcessUtilizationUnavailable is returned when per-process utilization cannot be obtained
type ErrProcessUtilizationUnavailable struct {
	Reason string
}

func (e ErrProcessUtilizationUnavailable) Error() string {
	return fmt.Sprintf("process utilization unavailable: %s", e.Reason)
}
