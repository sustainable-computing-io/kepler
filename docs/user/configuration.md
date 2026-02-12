# ⚙️ Kepler Configuration Guide

Kepler supports configuration through both command-line flags and a configuration file. This guide outlines all available configuration options for configuring Kepler.

## 🛠️ Configuration Methods

Kepler supports two primary methods for configuration:

1. **Command-line flags**: For quick adjustments and one-time settings
2. **Configuration file**: For persistent and comprehensive configuration

> ⚡ **Tip:** Command-line flags take precedence over configuration file settings when both are specified.

## 🖥️ Command-line Flags

You can configure Kepler by passing flags when starting the service. The following flags are available:

| Flag                                          | Description                                                             | Default                         | Values                                                             |
|-----------------------------------------------|-------------------------------------------------------------------------|---------------------------------|--------------------------------------------------------------------|
| `--config.file`                               | Path to YAML configuration file                                         |                                 | Any valid file path                                                |
| `--log.level`                                 | Logging level                                                           | `info`                          | `debug`, `info`, `warn`, `error`                                   |
| `--log.format`                                | Output format for logs                                                  | `text`                          | `text`, `json`                                                     |
| `--host.sysfs`                                | Path to sysfs filesystem                                                | `/sys`                          | Any valid directory path                                           |
| `--host.procfs`                               | Path to procfs filesystem                                               | `/proc`                         | Any valid directory path                                           |
| `--monitor.interval`                          | Monitor refresh interval                                                | `5s`                            | Any valid duration                                                 |
| `--monitor.max-terminated`                    | Maximum number of terminated workloads to keep in memory until exported | `500`                           | Negative number indicates `unlimited` and `0` disables the feature |
| `--web.config-file`                           | Path to TLS server config file                                          | `""`                            | Any valid file path                                                |
| `--web.listen-address`                        | Web server listen addresses (can be specified multiple times)           | `:28282`                        | Any valid host:port or :port format                                |
| `--debug.pprof`                               | Enable pprof debugging endpoints                                        | `false`                         | `true`, `false`                                                    |
| `--exporter.stdout`                           | Enable stdout exporter                                                  | `false`                         | `true`, `false`                                                    |
| `--exporter.prometheus`                       | Enable Prometheus exporter                                              | `true`                          | `true`, `false`                                                    |
| `--metrics`                                   | Metrics levels to export (can be specified multiple times)              | `node,process,container,vm,pod` | `node`, `process`, `container`, `vm`, `pod`                        |
| `--kube.enable`                               | Monitor kubernetes                                                      | `false`                         | `true`, `false`                                                    |
| `--kube.config`                               | Path to a kubeconfig file                                               | `""`                            | Any valid file path                                                |
| `--kube.node-name`                            | Name of kubernetes node on which kepler is running                      | `""`                            | Any valid node name                                                |
| `--experimental.platform.redfish.enabled`     | Enable experimental Redfish BMC power monitoring                        | `false`                         | `true`, `false`                                                    |
| `--experimental.platform.redfish.node-name`   | Node name for experimental Redfish platform power monitoring            | `""`                            | Any valid node name                                                |
| `--experimental.platform.redfish.config-file` | Path to experimental Redfish BMC configuration file                     | `""`                            | Any valid file path                                                |
| `--experimental.hwmon.enabled`                | Enable experimental hwmon power monitoring                              | `false`                         | `true`, `false`                                                    |
| `--experimental.hwmon.zones`                  | hwmon zones to be enabled (can be specified multiple times)             | All available zones             | Any valid hwmon zone name                                          |
| `--experimental.gpu.enabled`                  | Enable experimental GPU power monitoring                                | `false`                         | `true`, `false`                                                    |
| `--experimental.gpu.idle-power`               | GPU idle power in Watts (0 = auto-detect)                               | `0`                             | Any non-negative float                                             |

### 💡 Examples

