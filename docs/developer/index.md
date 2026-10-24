# Developer Documentation

This section contains documentation for developers working on Kepler.

## 🏗️ Architecture Documentation

**Start here to understand Kepler's design and implementation:**

- **[Architecture Overview](design/architecture/)** - Complete architectural documentation
  - [Design Principles](design/architecture/principles.md) - The fundamental principles that drive all architectural decisions
  - [System Components](design/architecture/components.md) - Deep dive into each architectural layer and component
  - [Data Flow & Attribution](design/architecture/data-flow.md) - Power attribution algorithm and data flow patterns
  - [Concurrency & Thread Safety](design/architecture/concurrency.md) - Thread safety guarantees and concurrent processing
  - [Interfaces & Contracts](design/architecture/interfaces.md) - Key interfaces, service contracts, and API boundaries
  - [Configuration System](design/architecture/configuration.md) - Hierarchical configuration and option management

## Power Attribution

- [Power Attribution Guide](power-attribution-guide.md) - Comprehensive guide on how Kepler measures and attributes power consumption to processes, containers, VMs, and pods

## Enhancement Proposals

- [Enhancement Proposals](proposal/) - Technical proposals for major features and changes to Kepler

## Development Workflow

- [Pre-commit Setup](pre-commit.md) - Setting up pre-commit hooks for code quality
- [E2E Testing Guide](e2e-testing.md) - Running and writing end-to-end tests

### Quick Developer Setup

Follow the installation guide for [local builds](../user/installation.md#2-local-installation)
or the [Docker Compose development stack](../user/installation.md#3-docker-compose-recommended-for-development).
See the [contributing guide](../../CONTRIBUTING.md) for prerequisites
and testing, and install the [pre-commit hooks](pre-commit.md) before submitting
changes.

### Related Projects

- [Kepler Operator](https://github.com/sustainable-computing-io/kepler-operator)
- [Kepler Model Server](https://github.com/sustainable-computing-io/kepler-model-server)
  (see [compatibility](power-attribution-guide.md#trained-model-server-compatibility))

## Release Management

- [Release Process](release.md) - Kepler release process and lifecycle

## Contributing

Review the [contributing guide](../../CONTRIBUTING.md) and
[code of conduct](../../code-of-conduct.md) before contributing.

## Developer Support

- [GitHub Issues](https://github.com/sustainable-computing-io/kepler/issues)
- [GitHub Discussions](https://github.com/sustainable-computing-io/kepler/discussions)
- [CNCF Slack #kepler](https://cloud-native.slack.com/archives/C06HYDN4A01)
- Community session recordings on
  [YouTube](https://youtube.com/playlist?list=PL8-Q56LqwwAjox1MSt8VzviKO-PBD4zue) and
  [Bilibili](https://space.bilibili.com/276546/lists/2476604)
