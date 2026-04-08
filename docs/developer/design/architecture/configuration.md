# Configuration System

This document explains Kepler's hierarchical configuration system, which provides flexible, user-friendly configuration management while maintaining operational simplicity.

## Design Principle: Simple Configuration

> *"Simple Configuration to reduce learning curve - keep flags and configuration in sync (as much as possible)"*

The configuration system balances flexibility with simplicity, providing sensible defaults while allowing precise control when needed.

## Configuration Hierarchy

Configuration follows a clear precedence order, with higher levels overriding lower levels:

```text
1. CLI Flags (highest precedence) → Operational overrides
2. YAML Files (middle precedence) → Persistent configuration
3. Default Values (lowest precedence) → Sensible out-of-box behavior
```

### Example Configuration Flow

```bash
# Start with defaults
kepler
# ↓ Override with YAML file
kepler --config=production.yaml
# ↓ Override specific values with CLI flags
kepler --config=production.yaml --log.level=debug --monitor.interval=5s
```

## Configuration Structure

### Main Configuration Types

```go
type Config struct {
    Log          Log           `yaml:"log"`          // Logging configuration
    Host         Host          `yaml:"host"`         // System paths
    Monitor      Monitor       `yaml:"monitor"`      // Collection behavior
    Rapl         Rapl          `yaml:"rapl"`         // Hardware filtering
    Exporter     Exporter      `yaml:"exporter"`     // Export configuration
    Web          Web           `yaml:"web"`          // HTTP server
    Debug        Debug         `yaml:"debug"`        // Debug features
    Dev          Dev           `yaml:"dev"`          // Development options (no CLI flags)
    Kube         Kube          `yaml:"kube"`         // Kubernetes integration
    Experimental *Experimental `yaml:"experimental"` // Experimental features (nil when unused)
}
```

### Logging Configuration

```go
type Log struct {
    Level  string `yaml:"level"`   // debug, info, warn, error
    Format string `yaml:"format"`  // text, json
}
```

**CLI Flags:**

- `--log.level`: Override log level
- `--log.format`: Override log format

**YAML Example:**

```yaml
log:
  level: info
  format: json
```

### System Paths Configuration

```go
type Host struct {
    SysFS  string `yaml:"sysfs"`   // Hardware sensor path (default: /sys)
    ProcFS string `yaml:"procfs"`  // Process info path (default: /proc)
}
```

**CLI Flags:**

- `--host.sysfs`: Override sysfs path
- `--host.procfs`: Override procfs path

**Use Cases:**

- **Container Deployment**: Mount host paths to different locations
- **Testing**: Point to test fixtures
- **Development**: Use different filesystem layouts

### Monitoring Configuration

```go
type Monitor struct {
    // Collection timing
    Interval  time.Duration `yaml:"interval"`   // How often to collect (default: 5s)
    Staleness time.Duration `yaml:"staleness"`  // Data freshness threshold (default: 500ms)

    // Terminated workload tracking
    MaxTerminated int   `yaml:"maxTerminated"`  // Capacity limit (default: 500)
    MinTerminatedEnergyThreshold int64 `yaml:"minTerminatedEnergyThreshold"` // Joules (default: 10)
}
```

**CLI Flags:**

- `--monitor.interval`: Collection frequency
- `--monitor.max-terminated`: Terminated workload limit

Note: `staleness` and `minTerminatedEnergyThreshold` are config-file only (no CLI flags).

**YAML Example:**

```yaml
monitor:
  interval: 5s
  staleness: 500ms
  maxTerminated: 500
  minTerminatedEnergyThreshold: 10
```

### Hardware Configuration

```go
type Rapl struct {
    Zones []string `yaml:"zones"`  // Filter specific zones (empty = all zones)
}
```

Note: `rapl.zones` is config-file only (no CLI flag).

**YAML Example:**

```yaml
rapl:
  zones: ["package", "dram"]  # Only collect package and DRAM zones
```

**Zone Options:**

- `package`: CPU package energy (recommended)
- `core`: CPU core energy
- `dram`: Memory energy
- `uncore`: Uncore/cache energy
- `psys`: Platform system energy (if available)

### Export Configuration

