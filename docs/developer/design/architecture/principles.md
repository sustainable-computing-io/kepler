# Design Principles

The Kepler architecture is built on a set of core principles that drive all design decisions. Understanding these principles is essential for contributing to the project and maintaining architectural consistency.

## 1. Fair Power Allocation

> *"You can't rely on Prometheus scrapes for fairness. Processes that consumed power but aren't running anymore should be reported."*

### Problem Statement

Traditional monitoring systems only report currently running workloads, leading to unfair power attribution when processes terminate between scrapes. This creates several issues:

- **Attribution Gaps**: Power consumed by terminated processes is unaccounted for
- **Unfair Billing**: Running workloads get charged for terminated process energy
- **Incomplete Metrics**: Power consumption data has gaps and inconsistencies

### Solution: Terminated Workload Tracking

Kepler implements a comprehensive terminated workload tracking system:

```go
// Terminated workload trackers in PowerMonitor
terminatedProcessesTracker  *TerminatedResourceTracker[*Process]
terminatedContainersTracker *TerminatedResourceTracker[*Container]
terminatedVMsTracker        *TerminatedResourceTracker[*VirtualMachine]
terminatedPodsTracker       *TerminatedResourceTracker[*Pod]
```

**Key Features:**

- **Priority-Based Retention**: Keep highest energy consumers to ensure fair attribution
- **Cleanup after a snapshot read**: A successful snapshot request marks retained entries for cleanup during the next collection
- **Configurable Capacity**: Control memory usage while maintaining fairness

**Configuration:**

```yaml
monitor:
  maxTerminated: 100  # Track top 100 terminated workloads by energy
  minTerminatedEnergyThreshold: 10  # At least 10 J in the primary zone
```

## 2. Data Consistency, Completeness & Mathematical Integrity

> *"Data exported should be consistent, complete and not half complete"*

### CPU power attribution

For each CPU energy zone, the monitor splits measured node power according to node
CPU utilization. It then uses CPU time from the same interval to allocate active power.

```text
Zone power = active zone power + idle zone power
Workload CPU power for the zone = active zone power × (workload CPU time delta / node process CPU time delta)
```

