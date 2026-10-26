# Kepler Installation Guide

This guide covers different methods to install and run Kepler (Kubernetes-based Efficient Power Level Exporter) for monitoring energy consumption metrics.

## Prerequisites

- **For Local Installation**: Use the Go version specified in the repository's [`go.mod`](../../go.mod) file. Root access is required to read RAPL energy counters.
- **For Kubernetes**: Kubernetes cluster (v1.20+) with kubectl configured
- **For Helm**: Helm 3.0+ installed

## Installation Methods

### 1. Helm Chart Installation (Recommended for Kubernetes)

#### Prerequisites for Helm

- Helm 3.0+
- Kubernetes cluster with kubectl configured

#### Install from OCI Registry (Recommended)

Install directly from the OCI registry (OCI registries cannot be added as traditional Helm repositories):

```bash
# Install specific version
helm install kepler oci://quay.io/sustainable_computing_io/charts/kepler \
  --version 0.11.1 \
  --namespace kepler \
  --create-namespace

# Install latest version (omit --version)
helm install kepler oci://quay.io/sustainable_computing_io/charts/kepler \
  --namespace kepler \
  --create-namespace
```

#### Install from Source (Development/Testing)

> **NOTE**: This method is intended for development and testing purposes.
> For production deployments, use the OCI registry method above.

```bash
# Clone the repository
git clone https://github.com/sustainable-computing-io/kepler.git
cd kepler

# Install Kepler using Helm
helm install kepler manifests/helm/kepler/ \
  --namespace kepler \
  --create-namespace \
  --set namespace.create=false
```

#### Customizing the Installation

Create a `values.yaml` file to customize the installation:

```yaml
# values.yaml
image:
  repository: quay.io/sustainable_computing_io/kepler
  tag: "v0.10.0"
  pullPolicy: IfNotPresent

daemonset:
  resources:
    limits:
      cpu: 100m
      memory: 400Mi
    requests:
      cpu: 100m
      memory: 200Mi

  tolerations:
    - operator: Exists

  nodeSelector:
    kubernetes.io/os: linux

# Enable ServiceMonitor for Prometheus
serviceMonitor:
  enabled: true
  interval: 30s
```

Install with custom values:

```bash
# From source
helm install kepler manifests/helm/kepler/ \
  --namespace kepler \
  --create-namespace \
  --set namespace.create=false \
  --values values.yaml

# From OCI registry
helm install kepler oci://quay.io/sustainable_computing_io/charts/kepler \
  --version 0.11.1 \
  --namespace kepler \
  --create-namespace \
  --values values.yaml
```

#### Enabling GPU Power Monitoring (NVIDIA GPU Operator)

