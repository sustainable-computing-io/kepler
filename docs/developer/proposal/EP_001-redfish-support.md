# EP-001: Redfish Power Monitoring Support

**Status**: Draft
**Author**: Sunil Thaha
**Created**: 2025-01-18
**Last Updated**: 2025-01-26

## Summary

This proposal adds Redfish BMC power monitoring support to Kepler, enabling collection
of platform-level power consumption data from server BMCs. This complements existing
RAPL CPU power monitoring and provides comprehensive server power visibility.

## Problem Statement

Currently, Kepler only measures CPU power consumption using Intel RAPL sensors. This
provides incomplete power visibility as it doesn't account for:

- **Platform Power**: Overall system power including PSU efficiency, cooling, storage,
  network interfaces, and other system components
- **Multi-vendor Support**: RAPL is Intel-specific and doesn't work on AMD or ARM systems
- **BMC Integration**: Modern data centers use BMCs for server management but Kepler
  can't leverage these existing power monitoring capabilities
- **Kubernetes Environments**: In containerized environments, understanding total node
  power consumption (not just CPU) is critical for resource allocation and cost attribution

### Current Limitations

1. **Incomplete Power Attribution**: Workloads are attributed only CPU power, missing
   significant power consumption from other components
2. **Platform Blindness**: No visibility into overall server power consumption trends
3. **Limited Hardware Support**: RAPL availability varies across processor generations and vendors
4. **Manual Power Management**: No integration with existing BMC-based power monitoring infrastructure

## Goals

- **Primary**: Add Redfish BMC power monitoring capability to Kepler
- **Multi-Environment Support**: Support Kubernetes, OpenStack bare metal, and standalone deployments
- **Seamless Integration**: Integrate with existing Kepler architecture and service patterns
- **Standard Metrics**: Provide platform power metrics via Prometheus following Kepler conventions
- **Security**: Maintain security best practices for credential management
- **Simplicity**: Focus on core functionality with minimal complexity

## Non-Goals

- Replace existing RAPL CPU power monitoring (complementary, not replacement)
- Support non-Redfish BMC protocols (IPMI, proprietary APIs) in initial implementation
- Implement power capping or control features (monitoring only)
- Provide historical power data storage beyond Prometheus metrics
- Support for edge devices or embedded systems without BMCs
- Advanced resilience patterns (circuit breakers, exponential backoff)

## Requirements

### Functional Requirements

- Use `github.com/stmcginnis/gofish` library for Redfish client functionality
- Support node-specific BMC configuration lookup in multi-node environments
- Implement standard Kepler service interfaces (Initializer, Runner, Shutdowner)
- Generate `kepler_node_platform_watts{source="redfish"}` and
  `kepler_node_platform_joules_total{source="redfish"}` metrics
- Follow Kepler's configuration patterns and coding conventions
- Support both secure (TLS verified) and insecure BMC connections

### Non-Functional Requirements

- **Performance**: Minimal impact on Kepler's CPU and memory footprint
- **Reliability**: Graceful handling of BMC connection failures
- **Security**: Secure credential storage and transmission
- **Maintainability**: Clean code following Go idioms and Kepler patterns
- **Testability**: Comprehensive unit and integration test coverage

## Proposed Solution

### High-Level Architecture

Add a new platform service layer to Kepler that collects power data from BMCs via
Redfish and exposes it directly through Prometheus collectors, separate from CPU
power attribution handled by PowerMonitor.

```text
┌─────────────────┐    ┌──────────────────┐
│   CPU Power     │    │  Platform Power  │
│   (RAPL)        │    │   (Redfish)      │
│                 │    │                  │
└─────────────────┘    └──────────────────┘
         │                        │
         ▼                        ▼
┌─────────────────┐    ┌──────────────────┐
│ Power Monitor   │    │ Platform         │
│ (Attribution)   │    │ Collector        │
└─────────────────┘    └──────────────────┘
         │                        │
         └──────────┬─────────────┘
                    ▼
           ┌──────────────────┐
           │ Prometheus       │
           │ Exporter         │
           └──────────────────┘
```

### Node Identification Strategy

The solution uses a flexible node identification approach that works across different environments:

1. **CLI Override**: `--platform.node-id=my-node` (highest priority)
2. **Kubernetes Node Name**: Uses existing `cfg.Kube.Node` from `--kube.node-name`
3. **Hostname Fallback**: Uses `os.Hostname()` as last resort

