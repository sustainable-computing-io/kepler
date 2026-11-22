# EP-XXX: OTLP Metrics Exporter

**Status**: Draft
**Authors**: @cvas22
**Reviewers**: TBD
**Approvers**: TBD
**Created**: 2026-11-21
**Last Updated**: 2026-11-21

## Summary

Add an exporter that pushes Kepler's metrics over OTLP to an OpenTelemetry
Collector or any OTLP endpoint, next to the existing Prometheus and stdout
exporters. It reuses the existing Prometheus collectors through the
OpenTelemetry Prometheus bridge, so the metric names, labels and metric levels
are the same as on `/metrics`. It is off by default. Tracking issue: #2780.

## Problem Statement

Kepler can only be read by scraping `/metrics`. More and more clusters ship
telemetry through an OpenTelemetry Collector, or straight to a vendor OTLP
endpoint, and have no Prometheus that scrapes node agents. They have to run a
Collector `prometheus` receiver with pod discovery and scrape config for every
Kepler pod just to get the data in.

This was asked for in #97 and #659. A proof of concept was shown in 2023 for
the old architecture but never merged, and #659 was closed as stale in April
2026 while people were still asking about it.

### Current Limitations

1. No push path. Nodes that can't be scraped (edge nodes behind NAT, clusters
   with push-only backends) can't report Kepler metrics without a sidecar.
2. OTel-first setups need extra scrape config per deployment, kept in sync with
   Kepler's port, TLS and auth settings.
3. Kepler's resource attributes (node, service version) only exist as labels
   and are lost on the way through a scraping receiver unless relabeled.

## Goals

- **Primary Goal**: Push the same metrics the Prometheus exporter serves to an
  OTLP endpoint over gRPC or HTTP/protobuf at a configurable interval.
- **Secondary Goal**: Work with the standard `OTEL_EXPORTER_OTLP_*` and
  `OTEL_RESOURCE_ATTRIBUTES` environment variables, so existing Collector
  setups need no Kepler-specific settings.
- Keep one definition of every metric, so `/metrics` and OTLP can't drift.

## Non-Goals

- Traces or logs. This is about metrics only.
- Replacing or deprecating the Prometheus exporter.
- Renaming metrics to OpenTelemetry semantic conventions. Names stay the same
  as on `/metrics`; a later EP can cover semconv names once they exist for
  energy metrics.
- Exporting the debug collectors (`go`, `process`). Collectors and SDKs already
  cover runtime metrics.

## Requirements

### Functional Requirements

- A new `exporter.otlp` config section and matching CLI flags, disabled by
  default.
- Protocols `grpc` (default) and `http/protobuf`.
- Endpoint, push interval, insecure (plain text) and headers are configurable.
  Settings left empty fall back to the standard OTel environment variables.
- The exported metric names, labels and values match `/metrics` for the same
  metrics level.
- The Prometheus and OTLP exporters can run at the same time, or either one
  alone.
- On shutdown, the exporter flushes once before closing the connection.

### Non-Functional Requirements

- **Performance**: One push per interval. It reads the same snapshot as a
  scrape, so the cost is the same as one extra scrape per interval.
- **Reliability**: A failing or unreachable endpoint is logged and retried by
  the SDK; it never blocks the monitor or the Prometheus exporter.
- **Security**: TLS by default. Headers (often API keys) are never logged or
  printed in the config dump.
- **Maintainability**: No second set of metric definitions. New metrics added
  to the Prometheus collectors show up in OTLP with no extra work.
- **Testability**: Unit tests use an in-memory exporter; no network needed.

## Proposed Solution

### High-Level Architecture

```text
                       ┌─────────────────────────────┐
                       │ PowerMonitor (Snapshot)     │
                       └──────────────┬──────────────┘
                                      │
                       ┌──────────────▼──────────────┐
                       │ Prometheus collectors       │
                       │ (power, cpu_info, gpu_info, │
                       │  build_info, platform)      │
                       └───────┬──────────────┬──────┘
                 registered in │              │ registered in
          ┌────────────────────▼───┐   ┌──────▼─────────────────────────┐
          │ Prometheus registry    │   │ OTLP registry                  │
          │ /metrics (pull)        │   │ Prometheus bridge (producer)   │
          └────────────────────────┘   │ PeriodicReader (interval)      │
                                       │ otlpmetricgrpc / otlpmetrichttp│
                                       └──────────────┬─────────────────┘
                                                      │ push
                                            OTel Collector / backend
```

### Key Design Choices

