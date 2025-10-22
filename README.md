# Kepler

[![GitHub license](https://img.shields.io/badge/License-Apache%202.0%20%7C%20GPL%202.0%20%7C%20BSD%202-blue.svg)](https://github.com/sustainable-computing-io/kepler/blob/main/LICENSES) [![codecov](https://codecov.io/gh/sustainable-computing-io/kepler/branch/main/graph/badge.svg?token=K9BDX9M86E)](https://codecov.io/gh/sustainable-computing-io/kepler/tree/main) [![CI Status](https://github.com/sustainable-computing-io/kepler/actions/workflows/push.yaml/badge.svg?branch=main)](https://github.com/sustainable-computing-io/kepler/actions/workflows/push.yaml) [![Releases](https://img.shields.io/github/v/tag/sustainable-computing-io/kepler)](https://github.com/sustainable-computing-io/kepler/releases) [![zread](https://img.shields.io/badge/Ask_Zread-_.svg?style=flat&color=00b0aa&labelColor=000000&logo=data%3Aimage%2Fsvg%2Bxml%3Bbase64%2CPHN2ZyB3aWR0aD0iMTYiIGhlaWdodD0iMTYiIHZpZXdCb3g9IjAgMCAxNiAxNiIgZmlsbD0ibm9uZSIgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvc3ZnIj4KPHBhdGggZD0iTTQuOTYxNTYgMS42MDAxSDIuMjQxNTZDMS44ODgxIDEuNjAwMSAxLjYwMTU2IDEuODg2NjQgMS42MDE1NiAyLjI0MDFWNC45NjAxQzEuNjAxNTYgNS4zMTM1NiAxLjg4ODEgNS42MDAxIDIuMjQxNTYgNS42MDAxSDQuOTYxNTZDNS4zMTUwMiA1LjYwMDEgNS42MDE1NiA1LjMxMzU2IDUuNjAxNTYgNC45NjAxVjIuMjQwMUM1LjYwMTU2IDEuODg2NjQgNS4zMTUwMiAxLjYwMDEgNC45NjE1NiAxLjYwMDFaIiBmaWxsPSIjZmZmIi8%2BCjxwYXRoIGQ9Ik00Ljk2MTU2IDEwLjM5OTlIMi4yNDE1NkMxLjg4ODEgMTAuMzk5OSAxLjYwMTU2IDEwLjY4NjQgMS42MDE1NiAxMS4wMzk5VjEzLjc1OTlDMS42MDE1NiAxNC4xMTM0IDEuODg4MSAxNC4zOTk5IDIuMjQxNTYgMTQuMzk5OUg0Ljk2MTU2QzUuMzE1MDIgMTQuMzk5OSA1LjYwMTU2IDE0LjExMzQgNS42MDE1NiAxMy43NTk5VjExLjAzOTlDNS42MDE1NiAxMC42ODY0IDUuMzE1MDIgMTAuMzk5OSA0Ljk2MTU2IDEwLjM5OTlaIiBmaWxsPSIjZmZmIi8%2BCjxwYXRoIGQ9Ik0xMy43NTg0IDEuNjAwMUgxMS4wMzg0QzEwLjY4NSAxLjYwMDEgMTAuMzk4NCAxLjg4NjY0IDEwLjM5ODQgMi4yNDAxVjQuOTYwMUMxMC4zOTg0IDUuMzEzNTYgMTAuNjg1IDUuNjAwMSAxMS4wMzg0IDUuNjAwMUgxMy43NTg0QzE0LjExMTkgNS42MDAxIDE0LjM5ODQgNS4zMTM1NiAxNC4zOTg0IDQuOTYwMVYyLjI0MDFDMTQuMzk4NCAxLjg4NjY0IDE0LjExMTkgMS42MDAxIDEzLjc1ODQgMS42MDAxWiIgZmlsbD0iI2ZmZiIvPgo8cGF0aCBkPSJNNCAxMkwxMiA0TDQgMTJaIiBmaWxsPSIjZmZmIi8%2BCjxwYXRoIGQ9Ik00IDEyTDEyIDQiIHN0cm9rZT0iI2ZmZiIgc3Ryb2tlLXdpZHRoPSIxLjUiIHN0cm9rZS1saW5lY2FwPSJyb3VuZCIvPgo8L3N2Zz4K&logoColor=ffffff)](https://zread.ai/sustainable-computing-io/kepler)
[![OpenSSF Best Practices](https://www.bestpractices.dev/projects/7391/badge)](https://www.bestpractices.dev/projects/7391)

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

> **📖 For comprehensive installation instructions, troubleshooting, and advanced deployment options, see our [Installation Guide](docs/user/installation.md)**

### ⚡ Quick Start

Choose your preferred method:

```bash
# 💻 Local Development
make build && sudo ./bin/kepler

# ✨ Docker Compose (with Prometheus & Grafana)
cd compose/dev && docker-compose up -d

# 🐳 Kubernetes
helm install kepler manifests/helm/kepler/ --namespace kepler --create-namespace
```

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

### Gen AI policy

Our project adheres to the Linux Foundation's Generative AI Policy, which can be viewed at [https://www.linuxfoundation.org/legal/generative-ai](https://www.linuxfoundation.org/legal/generative-ai).

## ⭐ Star History

[![Star History Chart](https://api.star-history.com/svg?repos=sustainable-computing-io/kepler&type=Date)](https://www.star-history.com/#sustainable-computing-io/kepler&Date)

## 📝 License

This project is licensed under the Apache License 2.0 - see the [LICENSES](LICENSES) for details.
