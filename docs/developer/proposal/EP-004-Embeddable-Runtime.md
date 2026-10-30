# EP-004: Embeddable Kepler runtime

**Status**: Draft
**Authors**: @ArthurSens
**Reviewers**: TBD
**Approvers**: TBD
**Created**: 2026-10-29
**Last Updated**: 2026-10-29

## Summary

Expose Kepler as a reusable Go library for applications such as the Prometheus
OpenTelemetry Collector distribution. The standalone CLI and embedded runtime
share hardware discovery, workload attribution, Kubernetes metadata discovery,
and Prometheus collectors.

## Problem Statement

Metric generation is hidden behind Go's internal package boundary and dependency
composition lives in main. Embedding currently requires copying the CLI's wiring
or scraping a separate process. Background services also need explicit ownership
when several runtimes share a process.

## Goals and Non-Goals

Provide public configuration and collector APIs, independent runtime instances,
and bounded, repeatable shutdown. Preserve existing CLI flags and metrics. Do not
add OpenTelemetry dependencies, change attribution algorithms, or require an HTTP
listener, process signal handler, or global Prometheus registry for library users.

## Detailed Design

Move the Prometheus collector implementations to `collector/`. Add
`collector.NewRuntime(*config.Config, *slog.Logger)`, `Collectors()`, and
`Shutdown(context.Context)`. Construction validates an owned configuration copy,
initializes dependencies, and starts the background services. An internal engine
shares dependency composition and lifecycle management with the standalone CLI.
Collectors and the runtime live in the same public package; hardware and resource
implementations remain internal.

Each engine owns its cancellation function, initialized service list, runner
completion, and cleanup result. Stop runners before releasing their dependencies,
clean up initialized non-running services too, unwind partial initialization, and
return the same cleanup result for repeated shutdown. Runtime lifetime does not
retain the Collector's startup context. Remove the collector readiness goroutine
in favor of checking the readiness notification during collection.

## Configuration

Reuse `config.Config` and `DefaultConfig`. Add deep-copy support and runtime-only
validation that excludes unused HTTP configuration. Runtime construction never
parses CLI arguments, installs signals, or configures process-global logging.
Kubernetes discovery remains optional and uses the configured node and credentials.

## Testing Plan

Race-enabled tests cover two independent registries and runtimes, actual collection
using existing fake CPU meters and procfs fixtures, configuration ownership,
invalid construction, cleanup after startup failure, cancellation deadlines, and
repeated shutdown. Preserve existing collector tests and standalone build checks.
An external module in the Collector integration verifies public importability.

## Migration Strategy

The standalone binary uses the shared engine and the same collector constructors.
Existing CLI configuration and Prometheus metric names and labels remain intact.
Downstream consumers register Runtime.Collectors in their own Prometheus registry
and return the runtime shutdown method from their bridge lifecycle adapter.

## Security and Deployment

Embedding preserves host procfs/sysfs access and Kubernetes authorization
requirements. It adds no listener and needs no extra privileges. Real sensor
validation requires supported Linux hardware; fixture tests do not establish
hardware accuracy. GPU/NVML remains optional and requires CGO and native libraries.

## Alternatives Considered

HTTP self-scraping retains a listener and parsing overhead. Reimplementing Kepler
in an OpenTelemetry receiver duplicates attribution logic. A public facade over
internal hardware services preserves those implementations while exposing native
client_golang collectors.
