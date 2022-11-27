# Deploying Kepler with Grafana Dashboard on OpenShift

If running OpenShift on virtual machines, Kepler requires cgroupsv2 - uncomment the
cgroupsv2 lines in manifests/openshift/cluster-prereqs/kustomization.yaml before proceeding.
The cgroupsv2 machineconfigs modifies all nodes to enable cgroupsv2.
Each node is decommissioned and rebooted - this may take ~20 minutes.

```bash
oc apply --kustomize $(pwd)/manifests/openshift/cluster-prereqs
# Check before proceeding that all nodes are Ready and Schedulable
oc get nodes

oc apply --kustomize $(pwd)/manifests/openshift/kepler
# Check that kepler pods are up and running before proceeding

# The following script applies the kustomize files in $(pwd)/manifests/openshift/dashboard
$(pwd)/manifests/openshift/dashboard/deploy-grafana.sh
```
