# Kepler Helm Chart Updates and Rolling Deployments

This guide covers how to manage updates and rolling deployments for Kepler using Helm charts published to the OCI registry.

## Chart Repository

Kepler Helm charts are published to:

- **OCI Registry**: `oci://quay.io/sustainable_computing_io/charts/kepler`

## Installation Methods

### Direct OCI Installation (Recommended)

OCI registries cannot be added as traditional Helm repositories, so use direct installation:

```bash
# Install specific version
helm install kepler oci://quay.io/sustainable_computing_io/charts/kepler \
  --version 0.11.1 \
  --namespace kepler \
  --create-namespace

# Install latest version (omit --version)
helm install kepler oci://quay.io/sustainable_computing_io/charts/kepler \
  --namespace kepler \
  --create-namespace
```

## Manual Updates

### Check for New Versions

Since OCI registries don't support traditional repository browsing, check for new versions using these methods:

```bash
# Show chart information for specific version
helm show chart oci://quay.io/sustainable_computing_io/charts/kepler --version 0.11.2

# Check quay.io web interface for available tags
# Visit: https://quay.io/repository/sustainable_computing_io/charts?tab=tags

# Or use helm pull to check if version exists
helm pull oci://quay.io/sustainable_computing_io/charts/kepler --version 0.11.2 --dry-run
```

### Upgrade to Specific Version

```bash
# Upgrade to specific version
helm upgrade kepler oci://quay.io/sustainable_computing_io/charts/kepler --version 0.11.2 --namespace kepler

# Upgrade with custom values
helm upgrade kepler oci://quay.io/sustainable_computing_io/charts/kepler --version 0.11.2 --namespace kepler --values values.yaml

# Upgrade and wait for rollout to complete
helm upgrade kepler oci://quay.io/sustainable_computing_io/charts/kepler --version 0.11.2 --namespace kepler --wait --timeout=300s
```

### Upgrade to Latest Version

```bash
# Upgrade to latest (omit --version)
helm upgrade kepler oci://quay.io/sustainable_computing_io/charts/kepler --namespace kepler

# Verify upgrade
helm status kepler --namespace kepler
```

### Rollback if Needed

```bash
# List release history
helm history kepler --namespace kepler

# Rollback to previous version
helm rollback kepler --namespace kepler

# Rollback to specific revision
helm rollback kepler 2 --namespace kepler
```

## Update Strategies

### Conservative Updates (Recommended for Production)

Pin to specific patch versions and test before upgrading:

```bash
# Pin to specific version in production
helm upgrade kepler oci://quay.io/sustainable_computing_io/charts/kepler --version 0.11.1 --namespace kepler

# Test new version in staging first
helm install kepler-staging oci://quay.io/sustainable_computing_io/charts/kepler --version 0.11.2 --namespace kepler-staging

# After validation, upgrade production
helm upgrade kepler oci://quay.io/sustainable_computing_io/charts/kepler --version 0.11.2 --namespace kepler
```

## Monitoring Updates

### Check Update Status

```bash
# Watch deployment progress
kubectl rollout status daemonset/kepler -n kepler

# Check pod status
kubectl get pods -n kepler -w

# View recent events
kubectl get events -n kepler --sort-by='.lastTimestamp'
```

### Verify Metrics After Update

```bash
# Port forward to access metrics
kubectl port-forward -n kepler svc/kepler 28282:28282

# Test metrics endpoint
curl http://localhost:28282/metrics | grep kepler_build_info

# Check for expected metrics
curl -s http://localhost:28282/metrics | grep -E "(kepler_node_cpu_watts|kepler_container_cpu_watts)"
```

## Troubleshooting Updates

### Failed Updates

```bash
# Check release status
helm status kepler -n kepler

# View release history
helm history kepler -n kepler

# Check for pending pods
kubectl get pods -n kepler | grep -E "(Pending|ContainerCreating|CrashLoopBackOff)"

# View pod logs
kubectl logs -n kepler -l app.kubernetes.io/name=kepler --tail=100
```

### Recovery Procedures

```bash
# Rollback to previous working version
helm rollback kepler -n kepler

# Force recreation of DaemonSet if stuck
kubectl delete daemonset kepler -n kepler
helm upgrade kepler kepler/kepler --version 0.11.1 -n kepler

# Emergency: use source charts if OCI registry is unavailable
helm upgrade kepler manifests/helm/kepler/ -n kepler
```

## Version Compatibility

| Kepler Version | Kubernetes Version | Helm Version | Notes                |
|----------------|--------------------|--------------|----------------------|
| 0.11.x         | 1.20+              | 3.8+         | OCI registry support |
| 0.10.x         | 1.19+              | 3.0+         | Legacy installation  |

## Best Practices

1. **Test Updates**: Always test in staging environment first
2. **Gradual Rollouts**: Use rolling updates with conservative settings
3. **Monitor Metrics**: Verify metrics collection after updates
4. **Backup Values**: Keep your custom `values.yaml` in version control
5. **Version Pinning**: Pin specific versions in production
6. **Health Checks**: Configure proper readiness and liveness probes
7. **Alerts**: Set up monitoring for failed deployments

## Getting Help

- **Chart Issues**: [Kepler GitHub Issues](https://github.com/sustainable-computing-io/kepler/issues)
- **Registry Issues**: [Quay.io Support](https://access.redhat.com/support)
- **Helm Issues**: [Helm Documentation](https://helm.sh/docs/)