```bash
# Run with debug logging
kepler --log.level=debug

# Use a different procfs path and JSON logging
kepler --host.procfs=/custom/proc --log.format=json

# Load configuration from file
kepler --config.file=/path/to/config.yaml

# Use custom listen addresses
kepler --web.listen-address=:8080 --web.listen-address=localhost:9090

# Enable stdout exporter and disable Prometheus exporter
kepler --exporter.stdout=true --exporter.prometheus=false

# Enable Kubernetes monitoring with specific kubeconfig and node name
kepler --kube.enable=true --kube.config=/path/to/kubeconfig --kube.node-name=my-node

# Enable experimental Redfish BMC power monitoring
kepler --experimental.platform.redfish.enabled=true \
       --experimental.platform.redfish.config-file=/path/to/redfish-config.yaml \
       --experimental.platform.redfish.node-name=worker-node-1

# Enable experimental hwmon power monitoring
kepler --experimental.hwmon.enabled=true

# Enable experimental hwmon power monitoring with specific zones
kepler --experimental.hwmon.enabled=true \
       --experimental.hwmon.zones=power1 \
       --experimental.hwmon.zones=power2

# Enable experimental GPU power monitoring
kepler --experimental.gpu.enabled=true

# Enable GPU monitoring with configured idle power (e.g. when GPUs are always under load)
kepler --experimental.gpu.enabled=true --experimental.gpu.idle-power=17.5

# Export only node and container level metrics
kepler --metrics=node --metrics=container

# Export only process level metrics
kepler --metrics=process

# Set maximum terminated workloads to 1000
kepler --monitor.max-terminated=1000

# Disable terminated workload tracking
kepler --monitor.max-terminated=0

# Unlimited terminated workload tracking
kepler --monitor.max-terminated=-1
```

## 🗂️ Configuration File

Kepler can load configuration from a YAML file. The configuration file offers more extensive options than command-line flags.

### 🧾 Sample Configuration File

```yaml
log:
  level: debug  # debug, info, warn, error (default: info)
  format: text  # text or json (default: text)

monitor:
  interval: 5s        # Monitor refresh interval (default: 5s)
  staleness: 1000ms   # Duration after which data is considered stale (default: 1000ms)
  maxTerminated: 500  # Maximum number of terminated workloads to keep in memory (default: 500)
  minTerminatedEnergyThreshold: 10  # Minimum energy threshold for terminated workloads (default: 10)

host:
  sysfs: /sys   # Path to sysfs filesystem (default: /sys)
  procfs: /proc # Path to procfs filesystem (default: /proc)

rapl:
  zones: []     # RAPL zones to be enabled, empty enables all default zones

exporter:
  stdout:       # stdout exporter related config
    enabled: false # disabled by default
  prometheus:   # prometheus exporter related config
    enabled: true
    debugCollectors:
      - go
      - process
    metricsLevel:
      - node
      - process
      - container
      - vm
      - pod

debug:          # debug related config
  pprof:        # pprof related config
    enabled: true

web:
  configFile: "" # Path to TLS server config file
  listenAddresses: # Web server listen addresses
    - ":28282"

kube:           # kubernetes related config
  enabled: false    # Enable kubernetes monitoring (default: false)
  config: ""        # Path to kubeconfig file (optional if running in-cluster)
  nodeName: ""      # Name of the kubernetes node (required when enabled)
  podInformer:      # Pod informer configuration
    mode: kubelet          # "kubelet" (default) or "apiserver"
    pollInterval: 15s      # Poll interval for kubelet mode (default: 15s)

experimental:   # experimental features (no stability guarantees)
  platform:     # platform power monitoring
    redfish:    # redfish BMC power monitoring
      enabled: false                  # Enable Redfish BMC monitoring (default: false)
      nodeID: ""                      # Node identifier (auto-resolved if empty)
      configFile: ""                  # Path to BMC configuration file (required when enabled)
      staleness: 30s                  # Cache duration for power readings (default: 30s)
      httpTimeout: 5s                 # HTTP timeout for BMC requests (default: 5s)
  hwmon:        # hwmon power monitoring
    enabled: false                    # Enable hwmon power monitoring (default: false)
    zones: []                         # hwmon zones to be enabled, empty enables all available zones
  gpu:          # GPU power monitoring
    enabled: false                    # Enable GPU power monitoring (default: false)
    idlePower: 0                      # GPU idle power in Watts, 0 = auto-detect (default: 0)

# WARN: DO NOT ENABLE THIS IN PRODUCTION - for development/testing only
dev:
  fake-cpu-meter:
    enabled: false
    zones: []  # Zones to be enabled, empty enables all default zones
```

## 🧩 Configuration Options in Detail

### 📝 Logging Configuration

```yaml
log:
  level: info   # Logging level
  format: text  # Output format
```

- **level**: Controls the verbosity of logging
  - `debug`: Very verbose, includes detailed operational information
  - `info`: Standard operational information
  - `warn`: Only warnings and errors
  - `error`: Only errors

- **format**: Controls the output format of logs
  - `text`: Human-readable format
  - `json`: JSON format, suitable for log processing systems

