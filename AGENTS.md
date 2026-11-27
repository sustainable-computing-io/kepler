<!-- SPDX-FileCopyrightText: 2025 The Kepler Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->

# Working on Kepler

Kepler is a Prometheus exporter that reads hardware energy and power sensors on a Linux
node and attributes consumption to processes, containers, virtual machines, and Kubernetes
pods. It can run on a standalone host or as a DaemonSet. Kubernetes supplies pod metadata
when enabled. [Installation guide](docs/user/installation.md)

## What matters in Kepler

- **Workload CPU power is attributed.** Kepler measures CPU energy or power at the hardware,
  splits power into active and idle using node CPU utilization, then allocates active
  power by workload CPU time. A pod and its containers account for the same work.
  Adding their CPU watts together counts that work twice.
  [Power attribution guide](docs/developer/power-attribution-guide.md#independent-attribution)
- **Workloads can exit between scrapes.** The monitor retains terminated workloads within
  configured limits so their energy can still be reported after they exit.
  [Monitor implementation](internal/monitor/monitor.go) · [Fair allocation](docs/developer/design/architecture/principles.md#1-fair-power-allocation)

## How to approach a change

Start with the behavior the contributor wants and check how the code behaves today.
Read that code before describing it. If a pod's CPU watts look wrong, trace its CPU
time and the node's active power before changing the Prometheus collector. Choose the
simplest change that meets the request. Add an interface or configuration flag only
when the change needs it.

Use the suggested workflows as starting points. Ask if a requirement, a referenced
dependency, or an architectural decision is unclear. If a file, type, or dependency
cannot be found, ask instead of adding one. The repository sets explicit
rules for approvals, race detection, and commit sign-off. [Permissions](#permissions)

## Terminology

- **Energy and power.** `Energy` counts microjoules and `Power` counts microwatts.
  `Joules()` and `Watts()` convert those values for Prometheus, which reports joules
  and watts. GPU power on a process, container, or pod is a separate `float64` already
  in watts. GPU energy still uses `Energy`.
  [Energy and power types](internal/device/energy.go) · [Metric catalog](docs/user/metrics.md)
- **Energy zone.** A hardware domain, such as a CPU package or its cores, for which
  a meter reports a reading. Zones can overlap, so summing every zone can double-count
  energy. [Energy zones](docs/developer/power-attribution-guide.md#energy-zones)
- **Snapshot and scrape.** A snapshot holds the monitor's readings and attribution for
  one collection. A scrape is a request to the metrics endpoint. Node, workload, and
  GPU metrics from the power collector reuse a fresh snapshot, or refresh it when it
  is stale. The platform collector asks the Redfish service for chassis watts. That
  service caches its BMC readings separately.
  [Monitor implementation](internal/monitor/monitor.go) ·
  [Redfish service](internal/platform/redfish/service.go)

## Where to make a change

- **Attribution or workload identity.** Start with sensor readings in `internal/device`
  or workload CPU time and identity in `internal/resource`. Kubernetes pod metadata comes
  from `internal/k8s`. Follow those inputs into the workload-specific calculation and tests
  in `internal/monitor`. For lifecycle changes, cover workloads that exit between scrapes
  and processes that reuse a PID. [Resource tests](internal/resource/informer_pidreuse_test.go) ·
  [Monitor tests](internal/monitor/process_pidreuse_test.go)
- **GPU and chassis power.** The experimental NVIDIA meter attributes GPU power to
  processes. The Prometheus platform collector gets chassis watts directly from the
  Redfish service. [NVIDIA attribution](internal/device/gpu/nvidia/collector.go) ·
  [Platform collector](internal/exporter/prometheus/collector/platform_collector.go)
- **Prometheus workload metrics.** Trace the value from the monitor snapshot to the power
  collector. Check its name, units, labels, and which metrics levels expose it. Update
  the collector tests and run
  `make gen-metrics-docs` to regenerate `docs/user/metrics.md`.
  [Snapshot types](internal/monitor/types.go) · [Power collector and tests](internal/exporter/prometheus/collector/)
- **Configuration.** Start in `config/config.go` for defaults, YAML fields, CLI flags,
  and validation. Explicit flags override YAML, which overrides defaults. Keep `dev.*`
  settings YAML-only. When a user-facing setting changes, check startup wiring, user docs,
  and the affected deployment examples. The config-change workflow lists the paths for
  Compose, Kustomize, Helm, and local development.
  [Configuration](config/config.go) · [Startup wiring](cmd/kepler/main.go) ·
  [Configuration guide](docs/user/configuration.md) · [Config-change workflow](.github/workflows/config-change.yaml)

## Build and verify

Build the exporter on Linux with the Go version specified in the module.
Use a Make target whenever one exists for the operation. Run `make all` before a PR.
It cleans build artifacts, formats, lints, vets, builds, and runs the unit tests.
Use `make help` for the full target list. [Go module](go.mod) · [Makefile](Makefile) ·
[Multi-architecture builds](docs/developer/multi-arch-builds.md)

```bash
make test TEST_PKGS=./internal/monitor/...  # Focused package tests with the race detector
make test                                  # All unit tests with race detection and coverage
make build                                 # bin/kepler
make build PRODUCTION=1                    # bin/kepler-release
```

- For a local scrape, run `docker compose up --build -d` from `compose/dev/` and open
  <http://localhost:28283/metrics>. On a development host without usable CPU sensors,
  set `preferredMeters: [fake]` under `cpu` before starting. Run `docker compose down`
  in the same directory when finished. [Compose dev config](compose/dev/kepler-dev/etc/kepler/config.yaml)
- For bare-metal E2E, `make test-e2e` builds the race-enabled test binary. Run
  `sudo ./bin/kepler-e2e.test -test.v` on a suitable host. For Kubernetes E2E against
  a cluster with Kepler deployed, use `CGO_ENABLED=1 make test-e2e-k8s GOTEST='go test -race'`.
  The Kubernetes target needs the explicit race flag. [E2E prerequisites and hardware options](docs/developer/e2e-testing.md)
- Add tests for new features and regressions. Use `testify` for assertions and mocks,
  maintain coverage, and investigate unexpected failures before changing expectations.
  [Pre-commit checks](docs/developer/pre-commit.md) · [PR workflow](.github/workflows/pr-checks.yaml)

## Permissions

### Allowed (no approval needed)

- Read any project files
- Run `make fmt`, `make vet`, `make lint`, `make test`, `make build`, `make coverage`
- Run `docker compose up/down` in `compose/dev/`
- Create or update test files (with SPDX headers)
- Update code documentation and comments
- Fix linter errors and race conditions
- Refactor code following existing patterns
- Update `docs/` (except auto-generated `docs/user/metrics.md`)

### Requires approval

- Committing changes (NEVER commit unless explicitly asked)
- Any git push, rebase, or reset operation
- Modifying `go.mod` or adding new dependencies
- Modifying CI/CD configs (`.github/`, Makefile)
- Modifying `AGENTS.md`, `CLAUDE.md`, or `GOVERNANCE.md`
- Deploying to clusters (`make deploy`, `kubectl apply`)
- Architectural changes (prepare an [Enhancement Proposal](docs/developer/proposal/EP-TEMPLATE.md) first)

### Never allowed

- Force push to `main`
- Bypass hooks with `--no-verify` or disable signing with `--no-gpg-sign`
- Commit without DCO sign-off (`-s` flag)
- Disable or skip race detection in tests
- Manually edit `docs/user/metrics.md` (auto-generated)

## Contributions and pull requests

All code must be thread-safe. Start every Go file with these two lines.

```go
// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0
```

Target `main` unless requested otherwise. Use Conventional Commits for commit messages
and PR titles. Keep each PR focused, link a triaged issue, and fill in every section
of the [PR template](.github/pull_request_template.md), including the AI Disclosure.
[Pull request policy](CONTRIBUTING.md#pull-request-policy)

User-visible feature, configuration, metric or deployment changes must ship with
matching docs. Update `docs/` in the same PR. A follow-up issue alone does not satisfy
this rule. Internal-only changes can use `Doc - No Need` in the PR template's
Documentation section. Breaking changes require deprecation and migration guidance.
When a commit is requested, sign off with `git commit -s` (DCO).
[Commit hygiene](CONTRIBUTING.md#commit-hygiene)

When helping a first-time contributor or new maintainer, remind them to record their
affiliation in `cncf/gitdm` for Kepler DevStats. [Contributing guide](CONTRIBUTING.md)
