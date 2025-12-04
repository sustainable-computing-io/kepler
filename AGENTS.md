<!-- SPDX-FileCopyrightText: 2025 The Kepler Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->

# AGENTS.md

> **Quick Reference for AI Coding Agents**
>
> Kepler is a Kubernetes-based Efficient Power Level Exporter that measures energy consumption at container, pod, VM, process, and node levels. This guide provides essential context for AI agents contributing to the project.

---

## Quick Start (TL;DR)

```bash
# Essential workflow
make deps        # Install dependencies
make test        # Run tests with race detection
make build       # Build binary
make fmt lint    # Format & lint code
```

**Critical Rules**:

- All Go files MUST have SPDX headers (`// SPDX-FileCopyrightText: 2025 The Kepler Authors` + `// SPDX-License-Identifier: Apache-2.0`)
- Use `testify` for tests (`github.com/stretchr/testify/assert` and `/mock`)
- All commits MUST use conventional commits format with `-s` flag (DCO sign-off)
- All code MUST pass `-race` flag tests (thread-safety required)
- Run `make fmt vet lint test` before any PR

---

## AI Agent Permissions

### ✅ Autonomous Actions Allowed

- Read any project files
- Run: `make fmt`, `make vet`, `make lint`, `make test`, `make build`, `make coverage`
- Run: `docker compose up/down` in `compose/dev/` for local testing
- Create/update test files (with proper SPDX headers)
- Update code documentation and comments
- Fix linter errors and race conditions
- Refactor code following existing patterns
- Update `docs/` (except auto-generated `docs/user/metrics.md`)

### ⚠️ Requires Human Approval

- **Committing changes** (NEVER commit unless explicitly requested)
- **Git operations**: push, force push, reset --hard, rebase
- Creating new dependencies or modifying `go.mod`
- Modifying CI/CD configurations (`.github/`, Makefile targets)
- Architectural changes (should create Enhancement Proposal first)
- Modifying `AGENTS.md` or `GOVERNANCE.md`
- Deploying to clusters (`make deploy`, `kubectl apply`)
- Manually editing `docs/user/metrics.md` (auto-generated via `make gen-metrics-docs`)

### 🚫 Never Allowed

- Force push to main/master branches
- Skip hooks (`--no-verify`, `--no-gpg-sign`)
- Commit without DCO sign-off
- Disable race detection in tests

---

## Dev Environment Setup

### Prerequisites

- **Go**: See `go.mod` for required version
- **Pre-commit**: For automated quality checks
- **Docker**: For container image builds (optional)
- **Kind**: For local Kubernetes testing (optional)

### Initial Setup

```bash
# Clone and navigate to repository
cd /path/to/kepler

# Install pre-commit hooks (required)
pre-commit install

# Install/verify dependencies
make deps

# Build the project
make build

# Run tests to verify setup
make test
```

### Development Workflow

Common targets (run `make help` for all):

```bash
make fmt          # Format code (go fmt)
make vet          # Static analysis (go vet)
make lint         # Linting (golangci-lint)
make test         # Tests with race detection
make coverage     # HTML coverage report
make build        # Production binary
make build-debug  # Debug binary with race detection
make clean        # Clean artifacts
```

### Local Testing Environment

For testing code changes in a complete monitoring stack:

```bash
# Docker Compose (recommended for local development/testing)
cd compose/dev
docker compose up --build -d  # Start: Kepler + Prometheus + Grafana + comparisons (scaphandre, node-exporter)

# View logs
docker compose logs -f kepler-dev

# Stop and clean up
docker compose down --volumes
```

**Access Points:**

- Kepler Metrics: <http://localhost:28283/metrics>
- Prometheus: <http://localhost:29090>
- Grafana: <http://localhost:23000> (credentials: admin/admin)

**Alternative Testing Methods:**

```bash
# Local binary (requires sudo for hardware access)
sudo ./bin/kepler --config.file hack/config.yaml

# Kubernetes with Kind (full cluster testing)
make cluster-up                         # Create local Kind cluster
make image deploy                       # Build and deploy Kepler
kubectl get pods -n kepler              # Verify deployment
kubectl logs -n kepler -l app=kepler -f # View logs
make undeploy cluster-down              # Clean up
```

