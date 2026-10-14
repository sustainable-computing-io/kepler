<!-- SPDX-FileCopyrightText: 2025 The Kepler Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->

# Composition-Based Power Meter Architecture

- **Status**: Draft (unnumbered, for community review)
- **Author**: Vimal Kumar
- **Created**: 2026-01-28
- **Last Updated**: 2026-01-28

## Summary

Refactor Kepler's power monitoring internals from separate, device-specific code paths (CPU via RAPL/HWMON, GPU via NVML) into a unified composition-based architecture. Every hardware device (CPU, GPU, DRAM, uncore, platform) becomes a `Meter` composed of a power **Source** and a device-specific **ResourceInformer** for per-process attribution. This eliminates special-casing in the monitor layer, enables GPU power on containers/VMs/pods (currently missing), and makes adding new device types (DRAM, uncore, platform) straightforward.

## Problem Statement

The current architecture has separate code paths for CPU and GPU power monitoring. The `PowerMonitor` treats CPU zones and GPU meters as fundamentally different entities, leading to:

1. **GPU power flows through a parallel attribution path** — `GPUPower` is a flat `float64` field on `Process`, `Container`, `VM`, and `Pod` (aggregated through duplicated logic in each layer). The zone-based attribution that works for CPU (`ZoneUsageMap`) does not extend to GPU; GPU rollup is a separate accumulation in `container.go`/`pod.go`. Every new device category would need the same duplication.

2. **Device-specific branching in monitor** — `node.go` has `isEnergySensor` branching between energy counters (RAPL) and power sensors (HWMON). Adding DRAM or platform meters requires more branches.

3. **No per-process DRAM/uncore/platform attribution** — These RAPL zones exist but are only tracked at node level. There's no mechanism to attribute their power to processes using device-appropriate metrics (e.g., page faults for DRAM).

4. **Tight coupling between source and attribution** — The `EnergyZone` interface combines hardware reading (`Energy()`, `Power()`) with zone identity. Attribution logic is embedded in the monitor rather than being composable per device.

5. **Duplication across aggregation levels** — `calculateContainerPower()`, `calculateVMPower()`, `calculatePodPower()` are structurally identical: sum process power by grouping key. These could be a single pass.

### Current Limitations

1. `Process.GPUPower float64` is separate from `Process.Zones ZoneUsageMap` — GPU power can't flow through the same aggregation path as CPU zones
2. Adding a new device type (DRAM attribution, platform meter) requires changes in monitor, snapshot types, and each aggregation function
3. `EnergyZone` interface mixes power source concerns with zone identity
4. No extensibility for future power sources (INA current/voltage sensors, CWF direct metering)

## Goals

- **Primary**: Unify all device power monitoring behind a single `Meter` interface that produces per-process power and energy
- **Secondary**: Enable GPU power attribution to containers, VMs, and pods through the same zone aggregation path as CPU
- **Tertiary**: Make adding new device types (DRAM with page-fault attribution, platform, uncore) a matter of composing existing building blocks

## Non-Goals

- Changing the Prometheus metric names or labels (wire compatibility preserved)
- Implementing CWF (Cumulative Workload Frequency) direct metering — future work that can implement `Meter` directly
- Replacing the `resource.Informer` — it remains the single source of process/container/VM/pod data
- AMD or Intel GPU backends — those are EP-003 future work and will implement `Meter` when ready
- Real-time sub-second monitoring

## Requirements

### Functional Requirements

- All devices (CPU, GPU, DRAM, uncore, platform) produce per-process power and energy through the `Meter` interface
- GPU power flows to container/VM/pod level through the same aggregation path as CPU
- RAPL energy counter wraparound handling preserved (currently in `AggregatedZone`)
- Idle vs active power separation preserved for all devices
- Terminated workload tracking works uniformly across all device types
- Existing Prometheus metrics continue to work unchanged

### Non-Functional Requirements

- **Performance**: No additional per-cycle overhead beyond what current code does. ResourceInformers are read-only views over data already collected by `resource.Informer.Refresh()`
- **Reliability**: Energy counter wraparound, first-read handling, and zero-delta-time handling must be correct
- **Thread Safety**: All code must pass `-race` tests. `Meter.Read()` is called from the monitor's single-writer goroutine
- **Testability**: Each component (Source, Adapter, ResourceInformer, Meter) is independently unit-testable with simple interfaces
- **Maintainability**: Adding a new device type requires implementing Source + ResourceInformer + composing a ProportionalMeter. No monitor changes needed

