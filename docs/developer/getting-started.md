# Developer Getting Started Guide

This guide covers setting up your local development environment for Kepler, including building from source, Docker Compose development stacks, and testing workflows.

> **For end users:** See the [User Getting Started Guide](../user/getting-started.md) for Kubernetes cluster deployment.

## Prerequisites

- **Go 1.21+** for building from source
- **Docker** for containerized builds and development
- **kubectl** and **Helm** for Kubernetes deployments
- **make** for using project Makefile targets

## Building from Source

### Local Development Build

```bash
# Clone the repository
git clone https://github.com/sustainable-computing-io/kepler.git
cd kepler

# Build Kepler binary
make build

# Run locally (requires sudo for hardware access)
sudo ./bin/kepler --config hack/config.yaml
```

### Development Configuration

Kepler can be configured using YAML files or CLI flags. For development, use the provided configuration:

```bash
# Run with custom configuration
sudo ./bin/kepler --config /path/to/your/config.yaml

# Run with CLI flags for debugging
sudo ./bin/kepler --log.level=debug --exporter.stdout

# Run with fake CPU meter for development (no hardware required)
sudo ./bin/kepler --config hack/config.yaml --fake-cpu-meter
```

**Development Access Points:**

- Metrics: <http://localhost:28282/metrics>
- Health: <http://localhost:28282/healthz>

### Optimized Build

```bash
# Build optimized binary for testing
make build PRODUCTION=1

# This creates an optimized kepler binary for performance testing
```

## Container Development

### Building Container Images

```bash
# Build development image
make image

# Build with custom tag and registry
make image IMG_BASE=your-registry.com/yourorg VERSION=v1.0.0-dev

# Push to registry
make push IMG_BASE=your-registry.com/yourorg VERSION=v1.0.0-dev
```

### Docker Compose Development Setup

The complete development stack with live code changes:

```bash
cd compose/dev

# Build and start development stack
docker compose up --build -d

# View logs
docker compose logs -f kepler-dev

# Rebuild after code changes
docker compose build kepler-dev
docker compose restart kepler-dev

# Stop the stack
docker compose down --volumes
```

**Development Stack Includes:**

- Kepler (built from local source)
- Prometheus (with dev scrape configs)
- Grafana (with development dashboards)

**Access Points:**

- Kepler Metrics: <http://localhost:28283/metrics>
- Prometheus: <http://localhost:29090>
- Grafana: <http://localhost:23000> (admin/admin)

## Kubernetes Development

### Local Cluster Setup

Create a complete development cluster with monitoring stack:

```bash
# Create Kind cluster with Prometheus and Grafana
make cluster-up

# Deploy development version of Kepler
make build deploy

# Check deployment
kubectl get pods -n kepler

# Clean up when done
make cluster-down
```

### Custom Image Deployment

Deploy with your custom-built image:

```bash
# Build and push custom image
make image push IMG_BASE=your-registry.com/yourorg VERSION=v1.0.0-dev

# Deploy with custom image
make deploy IMG_BASE=your-registry.com/yourorg VERSION=v1.0.0-dev
```

### Development Cluster Configuration

The `make cluster-up` command supports several environment variables:

```bash
# Customize cluster setup
CLUSTER_PROVIDER=kind \
GRAFANA_ENABLE=true \
PROMETHEUS_ENABLE=true \
KIND_WORKER_NODES=2 \
make cluster-up
```

## Advanced Development Workflows

### Testing Changes

```bash
# Run tests
make test

# Run tests with coverage
make test-coverage

# Run linting
make lint

# Run all checks
make ci
```

### Development Deployment Debugging

For general Kubernetes troubleshooting, see our [User Troubleshooting Guide](../user/troubleshooting.md).

**Development-specific debugging:**

```bash
# Debug development builds
kubectl logs -n kepler -l app.kubernetes.io/name=kepler --tail=100 | grep -E "(DEBUG|ERROR|build|version)"

# Check development image deployment
kubectl describe pod -n kepler -l app.kubernetes.io/name=kepler | grep -A 5 "Image:"

# Test development features
kubectl exec -n kepler -it <pod-name> -- curl localhost:28282/debug/pprof/
```

