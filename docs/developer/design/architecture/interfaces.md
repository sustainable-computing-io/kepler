# Interfaces & Contracts

This document details the key interfaces and contracts that define Kepler's architecture boundaries, enabling modularity, testability, and extensibility.

## Interface Design Philosophy

Kepler follows interface-based design principles:

- **Dependency Inversion**: High-level modules depend on abstractions, not concretions
- **Interface Segregation**: Clients depend only on methods they use
- **Substitutability**: Implementations can be swapped without affecting clients
- **Testability**: Interfaces enable easy mocking and testing

## Core Service Interfaces

### Service Framework Contracts

The foundation of Kepler's service-oriented architecture:

```go
// Base service interface - all services must implement
type Service interface {
    Name() string  // Human-readable service identifier
}

// Services requiring initialization before use
type Initializer interface {
    Service
    Init() error  // Called sequentially during startup
}

// Services that run continuously with context cancellation
type Runner interface {
    Service
    Run(ctx context.Context) error  // Called concurrently, should block
}

// Services requiring cleanup during shutdown
type Shutdowner interface {
    Service
    Shutdown() error  // Called during graceful shutdown
}
```

**Usage Patterns:**

```go
// Service implementing all lifecycle interfaces
type PowerMonitor struct { /* ... */ }

func (pm *PowerMonitor) Name() string                     { return "power-monitor" }
func (pm *PowerMonitor) Init() error                      { return pm.initialize() }
func (pm *PowerMonitor) Run(ctx context.Context) error    { return pm.runCollectionLoop(ctx) }
func (pm *PowerMonitor) Shutdown() error                  { return pm.cleanup() }
```

**Contract Guarantees:**

- `Init()` called exactly once, sequentially, before `Run()`
- `Run()` called concurrently for all services
- `Shutdown()` called during cleanup, regardless of `Run()` outcome
- Context cancellation in `Run()` indicates shutdown request

## Power Data Interfaces

### PowerDataProvider Contract

The core interface for accessing power data:

```go
type PowerDataProvider interface {
    // Snapshot returns the current power data (thread-safe)
    Snapshot() (*Snapshot, error)

    // DataChannel returns a channel that signals when new data is available
    DataChannel() <-chan struct{}

    // ZoneNames returns the names of the available RAPL zones
    ZoneNames() []string
}
```

**Thread Safety Guarantees:**

- `Snapshot()` is safe for concurrent calls
- `DataChannel()` returns a read-only channel
- `ZoneNames()` is safe for concurrent calls
- No blocking operations (data retrieval is non-blocking)

**Usage by Exporters:**

```go
// Prometheus exporter uses the interface
type PowerCollector struct {
    pm PowerDataProvider
}

func (c *PowerCollector) Collect(ch chan<- prometheus.Metric) {
    snapshot, err := c.pm.Snapshot()  // Thread-safe access
    if err != nil {
        return
    }

    // Process snapshot data...
}

// Stdout exporter also uses the same interface
func (e *Exporter) Run(ctx context.Context) error {
    for {
        select {
        case <-e.monitor.DataChannel():  // Wait for new data
            snapshot, _ := e.monitor.Snapshot()
            e.printSnapshot(snapshot)
        case <-ctx.Done():
            return ctx.Err()
        }
    }
}
```

## Hardware Abstraction Interfaces

### CPUPowerMeter Contract

Abstracts hardware energy measurement:

```go
type CPUPowerMeter interface {
    service.Service
    service.Initializer

    // Zones returns all available energy zones
    Zones() ([]EnergyZone, error)

    // PrimaryEnergyZone returns the zone with highest energy coverage
    PrimaryEnergyZone() (EnergyZone, error)
}
```

**Implementation Requirements:**

- `Init()` must validate hardware access
- `Zones()` results should be cached after first call
- `PrimaryEnergyZone()` should return most comprehensive zone
- Thread safety not required (single-goroutine access)

### EnergyZone Contract

Represents individual energy measurement points:

```go
type EnergyZone interface {
    Name() string                    // Zone identifier (package, core, dram, uncore)
    Index() int                     // Zone index for multi-socket systems
    Path() string                   // Hardware path (for debugging)
    Energy() (Energy, error)        // Current energy reading in microjoules
    MaxEnergy() Energy              // Maximum value before wraparound
}
```

**Contract Details:**

- `Energy()` returns monotonically increasing values (except wraparound)
- `MaxEnergy()` defines wraparound boundary for delta calculations
- `Path()` provides debug information (sysfs path, etc.)
- Multiple zones may have same `Name()` (multi-socket aggregation)

**Implementations:**