```go
type Exporter struct {
    Stdout     StdoutExporter     `yaml:"stdout"`
    Prometheus PrometheusExporter `yaml:"prometheus"`
}

type StdoutExporter struct {
    Enabled *bool `yaml:"enabled"`  // Pointer allows nil = use default
}

type PrometheusExporter struct {
    Enabled         *bool    `yaml:"enabled"`
    DebugCollectors []string `yaml:"debugCollectors"`
    MetricsLevel    Level    `yaml:"metricsLevel"`
}
```

**CLI Flags:**

- `--exporter.stdout`: Enable stdout exporter
- `--exporter.prometheus`: Enable Prometheus exporter
- `--metrics`: Metrics granularity level

Note: `debugCollectors` is config-file only (no CLI flag).

**YAML Example:**

```yaml
exporter:
  stdout:
    enabled: false
  prometheus:
    enabled: true
    debugCollectors: ["go", "process"]
    metricsLevel: "container"
```

### Web Server Configuration

```go
type Web struct {
    Config          string   `yaml:"configFile"`       // TLS configuration file
    ListenAddresses []string `yaml:"listenAddresses"` // Bind addresses
}
```

**CLI Flags:**

- `--web.config-file`: TLS/auth configuration
- `--web.listen-address`: HTTP listen addresses (can be repeated)

**YAML Example:**

```yaml
web:
  listenAddresses: [":28282"]
  configFile: "/etc/kepler/web-config.yaml"
```

### Kubernetes Integration

```go
type Kube struct {
    Enabled     *bool       `yaml:"enabled"`      // Enable Kubernetes features
    Config      string      `yaml:"config"`       // Kubeconfig path (empty = in-cluster)
    Node        string      `yaml:"nodeName"`     // Node name for metrics labels
    PodInformer PodInformer `yaml:"podInformer"`  // Pod informer settings
}

type PodInformer struct {
    Mode         string        `yaml:"mode"`         // "kubelet" (default) or "apiserver"
    PollInterval time.Duration `yaml:"pollInterval"` // Poll interval for kubelet mode (default: 15s)
}
```

**CLI Flags:**

- `--kube.enable`: Enable Kubernetes integration
- `--kube.config`: Kubeconfig file path
- `--kube.node-name`: Node name override

**YAML Example:**

```yaml
kube:
  enabled: true
  config: ""  # Use in-cluster config
  nodeName: "node-1"
```

### Debug Configuration Structure

```go
type Debug struct {
    Pprof PprofDebug `yaml:"pprof"`
}

type PprofDebug struct {
    Enabled *bool `yaml:"enabled"`
}
```

**CLI Flags:**

- `--debug.pprof`: Enable pprof endpoints

**YAML Example:**

```yaml
debug:
  pprof:
    enabled: true
```

### Development Configuration Structure

```go
type Dev struct {
    FakeCpuMeter struct {
        Enabled *bool    `yaml:"enabled"`  // Use fake CPU meter
        Zones   []string `yaml:"zones"`    // Fake zone list
    } `yaml:"fake-cpu-meter"`
}
```

**Important**: Development options are **NOT** exposed as CLI flags - they must be set in YAML files only. This prevents accidental use in production.

**YAML Example:**

```yaml
dev:
  fake-cpu-meter:
    enabled: true
    zones: ["package", "core", "dram"]  # zones must be specified when enabled
```

## Configuration Loading Process

### 1. Default Configuration

Every configuration option has a sensible default:

```go
func DefaultConfig() *Config {
    return &Config{
        Log: Log{
            Level:  "info",
            Format: "text",
        },
        Host: Host{
            SysFS:  "/sys",
            ProcFS: "/proc",
        },
        Rapl: Rapl{
            Zones: []string{},
        },
        Monitor: Monitor{
            Interval:                     5 * time.Second,
            Staleness:                    500 * time.Millisecond,
            MaxTerminated:                500,
            MinTerminatedEnergyThreshold: 10,
        },
        Exporter: Exporter{
            Stdout: StdoutExporter{
                Enabled: ptr.To(false),
            },
            Prometheus: PrometheusExporter{
                Enabled:         ptr.To(true),
                DebugCollectors: []string{"go"},
                MetricsLevel:    MetricsLevelAll,
            },
        },
        Web: Web{
            ListenAddresses: []string{":28282"},
        },
        Kube: Kube{
            Enabled: ptr.To(false),
            PodInformer: PodInformer{
                Mode:         "kubelet",
                PollInterval: 15 * time.Second,
            },
        },
        Debug: Debug{
            Pprof: PprofDebug{
                Enabled: ptr.To(false),
            },
        },
        // Experimental is nil by default (allocated on demand)
    }
}
```

