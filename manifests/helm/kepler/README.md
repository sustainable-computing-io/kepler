# kepler

A Helm chart for Kepler - Kubernetes Efficient Power Level Exporter

## Install

```bash
helm install kepler oci://quay.io/sustainable_computing_io/charts/kepler \
  --namespace kepler \
  --create-namespace
```

## Grafana dashboards

The chart can create one ConfigMap per Kepler dashboard for the
[Grafana chart sidecar](https://github.com/grafana-community/helm-charts/tree/main/charts/grafana#sidecar-for-dashboards).
`kube-prometheus-stack` enables the sidecar by default.

```yaml
dashboards:
  enabled: true
```

The defaults match `kube-prometheus-stack` (`grafana_dashboard: "1"`, all namespaces).
For the standalone Grafana chart, match its sidecar settings:

```yaml
dashboards:
  enabled: true
  # Must match sidecar.dashboards.label / labelValue
  labels:
    grafana_dashboard: "import"
  # Needs sidecar.dashboards.folderAnnotation and provider.foldersFromFilesStructure
  annotations:
    grafana_folder: Kepler
  # Only if the sidecar doesn't set searchNamespace: ALL
  namespace: monitoring
```

<!-- markdown-table-prettify-ignore-start -->
## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| annotations | object | `{}` | Additional annotations to add to all resources |
| config.cpu.preferredMeters | list | `["rapl","hwmon"]` | Ordered preference list of CPU power-meter backends. The first backend that initializes successfully and reports zones is used. Built-in backends: rapl, hwmon, zpwr (IBM Z / LinuxONE), fake. zpwr reads /dev/diag, which reaches the container only when it is privileged: to use it, add zpwr here and set daemonset.securityContext.privileged=true (see docs/user/installation.md). |
| config.debug.pprof.enabled | bool | `false` | Turn on pprof endpoints |
| config.dev.fake-cpu-meter.enabled | bool | `false` | DEPRECATED: set cpu.preferredMeters: ["fake"] instead. Overrides cpu.preferredMeters and emits a deprecation warning when true. |
| config.dev.fake-cpu-meter.zones | list | `[]` | Fake CPU meter zones |
| config.dev.fake-gpu-meter.deviceCount | int | `1` | Number of fake GPUs |
| config.dev.fake-gpu-meter.enabled | bool | `false` | Use a fake GPU meter. For testing only. |
| config.dev.fake-gpu-meter.sharingMode | string | `"time-slicing"` | "exclusive" or "time-slicing" |
| config.experimental.gpu.dcgmEndpoint | string | `""` | dcgm-exporter metrics URL for MIG (auto-discovered if empty) |
| config.experimental.gpu.enabled | bool | `false` | Enable GPU power monitoring. Also forces privileged mode and privilege escalation, and adds the NVML init container. |
| config.experimental.gpu.idlePower | int | `0` | GPU idle power in Watts (0 = auto-detect) |
| config.experimental.hwmon.chipRules | list | `[]` | User-defined chip pairing rules (override/add to hardcoded defaults) |
| config.experimental.hwmon.forceEnabled | bool | `false` | DEPRECATED: set cpu.preferredMeters: ["hwmon"] instead. Emits a deprecation warning when true. |
| config.experimental.hwmon.zones | list | `[]` | List of zones to enable (default enable all) |
| config.experimental.platform.redfish.configFile | string | `"/etc/kepler/redfish.yaml"` | Path to Redfish BMC configuration file |
| config.experimental.platform.redfish.enabled | bool | `false` | Enable experimental Redfish BMC power monitoring |
| config.experimental.platform.redfish.httpTimeout | string | `"5s"` | HTTP client timeout for BMC requests |
| config.experimental.platform.redfish.nodeName | string | `""` | Node name to use (overrides Kubernetes node name and hostname fallback) |
| config.exporter.prometheus.debugCollectors | list | `["go"]` | Extra Go runtime collectors to expose |
| config.exporter.prometheus.enabled | bool | `true` | Expose Prometheus metrics |
| config.exporter.prometheus.metricsLevel | list | `["node","process","container","vm","pod"]` | Metric levels to export |
| config.exporter.stdout.enabled | bool | `false` | Print metrics to stdout |
| config.host.procfs | string | `"/host/proc"` | Path to the host procfs inside the container |
| config.host.sysfs | string | `"/host/sys"` | Path to the host sysfs inside the container |
| config.kube.config | string | `""` | Path to a kubeconfig. Empty uses the in-cluster config. |
| config.kube.enabled | bool | `false` | Ignored. The DaemonSet always passes `--kube.enable`. |
| config.kube.nodeName | string | `""` | Ignored. The DaemonSet sets it from the pod's node. |
| config.kube.podInformer.mode | string | `"kubelet"` | "kubelet" (default) or "apiserver" |
| config.kube.podInformer.pollInterval | string | `"15s"` | Poll interval for kubelet mode (default: 15s) |
| config.log.format | string | `"text"` | Log format: text or json |
| config.log.level | string | `"debug"` | Log level: debug, info, warn or error |
| config.monitor.interval | string | `"5s"` | How often Kepler refreshes power data |
| config.monitor.maxTerminated | int | `100` | Max terminated workloads kept in memory until exported. 0 turns it off. |
| config.monitor.minTerminatedEnergyThreshold | int | `10` | Minimum energy in joules for a terminated workload to be tracked |
| config.monitor.staleness | string | `"500ms"` | How long data is reused before it is recomputed |
| config.rapl.zones | list | `[]` | RAPL zones to enable. Empty enables the default zones. |
| config.web.configFile | string | `""` | Path to a TLS config file |
| config.web.listenAddresses | list | `[":28282"]` | Addresses to listen on |
| daemonset.affinity | object | `{}` | Affinity rules |
| daemonset.hostPID | bool | `true` | Use the host PID namespace so Kepler can see all processes |
| daemonset.hostUsers | bool | `true` | Use the host user namespace |
| daemonset.livenessProbe | object | `{"httpGet":{"path":"/probe/livez","port":"http"},"initialDelaySeconds":10,"periodSeconds":60}` | Liveness probe |
| daemonset.nodeSelector | object | `{}` | Node selector |
| daemonset.nvidia.driverPath | string | `"/run/nvidia/driver"` | Path on the host where the NVIDIA GPU Operator exposes the driver container root FS. Override if your GPU Operator uses a non-default path. |
| daemonset.nvidia.nvmlInitImage.repository | string | `"busybox"` | NVML init container image |
| daemonset.nvidia.nvmlInitImage.tag | string | `"1.36.1"` | NVML init container image tag |
| daemonset.priorityClassName | string | `""` | Priority class for the pods, e.g. `system-node-critical`. Empty uses the cluster default. |
| daemonset.readinessProbe | object | `{"httpGet":{"path":"/probe/readyz","port":"http"},"initialDelaySeconds":5,"periodSeconds":10}` | Readiness probe |
| daemonset.resources | object | `{}` | CPU and memory requests and limits |
| daemonset.securityContext.allowPrivilegeEscalation | bool | `false` | Allow privilege escalation. Forced on when `privileged` is true. |
| daemonset.securityContext.appArmorProfile | object | `{"type":"Unconfined"}` | AppArmor profile |
| daemonset.securityContext.capabilities | object | `{"add":["SYS_PTRACE"],"drop":["ALL"]}` | Linux capabilities |
| daemonset.securityContext.privileged | bool | `false` | Run privileged. Needed to name non-root, non-Kubernetes containers. Forced on when `config.experimental.gpu.enabled` is true. |
| daemonset.securityContext.readOnlyRootFilesystem | bool | `true` | Read-only root filesystem |
| daemonset.securityContext.runAsNonRoot | bool | `false` | Require a non-root user |
| daemonset.securityContext.runAsUser | int | `0` | User ID to run as |
| daemonset.securityContext.seLinuxOptions | object | `{"type":"spc_t"}` | SELinux options |
| daemonset.securityContext.seccompProfile | object | `{"type":"RuntimeDefault"}` | Seccomp profile |
| daemonset.startupProbe | object | `{}` | Startup probe. Off when empty. |
| daemonset.tolerations | list | `[{"effect":"NoSchedule","key":"node-role.kubernetes.io/control-plane","operator":"Exists"},{"effect":"NoSchedule","key":"node-role.kubernetes.io/master","operator":"Exists"}]` | Tolerations. The defaults let Kepler run on control plane nodes. |
| dashboards.annotations | object | `{}` | Extra annotations, e.g. `grafana_folder: kepler`. Folder annotations need `sidecar.dashboards.folderAnnotation` and `provider.foldersFromFilesStructure` in the Grafana chart. |
| dashboards.enabled | bool | `false` | Create a ConfigMap for each Kepler Grafana dashboard |
| dashboards.labels | object | `{"grafana_dashboard":"1"}` | Labels the Grafana sidecar selects on. Must match `sidecar.dashboards.label` (and `labelValue`, if set) in the Grafana chart. |
| dashboards.namespace | string | `""` | Namespace for the dashboard ConfigMaps (defaults to the release namespace). The Grafana chart sidecar watches only its own namespace unless `sidecar.dashboards.searchNamespace` is set, e.g. to `ALL`. |
| fullnameOverride | string | `""` | Override the full resource name |
| image.pullPolicy | string | `"IfNotPresent"` | Image pull policy |
| image.repository | string | `"quay.io/sustainable_computing_io/kepler"` | Kepler image repository |
| image.tag | string | `""` | Image tag. Uses the chart appVersion when empty. |
| imagePullSecrets | list | `[]` | Secrets for pulling images from a private registry, e.g. `[{name: my-registry-secret}]` |
| labels | object | `{}` | Additional labels to add to all resources |
| nameOverride | string | `""` | Override the chart name |
| namespace.create | bool | `false` | Create the namespace. When false, the release namespace is used. |
| namespace.name | string | `"kepler"` | Namespace name, used when `namespace.create` is true |
| networkPolicy.egress | list | `[]` | Egress rules. Empty blocks all egress. |
| networkPolicy.enabled | bool | `false` | Create a NetworkPolicy |
| networkPolicy.ingress | list | `[{"ports":[{"port":28282,"protocol":"TCP"}]}]` | Ingress rules |
| rbac.create | bool | `true` | Create the ClusterRole and ClusterRoleBinding |
| service.port | int | `28282` | Service port |
| service.protocol | string | `"TCP"` | Service protocol |
| service.targetPort | string | `"http"` | Target port on the pod |
| service.type | string | `"ClusterIP"` | Service type |
| serviceAccount.create | bool | `true` | Create a service account |
| serviceAccount.name | string | `"kepler"` | Service account name |
| serviceMonitor.annotations | object | `{}` | Extra annotations |
| serviceMonitor.enabled | bool | `false` | Create a ServiceMonitor |
| serviceMonitor.interval | string | `"5s"` | Scrape interval |
| serviceMonitor.labels | object | `{}` | Extra labels |
| serviceMonitor.path | string | `"/metrics"` | Metrics path |
<!-- markdown-table-prettify-ignore-end -->