```go
// RAPL implementation (production)
type sysfsRaplZone struct {
    zone sysfs.RaplZone
}

func (s sysfsRaplZone) Energy() (Energy, error) {
    mj, err := s.zone.GetEnergyMicrojoules()
    return Energy(mj), err
}

// Fake implementation (development)
type fakeEnergyZone struct {
    name      string
    energy    Energy
    increment Energy
}

func (z *fakeEnergyZone) Energy() (Energy, error) {
    z.energy += z.increment  // Simulate energy consumption
    return z.energy, nil
}
```

## Resource Monitoring Interfaces

### Informer Contract

Abstracts system resource monitoring:

```go
type Informer interface {
    service.Service
    service.Initializer

    // Refresh updates all resource information
    Refresh() error

    // Resource accessors (safe after Refresh)
    Node() *Node
    Processes() *Processes
    Containers() *Containers
    VirtualMachines() *VirtualMachines
    Pods() *Pods
}
```

**Usage Contract:**

- `Refresh()` must be called before accessing resource data
- Resource data is valid until next `Refresh()` call
- Thread safety not required (single-goroutine access)
- `Refresh()` should handle transient errors gracefully

### Internal Abstractions

The resource informer uses internal interfaces for flexibility:

```go
// Abstracts process information reading
type allProcReader interface {
    AllProcs() ([]procInfo, error)
    CPUUsageRatio() (float64, error)
}

// Individual process information
type procInfo interface {
    PID() int
    Comm() string
    Exe() string
    CPUTotalTime() float64
    CgroupPath() string
    // ... other process metadata
}
```

**Benefits:**

- Easy to mock for testing
- Can swap implementations (procfs → eBPF in future)
- Clear separation between reading and processing logic

## Export Layer Interfaces

### APIRegistry Contract

Abstracts HTTP endpoint registration:

```go
type APIRegistry interface {
    Register(endpoint, summary, description string, handler http.Handler) error
}
```

**Usage by Exporters:**

```go
func (e *Exporter) Init() error {
    handler := promhttp.HandlerFor(e.registry, promhttp.HandlerOpts{
        EnableOpenMetrics: true,
    })

    return e.server.Register("/metrics", "Metrics", "Prometheus metrics", handler)
}
```

**Implementation:**

```go
type APIServer struct {
    mux    *http.ServeMux
    server *http.Server
}

func (s *APIServer) Register(endpoint, summary, description string, handler http.Handler) error {
    s.mux.Handle(endpoint, handler)
    return nil
}
```

## Configuration Interfaces

### Functional Options Pattern

Kepler uses functional options for flexible service configuration:

```go
// Option function type
type OptionFn func(*PowerMonitor)

// Option constructors
func WithLogger(logger *slog.Logger) OptionFn {
    return func(pm *PowerMonitor) {
        pm.logger = logger
    }
}

func WithInterval(interval time.Duration) OptionFn {
    return func(pm *PowerMonitor) {
        pm.interval = interval
    }
}

// Service constructor
func NewPowerMonitor(meter CPUPowerMeter, opts ...OptionFn) *PowerMonitor {
    pm := &PowerMonitor{
        cpu:          meter,
        interval:     3 * time.Second,  // default
        maxStaleness: 10 * time.Second, // default
    }

    for _, opt := range opts {
        opt(pm)  // Apply each option
    }

    return pm
}
```

**Benefits:**

- Optional parameters with sensible defaults
- Extensible without breaking existing code
- Clear and readable service construction
- Easy to test different configurations

## Kubernetes Integration Interfaces

### Pod Informer Contract

Abstracts Kubernetes pod information:

```go
type Informer interface {
    service.Service
    service.Initializer
    service.Runner  // Runs watch loop

    // LookupByContainerID returns pod information for a container
    LookupByContainerID(containerID string) (ContainerInfo, bool, error)
}

type ContainerInfo struct {
    PodID         string
    PodName       string
    Namespace     string
    ContainerName string
}
```

**Usage:**

```go
func (ri *resourceInformer) refreshPods() error {
    if ri.podInformer == nil {
        return nil  // Kubernetes integration disabled
    }

    for _, container := range ri.containers.Running {
        cntrInfo, found, err := ri.podInformer.LookupByContainerID(container.ID)
        if err != nil {
            continue  // Skip on error
        }

        if found {
            // Associate container with pod
            container.Pod = &Pod{
                ID:        cntrInfo.PodID,
                Name:      cntrInfo.PodName,
                Namespace: cntrInfo.Namespace,
            }
        }
    }

    return nil
}
```

## Testing Interfaces

### Mock Generation

Kepler uses interface-based mocking for testing:

```go
//go:generate mockgen -source=power_meter.go -destination=mock_power_meter.go

type MockCPUPowerMeter struct {
    zones []EnergyZone
}

func (m *MockCPUPowerMeter) Name() string { return "mock" }
func (m *MockCPUPowerMeter) Init() error  { return nil }

func (m *MockCPUPowerMeter) Zones() ([]EnergyZone, error) {
    return m.zones, nil
}

func (m *MockCPUPowerMeter) PrimaryEnergyZone() (EnergyZone, error) {
    if len(m.zones) == 0 {
        return nil, errors.New("no zones")
    }
    return m.zones[0], nil
}
```

