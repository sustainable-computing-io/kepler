# Concurrency & Thread Safety

This document explains Kepler's concurrency patterns, thread safety guarantees, and how the system achieves deterministic results regardless of concurrent access patterns.

## Core Principle: Deterministic Processing

> *"Parallel vs Serial processing should not produce different results"*

Kepler's architecture ensures that concurrent access never produces different results than serial access, maintaining data consistency and predictable behavior.

## Thread Safety Guarantees

### Component-Level Thread Safety

| Component                         | Thread Safety  | Access Pattern                  | Notes                    |
|-----------------------------------|----------------|---------------------------------|--------------------------|
| **PowerMonitor** (public methods) | ✅ Thread-safe  | Multiple readers, single writer | Except `Init()`          |
| **Device Layer**                  | ❌ Not required | Single goroutine access         | Called only from monitor |
| **Resource Layer**                | ❌ Not required | Single goroutine access         | Called only from monitor |
| **Snapshot**                      | ✅ Immutable    | Multiple readers                | Copy-on-write semantics  |
| **Exporters**                     | ✅ Thread-safe  | Multiple concurrent exports     | Independent collection   |
| **Service Framework**             | ✅ Thread-safe  | Concurrent lifecycle management | Coordinated shutdown     |

### PowerMonitor Thread Safety

The `PowerMonitor` is the only component that requires explicit thread safety since it's accessed concurrently by multiple exporters:

```go
type PowerMonitor struct {
    // Thread-safe fields
    snapshot     atomic.Pointer[Snapshot]  // Atomic pointer for lock-free reads
    computeGroup singleflight.Group        // Prevent redundant calculations
    exported     atomic.Bool               // Track export state

    // Single-writer fields (no concurrent access)
    cpu       device.CPUPowerMeter
    resources resource.Informer

    // Configuration (read-only after init)
    interval     time.Duration
    maxStaleness time.Duration
}
```

## Concurrency Patterns

### 1. Single Writer, Multiple Readers

The fundamental pattern that ensures data consistency:

```go
// Single writer: Only the collection goroutine updates snapshots
func (pm *PowerMonitor) refreshSnapshot() error {
    newSnapshot := NewSnapshot()

    // Perform all calculations
    pm.calculatePower(newSnapshot)

    // Atomic update - visible to all readers instantly
    pm.snapshot.Store(newSnapshot)

    return nil
}

// Multiple readers: Exporters read snapshots concurrently
func (pm *PowerMonitor) Snapshot() (*Snapshot, error) {
    return pm.snapshot.Load(), nil  // Lock-free read
}
```

**Benefits:**

- No locks required for reads
- Consistent view across all readers
- No partial updates possible

### 2. Singleflight Protection

Prevents redundant calculations when multiple goroutines request fresh data simultaneously:

```go
func (pm *PowerMonitor) synchronizedPowerRefresh() error {
    _, err, _ := pm.computeGroup.Do("compute", func() (any, error) {
        // Double-check pattern: verify freshness after acquiring singleflight lock
        if pm.isFresh() {
            return nil, nil  // Another goroutine already computed fresh data
        }

        return nil, pm.refreshSnapshot()
    })

    return err
}
```

**Scenario Protected:**

```text
Goroutine 1: isFresh() → false → waits for singleflight lock
Goroutine 2: isFresh() → false → waits for singleflight lock
Goroutine 1: acquires lock → computes → releases lock
Goroutine 2: acquires lock → isFresh() → true → returns without computation
```

### 3. Immutable Snapshots

Snapshots are immutable after creation, ensuring thread-safe access:

```go
type Snapshot struct {
    Timestamp time.Time  // Set once during creation

    // All fields are populated once and never modified
    Node              *Node
    Processes         map[string]*Process
    Containers        map[string]*Container
    VirtualMachines   map[string]*VirtualMachine
    Pods              map[string]*Pod

    // Terminated workloads (also immutable)
    TerminatedProcesses      []*Process
    TerminatedContainers     []*Container
    TerminatedVirtualMachines []*VirtualMachine
    TerminatedPods           []*Pod
}

func NewSnapshot() *Snapshot {
    return &Snapshot{
        Node:              &Node{Zones: make(NodeZoneUsageMap)},
        Processes:         make(map[string]*Process),
        Containers:        make(map[string]*Container),
        VirtualMachines:   make(map[string]*VirtualMachine),
        Pods:              make(map[string]*Pod),
        // Terminated slices initialized during calculation
    }
}
```

### 4. Atomic State Management

Simple state changes use atomic operations to avoid locks:

