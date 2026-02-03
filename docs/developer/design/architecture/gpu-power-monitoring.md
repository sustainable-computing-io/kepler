# GPU Power Monitoring

This document describes how Kepler monitors GPU power consumption, focusing on the NVIDIA implementation via NVML.

## Overview

GPU power monitoring differs fundamentally from CPU (RAPL) power monitoring:

| Aspect                  | CPU (RAPL)                        | GPU (NVML)                               |
|-------------------------|-----------------------------------|------------------------------------------|
| **Power data**          | Cumulative energy counters        | Instantaneous power                      |
| **Process attribution** | CPU time ratios from `/proc`      | SM utilization from driver               |
| **Delta calculation**   | Required (monitor computes)       | Not needed (hardware provides)           |
| **Interface**           | `CPUPowerMeter` with `EnergyZone` | `GPUPowerMeter` with `GetProcessPower()` |

## Architecture

```text
┌─────────────────────────────────────────────────────────────┐
│                      Monitor Layer                          │
│                  (internal/monitor/)                        │
│  - Calls GetProcessPower() for per-process GPU watts        │
│  - Calls GetDevicePowerStats() for device-level metrics     │
│  - Stores GPUPower in Process struct                        │
└─────────────────────────┬───────────────────────────────────┘
                          │
┌─────────────────────────▼───────────────────────────────────┐
│                    GPU Interface                            │
│              (internal/device/gpu/)                         │
│  - GPUPowerMeter interface                                  │
│  - Registry pattern for multi-vendor support                │
│  - Vendor-agnostic types (GPUDevice, GPUPowerStats)         │
└─────────────────────────┬───────────────────────────────────┘
                          │
┌─────────────────────────▼───────────────────────────────────┐
│                  NVIDIA Collector                           │
│           (internal/device/gpu/nvidia/)                     │
│  - NVML library wrapper                                     │
│  - Sharing mode detection (exclusive/time-slicing/MIG)      │
│  - Power attribution algorithms                             │
└─────────────────────────┬───────────────────────────────────┘
                          │
┌─────────────────────────▼───────────────────────────────────┐
│                    NVML Library                             │
│              (github.com/NVIDIA/go-nvml)                    │
│  - nvmlDeviceGetPowerUsage()                                │
│  - nvmlDeviceGetComputeRunningProcesses()                   │
│  - nvmlDeviceGetProcessUtilization()                        │
└─────────────────────────────────────────────────────────────┘
```

## GPU Sharing Modes

NVIDIA GPUs support different sharing configurations:

### 1. Exclusive Mode

One process has exclusive access to the GPU.

```go
// All active power attributed to the single process
powerPerProc := stats.ActivePower / float64(len(procs))
```

### 2. Time-Slicing Mode

Multiple processes share the GPU via time-division multiplexing.

```go
// Power distributed proportionally to SM utilization
for _, proc := range runningProcs {
    smUtil := utilMap[proc.PID]
    fraction := float64(smUtil) / float64(totalSmUtil)
    result[proc.PID] = stats.ActivePower * fraction
}
```

**Formula**: `P_process = P_active × (SM_util_process / Σ SM_util_all)`

### 3. MIG Mode (Multi-Instance GPU)

GPU partitioned into isolated instances. MIG detection is implemented, but **power attribution is not yet implemented**.

```go
// Detection works:
device.IsMIGEnabled()      // returns true if MIG enabled
device.GetMIGInstances()   // enumerates MIG partitions

// Attribution skipped:
case gpu.SharingModePartitioned:
    c.logger.Debug("partitioned mode detected, skipping (not yet implemented)")
    continue  // no power data returned for MIG devices
```

Per-instance power attribution requires DCGM integration (NVML returns N/A for MIG power). DCGM support is planned for upcoming PRs.

## Idle Power Detection

Kepler tracks minimum observed power per device as an approximation of idle power:

