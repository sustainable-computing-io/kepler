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
- **Export-Triggered Cleanup**: Clear terminated workloads only after they've been exported to prevent data loss
- **Configurable Capacity**: Control memory usage while maintaining fairness

**Configuration:**

```yaml
monitor:
  maxTerminated: 100  # Track top 100 terminated workloads by energy
  minTerminatedEnergyThreshold: 10  # Only track workloads with >10J consumption
```

## 2. Data Consistency, Completeness & Mathematical Integrity

> *"Data exported should be consistent, complete and not half complete"*

### Mathematical Relationships Enforced

Kepler enforces strict mathematical consistency across all power attribution levels:

```text
Node Watts = Node Active Watts + Node Idle Watts
Node Active Watts = Σ(Process Watts)
Container Watts = Σ(Process Watts in Container)
VM Watts = Σ(Process Watts associated with VM)
Pod Watts = Σ(Container Watts in Pod)
```

### Implementation Guarantees

- **Atomic Snapshots**: All related data captured at the same instant
- **Energy Conservation Validation**: Attribution totals must equal measured energy
- **Hierarchical Aggregation**: Bottom-up rollup ensures mathematical consistency
- **Immutable Data Structures**: Prevent partial updates during export

### Testing Strategy

```go
// Energy conservation tests validate:
assert.Equal(t, nodeActiveEnergy, sumOfProcessEnergies)
assert.Equal(t, containerEnergy, sumOfContainerProcessEnergies)
assert.Equal(t, podEnergy, sumOfPodContainerEnergies)
```

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

### Freshness Controls

- **Configurable Staleness Threshold**: Default 10s, prevents serving outdated data
- **Automatic Refresh**: Fresh data computed on-demand when stale
- **Singleflight Protection**: Prevents redundant calculations during concurrent requests
- **Data Channel Notifications**: Signal exporters when new data is available

### Freshness Implementation

```go
func (pm *PowerMonitor) isFresh() bool {
    age := pm.clock.Now().Sub(snapshot.Timestamp)
    return age <= pm.maxStaleness
}

func (pm *PowerMonitor) Snapshot() (*Snapshot, error) {
    if !pm.isFresh() {
        pm.synchronizedPowerRefresh() // Automatic refresh
    }
    return pm.snapshot.Load(), nil
}
```

### Configuration

```yaml
monitor:
  staleness: 10s  # Data older than 10s triggers automatic refresh
```

## 5. Deterministic Processing

> *"Parallel vs Serial processing should not produce different results"*

### Thread Safety Guarantees

- **Atomic Snapshot Updates**: Single writer ensures consistency
- **Immutable Snapshots**: No partial modifications possible
- **Deterministic Attribution**: Same CPU time ratios → same power attribution
- **Race-Free Calculations**: All concurrent access is read-only

### Implementation Patterns

```go
// Single writer pattern
func (pm *PowerMonitor) refreshSnapshot() error {
    // Only one goroutine can update snapshots
    newSnapshot := NewSnapshot()
    // ... perform calculations ...
    pm.snapshot.Store(newSnapshot) // Atomic update
}

// Multiple readers pattern
func (pm *PowerMonitor) Snapshot() (*Snapshot, error) {
    return pm.snapshot.Load(), nil // Lock-free read
}
```

### Testing Requirements

All concurrent code must be validated with race detection:

```bash
go test -race ./...
```

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

Users can configure exactly what metrics to collect and expose:

```yaml
exporter:
  prometheus:
    metricsLevel: "container"  # node|process|container|vm|pod|all
```

### Current Support Matrix

| Level         | Status | Metrics                                   | Benefits                       |
|---------------|--------|-------------------------------------------|--------------------------------|
| **Node**      | ✅      | `kepler_node_cpu_watts{zone="package"}`   | System-level monitoring        |
| **Process**   | 🔄     | `kepler_process_cpu_watts{pid="1234"}`    | Fine-grained analysis          |
| **Container** | 🔄     | `kepler_container_cpu_watts{id="abc123"}` | Container-level billing        |
| **VM**        | 🔄     | `kepler_vm_cpu_watts{id="vm-123"}`        | Virtualization monitoring      |
| **Pod**       | ✅      | `kepler_pod_cpu_watts{pod="webapp"}`      | Kubernetes workload monitoring |

### Configuration Benefits

- **Reduced Cardinality**: Lower storage and query costs for large clusters
- **Lower Resource Usage**: Collect only necessary data
- **Focused Monitoring**: Target specific workload types
- **Scalability**: Scale metrics collection based on cluster size

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
- **Consistent Naming**: `--exporter.prometheus.enabled` ↔ `exporter.prometheus.enabled`

### Examples

```bash
# CLI override of YAML config
kepler --config=production.yaml --log.level=debug --monitor.interval=5s

# Development mode (not exposed as CLI flag - internal only)
# Must be set in YAML: dev.fake_cpu_meter.enabled: true
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
| Mathematical Integrity     | Atomic snapshots + energy conservation            | Immutable `Snapshot` with validation             |
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
