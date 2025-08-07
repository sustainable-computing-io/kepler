# Helm Chart Development Guide

This guide covers how to develop, test, and maintain the Kepler Helm chart.

## Chart Structure

```text
manifests/helm/
├── cr.yaml                    # Chart-releaser configuration
└── kepler/
    ├── Chart.yaml            # Chart metadata
    ├── values.yaml           # Default values
    ├── ci/
    │   └── test-values.yaml  # CI test values
    └── templates/
        ├── _helpers.tpl      # Template helpers
        ├── configmap.yaml    # Kepler configuration
        ├── daemonset.yaml    # Main workload
        ├── namespace.yaml    # Optional namespace
        ├── networkpolicy.yaml # Network security
        ├── rbac.yaml         # RBAC resources
        ├── service.yaml      # Service exposure
        └── servicemonitor.yaml # Prometheus monitoring
```

## Development Workflow

### 1. Setup Development Environment

```bash
# Install Helm (if not already installed)
curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash

# Verify installation
helm version

# Clone the repository
git clone https://github.com/sustainable-computing-io/kepler.git
cd kepler
```

### 2. Chart Development

#### Validating Changes

```bash
cd manifests/helm

# Lint the chart for syntax errors
helm lint kepler

# Test template rendering
helm template test-release kepler --debug

# Test with CI values
helm lint kepler -f kepler/ci/test-values.yaml
helm template test-ci kepler -f kepler/ci/test-values.yaml
```

#### Testing Chart Installation

```bash
# Create a test namespace
kubectl create namespace kepler-test

# Install chart
helm install kepler-test kepler -n kepler-test

# Check status
helm status kepler-test -n kepler-test
kubectl get all -n kepler-test

# Test with custom values
helm install kepler-ci kepler -f kepler/ci/test-values.yaml -n kepler-test

# Clean up
helm uninstall kepler-test -n kepler-test
helm uninstall kepler-ci -n kepler-test
kubectl delete namespace kepler-test
```

### 3. Using Kind for Testing

```bash
# Create a kind cluster
kind create cluster --name kepler-dev

# Install chart
helm install kepler kepler -n kepler --create-namespace

# Port forward for testing
kubectl port-forward -n kepler svc/kepler 28282:28282

# Test metrics endpoint
curl http://localhost:28282/metrics

# Clean up
kind delete cluster --name kepler-dev
```

## Chart Testing

### CI Testing

The `ci/test-values.yaml` file contains test configurations that validate:

```yaml
# Test enhanced features
serviceMonitor:
  enabled: true        # Prometheus monitoring
networkPolicy:
  enabled: true        # Network security
daemonset:
  resources:           # Resource constraints
    limits:
      cpu: 500m
      memory: 512Mi
    requests:
      cpu: 100m
      memory: 128Mi
labels:
  test: "ci"          # Custom labels
annotations:
  test.io/purpose: "ci-validation"  # Custom annotations
```

### Automated Testing with chart-testing

```bash
# Install chart-testing (ct)
curl -sSL https://github.com/helm/chart-testing/releases/download/v3.7.1/chart-testing_3.7.1_linux_amd64.tar.gz | tar xz
sudo mv ct /usr/local/bin/

# Lint charts
ct lint --config .github/ct.yaml

# Install and test charts
ct install --config .github/ct.yaml
```

## Chart Configuration

### Key Values Structure

```yaml
# Global settings
labels: {}                    # Applied to all resources
annotations: {}               # Applied to all resources

# Image configuration
image:
  repository: quay.io/sustainable_computing_io/kepler
  tag: ""                     # Uses Chart.yaml appVersion
  pullPolicy: IfNotPresent

# Service configuration
service:
  type: ClusterIP
  port: 28282

# DaemonSet configuration
daemonset:
  securityContext:
    privileged: true          # Required for hardware access
  nodeSelector:
    kubernetes.io/os: linux
  tolerations:
    - effect: NoSchedule
      key: node-role.kubernetes.io/control-plane
  resources: {}
  livenessProbe:
    httpGet:
      path: /metrics
      port: http
    initialDelaySeconds: 10
    periodSeconds: 60

# RBAC
rbac:
  create: true

serviceAccount:
  create: true
  name: kepler

# Monitoring
serviceMonitor:
  enabled: false
  interval: 30s

# Security
networkPolicy:
  enabled: false

# Kepler configuration
config:
  log:
    level: debug
  host:
    sysfs: /host/sys
    procfs: /host/proc
  monitor:
    interval: 5s
  web:
    listenAddresses:
      - :28282
```

