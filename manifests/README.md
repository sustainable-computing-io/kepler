# Install Kepler on Kubernetes
```bash
kubectl apply -f kubernetes/deployment.yaml
```

# Install Kepler on OpenShift

Note: The installation will enable cgroup-v2 and kernel-devel extensions for `worker` and `master` MachineConfigPools 

## Install Kepler
```bash
kubectl apply -k openshift
```


## References
- Enabling [Cgroup V2](https://docs.okd.io/latest/post_installation_configuration/machine-configuration-tasks.html#nodes-nodes-cgroups-2_post-install-machine-configuration-tasks) in OpenShift