### Custom Configuration for Development

Create a development-specific values.yaml:

```yaml
# dev-values.yaml
image:
  repository: your-registry.com/yourorg/kepler
  tag: "dev"
  pullPolicy: Always  # Always pull for development

# Enable debug logging
env:
  KEPLER_LOG_LEVEL: "debug"
  KEPLER_FAKE_CPU_METER: "true"  # For development without RAPL

# Reduce resource requirements for development
resources:
  limits:
    cpu: 50m
    memory: 200Mi
  requests:
    cpu: 25m
    memory: 100Mi

# Enable all monitoring for development
serviceMonitor:
  enabled: true
  interval: 15s  # More frequent scraping for development
```

Deploy with development configuration:

```bash
helm install kepler-dev manifests/helm/kepler/ \
  --namespace kepler-dev \
  --create-namespace \
  --values dev-values.yaml
```

## Fake CPU Meter for Development

⚠️ **WARNING: FOR DEVELOPMENT AND TESTING ONLY - DO NOT USE IN PRODUCTION**

The fake CPU meter is essential for development when you don't have access to Intel RAPL hardware sensors. It generates synthetic power data that allows you to develop and test Kepler functionality without physical power measurement hardware.

### When to Use Fake CPU Meter in Development

- **CI/CD pipelines** - Automated testing in virtualized environments
- **Local development** - Working on VMs, containers, or non-Intel systems
- **Unit testing** - Testing power attribution algorithms
- **Demo environments** - Showing Kepler functionality without hardware
- **Algorithm development** - Testing power attribution logic

### Enabling Fake CPU Meter

#### Local Binary Development

```bash
# Method 1: Command line flag
sudo ./bin/kepler --config hack/config.yaml --fake-cpu-meter

# Method 2: Environment variable
export KEPLER_FAKE_CPU_METER=true
sudo ./bin/kepler --config hack/config.yaml

# Method 3: Configuration file
# Edit hack/config.yaml or create custom config
cat > dev-config.yaml << EOF
dev:
  fake-cpu-meter:
    enabled: true
    zones: [] # empty enables all default zones
EOF
sudo ./bin/kepler --config dev-config.yaml
```

#### Docker Compose Development

```bash
cd compose/dev

# Method 1: Environment variable in compose override
cat > docker-compose.override.yml << EOF
services:
  kepler-dev:
    environment:
      - KEPLER_FAKE_CPU_METER=true
EOF

# Method 2: Modify the config file directly
sed -i '/fake-cpu-meter:/,/zones:/c\  fake-cpu-meter:\n    enabled: true\n    zones: []' kepler-dev/etc/kepler/config.yaml

# Start development stack
docker compose up --build -d
```

#### Development Kubernetes Cluster

```bash
# Method 1: Enable during cluster setup (automatic in CI)
make cluster-up  # Fake meter enabled automatically in test environments

# Method 2: Modify configmap before deployment
sed -i '/fake-cpu-meter:/{n;s/enabled: false/enabled: true/}' manifests/k8s/configmap.yaml
make build deploy

# Method 3: Enable on running deployment
kubectl patch configmap kepler-config -n kepler --type merge \
  --patch '{"data":{"config.yaml":"$(kubectl get configmap kepler-config -n kepler -o jsonpath={.data.config\\.yaml} | sed \"s/enabled: false/enabled: true/\")"}}'
kubectl rollout restart daemonset/kepler -n kepler
```

#### Development Helm Charts

```bash
# Create development values file
cat > dev-values.yaml << EOF
# Enable fake CPU meter for development
env:
  KEPLER_FAKE_CPU_METER: "true"

# Development-friendly settings
image:
  pullPolicy: Always  # Always pull latest dev images
  repository: localhost:5001/kepler  # Local registry
  tag: dev

# Reduced resources for local testing
resources:
  limits:
    cpu: 100m
    memory: 200Mi
  requests:
    cpu: 50m
    memory: 100Mi

# Enable debug logging
config:
  log:
    level: debug
EOF

# Deploy with development settings
helm install kepler-dev manifests/helm/kepler/ \
  --namespace kepler-dev \
  --create-namespace \
  --values dev-values.yaml
```

