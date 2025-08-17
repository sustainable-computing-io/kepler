# Kepler Troubleshooting Guide

This guide helps you diagnose and fix common issues when running Kepler. Whether
you're using Docker Compose, Kubernetes, or having configuration problems,
you'll find solutions here.

## 🩺 Quick Health Check

Start with these commands to quickly assess Kepler's status:

### Docker Compose Health Check

```bash
# Check service status
docker compose ps

# Check Kepler logs
docker compose logs kepler-dev

# Test metrics endpoint
curl -f http://localhost:28282/metrics >/dev/null && echo "✅ Metrics OK" || echo "❌ Metrics failed"

# Test Grafana access
curl -f http://localhost:23000 >/dev/null && echo "✅ Grafana OK" || echo "❌ Grafana failed"
```

### Kubernetes Health Check

```bash
# Check pod status
kubectl get pods -n kepler

# Check logs for errors
kubectl logs -n kepler -l app.kubernetes.io/name=kepler --tail=50

# Test metrics endpoint
kubectl port-forward -n kepler svc/kepler 28282:28282 &
curl -f http://localhost:28282/metrics >/dev/null && echo "✅ Metrics OK" || echo "❌ Metrics failed"
```

---

## 🐳 Docker Compose Issues

### Services Won't Start

**Symptoms:** `docker compose ps` shows services as "Exit 1" or "Restarting"

**Diagnosis:**

```bash
# Check what's failing
docker compose ps

# Check logs for the failing service
docker compose logs [service-name]

# Check port conflicts
netstat -tlnp | grep -E ":(23000|28282|9090)"
```

**Solutions:**

1. **Port Conflicts:**

```bash
# Stop conflicting services
sudo lsof -i :23000  # Find what's using port 23000
sudo kill -9 <PID>  # Kill the process

# Or change ports in compose.yaml
```

1. **Insufficient Resources:**

```bash
# Check available resources
docker system df
docker system prune  # Free up space if needed

# Check memory usage
free -h
```

1. **Permission Issues:**

```bash
# Fix docker permissions (Linux)
sudo usermod -aG docker $USER
newgrp docker
```

### No Metrics Data

**Symptoms:** Kepler starts but `/metrics` endpoint is empty or shows no `kepler_*` metrics

**Diagnosis:**

```bash
# Check if hardware is supported
docker compose exec kepler-dev ls /sys/class/powercap/intel-rapl/ 2>/dev/null || echo "No RAPL support"

# Check fake meter status
docker compose logs kepler-dev | grep -i fake
```

**Solutions:**

1. **Enable Fake CPU Meter (for VMs/testing):**

```bash
# Edit config to enable fake meter
sed -i 's/enabled: false/enabled: true/' kepler-dev/etc/kepler/config.yaml

# Restart Kepler
docker compose restart kepler-dev
```

1. **Hardware Access Issues:**

```bash
# Check container privileges
docker compose exec kepler-dev ls -la /proc/stat
docker compose exec kepler-dev ls -la /sys/class/powercap/
```

### Dashboard Shows No Data

**Symptoms:** Grafana loads but dashboards are empty

**Diagnosis:**

```bash
# Check if Prometheus is scraping Kepler
curl http://localhost:9090/api/v1/targets | jq '.data.activeTargets[] | select(.job=="kepler")'

# Check if metrics are available
curl http://localhost:28282/metrics | grep kepler_node_cpu_watts
```

**Solutions:**

1. **Prometheus Configuration:**

```bash
# Restart Prometheus to reload config
docker compose restart prometheus

# Check scrape targets in Prometheus UI
# Open http://localhost:9090/targets
```

1. **Wait for Data Collection:**

```bash
# Kepler needs time to collect data (wait 1-2 minutes)
# Check metrics are increasing
curl http://localhost:28282/metrics | grep kepler_node_cpu_joules_total
```

---

## ☸️ Kubernetes Issues

### Pods Not Starting

**Symptoms:** `kubectl get pods -n kepler` shows pods in `Pending`, `CrashLoopBackOff`, or `Error`

**Diagnosis:**

```bash
# Check pod status details
kubectl describe pods -n kepler

# Check events
kubectl get events -n kepler --sort-by=.metadata.creationTimestamp

# Check node resources
kubectl top nodes
```

**Solutions:**

1. **Resource Constraints:**

```bash
# Check node resources
kubectl describe node | grep -A 5 "Allocated resources"

# Adjust resource requests in deployment
kubectl edit daemonset -n kepler kepler
```

1. **Security/Privilege Issues:**

```bash
# Verify security context
kubectl get daemonset -n kepler -o yaml | grep -A 10 securityContext

# Check if privileged access is enabled
kubectl get daemonset -n kepler -o yaml | grep privileged
```

1. **Node Selector/Tolerations:**

