// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package nvidia

import "context"

// DCGMBackend provides access to MIG metrics for power attribution.
//
// Why we need MIG-specific metrics:
// When MIG is enabled, standard NVML utilization queries return "N/A" because
// the physical GPU is partitioned into isolated GPU Instances. We need to query
// metrics at the GPU Instance level to attribute power to processes.
//
// The key metric is DCGM_FI_PROF_GR_ENGINE_ACTIVE (Field 1001) which measures
// the graphics/compute engine activity ratio (0.0-1.0) for each MIG instance.
//
// Implementations:
//   - DCGMExporterBackend: Queries dcgm-exporter's Prometheus endpoint (no library dependency)
type DCGMBackend interface {
	Init(ctx context.Context) error
	Shutdown() error
	IsInitialized() bool

	// GetMIGInstanceActivity returns the GR_ENGINE_ACTIVE metric for a MIG instance.
	// Returns a value 0.0-1.0 representing the activity ratio.
	GetMIGInstanceActivity(ctx context.Context, gpuIndex int, gpuInstanceID uint) (float64, error)
}

// MIGGPUInstance represents a GPU Instance in MIG mode
type MIGGPUInstance struct {
	// ParentGPUIndex is the index of the physical GPU that contains this MIG instance
	ParentGPUIndex int

	// GPUInstanceID is the unique identifier for this GPU Instance within the parent GPU
	GPUInstanceID uint
}
