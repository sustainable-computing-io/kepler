# Kepler

[![GitHub license](https://img.shields.io/github/license/sustainable-computing-io/kepler)](https://github.com/sustainable-computing-io/kepler/blob/main/LICENSES) [![codecov](https://codecov.io/gh/sustainable-computing-io/kepler/branch/main/graph/badge.svg?token=K9BDX9M86E)](https://codecov.io/gh/sustainable-computing-io/kepler/tree/main) [![CI Status](https://github.com/sustainable-computing-io/kepler/actions/workflows/push.yaml/badge.svg?branch=main)](https://github.com/sustainable-computing-io/kepler/actions/workflows/push.yaml) [![Releases](https://img.shields.io/github/v/tag/sustainable-computing-io/kepler)](https://github.com/sustainable-computing-io/kepler/releases)

Kepler (Kubernetes-based Efficient Power Level Exporter) is a Prometheus exporter that measures energy consumption metrics at the container, pod, and node level in Kubernetes clusters.

## 🚀 Major Rewrite: Kepler (0.10.0 and above)

**Important Notice:** Starting with version 0.10.0, Kepler has undergone a complete ground-up rewrite.
This represents a significant architectural improvement while maintaining the core mission of
accurate energy consumption monitoring for cloud-native workloads.

> 📢 **Read the full announcement:** [CNCF Slack Announcement](https://cloud-native.slack.com/archives/C05QK3KN3HT/p1752049660866519)

### ✨ What's New in the Rewrite

**Enhanced Performance & Accuracy:**

- Dynamic detection of Nodes' RAPL zones - no more hardcoded RAPL zones
- More accurate power attribution based on active CPU usage (no more idle/dynamic for workloads)
- Improved VM, Container, and Pod detection with more meaningful label values
- Significantly reduced resource usage compared to old Kepler

**Reduced Security Requirements:**

- Requires only readonly access to host `/proc` and `/sys`
- No more `CAP_SYSADMIN` or `CAP_BPF` capabilities required
- Much fewer privileges than previous versions

**Modern Architecture:**

- Service-oriented design with clean separation of concerns
- Thread-safe operations throughout the codebase
- Graceful shutdown handling with proper resource cleanup
- Comprehensive error handling with structured logging

**Current Limitations:**

- Only supports Baremetal (platform power support in roadmap)
- Supports only RAPL/powercap framework
- No GPU power support yet

### 📚 Migration & Legacy Support

**For New Users:** Use the current version (0.10.0+) for the best experience and latest features.

**For Existing Users:** If you need to continue using the old version:

- Pin your deployment to version `0.9.0` (final legacy release)
- Access the old codebase in the [archived branch](https://github.com/sustainable-computing-io/kepler/tree/archived)
- **Important:** The legacy version (0.9.x and earlier) is now frozen - no bug fixes or feature requests will be accepted for the old version

**Migration Note:** Please review the new configuration format and deployment methods below when upgrading to 0.10.0+.

## 🚀 Getting Started

There are two main ways to run Kepler:

### 1️⃣ Running Kepler Locally

To run Kepler on your local machine:

```bash
# Build Kepler
make build

# Run Kepler
sudo ./bin/kepler
```

**Configuration:** Kepler can be configured using the `hack/config.yaml` file. You can customize settings like log level, filesystem paths, and more:

```bash
# Run Kepler with a custom configuration file
sudo ./bin/kepler --config hack/config.yaml
```

**Note:** Running Kepler locally requires you to:

- Set up Prometheus and Grafana separately for metrics collection and visualization
- Configure Prometheus to scrape metrics from Kepler's endpoint

**📋 Access the Services:**

- **Kepler Metrics**: <http://localhost:28282/metrics>

### 2️⃣ Running with Docker Compose ✨

The Docker Compose method provides a complete environment with Kepler, Prometheus, and Grafana configured and ready to use:

```bash
cd compose/dev

# Start the Docker Compose environment
docker-compose up -d
```

**With Docker Compose:**

- 🔍 Prometheus is automatically deployed and configured
- 📊 Grafana is automatically deployed with pre-configured dashboards
- 🔄 All services are connected and ready to use
- 🛠️ No additional setup required

**📋 Access the Services:**

- **Kepler Metrics**: <http://localhost:28283/metrics>
- **Prometheus**: <http://localhost:29090>
- **Grafana**: <http://localhost:23000> (default credentials: admin/admin)

### 3️⃣ Running on Kubernetes 🐳

Deploy Kepler to your Kubernetes cluster using Helm or Kustomize:

#### Using Helm Chart (Recommended)

```bash
# Install using Helm
helm install kepler manifests/helm/kepler/ \
  --namespace kepler \
  --create-namespace \
  --set namespace.create=false
```

#### Using Kustomize

```bash
# Set up a local development cluster with Kind
make cluster-up

# Deploy Kepler to the cluster
make deploy
```

**Custom Image Deployment:**

You can build, push, and deploy Kepler using your own image:

```bash
# Build and push image to your registry
make image push IMG_BASE=<your registry> VERSION=<your version>

# Deploy Kepler using the custom image
make deploy IMG_BASE=<your registry> VERSION=<your version>
```

## 📖 Documentation

- **[Installation Guide](docs/user/installation.md)** - Detailed installation instructions for all deployment methods
- **[Configuration Guide](docs/configuration/configuration.md)** - Configuration options and examples
- **[Metrics Documentation](docs/metrics/metrics.md)** - Available metrics and their descriptions

For more detailed documentation, please visit the [official Kepler documentation](https://sustainable-computing.io/kepler/).

## 🤝 Contributing

Contributions are welcome! Please feel free to submit a Pull Request. For more detailed information about contributing to this project, please refer to our [CONTRIBUTING.md](CONTRIBUTING.md) file.

## 📝 License

This project is licensed under the Apache License 2.0 - see the [LICENSES](LICENSES) for details.
