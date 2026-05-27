# Draft: Extensible Power Meter Selection

* **Status**: Draft
* **Author**: Niki Manoledaki
* **Created**: 2026-05-03

## Problem

For context, `cmd/kepler/main.go` constructs CPU and GPU power meters in two different shapes:

* `createCPUMeter` hardcodes RAPL with hwmon as a fallback. It returns `(device.CPUPowerMeter, error)`. This is strict by design: Kepler must produce a CPU meter or fail startup.
* `createGPUMeters` fans out across registered vendors via `gpu.DiscoverAll`. It returns `[]gpu.GPUPowerMeter` with no error, which is a soft failure mode, also by design, as it assumes that GPU monitoring is not required for Kepler.

Once we agree on these assumptions, here are the problems that follow.

1. **Asymmetry.** Two patterns for the same job (turn config into hardware abstractions) makes `cmd/kepler/main.go` harder to read and audit.
2. **Hidden GPU failures.** `gpu.Discover` (`internal/device/gpu/registry.go:48`) collapses three failure modes into one `nil` return:
   * factory error (driver missing or unsupported)
   * `Init()` error (driver present but broken)
   * empty `Devices()` (vendor registered, no hardware on this node)

   Only the third is a legitimate soft outcome. Real backend failures surface only as a `Warn` log line.
3. **Closed CPU path.** RAPL and hwmon are hard-coded fallbacks with hwmon force-enable as the only fallback. Operators on infrastructure where neither is the right source cannot extend this model to select a different backend easily.

## Goals

* Extend CPU power meter backend selection to a config-driven list.
* Unify more of the CPU and GPU lifecycle shape: discovery, failure, initialization.
* Surface real GPU backend failures as errors instead of logging them.
* Preserve default behaviour for healthy systems running default config.

## Non-Goals

* This proposal does not build a common interface **per device** across CPU zones and GPU devices. Their measurement shapes differ and a unifying interface would abstract hardware-specific concerns through `interface{}`, which is not ideal. The CPU and GPU attributes should remain separate.
* This proposal does not replace `monitor.PowerDataProvider`, which will continue to be the consumer for exporters. This proposal unifies the step right before that, which is how to initialize the hardware backend.

## Proposed Solution

### Promote `device.PowerMeter`

Promote the unexported `powerMeter` interface in [internal/device/power_meter.go](../../../internal/device/power_meter.go) to an external interface `PowerMeter`, like this:

```go
// PowerMeter reads power from a class of hardware (CPU, GPU, etc).
//
// Many PowerMeters can be selected. Each contributes its own readings.
// Domain-specific methods live on subinterfaces that embed PowerMeter.
type PowerMeter interface {
    service.Service     // Name()
    service.Initializer // Init()
}
```

`Shutdown` is optional via `service.Shutdowner`. Some backends like NVML implement it.

`device.CPUPowerMeter` and `gpu.GPUPowerMeter` embed `device.PowerMeter` and add their own methods. No change to those domain methods in this EP.

### Switch-based CPU dispatch

CPU gains a config field for ordering plus a switch that walks the priority list:

```yaml
cpu:
  meters: ["rapl", "hwmon"]   # ordered priority
```

```go
// internal/device/cpu_power_meter.go

func CreateCPUMeter(logger *slog.Logger, cfg *config.Config) (CPUPowerMeter, error) {
    var errs []error
    for _, name := range cfg.Cpu.Meters {
        meter, err := buildCPUMeter(name, logger, cfg)
        if err != nil { /* aggregate, continue */ }
        if err := meter.Init(); err != nil { /* aggregate, continue */ }
        zones, _ := meter.Zones()
        if len(zones) == 0 { /* soft skip, continue */ }
        return meter, nil
    }
    return nil, errors.Join(errs...)
}

func buildCPUMeter(name string, logger *slog.Logger, cfg *config.Config) (CPUPowerMeter, error) {
    switch name {
    case "rapl":  return NewCPUPowerMeter(...)
    case "hwmon": return NewHwmonPowerMeter(...)
    case "fake":  return NewFakeCPUMeter(...)
    default:      return nil, fmt.Errorf("unknown cpu meter %q", name)
    }
}
```