- **Bridge instead of native instruments.** The
  `go.opentelemetry.io/contrib/bridges/prometheus` producer turns what a
  Prometheus `Gatherer` returns into OTel metrics at each read. Counters stay
  cumulative sums, gauges stay gauges. Writing native OTel instruments for
  every metric would duplicate the collector logic and the two outputs would
  drift.
- **Shared collector instances.** The power collector waits on the monitor's
  data channel, which holds one signal. A second set of collectors would
  compete for that signal, so the same instances are registered in both
  registries. A Prometheus collector can be registered in more than one
  registry.
- **Standard SDK exporters.** `otlpmetricgrpc` and `otlpmetrichttp` already
  read the `OTEL_EXPORTER_OTLP_*` variables, handle retries, compression and
  TLS. Kepler only sets what the user configures explicitly.
- **Resource attributes.** `service.name=kepler`, `service.version`, and, when
  the node name is known, `host.name` and `k8s.node.name`. Values from
  `OTEL_RESOURCE_ATTRIBUTES` and `OTEL_SERVICE_NAME` take precedence.

## Detailed Design

### Package Structure

```text
internal/exporter/
├── otlp/
│   ├── otlp.go          # Exporter service (Init, Shutdown)
│   └── otlp_test.go
└── prometheus/          # unchanged, collectors reused
config/
└── config.go            # OTLPExporter config, flags, validation
cmd/kepler/
└── main.go              # create collectors once, pass to both exporters
```

### API/Interface Changes

No change to existing interfaces. The new exporter implements the existing
`service.Initializer` and `service.Shutdowner`:

```go
type Exporter struct { /* ... */ }

func NewExporter(cfg config.OTLPExporter, opts ...OptionFn) *Exporter
func (e *Exporter) Name() string   // "otlp"
func (e *Exporter) Init() error    // registers collectors, starts the PeriodicReader
func (e *Exporter) Shutdown() error // flushes and closes, 5s timeout
```

`cmd/kepler/main.go` creates the collectors once with
`prometheus.CreateCollectors` when either exporter is enabled, and passes them
to both.

## Configuration

### Main Configuration Changes

```go
OTLPExporter struct {
    Enabled  *bool             `yaml:"enabled"`  // default false
    Endpoint string            `yaml:"endpoint"` // empty: OTEL_EXPORTER_OTLP_ENDPOINT
    Protocol string            `yaml:"protocol"` // grpc (default) | http/protobuf
    Interval time.Duration     `yaml:"interval"` // default 30s, must be > 0
    Insecure *bool             `yaml:"insecure"` // default false
    Headers  map[string]string `yaml:"headers"`  // YAML only, not a flag
}
```

```yaml
exporter:
  otlp:
    enabled: true
    endpoint: otel-collector.observability:4317
    protocol: grpc
    interval: 30s
    insecure: false
    headers:
      x-api-key: ${...} # prefer OTEL_EXPORTER_OTLP_HEADERS from a Secret
```

CLI flags: `--exporter.otlp`, `--exporter.otlp.endpoint`,
`--exporter.otlp.protocol`, `--exporter.otlp.interval`,
`--exporter.otlp.insecure`.

The metrics level comes from the existing `exporter.prometheus.metricsLevel`
(`--metrics`), since both exporters share the collectors.

### Security Considerations

- TLS is on unless `insecure` is set. Custom CAs and client certificates go
  through the standard `OTEL_EXPORTER_OTLP_CERTIFICATE`,
  `OTEL_EXPORTER_OTLP_CLIENT_CERTIFICATE` and `OTEL_EXPORTER_OTLP_CLIENT_KEY`
  variables, with the files mounted from a Secret.
- Headers are not printed by the config dump or logged. The docs recommend
  `OTEL_EXPORTER_OTLP_HEADERS` from a Secret over putting keys in the ConfigMap.
- No new listening ports. The exporter only makes outbound connections.

## Deployment Examples

### Kubernetes Environment

Helm values (all optional, `config` is rendered into the ConfigMap):

```yaml
config:
  exporter:
    otlp:
      enabled: true
      endpoint: otel-collector.observability:4317
      protocol: grpc
      insecure: true
```

Extra environment from a Secret, for example for a vendor endpoint:

```yaml
daemonset:
  extraEnv:
    - name: OTEL_EXPORTER_OTLP_HEADERS
      valueFrom:
        secretKeyRef:
          name: otlp-auth
          key: headers
```

### Standalone Deployment

```bash
kepler --exporter.otlp --exporter.otlp.endpoint=localhost:4317 --exporter.otlp.insecure
# or with only the standard variables
OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318 kepler --exporter.otlp --exporter.otlp.protocol=http/protobuf
```