See [docs/user/installation.md](docs/user/installation.md#3-docker-compose-recommended-for-development) for detailed setup instructions.

### Project Structure

```text
kepler/
├── cmd/kepler/              # Main entry point
├── internal/                # Core implementation
│   ├── device/             # Hardware abstraction (RAPL sensors)
│   ├── exporter/           # Prometheus, stdout exporters
│   ├── k8s/                # Kubernetes integration
│   ├── monitor/            # Power monitoring and attribution
│   ├── resource/           # Process/container tracking
│   └── service/            # Service framework
├── docs/                   # Documentation
│   ├── user/              # User guides
│   └── developer/         # Architecture, proposals
├── manifests/             # Kubernetes/Helm manifests
└── hack/                  # Development scripts
```

---

## Testing Instructions

### Running Tests

```bash
make test                                             # All tests with race detection
make coverage                                         # Generate coverage.html
CGO_ENABLED=1 go test -v -race ./internal/monitor/... # Specific package
```

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

### CI Checks

All PRs must pass:

- `make fmt` - Code formatting
- `make vet` - Static analysis
- `make lint` - Linting (golangci-lint with 5m timeout)
- `make test` - Tests with race detection
- `make gen-metrics-docs` - Metrics documentation must be up-to-date
- Pre-commit hooks (markdownlint, yamllint, commitlint, reuse-lint, shellcheck)
- Container image builds
- OpenSSF Scorecard

---

## PR Instructions

### Commit Message Format

**REQUIRED**: Follow [Conventional Commits](https://www.conventionalcommits.org/) with DCO sign-off:

```bash
# Format
<type>[optional scope]: <description>

[optional body]

[optional footer]

# Examples
git commit -s -m "feat(monitor): add terminated workload tracking"
git commit -s -m "fix(exporter): resolve race condition in metrics handler"
git commit -s -m "docs: update architecture diagram"

# Types: feat, fix, docs, style, refactor, test, chore, ci, perf
# -s flag required (DCO sign-off)
```

Enforced via `commitlint` in pre-commit hooks.

### Pre-Submission Checklist

Run locally BEFORE submitting PR:

```bash
make all              # Runs: clean fmt lint vet build test (recommended)
# OR individually:
make fmt vet lint     # Format, analyze, lint
make test coverage    # Test with race detection + coverage report
make build            # Verify build succeeds
make gen-metrics-docs # If you modified metrics (then git add docs/user/metrics.md)
make deps             # Ensure dependencies are tidy
```

**If you modified ANY documentation**, verify consistency across all docs:

```bash
# Search for the same or similar content across all documentation files
grep -rn "<your-modified-content>" AGENTS.md README.md CONTRIBUTING.md docs/
```

**Documentation files to check**: AGENTS.md, README.md, CONTRIBUTING.md, docs/user/, docs/developer/

### File Headers

All source files MUST include SPDX license headers (REUSE-compliant):

```go
// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0
```

### PR Guidelines

1. **Title**: Use conventional commit format (e.g., `feat: add feature X`)
2. **Description**: Reference related issues (`Closes #123`)
3. **Scope**: One feature/fix per PR (keep focused)
4. **Tests**: Include tests for new features and bug fixes
5. **Documentation**: Update docs if behavior changes
6. **No Breaking Changes**: Without proper deprecation and migration guide

### Common Gotchas

- **Metrics Documentation**: Auto-generated via `make gen-metrics-docs` - don't manually edit `docs/user/metrics.md`
- **Race Conditions**: All code must be thread-safe; tests run with `-race`
- **Commit Sign-off**: Forgot `-s`? Amend: `git commit --amend -s --no-edit`
- **Pre-commit Failures**: Run `pre-commit run --all-files` to check everything

---

## Kepler-Specific Context

### Core Mission

Provide accurate, reliable power consumption monitoring for cloud-native workloads in Kubernetes environments using hardware sensors (Intel RAPL) and process-level attribution.

### Nine Design Principles

When making design decisions, follow these architectural principles:

1. **Fair Power Allocation** - Track terminated workloads to prevent unfair attribution
2. **Data Consistency & Mathematical Integrity** - Maintain atomic snapshots; validate energy conservation
3. **Computation-Presentation Separation** - Separate data models (Monitor) from export formats (Exporters) via `PowerDataProvider` interface
4. **Data Freshness Guarantee** - Configurable staleness threshold (default 10s); automatic refresh
5. **Deterministic Processing** - Thread-safe, race-free operations with immutable snapshots
6. **Prefer Package Reuse** - Use battle-tested libraries over custom implementations
7. **Configurable Collection & Exposure** - Users control which metrics to collect/expose
8. **Implementation Abstraction** - Interface-based design for flexibility
9. **Simple Configuration** - Hierarchical config: CLI flags > YAML files > Defaults

Reference: `docs/developer/design/architecture/principles.md`

### Key Architectural Patterns

- **Service-Oriented Design**: Components implement `service.Service` interface
- **Interface-Based Abstractions**: Hardware, resources, and exporters use interfaces
- **Dependency Injection**: Services composed at startup in `cmd/kepler/main.go`
- **Single Writer, Multiple Readers**: Power monitor updates atomically; exporters read snapshots
- **Graceful Shutdown**: All services handle context cancellation properly

### Technology Stack

- **Logging**: `go.uber.org/zap` (structured logging)
- **Metrics**: `prometheus/client_golang`
- **Kubernetes Client**: `k8s.io/client-go`
- **Service Management**: `oklog/run`
- **CLI Parsing**: `alecthomas/kingpin/v2`
- **Testing**: `stretchr/testify`
- **Concurrency**: `golang.org/x/sync` (singleflight)

### Code Quality Standards

- **Idiomatic Go**: Follow [Effective Go](https://go.dev/doc/effective_go)
- **Error Handling**: Always handle errors explicitly; use structured logging for context
- **Modularity**: Functions <50 lines; single responsibility
- **Naming**: Descriptive names; avoid abbreviations unless universal
- **Performance**: Profile before optimizing; document performance-critical sections
- **Security**: Validate inputs; avoid injection vulnerabilities (command, SQL, XSS)

### Enhancement Proposals (EPs)

For significant changes, use template at `docs/developer/proposal/EP_TEMPLATE.md` (see `EP-001-redfish-support.md` example). Required sections: Problem Statement, Goals/Non-Goals, Detailed Design, Testing Plan, Migration Strategy.

### Configuration

- **Hierarchical**: CLI flags override YAML files, which override defaults
- **Dev Options**: Config keys prefixed with `dev.*` are not exposed as CLI flags
- **Validation**: All configs validated at startup; fail fast on errors

### Additional Resources

For detailed information beyond this quick reference, consult:

- **Quick Start & Installation**: `README.md` - Helm Quick Start, other deployment methods (Kustomize, local binary)
- **Installation Guide**: `docs/user/installation.md` - Comprehensive installation instructions for all methods
- **Architecture Deep-Dive**: `docs/developer/design/architecture/` - Detailed system design, component interactions
- **Full Contributing Guide**: `CONTRIBUTING.md` - Complete process, DCO requirements, governance
- **Enhancement Proposals**: `docs/developer/proposal/` - EP template and examples for significant changes
- **User Documentation**: `docs/user/` - Configuration reference, metrics catalog, Helm updates
- **Governance**: `GOVERNANCE.md` - Maintainer roles, decision-making process
- **Security Policy**: `SECURITY.md` - Vulnerability reporting procedures

### Getting Help

- **Issues**: Search existing issues before creating new ones at github.com/sustainable-computing-io/kepler/issues
- **Maintainers**: Tag in PR comments for architectural/design questions
- **Community**: CNCF Slack (#kepler), GitHub Discussions
- **Significant Changes**: Create Enhancement Proposal using `docs/developer/proposal/EP_TEMPLATE.md`

---

**Remember**: Run `make help` to see all available development targets. Quality contributions are focused, well-tested, and aligned with the nine design principles. When in doubt, ask before making architectural changes.
