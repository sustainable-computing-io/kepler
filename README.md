# Kepler

[![GitHub license](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://github.com/sustainable-computing-io/kepler/blob/main/LICENSES) [![codecov](https://codecov.io/gh/sustainable-computing-io/kepler/branch/main/graph/badge.svg?token=K9BDX9M86E)](https://codecov.io/gh/sustainable-computing-io/kepler/tree/main) [![CI Status](https://github.com/sustainable-computing-io/kepler/actions/workflows/push.yaml/badge.svg?branch=main)](https://github.com/sustainable-computing-io/kepler/actions/workflows/push.yaml) [![Releases](https://img.shields.io/github/v/tag/sustainable-computing-io/kepler)](https://github.com/sustainable-computing-io/kepler/releases) [![zread](https://img.shields.io/badge/Ask_Zread-_.svg?style=flat&color=00b0aa&labelColor=000000&logo=data%3Aimage%2Fsvg%2Bxml%3Bbase64%2CPHN2ZyB3aWR0aD0iMTYiIGhlaWdodD0iMTYiIHZpZXdCb3g9IjAgMCAxNiAxNiIgZmlsbD0ibm9uZSIgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvc3ZnIj4KPHBhdGggZD0iTTQuOTYxNTYgMS42MDAxSDIuMjQxNTZDMS44ODgxIDEuNjAwMSAxLjYwMTU2IDEuODg2NjQgMS42MDE1NiAyLjI0MDFWNC45NjAxQzEuNjAxNTYgNS4zMTM1NiAxLjg4ODEgNS42MDAxIDIuMjQxNTYgNS42MDAxSDQuOTYxNTZDNS4zMTUwMiA1LjYwMDEgNS42MDE1NiA1LjMxMzU2IDUuNjAxNTYgNC45NjAxVjIuMjQwMUM1LjYwMTU2IDEuODg2NjQgNS4zMTUwMiAxLjYwMDEgNC45NjE1NiAxLjYwMDFaIiBmaWxsPSIjZmZmIi8%2BCjxwYXRoIGQ9Ik00Ljk2MTU2IDEwLjM5OTlIMi4yNDE1NkMxLjg4ODEgMTAuMzk5OSAxLjYwMTU2IDEwLjY4NjQgMS42MDE1NiAxMS4wMzk5VjEzLjc1OTlDMS42MDE1NiAxNC4xMTM0IDEuODg4MSAxNC4zOTk5IDIuMjQxNTYgMTQuMzk5OUg0Ljk2MTU2QzUuMzE1MDIgMTQuMzk5OSA1LjYwMTU2IDE0LjExMzQgNS42MDE1NiAxMy43NTk5VjExLjAzOTlDNS42MDE1NiAxMC42ODY0IDUuMzE1MDIgMTAuMzk5OSA0Ljk2MTU2IDEwLjM5OTlaIiBmaWxsPSIjZmZmIi8%2BCjxwYXRoIGQ9Ik0xMy43NTg0IDEuNjAwMUgxMS4wMzg0QzEwLjY4NSAxLjYwMDEgMTAuMzk4NCAxLjg4NjY0IDEwLjM5ODQgMi4yNDAxVjQuOTYwMUMxMC4zOTg0IDUuMzEzNTYgMTAuNjg1IDUuNjAwMSAxMS4wMzg0IDUuNjAwMUgxMy43NTg0QzE0LjExMTkgNS42MDAxIDE0LjM5ODQgNS4zMTM1NiAxNC4zOTg0IDQuOTYwMVYyLjI0MDFDMTQuMzk4NCAxLjg4NjY0IDE0LjExMTkgMS42MDAxIDEzLjc1ODQgMS42MDAxWiIgZmlsbD0iI2ZmZiIvPgo8cGF0aCBkPSJNNCAxMkwxMiA0TDQgMTJaIiBmaWxsPSIjZmZmIi8%2BCjxwYXRoIGQ9Ik00IDEyTDEyIDQiIHN0cm9rZT0iI2ZmZiIgc3Ryb2tlLXdpZHRoPSIxLjUiIHN0cm9rZS1saW5lY2FwPSJyb3VuZCIvPgo8L3N2Zz4K&logoColor=ffffff)](https://zread.ai/sustainable-computing-io/kepler) [![OpenSSF Best Practices](https://www.bestpractices.dev/projects/7391/badge)](https://www.bestpractices.dev/projects/7391) [![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/sustainable-computing-io/kepler/badge)](https://scorecard.dev/viewer/?uri=github.com/sustainable-computing-io/kepler)

Kepler (Kubernetes-based Efficient Power Level Exporter) is a Prometheus exporter that measures energy consumption metrics at the container, pod, and node level in Kubernetes clusters.

🌟 Star the [Kepler repository](https://github.com/sustainable-computing-io/kepler) to help others find it!

## 🚀 Major Rewrite: Kepler (0.10.0 and above)

Kepler 0.10.0 is a ground-up rewrite. See the [rewrite announcement](https://sustainable-computing.io/blog/kepler-0-10-rewrite/) for changes and migration from `0.9.x`.

## 🚀 Getting Started

> **📖 For comprehensive installation instructions, troubleshooting, and advanced deployment options, see our [Installation Guide](docs/user/installation.md)**

### ⚡ Quick Start (Kubernetes with Helm)

```sh
# 1. Install Kepler using Helm from OCI registry
helm install kepler oci://quay.io/sustainable_computing_io/charts/kepler \
  --namespace kepler \
  --create-namespace

# Wait for Kepler pods to be running
kubectl wait --for=condition=ready --timeout=120s pod -n kepler --all

# 2. Verify installation
kubectl get pods -n kepler

# 3. Access metrics (port-forward)
kubectl port-forward -n kepler svc/kepler 28282:28282

# Test metrics endpoint
curl http://localhost:28282/metrics | grep kepler_node_cpu_watts
```

> **📋 For Production Deployments:** Consider using the [Kepler Operator](https://github.com/sustainable-computing-io/kepler-operator#-getting-started) for advanced lifecycle management and operational capabilities.

**Next Steps:**

To ensure Kepler is working correctly and to visualize the metrics:

- **[Verify Metrics Collection](docs/user/installation.md#verify-metrics-collection)** - Verify power consumption metrics are being collected
- **[Configuration Options](docs/user/configuration.md)** - Customize Kepler deployment
- **[Helm Updates & Management](docs/user/helm-updates.md)** - Learn how to upgrade and manage Kepler with Helm

**Need Help?**

- [Installation Guide](docs/user/installation.md) - Detailed prerequisites, configuration options, and installation steps
- [Metrics Documentation](docs/user/metrics.md) - Available metrics and their descriptions

### 🔧 Other Installation Methods

Choose your preferred method:

```bash
# 💻 Local Development
make build && sudo ./bin/kepler

# ✨ Docker Compose (with Prometheus & Grafana)
cd compose/dev && docker compose up -d

# 🐳 Kubernetes with Kustomize
git clone https://github.com/sustainable-computing-io/kepler.git
cd kepler && git checkout v0.12.0 # pin to a release tag, see note below
kubectl kustomize manifests/k8s | \
  sed -e "s|<KEPLER_IMAGE>|quay.io/sustainable_computing_io/kepler:v0.12.0|g" | \
  kubectl apply --server-side --force-conflicts -f -
```

> **⚠️ Important:** Always check out a matching release tag (rather than `main`) before
> applying manifests from `manifests/k8s`, and set `<KEPLER_IMAGE>` to that same version.
> Manifests on `main` can reference features not yet present in the last published image
> (for example, the `/probe/livez` and `/probe/readyz` endpoints added for liveness/readiness
> probes) — mixing a `main` manifest with an older image can make the container fail its
> liveness probe and crash-loop. See the [Releases page](https://github.com/sustainable-computing-io/kepler/releases)
> for available tags.

## 📖 Documentation

### User Documentation

- **[Installation Guide](docs/user/installation.md)** - Detailed installation instructions for all deployment methods
- **[Configuration Guide](docs/user/configuration.md)** - Configuration options and examples
- **[Metrics Documentation](docs/user/metrics.md)** - Available metrics and their descriptions

### Developer Documentation

- **[Architecture Documentation](docs/developer/design/architecture/)** - Complete architectural documentation including design principles, system components, data flow, concurrency model, and deployment patterns
- **[Power Attribution Guide](docs/developer/power-attribution-guide.md)** - How Kepler measures and attributes power consumption
- **[Developer Documentation](docs/developer/)** - Contributing guidelines and development workflow

For more detailed documentation, please visit the [official Kepler documentation](https://sustainable-computing.io/kepler/).

## 🤝 Contributing

Contributions are welcome! Please feel free to submit a Pull Request. For more detailed information about contributing to this project, please refer to our [CONTRIBUTING.md](CONTRIBUTING.md) file.

### Roadmap

See the [Kepler Roadmap](https://github.com/orgs/sustainable-computing-io/projects/9) for planned and in-progress work.

### Code of Conduct

Kepler follows the [CNCF Code of Conduct](code-of-conduct.md).

### Gen AI policy

Our project adheres to the Linux Foundation's Generative AI Policy, which can be viewed at [https://www.linuxfoundation.org/legal/generative-ai](https://www.linuxfoundation.org/legal/generative-ai).

## ⭐ Star History

[![Star History Chart](https://star-history.dera.page/svg?repos=sustainable-computing-io/kepler&type=Date)](https://star-history.dera.page/#sustainable-computing-io/kepler&type=Date)

## 📝 License

This project is licensed under the Apache License 2.0 - see the [LICENSES](LICENSES) for details.
