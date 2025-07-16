# Kepler Power Attribution Guide

This guide explains how Kepler measures and attributes power consumption to processes, containers, VMs, and pods.

## How Power Measurement Works

Kepler's power attribution follows a simple but effective approach: measure total system energy consumption from hardware, then distribute it fairly to individual workloads based on their resource usage.

### The Big Picture

Think of your computer like an apartment building with a single electricity meter. The meter shows total power consumption (e.g., 40W), but you need to know how much each apartment (process) is using. Kepler solves this by:

1. **Reading the main meter** - Hardware sensors (Intel RAPL) provide total energy consumption
2. **Understanding system activity** - Monitor CPU usage to determine how "busy" the system is
3. **Splitting costs fairly** - Divide energy between "active work" and "idle baseline"
4. **Allocating to tenants** - Give each process power proportional to their CPU usage

### Core Insight: Active vs Idle Power

The key insight is that system power has two components:

- **Active Power**: Energy consumed doing actual work (running processes)
- **Idle Power**: Baseline energy for keeping the system running (even when idle)

If your system uses 25% of CPU capacity, then 25% of total power goes to "active" and 75% stays as "idle."

### Attribution Principle

Once Kepler knows the active power available, it distributes it proportionally:

```text
Process Power = (Process CPU Time / Total CPU Time) × Active Power
```

This ensures that processes consuming more CPU get more power attribution, while the total never exceeds what hardware actually measured.

## Overview

Kepler uses a hierarchical power attribution system that starts with hardware energy measurements and distributes power proportionally based on CPU utilization. The system ensures energy conservation while providing fair attribution across workloads.

![Power Attribution Diagram](assets/power-attribution.png)

*Figure 1: Power attribution flow showing how 40W total power is split between active (10W) and idle (30W) components, then distributed to workloads based on CPU usage ratios.*

### Real-World Example

Using the diagram above:

- **Hardware reports**: 40W total system power
- **System analysis**: 25% CPU usage ratio
- **Power split**: 40W × 25% = 10W active, 30W idle
- **VM attribution**: VM uses 100% of active CPU → gets all 10W active power
- **Container breakdown**: Within the VM, containers get proportional shares of the 10W

## Architecture Components

### 1. Hardware Energy Reading (`internal/device/`)

The device layer provides the foundation for all power measurements:

#### Energy Zones

- **Package**: CPU package-level energy consumption
- **Core**: Individual CPU core energy
- **DRAM**: Memory subsystem energy
- **Uncore**: Integrated graphics and other uncore components
- **PSys**: Platform-level energy (most comprehensive when available)

#### Key Interfaces

- `EnergyZone`: Interface for reading energy from hardware zones
- `CPUPowerMeter`: Main interface for accessing energy zones
- `AggregatedZone`: Combines multiple zones of the same type

#### Energy Types

- **Energy**: Measured in microjoules (μJ) as cumulative counters
- **Power**: Calculated as rate in microwatts (μW) using `Power = ΔEnergy / Δtime`

#### Wraparound Handling

Hardware energy counters have maximum values and wrap around to zero.
Kepler handles this in `calculateEnergyDelta()`:

```go
func calculateEnergyDelta(current, previous, maxJoules Energy) Energy {
    if current >= previous {
        return current - previous
    }
    // Handle counter wraparound
    if maxJoules > 0 {
        return (maxJoules - previous) + current
    }
    return 0 // Unable to calculate delta
}
```

### 2. Node-Level Power Calculation (`internal/monitor/node.go`)

The node calculation is the first step in power attribution, splitting total hardware energy into active and idle components.

#### CPU Usage Calculation

```go
nodeCPUTimeDelta := pm.resources.Node().ProcessTotalCPUTimeDelta
nodeCPUUsageRatio := pm.resources.Node().CPUUsageRatio
```

#### Energy Split Algorithm

For each energy zone, Kepler calculates:

```go
deltaEnergy := calculateEnergyDelta(absEnergy, prevZone.EnergyTotal, zone.MaxEnergy())
activeEnergy = Energy(float64(deltaEnergy) * nodeCPUUsageRatio)
idleEnergy := deltaEnergy - activeEnergy
```

**Key Principle**: Active energy represents the portion consumed by CPU-intensive work, while idle energy represents baseline system power consumption.

#### Power Calculation

```go
powerF64 := float64(deltaEnergy) / float64(timeDiff)
power = Power(powerF64)
activePower = Power(powerF64 * nodeCPUUsageRatio)
idlePower = power - activePower
```

