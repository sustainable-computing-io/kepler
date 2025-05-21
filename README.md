# Kepler

[![GitHub license](https://img.shields.io/github/license/sustainable-computing-io/kepler)](https://github.com/sustainable-computing-io/kepler/blob/reboot/LICENSES) [![codecov](https://codecov.io/gh/sustainable-computing-io/kepler/branch/reboot/graph/badge.svg?token=K9BDX9M86E)](https://codecov.io/gh/sustainable-computing-io/kepler/tree/reboot) [![CI Status](https://github.com/sustainable-computing-io/kepler/actions/workflows/push.yaml/badge.svg?branch=reboot)](https://github.com/sustainable-computing-io/kepler/actions/workflows/push.yaml) [![Releases](https://img.shields.io/github/v/tag/sustainable-computing-io/kepler)](https://github.com/sustainable-computing-io/kepler/releases)

Kepler (Kubernetes-based Efficient Power Level Exporter) is a Prometheus exporter that measures energy consumption metrics at the container, pod, and node level in Kubernetes clusters.

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

Deploy Kepler to your Kubernetes cluster:

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

For more detailed documentation, please visit the [official Kepler documentation](https://sustainable-computing.io/kepler/).

## 🤝 Contributing

Contributions are welcome! Please feel free to submit a Pull Request. For more detailed information about contributing to this project, please refer to our [CONTRIBUTING.md](CONTRIBUTING.md) file.

## 📝 License

This project is licensed under the Apache License 2.0 - see the [LICENSES](LICENSES) for details.