```go
if totalPower < c.minObservedPower[uuid] {
    c.minObservedPower[uuid] = totalPower
}
idlePower := c.minObservedPower[uuid]
activePower := totalPower - idlePower
```

**Active power** = Total power - Idle power

## Key NVML APIs Used

| API                                      | Purpose                                |
|------------------------------------------|----------------------------------------|
| `nvmlDeviceGetPowerUsage()`              | Current power consumption (milliwatts) |
| `nvmlDeviceGetComputeRunningProcesses()` | List of processes using GPU compute    |
| `nvmlDeviceGetProcessUtilization()`      | Per-process SM utilization samples     |
| `nvmlDeviceGetComputeMode()`             | Detect exclusive vs shared mode        |
| `nvmlDeviceGetMigMode()`                 | Detect if MIG is enabled               |

## Prometheus Metrics

| Metric                         | Description                            |
|--------------------------------|----------------------------------------|
| `kepler_node_gpu_info`         | Device metadata (UUID, name, vendor)   |
| `kepler_node_gpu_watts`        | Total GPU power                        |
| `kepler_node_gpu_idle_watts`   | Minimum observed power (idle estimate) |
| `kepler_node_gpu_active_watts` | Power above idle baseline              |
| `kepler_process_gpu_watts`     | Per-process GPU power attribution      |

## Why GPU Interface Differs from CPU

### 1. Energy vs Power

**RAPL (CPU)**: Returns cumulative energy counters. Kepler must:

- Read energy at T1 and T2
- Calculate ΔE = E2 - E1
- Derive power: P = ΔE / Δt

**NVML (GPU)**: Returns instantaneous power directly. No delta calculation needed.

### 2. Process Attribution Data

**CPU**: No per-process energy data from hardware. Kepler must:

- Read cumulative CPU time from `/proc/[pid]/stat`
- Calculate delta to get CPU time ratio
- Attribute power: `P_process = ratio × P_active`

**GPU**: NVML provides per-process SM utilization directly via `GetProcessUtilization()`.

### 3. Complexity Location

- **CPU**: Complex logic in monitor layer (delta calculations, ratio attribution)
- **GPU**: Complex logic in device layer (NVML calls, mode detection), simple pass-through in monitor

## Known Limitations

### 1. Idle Power Calibration

Idle power is estimated as minimum observed power. If Kepler starts while GPU is under load, the first reading becomes the "idle" baseline, causing inaccurate active power calculations.

**Mitigation**: Start Kepler before GPU workloads, or wait for a period of low GPU activity.

### 2. Time-Slicing Accuracy

`GetProcessUtilization()` returns sampled data, not continuous measurements. Rapid process switching may not be fully captured.

### 3. MIG Power Attribution Not Yet Implemented

Multi-Instance GPU mode is detected, but power attribution requires DCGM for per-partition metrics (NVML returns N/A for MIG power). DCGM integration is planned for upcoming PRs.

### 4. Single Vendor Per Node

The implementation assumes homogeneous GPU nodes (single vendor). While the code supports multiple meters, a process using both NVIDIA and AMD GPUs simultaneously is not expected.

## Code References

- **Interface**: `internal/device/gpu/interface.go`
- **Registry**: `internal/device/gpu/registry.go`
- **NVIDIA Collector**: `internal/device/gpu/nvidia/collector.go`
- **NVML Wrapper**: `internal/device/gpu/nvidia/nvml.go`
- **Monitor Integration**: `internal/monitor/process.go:115-150`
- **Prometheus Metrics**: `internal/exporter/prometheus/collector/power_collector.go`

## Future Work

1. **MIG Support**: Integrate with DCGM for per-instance power attribution (planned)
2. **Idle Power Model**: Linear regression from (utilization, power) pairs for better idle estimation
3. **AMD ROCm Support**: Implement `GPUPowerMeter` for AMD GPUs using ROCm SMI
4. **Intel GPU Support**: Implement for Intel discrete GPUs
