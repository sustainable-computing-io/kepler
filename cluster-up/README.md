# How to use cluster-up

This directory provides the scripts to create a local kubernetes cluster to used for development or integration tests. The scripts are the source for the kepler cluster commands like `make cluster-up` and `make cluster-down`.

## Docker registry

There's a docker registry available which is exposed at `localhost:5001`.

## Choosing a cluster version

The env variable `CLUSTER_PROVIDER` tells keplerci what cluster version to spin up.

Therefore, before calling one of the make targets, the environment variable `CLUSTER_PROVIDER` must be exported to set the name of the tool that will create the kubernetes cluster. Currently, we only support the type `kind`. In the future, we will support other types such as `microshift`.

## Bringing the cluster up
```bash
export CLUSTER_PROVIDER=`kind`
make cluster-up
```

## Bringing the cluster down

```bash
export CLUSTER_PROVIDER=`kind`
make cluster-down
```
This destroys the whole cluster.

## Deploying kepler in the cluster

```bash
export CLUSTER_PROVIDER=`kind`
IMAGE_REPO="localhost:5001/kepler" IMAGE_TAG="devel" make cluster-sync
```
This removes a running kepler deployment, creates a new docker image, pushes it to the local cluster registry, deploys kepler with the newly created image, and waits for the kepler container to be in the running state.
