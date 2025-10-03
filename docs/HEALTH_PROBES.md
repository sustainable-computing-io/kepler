# Kubernetes Health Probes for Kepler

This documentation describes the implementation of health check endpoints for Kubernetes probes in Kepler.

## Overview

Health probes allow Kubernetes to determine the health status of your application. Kepler implements two types of probes:

- **Liveness Probe** (`/probe/livez`): Determines if the application is alive and responding
- **Readiness Probe** (`/probe/readyz`): Determines if the application is ready to receive traffic

## Endpoints

### `/probe/livez` - Liveness Probe

**Description**: Checks if Kepler's monitor service is alive and responding.

**Success Criteria**:
- PowerMonitor service is not nil
- Collection context is not cancelled

**Response**:
- `200 OK`: Service is alive
- `503 Service Unavailable`: Service is not alive

**Example Response**:
```json
{
  "status": "ok",
  "timestamp": "2025-01-17T10:30:00Z",
  "duration": "1.2µs"
}
```

### `/probe/readyz` - Readiness Probe

**Description**: Checks if Kepler's monitor service is ready to serve data.

**Success Criteria**:
- Service is alive (checks liveness first)
- At least one snapshot is available
- Snapshot is not too old (within staleness limit)
- CPU meter is functional
- Energy zones are initialized

**Response**:
- `200 OK`: Service is ready
- `503 Service Unavailable`: Service is not ready

**Example Response**:
```json
{
  "status": "ok", 
  "timestamp": "2025-01-17T10:30:00Z",
  "duration": "1.8µs"
}
```

## Kubernetes Configuration

### DaemonSet with Health Probes

```yaml
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: kepler
  namespace: kepler
spec:
  selector:
    matchLabels:
      app: kepler
  template:
    metadata:
      labels:
        app: kepler
    spec:
      containers:
      - name: kepler
        image: quay.io/sustainable_computing_io/kepler:latest
        ports:
        - containerPort: 28282
          name: http-metrics
        livenessProbe:
          httpGet:
            path: /probe/livez
            port: 28282
          initialDelaySeconds: 10
          periodSeconds: 30
          timeoutSeconds: 5
          failureThreshold: 3
        readinessProbe:
          httpGet:
            path: /probe/readyz
            port: 28282
          initialDelaySeconds: 5
          periodSeconds: 10
          timeoutSeconds: 5
          failureThreshold: 3
```

## Testing

### Unit Tests

Unit tests are available in `internal/server/health_test.go`:

```bash
go test ./internal/server/ -v
```

### Integration Tests

A test script is provided to test the endpoints live:

```bash
# Start Kepler
go run ./cmd/kepler/

# In another terminal, test the endpoints
./examples/test-health-endpoints.sh
```

## Architecture

### Interfaces

The following interfaces were added in `internal/service/service.go`:

```go
// LiveChecker checks if a service is alive
type LiveChecker interface {
    IsLive(ctx context.Context) (bool, error)
}

// ReadyChecker checks if a service is ready
type ReadyChecker interface {
    IsReady(ctx context.Context) (bool, error)
}
```

### Implementation

1. **PowerMonitor** (`internal/monitor/monitor.go`): Implements `LiveChecker` and `ReadyChecker` interfaces
2. **HealthProbeService** (`internal/server/health.go`): Service that exposes HTTP endpoints
3. **Integration** (`cmd/kepler/main.go`): Service registration in the main application

### Verification Flow

#### Liveness Check
1. Verify the monitor is not nil
2. Verify the collection context is not cancelled

#### Readiness Check  
1. Execute liveness check
2. Verify a snapshot is available
3. Verify the snapshot is not stale
4. Verify the CPU meter is available
5. Verify energy zones are initialized

## Performance

Health checks are designed to be very lightweight:
- **Liveness**: Typically 1-5 microseconds
- **Readiness**: Typically 1-10 microseconds

No forced data collection is performed during health checks to avoid performance impact.

## Debugging

### Logs

Health checks generate DEBUG level logs for successes and ERROR level logs for failures:

```bash
# View health check logs
journalctl -u kepler -f | grep "health-probe"
```

### Manual Testing

```bash
# Test liveness
curl -v http://localhost:28282/probe/livez

# Test readiness  
curl -v http://localhost:28282/probe/readyz

# With jq to format response
curl -s http://localhost:28282/probe/livez | jq .
```

## Troubleshooting

### Liveness probe fails

- Verify Kepler is started
- Check logs for startup errors
- Verify port 28282 is accessible

### Readiness probe fails

- Verify liveness probe works
- Verify `/proc` and `/sys` files are accessible
- Check RAPL zones configuration
- Verify collection interval is not too long

## Migration

This implementation is compatible with existing Kepler versions. The new endpoints are optional and do not affect existing functionality.

To enable health probes in an existing installation, simply update the Kubernetes configuration to include the new probes.

## Files Modified/Added

### New Files
- `internal/server/health.go` - Health probe service implementation
- `internal/server/health_test.go` - Unit tests for health probes
- `examples/kubernetes-health-probes.yaml` - Kubernetes DaemonSet example
- `examples/test-health-endpoints.sh` - Integration test script

### Modified Files
- `internal/service/service.go` - Added LiveChecker and ReadyChecker interfaces
- `internal/monitor/monitor.go` - Added IsLive() and IsReady() methods to PowerMonitor
- `cmd/kepler/main.go` - Registered health probe service

## Future Enhancements

- Add more granular health checks for different components
- Implement health check metrics for monitoring
- Add configuration options for health check behavior
- Support for custom health check plugins