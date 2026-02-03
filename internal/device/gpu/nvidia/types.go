// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package nvidia

// ComputeMode represents NVIDIA GPU's compute mode configuration.
// Maps directly to nvmlComputeMode_t from NVML.
type ComputeMode int

const (
	// ComputeModeDefault allows multiple processes to share the GPU (time-slicing)
	ComputeModeDefault ComputeMode = 0

	// ComputeModeExclusiveThread allows only one compute thread (legacy mode)
	ComputeModeExclusiveThread ComputeMode = 1

	// ComputeModeExclusiveProcess allows only one compute process
	ComputeModeExclusiveProcess ComputeMode = 2

	// ComputeModeProhibited disallows compute processes
	ComputeModeProhibited ComputeMode = 3
)

// String returns a human-readable name for the compute mode
func (m ComputeMode) String() string {
	switch m {
	case ComputeModeDefault:
		return "default"
	case ComputeModeExclusiveThread:
		return "exclusive-thread"
	case ComputeModeExclusiveProcess:
		return "exclusive-process"
	case ComputeModeProhibited:
		return "prohibited"
	default:
		return "unknown"
	}
}
