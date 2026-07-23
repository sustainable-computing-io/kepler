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

GPU partitioned into isolated instances. Both MIG detection **and** per-instance
power attribution are implemented, backed by DCGM (NVML returns N/A for MIG
power, so DCGM activity is used to split power across instances).

```go
// Partitioned (MIG) devices are attributed per instance:
case gpu.SharingModePartitioned:
    if err := c.attributePartitioned(dev.Index, result); err != nil {
        c.logger.Debug("MIG attribution failed", "device", dev.Index, "error", err)
    }
```

`attributePartitioned` works as follows:

- reads the device active power via the shared idle-detection logic;
- queries per-instance activity from DCGM (`GetMIGInstanceActivity`) using the
  cached MIG hierarchy, and skips NVML calls for instances whose activity is 0
  (idle instances) as an optimization;
- splits active power across instances proportionally to their activity and
  per-process SM utilization.

**Fallback:** when DCGM is unavailable (not deployed, unreachable, or not
initialized) or no MIG instances are cached, `attributePartitionedFallback`
distributes active power equally among the running processes so MIG devices
still report data.

## Idle Power Detection

Kepler approximates a device's idle power from the **minimum power observed
while the GPU is truly idle**, i.e. when no compute processes are running.
Restricting the baseline update to idle periods prevents a false, inflated
baseline when Kepler starts while the GPU is already under load.

This logic lives in `getDevicePowerStatsLocked`
([`collector.go`][collector-idle]). A failed
`GetComputeRunningProcesses()` call is non-fatal: idle detection is skipped
for that reading rather than corrupting the baseline.

```go
// Check if the GPU is truly idle (no compute processes running)
procs, err := dev.GetComputeRunningProcesses()
if err != nil {
    // Non-fatal: log and skip idle detection for this reading
    c.logger.Debug("GetComputeRunningProcesses failed, skipping idle detection",
        "device", deviceIndex, "error", err)
} else if len(procs) == 0 {
    // GPU is truly idle — update minimum observed power
    if min, exists := c.minObservedPower[uuid]; !exists || totalPower < min {
        c.minObservedPower[uuid] = totalPower
    }
    c.idleObserved[uuid] = true
}
```

Idle power is then resolved with the following precedence:

1. **User-configured idle power** — when set via `SetIdlePower(watts)` with a
   value `> 0`, it always takes precedence.
2. **Observed idle power** — `minObservedPower[uuid]`, used once a true idle
   period has been observed for the device.
3. **Conservative fallback** — `0`, so that until a real idle baseline exists,
   all power is attributed as active rather than guessing.

```go
var idlePower float64
switch {
case c.idlePower > 0:
    idlePower = c.idlePower
case c.idleObserved[uuid]:
    idlePower = c.minObservedPower[uuid]
default:
    idlePower = 0
}

activePower := totalPower - idlePower
if activePower < 0 {
    activePower = 0
}
```

**Active power** = Total power − Idle power (clamped at 0)

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

[collector-idle]: https://github.com/sustainable-computing-io/kepler/blob/7486e93d6793aca66c09fbf590e011493f3db046/internal/device/gpu/nvidia/collector.go#L234-L247
