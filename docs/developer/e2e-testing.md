<!-- SPDX-FileCopyrightText: 2025 The Kepler Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->

# E2E Testing Guide

## Overview

End-to-end (e2e) tests verify that Kepler works correctly as a complete system. Kepler has two types of e2e tests:

| Type           | Location        | Purpose                                   | Requirements                     |
|----------------|-----------------|-------------------------------------------|----------------------------------|
| **Bare-metal** | `test/e2e/`     | Node and process metrics on real hardware | Intel RAPL, root access          |
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

# Run the GPU tests against the fake GPU meter instead of a real NVIDIA GPU
sudo ./bin/kepler-e2e.test -test.v -gpu.fake

# Override the image or the dcgmproftester build used for the GPU load test
sudo ./bin/kepler-e2e.test -test.v -test.run TestGPU -gpu.load-image=IMAGE -gpu.load-binary=/usr/bin/dcgmproftester13
```

### Run the GPU Tests on a Real GPU

By default the GPU tests run against a real NVIDIA GPU and use `test/testdata/e2e-gpu-config.yaml`, unless `-kepler.config` is set; the other tests keep the default config. That config uses the fake CPU meter, so the host does not need RAPL. Cloud GPU VMs usually have none. On a host without an NVIDIA GPU the GPU tests skip; run with `-gpu.fake` to test them against the fake GPU meter instead, which needs RAPL like the other tests.

Prerequisites:

- An NVIDIA GPU with its driver and `nvidia-smi`
- GPU persistence mode on (`sudo nvidia-smi -pm 1`). With it off, the GPU powers up each time a new Kepler starts and only drops back to idle after a while, so Kepler's idle baseline starts too high
- Docker set up for GPUs, for the load test. On the CI runner this is `sudo nvidia-ctk runtime configure --runtime=docker --set-as-default` followed by `sudo systemctl restart docker`
- Root access, as for the other e2e tests

```bash
make test-e2e
sudo ./bin/kepler-e2e.test -test.v -test.run TestGPU
```

On a real GPU, three more tests run:

- `TestGPURealHardware`: the GPU is an NVIDIA device with an NVML UUID, and it draws power
- `TestGPUPowerMatchesNvidiaSMI`: Kepler's GPU power matches `nvidia-smi` within 5% or 3 W, whichever is larger
- `TestGPUWorkloadAttribution`: loads the GPU with `dcgmproftester` from NVIDIA's DCGM image and checks that the load's processes and container get the active GPU power. It skips when another process is using a GPU, since the load must be the only GPU process on the host

Kepler's GPU readings can trail the GPU by several seconds, so these tests wait for fresh readings before checking. The load test pulls `nvcr.io/nvidia/cloud-native/dcgm:4.6.1-1-ubuntu24.04`, a few GB, the first time it runs, and picks the `dcgmproftester` build that matches the driver's CUDA version (for example `dcgmproftester13` for CUDA 13). A CUDA 13.0 driver with a GPU older than compute capability 7.5 (Maxwell, Pascal or Volta) gets `dcgmproftester12`, which DCGM requires there.

### Run the GPU Tests on Your Own GPU First

A run on the CNCF A10 needs a maintainer and a GPU shared by all CNCF projects (see [GPU E2E Workflow](#gpu-e2e-workflow)). If your pull request touches GPU code and you have an NVIDIA GPU, run the GPU tests on it first and post the result in the pull request. The CNCF run then confirms it on the A10.

**On any NVIDIA machine**, such as a workstation or a cloud GPU VM, with the prerequisites above:

```bash
make test-e2e
sudo nvidia-smi -pm 1
sudo ./bin/kepler-e2e.test -test.v -test.timeout=20m -test.run TestGPU 2>&1 | tee gpu-e2e.log

# The same run summary the workflow writes, as Markdown you can paste in the pull request
GPU_E2E_TARGET="$(git branch --show-current)" GPU_E2E_SHA="$(git rev-parse HEAD)" GPU_E2E_RUNNER="my workstation" \
  hack/gpu-e2e/summary.sh gpu-e2e.log > gpu-e2e-summary.md
