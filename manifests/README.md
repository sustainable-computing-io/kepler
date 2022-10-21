# Kepler on Kubernetes

## Prerequisites

The operating system must provide:
- Support for cgroup v2
- Provide the kernel headers (required by eBPF)
- Kernel with eBPF support

Consult the documentation of your Linux distribution on details for enabling these prerequisites.
## Installing Kepler on Kubernetes

Deploying the Kepler exporter as a daemonset to run on all nodes. The following deployment will also create a service listening on
port 9102.
```
# build manifests file for VM+Baremetal and Baremetal only
# manifests are created in  _output/manifests/kubernetes/generated/ by default
# kubectl v1.21.0 is minimum version that support build manifest
# make build-manifest
```

if you are running with Baremetal only
```
kubectl create -f _output/manifests/kubernetes/generated/bm/deployment.yaml
```

if you are running with Baremetal and/or VM
```
kubectl create -f _output/manifests/kubernetes/generated/vm/deployment.yaml
```

# Kepler on OpenShift

The following steps have been tested with OpenShift 4.9.x and OpenShift 4.10.x.

## Prerequisites

***NOTE: THIS STEP ONLY NEEDS TO BE DONE ONCE AND ONLY IF THE CLUSTER IS NOT ALREADY CONFIGURED TO SUPPORT THE PREREQUISITES***

Kepler requires the nodes to support cgroup-v2 and kernel-devel extensions. In OpenShift this is done by enabling these capabilities using a MachineConfig (MC) manifest for the corresponding MachineConfigPool (MCP). The reference manifests enable these capabilities for the default `worker` and `master` MachineConfigPools.

- Create MachineConfig (MC) for the MachineConfigPools (MCPs)
```bash
# WARNING: THIS WILL TRIGGER A ROLLING UPGRADE/REBOOT OF THE NODES
kubectl apply -k openshift/cluster-prereqs
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
# NOTE: These manifest take care of creating the namespace, SCC and Kepler exporter
kubectl apply -k openshift/kepler

# Note: During the initialization of `kepler-exporter` Pods or after rebooting the nodes, 
# it could take a while for the metrics to be stable.
```

Note: During the initialization of `kepler-exporter` Pods or after reboots, it could take few minutes for the metrics to be stable.

- For enabling the example dashbaord in OpenShift see [openshift/dashboard/README.md](openshift/dashboard/README.md)


## References
- Enabling [Cgroup V2](https://docs.okd.io/latest/post_installation_configuration/machine-configuration-tasks.html#nodes-nodes-cgroups-2_post-install-machine-configuration-tasks) in OpenShift