## Proposed Solution

### High-Level Architecture

```text
┌───────────────────────────────────────────────────────────────────────┐
│                           PowerMonitor                                │
│                                                                       │
│  meters []Meter           ┌──────────────────────────────────────┐    │
│  ┌────────────────┐       │         aggregateWorkloads()         │    │
│  │ cpu:rapl:pkg   │──┐    │  Process → Container → VM → Pod      │    │
│  │ gpu:nvidia:0   │──┤    │  (single pass over all meters)       │    │
│  │ gpu:nvidia:1   │──┼───>│                                      │    │
│  │ dram:rapl:dram │──┤    └──────────────────────────────────────┘    │
│  │ platform:psys  │──┘                                                │
│  └────────────────┘                                                   │
└──────────────────────── ──────────────────────────────────────────────┘

Each Meter is composed of:

┌──────────────────────────────────────────┐
│            ProportionalMeter             │
│                                          │
│  ┌────────────┐    ┌──────────────────┐  │
│  │   Source   │    │ResourceInformer  │  │
│  │ (power or  │    │ (per-process     │  │
│  │  energy)   │    │  utilization)    │  │
│  └─────┬──────┘    └────────┬─────────┘  │
│        │                    │            │
│        ▼                    ▼            │
│   total power    ×    pid ratios         │
│   ──────────────────────────────────     │
│   = per-process power + energy           │
└──────────────────────────────────────────┘
```

### Device Composition Table

| Device       | Power Source             | Adapter                   | Attribution (ResourceInformer)                        |
|--------------|--------------------------|---------------------------|-------------------------------------------------------|
| CPU          | RAPL package `energy_uj` | EnergyToPowerAdapter      | CPUTimeInformer (`/proc/[pid]/stat` CPU time ratio)   |
| CPU (alt)    | HWMON `power_input`      | PowerToEnergyDeltaAdapter | CPUTimeInformer                                       |
| GPU          | NVML `GetPowerUsage`     | PowerToEnergyDeltaAdapter | GPUUtilInformer (SM utilization ratio)                |
| DRAM         | RAPL dram `energy_uj`    | EnergyToPowerAdapter      | MemoryInformer (page fault ratio, fallback: CPU time) |
| Uncore       | RAPL uncore `energy_uj`  | EnergyToPowerAdapter      | CPUTimeInformer (initially)                           |
| Platform     | RAPL psys `energy_uj`    | EnergyToPowerAdapter      | UsageRatioInformer (overall CPU usage)                |
| CWF (future) | Direct per-process power | N/A                       | N/A — implements Meter directly                       |

### Key Design Choices

1. **Composition over inheritance** — `ProportionalMeter` composes a Source + ResourceInformer rather than requiring each device to implement the full Meter interface. Only backends that provide per-process power directly (CWF) implement Meter from scratch.

2. **Asymmetric adapters** — `EnergyToPowerAdapter` returns cumulative energy (from hardware counter) while `PowerToEnergyDeltaAdapter` returns only delta energy. `ProportionalMeter` handles both cases, accumulating software-tracked cumulative energy when hardware doesn't provide it.

3. **ResourceInformer is read-only** — Informers don't call `Refresh()`. The monitor calls `resource.Informer.Refresh()` once per cycle, then all ResourceInformers read from the already-updated data. This preserves the single-writer pattern.

4. **String-keyed zones** — Zone keys change from `EnergyZone` interface to plain strings (e.g., `"cpu:rapl:package"`, `"gpu:nvidia:0"`). This simplifies map operations and removes the need for interface equality semantics.

## Detailed Design

### Package Structure

```text
internal/device/meter/
├── source.go               # EnergySource, PowerSource interfaces
├── informer.go             # ResourceInformer interface
├── meter.go                # Meter interface, MeterReading, ProportionalMeter
├── adapter.go              # EnergyToPowerAdapter, PowerToEnergyDeltaAdapter
├── adapter_test.go
├── cputime_informer.go     # CPUTimeInformer
├── cputime_informer_test.go
├── memory_informer.go      # MemoryInformer (DRAM attribution)
├── memory_informer_test.go
├── usage_ratio_informer.go # UsageRatioInformer (platform fallback)
├── rapl/
│   ├── source.go           # sysfsRaplSource implements EnergySource
│   └── discovery.go        # zone discovery, maps zones to devices
├── hwmon/
│   ├── source.go           # sysfsHwmonSource implements PowerSource
│   └── discovery.go        # hwmon sensor discovery
└── gpu/
    ├── source.go           # NVMLPowerSource implements PowerSource
    ├── informer.go         # GPUUtilInformer implements ResourceInformer
    └── meter.go            # GPU-specific meter construction helpers
```

