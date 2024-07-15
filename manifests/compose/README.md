# Kepler Docker Compose Configuration

This folder contains multiple Docker Compose configurations for setting up
Kepler in various environments. The directory structure is organized to
facilitate different configurations and components including Prometheus
and Grafana

## Structure

```bash
📁 .
├── 🐳 compose.yaml
├── 📁 default
│  ├── 📁 kepler
│  │  └── 📁 etc
│  │     └── 📁 kepler
│  │        └── 🛠️ kepler.config
│  └── 📁 prometheus
│     ├── 📁 rules
│     └── 📁 scrape-configs
├── 📁 dev
│  ├── 🐳 compose.yaml
│  ├── 📁 grafana
│  │  └── 📁 dashboards
│  │     └── 📁 dev
│  ├── 📁 kepler
│  │  └── 📁 etc
│  │     └── 📁 kepler
│  │        └── 🛠️ kepler.config
│  └── 📁 prometheus
│     └── 📁 scrape-configs
├── 📁 mock-acpi
│  ├── 🐳 compose.yaml
│  ├── 📁 grafana
│  │  └── 📁 dashboards
│  │     ├── 📁 intel-pcm
│  │     └── 📁 mock-acpi
│  ├── 📁 intel-pcm
│  ├── 📁 kepler
│  │  └── 📁 etc
│  │     └── 📁 kepler
│  │        └── 🛠️ kepler.config
│  ├── 📁 mock-acpi
│  ├── 📁 mock-acpi-config
│  ├── 📁 prometheus
│  │  └── 📁 scrape-configs
│  └── 📁 turbostat
├── 📁 monitoring
│  ├── 🐳 compose.yaml
│  ├── 📁 grafana
│  └── 📁 prometheus
│     └── 📁 rules
└── 📁 validation
   ├── 📁 metal
   │  ├── 🐳 compose.yaml
   │  ├── 📁 grafana
   │  │  └── 📁 dashboards
   │  │     ├── 📁 scaphandre
   │  │     └── 📁 validation
   │  ├── 📁 kepler
   │  │  └── 📁 etc
   │  │     └── 📁 kepler
   │  │        └── 🛠️ kepler.config
   │  └── 📁 prometheus
   │     └── 📁 scrape-configs
   ├── 📄 README.md
   └── 📁 vm
      ├── 🐳 compose.yaml
      └── 📁 kepler
         └── 📁 etc
            └── 📁 kepler
               └── 🛠️ kepler.config
```

## Getting Started

### Prerequisites

Ensure you have the following installed:

- Docker
- Docker Compose

### Setup

- Choose the environment:

  Depending upon your needs, navigate to the appropriate directory for the
  environment you wish to set up:

  - `dev` → Builds and deploy Kepler dev and latest along with Grafana dashboard
     and Prometheus
  - `mock-acpi` → Builds and deploys Kepler with mock ACPI power data along
     with Grafana dashboard and Prometheus. Includes scripts and
     configurations to simulate ACPI power consumption
  - `validation/metal` → Builds and deploy Kepler on Baremetal including
     Scaphandre, Grafana and Prometheus.
     **This environment is intended **solely for model validation**
  - `validation/vm` → Deploys Kepler on Virtual Machines.
     **This environment is intended solely for model validation**

- Modify Configurations (If needed):

  - Each environment may have specific configuration files that you can
    customize. For instance `kepler.config` files available under the
    `kepler/etc/kepler/` directory.
  - Update `override.yaml` if you need to override any default configurations.

- Run Docker Compose:

Navigate to the directory containing `compose.yaml` and run:

```bash
docker compose up -d
```

This command will start all the necessary services in detached mode

To stop the services, run:

```bash
docker compose down --volumes
```
