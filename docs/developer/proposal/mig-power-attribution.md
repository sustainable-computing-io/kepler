<!-- SPDX-FileCopyrightText: 2025 The Kepler Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->

# MIG Power Attribution Design

This document describes Kepler's approach to per-process GPU power attribution
for NVIDIA Multi-Instance GPU (MIG) enabled systems.

## Problem Statement

When MIG is enabled on NVIDIA GPUs (A100, H100, etc.), the physical GPU is
partitioned into isolated GPU Instances. This creates challenges for power
monitoring:

1. **Standard NVML utilization queries return "N/A"** - The `nvmlDeviceGetUtilizationRates()`
   API doesn't work at the MIG instance level
2. **Multiple processes share the same physical GPU** - Each MIG instance can
   run independent workloads that need separate power attribution
3. **Power is only reported at the board level** - NVML reports total GPU power,
   not per-MIG-instance power

## Architecture Overview

```text
┌─────────────────────────────────────────────────────────────────────┐
│                        GPUPowerCollector                            │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  ┌─────────────────┐        ┌─────────────────────────────────────┐ │
│  │   NVML Backend  │        │      dcgm-exporter Backend          │ │
│  │                 │        │                                     │ │
│  │ • Device enum   │        │ • HTTP client to dcgm-exporter      │ │
│  │ • Power usage   │        │ • MIG activity metrics              │ │
│  │ • MIG hierarchy │        │ • 2-second cached responses         │ │
│  │ • Process PIDs  │        │                                     │ │
│  └────────┬────────┘        └──────────────┬──────────────────────┘ │
│           │                                │                        │
│           ▼                                ▼                        │
│  ┌────────────────────────────────────────────────────────────────┐ │
│  │                    Hybrid Attribution Logic                    │ │
│  │                                                                │ │
│  │   if MIG mode:                                                 │ │
│  │     → Use DCGM activity + NVML process info                    │ │
│  │   else if Time-slicing:                                        │ │
│  │     → Use NVML GetProcessUtilization()                         │ │
│  │   else (Exclusive):                                            │ │
│  │     → 100% to single process                                   │ │
│  └────────────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────────┘
```

## Key Design Decisions

### 1. dcgm-exporter HTTP Backend (Not go-dcgm Library)

We query dcgm-exporter's Prometheus endpoint via HTTP rather than using the
`github.com/NVIDIA/go-dcgm` library.

**Why not go-dcgm**:

- The go-dcgm library requires `libdcgm.so.4` at runtime. This shared library
  is not available on the host filesystem - it's only present inside the DCGM
  container images deployed by the nvidia-gpu-operator
- Redistributing `datacenter-gpu-manager` binaries within Kepler's container
  image would conflict with NVIDIA's licensing terms
- While DCGM source is available, building it from source has significant
  complexity and maintenance burden

**Why dcgm-exporter HTTP**:

- dcgm-exporter is already deployed by NVIDIA GPU Operator as a DaemonSet
- No native library dependencies - just HTTP client
- Exposes the same DCGM metrics via standard Prometheus text format
- Clean dependency: Kepler queries an existing service rather than embedding proprietary libraries

**Trade-offs**:

- HTTP request latency (~1-5ms per request, mitigated by caching)
- Need to parse Prometheus text format
- Requires dcgm-exporter to be running (already required by GPU Operator)

### 2. MIG Hierarchy from NVML, Activity from dcgm-exporter

MIG topology (which instances exist on which GPUs) is enumerated from NVML at
startup. This is static configuration that only changes with admin intervention.

dcgm-exporter provides the `DCGM_FI_PROF_GR_ENGINE_ACTIVE` metric (Field ID 1001)
for each MIG instance. This metric measures the fraction of time (0.0-1.0) that
the compute engine was active during the sampling period. Standard NVML utilization
APIs don't work at the MIG instance level - they return "N/A" when MIG is enabled.

This activity ratio enables proportional power attribution across MIG instances:
if MIG instance A has activity 0.6 and instance B has activity 0.4, then A is
attributed 60% of the GPU's active power and B gets 40%. Within each MIG instance,
NVML identifies the running processes, and power is distributed among them.

```go
// At Init time - enumerate ALL MIG instances from NVML (static)
nvmlInstances, err := dev.GetMIGInstances()
c.migInstancesByDevice[deviceIndex] = nvmlInstances

// At collection time - get activity from dcgm-exporter (dynamic)
activity, err := c.dcgm.GetMIGInstanceActivity(deviceIndex, gi.GPUInstanceID)
```

This separation ensures we always know the complete MIG topology regardless of
workload state, while still getting accurate activity measurements for power
attribution.

### 3. Active Power Attribution (Not Total Power)

**Formula**:

```text
P_process = P_active × (activity_i / Σactivity) × (SmUtil_p / ΣSmUtil_instance)

Where:
  P_active = P_total - P_idle
  P_idle = min(observed_power) or configured value
```

**Rationale**: Idle power (cooling, memory refresh, etc.) exists regardless of
workloads. Only **active power** should be attributed to processes.

**Example**:

- GPU total power: 150W
- GPU idle power: 100W (auto-detected minimum)
- Active power: 50W
- Process gets portion of 50W, not 150W

### 4. Skip Idle MIG Instances (Performance Optimization)

