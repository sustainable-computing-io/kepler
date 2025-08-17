# Getting Started with Kepler

Get up and running with Kepler in your Kubernetes cluster in minutes! This
guide walks you through deploying Kepler for energy consumption monitoring.

## What is Kepler?

Kepler (Kubernetes-based Efficient Power Level Exporter) measures energy
consumption at the container, pod, and node level. By the end of this guide,
you'll have Kepler running in your Kubernetes cluster and collecting energy metrics.

## What You'll Accomplish

- ✅ Deploy Kepler to your Kubernetes cluster using Helm
- ✅ Verify Kepler is collecting energy consumption metrics
- ✅ Access metrics via port-forward or integrate with your monitoring stack
- ✅ Understand next steps for dashboards and production configuration

## Prerequisites

- **Kubernetes cluster** (v1.20+) with kubectl access
- **Helm 3.0+** installed
- **Admin permissions** for cluster installation
- **Intel RAPL support** on cluster nodes (or see [fake CPU meter](#running-without-hardware-support) for testing)

## Quick Installation with Helm

### Step 1: Clone Repository

```bash
# Clone the repository
git clone https://github.com/sustainable-computing-io/kepler.git
cd kepler
```

### Step 2: Install Kepler

```bash
# Install Kepler with Helm
helm install kepler manifests/helm/kepler/ \
  --namespace kepler \
  --create-namespace
```

This deploys Kepler as a DaemonSet to all nodes in your cluster.

### Step 3: Verify Installation

```bash
# Check Kepler pods are running
kubectl get pods -n kepler

# Should show kepler-* pods in Running state on each node
kubectl get daemonset -n kepler

# Check for any issues
kubectl describe pods -n kepler
```

### Step 4: Access Metrics

```bash
# Port forward to access Kepler metrics
kubectl port-forward -n kepler svc/kepler 28282:28282 &

# View available metrics
curl http://localhost:28282/metrics | grep kepler_node_cpu_watts

# Check that metrics are being collected
curl -s http://localhost:28282/metrics | grep -c "kepler_"
```

You should see metrics like:

- `kepler_node_cpu_watts` - CPU power consumption at node level
- `kepler_container_cpu_watts` - Per-container power usage
- `kepler_pod_cpu_watts` - Per-pod power usage

---

## Understanding Your Metrics

Once Kepler is running, you'll see these key metric types:

### 🔋 Power Metrics (Watts)

- **kepler_node_cpu_watts** - Instantaneous CPU power consumption
- **kepler_container_cpu_watts** - Power attributed to containers
- **kepler_pod_cpu_watts** - Power attributed to pods

### ⚡ Energy Metrics (Joules)

- **kepler_node_cpu_joules_total** - Cumulative energy consumed
- **kepler_container_cpu_joules_total** - Energy per container over time

### 💡 Key Concepts

- **Watts (W)** - Instantaneous power consumption (like speedometer)
- **Joules (J)** - Total energy consumed over time (like odometer)
- **Attribution** - How Kepler estimates which workload used energy

---

## Running Without Hardware Support

⚠️ **WARNING: For Development/Testing/Experimental Purposes Only**

If you're running on a cluster without Intel RAPL support (VMs, non-Intel hardware,
cloud instances), you can enable the fake CPU meter to generate synthetic
power data for experimentation.

**Important:** This feature is experimental and generates simulated data only

- never use for production monitoring or real optimization decisions.

For complete fake CPU meter setup instructions, see
the **[Fake CPU Meter Configuration section](configuration.md#fake-cpu-meter-configuration)** in our Configuration Guide.

---

## Local Development Setup

🧑‍💻 **Want to run Kepler locally for development?**

This guide focuses on deploying Kepler to Kubernetes clusters. For local
development with Docker Compose, complete monitoring stacks, and development
workflows, see our comprehensive **[Developer Getting Started Guide](../developer/getting-started.md)**.

The developer guide includes:

- **Docker Compose setup** with Prometheus & Grafana dashboards
- **make cluster-up** for local Kubernetes development
- **Building from source** and development workflows
- **Local testing** with fake CPU meter

---

## Next Steps

🎉 **Congratulations!** You now have Kepler running and collecting energy metrics.

### Immediate Next Steps

1. **📊 Set up dashboards** - Integrate with Grafana to visualize your energy data
2. **📈 Configure monitoring** - Connect to your existing Prometheus setup
3. **🔧 Verify metrics** - Ensure data collection is working properly

### Ready for More?

1. **🏗️ Production Deployment** - Ready for production? See our [Installation Guide](installation.md) for advanced Helm configurations, enterprise integration, and production best practices
2. **⚙️ Advanced Configuration** - Want to customize Kepler? Check the [Configuration Guide](configuration.md) for all configuration options
3. **📊 Metrics Deep Dive** - Understand all available metrics in the [Metrics Reference](metrics.md)
4. **🔍 Having Problems?** - Check our [Troubleshooting Guide](troubleshooting.md) for common issues and solutions

### Join the Community

- **🐛 Issues:** [GitHub Issues](https://github.com/sustainable-computing-io/kepler/issues)
- **💬 Discussions:** [GitHub Discussions](https://github.com/sustainable-computing-io/kepler/discussions)
- **🗨️ Slack:** [#kepler in CNCF Slack](https://cloud-native.slack.com/archives/C06HYDN4A01)

### Want to Contribute?

- **🔧 Developer docs:** [docs/developer/](../developer/)
- **📋 Contributing guide:** [CONTRIBUTING.md](../../CONTRIBUTING.md)

---

⚡ Happy energy monitoring with Kepler! ⚡