```go
type PowerMonitor struct {
    exported atomic.Bool  // Track whether current snapshot has been exported
}

// Mark snapshot as exported (thread-safe)
func (pm *PowerMonitor) Snapshot() (*Snapshot, error) {
    snapshot := pm.snapshot.Load()
    pm.exported.Store(true)  // Atomic flag update
    return snapshot, nil
}

// Check export state during collection (thread-safe)
func (pm *PowerMonitor) refreshSnapshot() error {
    if pm.exported.Load() {
        // Clear terminated workloads after export
        pm.terminatedProcessesTracker.Clear()
    }

    // Reset export flag
    pm.exported.Store(false)

    return nil
}
```

## Service Framework Concurrency

### Coordinated Lifecycle Management

The service framework manages concurrent service execution using `oklog/run`:

```go
func Run(ctx context.Context, logger *slog.Logger, services []Service) error {
    ctx, cancel := context.WithCancel(ctx)
    defer cancel()

    var g run.Group

    // Add each service to the run group
    for _, s := range services {
        if runner, ok := s.(Runner); ok {
            svc := s  // Capture for closure
            r := runner

            g.Add(
                func() error {
                    return r.Run(ctx)  // Execute service
                },
                func(err error) {
                    cancel()  // Cancel all services on any failure

                    // Graceful shutdown
                    if shutdowner, ok := svc.(Shutdowner); ok {
                        shutdowner.Shutdown()
                    }
                },
            )
        }
    }

    return g.Run()  // Execute all services concurrently
}
```

**Concurrency Features:**

- **Concurrent Execution**: All services run in parallel
- **Failure Propagation**: Any service failure cancels all others
- **Graceful Shutdown**: Services shut down in proper order
- **Context Cancellation**: Clean cancellation propagation

### Service Independence

Services are designed to be independent and not share mutable state:

```go
// Each service has its own dependencies and state
func createServices(cfg *config.Config) []service.Service {
    // Create independent instances
    cpuPowerMeter := device.NewCPUPowerMeter(cfg.Host.SysFS)
    resourceInformer := resource.NewInformer(cfg.Host.ProcFS)
    powerMonitor := monitor.NewPowerMonitor(cpuPowerMeter, resourceInformer)

    // Services communicate through well-defined interfaces
    promExporter := prometheus.NewExporter(powerMonitor, apiServer)
    stdoutExporter := stdout.NewExporter(powerMonitor)

    return []service.Service{
        resourceInformer,
        cpuPowerMeter,
        powerMonitor,
        promExporter,
        stdoutExporter,
    }
}
```

## Exporter Concurrency

### Independent Collection

Each exporter independently accesses the PowerMonitor without coordination:

```go
// Prometheus exporter collects metrics independently
func (c *PowerCollector) Collect(ch chan<- prometheus.Metric) {
    snapshot, err := c.pm.Snapshot()  // Thread-safe access
    if err != nil {
        return
    }

    // Process snapshot data independently
    c.collectNodeMetrics(ch, snapshot.Node)
    c.collectProcessMetrics(ch, snapshot.Processes)
    // ... other metrics
}

// Stdout exporter also accesses independently
func (e *Exporter) Run(ctx context.Context) error {
    for {
        select {
        case <-e.monitor.DataChannel():
            snapshot, _ := e.monitor.Snapshot()  // Same thread-safe access
            e.printSnapshot(snapshot)
        case <-ctx.Done():
            return ctx.Err()
        }
    }
}
```

**Benefits:**

- No coordination required between exporters
- Each exporter can have different collection frequencies
- Independent failure handling
- Easy to add new exporters

### Data Channel Notifications

The PowerMonitor notifies exporters of new data without blocking:

```go
func (pm *PowerMonitor) signalNewData() {
    select {
    case pm.dataCh <- struct{}{}:
        // Successfully notified
    default:
        // Channel full, skip notification (non-blocking)
    }
}

func (pm *PowerMonitor) DataChannel() <-chan struct{} {
    return pm.dataCh  // Read-only channel for exporters
}
```

## Resource Layer Concurrency

### Parallel Resource Processing

Within the resource informer, independent workload types are processed in parallel:

```go
func (ri *resourceInformer) Refresh() error {
    // 1. Refresh processes first (foundation for other types)
    containerProcs, vmProcs, err := ri.refreshProcesses()

    // 2. Process independent workload types in parallel
    var wg sync.WaitGroup
    var cntrErrs, vmErrs, nodeErrs, podErrs error

    wg.Add(3)

    // Containers and pods (containers → pods dependency)
    go func() {
        defer wg.Done()
        cntrErrs = ri.refreshContainers(containerProcs)
        podErrs = ri.refreshPods()  // Depends on containers
    }()

    // VMs (independent)
    go func() {
        defer wg.Done()
        vmErrs = ri.refreshVMs(vmProcs)
    }()

    // Node stats (independent)
    go func() {
        defer wg.Done()
        nodeErrs = ri.refreshNode()
    }()

    wg.Wait()

    return errors.Join(cntrErrs, vmErrs, nodeErrs, podErrs)
}
```

**Safety Considerations:**

