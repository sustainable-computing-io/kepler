# Embedding Kepler in a Go application

Kepler's public `config` and `collector` packages support collecting metrics
without starting the standalone binary or an HTTP listener.

```go
cfg := config.DefaultConfig()
cfg.Host.ProcFS = "/host/proc"
cfg.Host.SysFS = "/host/sys"
cfg.Exporter.Prometheus.DebugCollectors = nil

runtime, err := collector.NewRuntime(cfg, logger)
if err != nil {
    return err
}
defer runtime.Shutdown(context.Background())

registry := prometheus.NewRegistry()
for _, metricCollector := range runtime.Collectors() {
    if err := registry.Register(metricCollector); err != nil {
        return err
    }
}
// Gather directly, pass registry to a bridge, or serve it with promhttp.HandlerFor.
families, err := registry.Gather()
```

`NewRuntime` validates and deep-copies the configuration, discovers hardware and
optional Kubernetes metadata, initializes services, and starts background sampling.
The constructor needs a complete configuration; begin with `DefaultConfig`.
Do not mutate the source configuration concurrently with construction. Changes to
it afterwards do not reconfigure an existing runtime. The CLI shares the same
sampling engine and native Prometheus collector implementations.

The constructor never parses flags, installs signal handlers, opens HTTP
listeners, or registers global metrics. Web, stdout, and pprof settings apply only
to the CLI. Prometheus metrics levels and debug collector selection still apply;
Go/process debug metrics describe the hosting application. Config.ValidateRuntime
excludes unused web settings; Config.Validate retains the CLI's validation.

Stop gathering before calling `Shutdown`. Shutdown cancels runners, waits for
them to finish, and then releases their dependencies in reverse order, including
hardware services without a Run method. It is safe to call repeatedly or
concurrently. If the shutdown context expires, cleanup continues in the background
and a subsequent call can wait for completion. Separate runtimes and registries
own independent state. The component's startup context is not retained.

Kepler requires Linux, readable procfs/sysfs and supported hardware meters. CGO is
required by the optional NVIDIA NVML backend. Kubernetes integration is optional;
enable it and provide node identity and credentials to attribute power to pods.
Embedding does not change host access or Kubernetes RBAC requirements. Energy
zones can overlap; retain labels and avoid summing overlapping domains.