### Fake CPU Meter Configuration Options

The fake CPU meter can be configured in `config.yaml`:

```yaml
dev:
  fake-cpu-meter:
    enabled: true
    zones: [] # Empty list enables all default zones
    # zones: ["package-0", "core", "uncore", "dram"] # Specific zones
```

**Available fake zones:**

- `package-0` - Simulates main CPU package power
- `core` - Simulates CPU core power consumption
- `uncore` - Simulates uncore (cache, memory controller) power
- `dram` - Simulates memory power consumption

### Testing with Fake CPU Meter

#### Unit Testing

```bash
# Run tests with fake CPU meter enabled
KEPLER_FAKE_CPU_METER=true make test

# Test specific components
KEPLER_FAKE_CPU_METER=true go test -v ./internal/device/...
```

#### Integration Testing

```bash
# Start development environment with fake meter
make cluster-up  # Automatically enables fake meter

# Verify metrics are generated
kubectl port-forward -n kepler svc/kepler 28282:28282 &
curl -s http://localhost:28282/metrics | grep -E "(kepler_node|kepler_container)_cpu_watts"
```

#### CI/CD Pipeline Integration

The fake CPU meter is automatically used in CI environments:

```bash
# See .github/k8s/action.yaml for reference
# The CI automatically enables fake meter:
sed -i '/fake-cpu-meter:/{n;s/enabled: false/enabled: true/}' manifests/k8s/configmap.yaml
```

### Debugging Fake CPU Meter

#### Verify Fake Meter is Active

```bash
# Check logs for fake meter initialization
kubectl logs -n kepler -l app.kubernetes.io/name=kepler | grep -i fake

# Expected log messages:
# "Initializing fake CPU power meter for development"
# "Fake CPU meter enabled with zones: [package-0, core, uncore, dram]"

# For local development
sudo ./bin/kepler --config hack/config.yaml --fake-cpu-meter --log.level=debug
```

#### Monitor Synthetic Data Generation

```bash
# Watch metrics being generated
watch -n 5 'curl -s http://localhost:28282/metrics | grep kepler_node_cpu_watts'

# Check attribution to containers
curl -s http://localhost:28282/metrics | grep kepler_container_cpu_watts
```

### Fake CPU Meter Implementation Notes

For developers working on the fake CPU meter:

#### Data Generation Algorithm

The fake CPU meter generates realistic power data by:

1. **Base power calculation** - Simulates idle power consumption
2. **CPU utilization scaling** - Increases power based on CPU usage
3. **Workload attribution** - Distributes power across active processes
4. **Time-based variations** - Adds realistic fluctuations over time

#### Code Structure

```bash
# Key files for fake CPU meter development:
internal/device/fake_cpu_power_meter.go      # Main implementation
internal/device/fake_cpu_power_meter_test.go # Unit tests
config/config.go                             # Configuration handling
```

#### Adding New Fake Zones

```go
// Example: Adding a new fake zone type
func (f *FakeCPUPowerMeter) getZoneNames() []string {
    return []string{
        "package-0",
        "core",
        "uncore",
        "dram",
        "gpu",  // New fake zone
    }
}
```

## Development Environment Settings

### Environment Variables for Development

```bash
# Enable debug mode
export KEPLER_LOG_LEVEL=debug

# Use fake CPU meter when RAPL unavailable
export KEPLER_FAKE_CPU_METER=true

# Custom configuration path
export KEPLER_CONFIG_PATH=/path/to/dev/config.yaml

# Export to stdout instead of Prometheus format
export KEPLER_EXPORTER_STDOUT=true
```

### Hardware Requirements for Development

**For full hardware testing:**

- Intel CPU with RAPL support
- Linux system with `/sys/class/powercap/intel-rapl` available
- Privileged access to `/proc` and `/sys`

