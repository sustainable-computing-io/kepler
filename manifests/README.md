# Install Kepler on Kubernetes
```bash
kubectl apply -f deployment.yaml
```

# Install Kepler on OpenShift
## Cgroup V2 
If worker nodes don't have [Cgroup V2](https://docs.okd.io/latest/post_installation_configuration/machine-configuration-tasks.html#nodes-nodes-cgroups-2_post-install-machine-configuration-tasks) yet:
```bash
kubectl apply -f openshift-cgroupv2-workers.yaml
```
## Install Kepler
```bash
kubectl apply -f openshift-kernel-devel.yaml
kubectl apply -f openshift-scc.yaml
kubectl apply -f openshift-deployment.yaml
```