```

`hack/gpu-e2e/summary.sh` also takes a second file with one `seconds,watts` line per `nvidia-smi` sample, as `hack/gpu-e2e/sample.sh` records them, and then adds the GPU power over the run.

**In a GPU container**, for example on Modal: the GPU tests start the Kepler binary and read its metrics on `localhost`, so they need a GPU but not a VM or a Kubernetes cluster. Kepler has run this way on a Modal T4 container, with NVML working, the GPU found and `kepler_node_gpu_watts` reporting live power ([#2506](https://github.com/sustainable-computing-io/kepler/issues/2506)). Compared with a full machine:

- The container needs `nvidia-smi` from the NVIDIA driver, Go, and a C compiler: `make test-e2e` builds with `CGO_ENABLED=1`
- You usually run as root in the container, so leave out `sudo`
- Most GPU containers cannot run Docker, so `TestGPUWorkloadAttribution` skips with "docker not found" and the other six GPU tests run. The load test needs a machine with Docker set up for GPUs, like the CNCF runner
- Persistence mode is set on the host, and the container may not be allowed to change it. Check it with `nvidia-smi --query-gpu=persistence_mode --format=csv`; with it off, Kepler's idle baseline can start too high
- Such a container cannot run a Kubernetes cluster, so it only covers these e2e tests, not the Kubernetes ones

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
| `gpu_test.go`        | GPU metrics on a real GPU, or the fake GPU with `-gpu.fake`; GPU load attribution      |

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
- `kepler_node_gpu_watts` - Total GPU power (a real GPU, or the fake GPU with `-gpu.fake`)
- `kepler_node_gpu_active_watts` / `kepler_node_gpu_idle_watts` - Active and idle GPU power (under load on a real GPU, idle with the fake GPU)
- `kepler_node_gpu_info` - GPU device information

GPU tests require GPU metrics to be enabled, as in both e2e configurations.
Custom configurations passed through `-kepler.config` must also enable GPU metrics when running these tests.

### Process-Level Metrics

- `kepler_process_cpu_joules_total` - Per-process energy
- `kepler_process_cpu_watts` - Per-process power
- `kepler_process_cpu_seconds_total` - Per-process CPU time
- `kepler_process_gpu_watts` / `kepler_container_gpu_watts` - GPU power of a GPU load's process and container (real GPU only)

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

| Feature                    | Reason Not Tested in Bare-Metal E2E                                      | Where It's Tested                                            |
|----------------------------|--------------------------------------------------------------------------|--------------------------------------------------------------|
| GPU pod metrics            | Needs Kubernetes; GPU process and container metrics need a real GPU      | Unit tests in `internal/monitor/`; Phase 2: fake-gpu K8s e2e |
| VM metrics (`kepler_vm_*`) | Requires libvirt/hypervisor not available in test environment            | Unit tests in `internal/monitor/vm_test.go`                  |
| Redfish platform metrics   | Requires BMC hardware access not available in CI/dev environments        | Unit tests in `internal/platform/redfish/`                   |
| Metrics level filtering    | Low priority; config parsing is unit tested; e2e uses full metrics level | Unit tests in `config/`                                      |
| pprof debug endpoints      | Debug feature with low e2e value                                         | Unit tests in `internal/server/pprof_test.go`                |

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
  sudo ./bin/kepler-e2e.test -test.v -test.timeout=15m -gpu.fake
  ```

### Why Self-Hosted Runners?

Standard GitHub-hosted runners use virtualized environments without access to hardware power meters. E2E tests require:

1. Physical Intel CPU with RAPL support
2. Root access to read `/sys/class/powercap/`
3. Kernel support for powercap subsystem

### GPU E2E Workflow

**Workflow file**: `.github/workflows/gpu-e2e.yaml`

The GPU tests run against a real NVIDIA A10 on a CNCF-hosted GPU runner (`oracle-vm-gpu-a10-1`). These GPUs are shared by all CNCF projects, so the workflow does not run on every pull request:

- **On demand**: a maintainer runs it for a pull request that touches GPU code, from Actions > GPU e2e > Run workflow. Fill in the PR number and the commit you reviewed. The run stops before using a GPU if the PR is closed or has new commits since your review, so only reviewed code runs on the GPU runner, where the tests run with `sudo`.
- **Weekly**: every Monday it tests `main`. If that run fails, or does not finish because it hit the 30 minute limit or was cancelled (including by a newer run while it waited for the GPU, or by hand), it opens an issue named "Weekly GPU e2e run failed", or comments on the one already open, with what went wrong: the failing tests and their errors, or the end of the failed step's log when the tests did not get to run, along with the GPU, driver and CUDA version, the commit tested, and links to the run and its logs.

For a pull request, the `GPU e2e` status on its latest commit goes to pending as soon as the run starts, then to passed or failed, or to error if the GPU job did not finish, and links to the run. A run in which no GPU test passes counts as failed, since it did not test the GPU: for example when every GPU test skips because the runner has no usable GPU, or when no test matches the `TestGPU` filter. The run summary shows the GPU, each test's result, Kepler's GPU power next to `nvidia-smi` at idle and under load, and the GPU power over the run. The `gpu-e2e-logs` artifact keeps, for 7 days, the full test output with Kepler's logs, the `nvidia-smi` power samples, the run summary, and Kepler's GPU metrics (`kepler_node_gpu_*`, `kepler_process_gpu_*` and `kepler_container_gpu_*`) scraped every 3 seconds while the tests ran, so a failure can be looked into without another GPU run.

Only one GPU run happens at a time. If a run is already waiting for the GPU, a newer run replaces it. Runs for the same pull request also wait for each other from the start, so an older run never replaces the status of a newer one. The job that runs the code under test only gets read access; the PR status and the weekly issue are written by separate jobs that do not run that code.

The workflow's steps live in `hack/gpu-e2e/`, and every job takes them from the workflow's own commit, never from the code under test: `target.sh` checks the PR and the reviewed commit, `sample.sh` records `nvidia-smi` power and Kepler's GPU metrics, `summary.sh` writes the run summary, `status.sh` sets the PR status and `issue.sh` opens the weekly issue. `hack/gpu-e2e/test.sh` checks them without a GPU or the GitHub API: the run summary for passed, failed, skipped, interrupted and other runs against the expected Markdown in `hack/gpu-e2e/testdata/*.want`, and the other scripts against a fake `gh`. Run it after changing any of them, and `UPDATE=1 hack/gpu-e2e/test.sh` when a change to the summary is intended.

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

## Related Documentation

- [Architecture Overview](design/architecture/) - Understanding Kepler's design
- [Power Attribution Guide](power-attribution-guide.md) - How power is measured and attributed
- [Pre-commit Setup](pre-commit.md) - Code quality checks before committing