### Test Utilities

Common test patterns for interface validation:

```go
func TestServiceContract(t *testing.T, service Service) {
    // Test service interface compliance
    assert.NotEmpty(t, service.Name())

    if initializer, ok := service.(Initializer); ok {
        assert.NoError(t, initializer.Init())
    }

    if runner, ok := service.(Runner); ok {
        ctx, cancel := context.WithTimeout(context.Background(), time.Second)
        defer cancel()

        err := runner.Run(ctx)
        assert.True(t, errors.Is(err, context.DeadlineExceeded) || err == nil)
    }
}

func TestAllServices(t *testing.T) {
    services := []Service{
        &PowerMonitor{},
        &prometheus.Exporter{},
        &stdout.Exporter{},
        // ... other services
    }

    for _, service := range services {
        t.Run(service.Name(), func(t *testing.T) {
            TestServiceContract(t, service)
        })
    }
}
```

## Interface Evolution & Compatibility

### Backward Compatibility

When evolving interfaces, Kepler follows Go best practices:

```go
// Add new methods to new interfaces
type PowerDataProviderV2 interface {
    PowerDataProvider  // Embed existing interface

    // New methods
    HealthCheck() error
    Statistics() Statistics
}

// Use type assertion for optional features
func (c *PowerCollector) collectHealthMetrics() {
    if provider, ok := c.pm.(PowerDataProviderV2); ok {
        if err := provider.HealthCheck(); err != nil {
            // Handle health check
        }
    }
}
```

### Interface Composition

Complex interfaces are built from smaller, focused interfaces:

```go
// Focused interfaces
type EnergyReader interface {
    Energy() (Energy, error)
}

type EnergyMetadata interface {
    Name() string
    Index() int
    Path() string
}

type EnergyLimits interface {
    MaxEnergy() Energy
}

// Composed interface
type EnergyZone interface {
    EnergyReader
    EnergyMetadata
    EnergyLimits
}
```

## Error Handling Contracts

### Error Types

Interfaces define expected error types and handling:

```go
type EnergyUnavailableError struct {
    Zone string
    Err  error
}

func (e *EnergyUnavailableError) Error() string {
    return fmt.Sprintf("energy unavailable for zone %s: %v", e.Zone, e.Err)
}

func (e *EnergyUnavailableError) Unwrap() error {
    return e.Err
}

// Interface contract specifies error types
type EnergyZone interface {
    // Energy returns current energy or EnergyUnavailableError
    Energy() (Energy, error)
}
```

### Error Handling Patterns

```go
func (pm *PowerMonitor) collectZoneEnergy(zone EnergyZone) (Energy, error) {
    energy, err := zone.Energy()
    if err != nil {
        var unavailableErr *EnergyUnavailableError
        if errors.As(err, &unavailableErr) {
            // Handle known error type
            pm.logger.Debug("Zone temporarily unavailable", "zone", zone.Name())
            return 0, nil  // Skip this zone
        }

        // Unknown error type
        return 0, fmt.Errorf("failed to read zone %s: %w", zone.Name(), err)
    }

    return energy, nil
}
```

## Interface Documentation Standards

### Contract Documentation

All interfaces include comprehensive contract documentation:

```go
// PowerDataProvider provides thread-safe access to power consumption data.
//
// Thread Safety:
//   - All methods are safe for concurrent use
//   - Snapshot() returns immutable data
//   - DataChannel() returns a read-only channel
//
// Error Handling:
//   - Snapshot() returns ErrNotReady if system is initializing
//   - Temporary failures should be retried by clients
//   - Permanent failures indicate system misconfiguration
//
// Performance:
//   - Snapshot() is non-blocking and lock-free
//   - DataChannel() notifications are best-effort (may be dropped if channel is full)
type PowerDataProvider interface {
    // Snapshot returns the current power consumption data across all workload levels.
    // The returned snapshot is immutable and safe for concurrent access.
    //
    // Returns:
    //   - Current power data snapshot
    //   - ErrNotReady if system is still initializing
    //   - Other errors indicate system failures
    Snapshot() (*Snapshot, error)

    // DataChannel returns a channel that receives notifications when new power data
    // is available. Clients should call Snapshot() after receiving notifications.
    //
    // The channel is read-only and may drop notifications if the client cannot
    // keep up with data generation.
    DataChannel() <-chan struct{}

    // ZoneNames returns the names of all available RAPL energy zones.
    // The returned slice is safe to modify and will not change during runtime.
    ZoneNames() []string
}
```

---

## Next Steps

After understanding interfaces and contracts:

- **[Configuration](configuration.md)**: Learn how interfaces enable flexible configuration

- **[Components](components.md)**: Review how interfaces connect system components