### 📊 Monitor Configuration

```yaml
monitor:
  interval: 5s
  staleness: 1000ms
  maxTerminated: 500
  minTerminatedEnergyThreshold: 10
```

- **interval**: The monitor's refresh interval. All processes with a lifetime less than this interval will be ignored. Setting to 0s disables monitor refreshes.

- **staleness**: Duration after which data computed by the monitor is considered stale and recomputed when requested again. Especially useful when multiple Prometheus instances are scraping Kepler, ensuring they receive the same data within the staleness window. Should be shorter than the monitor interval.

- **maxTerminated**: Maximum number of terminated workloads (processes, containers, VMs, pods) to keep in memory until the data is exported. This prevents unbounded memory growth in high-churn environments. Set 0 to disable. When the limit is reached, the least power consuming terminated workloads are removed first.

- **minTerminatedEnergyThreshold**: Minimum energy consumption threshold (in joules) for terminated workloads to be tracked. Only terminated workloads with energy consumption above this threshold will be included in the tracking. This helps filter out short-lived processes that consume minimal energy. Default is 10 joules.

### 🗄️ Host Configuration

```yaml
host:
  sysfs: /sys    # Path to sysfs
  procfs: /proc  # Path to procfs
```

These settings specify where Kepler should look for system information. In containerized environments, you might need to adjust these paths.

### 🔋 RAPL Zones Configuration

```yaml
rapl:
  zones: []  # RAPL zones to be enabled
```

Running Average Power Limiting (RAPL) is Intel's power capping mechanism. By default, Kepler enables all available zones. You can restrict to specific zones by listing them.

Example with specific zones:

```yaml
rapl:
  zones: ["package", "core", "uncore"]
```

### 📦 Exporter Configuration

```yaml
exporter:
  stdout:       # stdout exporter related config
    enabled: false # disabled by default
  prometheus:   # prometheus exporter related config
    enabled: true
    debugCollectors:
      - go
      - process
    metricsLevel:
      - node
      - process
      - container
      - vm
      - pod
```

- **stdout**: Configuration for the stdout exporter
  - `enabled`: Enable or disable the stdout exporter (default: false)

- **prometheus**: Configuration for the Prometheus exporter
  - `enabled`: Enable or disable the Prometheus exporter (default: true)
  - `debugCollectors`: List of debug collectors to enable (available: "go", "process")
  - `metricsLevel`: List of metric levels to expose. Controls the granularity of metrics exported:
    - `node`: Node-level metrics (system-wide power consumption)
    - `process`: Process-level metrics (per-process power consumption)
    - `container`: Container-level metrics (per-container power consumption)
    - `vm`: Virtual machine-level metrics (per-VM power consumption)
    - `pod`: Pod-level metrics (per-pod power consumption in Kubernetes)

### 🐞 Debug Configuration

```yaml
debug:
  pprof:
    enabled: true
```

