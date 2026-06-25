<!-- SPDX-FileCopyrightText: 2025 The Kepler Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->

# E2E Testing Guide

## Overview

End-to-end (e2e) tests verify that Kepler works correctly as a complete system. Kepler has two types of e2e tests:

| Type           | Location        | Purpose                                   | Requirements                     |
|----------------|-----------------|-------------------------------------------|----------------------------------|
| **Bare-metal** | `test/e2e/`     | Node and process metrics on real hardware | Intel RAPL, root access          |
| **GPU**        | `test/e2e/`     | GPU metrics with fake NVML library        | gcc, root access (no GPU needed) |
| **Kubernetes** | `test/e2e-k8s/` | Pod and container metrics in a cluster    | K8s cluster with Kepler deployed |

Unlike unit tests that mock dependencies, e2e tests run the actual Kepler binary and verify metrics are correctly exposed.

**When to run e2e tests:**

- Before submitting code changes that affect power monitoring, metrics export, or core functionality
- When modifying the monitor, exporter, or device packages
- **Bare-metal**: To verify Kepler works correctly on a new hardware platform
- **Kubernetes**: To verify pod/container attribution works correctly

## Prerequisites

### Hardware Requirements

- **Intel CPU with RAPL support**: The tests require access to Intel Running Average Power Limit (RAPL) energy counters
- **Readable RAPL sysfs interface**: `/sys/class/powercap/intel-rapl` must be accessible

### Software Requirements

- **Go**: See `go.mod` for required version
- **Root access**: Required to read RAPL energy counters
- **stress-ng** (optional): Required for workload-based tests

### Verify Prerequisites

```bash
# Check RAPL availability
ls -la /sys/class/powercap/intel-rapl/

# Check RAPL is readable (may require root)
cat /sys/class/powercap/intel-rapl/intel-rapl:0/energy_uj

```

## Running E2E Tests Locally

### Build the E2E Test Binary

```bash
# From the project root
make test-e2e
```

This builds both the Kepler binary (`bin/kepler`) and the e2e test binary (`bin/kepler-e2e.test`).

### Run All E2E Tests

```bash
# E2E tests require root for RAPL access
sudo ./bin/kepler-e2e.test -test.v
```

### Run Specific Tests

```bash
# Run a single test
sudo ./bin/kepler-e2e.test -test.v -test.run TestKeplerStarts

# Run tests matching a pattern
sudo ./bin/kepler-e2e.test -test.v -test.run "TestMetrics.*"

# Run with timeout
sudo ./bin/kepler-e2e.test -test.v -test.timeout=10m
```

### Optional Flags

```bash
# Use a custom Kepler binary
sudo ./bin/kepler-e2e.test -test.v -kepler.binary=/path/to/kepler

# Use a custom config file
sudo ./bin/kepler-e2e.test -test.v -kepler.config=/path/to/config.yaml

# Use a different metrics port
sudo ./bin/kepler-e2e.test -test.v -kepler.port=9999
```

## Test Organization

The e2e tests are organized by concern across multiple files:

| File                 | Purpose                                                                                |
|----------------------|----------------------------------------------------------------------------------------|
| `suite_test.go`      | Test configuration, timing constants, and shared setup functions                       |
| `helpers_test.go`    | Test utilities: `KeplerInstance`, `MetricsScraper`, `Workload`, polling helpers        |
| `kepler_test.go`     | Core functionality: startup, shutdown, metrics endpoint, build info                    |
| `metrics_test.go`    | Metric format validation: required labels, non-negative values, multiple scrapes       |
| `invariants_test.go` | Energy conservation laws: Total = Active + Idle, process power attribution             |
| `workload_test.go`   | Workload detection: stress-ng detection, power changes under load, terminated tracking |

### Test Configuration

The e2e tests use a dedicated configuration file at `test/testdata/e2e-config.yaml`:

```yaml
monitor:
  interval: 3s              # Collection interval
  staleness: 10s            # Max data staleness
  maxTerminated: 100        # Track up to 100 terminated processes
  minTerminatedEnergyThreshold: 1  # 1 joule minimum for terminated tracking
```

## What Is Covered

### Core Functionality

- **Kepler startup and shutdown**: Verifies graceful lifecycle management
- **Metrics endpoint availability**: `/metrics` returns valid Prometheus format
- **Build info metric**: `kepler_build_info` with version labels

### Node-Level Metrics

