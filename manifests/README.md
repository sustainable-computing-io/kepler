# Prerequisites

The operating system must provide:

- Kernel with eBPF support

Consult the documentation of your Linux distribution on details for enabling prerequisites.

## Build Manifests

```bash
make build-manifest OPTS="<deployment options>"
# minimum deployment:
# > make build-manifest
# deployment with sidecar on openshift:
# > make build-manifest OPTS="ESTIMATOR_SIDECAR_DEPLOY OPENSHIFT_DEPLOY"
```

| Deployment Option        | Description                                                                                                  | Dependency                                                                                                                                                                                           |
| ------------------------ | ------------------------------------------------------------------------------------------------------------ | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| BM_DEPLOY                | baremetal deployment patched with node selector feature.node.kubernetes.io/cpu-cpuid.HYPERVISOR to not exist | -                                                                                                                                                                                                    |
| OPENSHIFT_DEPLOY         | patch openshift-specific attribute to kepler daemonset and deploy SecurityContextConstraints                 | -                                                                                                                                                                                                    |
| PROMETHEUS_DEPLOY        | patch prometheus-related resource (ServiceMonitor, RBAC role, rolebinding)                                   | require prometheus deployment which can be OpenShift integrated or [custom deploy](https://github.com/sustainable-computing-io/kepler#deploy-the-prometheus-operator-and-the-whole-monitoring-stack) |
| CI_DEPLOY                | update proc path for kind cluster using in CI                                                                | -                                                                                                                                                                                                    |
| ESTIMATOR_SIDECAR_DEPLOY | patch estimator sidecar and corresponding configmap to kepler daemonset                                      | -                                                                                                                                                                                                    |
| MODEL_SERVER_DEPLOY      | deploy model server and corresponding configmap to kepler daemonset                                          | -                                                                                                                                                                                                    |                                                                                                                                                                     |
| DEBUG_DEPLOY             | patch KEPLER_LOG_LEVEL for debugging                                                                         |
| QAT_DEPLOY               | update proc path for Kepler to enable accelerator QAT                                                        | Intel QAT installed                                                                                                                                                                                  |

- build-manifest requirements:
  - kubectl v1.21+
  - make
  - go
- manifest sources and outputs will be in `_output/generated-manifest` by default

# Kepler on Kubernetes

## Installing Kepler on Kubernetes

Deploying Kepler (namespace, exporter, etc.)

```bash
# NOTE: The manifest must be built with CI_DEPLOY option
kubectl create -f _output/generated-manifest/deployment.yaml
```

# Kepler on OpenShift

The following steps have been tested with OpenShift 4.12.x onwards.

## Installing Kepler on OpenShift

- Deploying Kepler (namespace, scc, exporter, etc.)

```bash
# NOTE: The manifest must be built with OPENSHIFT_DEPLOY option
kubectl create -f _output/generated-manifest/deployment.yaml
```

- For enabling the example dashbaord in OpenShift see [dashboard/README.md](config/dashboard/README.md)
