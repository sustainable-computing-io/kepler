# Kepler on Kubernetes

## Prerequisites

The operating system must provide:
- Support for cgroup v2
- Provide the kernel headers (required by eBPF)
- Kernel with eBPF support

Consult the documentation of your Linux distribution on details for enabling these prerequisites.

### Build Manifests
  ```bash
  make build-manifest OPTS="<deployment options>"
  # minimum deployment: 
  # > make build-manifest
  # deployment with sidecar on openshift: 
  # > make build-manifest OPTS="ESTIMATOR_SIDECAR_DEPLOY OPENSHIFT_DEPLOY"
  ```

Deployment Option|Description|Dependency
---|---|---
BM_DEPLOY|baremetal deployment patched with node selector feature.node.kubernetes.io/cpu-cpuid.HYPERVISOR to not exist|-
OPENSHIFT_DEPLOY|patch openshift-specific attribute to kepler daemonset and deploy SecurityContextConstraints|-
PROMETHEUS_DEPLOY|patch prometheus-related resource (ServiceMonitor, RBAC role, rolebinding) |require prometheus deployment which can be OpenShift integrated or [custom deploy](https://github.com/sustainable-computing-io/kepler#deploy-the-prometheus-operator-and-the-whole-monitoring-stack)
CLUSTER_PREREQ_DEPLOY|deploy prerequisites for kepler on openshift cluster| OPENSHIFT_DEPLOY option set
CI_DEPLOY|update proc path for kind cluster using in CI|-
ESTIMATOR_SIDECAR_DEPLOY|patch estimator sidecar and corresponding configmap to kepler daemonset|-
MODEL_SERVER_DEPLOY|deploy model server and corresponding configmap to kepler daemonset|-
TRAIN_DEPLOY|patch online-trainer sidecar to model server| MODEL_SERVER_DEPLOY option set
DEBUG_DEPLOY|patch KEPLER_LOG_LEVEL for debugging|

 -  build-manifest requirements:
    -  kubectl v1.21+
    -  make
    -  go
 -  manifest sources and outputs will be in  `_output/generated-manifests` by default
## Installing Kepler on Kubernetes

Deploy kustomized manifest

 ```bash
 kubectl create -f _output/generated-manifests/deployment.yaml
 ```

# Kepler on OpenShift

The following steps have been tested with OpenShift 4.9.x and OpenShift 4.10.x.

## Prerequisites

***NOTE: THIS STEP ONLY NEEDS TO BE DONE ONCE AND ONLY IF THE CLUSTER IS NOT ALREADY CONFIGURED TO SUPPORT THE PREREQUISITES***

Kepler requires the nodes to support cgroup-v2 and kernel-devel extensions. In OpenShift this is done by enabling these capabilities using a MachineConfig (MC) manifest for the corresponding MachineConfigPool (MCP). The reference manifests enable these capabilities for the default `worker` and `master` MachineConfigPools.

- Create MachineConfig (MC) for the MachineConfigPools (MCPs)
```bash
# NOTE: The manifest must be built with CLUSTER_PREREQ_DEPLOY option
# If it is not built with BM_DEPLOY, the cgroupv2 installation will be also applied.
# WARNING: THIS WILL TRIGGER A ROLLING UPGRADE/REBOOT OF THE NODES
kubectl apply -k _output/generated-manifests/cluster-prereqs
```

- Wait for this step to be completed before trying to install and configure Kepler. You may track the progress with `kubectl get mcp` and `kubectl get nodes`.

## Installing Kepler on OpenShift

- Apply label `sustainable-computing.io/kepler=''` to nodes where you like to enable Kepler. Note: These nodes must be part of an MCP with the prerequisites in place.

```bash
# Example 1: enable kepler for all nodes in the cluster
kubectl label node --all sustainable-computing.io/kepler=''

# Example 2: enable kepler for a specific MCP (e.g. worker)
kubectl label node -l node-role.kubernetes.io/worker='' sustainable-computing.io/kepler=''

# Example 3: enable kepler for a specific node
kubectl label node worker1 sustainable-computing.io/kepler=''
```

- Deploying Kepler (namespace, scc, exporter, etc.) 
```bash
# NOTE: The manifest must be built with OPENSHIFT_DEPLOY option
# These manifest take care of creating the namespace, SCC and Kepler exporter
kubectl create -f _output/generated-manifests/deployment.yaml

# Note: During the initialization of `kepler-exporter` Pods or after rebooting the nodes, 
# it could take a while for the metrics to be stable.
```

Note: During the initialization of `kepler-exporter` Pods or after reboots, it could take few minutes for the metrics to be stable.

- For enabling the example dashbaord in OpenShift see [dashboard/README.md](config/dashboard/README.md)


## References
- Enabling [Cgroup V2](https://docs.okd.io/latest/post_installation_configuration/machine-configuration-tasks.html#nodes-nodes-cgroups-2_post-install-machine-configuration-tasks) in OpenShift