**For development and testing:**

- Use `--fake-cpu-meter` flag or `KEPLER_FAKE_CPU_METER=true`
- Generates synthetic power data for development
- Works on any system (VMs, containers, non-Intel hardware)
- Perfect for development workflows and CI/CD
- See [Fake CPU Meter section](#fake-cpu-meter-for-development) for detailed setup

## Troubleshooting Development Issues

### Common Development Problems

#### Build Issues

```bash
# Clean and rebuild
make clean
make build

# Check Go version
go version  # Should be 1.21+

# Update dependencies
go mod tidy
go mod vendor  # if using vendor
```

#### Runtime Issues

```bash
# Check hardware access
sudo ls -la /sys/class/powercap/intel-rapl/

# Test with fake meter
sudo ./bin/kepler --fake-cpu-meter --log.level=debug

# Check permissions
sudo ./bin/kepler --config hack/config.yaml --log.level=debug
```

#### Container Issues

```bash
# Check container logs
docker compose logs kepler-dev

# Rebuild container
docker compose build --no-cache kepler-dev

# Check bind mounts
docker compose exec kepler-dev ls -la /host/proc /host/sys
```

#### Development Cluster Issues

For general Kubernetes troubleshooting, see our [User Troubleshooting Guide](../user/troubleshooting.md).

**Development cluster specific issues:**

```bash
# Debug make cluster-up issues
make cluster-down && make cluster-up

# Check development cluster status
kubectl get nodes -o wide
kubectl get pods --all-namespaces | grep -E "(kepler|prometheus|grafana)"

# Verify development image builds
docker images | grep kepler
```

## Contributing Workflow

### Pre-commit Setup

```bash
# Install pre-commit hooks
make pre-commit-install

# Run pre-commit on all files
make pre-commit-run-all
```

### Testing Your Changes

```bash
# Unit tests
make test

# Integration tests (requires cluster)
make test-integration

# E2E tests
make test-e2e
```

### Documentation Updates

When making changes that affect installation:

1. Update this developer getting started guide for development workflow changes
2. Update [User Getting Started Guide](../user/getting-started.md) for user-facing deployment changes
3. Update [User Configuration Guide](../user/configuration.md) for configuration option changes
4. Update [User Troubleshooting Guide](../user/troubleshooting.md) for user-facing issues

## Release Builds

### Creating Release Artifacts

```bash
# Build release binaries for multiple platforms
make build-release

# Create container images for release
make image-release VERSION=v0.10.1

# Generate release documentation
make docs-release
```

### Testing Release Candidates

```bash
# Deploy release candidate
make deploy IMG_BASE=quay.io/sustainable_computing_io/kepler VERSION=v0.10.1-rc1

# Run release validation tests
make test-release
```

## Next Steps

After setting up your development environment:

1. **Read the [Architecture Documentation](design/architecture/)** - Understand the system design
2. **Review [Power Attribution Guide](power-attribution-guide.md)** - Understand how power measurement works
3. **Set up [Pre-commit Hooks](pre-commit.md)** - Ensure code quality
4. **Check [Contributing Guidelines](../../CONTRIBUTING.md)** - Follow project conventions

## Getting Help

**Development-specific help:**

- **🏗️ Architecture Docs:** [docs/developer/design/architecture/](design/architecture/)
- **🤝 Contributing:** [CONTRIBUTING.md](../../CONTRIBUTING.md)

**User deployment and configuration help:**

- **📚 User Guides:** [docs/user/README.md](../user/README.md)
- **🔧 Configuration:** [User Configuration Guide](../user/configuration.md)
- **🔍 Troubleshooting:** [User Troubleshooting Guide](../user/troubleshooting.md)

**Community support:**

- **🐛 Issues:** [GitHub Issues](https://github.com/sustainable-computing-io/kepler/issues)
- **💬 Discussions:** [GitHub Discussions](https://github.com/sustainable-computing-io/kepler/discussions)
- **🗨️ Slack:** [#kepler in CNCF Slack](https://cloud-native.slack.com/archives/C06HYDN4A01)