- `kepler_node_cpu_joules_total` - Total energy consumption
- `kepler_node_cpu_watts` - Current power draw
- `kepler_node_cpu_active_joules_total` / `kepler_node_cpu_active_watts` - Active power
- `kepler_node_cpu_idle_joules_total` / `kepler_node_cpu_idle_watts` - Idle power
- `kepler_node_cpu_usage_ratio` - CPU utilization (0.0 to 1.0)
- `kepler_node_cpu_info` - CPU hardware information

### Process-Level Metrics

- `kepler_process_cpu_joules_total` - Per-process energy
- `kepler_process_cpu_watts` - Per-process power
- `kepler_process_cpu_seconds_total` - Per-process CPU time

### Energy Conservation Invariants

- **Power conservation**: Total Watts = Active Watts + Idle Watts
- **Energy conservation**: Total Joules = Active Joules + Idle Joules
- **Process attribution**: Sum of process power ≈ Node active power
- **Monotonicity**: Energy counters never decrease

### Workload Detection

- Stress workloads appear in process metrics
- Power increases under CPU load
- Power decreases after load removal
- Multiple workloads are individually attributed
- Terminated processes tracked with `state=terminated`

## What Is NOT Covered (and Why)

| Feature                    | Reason Not Tested in Bare-Metal E2E                                      | Where It's Tested                             |
|----------------------------|--------------------------------------------------------------------------|-----------------------------------------------|
| VM metrics (`kepler_vm_*`) | Requires libvirt/hypervisor not available in test environment            | Unit tests in `internal/monitor/vm_test.go`   |
| Redfish platform metrics   | Requires BMC hardware access not available in CI/dev environments        | Unit tests in `internal/platform/redfish/`    |
| Metrics level filtering    | Low priority; config parsing is unit tested; e2e uses full metrics level | Unit tests in `config/`                       |
| pprof debug endpoints      | Debug feature with low e2e value                                         | Unit tests in `internal/server/pprof_test.go` |