### API/Interface Changes

#### Source Interfaces (`source.go`)

```go
// EnergySource reads cumulative energy counters (RAPL sysfs, some HWMON sensors).
type EnergySource interface {
    ReadEnergy() (device.Energy, error)
    MaxEnergy() device.Energy
}

// PowerSource reads instantaneous power (HWMON power_input, NVML device).
type PowerSource interface {
    ReadPower() (device.Power, error)
}

// CurrentSource reads current in amps (future: INA sensors).
type CurrentSource interface {
    ReadCurrent() (float64, error)
}

// VoltageSource reads voltage in volts (future: INA sensors).
type VoltageSource interface {
    ReadVoltage() (float64, error)
}
```

#### ResourceInformer Interface (`informer.go`)

```go
// ResourceInformer provides per-process utilization ratios for power attribution.
// Each device type uses a different informer because workloads stress devices differently.
// Implementations are read-only: they read from resource.Informer data that was already
// refreshed by the monitor's collection loop.
type ResourceInformer interface {
    // Utilization returns per-PID utilization ratios (0.0-1.0) that should sum to ~1.0.
    // Returns nil map with nil error if no utilization data is available.
    Utilization() (map[uint32]float64, error)
}
```

#### Meter Interface (`meter.go`)

```go
// Meter is the unified interface for all device power monitoring.
// The monitor treats all devices uniformly through this interface.
type Meter interface {
    // Name returns a unique identifier for this meter (e.g., "cpu:rapl:package", "gpu:nvidia:0")
    Name() string
    // Category returns the device category (e.g., "cpu", "gpu", "dram", "platform", "uncore")
    Category() string
    // Read produces a new MeterReading given the previous reading and time delta.
    // usageRatio is the node-level CPU usage ratio for idle/active power splitting.
    Read(prev *MeterReading, timeDelta float64, usageRatio float64) (*MeterReading, error)
    // IsPrimary returns true if this meter should be used for terminated workload tracking.
    IsPrimary() bool
}

// MeterReading contains both node-level totals and per-process attribution.
type MeterReading struct {
    // Node-level totals
    TotalPower        device.Power
    ActivePower       device.Power
    IdlePower         device.Power
    EnergyTotal       device.Energy // cumulative
    ActiveEnergyTotal device.Energy // cumulative
    IdleEnergyTotal   device.Energy // cumulative

    // Per-process attribution
    ProcessPower  map[uint32]device.Power  // PID -> power this interval
    ProcessEnergy map[uint32]device.Energy // PID -> cumulative energy
}
```

#### Adapters (`adapter.go`)

```go
// EnergyToPowerAdapter wraps an EnergySource and computes power from energy deltas.
// Used by RAPL zones where hardware provides cumulative energy counters.
type EnergyToPowerAdapter struct {
    source  EnergySource
    prev    device.Energy
    hasLast bool
}

// Read returns (power, deltaEnergy, cumulativeEnergy).
// Handles counter wraparound using MaxEnergy().
func (a *EnergyToPowerAdapter) Read(timeDelta float64) (device.Power, device.Energy, device.Energy, error)

// PowerToEnergyDeltaAdapter wraps a PowerSource and computes delta energy from power × time.
// Used by HWMON power sensors and NVML. Does NOT produce cumulative energy —
// ProportionalMeter accumulates that in software.
type PowerToEnergyDeltaAdapter struct {
    source PowerSource
}

// Read returns (power, deltaEnergy).
func (a *PowerToEnergyDeltaAdapter) Read(timeDelta float64) (device.Power, device.Energy, error)

// IVToPowerAdapter composes CurrentSource + VoltageSource, computing P = I × V.
// Future use for INA current/voltage sensors.
type IVToPowerAdapter struct {
    current CurrentSource
    voltage VoltageSource
}

func (a *IVToPowerAdapter) ReadPower() (device.Power, error)
```

#### ProportionalMeter (`meter.go`)

```go
// ProportionalMeter implements Meter by combining a power/energy source with a
// ResourceInformer for per-process attribution. This is the standard composition
// for most devices. Only backends providing direct per-process power (CWF) skip this.
type ProportionalMeter struct {
    name     string
    category string
    primary  bool

    // Exactly one of these is set, depending on the source type:
    energyAdapter *EnergyToPowerAdapter       // for RAPL-backed devices
    powerAdapter  *PowerToEnergyDeltaAdapter  // for HWMON/NVML-backed devices

    informer ResourceInformer
}
```