`CreateCPUMeter` owns the lifecycle (build, init, zones, error aggregation). `buildCPUMeter` is the dispatch. Adding a backend means a new case + a new constructor, which is straightforward.

Default value `["rapl", "hwmon"]` preserves the prior fallback behaviour. Operators make a selection via the config.

### GPU failure-mode split

As part of this unification, `gpu.Discover` is refactored to split the three failure modes and return `(meters, error)`:

| State                        | GPU result                                      |
|------------------------------|-------------------------------------------------|
| Hardware works               | `meters, nil`                                   |
| Hardware absent on this node | `nil, nil`                                      |
| Configured backend broken    | `meters?, err` (per-vendor failures aggregated) |
| Feature off                  | `nil, nil`                                      |

Result per scenario at startup:

| Scenario          | Result                   |
|-------------------|--------------------------|
| Factory error     | Real failure. Aggregate. |
| `Init()` error    | Real failure. Aggregate. |
| Empty `Devices()` | Soft skip. Not an error. |
| Success           | Append to result.        |

Error aggregation uses `errors.Join`. `gpu.Discover` returns the joined error only when the GPU feature is explicitly enabled and every registered vendor returned a real failure. "No GPU on this node" stays `(nil, nil)`.

### Unified `cmd/kepler/main.go`

End result after the refactor:

```go
cpuMeter,  err := device.CreateCPUMeter(logger, cfg)
if err != nil { ... }

gpuMeters, err := gpu.Discover(logger, cfg)
if err != nil { ... }
```

Symmetric shape (`(meter(s), error)`), different failure semantics (CPU strict, GPU soft).

## Detailed Design

### Package layout

```text
internal/device/
├── power_meter.go              # PowerMeter interface (promoted from private)
├── cpu_power_meter.go          # CPUPowerMeter, CreateCPUMeter, buildCPUMeter
├── rapl_sysfs_power_meter.go   # NewCPUPowerMeter (RAPL)
├── hwmon_power_meter.go        # NewHwmonPowerMeter
├── fake_cpu_power_meter.go     # NewFakeCPUMeter
└── gpu/
    ├── interface.go            # GPUPowerMeter embeds device.PowerMeter, plus Vendor and GPUDevice types
    ├── registry.go             # gpu.Register, gpu.Discover (refactored failure modes)
    └── nvidia/                 # init() calls gpu.Register(NVIDIA, ...)
```

No new file for CPU dispatch — `CreateCPUMeter` and `buildCPUMeter` live in `cpu_power_meter.go` next to the `CPUPowerMeter` interface they construct.

### Logging

Per-backend `Warn` on failure, `Info` on selection:

```text
INFO using rapl power meter
```

```text
WARN rapl not available, trying next backend  error="..."
INFO using hwmon power meter
```

GPU keeps its existing per-vendor `Warn` plus a one-line summary:

```text
INFO gpu meter discovery   ok=[nvidia] failed=[amd: factory: rocm-smi not found]
```

## Configuration

```yaml
cpu:
  meters: ["rapl", "hwmon"]   # ordered priority
```