Processes, containers, VMs, and pods each use their own CPU time delta. For example,
a pod receives its share directly from each zone's active power. Its containers
provide another view of that same power. [Power attribution guide](../../power-attribution-guide.md#independent-attribution)

The node tests exercise the active/idle split. The pod tests exercise CPU-time
attribution and the separate aggregation of GPU power from containers.
[Node tests](../../../../internal/monitor/node_test.go) · [Pod tests](../../../../internal/monitor/pod_power_test.go)

## 3. Computation-Presentation Separation (M/V Pattern)

> *"Keep Computation away from Presentation → Allow multiple ways to present the same data"*

### Architecture Pattern

```text
Monitor (Model) → PowerDataProvider Interface → Multiple Exporters (Views)
```

### Current Exporters

- ✅ **Prometheus**: Production metrics endpoint with full Prometheus ecosystem integration
- ✅ **Stdout**: Development and debugging with human-readable output
- 🔄 **OpenTelemetry**: Future extensibility for emerging observability standards

### Benefits

- **Extensibility**: Add new export formats without changing core logic
- **Flexibility**: Different presentation needs (dashboards, alerts, debugging)
- **Future-Proofing**: Ready for emerging standards and requirements
- **Testing**: Easy to validate computation logic independently from presentation

### M/V Pattern Implementation

```go
type PowerDataProvider interface {
    Snapshot() (*Snapshot, error)      // Core data access
    DataChannel() <-chan struct{}      // Change notifications
    ZoneNames() []string               // Metadata access
}
```

## 4. Data Freshness Guarantee

> *"Data presented should be as fresh as possible within the stale duration"*

`monitor.interval` controls periodic collection. `monitor.staleness` controls how old
a snapshot can be when a reader requests it. Scrapes can reuse a fresh sample.
A request for a stale sample triggers a refresh, and concurrent requests share
that work through `singleflight`.
[Configuration defaults](../../../../config/config.go) · [Monitor implementation](../../../../internal/monitor/monitor.go)

## 5. Deterministic Processing

> *"Parallel vs Serial processing should not produce different results"*

Only one refresh computes a new snapshot at a time. The monitor publishes it
atomically once the calculations are complete. A reader can therefore work with
one sample while the monitor prepares the next. When adding a snapshot field,
preserve reader isolation in `Snapshot.Clone` and run the monitor tests with the
race detector using `make test TEST_PKGS=./internal/monitor/...`.
[Clone tests](../../../../internal/monitor/clone_test.go) · [Concurrency tests](../../../../internal/monitor/monitor_concurrency_test.go)

## 6. Prefer Package Reuse Over Reinvention

> *"Prefer reuse (of packages) over reinventing them (like in old kepler)"*

### Well-Maintained Dependencies

| Area               | Package                          | Benefit                           |
|--------------------|----------------------------------|-----------------------------------|
| Service Management | `oklog/run`                      | Coordinated lifecycle management  |
| Metrics            | `prometheus/client_golang`       | Standard Prometheus exposition    |
| Concurrency        | `golang.org/x/sync/singleflight` | Proven deduplication patterns     |
| Kubernetes         | `k8s.io/client-go`               | Native Kubernetes API integration |
| Configuration      | `alecthomas/kingpin/v2`          | Battle-tested CLI parsing         |
| Logging            | `log/slog`                       | Standard structured logging       |

### Package Reuse Benefits

- **Reduced Maintenance Burden**: Focus on core logic, not infrastructure
- **Battle-Tested Implementations**: Proven reliability and performance
- **Community Support**: Active maintenance and security updates
- **Standards Compliance**: Follow established patterns and conventions

### Evaluation Criteria

When choosing external packages:

1. **Active Maintenance**: Regular updates and responsive maintainers
2. **Production Usage**: Proven track record in production systems
3. **API Stability**: Stable interfaces that won't break frequently
4. **Performance**: Suitable for Kepler's performance requirements
5. **License Compatibility**: Compatible with Apache 2.0 license

## 7. Configurable Collection & Exposure

> *"Allow Users to configure what to collect and expose only those"*

### Metrics Level Configuration

The Prometheus power collector uses `metricsLevel` to select which workload and node
series it emits from a monitor snapshot. The monitor still computes the snapshot.

```yaml
exporter:
  prometheus:
    metricsLevel: "container"  # node|process|container|vm|pod
```

### Current Support Matrix

| Level         | Status | Metrics                                             | Benefits                       |
|---------------|--------|-----------------------------------------------------|--------------------------------|
| **Node**      | ✅     | `kepler_node_cpu_watts{zone="package"}`             | System-level monitoring        |
| **Process**   | ✅     | `kepler_process_cpu_watts{pid="1234"}`              | Fine-grained analysis          |
| **Container** | ✅     | `kepler_container_cpu_watts{container_id="abc123"}` | Container-level billing        |
| **VM**        | ✅     | `kepler_vm_cpu_watts{vm_id="vm-123"}`               | Virtualization monitoring      |
| **Pod**       | ✅     | `kepler_pod_cpu_watts{pod_name="webapp"}`           | Kubernetes workload monitoring |

### Configuration Benefits

- **Reduced Cardinality**: Lower storage and query costs for large clusters
- **Smaller scrapes**: Emit fewer time series
- **Focused Monitoring**: Target specific workload types

### Zone Filtering

```yaml
rapl:
  zones: ["package", "dram"]  # Only collect package and DRAM zones
```

## 8. Implementation Abstraction

> *"Allow implementation details to change without affecting the rest of the code. E.g: eBPF is only a means to an end"*

### Interface-Based Design

Core abstractions enable implementation flexibility:

```go
// Hardware abstraction - can be RAPL, eBPF, or other implementations
type CPUPowerMeter interface {
    Zones() ([]EnergyZone, error)
    PrimaryEnergyZone() (EnergyZone, error)
}

// Resource tracking abstraction - can be procfs, eBPF, or other methods
type Informer interface {
    Refresh() error
    Processes() *Processes
    Containers() *Containers
}
```

### Current and Future Implementations

| Layer                 | Current            | Future             | Abstraction Benefit           |
|-----------------------|--------------------|--------------------|-------------------------------|
| **Hardware**          | RAPL sysfs         | eBPF perf counters | Pluggable energy sources      |
| **Resource Tracking** | procfs reader      | eBPF tracer        | Different collection methods  |
| **Export**            | Prometheus, stdout | OpenTelemetry      | Multiple presentation formats |

### Design Benefits

- **Technology Evolution**: Adopt new technologies without architectural changes
- **Platform Support**: Support different platforms through different implementations
- **Testing**: Easy to create fake implementations for testing
- **Performance Optimization**: Swap implementations based on performance needs

## 9. Simple Configuration & Low Learning Curve

> *"Simple Configuration to reduce learning curve - keep flags and configuration in sync (as much as possible)"*

### Hierarchical Configuration

Configuration follows a clear precedence order:

1. **CLI Flags** (highest) - for operational overrides
2. **YAML Files** (middle) - for persistent configuration
3. **Defaults** (lowest) - for sensible out-of-box experience

### Synchronization Strategy

- **Operational Flags**: Every YAML config option has corresponding CLI flag where operationally relevant
- **Development Options**: `dev.*` settings intentionally not exposed as CLI flags (internal only)
- The `--exporter.prometheus` flag controls the YAML setting `exporter.prometheus.enabled`.

### Examples

```bash
# CLI override of YAML config
kepler --config.file=production.yaml --log.level=debug --monitor.interval=5s
```

### Design Philosophy

- **Operational vs Development**: Clear separation between production and development settings
- **Discoverability**: All options documented with clear descriptions
- **Validation**: Configuration validation with helpful error messages
- **Defaults**: Sensible defaults that work out-of-the-box

## How Principles Shape Architecture

These principles directly influenced major architectural decisions:

| Principle                  | Architectural Impact                              | Implementation                                   |
|----------------------------|---------------------------------------------------|--------------------------------------------------|
| Fair Allocation            | Terminated workload tracking system               | `TerminatedResourceTracker` with priority queues |
| Mathematical Integrity     | Consistent per-zone CPU attribution               | Workload CPU time shares of node active power    |
| M/V Separation             | PowerDataProvider interface + pluggable exporters | Interface-based exporter system                  |
| Data Freshness             | Staleness control + singleflight pattern          | `isFresh()` checks + automatic refresh           |
| Deterministic Processing   | Thread-safe design + immutable snapshots          | Single writer, multiple readers                  |
| Package Reuse              | Minimal custom implementations + proven libraries | Strategic dependency selection                   |
| Configurable Exposure      | Metrics level system + zone filtering             | `config.Level` + filtering logic                 |
| Implementation Abstraction | Interface-based layers + dependency injection     | Clean interface boundaries                       |
| Simple Configuration       | Hierarchical config + CLI/YAML sync               | `kingpin` + YAML parsing                         |

---

## Next Steps

After understanding these principles, explore:

- **[System Components](components.md)**: How principles are implemented in practice
- **[Data Flow](data-flow.md)**: How principles ensure fair and accurate power attribution
- **[Concurrency](concurrency.md)**: How deterministic processing is achieved