#### Snapshot Type Changes

```go
// Zone keys become strings instead of EnergyZone interface
type ZoneUsageMap     map[string]Usage      // was map[EnergyZone]Usage
type NodeZoneUsageMap map[string]NodeUsage  // was map[EnergyZone]NodeUsage

// Process.GPUPower removed — GPU power is in Zones["gpu:nvidia:0"]
type Process struct {
    PID, Comm, Exe string
    Type           resource.ProcessType
    CPUTotalTime   float64
    Zones          ZoneUsageMap
    // GPUPower float64  -- REMOVED: now in Zones
    ContainerID      string
    VirtualMachineID string
}

// GPUDeviceStats replaced by generic MeterStats
type MeterStats struct {
    Name        string  // e.g., "gpu:nvidia:0"
    Category    string  // e.g., "gpu"
    TotalPower  float64
    IdlePower   float64
    ActivePower float64
    // GPU-specific fields (populated when Category == "gpu")
    DeviceIndex int
    UUID        string
    DeviceName  string
    Vendor      string
}
```

#### Monitor Changes

```go
// OLD
type PowerMonitor struct {
    cpu       device.CPUPowerMeter
    gpuMeters []gpu.GPUPowerMeter
    // ...
}

// NEW
type PowerMonitor struct {
    meters       []meter.Meter
    primaryMeter meter.Meter  // for terminated workload tracking
    // ...
}
```

- `NewPowerMonitor` takes `[]meter.Meter` instead of `device.CPUPowerMeter` + optional GPU meters
- `node.go`: uniform `meter.Read()` loop — no `isEnergySensor` branching
- `process.go`: single loop over all meters' `ProcessPower` + `ProcessEnergy`
- `container.go`, `vm.go`, `pod.go`: replaced by single `aggregateWorkloads()` function

#### Aggregation Simplification

```go
// Single aggregation pass replaces calculateContainerPower(), calculateVMPower(), calculatePodPower()
func aggregateWorkloads(processes Processes, processMap map[int]*resource.Process,
    containers Containers, vms VirtualMachines, pods Pods) {

    for _, proc := range processes {
        for zoneName, usage := range proc.Zones {
            // Aggregate to container
            if proc.ContainerID != "" {
                container := containers[proc.ContainerID]
                zu := container.Zones[zoneName]
                zu.Power += usage.Power
                zu.EnergyTotal += usage.EnergyTotal
                container.Zones[zoneName] = zu
            }
            // Same for VM, Pod
        }
    }
}
```

## Configuration

No new configuration required. Device discovery happens at startup:

1. Probe RAPL sysfs for available zones → create CPU, DRAM, uncore, platform meters
2. Probe HWMON for power sensors → create CPU meter (if no RAPL)
3. Run GPU registry `DiscoverAll()` → create GPU meters
4. Compose all into `[]meter.Meter` and pass to `NewPowerMonitor()`

Future configuration options (not part of this EP):

- Per-device idle power overrides
- Selective device enabling/disabling

## Testing Strategy

### Unit Tests

- **Adapters**: Energy delta calculation, wraparound handling, first-read returns zero, zero time delta
- **ResourceInformers**: CPU time ratio calculation, GPU SM ratio normalization, memory informer fallback
- **ProportionalMeter**: End-to-end with mock Source + mock Informer, verify per-process power sums to total active power
- **Aggregation**: Process → Container → Pod rollup with multiple zones

### Integration Tests

- **Side-by-side comparison**: Run old and new code paths on same fake data, verify identical Snapshot output
- **Docker Compose**: Verify Prometheus metrics unchanged after migration

### Race Detection

All tests run with `-race` flag as enforced by `make test`.

## Migration and Compatibility

### Backward Compatibility

- Prometheus metric names and labels unchanged
- Configuration unchanged
- Deployment unchanged (same binary, same privileges)

### Migration Path

Each phase is a separate PR. Tests pass at every phase. No behavior changes until Phase 6.

#### Phase 1: Interfaces + Adapters

Define Source, ResourceInformer, Meter, adapters in `internal/device/meter/`. Unit tests only. No integration with monitor.

#### Phase 2: CPU Meter (RAPL + HWMON)

Implement `ProportionalMeter` for CPU device. `CPUTimeInformer`. Side-by-side tests comparing output with current `CPUPowerMeter`.

#### Phase 3: GPU Meter