```bash
# Check node labels and taints
kubectl describe nodes | grep -E "(Taints|Labels)"

# Update tolerations if needed
kubectl patch daemonset kepler -n kepler -p '{"spec":{"template":{"spec":{"tolerations":[{"operator":"Exists"}]}}}}'
```

### Permission Denied Errors

**Symptoms:** Kepler pods show permission errors in logs

**Diagnosis:**

```bash
# Check common permission issues
kubectl logs -n kepler -l app.kubernetes.io/name=kepler | grep -i "permission denied"

# Check security context
kubectl get pods -n kepler -o yaml | grep -A 10 securityContext
```

**Solutions:**

1. **Fix Security Context:**

```bash
# Ensure privileged access for hardware access
kubectl patch daemonset kepler -n kepler -p '{"spec":{"template":{"spec":{"containers":[{"name":"kepler","securityContext":{"privileged":true}}]}}}}'
```

1. **Check Host Path Permissions:**

```bash
# Verify host paths are accessible
kubectl exec -n kepler -it <pod-name> -- ls -la /host/proc/stat
kubectl exec -n kepler -it <pod-name> -- ls -la /host/sys/class/powercap/
```

### No Hardware Support

**Symptoms:** Logs show "No hardware support found" or similar

**Diagnosis:**

```bash
# Check for hardware power measurement support
kubectl exec -n kepler -it <pod-name> -- find /sys/class/powercap/ -name "energy_uj" | head -5

# Check CPU info
kubectl exec -n kepler -it <pod-name> -- grep -i "model name" /proc/cpuinfo | head -1
```

**Solutions:**

1. **Enable Fake CPU Meter for Testing:**

```bash
# Update configmap to enable fake meter
kubectl edit configmap kepler-config -n kepler

```

1. Update `config.yaml` to enable fake meter as follows

```yaml
dev:
  fake-cpu-meter:
    enabled: false
    zones: []  # Zones to be enabled, empty enables all default zones

```

1. Restart pods to pick up changes

```bash
kubectl rollout restart daemonset/kepler -n kepler
```

---

## 📊 Metrics and Monitoring Issues

### Missing or Zero Metrics

**Symptoms:** Some metrics are missing or always show zero values

**Diagnosis:**

```bash
# Check available metrics
curl -s http://localhost:28282/metrics | grep ^kepler | wc -l

# Check specific metric patterns
curl -s http://localhost:28282/metrics | grep -E "(kepler_node|kepler_container|kepler_process)_cpu_watts"

# Check metric labels
curl -s http://localhost:28282/metrics | grep kepler_node_cpu_watts | head -3
```

**Solutions:**

1. **Enable More Metric Levels:**

```bash
# For Docker Compose - check config.yaml
grep -A 10 "metricsLevel:" kepler-dev/etc/kepler/config.yaml

# For Kubernetes - update configmap
kubectl get configmap kepler-config -n kepler -o yaml
```

1. **Wait for Data Collection:**

```bash
# Metrics need time to accumulate
# Check that counters are increasing over time
curl -s http://localhost:28282/metrics | grep kepler_node_cpu_joules_total
sleep 30
curl -s http://localhost:28282/metrics | grep kepler_node_cpu_joules_total
```

### High Memory Usage

**Symptoms:** Kepler consumes excessive memory

**Diagnosis:**

```bash
# Check memory usage
docker stats kepler-dev  # For Docker Compose
kubectl top pods -n kepler  # For Kubernetes

# Check terminated workload tracking
curl -s http://localhost:28282/metrics | grep kepler_terminated_
```

**Solutions:**

1. **Adjust Terminated Workload Limits:**

```bash
# Edit configuration to limit terminated workload tracking
# Set maxTerminated to lower value (default: 500)
# Set minTerminatedEnergyThreshold higher (default: 10)
```

1. **Reduce Monitoring Scope:**

```bash
# Disable process-level monitoring if not needed
# Edit config to remove "process" from metricsLevel
```

### Inconsistent Power Attribution

**Symptoms:** Power values seem incorrect or inconsistent

**Diagnosis:**

```bash
# Check if fake meter is inadvertently enabled
curl -s http://localhost:28282/metrics | grep kepler_build_info

# Verify hardware support
ls -la /sys/class/powercap/intel-rapl*/energy_uj 2>/dev/null || echo "No RAPL support"
```

**Solutions:**

1. **Ensure Real Hardware Mode:**

```bash
# Disable fake CPU meter if accidentally enabled
# Check config.yaml for dev.fake-cpu-meter.enabled: false
```

1. **Calibrate Expectations:**

```bash
# Power attribution is estimated based on resource usage
# Values are approximate, not precise measurements
# Compare trends rather than absolute values
```

---

## ⚙️ Configuration Issues