### Template Functions

The chart uses these helper functions defined in `_helpers.tpl`:

```yaml
# Resource naming
{{- include "kepler.name" . }}           # Chart name
{{- include "kepler.fullname" . }}       # Full resource name
{{- include "kepler.serviceAccountName" . }} # Service account name

# Labels and selectors
{{- include "kepler.labels" . }}         # Standard labels
{{- include "kepler.selectorLabels" . }} # Pod selector labels

# Namespace
{{- include "kepler.namespace" . }}      # Target namespace
```

## Release Process

### Chart Versioning

- **Chart Version**: Semantic versioning in `Chart.yaml`
- **App Version**: Kepler version being packaged
- **Image Tag**: Defaults to `appVersion` if not specified

```yaml
# Chart.yaml
apiVersion: v2
name: kepler
version: 1.0.0        # Chart version
appVersion: "v0.8.0"  # Kepler version
```

### Publishing (Future)

When the Helm repository is set up:

1. **Automated**: Releases trigger chart publishing via GitHub Actions
2. **Manual**: Use chart-releaser for manual releases

```bash
# Manual chart release (future)
cr package
cr upload
cr index
```

## Best Practices

### 1. Template Development

```yaml
# Use consistent indentation
{{- with .Values.someValue }}
  key: {{ . | quote }}
{{- end }}

# Handle empty values gracefully
{{- if .Values.optional.setting }}
optional_setting: {{ .Values.optional.setting }}
{{- end }}

# Use proper type assertions
{{- if .Values.enabled | default false }}
```

### 2. Values Organization

- Group related values under common keys (`daemonset`, `service`, etc.)
- Provide sensible defaults
- Document all values with comments
- Use consistent naming conventions

### 3. Resource Management

```yaml
# Always include resource limits for production
daemonset:
  resources:
    limits:
      memory: 512Mi
      cpu: 200m
    requests:
      memory: 256Mi
      cpu: 100m
```

### 4. Security Considerations

```yaml
# Minimize required privileges
daemonset:
  securityContext:
    privileged: true     # Only when necessary
    runAsNonRoot: false  # Document why root is needed

# Use network policies when available
networkPolicy:
  enabled: false  # Let users opt-in
```

## Troubleshooting

### Common Issues

1. **Template Rendering Errors**

   ```bash
   helm template test kepler --debug
   # Look for template syntax errors
   ```

2. **Value Type Mismatches**

   ```bash
   # Check value types in templates
   {{- if kindIs "string" .Values.someValue }}
   ```

3. **RBAC Permission Issues**

   ```bash
   kubectl auth can-i get pods --as=system:serviceaccount:kepler:kepler
   ```

### Debugging Commands

```bash
# Dry run installation
helm install kepler kepler --dry-run --debug

# Get generated manifests
helm get manifest kepler -n kepler

# Compare value differences
helm diff upgrade kepler kepler -f new-values.yaml

# Validate against Kubernetes API
helm install kepler kepler --dry-run --validate
```

## Contributing

### Making Changes

1. **Test Locally**: Always test changes with `helm lint` and `helm template`
2. **Update CI Values**: Modify `ci/test-values.yaml` if adding new features
3. **Document Changes**: Update this guide and user documentation
4. **Version Appropriately**: Follow semantic versioning for chart changes

### Pull Request Checklist

- [ ] Chart lints successfully
- [ ] Templates render correctly
- [ ] CI values test new features
- [ ] Documentation updated
- [ ] Breaking changes noted
- [ ] Version bumped appropriately

## Resources

- **Helm Documentation**: <https://helm.sh/docs/>
- **Chart Best Practices**: <https://helm.sh/docs/chart_best_practices/>
- **Chart Testing**: <https://github.com/helm/chart-testing>
- **Kepler Documentation**: <https://sustainable-computing.io/kepler/>
