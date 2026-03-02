<!-- SPDX-FileCopyrightText: 2025 The Kepler Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->

# AGENTS.md

> Kepler is a Kubernetes-based Efficient Power Level Exporter that measures
> energy consumption at container, pod, VM, process, and node levels. This
> guide provides essential context for AI agents contributing to the project.

## Commands

```bash
# Build and validate (run before any PR)
make all                # clean -> fmt -> lint -> vet -> build -> test

# Individual targets
make fmt                # Format code (go fmt)
make vet                # Static analysis (go vet)
make lint               # Linting (golangci-lint, 5m timeout locally, 3m in CI)
make test               # Tests with race detection
make build              # Production binary
make build-debug        # Debug binary with race detection
make coverage           # HTML coverage report
make deps               # Tidy and verify go.mod
make clean              # Clean artifacts
make gen-metrics-docs   # Regenerate docs/user/metrics.md (do NOT edit manually)

# Test a specific package (preferred when working in one area)
CGO_ENABLED=1 go test -v -race ./internal/monitor/...
CGO_ENABLED=1 go test -v -race ./internal/device/...
CGO_ENABLED=1 go test -v -race ./config/...

# Local integration testing
cd compose/dev && docker compose up --build -d   # Kepler + Prometheus + Grafana
cd compose/dev && docker compose down --volumes   # cleanup
```

**Access points** (Docker Compose):

- Kepler Metrics: <http://localhost:28283/metrics>
- Prometheus: <http://localhost:29090>
- Grafana: <http://localhost:23000> (credentials: admin/admin)

## Permissions

### Allowed (no approval needed)

- Read any project files
- Run: `make fmt`, `make vet`, `make lint`, `make test`, `make build`, `make coverage`
- Run: `docker compose up/down` in `compose/dev/`
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
- Architectural changes (create Enhancement Proposal first; see `docs/developer/proposal/EP_TEMPLATE.md`)

### Never allowed

- Force push to `main`
- Skip hooks (`--no-verify`, `--no-gpg-sign`)
- Commit without DCO sign-off (`-s` flag)
- Disable or skip race detection in tests
- Manually edit `docs/user/metrics.md` (auto-generated)

## Critical Rules

All Go files MUST have these SPDX headers as the first two lines:

```go
// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0
```

All code MUST be thread-safe. Tests run with `-race` and this is non-negotiable.

Always use `make` targets over raw `go` commands — only run raw commands if no `make` target exists for that operation.

## Project Structure

```text
kepler/
├── cmd/kepler/              # Main entry point
├── config/                  # Configuration (builder, validation, CLI flags)
├── internal/                # Core implementation
│   ├── device/             # Hardware abstraction (RAPL, HWMon, GPU)
│   ├── exporter/           # Prometheus, stdout exporters
│   ├── k8s/                # Kubernetes integration
│   ├── logger/             # Logging setup
│   ├── monitor/            # Power monitoring and attribution
│   ├── platform/           # Platform integrations (Redfish)
│   ├── resource/           # Process/container tracking
│   ├── server/             # HTTP server
│   └── service/            # Service framework
├── test/                    # E2E test suites
├── docs/                   # Documentation
│   ├── user/              # User guides
│   └── developer/         # Architecture, proposals
├── manifests/             # Kubernetes/Helm manifests
└── hack/                  # Development scripts
```

## Testing

### Test Requirements

- **Race Detection**: All tests MUST pass with `-race` flag (enforced by `make test`)
- **Coverage**: Maintain or improve existing coverage (tracked via Codecov)
- **Framework**: Use `testify` for assertions and mocking
  - Import: `github.com/stretchr/testify/assert`
  - Import: `github.com/stretchr/testify/mock`

### Test Patterns

```go
// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

// Verify interface implementations
var _ PowerDataProvider = (*MockPowerMonitor)(nil)

// Use t.Helper() in test helpers
func assertMetricValue(t *testing.T, expected, actual float64) {
    t.Helper()
    assert.Equal(t, expected, actual)
}

// Include cleanup logic
func TestExample(t *testing.T) {
    resource := setupResource()
    t.Cleanup(func() {
        resource.Close()
    })
    // test logic
}
```

### E2E Tests

```bash
make test-e2e && sudo ./bin/kepler-e2e.test -test.v   # Bare-metal (requires RAPL)
make test-e2e-k8s                                      # Kubernetes (requires: make cluster-up image deploy)
```

See `docs/developer/e2e-testing.md` for details.

## Commits and PRs

Default branch: `main`. PRs target `main` unless stated otherwise.