This creates a single BMC configuration file that maps node identifiers to BMC configurations,
eliminating the need for environment-specific configuration management.

## Detailed Design

### Package Structure

```text
internal/
├── platform/
│   └── redfish/
│       ├── service.go          # Main service implementation
│       ├── config.go           # Configuration parsing and validation
│       ├── power_reader.go     # Power data collection logic
│       ├── client.go           # Gofish client wrapper
│       └── service_test.go     # Unit tests
└── exporter/prometheus/collector/
    └── platform_collector.go  # Platform power metrics collector
```

### Service Interfaces

The Redfish service implements standard Kepler service interfaces:

- **`service.Initializer`**: Load configuration, resolve BMC, establish connection
- **`service.Runner`**: Periodic power data collection with context cancellation
- **`service.Shutdowner`**: Clean connection closure and resource cleanup

## Configuration

### Kepler Configuration

Platform configuration integrates with existing Kepler config structure:

```go
type Platform struct {
    NodeID  string  `yaml:"nodeID"`  // High-level node identifier
    Redfish Redfish `yaml:"redfish"`
}

type Redfish struct {
    Enabled    *bool  `yaml:"enabled"`
    ConfigFile string `yaml:"configFile"`
}
```

**CLI Flags:**

```bash
--platform.node-id=worker-node-1                     # Node identifier override
--platform.redfish.enabled=true                      # Enable Redfish monitoring
--platform.redfish.config=/etc/kepler/redfish.yaml   # BMC configuration file
```

### BMC Configuration File

Single configuration file maps nodes to BMCs (`/etc/kepler/redfish.yaml`):

```yaml
# Node identifier to BMC ID mapping
nodes:
  worker-node-1: bmc-1
  worker-node-2: bmc-2
  control-plane-1: control-bmc

# BMC connection details
bmcs:
  bmc-1:
    endpoint: "https://192.168.1.100"
    username: "admin"
    password: "secret123"
    insecure: true                # Skip TLS verification

  bmc-2:
    endpoint: "https://192.168.1.101"
    username: "admin"
    password: "secret456"
    insecure: false               # Verify TLS certificates

  control-bmc:
    endpoint: "https://192.168.1.102"
    username: "root"
    password: "admin123"
    insecure: false
```

### Security Considerations

- Store BMC credentials in Kubernetes secrets or secure files (permissions 600)
- Never log credentials or include in error messages
- Support both secure (TLS verified) and insecure connections
- Implement proper session management and connection timeouts

## Deployment Examples

### Kubernetes Environment

Standard DaemonSet deployment with BMC configuration from secrets:

```yaml
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: kepler
spec:
  template:
    spec:
      containers:
      - name: kepler
        args:
        - --kube.enable=true
        - --kube.node-name=$(NODE_NAME)
        - --platform.redfish.enabled=true
        - --platform.redfish.config=/etc/kepler/redfish.yaml
        env:
        - name: NODE_NAME
          valueFrom:
            fieldRef:
              fieldPath: spec.nodeName
        volumeMounts:
        - name: redfish-config
          mountPath: /etc/kepler
          readOnly: true
      volumes:
      - name: redfish-config
        secret:
          secretName: redfish-config
```

### Standalone Deployment

```bash
# Create BMC configuration
cat > /etc/kepler/redfish.yaml <<EOF
nodes:
  $(hostname): local-bmc
bmcs:
  local-bmc:
    endpoint: "https://192.168.1.100"
    username: "admin"
    password: "secret123"
    insecure: true
EOF

# Run Kepler with Redfish support
./kepler --platform.redfish.enabled=true
```

## Error Handling and Resilience

### Basic Error Handling

The initial implementation includes simple error handling:

- **Connection Failures**: Log errors and continue with RAPL-only monitoring
- **Authentication Errors**: Retry once, then disable BMC monitoring for node
- **Timeout Handling**: Use Go context with 30-second timeout for BMC requests
- **Graceful Degradation**: Kepler continues normal operation when BMCs unavailable

### Simple Retry Logic

```go
func (c *RedfishClient) readPowerWithRetry() (float64, error) {
    const maxRetries = 3
    const retryDelay = 2 * time.Second

    for attempt := 0; attempt < maxRetries; attempt++ {
        power, err := c.readPower()
        if err == nil {
            return power, nil
        }

        if attempt < maxRetries-1 {
            time.Sleep(retryDelay)
        }
    }

    return 0, fmt.Errorf("failed to read power after %d attempts", maxRetries)
}
```

