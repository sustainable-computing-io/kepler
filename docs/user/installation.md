# Kepler Installation Guide

This guide covers production-ready installation methods for deploying Kepler
to Kubernetes clusters. For local development and testing setups,
see our [**Developer Getting Started Guide**](../developer/getting-started.md).

## Prerequisites

- **Kubernetes cluster** (v1.20+) with kubectl configured
- **Admin access** for creating namespaces and RBAC resources
- **Helm 3.0+** (recommended) or kubectl with kustomize support

## Deployment Methods

### 1. Helm Installation (Recommended)

Helm provides the most flexible and user-friendly way to deploy Kepler to production Kubernetes clusters.

#### Install from Source

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

#### Install from Release (Future)

```bash
# Add Kepler Helm repository (once published)
helm repo add kepler https://sustainable-computing-io.github.io/kepler

# Update repository
helm repo update

# Install Kepler
helm install kepler kepler/kepler \
  --namespace kepler \
  --create-namespace
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
helm install kepler manifests/helm/kepler/ \
  --namespace kepler \
  --create-namespace \
  --set namespace.create=false \
  --values values.yaml
```

#### Helm Management Commands

```bash
# Check installation status
helm status kepler -n kepler

# List releases
helm list -n kepler

# Upgrade release
helm upgrade kepler manifests/helm/kepler/ -n kepler

# Uninstall
helm uninstall kepler -n kepler
```

### 2. kubectl/Kustomize Deployment

For environments requiring manual control or GitOps integration:

```bash
# Clone the repository for manifest access
git clone https://github.com/sustainable-computing-io/kepler.git
cd kepler

# Deploy using kustomize
kubectl kustomize manifests/k8s | \
  sed -e "s|<KEPLER_IMAGE>|quay.io/sustainable_computing_io/kepler:latest|g" | \
  kubectl apply --server-side --force-conflicts -f -

# Check deployment status
kubectl get pods -n kepler

# Access metrics (port-forward)
kubectl port-forward -n kepler svc/kepler 28282:28282
```

#### Custom Kustomization

For advanced users requiring specific configurations:

```yaml
# kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

namespace: kepler

resources:
- https://github.com/sustainable-computing-io/kepler/manifests/k8s

patchesStrategicMerge:
- resource-limits.yaml

images:
- name: quay.io/sustainable_computing_io/kepler
  newTag: v0.10.0
```

Deploy with custom kustomization:

```bash
kubectl apply -k .
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

### Production Considerations

- **Hardware Requirements**: Intel RAPL support (most Intel processors since 2011)
- **Security Context**: Kepler requires privileged access for hardware monitoring
- **Resource Planning**: Minimum 100m CPU, 200Mi memory per node
- **Monitoring Integration**: Configure ServiceMonitor for Prometheus scraping
- **High Availability**: Deploy across multiple nodes with appropriate tolerations

## Local Development Setup

🧑‍💻 **Want to try Kepler locally first?**

This guide focuses on production Kubernetes deployments. For local development, testing, and learning setups including Docker Compose with dashboards, see our comprehensive [**Developer Getting Started Guide**](../developer/getting-started.md).

The developer guide includes:

- **Docker Compose** setup with Prometheus & Grafana dashboards
- **make cluster-up** for local Kubernetes development
- **Building from source** and development workflows
- **Fake CPU meter** configuration for systems without RAPL support

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

```

### Getting Help

- **Documentation**: <https://sustainable-computing.io/kepler/>
- **Issues**: <https://github.com/sustainable-computing-io/kepler/issues>
- **Discussions**: <https://github.com/sustainable-computing-io/kepler/discussions>
- **Slack**: [#kepler channel in CNCF Slack](https://cloud-native.slack.com/archives/C06HYDN4A01)

## Next Steps

After successful installation:

1. **📊 Set up monitoring** - Configure Prometheus scraping and Grafana dashboards
2. **🔧 Customize configuration** - Review [Configuration Guide](configuration.md) for environment-specific settings
3. **📈 Analyze metrics** - Learn about available metrics in our [Metrics Reference](metrics.md)
4. **🚨 Plan troubleshooting** - Familiarize yourself with our [Troubleshooting Guide](troubleshooting.md)
