# Kepler Architecture

## Overview

Kepler (Kubernetes Efficient Power Level Exporter) is a Prometheus exporter that measures energy consumption at container, pod, VM, process, and node levels by reading hardware sensors (Intel RAPL) and attributing power to workloads based on CPU utilization.

**Key Capabilities:**

- Hardware sensor-based energy measurement (Intel RAPL)
- Multi-level power attribution (node → process → container/VM → pod)
- Real-time power monitoring with configurable intervals
- Prometheus metrics export with multiple collectors
- Kubernetes-aware pod and container tracking
- Terminated workload tracking for complete energy accounting

## Architecture Documentation

This section provides comprehensive documentation of Kepler's architecture, design decisions, and implementation patterns.

### Core Architecture

| Document                                          | Description                                                       |
|---------------------------------------------------|-------------------------------------------------------------------|
| **[Design Principles](principles.md)**            | The fundamental principles that drive all architectural decisions |
| **[System Components](components.md)**            | Deep dive into each architectural layer and component             |
| **[Data Flow & Attribution](data-flow.md)**       | Power attribution algorithm and data flow patterns                |
| **[Concurrency & Thread Safety](concurrency.md)** | Thread safety guarantees and concurrent processing                |

### Implementation Details

| Document                                     | Description                                           |
|----------------------------------------------|-------------------------------------------------------|
| **[Interfaces & Contracts](interfaces.md)**  | Key interfaces, service contracts, and API boundaries |
| **[Configuration System](configuration.md)** | Hierarchical configuration and option management      |

## Quick Reference

### System Architecture Overview

![Kepler Architecture Diagram](assets/architecture.svg)

**High-Level Data Flow:**

```text
Hardware (RAPL) → Device Layer → Monitor (Attribution) → Exporters
    ↑                                ↑
/proc filesystem → Resource Layer ----┘
```

### Power Attribution Flow

1. **Hardware Collection**: Read RAPL sensors for total energy
2. **Node Breakdown**: Split energy into Active/Idle based on CPU usage
3. **Workload Attribution**: Distribute active energy via CPU time ratios
4. **Hierarchical Aggregation**: Roll up process → container/VM → pod

### Key Design Principles

- **Fair Power Allocation**: Track terminated workloads for complete attribution
- **Mathematical Integrity**: Enforce energy conservation across all levels
- **M/V Pattern**: Separate computation from presentation
- **Data Freshness**: Configurable staleness with automatic refresh
- **Deterministic Processing**: Thread-safe with consistent results
- **Package Reuse**: Prefer well-maintained libraries over custom implementations
- **Configurable Exposure**: User control over metric collection and export
- **Implementation Abstraction**: Interface-based design for flexibility

## Getting Started

For developers new to the Kepler architecture:

1. **Start with [Design Principles](principles.md)** to understand the fundamental design drivers
2. **Review [System Components](components.md)** for the overall structure
3. **Study [Data Flow](data-flow.md)** to understand the power attribution algorithm
4. **Check [Concurrency](concurrency.md)** for thread safety patterns

For specific implementation work:

- **Adding new workload types**: See [System Components](components.md#resource-monitoring)
- **Adding new exporters**: See [Interfaces](interfaces.md#power-data-provider-contract)
- **Modifying power attribution**: See [Data Flow](data-flow.md#power-attribution-algorithm)
- **Configuration changes**: See [Configuration System](configuration.md)

## Related Documentation

- **[Power Attribution Guide](../../power-attribution-guide.md)**: Detailed explanation of power calculation methodology
- **[Development Setup](../../index.md)**: Setting up the development environment
- **[User Configuration Guide](../../../user/configuration.md)**: End-user configuration options

---

> **Note**: This architecture documentation reflects the current design as of the latest version. For historical context or evolution of design decisions, see the individual component documentation and git history.