### 2. YAML File Loading

YAML files override defaults:

```go
func FromFile(filename string) (*Config, error) {
    data, err := os.ReadFile(filename)
    if err != nil {
        return nil, fmt.Errorf("failed to read config file: %w", err)
    }

    cfg := DefaultConfig()
    if err := yaml.Unmarshal(data, cfg); err != nil {
        return nil, fmt.Errorf("failed to parse config file: %w", err)
    }

    return cfg, nil
}
```

### 3. CLI Flag Integration

CLI flags are registered with kingpin and applied last:

```go
func RegisterFlags(app *kingpin.Application) func(*Config) error {
    // Register all flags (only explicitly set flags override config)
    logLevel := app.Flag("log.level", "Log level").String()
    logFormat := app.Flag("log.format", "Log format").String()

    monitorInterval := app.Flag("monitor.interval", "Collection interval").Duration()

    exporterPrometheus := app.Flag("exporter.prometheus", "Enable Prometheus exporter").Bool()
    exporterStdout := app.Flag("exporter.stdout", "Enable stdout exporter").Bool()

    // ... more flags

    // Return function that applies only flags the user explicitly set
    return func(cfg *Config) error {
        if flagsSet["log.level"] {
            cfg.Log.Level = *logLevel
        }
        if flagsSet["monitor.interval"] {
            cfg.Monitor.Interval = *monitorInterval
        }
        if flagsSet["exporter.prometheus"] {
            cfg.Exporter.Prometheus.Enabled = exporterPrometheus
        }
        if flagsSet["exporter.stdout"] {
            cfg.Exporter.Stdout.Enabled = exporterStdout
        }

        return nil
    }
}
```

### 4. Complete Loading Flow

```go
func parseArgsAndConfig() (*Config, error) {
    app := kingpin.New("kepler", "Power consumption monitoring exporter")

    configFile := app.Flag("config.file", "Path to YAML configuration file").String()
    updateConfig := RegisterFlags(app)
    kingpin.MustParse(app.Parse(os.Args[1:]))

    // Start with defaults
    cfg := DefaultConfig()

    // Override with YAML file if provided
    if *configFile != "" {
        loadedCfg, err := FromFile(*configFile)
        if err != nil {
            return nil, err
        }
        cfg = loadedCfg
    }

    // Apply CLI flags (highest precedence)
    if err := updateConfig(cfg); err != nil {
        return nil, err
    }

    return cfg, nil
}
```

## Configuration Validation

### Type Safety

Configuration uses Go's type system for validation:

```go
type Level int

const (
    MetricsLevelNode Level = iota
    MetricsLevelProcess
    MetricsLevelContainer
    MetricsLevelVM
    MetricsLevelPod
    MetricsLevelAll
)

func (l *Level) UnmarshalYAML(value *yaml.Node) error {
    var s string
    if err := value.Decode(&s); err != nil {
        return err
    }

    switch s {
    case "node":
        *l = MetricsLevelNode
    case "process":
        *l = MetricsLevelProcess
    case "container":
        *l = MetricsLevelContainer
    case "vm":
        *l = MetricsLevelVM
    case "pod":
        *l = MetricsLevelPod
    case "all":
        *l = MetricsLevelAll
    default:
        return fmt.Errorf("invalid metrics level: %s", s)
    }

    return nil
}
```

### Runtime Validation

Configuration is validated after loading:

```go
func (cfg *Config) Validate() error {
    var errs []error

    // Validate log level
    validLevels := []string{"debug", "info", "warn", "error"}
    if !contains(validLevels, cfg.Log.Level) {
        errs = append(errs, fmt.Errorf("invalid log level: %s", cfg.Log.Level))
    }

    // Validate paths exist
    if _, err := os.Stat(cfg.Host.SysFS); err != nil {
        errs = append(errs, fmt.Errorf("sysfs path not accessible: %w", err))
    }

    // Validate intervals
    if cfg.Monitor.Interval < 0 {
        errs = append(errs, fmt.Errorf("invalid monitor interval: can't be negative"))
    }

    if cfg.Monitor.Staleness < 0 {
        errs = append(errs, fmt.Errorf("invalid monitor staleness: can't be negative"))
    }

    return errors.Join(errs...)
}
```

## Configuration Examples

### Development Configuration Example