NVML calls like `GetMIGDeviceByInstanceID()` and `GetProcessUtilization()` are
slow (~50-100ms each). With 8 GPUs × 7 MIG instances = 56 potential calls per
collection cycle.

**Optimization**: Check activity from dcgm-exporter first (cached, cheap).
Skip NVML calls for MIG instances with `activity == 0`.

```go
activity, _ := c.dcgm.GetMIGInstanceActivity(deviceIndex, gi.GPUInstanceID)
if activity == 0 {
    continue  // Skip expensive NVML calls
}
migDevice, _ := dev.GetMIGDeviceByInstanceID(gi.GPUInstanceID)
```

### 5. Singleflight for Concurrent Scrapes

Prometheus may send overlapping scrape requests (e.g., 5s interval with 1.5s
collection time). Concurrent NVML calls cause contention and gaps in metrics.

**Solution**: Use `singleflight` to coalesce concurrent `GetProcessPower()` calls.

```go
result, _, _ := c.processPowerGroup.Do("process-power", func() (interface{}, error) {
    return c.collectProcessPower(), nil
})
```

### 6. HTTP Response Caching (2-second TTL)

Without caching, querying activity for 56 MIG instances would make 56 HTTP
requests per collection cycle.

**Solution**: Cache dcgm-exporter response with 2-second TTL.

```go
const metricsCacheTTL = 2 * time.Second

func (d *DCGMExporterBackend) fetchMetrics() (*dcgmMetrics, error) {
    if d.cachedMetrics != nil && time.Since(d.cachedMetrics.timestamp) < metricsCacheTTL {
        return d.cachedMetrics, nil
    }
    // Fetch fresh metrics...
}
```

## Data Flow

### Collection Cycle

```text
1. Prometheus scrapes /metrics
   │
2. GetProcessPower() called (via singleflight)
   │
3. For each GPU device:
   │
   ├── Get total board power from NVML
   │
   ├── Calculate active power (total - idle)
   │
   ├── Get cached MIG instances (from NVML, at init)
   │
   └── For each MIG instance:
       │
       ├── Get activity from dcgm-exporter (cached HTTP)
       │
       ├── If activity == 0: skip (optimization)
       │
       ├── Get process PIDs from NVML (MIG device handle)
       │
       └── Attribute power:
           P_process = P_active × (activity / Σactivity)
```

### Endpoint Discovery

dcgm-exporter runs as a DaemonSet. Kepler discovers the local instance:

```text
1. Check NODE_NAME environment variable (Kubernetes Downward API)
   │
2. Query K8s API for pods with label "app=nvidia-dcgm-exporter"
   │
3. Filter to pods on same node, get Pod IP
   │
4. Use http://<pod-ip>:9400/metrics
   │
5. Fallback: localhost:9400 or ClusterIP service
```

## Metrics from dcgm-exporter

Key metric used:

```prometheus
DCGM_FI_PROF_GR_ENGINE_ACTIVE{
  gpu="0",
  GPU_I_ID="7",
  GPU_I_PROFILE="1g.5gb",
  UUID="GPU-xxx",
  container="pytorch",
  namespace="default",
  pod="my-workload-xxx"
} 0.85
```

Labels:

- `gpu`: Physical GPU index (0-7)
- `GPU_I_ID`: MIG GPU Instance ID within the parent GPU
- `GPU_I_PROFILE`: MIG profile (e.g., "1g.5gb", "3g.20gb")

## Power Conservation Validation

The implementation ensures power conservation:

```text
Σ(process_gpu_watts) ≈ active_watts

Where:
  active_watts = total_watts - idle_watts
```

Tested with 10 workloads across 8 MIG-enabled GPUs:

- GPU 3 with 2 processes: 52.46W + 52.43W ≈ 105W active
- Total active_watts: 460W
- Sum of process_gpu_watts: 462W (within measurement tolerance)

## File Structure

```text
internal/device/gpu/nvidia/
├── collector.go       # Main GPUPowerCollector, hybrid attribution logic
├── collector_test.go  # Unit tests
├── dcgm_exporter.go   # HTTP backend for dcgm-exporter
├── detector.go        # GPU sharing mode detection
├── mig_types.go       # DCGMBackend interface, MIG types
└── nvml.go           # NVML backend wrapper
```

## Configuration

```yaml
gpu:
  # dcgm-exporter endpoint (auto-discovered if not set)
  dcgm_exporter_endpoint: ""

  # Idle power per device (auto-detected if not set)
  idle_power: {}
```

## Future Improvements

1. **Profile-based idle power**: Different MIG profiles may have different
   idle power characteristics

2. **H100/H200 support**: Field 1001 may not be available on newer GPUs
   (DCGM issue #226). Fall back to Field 1002 (SM_ACTIVE)

3. **MIG profile slice ratios**: Currently equal distribution within MIG
   instance. Could use slice ratio for more accurate attribution

## References

- [NVIDIA MIG User Guide](https://docs.nvidia.com/datacenter/tesla/mig-user-guide/)
- [DCGM Field Identifiers](https://docs.nvidia.com/datacenter/dcgm/latest/dcgm-api/dcgm-api-field-ids.html)
- [dcgm-exporter](https://github.com/NVIDIA/dcgm-exporter)
- [NVML API Reference](https://docs.nvidia.com/deploy/nvml-api/)