## Testing Strategy

### Test Coverage

- **Unit Tests**: Service lifecycle, BMC resolution, configuration parsing
- **Integration Tests**: End-to-end with Redfish simulator/emulator
- **Vendor Testing**: Validation with Dell iDRAC, HPE iLO, Lenovo XCC
- **Performance Testing**: Impact on Kepler resource consumption
- **Security Testing**: Credential handling and TLS configuration

### Test Infrastructure

- Mock Redfish responses for unit testing
- Redfish simulator for integration testing
- Kubernetes test environments for DaemonSet validation

## Migration and Compatibility

### Backward Compatibility

- **No Breaking Changes**: Existing RAPL functionality remains unchanged
- **Opt-in Feature**: Redfish support is disabled by default
- **Configuration Isolation**: Platform configuration is separate from existing settings

### Migration Path

1. **Phase 1**: Deploy new Kepler version with Redfish support disabled
2. **Phase 2**: Create BMC configuration files and secrets
3. **Phase 3**: Enable Redfish monitoring on subset of nodes for testing
4. **Phase 4**: Roll out to all nodes after validation

### Rollback Strategy

- Disable Redfish monitoring via configuration flag
- Remove BMC configuration files
- Kepler continues operating with RAPL-only monitoring

## Metrics Output

Platform power metrics complement existing CPU power metrics:

```prometheus
# New platform power metrics
kepler_node_platform_watts{source="redfish",node_name="worker-1"} 450.5
kepler_node_platform_joules_total{source="redfish",node_name="worker-1"} 123456.789

# Existing CPU power metrics (unchanged)
kepler_node_cpu_watts{zone="package",node_name="worker-1"} 125.2
kepler_node_cpu_joules_total{zone="package",node_name="worker-1"} 89234.567
```

## Implementation Plan

### Phase 1: Foundation

- Add Redfish dependencies and basic service structure
- Implement configuration parsing and validation
- Create BMC resolution logic with node identifier fallback

### Phase 2: Core Functionality

- Implement Redfish client integration using gofish library
- Add power data collection from BMC endpoints
- Create service interface for platform power access

### Phase 3: Metrics and Export

- Create platform power Prometheus collector that directly queries Redfish service
- Add platform metrics to exporter registration
- Validate metrics format and output

### Phase 4: Testing and Validation

- Comprehensive unit and integration testing
- Multi-vendor BMC validation (Dell, HPE, Lenovo)
- Kubernetes deployment testing

### Phase 5: Documentation and Release

- User documentation and deployment guides
- Security best practices documentation
- Migration guide for existing deployments

## Risks and Mitigations

### Technical Risks

- **BMC Connectivity Issues**: Mitigate with robust retry logic and circuit breaker patterns
- **Vendor Compatibility**: Address through comprehensive testing with major BMC vendors
- **Performance Impact**: Validate minimal resource overhead through performance testing
- **Security Concerns**: Implement secure credential handling and TLS by default

### Operational Risks

- **Configuration Complexity**: Mitigate with clear documentation and examples
- **Deployment Dependencies**: Provide fallback to RAPL-only operation when BMC unavailable
- **Monitoring Gaps**: Ensure graceful degradation when platform power unavailable

## Success Metrics

- **Functional**: Platform power metrics available on 95%+ of nodes with BMCs
- **Performance**: <2% overhead on Kepler CPU/memory usage
- **Reliability**: <5% data collection failure rate under normal conditions
- **Adoption**: Documentation enables successful deployment by operations teams

## Future Enhancements

This implementation provides the foundation for enhanced resilience features
such as:

- **Circuit Breaker Pattern**: Prevent cascading failures when BMCs become unresponsive
- **Exponential Backoff**: Smart retry strategies with vendor-specific tuning
- **Health Monitoring**: Continuous BMC connectivity validation
- **Vendor Optimizations**: BMC-specific timeout and configuration tuning
- **Security Enhancements**: External secret integration, certificate management

## Open Questions

1. **Multi-chassis Support**: How should Kepler handle servers with multiple power supplies?
2. **Power Zones**: Should we expose chassis sub-component power (PSU, fans, storage)?

These questions will be addressed during implementation based on user feedback
and technical constraints discovered during development.
