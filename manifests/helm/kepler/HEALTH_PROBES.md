# Health Probes Configuration

## Overview

Kepler now provides optimized health check endpoints for better Kubernetes integration and performance.

## Optimized Endpoints (Default)

**Ultra-lightweight endpoints designed for Kubernetes health checks:**

- **`/probe/readyz`** - Readiness probe
  - Returns 200 when Kepler has collected at least one power sample
  - Returns 503 before first successful collection
  - Response time: < 1ms (no heavy calculations)

- **`/probe/livez`** - Liveness probe  
  - Returns 200 if sampling loop is active (< 30s since last sample)
  - Returns 503 if sampling has stalled
  - Response time: < 1ms (no heavy calculations)

## Configuration

### Use Optimized Probes (Recommended)

```yaml
daemonset:
  useOptimizedProbes: true  # Default
  
  livenessProbe:
    httpGet:
      path: /probe/livez
      port: http
    initialDelaySeconds: 10
    periodSeconds: 10        # Can be frequent due to low overhead
    
  readinessProbe:
    httpGet:
      path: /probe/readyz
      port: http
    initialDelaySeconds: 5
    periodSeconds: 5         # Can be frequent due to low overhead
```

### Fallback to Legacy Probes

For backward compatibility with existing monitoring systems:

```yaml
daemonset:
  useOptimizedProbes: false  # Use legacy /metrics endpoint
```

This will automatically switch back to:
- Liveness: `GET /metrics` (slower, may timeout under load)
- Readiness: No probe (legacy behavior)

## Benefits of Optimized Probes

| Feature | Optimized Endpoints | Legacy /metrics |
|---------|-------------------|------------------|
| **Response Time** | < 1ms | 50-200ms |
| **CPU Usage** | Minimal | High (triggers calculations) |
| **Memory Usage** | Minimal | High (snapshot generation) |
| **Timeout Risk** | Very low | Moderate (under load) |
| **Kubernetes Best Practice** | ✅ Separate readiness/liveness | ❌ Same endpoint for both |
| **Monitoring Overhead** | None | High (frequent /metrics calls) |

## Migration Guide

### For New Deployments
- No action needed, optimized probes are enabled by default

### For Existing Deployments  
1. Upgrade Kepler to version with health endpoints
2. Update your values.yaml:
   ```yaml
   daemonset:
     useOptimizedProbes: true
   ```
3. Apply the Helm upgrade
4. Monitor pod restarts during transition period

### Rollback if Needed
```yaml
daemonset:
  useOptimizedProbes: false
```

## Monitoring

Both probe types provide JSON responses for debugging:

**Optimized Success:**
```json
{"status": "ok"}
{"status": "alive"}
```

**Optimized Failure:**
```json
{"status": "not ready", "reason": "no successful sample yet"}
{"status": "not alive", "reason": "sampling loop stalled"}
```

## Troubleshooting

### Probe Failures
- Check Kepler logs for initialization errors
- Verify RBAC permissions for system access
- Ensure sufficient resources for Kepler pod

### Performance Issues  
- If using legacy probes and experiencing timeouts, switch to optimized endpoints
- Consider increasing probe timeout values for heavy-loaded clusters

---

**Recommendation:** Use optimized probes for all new deployments. They provide better performance, lower resource usage, and follow Kubernetes best practices.