```yaml
# dev-config.yaml
log:
  level: debug
  format: text

dev:
  fake-cpu-meter:
    enabled: true
    zones: ["package", "core", "dram"]

exporter:
  stdout:
    enabled: true
  prometheus:
    enabled: true
    debugCollectors: ["go", "process"]
    metricsLevel: "all"

monitor:
  interval: 1s
  staleness: 3s
```

Usage:

```bash
kepler --config=dev-config.yaml
```

### Production Configuration

```yaml
# production.yaml
log:
  level: info
  format: json

host:
  sysfs: /host/sys
  procfs: /host/proc

kube:
  enabled: true
  nodeName: "${NODE_NAME}"

exporter:
  stdout:
    enabled: false
  prometheus:
    enabled: true
    metricsLevel: "container"

monitor:
  interval: 5s
  staleness: 500ms
  maxTerminated: 50

web:
  listenAddresses: [":28282"]
  configFile: "/etc/kepler/web-config.yaml"

rapl:
  zones: ["package", "dram"]
```

Usage:

```bash
kepler --config=production.yaml --log.level=warn
```

### Minimal Configuration

```yaml
# minimal.yaml - only override what's necessary
kube:
  enabled: true

rapl:
  zones: ["package"]
```

Usage:

```bash
kepler --config=minimal.yaml
```

## Environment-Specific Patterns

### Container Deployment

```yaml
host:
  sysfs: /host/sys    # Host sysfs mounted in container
  procfs: /host/proc  # Host procfs mounted in container

kube:
  enabled: true
  nodeName: "${NODE_NAME}"  # From downward API

web:
  listenAddresses: [":28282"]
```

### Kubernetes DaemonSet

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: kepler-config
data:
  config.yaml: |
    host:
      sysfs: /host/sys
      procfs: /host/proc
    kube:
      enabled: true
      nodeName: "${NODE_NAME}"
    exporter:
      prometheus:
        metricsLevel: "pod"
---
apiVersion: apps/v1
kind: DaemonSet
spec:
  template:
    spec:
      containers:
      - name: kepler
        command: ["/kepler", "--config=/etc/kepler/config.yaml"]
        env:
        - name: NODE_NAME
          valueFrom:
            fieldRef:
              fieldPath: spec.nodeName
        volumeMounts:
        - name: config
          mountPath: /etc/kepler
      volumes:
      - name: config
        configMap:
          name: kepler-config
```

## Configuration Best Practices

### 1. Use Defaults When Possible

Don't override unless necessary:

```yaml
# Good - only override what's needed
log:
  level: debug

# Avoid - unnecessary overrides
log:
  level: debug
  format: text  # This is already the default
```

### 2. Operational vs Development Settings

**CLI Flags**: Use for operational overrides

```bash
# Override log level for debugging
kepler --config=production.yaml --log.level=debug

# Override collection interval for testing
kepler --config=production.yaml --monitor.interval=1s
```

**YAML Files**: Use for persistent configuration

```yaml
# production.yaml - persistent settings
monitor:
  interval: 5s
  maxTerminated: 50
```

### 3. Environment Variable Integration

For containerized deployments, use environment variables in YAML:

```yaml
kube:
  nodeName: "${NODE_NAME}"

web:
  listenAddresses: ["${LISTEN_ADDRESS:-0.0.0.0:8080}"]
```

### 4. Configuration Validation

Always validate configuration in CI/CD:

```bash
# Validate configuration syntax
kepler --config=production.yaml --help > /dev/null

# Test with dry-run mode (if available)
kepler --config=production.yaml --dry-run
```

## Troubleshooting Configuration

### Common Issues

1. **Path Problems**: Incorrect sysfs/procfs paths in containers
2. **Permission Issues**: Insufficient privileges for hardware access
3. **YAML Syntax**: Indentation and format errors
4. **Type Mismatches**: Wrong data types in YAML

### Debug Configuration Troubleshooting

Enable configuration debugging:

```bash
kepler --config=debug.yaml --log.level=debug
```

The startup log shows the final configuration:

```text
INFO Configuration
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
log:
  level: debug
  format: text
monitor:
  interval: 5s
  staleness: 500ms
...
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
```

---

## Next Steps

After understanding the configuration system:

- **[Components](components.md)**: Understand how configuration flows through system components
- **[User Configuration Guide](../../../user/configuration.md)**: End-user configuration documentation