- Different goroutines operate on completely separate data structures
- No shared mutable state between parallel operations
- Dependencies respected (containers before pods)

## Power Attribution Concurrency

### Single-Threaded Attribution

Power attribution is intentionally single-threaded to ensure deterministic results:

```go
func (pm *PowerMonitor) refreshSnapshot() error {
    newSnapshot := NewSnapshot()
    prevSnapshot := pm.snapshot.Load()

    // All attribution calculations run in single goroutine
    if prevSnapshot == nil {
        err := pm.firstReading(newSnapshot)
    } else {
        err := pm.calculatePower(prevSnapshot, newSnapshot)
    }

    // Atomic update makes results visible to all readers
    newSnapshot.Timestamp = pm.clock.Now()
    pm.snapshot.Store(newSnapshot)

    return err
}
```

**Why Single-Threaded?**

- **Deterministic Results**: Same inputs always produce same outputs
- **Mathematical Consistency**: Energy conservation enforced across all levels
- **Simplified Reasoning**: No race conditions in complex attribution logic
- **Performance**: Attribution is CPU-bound, not I/O-bound

## Testing Concurrency

### Race Detection

All concurrent code is tested with Go's race detector:

```bash
# Run tests with race detection
go test -race ./...

# Run specific concurrency tests
go test -race -run TestConcurrency ./internal/monitor/
```

### Stress Testing

Concurrency stress tests validate behavior under high contention:

```go
func TestPowerMonitor_ConcurrentAccess(t *testing.T) {
    pm := setupPowerMonitor(t)

    // Start multiple goroutines accessing snapshots
    var wg sync.WaitGroup
    const numGoroutines = 100

    for i := 0; i < numGoroutines; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()

            // Multiple concurrent calls should not race
            for j := 0; j < 100; j++ {
                snapshot, err := pm.Snapshot()
                assert.NoError(t, err)
                assert.NotNil(t, snapshot)
            }
        }()
    }

    wg.Wait()
}
```

### Determinism Testing

Tests verify that concurrent access produces identical results to serial access:

```go
func TestPowerMonitor_DeterministicResults(t *testing.T) {
    pm := setupPowerMonitor(t)

    // Trigger calculation
    snapshot1, _ := pm.Snapshot()

    // Multiple concurrent accesses should return identical data
    var results []*Snapshot
    var mu sync.Mutex
    var wg sync.WaitGroup

    for i := 0; i < 10; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            snapshot, _ := pm.Snapshot()

            mu.Lock()
            results = append(results, snapshot)
            mu.Unlock()
        }()
    }

    wg.Wait()

    // All results should be identical
    for _, result := range results {
        assert.Equal(t, snapshot1, result)
    }
}
```

## Common Concurrency Patterns

### 1. Reader-Writer Pattern

```go
type SafeCounter struct {
    mu    sync.RWMutex
    value int64
}

func (c *SafeCounter) Increment() {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.value++
}

func (c *SafeCounter) Value() int64 {
    c.mu.RLock()
    defer c.mu.RUnlock()
    return c.value
}
```

**Used in**: Terminated workload tracking with priority queues

### 2. Atomic Operations

```go
type AtomicFlag struct {
    flag int64
}

func (f *AtomicFlag) Set() {
    atomic.StoreInt64(&f.flag, 1)
}

func (f *AtomicFlag) IsSet() bool {
    return atomic.LoadInt64(&f.flag) == 1
}
```

**Used in**: Export state tracking, freshness flags

### 3. Channel-Based Coordination

```go
type Coordinator struct {
    dataCh chan struct{}
    done   chan struct{}
}

func (c *Coordinator) Signal() {
    select {
    case c.dataCh <- struct{}{}:
    default: // Non-blocking
    }
}

func (c *Coordinator) Wait(ctx context.Context) error {
    select {
    case <-c.dataCh:
        return nil
    case <-ctx.Done():
        return ctx.Err()
    }
}
```

**Used in**: Data change notifications, service coordination

## Performance Implications

### Lock-Free Reads

Most reads in Kepler are lock-free, providing excellent read performance:

```go
// O(1) lock-free read
func (pm *PowerMonitor) Snapshot() (*Snapshot, error) {
    return pm.snapshot.Load(), nil
}
```

### Single Writer Bottleneck

The single writer pattern creates a natural bottleneck that prevents:

- Inconsistent state
- Race conditions in complex calculations
- Need for complex locking schemes

### Memory Consistency

Atomic pointer updates provide memory consistency guarantees:

- Readers see either old snapshot or new snapshot, never partial updates
- Memory barriers ensure visibility across CPU cores
- No cache coherency issues

---

## Next Steps

After understanding concurrency patterns:

- **[Interfaces](interfaces.md)**: Learn the contracts that enable safe concurrent access
- **[Configuration](configuration.md)**: Configure collection intervals and concurrency parameters
