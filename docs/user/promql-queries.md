# Sample PromQL Queries

This guide provides tested, recommended PromQL queries for querying and aggregating Kepler energy and power metrics across various workload levels: Node, Pod, Container, Process, VM, and GPU.

## Best Practices & Important Caveats

1. **Do Not Sum Across Overlapping Zones**:
   CPU power and energy metrics carry a `zone` hardware label (e.g., `package`, `core`, `uncore`, `dram`). The `package` zone already includes `core` and `uncore` energy. Summing across zones will double-count energy consumption. Always filter on a single zone such as `zone="package"`.
2. **Watts (Gauge) vs Joules (Counter)**:
   - Use `*_watts` metrics for instantaneous power readings: `kepler_node_cpu_watts{zone="package"}`.
   - Use `*_joules_total` metrics with `rate()` or `increase()` over a time window for calculated average power or total energy consumed:
     `rate(kepler_node_cpu_joules_total{zone="package"}[5m])`.
3. **Filtering Terminated Series**:
   Per-workload metrics can include `state="terminated"` to preserve fair attribution of exited processes/containers. When reporting live workloads, filter by `state="running"`.

---

## 1. Node Level Queries

### Current Power Consumption by Node and Zone

Calculates the current node power in Watts for each zone:

```promql
sum by (node_name, zone) (kepler_node_cpu_watts)
```

### Active vs. Idle Power by Zone

Compare active workload power against baseline idle power across nodes:

```promql
# Active node power
sum by (node_name, zone) (kepler_node_cpu_active_watts{zone="package"})

# Baseline idle node power
sum by (node_name, zone) (kepler_node_cpu_idle_watts{zone="package"})
```

### Average Node Power over a 5-minute Window

```promql
sum by (node_name) (rate(kepler_node_cpu_joules_total{zone="package"}[5m]))
```

---

## 2. Pod Level Queries

### Total Power Consumption per Namespace

Identify the highest energy-consuming Kubernetes namespaces:

```promql
sum by (pod_namespace) (kepler_pod_cpu_watts{zone="package", state="running"})
```

### Top 10 Energy Consuming Pods

Find the top 10 pods by power usage across the cluster:

```promql
topk(10, sum by (pod_namespace, pod_name) (kepler_pod_cpu_watts{zone="package", state="running"}))
```

### Total Energy Consumed by a Specific Namespace in the Last 1 Hour (in Joules)

```promql
sum(increase(kepler_pod_cpu_joules_total{pod_namespace="default", zone="package"}[1h]))
```

---

## 3. Container Level Queries

### Top 10 Containers by Power Consumption

Find the top 10 containers across the cluster by power consumption (identifying containers uniquely by `container_id` and `container_name`):

```promql
topk(10, sum by (container_name, container_id) (kepler_container_cpu_watts{zone="package", state="running"}))
```

### Container Power Consumption per Node

```promql
sum by (container_name, node_name) (kepler_container_cpu_watts{zone="package", state="running"})
```

---

## 4. Process Level Queries

### Top 10 CPU Consuming Commands / Binaries

```promql
topk(10, sum by (comm) (kepler_process_cpu_watts{zone="package", state="running"}))
```

### Power of a Specific Process by PID

```promql
kepler_process_cpu_watts{pid="1234", zone="package", state="running"}
```

---

## 5. Virtual Machine (VM) Level Queries

### Current VM Power Consumption

```promql
sum by (vm_name, zone) (kepler_vm_cpu_watts)
```

### Top 10 VMs by Energy Consumption

```promql
topk(10, sum by (vm_name) (kepler_vm_cpu_watts{zone="package"}))
```

---

## 6. GPU Power Queries

### Total GPU Power per Pod

```promql
sum by (pod_namespace, pod_name) (kepler_pod_gpu_watts)
```

### Total GPU Power per Container

```promql
sum by (container_name, container_id) (kepler_container_gpu_watts)
```

### GPU Power per Process

```promql
sum by (comm) (kepler_process_gpu_watts)
```
