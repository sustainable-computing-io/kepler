# Kepler Metrics

This document describes the metrics exported by Kepler for monitoring energy consumption at various levels (node, container, process, VM).

## Overview

Kepler exports metrics in Prometheus format that can be scraped by Prometheus or other compatible monitoring systems.

### Metric Types

- **COUNTER**: A cumulative metric that only increases over time
- **GAUGE**: A metric that can increase and decrease

## Metrics Reference

### Node Metrics

These metrics provide energy and power information at the node level.

#### kepler_node_cpu_active_joules_total

- **Type**: COUNTER
- **Description**: Energy consumption of cpu in active state at node level in joules
- **Labels**:
  - `zone`
  - `path`
- **Constant Labels**:
  - `node_name`

#### kepler_node_cpu_active_watts

- **Type**: GAUGE
- **Description**: Power consumption of cpu in active state at node level in watts
- **Labels**:
  - `zone`
  - `path`
- **Constant Labels**:
  - `node_name`

#### kepler_node_cpu_idle_joules_total

- **Type**: COUNTER
- **Description**: Energy consumption of cpu in idle state at node level in joules
- **Labels**:
  - `zone`
  - `path`
- **Constant Labels**:
  - `node_name`

#### kepler_node_cpu_idle_watts

- **Type**: GAUGE
- **Description**: Power consumption of cpu in idle state at node level in watts
- **Labels**:
  - `zone`
  - `path`
- **Constant Labels**:
  - `node_name`

#### kepler_node_cpu_info

- **Type**: GAUGE
- **Description**: CPU information from procfs
- **Labels**:
  - `processor`
  - `vendor_id`
  - `model_name`
  - `physical_id`
  - `core_id`

#### kepler_node_cpu_joules_total

- **Type**: COUNTER
- **Description**: Energy consumption of cpu at node level in joules
- **Labels**:
  - `zone`
  - `path`
- **Constant Labels**:
  - `node_name`

#### kepler_node_cpu_usage_ratio

- **Type**: GAUGE
- **Description**: CPU usage ratio of a node (value between 0.0 and 1.0)
- **Constant Labels**:
  - `node_name`

#### kepler_node_cpu_watts

- **Type**: GAUGE
- **Description**: Power consumption of cpu at node level in watts
- **Labels**:
  - `zone`
  - `path`
- **Constant Labels**:
  - `node_name`

### Container Metrics

These metrics provide energy and power information for containers.

#### kepler_container_cpu_joules_total

- **Type**: COUNTER
- **Description**: Energy consumption of cpu at container level in joules
- **Labels**:
  - `container_id`
  - `container_name`
  - `runtime`
  - `state`
  - `zone`
  - `pod_id`
- **Constant Labels**:
  - `node_name`

#### kepler_container_cpu_watts

- **Type**: GAUGE
- **Description**: Power consumption of cpu at container level in watts
- **Labels**:
  - `container_id`
  - `container_name`
  - `runtime`
  - `state`
  - `zone`
  - `pod_id`
- **Constant Labels**:
  - `node_name`

### Process Metrics

These metrics provide energy and power information for individual processes.

#### kepler_process_cpu_joules_total

- **Type**: COUNTER
- **Description**: Energy consumption of cpu at process level in joules
- **Labels**:
  - `pid`
  - `comm`
  - `exe`
  - `type`
  - `state`
  - `container_id`
  - `vm_id`
  - `zone`
- **Constant Labels**:
  - `node_name`

#### kepler_process_cpu_seconds_total

- **Type**: COUNTER
- **Description**: Total user and system time of cpu at process level in seconds
- **Labels**:
  - `pid`
  - `comm`
  - `exe`
  - `type`
  - `container_id`
  - `vm_id`
- **Constant Labels**:
  - `node_name`

#### kepler_process_cpu_watts

- **Type**: GAUGE
- **Description**: Power consumption of cpu at process level in watts
- **Labels**:
  - `pid`
  - `comm`
  - `exe`
  - `type`
  - `state`
  - `container_id`
  - `vm_id`
  - `zone`
- **Constant Labels**:
  - `node_name`

### Virtual Machine Metrics

These metrics provide energy and power information for virtual machines.

#### kepler_vm_cpu_joules_total

- **Type**: COUNTER
- **Description**: Energy consumption of cpu at vm level in joules
- **Labels**:
  - `vm_id`
  - `vm_name`
  - `hypervisor`
  - `state`
  - `zone`
- **Constant Labels**:
  - `node_name`

#### kepler_vm_cpu_watts

- **Type**: GAUGE
- **Description**: Power consumption of cpu at vm level in watts
- **Labels**:
  - `vm_id`
  - `vm_name`
  - `hypervisor`
  - `state`
  - `zone`
- **Constant Labels**:
  - `node_name`

### Pod Metrics

These metrics provide energy and power information for pods.

#### kepler_pod_cpu_joules_total

- **Type**: COUNTER
- **Description**: Energy consumption of cpu at pod level in joules
- **Labels**:
  - `pod_id`
  - `pod_name`
  - `pod_namespace`
  - `state`
  - `zone`
- **Constant Labels**:
  - `node_name`

#### kepler_pod_cpu_watts

- **Type**: GAUGE
- **Description**: Power consumption of cpu at pod level in watts
- **Labels**:
  - `pod_id`
  - `pod_name`
  - `pod_namespace`
  - `state`
  - `zone`
- **Constant Labels**:
  - `node_name`

### Other Metrics

Additional metrics provided by Kepler.

#### kepler_build_info

- **Type**: GAUGE
- **Description**: A metric with a constant '1' value labeled with version information
- **Labels**:
  - `arch`
  - `branch`
  - `revision`
  - `version`
  - `goversion`

---

This documentation was automatically generated by the gen-metric-docs tool.