### Configuration Not Loading

**Symptoms:** Changes to config.yaml don't take effect

**Diagnosis:**

```bash
# Check if config file is being read
# Look for config-related log messages
docker compose logs kepler-dev | grep -i config  # Docker Compose
kubectl logs -n kepler -l app.kubernetes.io/name=kepler | grep -i config  # Kubernetes
```

**Solutions:**

1. **Docker Compose:**

```bash
# Restart after config changes
docker compose restart kepler-dev

# Verify config file is mounted correctly
docker compose exec kepler-dev cat /etc/kepler/config.yaml
```

1. **Kubernetes:**

```bash
# Restart pods after configmap changes
kubectl rollout restart daemonset/kepler -n kepler

# Verify configmap is updated
kubectl get configmap kepler-config -n kepler -o yaml
```

### Invalid YAML Configuration

**Symptoms:** Kepler fails to start with YAML parsing errors

**Diagnosis:**

```bash
# Check YAML syntax
python -c "import yaml; yaml.safe_load(open('config.yaml'))" 2>&1 || echo "Invalid YAML"

# Check logs for specific parsing errors
docker compose logs kepler-dev | grep -i "yaml\|parse\|config"
```

**Solutions:**

1. **Validate YAML:**

```bash
# Use online YAML validator or:
python -m yaml config.yaml  # If PyYAML is installed
```

1. **Reset to Default:**

```bash
# Copy default config from repository
curl -o config.yaml https://raw.githubusercontent.com/sustainable-computing-io/kepler/main/hack/config.yaml
```

---

## 🔍 Advanced Debugging

### Enable Debug Logging

```bash
# Docker Compose - edit config.yaml
sed -i 's/level: info/level: debug/' kepler-dev/etc/kepler/config.yaml
docker compose restart kepler-dev

# Kubernetes - update configmap
kubectl patch configmap kepler-config -n kepler --type merge -p '{"data":{"config.yaml":"log:\n  level: debug\n  format: text\n..."}}'
kubectl rollout restart daemonset/kepler -n kepler
```

### Use pprof for Performance Analysis

```bash
# Enable pprof (usually enabled by default)
curl http://localhost:28282/debug/pprof/goroutine?debug=1

# Get heap profile
curl -o heap.prof http://localhost:28282/debug/pprof/heap

# Analyze with go tool (if available)
go tool pprof heap.prof
```

### Network Connectivity Issues

```bash
# Test internal connectivity
kubectl exec -n kepler -it <pod-name> -- netstat -tlnp

# Check DNS resolution
kubectl exec -n kepler -it <pod-name> -- nslookup kubernetes.default.svc.cluster.local

# Test external connectivity (if needed)
kubectl exec -n kepler -it <pod-name> -- curl -I https://google.com
```

---

## 🆘 Getting Help

If you're still experiencing issues after trying these solutions:

### Before Asking for Help

1. **Gather Information:**

```bash
# System information
uname -a
docker --version  # or kubectl version
```

1. **Collect Logs:**

```bash
# Save relevant logs
docker compose logs kepler-dev > kepler-logs.txt  # Docker Compose
kubectl logs -n kepler -l app.kubernetes.io/name=kepler > kepler-logs.txt  # Kubernetes
```

1. **Configuration:**

```bash
# Export current configuration
docker compose exec kepler-dev cat /etc/kepler/config.yaml > current-config.yaml  # Docker Compose
kubectl get configmap kepler-config -n kepler -o yaml > current-config.yaml  # Kubernetes
```

### Community Resources

- **🐛 GitHub Issues:** [Create an issue](https://github.com/sustainable-computing-io/kepler/issues/new) with logs and configuration
- **💬 GitHub Discussions:** [Ask questions](https://github.com/sustainable-computing-io/kepler/discussions)
- **🗨️ CNCF Slack:** Join [#kepler](https://cloud-native.slack.com/archives/C06HYDN4A01) for real-time help
- **📚 Documentation:** [Full documentation](https://sustainable-computing.io/kepler/docs/)

### Issue Report Template

When reporting issues, please include:

```text
**Environment:**
- OS: [Linux/macOS/Windows]
- Deployment: [Docker Compose/Kubernetes]
- Kepler version: [from kepler_build_info metric]

**Problem:**
[Describe what's not working]

**Expected:**
[What you expected to happen]

**Logs:**
[Paste relevant log output]

**Configuration:**
[Paste relevant configuration]
```

---

## 📚 Related Documentation

- **[Getting Started Guide](getting-started.md)** - Basic setup instructions
- **[Installation Guide](installation.md)** - Production deployment
- **[Configuration Guide](configuration.md)** - Detailed configuration options
- **[Metrics Reference](metrics.md)** - Available metrics documentation

---

Happy troubleshooting! 🔧