Commit format: [Conventional Commits](https://www.conventionalcommits.org/)
with DCO sign-off. Enforced by `commitlint` pre-commit hook.

```bash
git commit -s -m "feat(monitor): add terminated workload tracking"
git commit -s -m "fix(exporter): resolve race condition in metrics handler"
git commit -s -m "docs: update architecture diagram"
# Types: feat, fix, docs, style, refactor, test, chore, ci, perf
```

### PR Guidelines

1. **Title**: Use conventional commit format (e.g., `feat: add feature X`)
2. **Description**: Reference related issues (`Closes #123`)
3. **Scope**: One feature/fix per PR (keep focused)
4. **Tests**: Include tests for new features and bug fixes
5. **Documentation**: Update docs if behavior changes
6. **No Breaking Changes**: Without proper deprecation and migration guide

### CI Checks

All PRs must pass:

- `make fmt` - Code formatting
- `make vet` - Static analysis
- `make lint` - Linting (golangci-lint with 3m timeout in CI)
- `make test` - Tests with race detection
- `make gen-metrics-docs` - Metrics documentation must be up-to-date
- Pre-commit hooks (markdownlint, yamllint, commitlint, reuse-lint, shellcheck)
- Container image builds
- OpenSSF Scorecard

### Common Gotchas

- **Metrics Documentation**: Auto-generated via `make gen-metrics-docs` - don't manually edit `docs/user/metrics.md`
- **Race Conditions**: All code must be thread-safe; tests run with `-race`
- **Commit Sign-off**: Forgot `-s`? Amend: `git commit --amend -s --no-edit`
- **Pre-commit Failures**: Run `pre-commit run --all-files` to check everything

## Architecture

### Key Patterns

- **Service-Oriented Design**: Components implement `service.Service` interface (see `internal/service/service.go`)
- **Dependency Injection**: Services composed at startup in `cmd/kepler/main.go`
- **Single Writer, Multiple Readers**: Power monitor updates atomically; exporters read snapshots via `PowerDataProvider` interface
- **Interface-Based Abstractions**: Hardware, resources, and exporters use interfaces
- **Graceful Shutdown**: All services handle context cancellation properly

### Configuration

- **Hierarchical**: CLI flags override YAML files, which override defaults
- **Dev Options**: Config keys prefixed with `dev.*` are not exposed as CLI flags
- **Validation**: All configs validated at startup; fail fast on errors

### Technology Stack

- **Logging**: `log/slog` (structured logging, stdlib)
- **Metrics**: `prometheus/client_golang`
- **Kubernetes Client**: `k8s.io/client-go`
- **Service Management**: `oklog/run`
- **CLI Parsing**: `alecthomas/kingpin/v2`
- **Testing**: `stretchr/testify`
- **Concurrency**: `golang.org/x/sync` (singleflight)

### Design Principles

When making design decisions, follow these architectural principles
(reference: `docs/developer/design/architecture/principles.md`):

1. **Fair Power Allocation** - Track terminated workloads to prevent unfair attribution
2. **Data Consistency & Mathematical Integrity** - Maintain atomic snapshots; validate energy conservation
3. **Computation-Presentation Separation** - Separate data models (Monitor) from export formats (Exporters) via `PowerDataProvider` interface
4. **Data Freshness Guarantee** - Configurable staleness threshold (default 10s); automatic refresh
5. **Deterministic Processing** - Thread-safe, race-free operations with immutable snapshots
6. **Prefer Package Reuse** - Use battle-tested libraries over custom implementations
7. **Configurable Collection & Exposure** - Users control which metrics to collect/expose
8. **Implementation Abstraction** - Interface-based design for flexibility
9. **Simple Configuration** - Hierarchical config: CLI flags > YAML files > Defaults

### Code Quality

- **Error Handling**: Always handle errors explicitly; use structured logging for context
- **Idiomatic Go**: Follow [Effective Go](https://go.dev/doc/effective_go)
- **Security**: Validate inputs; avoid injection vulnerabilities (command, SQL, XSS)

### Enhancement Proposals

For significant changes, use the template at `docs/developer/proposal/EP_TEMPLATE.md`.
Required sections: Problem Statement, Goals/Non-Goals, Detailed Design, Testing Plan,
Migration Strategy.

## When Stuck

- Do not invent facts about the codebase — verify by reading the code before making claims or changes.
- Do not over-engineer; implement exactly what is asked, nothing more.
- If requirements are unclear, ask the user rather than guessing.
- If a test fails unexpectedly, report the failure. Do not modify the test to make it pass without understanding why it failed.
- If you cannot find a file, interface, or dependency, ask rather than creating new ones.
- If you are unsure whether a change is architectural, treat it as requiring approval.
- Run `make help` to see all available Makefile targets.

## References

- Architecture: `docs/developer/design/architecture/`
- Contributing: `CONTRIBUTING.md`
- Installation: `docs/user/installation.md`
- Configuration: `docs/user/configuration.md`
- Metrics catalog: `docs/user/metrics.md` (auto-generated)
- Enhancement Proposals: `docs/developer/proposal/EP_TEMPLATE.md`
- Governance: `GOVERNANCE.md`
- Security: `SECURITY.md`
- Issues: `github.com/sustainable-computing-io/kepler/issues`
