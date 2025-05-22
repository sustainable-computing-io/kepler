# ⚙️ Kepler Configuration Guide

Kepler supports configuration through both command-line flags and a configuration file. This guide outlines all available configuration options for configuring Kepler.

## 🛠️ Configuration Methods

Kepler supports two primary methods for configuration:

1. **Command-line flags**: For quick adjustments and one-time settings
2. **Configuration file**: For persistent and comprehensive configuration

> ⚡ **Tip:** Command-line flags take precedence over configuration file settings when both are specified.

## 🖥️ Command-line Flags

You can configure Kepler by passing flags when starting the service. The following flags are available:

| Flag | Description | Default | Values |
|------|-------------|---------|--------|
| `--config.file` | Path to YAML configuration file | | Any valid file path |
| `--log.level` | Logging level | `info` | `debug`, `info`, `warn`, `error` |
| `--log.format` | Output format for logs | `text` | `text`, `json` |
| `--host.sysfs` | Path to sysfs filesystem | `/sys` | Any valid directory path |
| `--host.procfs` | Path to procfs filesystem | `/proc` | Any valid directory path |
| `--monitor.interval` | Monitor refresh interval | `5s` | Any valid duration |
| `--web.config-file` | Path to TLS server config file | `""` | Any valid file path |
| `--debug.pprof` | Enable pprof debugging endpoints | `false` | `true`, `false` |
| `--exporter.stdout` | Enable stdout exporter | `false` | `true`, `false` |
| `--exporter.prometheus` | Enable Prometheus exporter | `true` | `true`, `false` |

### 💡 Examples

```bash
# Run with debug logging
kepler --log.level=debug

# Use a different procfs path and JSON logging
kepler --host.procfs=/custom/proc --log.format=json

# Load configuration from file
kepler --config.file=/path/to/config.yaml

# Enable stdout exporter and disable Prometheus exporter
kepler --exporter.stdout=true --exporter.prometheus=false
```

## 🗂️ Configuration File

Kepler can load configuration from a YAML file. The configuration file offers more extensive options than command-line flags.

### 🧾 Sample Configuration File

```yaml
log:
  level: debug  # debug, info, warn, error (default: info)
  format: text  # text or json (default: text)

monitor:
  interval: 5s      # Monitor refresh interval (default: 5s)
  staleness: 1000ms # Duration after which data is considered stale (default: 1000ms)

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

debug:          # debug related config
  pprof:        # pprof related config
    enabled: true

web:
  configFile: "" # Path to TLS server config file

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
```

- **interval**: The monitor's refresh interval. All processes with a lifetime less than this interval will be ignored. Setting to 0s disables monitor refreshes.

- **staleness**: Duration after which data computed by the monitor is considered stale and recomputed when requested again. Especially useful when multiple Prometheus instances are scraping Kepler, ensuring they receive the same data within the staleness window. Should be shorter than the monitor interval.

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
```

- **stdout**: Configuration for the stdout exporter
  - `enabled`: Enable or disable the stdout exporter (default: false)

- **prometheus**: Configuration for the Prometheus exporter
  - `enabled`: Enable or disable the Prometheus exporter (default: true)
  - `debugCollectors`: List of debug collectors to enable (available: "go", "process")

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
```

This setting specifies the path to a TLS server configuration file for securing Kepler's web endpoints.

Example TLS server configuration file content:

```yaml
# TLS server configuration
tls_server_config:
  cert_file: /path/to/cert.pem  # Path to the certificate file
  key_file: /path/to/key.pem    # Path to the key file
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