- **pprof**: Configuration for pprof debugging
  - `enabled`: When enabled, this exposes [pprof](https://golang.org/pkg/net/http/pprof/) debug endpoints that can be used for profiling Kepler (default: true)

### 🌐 Web Configuration

```yaml
web:
  configFile: ""  # Path to TLS server config file
  listenAddresses: # Web server listen addresses
    - ":28282"
```

- **configFile**: Path to a TLS server configuration file for securing Kepler's web endpoints
- **listenAddresses**: List of addresses where the web server should listen (default: [":28282"])
  - Supports both host:port format (e.g., "localhost:8080", "0.0.0.0:9090") and port-only format (e.g., ":8080")
  - Multiple addresses can be specified for listening on different interfaces or ports
  - IPv6 addresses are supported using bracket notation (e.g., "[::1]:8080")

Example TLS server configuration file content:

```yaml
# TLS server configuration
tls_server_config:
  cert_file: /path/to/cert.pem  # Path to the certificate file
  key_file: /path/to/key.pem    # Path to the key file
```

### 🐳 Kubernetes Configuration

```yaml
kube:
  enabled: false    # Enable kubernetes monitoring
  config: ""        # Path to kubeconfig file
  nodeName: ""      # Name of the kubernetes node
  podInformer:
    mode: kubelet          # "kubelet" or "apiserver"
    pollInterval: 15s      # Poll interval for kubelet mode
```

- **enabled**: Enable or disable Kubernetes monitoring (default: false)
  - When enabled, Kepler will monitor Kubernetes resources and expose pod level information

- **config**: Path to a kubeconfig file (optional)
  - Required when running Kepler outside of a Kubernetes cluster
  - When running inside a cluster, Kepler can use the in-cluster configuration
  - Must be a valid and readable kubeconfig file

- **nodeName**: Name of the Kubernetes node on which Kepler is running (required when enabled)
  - This helps Kepler identify which node it's monitoring
  - Must match the actual node name in the Kubernetes cluster
  - Required when `enabled` is set to `true`

- **podInformer**: Configuration for how Kepler discovers pod metadata
  - **mode**: Pod informer mode (default: `kubelet`)
    - `kubelet`: Polls the local kubelet `/pods` endpoint. Reduces API server load. The kubelet host and port are auto-discovered from the Node object at startup.
    - `apiserver`: Watches the kube-apiserver for pod events via a shared informer cache.
  - **pollInterval**: How often to poll kubelet for pod data (default: `15s`, kubelet mode only)

### 🧪 Experimental Configuration

```yaml
experimental:
  platform:
    redfish:
      enabled: false
      nodeID: ""
      configFile: ""
      staleness: 30s
      httpTimeout: 5s
  hwmon:
    enabled: false
    zones: []
  gpu:
    enabled: false
```

⚠️ **WARNING**: This section contains experimental features with no stability guarantees.

#### Redfish BMC Power Monitoring

- **enabled**: Enable experimental Redfish BMC power monitoring (default: false)
  - When enabled, Kepler will collect platform-level power metrics from BMC via Redfish API
  - Requires a valid BMC configuration file

- **nodeID**: Node identifier for power monitoring (auto-resolved if empty)
  - Priority: CLI flag → Kubernetes node name → hostname fallback
  - Must match the node identifier in your BMC configuration

- **configFile**: Path to BMC configuration file (required when enabled)
  - YAML file containing BMC endpoints, credentials, and node mappings
  - See [hack/redfish.yaml](../../hack/redfish.yaml) for example configuration

- **staleness**: Cache duration for power readings (default: 30s)
  - How long to cache BMC power readings before fetching new data
  - Reduces BMC load by serving cached data for repeated requests

- **httpTimeout**: HTTP timeout for BMC requests (default: 5s)
  - Maximum time to wait for BMC HTTP responses
  - Adjust based on your BMC's response time characteristics

**Example BMC Configuration File:**

```yaml
nodes:
  worker-node-1: bmc-1
  worker-node-2: bmc-2

bmcs:
  bmc-1:
    endpoint: https://192.168.1.100
    username: admin
    password: secret123
    insecure: true
  bmc-2:
    endpoint: https://192.168.1.101
    username: admin
    password: secret456
    insecure: true
```

#### hwmon Power Monitoring

- **enabled**: Enable experimental hwmon power monitoring (default: false)
  - When enabled, Kepler will collect power metrics from hwmon sensors
  - Available on architectures with hwmon power sensors
  - Error will be returned if hwmon is not available

- **zones**: List of hwmon zones to enable (default: all available power zones)
  - By default, Kepler enables all available hwmon power zones
  - You can restrict to specific zones by listing them

**Example with specific zones:**

```yaml
experimental:
  hwmon:
    enabled: true
    zones: ["power1", "power2"]
```

#### GPU Power Monitoring

- **enabled**: Enable experimental GPU power monitoring (default: false)
  - When enabled, Kepler will collect power metrics from NVIDIA GPUs using NVML
  - Requires NVIDIA drivers and NVML library to be available
  - Supports per-process power attribution based on GPU compute utilization
- **idlePower**: GPU idle power in Watts (default: 0 = auto-detect)
  - When set to 0, Kepler auto-detects idle power by tracking the minimum power observed when no compute processes are running
  - Set to a non-zero value to override auto-detection (useful when GPUs are always under load and true idle cannot be observed)

**Example:**

```yaml
experimental:
  gpu:
    enabled: true
    idlePower: 17.5  # Override idle power to 17.5W (0 = auto-detect)
```

### 🧑‍🔬 Development Configuration

```yaml
dev:
  fake-cpu-meter:
    enabled: false
    zones: []
```

⚠️ **WARNING**: This section is for development and testing only. Do not enable in production.

- **fake-cpu-meter**: When enabled, uses a fake CPU meter instead of real hardware metrics
  - `enabled`: Set to `true` to enable fake CPU meter
  - `zones`: Specific zones to enable, empty enables all

## 📖 Further Reading

For more details see the [config file](../../hack/config.yaml)

Happy configuring! 🎉