### 3. Process Power Attribution (`internal/monitor/process.go`)

Individual processes receive power proportional to their CPU time usage relative to total system CPU time.

#### Attribution Formula

For each running process:

```go
cpuTimeRatio := proc.CPUTimeDelta / nodeCPUTimeDelta
activeEnergy := Energy(cpuTimeRatio * float64(nodeZoneUsage.activeEnergy))
```

#### Power Assignment

```go
process.Zones[zone] = Usage{
    Power:       Power(cpuTimeRatio * nodeZoneUsage.ActivePower.MicroWatts()),
    EnergyTotal: absoluteEnergy,
}
```

#### Cumulative Energy Tracking

Process energy accumulates over time:

```go
absoluteEnergy := activeEnergy
if prev, exists := prev.Processes[pid]; exists {
    if prevUsage, hasZone := prev.Zones[zone]; hasZone {
        absoluteEnergy += prevUsage.EnergyTotal
    }
}
```

## Attribution Flow Example

Using the diagram as reference, here's how 40W total power gets attributed:

### Step 1: Hardware Measurement

- RAPL sensors report total energy consumption for the measurement interval
- Convert to power: `40W total power`

### Step 2: Node CPU Usage Analysis

- System reports 25% CPU usage ratio
- Split power: `40W × 25% = 10W active`, `40W - 10W = 30W idle`

### Step 3: Process Attribution

- VM process uses 100% of active CPU time
- VM gets: `10W × (100% CPU usage) = 10W`
- Container processes within VM get proportional shares of the 10W

### Step 4: Workload Aggregation

- **Container power** = sum of constituent process power
- **VM power** = sum of all processes in the VM
- **Pod power** = sum of container power (in Kubernetes)

## Key Principles

### 1. Energy Conservation

The total attributed power always equals the measured hardware power:

```text
Σ(Process Power) + Idle Power = Total Hardware Power
```

### 2. Proportional Attribution

Power distribution is strictly proportional to CPU time usage:

```text
Process Power = (Process CPU Time / Total CPU Time) × Active Power
```

### 3. Hierarchical Aggregation

Higher-level workloads inherit power from their constituent processes:

- **Pods** = sum of container power
- **Containers** = sum of process power
- **VMs** = sum of process power

### 4. Idle Power Handling

Idle power represents baseline system consumption and is tracked separately but not attributed to individual workloads.

## Implementation Details

### Thread Safety

- **Device Layer**: Not required to be thread-safe (single monitor goroutine)
- **Monitor Layer**: All public methods except `Init()` must be thread-safe
- **Singleflight Pattern**: Prevents redundant power calculations during concurrent requests

### Data Freshness

- Configurable staleness threshold ensures data isn't stale
- Atomic snapshots provide consistent power readings across all workloads

### Terminated Process Handling

- Terminated processes are tracked in a separate collection
- Power attribution continues until the next export cycle
- Priority-based retention manages memory usage

### Error Handling

- Individual zone read failures don't stop attribution
- Graceful degradation when hardware sensors are unavailable
- Comprehensive logging for debugging attribution issues

## Configuration

### Key Settings

- **Collection Interval**: How frequently to read hardware sensors
- **Staleness Threshold**: Maximum age of cached power data
- **Zone Filtering**: Which RAPL zones to use for attribution
- **Fake Meter**: Development mode when hardware unavailable

### Development Mode

```bash
# Use fake CPU meter for development
sudo ./bin/kepler --dev.fake-cpu-meter.enabled --config hack/config.yaml
```

## Monitoring and Debugging

### Metrics Access

- **Local**: `http://localhost:28282/metrics`
- **Compose**: `http://localhost:28283/metrics`
- **Grafana**: `http://localhost:23000`

### Debug Options

```bash
# Enable debug logging
--log.level=debug

# Use stdout exporter for immediate inspection
--exporter.stdout

# Enable performance profiling
--debug.pprof
```

### Key Metrics

- `kepler_node_cpu_watts{}`: Total node power consumption
- `kepler_process_cpu_watts{}`: Individual process power
- `kepler_container_cpu_watts{}`: Container-level aggregation
- `kepler_vm_cpu_watts{}`: Virtual machine power attribution

## Conclusion

Kepler's power attribution system provides accurate, proportional distribution of hardware energy consumption to individual workloads. By using CPU utilization as the primary attribution factor and maintaining strict energy conservation, Kepler enables fine-grained energy accounting for modern containerized and virtualized environments.

The implementation balances accuracy with performance, providing thread-safe concurrent access while minimizing the overhead of continuous power monitoring.
