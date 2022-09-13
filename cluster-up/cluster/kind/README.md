# K8S in a Kind cluster

This folder serves as base to spin a k8s cluster up using [kind](https://github.com/kubernetes-sigs/kind). The cluster is completely ephemeral and is recreated on every cluster restart. 
The Kepler container is built on the local machine and is then pushed to a registry which is exposed at `localhost:5001` (we use port 5001 to work on [macOS](https://github.com/kubernetes-sigs/kind/pull/2621)). 

A kind cluster must specify:
* CLUSTER_NAME representing the cluster name (default: `kind`)
* IMAGE_REPO representing the image name with the local repository (default: `localhost:5001/kepler`)

The provider is supposed to copy a valid `kind.yaml` file under `cluster-up/cluster/${CLUSTER_PROVIDER}/manifests/kind.yaml`