Implement `ProportionalMeter` for GPU. `GPUUtilInformer` wrapping existing NVML collector. Verify identical per-process power output.

#### Phase 4: DRAM Meter

Extend `resource.Process` with page fault deltas from `/proc/[pid]/stat`. Implement `MemoryInformer`. Create DRAM `ProportionalMeter` from RAPL dram zone. Initially fall back to `CPUTimeInformer`.

#### Phase 5: Platform + Uncore Meters

`ProportionalMeter` for RAPL psys and uncore zones. `UsageRatioInformer` for platform.

#### Phase 6: Monitor Migration

Replace `cpu CPUPowerMeter` + `gpuMeters []GPUPowerMeter` with `meters []Meter`. Uniform loop. GPU power flows to containers/VMs/pods through Zones. Single `aggregateWorkloads()` replaces three separate functions.

#### Phase 7: Exporter + Wiring

Zone keys become strings. `main.go` discovers hardware and builds `[]Meter`. Exporter adapts to new zone key type.

#### Phase 8: Cleanup

Remove `CPUPowerMeter`, `EnergyZone`, `GPUPowerMeter` interfaces and their implementations.

### Rollback Strategy

Each phase is independently revertible. Since no behavior changes until Phase 6, phases 1-5 can coexist with current code. Phase 6+ can be reverted by reverting the PR.

## Risks and Mitigations

### Technical Risks

- **Risk**: Per-process energy accumulation drift when using `PowerToEnergyDeltaAdapter` (no hardware counter)
  - **Mitigation**: This is the same approach currently used for GPU. Document that software-accumulated energy counters may drift slightly vs hardware counters. Use hardware counters (RAPL) wherever available.

- **Risk**: ResourceInformer utilization ratios don't sum to exactly 1.0 due to timing
  - **Mitigation**: `ProportionalMeter` normalizes ratios. Any remainder attributed to idle. This matches current behavior.

- **Risk**: Breaking changes to Snapshot types affect exporters
  - **Mitigation**: Phase 7 (exporter changes) is a single focused PR. Zone key change from `EnergyZone` to `string` is mechanical.

### Operational Risks

- **Risk**: Regression in power accuracy during migration
  - **Mitigation**: Side-by-side tests comparing old and new output on identical input data. Docker Compose end-to-end verification.

## Alternatives Considered

### Alternative 1: Extend Current Architecture

- **Description**: Add GPU to `ZoneUsageMap` by making GPU implement `EnergyZone`, add DRAM/uncore/platform as more zones on `CPUPowerMeter`
- **Reason for Rejection**: `EnergyZone` conflates source reading with zone identity. GPU attribution (SM util) is fundamentally different from CPU attribution (CPU time). Extending `CPUPowerMeter` to cover non-CPU devices is a misnomer. The monitor branching would only grow worse.

### Alternative 2: Separate Meter Per Category

- **Description**: Separate interfaces: `CPUMeter`, `GPUMeter`, `DRAMMeter`, etc., each with category-specific methods
- **Reason for Rejection**: Requires the monitor to know about each category. Adding a new device type requires monitor changes. The whole point is that the monitor should treat all devices uniformly.

### Alternative 3: Plugin Architecture with Dynamic Loading

- **Description**: Load device backends as Go plugins at runtime
- **Reason for Rejection**: Go plugins have significant limitations (same Go version, same build flags). The composition approach achieves extensibility without runtime plugin complexity.

## Success Metrics

- **Functional**: GPU power appears on container/VM/pod metrics (currently missing)
- **Code Quality**: Monitor's `computeSnapshot()` reduces from ~5 device-specific branches to 1 uniform loop
- **Extensibility**: Adding DRAM meter (Phase 4) requires zero changes to monitor code
- **Test Coverage**: Each component independently testable; overall coverage maintained or improved

## Open Questions

1. Should `MeterReading.ProcessEnergy` track cumulative energy per-process, or should cumulative tracking be deferred to the exporter? (Current proposal: track in MeterReading for consistency with node-level `EnergyTotal`)
2. For DRAM attribution, should we use page faults, RSS, or memory bandwidth? Page faults are available from `/proc/[pid]/stat` without additional kernel support. Memory bandwidth requires perf counters.
3. Should uncore attribution use CPU time ratio (simple, available now) or cache miss ratio (more accurate, requires perf counters)?
4. How should `ProportionalMeter` handle the case where `ResourceInformer.Utilization()` returns an empty map but the Source reports non-zero power? (Proposed: attribute all to idle)
