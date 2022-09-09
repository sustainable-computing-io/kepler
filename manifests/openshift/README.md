# Deploying Kepler with Grafana Dashboard on OpenShift

```bash
oc apply --kustomize $(pwd)/manifests/openshift/cluster-prereqs
# The cluster-prereqs modifies all nodes to enable cgroupsv2. This takes a long time
# Each node is decommissioned and rebooted - may take ~20 minutes.

# Check before proceeding that all nodes are Ready and Schedulable
oc get nodes

oc apply --kustomize $(pwd)/manifests/openshift/kepler
# Check that kepler pods are up and running before proceeding

# The following script applies the kustomize files in $(pwd)/manifests/openshift/dashboard
$(pwd)/manifests/openshift/dashboard/deploy-grafana.sh
```