Per-backend tuning keys (`rapl.zones`, `experimental.hwmon.zones`, `experimental.hwmon.chipRules`, `dev.fake-cpu-meter.zones`) are unchanged. Legacy selectors translate at startup to `cpu.meters` and emit a deprecation warning. See [Backward compatibility](#backward-compatibility) for the full migration.

GPU config does not change in this EP.

## Testing Strategy

* `device.CreateCPUMeter` table-driven tests: unknown name, factory error, `Init()` error, empty zones, success, ordered priority, all-fail aggregation.
* `gpu.Discover` table-driven tests: all-fail, mixed, all-empty, all-succeed; per-vendor failure modes (factory, `Init()`, empty `Devices()`).

## Backward compatibility

Default `cpu.meters: ["rapl", "hwmon"]` reproduces default behaviour. No breaking change for healthy systems running default config.

Two legacy selectors require migration. They translate at startup to an effective `cpu.meters` value, log a deprecation warning, and stop working in a future release:

* `experimental.hwmon.forceEnabled: true` (today: force hwmon, skip RAPL) → effective `cpu.meters: ["hwmon"]`.
* `dev.fake-cpu-meter.enabled: true` (today: use fake meter, skip RAPL/hwmon) → effective `cpu.meters: ["fake"]`.

Per-backend tuning keys are unchanged and remain valid: `rapl.zones`, `experimental.hwmon.zones`, `experimental.hwmon.chipRules`, `dev.fake-cpu-meter.zones`.

Operators on broken systems will see `kepler` exit with a clearer error when every CPU backend fails (the prior path also exits, but with the hwmon error only, which can be misleading).

GPU error semantics tighten. A node with `gpu.enabled=true` and a broken NVML driver will fail startup instead of silently running CPU-only. This matches the CPU contract. CPU-only is the default (`gpu.enabled=false`); the tightening only affects operators who explicitly opt in to GPU.

## Implementation

Single PR introduces:

1. Promote `device.PowerMeter` interface (`Name`, `Init`).
2. Add `cfg.Cpu.Meters` config field with default `["rapl", "hwmon"]`.
3. Add `device.CreateCPUMeter` + `buildCPUMeter` switch dispatch.
4. Add `Config.ApplyCpuMeterDeprecations` to translate the two legacy keys.
5. Replace `cmd/kepler/main.go`'s `createCPUMeter` / `createHwmonMeter` with a call to `device.CreateCPUMeter`.
6. Update sample configs and docs.

GPU failure-mode split lands as a follow-up PR — separable since CPU and GPU paths are independent in `cmd/kepler/main.go`.

## Risks and Mitigations

### Operational risk: stricter GPU error path (follow-up PR)

* **Risk**: Nodes with GPU explicitly enabled (`gpu.enabled=true`) and broken drivers fail startup where the prior code silently continued without GPU metrics.
* **Mitigation**: Matches the CPU contract. Default-off (`gpu.enabled=false`) means only opted-in operators are affected; they can revert to CPU-only by removing the explicit opt-in.

### Configuration risk: invalid `cpu.meters` value

* **Risk**: Operator typo (`"rappl"`) is rejected by `buildCPUMeter` with `unknown cpu meter "rappl"`.
* **Mitigation**: The error names the unknown backend and the operator's `cpu.meters` is preserved in the printed config at startup so the typo is visible in logs.

## Alternatives Considered

### Alternative 1: Registry pattern with `init()`-time registration

A `RegisterCPUMeter(name, factory)` API where each backend's source file calls it from `init()`, plus a `DiscoverCPU` that walks the registry. This is what GPU does today.

**Why rejected**: speculative architecture for three backends. The registry pattern's value doesn't apply.

The switch dispatch achieves the same operator outcome (config-driven priority) with less complexity. If other CPU backends are required, migrating each `case` to a `RegisterCPUMeter` call is possible and the `CPUPowerMeter` interface doesn't change.

### Alternative 2: Unify CPU and GPU under one per-device interface

This would define a common `Device` interface that both CPU zones and GPU devices implement.

This is not viable since CPU zones (logical, energy in µJ per zone) and GPU devices (physical, watts plus per-process attribution) have different shapes. A unifying interface would push hardware-specific concerns through `interface{}` or lose information.

### Alternative 3: Per-backend Prometheus error counter

This would expose `kepler_meter_failures_total{backend, stage}` instead of returning errors.

While this solves visibility for transient failures during runtime, it doesn't address the startup-time question this proposal targets — "which backend was selected and why." Operational metrics could be added orthogonally.

## Next Steps

Out of scope for this EP.

### GPU failure-mode split (separate PR)

Refactor `gpu.Discover` to split factory / `Init()` / empty-devices failure modes and return `(meters, error)`. Separate PR; independent of the CPU work.

### Operational metrics per backend

A small set of operational metrics per backend: discovery success/failure counters, duration. Useful for operators running heterogeneous fleets.

### Migrate to a registry pattern when warranted

If out-of-tree CPU backends or a plugin model becomes a real requirement (BMC, MSR, ARM-vendor counters arriving from external contributors), the switch in `buildCPUMeter` migrates to `RegisterCPUMeter` calls. The interface (`CPUPowerMeter`) and config (`cpu.meters`) don't change, so the migration is mechanical.