## Testing Strategy

### Test Coverage

- **Unit Tests**: config defaults, flags, YAML and validation; exporter `Init`
  and `Shutdown` with an in-memory `sdkmetric.Exporter`, checking that a fake
  collector's metrics, labels and resource attributes come through; endpoint,
  insecure and header options for both protocols.
- **Integration Tests**: add an OTel Collector to `compose/dev` with an OTLP
  receiver and a `debug` exporter, to check the output by hand next to
  Prometheus.
- **End-to-End Tests**: optional. Compare a scrape of `/metrics` with what the
  Collector received for the same interval.

### Test Infrastructure

No new infrastructure. `sdkmetric` has a manual reader and in-memory exporters
for tests.

## Migration and Compatibility

### Backward Compatibility

No breaking changes. The exporter is off by default and the Prometheus
exporter is unchanged.

### Migration Path

1. **Phase 1**: Enable OTLP next to Prometheus and compare.
2. **Phase 2**: Optionally disable the Prometheus exporter and remove the
   Collector scrape config.

### Rollback Strategy

Set `exporter.otlp.enabled: false`, or drop the flag.

## Metrics Output

No new metrics. The same metrics as `/metrics`, for example:

```prometheus
kepler_node_cpu_joules_total{zone="package",path="..."}   # OTLP: cumulative Sum
kepler_node_cpu_watts{zone="package",path="..."}          # OTLP: Gauge
```

The `kepler_` names and labels are kept. Resource attributes carry the service
and node identity.

## Implementation Plan

### Phase 1: Foundation

- Config section, flags and validation.
- Add the OTel SDK, OTLP exporters and the Prometheus bridge to `go.mod`.

### Phase 2: Core Functionality

- `internal/exporter/otlp` service.
- Create collectors once in `main.go` and share them.

### Phase 3: Testing and Documentation

- Unit tests, `docs/user/configuration.md`, `hack/config.yaml`.
- Helm values and a Collector in `compose/dev`.

## Risks and Mitigations

### Technical Risks

- **Risk**: Terminated workloads are cleared after the first `Snapshot()` that
  marks the data as exported. With both exporters on, a short-lived workload
  could be seen by one exporter and cleared before the other reads it.
  - **Mitigation**: See open question 1. For phase 1 this is documented, and it
    is the same behaviour two Prometheus servers scraping one Kepler have today.
- **Risk**: The OTel modules add to the binary size and the dependency tree.
  - **Mitigation**: Only the metric SDK, the two OTLP metric exporters and the
    bridge. The gRPC exporter pulls in `google.golang.org/grpc`, which Kepler
    doesn't depend on yet. If that's too much, phase 1 can ship
    `http/protobuf` only. The size difference will be measured in the
    implementation PR.
- **Risk**: The bridge maps Prometheus types in a fixed way (no native
  histograms needed here; Kepler only uses counters and gauges).
  - **Mitigation**: Unit test the mapping for every collector.

### Operational Risks

- **Risk**: An unreachable endpoint fills the logs.
  - **Mitigation**: The SDK's error handler goes through Kepler's logger,
    rate limited.

## Alternatives Considered

### Alternative 1: Collector `prometheus` receiver only

- **Description**: Document a Collector scrape config and don't change Kepler.
- **Reason for Rejection**: It's what people do today. It needs per-deployment
  scrape config and doesn't work where nodes can't be scraped.

### Alternative 2: Native OTel instruments

- **Description**: Re-implement every metric with OTel asynchronous
  instruments reading the snapshot.
- **Reason for Rejection**: Duplicates the collectors and the metric
  definitions, and the outputs would drift over time.

### Alternative 3: OTel Prometheus exporter for everything

- **Description**: Move all metrics to the OTel SDK and serve `/metrics`
  through the OTel Prometheus exporter.
- **Reason for Rejection**: A much bigger change that would alter metric
  output for every existing user.

## Success Metrics

- **Functional Metric**: For the same interval, every series on `/metrics` is
  received by the Collector with the same name, labels and value.
- **Performance Metric**: No measurable change in Kepler's CPU use with the
  default 30s interval compared to a 30s scrape.
- **Adoption Metric**: Users from #659 can drop their scrape config.

## Open Questions

1. Should `Snapshot()` track exports per consumer, so terminated workloads are
   kept until every enabled exporter has read them once?
2. Should OTLP get its own metrics level, or always follow `--metrics`?
3. Should the default interval follow `monitor.interval` (5s) instead of 30s?
4. Is the Helm chart part of this EP, or a follow-up?
