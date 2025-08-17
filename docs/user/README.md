# Kepler User Documentation

Welcome to Kepler user documentation! This directory contains everything you need to deploy, configure, and monitor energy consumption with Kepler.

## 🗺️ Documentation Overview

| Guide | Purpose | Target Audience | Time Required |
|-------|---------|-----------------|---------------|
| **[Getting Started](getting-started.md)** | Quick Kubernetes deployment | New users, cluster operators | 5-10 minutes |
| **[Installation](installation.md)** | Production deployment | DevOps, SRE, platform teams | 30-60 minutes |
| **[Configuration](configuration.md)** | Customize Kepler settings | Advanced users, ops teams | As needed |
| **[Metrics](metrics.md)** | Understand available metrics | Monitoring teams, developers | Reference |
| **[Troubleshooting](troubleshooting.md)** | Diagnose and fix issues | All users | As needed |

## 🚀 Quick Start

**New to Kepler?** Start here:

1. **[Getting Started Guide](getting-started.md)** - Deploy Kepler to your Kubernetes cluster in under 10 minutes
2. **[Access your metrics](getting-started.md#access-metrics)** - Verify energy data collection
3. **Choose your next step** based on your needs:
   - Production deployment → [Installation Guide](installation.md)
   - Customize settings → [Configuration Guide](configuration.md)
   - Having issues? → [Troubleshooting Guide](troubleshooting.md)

**Want to try Kepler locally first?** See our [**🧑‍💻 Developer Getting Started Guide**](../developer/getting-started.md) for Docker Compose setup with pre-configured dashboards.

## 📋 Choose Your Path

### 🎯 I want to deploy Kepler to my cluster

**→ [Getting Started Guide](getting-started.md)**

- Quick Helm installation (5 minutes)
- Deploy to existing Kubernetes cluster
- Verify energy metrics collection
- Production-ready deployment path

### 🏗️ I need to deploy Kepler in production

**→ [Installation Guide](installation.md)**

- Helm installation (recommended)
- kubectl/kustomize deployment
- Enterprise integration (RBAC, network policies)
- Multi-cluster and high availability setup

### ⚙️ I need to customize Kepler configuration

**→ [Configuration Guide](configuration.md)**

- All configuration options explained
- Command-line flags vs config file
- Monitoring, logging, and export settings
- Kubernetes integration options
- Development features (fake CPU meter)

### 📊 I want to understand Kepler metrics

**→ [Metrics Reference](metrics.md)**

- Complete metrics catalog
- Node, container, process, VM, and pod level metrics
- Metric types and labels
- Power vs energy measurements
- Integration with Prometheus

### 🔍 I'm having problems with Kepler

**→ [Troubleshooting Guide](troubleshooting.md)**

- Quick health checks
- Docker Compose issues
- Kubernetes deployment problems
- Configuration and metrics issues
- Advanced debugging techniques

## 🛤️ Learning Progression

### Beginner Path

1. **[Getting Started](getting-started.md)** - Deploy to Kubernetes cluster
2. **[Understanding Metrics](metrics.md)** - Learn what you're measuring
3. **[Basic Configuration](configuration.md#configuration-methods)** - Simple customization

### Local Development Path

1. **[Developer Getting Started Guide](../developer/getting-started.md)** - Docker Compose with dashboards
2. **[Getting Started](getting-started.md)** - Deploy to cluster when ready
3. **[Configuration Guide](configuration.md)** - Customize for your needs

### Intermediate Path

1. **[Installation Guide](installation.md)** - Production deployment
2. **[Advanced Configuration](configuration.md#configuration-options-in-detail)** - Fine-tune settings
3. **[Troubleshooting](troubleshooting.md)** - Handle common issues

### Advanced Path

1. **[Enterprise Integration](installation.md#enterprise-integration)** - RBAC, security
2. **[Performance Tuning](configuration.md#monitor-configuration)** - Optimize for scale
3. **[Advanced Debugging](troubleshooting.md#advanced-debugging)** - Deep troubleshooting

## 🎯 Common Use Cases

### "I want to see energy monitoring in action"

**Solution:** [Developer Getting Started - Docker Compose](../developer/getting-started.md#docker-compose-development-setup)

- Complete monitoring stack with dashboards
- Local development environment
- 5-minute setup

### "I need Kepler in my Kubernetes cluster"

**Solution:** [Getting Started - Helm Installation](getting-started.md#quick-installation-with-helm)

- Quick cluster deployment (5 minutes)
- Then [Installation Guide](installation.md#helm-installation-recommended) for production config
- Integrates with existing monitoring

### "Power metrics are missing or incorrect"

**Solution:** [Troubleshooting - Metrics Issues](troubleshooting.md#metrics-and-monitoring-issues)

- Hardware support checks
- Fake CPU meter for testing
- Attribution troubleshooting

### "I want to customize how Kepler works"

**Solution:** [Configuration Guide](configuration.md)

- All configuration options
- Environment-specific settings
- Development vs production configs

### "Kepler isn't working as expected"

**Solution:** [Troubleshooting Guide](troubleshooting.md)

- Quick diagnostics
- Platform-specific issues
- Step-by-step problem solving

## 📚 Related Documentation

### Developer Resources

- **[Developer Documentation](../developer/)** - Contributing, development setup
- **[Architecture Guide](../developer/design/architecture/)** - How Kepler works internally
- **[API Documentation](../developer/)** - Technical implementation details

### Project Resources

- **[Main README](../../README.md)** - Project overview
- **[Contributing Guide](../../CONTRIBUTING.md)** - How to contribute
- **[Governance](../../GOVERNANCE.md)** - Project governance

## 🆘 Getting Help

### Self-Service Resources

1. **[Troubleshooting Guide](troubleshooting.md)** - Most common issues covered
2. **[Configuration Reference](configuration.md)** - All settings explained
3. **[Metrics Documentation](metrics.md)** - Understanding the data

### Community Support

- **🐛 GitHub Issues:** [Report bugs or request features](https://github.com/sustainable-computing-io/kepler/issues)
- **💬 GitHub Discussions:** [Ask questions and share experiences](https://github.com/sustainable-computing-io/kepler/discussions)
- **🗨️ CNCF Slack:** [Real-time community chat](https://cloud-native.slack.com/archives/C06HYDN4A01)

### Before Asking for Help

1. Check the [Troubleshooting Guide](troubleshooting.md) for your issue
2. Search [existing issues](https://github.com/sustainable-computing-io/kepler/issues)
3. Gather logs and configuration (see [troubleshooting checklist](troubleshooting.md#before-asking-for-help))

## 🔄 Documentation Updates

This documentation is actively maintained. If you find:

- **Outdated information** - Please [open an issue](https://github.com/sustainable-computing-io/kepler/issues/new)
- **Missing content** - Contributions welcome via [pull request](https://github.com/sustainable-computing-io/kepler/pulls)
- **Unclear instructions** - Let us know in [discussions](https://github.com/sustainable-computing-io/kepler/discussions)

## 📈 What's Next?

After mastering the user guides:

- **[Join the community](https://github.com/sustainable-computing-io/kepler/discussions)** - Share your experiences
- **[Contribute improvements](../../CONTRIBUTING.md)** - Help make Kepler better
- **[Try advanced features](../developer/)** - Explore cutting-edge capabilities

---

**Happy energy monitoring with Kepler!** ⚡