To export GPU power metrics on clusters with the [NVIDIA GPU Operator](https://docs.nvidia.com/datacenter/cloud-native/gpu-operator/overview.html), enable the experimental GPU flag. The chart then adds an `nvidia-libs` init container that copies `libnvidia-ml.so*` from the host driver path into an `emptyDir`, and points `LD_LIBRARY_PATH` at it:

```bash
helm install kepler oci://quay.io/sustainable_computing_io/charts/kepler \
  --namespace kepler \
  --create-namespace \
  --set config.experimental.gpu.enabled=true
```

What this changes:

- Adds an `nvidia-libs` init container that copies `libnvidia-ml.so*` into a 200Mi `emptyDir` using `cp -P` (preserves the driver symlink chain; `cp -L` can exceed the size limit when leftover driver versions are present).
- Mounts that `emptyDir` read-only at `/usr/local/nvidia/lib64` and sets `LD_LIBRARY_PATH=/usr/local/nvidia/lib64`.
- Mounts the host driver directory (default `/run/nvidia/driver`, overridable via `daemonset.nvidia.driverPath`) with `DirectoryOrCreate` so non-GPU nodes still schedule.
- Runs the Kepler container privileged (and with `allowPrivilegeEscalation: true`), since the GPU device nodes reach the container only that way when the pod does not run under the NVIDIA runtime.

Defaults assume the standard GPU Operator layout. Override via `values.yaml` if needed:

```yaml
config:
  experimental:
    gpu:
      enabled: true

daemonset:
  nvidia:
    driverPath: /run/nvidia/driver
    nvmlInitImage:
      repository: busybox
      tag: 1.36.1
```

Note: `config.experimental.gpu.enabled` is intentionally coupled with the chart's `daemonset.nvidia.*` plumbing. Enabling the binary flag without the init path (or the reverse) is not a supported split. Leave `enabled: false` if you do not want GPU power monitoring.

For GPUs in MIG mode, Kepler also needs dcgm-exporter. See [MIG Power Attribution](configuration.md#mig-power-attribution) for its requirements and limits.

#### Helm Management Commands

```bash
# Check installation status
helm status kepler -n kepler

# List releases
helm list -n kepler

# Upgrade release from source
helm upgrade kepler manifests/helm/kepler/ -n kepler

# Upgrade release from OCI registry to specific version
helm upgrade kepler oci://quay.io/sustainable_computing_io/charts/kepler --version 0.11.2 -n kepler

# Upgrade to latest version from OCI registry
helm upgrade kepler oci://quay.io/sustainable_computing_io/charts/kepler -n kepler

# Uninstall
helm uninstall kepler -n kepler
```

### 2. Local Installation

#### Building from Source

```bash
# Clone the repository
git clone https://github.com/sustainable-computing-io/kepler.git
cd kepler

# Build Kepler
make build

# Run Kepler (requires sudo for hardware access)
sudo ./bin/kepler --config.file hack/config.yaml
```

#### Configuration

Kepler can be configured using YAML files or CLI flags. The default configuration is in `hack/config.yaml`:

```bash
# Run with custom configuration
sudo ./bin/kepler --config.file /path/to/your/config.yaml

# Run with CLI flags
sudo ./bin/kepler --log.level=debug --exporter.stdout
```

**Access Points:**

- Metrics: <http://localhost:28282/metrics>
- Liveness probe: <http://localhost:28282/probe/livez>
- Readiness probe: <http://localhost:28282/probe/readyz>

### 3. Docker Compose (Recommended for Development)

The Docker Compose setup provides a complete monitoring stack with Kepler, Prometheus, and Grafana:

```bash
cd compose/dev

# Start the complete stack
docker compose up --build -d

# View logs
docker compose logs -f kepler-dev

# Stop the stack
docker compose down --volumes
```

**Access Points:**

- Kepler Metrics: <http://localhost:28283/metrics>
- Prometheus: <http://localhost:29090>
- Grafana: <http://localhost:23000> (admin/admin)

### 4. Kubernetes with Kustomize

#### Quick Setup with Kind

```bash
# Create a local cluster with monitoring stack
make cluster-up

# Deploy Kepler
make deploy

# Clean up
make cluster-down
```

#### Manual Kubernetes Deployment

```bash
# Deploy using kustomize
kubectl kustomize manifests/k8s | \
  sed -e "s|<KEPLER_IMAGE>|quay.io/sustainable_computing_io/kepler:latest|g" | \
  kubectl apply --server-side --force-conflicts -f -

# Check deployment status
kubectl get pods -n kepler

# Access metrics (port-forward)
kubectl port-forward -n kepler svc/kepler 28282:28282
```

#### Minimal Kubernetes Deployment (No Prometheus Operator)

If you only need the Kepler exporter and do not have Prometheus Operator
installed, deploy the core resources directly and skip Prometheus-specific
manifests.

```bash
# Create namespace and core RBAC/config
kubectl apply -f manifests/k8s/namespace.yaml
kubectl apply -f manifests/k8s/rbac.yaml
kubectl apply -f manifests/k8s/configmap.yaml

# Deploy DaemonSet with your desired image
sed -e "s|<KEPLER_IMAGE>|quay.io/sustainable_computing_io/kepler:latest|g" \
  manifests/k8s/daemonset.yaml | \
  kubectl apply --server-side --force-conflicts -f -

# Expose Kepler metrics service
kubectl apply -f manifests/k8s/service.yaml
```

This minimal path intentionally skips:

- `manifests/k8s/servicemonitor.yaml`
- `manifests/k8s/prometheus-rbac.yaml`

#### Custom Image Deployment

```bash
# Build and push custom image
make image push IMG_BASE=your-registry.com/yourorg VERSION=v1.0.0

# Deploy with custom image
make deploy IMG_BASE=your-registry.com/yourorg VERSION=v1.0.0
```

## Verification

### Check Deployment Status

```bash
# Check pods
kubectl get pods -n kepler

# Check DaemonSet
kubectl get daemonset -n kepler

# Check services
kubectl get svc -n kepler

# View logs
kubectl logs -n kepler -l app.kubernetes.io/name=kepler
```

### Access Metrics

```bash
# Port forward to access metrics locally
kubectl port-forward -n kepler svc/kepler 28282:28282

# Test metrics endpoint
curl http://localhost:28282/metrics
```

### Verify Metrics Collection

Look for key metrics like:

- `kepler_node_cpu_watts`
- `kepler_container_cpu_watts`
- `kepler_process_cpu_watts`

## Configuration Options

### Helm Chart Values

Key configuration options in `values.yaml` are below. See the [chart README](../../manifests/helm/kepler/README.md) for all of them.

```yaml
# Image configuration
image:
  repository: quay.io/sustainable_computing_io/kepler
  tag: "latest"
  pullPolicy: IfNotPresent

# DaemonSet configuration
daemonset:
  hostPID: true
  securityContext:
    privileged: false
    allowPrivilegeEscalation: false
    readOnlyRootFilesystem: true
    capabilities:
      drop: [ALL]
      add: [SYS_PTRACE]
    seccompProfile:
      type: RuntimeDefault
    appArmorProfile:
      type: Unconfined
    seLinuxOptions:
      type: spc_t

  # Resource limits
  resources:
    limits:
      cpu: 100m
      memory: 400Mi
    requests:
      cpu: 100m
      memory: 200Mi

  # Node scheduling
  tolerations:
    - operator: Exists

  nodeSelector:
    kubernetes.io/os: linux

# Monitoring
serviceMonitor:
  enabled: true
  interval: 30s
```

### Environment-Specific Settings

- **Development**: Use the fake CPU meter when hardware sensors are unavailable
- **Production**: Ensure nodes expose supported RAPL or hwmon sensors
- **Cloud**: May need different privilege configurations

## Troubleshooting

### Common Issues

1. **Permission Denied**: The chart runs Kepler unprivileged with only `SYS_PTRACE`; if your cluster policy strips it, or you see `permission denied` reading `/host/proc`, set `daemonset.securityContext.privileged=true`
2. **Missing Series After Upgrade**: The unprivileged default changes one output. `container_name` is empty for containers outside Kubernetes (Docker, Podman, nerdctl) whose process does not run as root, because Kepler cannot read `/proc/<pid>/environ` of another uid without `CAP_DAC_READ_SEARCH`. Set `daemonset.securityContext.privileged=true` to get it back
3. **No Metrics**: Check whether nodes expose supported RAPL or hwmon sensors
4. **Pod Crashes**: Review logs for hardware access issues
5. **ServiceMonitor Not Found**: Ensure Prometheus Operator is installed,
   or use the minimal deployment section above

### Debug Commands

```bash
# Check pod logs
kubectl logs -n kepler -l app.kubernetes.io/name=kepler

# Describe pod for events
kubectl describe pod -n kepler -l app.kubernetes.io/name=kepler

# Check node hardware
kubectl exec -n kepler -it <pod-name> -- ls /sys/class/powercap/intel-rapl

# Test with fake meter (development)
helm upgrade kepler manifests/helm/kepler/ -n kepler \
  --set 'config.cpu.preferredMeters[0]=fake'
```

### Getting Help

- **Documentation**: <https://sustainable-computing.io/>
- **Issues**: <https://github.com/sustainable-computing-io/kepler/issues>
- **Discussions**: <https://github.com/sustainable-computing-io/kepler/discussions>
- **Slack**: [#kepler channel in CNCF Slack](https://cloud-native.slack.com/archives/C06HYDN4A01)

## Next Steps

After successful installation:

1. **Set up Prometheus**: Configure scraping of Kepler metrics
2. **Install Grafana**: Use pre-built dashboards for visualization
3. **Configure Alerts**: Set up energy consumption alerts
4. **Explore Metrics**: Learn about available energy metrics
5. **Optimize Workloads**: Use insights to improve energy efficiency
