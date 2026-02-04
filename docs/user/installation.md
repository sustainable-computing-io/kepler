# Kepler Installation Guide

This guide covers different methods to install and run Kepler (Kubernetes-based Efficient Power Level Exporter) for monitoring energy consumption metrics.

## Prerequisites

- **For Local Installation**: Go 1.24.0+ and sudo access for hardware sensor access
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

### 3. Docker Compose (Recommended for Development)

The Docker Compose setup provides a complete monitoring stack with Kepler, Prometheus, and Grafana:

#### Docker Compose

```bash
cd compose/dev

# Start the complete stack
docker compose up --build -d

# View logs
docker compose logs -f kepler-dev

# Stop the stack
docker compose down --volumes
```

#### Podman Compose

For Podman users (especially on macOS/ARM), use the podman-compatible version:

```bash
cd compose/dev

# Start the complete stack
podman-compose -f compose-podman.yaml up --build -d

# View logs
podman-compose -f compose-podman.yaml logs -f kepler-dev

# Stop the stack
podman-compose -f compose-podman.yaml down --volumes
```

> **Note for ARM/Apple Silicon users**: The Scaphandre service is not available for ARM architecture. See [compose/dev/README-podman.md](../../compose/dev/README-podman.md) for platform-specific notes and workarounds.

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

Key configuration options in `values.yaml`:

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
    privileged: true

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
  scrapeTimeout: 10s
```

### Environment-Specific Settings

- **Development**: Use fake CPU meter when RAPL unavailable
- **Production**: Ensure nodes have Intel RAPL support
- **Cloud**: May need different privilege configurations

## Troubleshooting

### Common Issues

1. **Permission Denied**: Ensure privileged security context is enabled
2. **No Metrics**: Check if nodes support Intel RAPL sensors
3. **Pod Crashes**: Review logs for hardware access issues
4. **ServiceMonitor Not Found**: Ensure Prometheus Operator is installed

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
  --set env.KEPLER_FAKE_CPU_METER=true
```

### Getting Help

- **Documentation**: <https://sustainable-computing.io/kepler/>
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