> **Note**: Container and Pod metrics are tested in the [Kubernetes E2E tests](#kubernetes-e2e-tests) which run against a real cluster.

## CI Workflow

E2E tests run automatically on pull requests via GitHub Actions.

**Workflow file**: `.github/workflows/e2e.yaml`

### Key Details

- **Trigger**: Runs on pull requests when relevant files change (`cmd/`, `internal/`, `config/`, `test/`, `go.mod`, `go.sum`)
- **Runner**: Uses self-hosted runners with RAPL-enabled hardware
- **Commands**:

```bash
  make test-e2e
  sudo ./bin/kepler-e2e.test -test.v -test.timeout=15m
  ```

### Why Self-Hosted Runners?

Standard GitHub-hosted runners use virtualized environments without access to hardware power meters. E2E tests require:

1. Physical Intel CPU with RAPL support
2. Root access to read `/sys/class/powercap/`
3. Kernel support for powercap subsystem

## Kubernetes E2E Tests

In addition to bare-metal e2e tests, Kepler has Kubernetes-specific e2e tests that verify pod and container metrics work correctly in a real cluster environment.

### Location

Kubernetes e2e tests are located in `test/e2e-k8s/` and use the [sigs.k8s.io/e2e-framework](https://github.com/kubernetes-sigs/e2e-framework) testing framework.

### K8s Prerequisites

- **Kubernetes cluster**: A running cluster with Kepler deployed (Kind, minikube, or real cluster)
- **Kepler DaemonSet**: Kepler must be deployed and running in the `kepler` namespace
- **kubectl access**: Valid kubeconfig with cluster access

### Running K8s E2E Tests

**Important**: Kepler must be deployed and running in your cluster before running K8s e2e tests.

```bash
# Option 1: Local Kind cluster (recommended for development)
make cluster-up                    # Create Kind cluster
make image deploy                  # Build and deploy Kepler with default image

# Option 2: Existing cluster with custom image
make deploy IMG_BASE=your-registry.com/yourorg VERSION=v1.0.0

# Verify Kepler is running
kubectl get pods -n kepler
# Wait until kepler pods are Running

# Run the tests
make test-e2e-k8s
```

### K8s Test Files

| File                 | Purpose                                                               |
|----------------------|-----------------------------------------------------------------------|
| `main_test.go`       | TestMain setup: environment, port-forwarding, namespace management    |
| `helpers_test.go`    | Test utilities: workload deployment, metric waiting, snapshot helpers |
| `node_test.go`       | Node-level metrics presence and labels                                |
| `pod_test.go`        | Pod metrics presence, labels, and non-negative values                 |
| `container_test.go`  | Container metrics presence, labels, and non-negative values           |
| `hierarchy_test.go`  | Pod-container hierarchy validation (pod_id linkage)                   |
| `workload_test.go`   | Workload detection, power attribution, energy accumulation            |
| `terminated_test.go` | Terminated pod/container tracking                                     |
| `invariants_test.go` | Power attribution invariants (pod=Σcontainers, container=Σprocesses)  |

### K8s Test Coverage

#### Pod and Container Metrics

- `kepler_pod_cpu_watts` / `kepler_pod_cpu_joules_total` - Pod power metrics
- `kepler_container_cpu_watts` / `kepler_container_cpu_joules_total` - Container power metrics
- Required labels: `pod_name`, `pod_namespace`, `container_name`, `container_id`, etc.

#### K8s-Specific Invariants

- **Pod = Σ(Containers)**: Pod power equals sum of its container powers (per zone)
- **Container = Σ(Processes)**: Container power equals sum of its process powers (per zone)

#### Workload Lifecycle

- Deployed workloads appear in metrics
- Power is attributed to running pods/containers
- Terminated workloads tracked with `state=terminated`

### Local Development with Kind

```bash
# Create Kind cluster and deploy Kepler
make cluster-up
make image deploy

# Verify Kepler is running
kubectl get pods -n kepler -w

# Run the K8s e2e tests
make test-e2e-k8s

# Cleanup
make undeploy cluster-down
```

## Troubleshooting

### RAPL Not Available

**Error**: `Skipping: RAPL not available at /sys/class/powercap/intel-rapl`

**Solutions**:

- Verify you're on physical hardware (not a VM)
- Check kernel support: `ls /sys/class/powercap/`
- Load the intel_rapl kernel module: `sudo modprobe intel_rapl_common`

### Permission Denied Reading RAPL

**Error**: `Skipping: RAPL energy not readable`

**Solutions**:

- Run tests with sudo: `sudo ./bin/kepler-e2e.test -test.v`
- Check file permissions: `ls -la /sys/class/powercap/intel-rapl/intel-rapl:0/energy_uj`

### stress-ng Not Found

**Error**: `Skipping: stress-ng not found`

**Solution**: Install stress-ng (see Prerequisites section)

### Tests Timeout

**Error**: `panic: test timed out`

**Solutions**:

- Increase timeout: `-test.timeout=20m`
- Check if Kepler is starting correctly (look for port binding errors)
- Verify no other Kepler instance is using port 28282

### Kepler Binary Not Found

**Error**: `kepler binary not found at bin/kepler`

**Solution**: Build Kepler first: `make build` or `make test-e2e`

### Port Already in Use

**Error**: `bind: address already in use`

**Solutions**:

- Kill any running Kepler processes: `sudo pkill kepler`
- Use a different port: `-kepler.port=9999`

## Writing New E2E Tests

When adding new e2e tests, follow these guidelines:

1. **Use existing helpers**: `setupKeplerForTest()`, `MetricsScraper`, `StartWorkload()`
2. **Use `t.Helper()`**: Mark helper functions appropriately
3. **Use `t.Cleanup()`**: Register cleanup functions instead of defer in helpers
4. **Prefer polling over sleeping**: Use `WaitForValidCPUMetrics()`, `WaitForProcessInMetrics()` helpers
5. **Log diagnostic information**: Use `t.Logf()` for debugging
6. **Keep tests independent**: Each test should start fresh without depending on other tests

### Example Test Structure

```go
func TestNewFeature(t *testing.T) {
    // Setup Kepler with standard prerequisites
    _, scraper := setupKeplerForTest(t)

    // Wait for metrics to be ready (use polling, not sleep)
    require.True(t, WaitForValidCPUMetrics(t, scraper, 30*time.Second),
        "Kepler should have valid CPU metrics")

    // Take a snapshot and verify
    snapshot, err := scraper.TakeSnapshot()
    require.NoError(t, err)

    // Assert expected behavior
    assert.True(t, snapshot.HasMetric("kepler_new_metric"))

    // Log for debugging
    t.Logf("Found metric with value: %v", snapshot.GetAllWithName("kepler_new_metric"))
}
```

## GPU E2E Tests (Fake NVML)

### What It Is

GPU e2e tests verify Kepler's full GPU metrics pipeline — from NVML device discovery through power collection to Prometheus export — without requiring real NVIDIA hardware. They work by replacing the real `libnvidia-ml.so.1` shared library with a fake one that returns canned GPU data read from a JSON config file.

This means the GPU code paths that are normally untestable in CI (because they require physical GPUs) can now be tested on any Linux machine with `gcc` and root access.

### How It Works

Kepler's GPU support relies on [go-nvml](https://github.com/NVIDIA/go-nvml), which loads `libnvidia-ml.so.1` at runtime via `dlopen`. go-nvml then resolves individual NVML functions (e.g. `nvmlDeviceGetPowerUsage`) via `dlsym`. The fake library exploits this dynamic loading:

```text
┌──────────────────┐     dlopen      ┌─────────────────────┐
│     go-nvml      │ ──────────────► │  libnvidia-ml.so.1  │
│  (Kepler's GPU   │     dlsym       │  (FAKE: reads JSON  │
│   collector)     │ ◄────────────── │   from env var)     │
└──────────────────┘                 └─────────────────────┘
         │                                     ▲
         │  LD_LIBRARY_PATH points             │ FAKE_NVML_CONFIG
         │  to dir with fake .so               │ points to JSON
         ▼                                     │
┌──────────────────┐                 ┌─────────────────────┐
│  Kepler binary   │                 │  fake_nvml_config   │
│  exports GPU     │                 │  .json              │
│  metrics via     │                 │  {"devices": [...]} │
│  /metrics        │                 └─────────────────────┘
└──────────────────┘
```

**Key mechanism**: The test sets `LD_LIBRARY_PATH` to a temp directory containing the fake `.so`, so `dlopen("libnvidia-ml.so.1")` loads the fake instead of looking for a real NVIDIA driver. The fake library reads device configuration from the path in `FAKE_NVML_CONFIG`.

### Main Design Ideas

1. **One fake library, many scenarios**: The same C shared library supports all test scenarios. Different JSON configs produce different GPU topologies (single GPU, multiple GPUs, with/without processes, idle vs. loaded).

2. **Real PIDs for process attribution**: Tests start actual `sleep infinity` processes and use their real PIDs in the fake NVML config. This lets Kepler find them in `/proc`, exercising the full process-to-GPU attribution path.

3. **Accurate C ABI compatibility**: The fake implements the exact C struct layouts and calling conventions from `nvml.h` that go-nvml expects, including version-negotiation symbols (v1/v2/v3 variants) and the two-call pattern for `GetProcessUtilization` (first call returns count, second fills buffer).

4. **Simulated energy counters**: `nvmlDeviceGetTotalEnergyConsumption` returns `powerUsageMilliWatts × elapsed_seconds` since init using `CLOCK_MONOTONIC`, so energy increases monotonically just like real hardware.

5. **No build tags or special flags**: GPU tests live in the same `test/e2e/` package as bare-metal tests and are selected via `-test.run TestGPU`.

### File Layout

```text
test/e2e/
├── fake_nvml/
│   ├── fake_nvml.c          # Fake NVML C library (~420 lines, 44 exported symbols)
│   ├── cJSON.c              # Vendored MIT-licensed JSON parser (github.com/DaveGamble/cJSON)
│   └── cJSON.h
├── gpu_helpers_test.go       # Build helpers, config presets, setup functions
└── gpu_fake_test.go          # 7 GPU test cases
```

### JSON Config Format

The fake NVML library reads configuration from the file pointed to by `FAKE_NVML_CONFIG`:

```json
{
  "devices": [
    {
      "uuid": "GPU-FAKE-0000-0001",
      "name": "NVIDIA Fake A100",
      "powerUsageMilliWatts": 150000,
      "computeMode": 0,
      "migEnabled": false,
      "maxMigDevices": 0,
      "processes": [
        {
          "pid": 12345,
          "memoryUsed": 1073741824,
          "smUtil": 60
        }
      ]
    }
  ]
}
```

| Field                  | Description                                               |
|------------------------|-----------------------------------------------------------|
| `uuid`                 | GPU UUID reported to Kepler                               |
| `name`                 | GPU model name                                            |
| `powerUsageMilliWatts` | Constant power draw in milliwatts                         |
| `computeMode`          | 0=default (time-slicing), 1=exclusive thread, 3=exclusive |
| `migEnabled`           | Whether MIG mode is active                                |
| `maxMigDevices`        | Number of MIG device slots                                |
| `processes`            | Array of GPU processes (use real PIDs from running procs) |

### Test Cases

| Test                                | What It Verifies                                                   |
|-------------------------------------|--------------------------------------------------------------------|
| `TestGPU_DeviceDiscovered`          | `kepler_node_gpu_info` exists with correct uuid/name/vendor labels |
| `TestGPU_NodePowerMetrics`          | `kepler_node_gpu_watts` > 0 for the fake GPU                       |
| `TestGPU_IdlePower`                 | No processes → active power ≈ 0, total power = idle power          |
| `TestGPU_ProcessPowerAttribution`   | Real PID in fake config → `kepler_node_gpu_active_watts` > 0       |
| `TestGPU_MultipleDevices`           | Two fake GPUs both appear in `kepler_node_gpu_info`                |
| `TestGPU_EnergyIncreases`           | GPU power is consistently positive across two scrapes              |
| `TestGPU_GracefulStartupWithoutGPU` | Kepler starts and serves CPU metrics even when NVML init fails     |

### GPU Prerequisites

- **gcc**: Required to compile `fake_nvml.c` at test time
- **Root access**: Kepler needs root to read `/proc/*/exe` for all processes

No GPU hardware, NVIDIA drivers, or CUDA toolkit required.

### Running GPU E2E Tests

```bash
# Build Kepler binary + GPU e2e test binary
make test-e2e-gpu

# Run all GPU tests (requires root)
sudo ./bin/kepler-e2e-gpu.test -test.v -test.run TestGPU

# Run a single test
sudo ./bin/kepler-e2e-gpu.test -test.v -test.run TestGPU_DeviceDiscovered

# With timeout
sudo ./bin/kepler-e2e-gpu.test -test.v -test.run TestGPU -test.timeout=3m
```

### GPU Metrics Tested

| Metric                         | Type  | Labels                                  |
|--------------------------------|-------|-----------------------------------------|
| `kepler_node_gpu_info`         | Gauge | `gpu`, `gpu_uuid`, `gpu_name`, `vendor` |
| `kepler_node_gpu_watts`        | Gauge | `gpu_uuid`                              |
| `kepler_node_gpu_active_watts` | Gauge | `gpu_uuid`                              |

### GPU Troubleshooting

#### gcc Not Found

**Error**: `Skipping GPU e2e test: gcc not found`

**Solution**: Install gcc (`sudo dnf install gcc` or `sudo apt install gcc`)

#### GPU Metrics Not Appearing

If GPU metrics don't show up in scrapes, check the Kepler stderr output for:

- `fake_nvml: loaded N device(s)` — confirms the fake library loaded the config
- `discovered GPU` — confirms go-nvml found the fake device
- `Exporting GPU metrics` — confirms the power collector is emitting GPU metrics
- `Failed to collect power data` — indicates a Snapshot error (usually means not running as root)

#### Port Conflict

GPU tests use port **28283** (vs. 28282 for bare-metal tests) to avoid conflicts. If you see `bind: address already in use`, kill stale processes: `sudo pkill -f kepler-e2e`

### Writing New GPU Tests

```go
func TestGPU_NewScenario(t *testing.T) {
    skipIfNoGCC(t)

    // Define GPU topology via JSON config
    config := fakeNVMLConfig{
        Devices: []fakeNVMLDevice{{
            UUID:                 "GPU-FAKE-0000-0001",
            Name:                 "NVIDIA Fake A100",
            PowerUsageMilliWatts: 75000,
        }},
    }

    // Start Kepler with fake NVML wired up
    _, scraper := setupKeplerWithFakeGPU(t, config)

    // Wait for GPU metrics to appear
    require.True(t, waitForGPUMetrics(t, scraper, "kepler_node_gpu_watts", 30*time.Second))

    // Scrape and assert
    snapshot, err := scraper.TakeSnapshot()
    require.NoError(t, err)

    power := snapshot.SumValues("kepler_node_gpu_watts", nil)
    assert.Greater(t, power, 0.0)
}
```

Available helpers in `gpu_helpers_test.go`:

| Helper                        | Purpose                                              |
|-------------------------------|------------------------------------------------------|
| `buildFakeNVML(t)`            | Compiles fake `.so`, returns directory path          |
| `writeFakeNVMLConfig(t,c)`    | Writes JSON config, returns file path                |
| `writeGPUKeplerConfig(t)`     | Writes Kepler YAML with GPU enabled + fake CPU meter |
| `setupKeplerWithFakeGPU(t,c)` | One-call setup: build + config + start Kepler        |
| `singleGPUIdle()`             | Preset: 1 GPU at 40W, no processes                   |
| `singleGPUWithProcesses(p)`   | Preset: 1 GPU at 150W with given processes           |
| `waitForGPUMetrics(t,s,m,d)`  | Polls until a named GPU metric appears               |

## Related Documentation

- [Architecture Overview](design/architecture/) - Understanding Kepler's design
- [Power Attribution Guide](power-attribution-guide.md) - How power is measured and attributed
- [Pre-commit Setup](pre-commit.md) - Code quality checks before committing
