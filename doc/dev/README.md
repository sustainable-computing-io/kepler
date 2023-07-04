# Getting Started

A quick start guide to get Kepler up and running inside your container-based development cluster.

## Create a new ephemeral local kubernetes cluster

Use `make cluster-up` to setup a local development cluster running in Kind.

The make target `cluster-up` works by cloning [local-dev-cluster repo](https://github.com/sustainable-computing-io/local-dev-cluster)
locally and using the scripts in the repo to setup a Kubernetes running locally
using `Kind`.

**NOTE**: Considering that your local environment is different to CI, we strongly
recommend that you check [prerequisites](https://github.com/sustainable-computing-io/local-dev-cluster#pre-request)
and [start up](https://github.com/sustainable-computing-io/local-dev-cluster#start-up) to
customize your own local development environment.

You can find technical discussions within the community regarding this topic by
following [this enhancements propsal](../../enhancements/CICDv1.md) and
[related issue](https://github.com/sustainable-computing-io/kepler/issues/721).

## Deploying kepler in the cluster
```bash
export CLUSTER_PROVIDER='kind'
IMAGE_REPO="localhost:5001" IMAGE_TAG="devel" make cluster-sync
```

Make target `cluster-sync` does the following
  * removes any running kepler deployment
  * creates a new docker image
  * pushes it to the local cluster registry
  * deploys kepler with the newly created image
  * waits for the kepler container to be in the running state.


## Build and run kepler on your cluster

First, point the `Makefile` to the container registry of your choice:

```bash
export IMAGE_REPO=index.docker.io/myrepo
export IMAGE_TAG=mybuild
```

By default, please use:
```bash
export IMAGE_REPO="localhost:5001"
export IMAGE_TAG="devel"
```

Assuming that you have logged in the container registry pointed above,
point the `Makefile` to cluster provider to build the right manifests:
```bash
export CLUSTER_PROVIDER=kubernetes
```

By default we use the IMAGE_TAG=`devel` and CLUSTER_PROVIDER=`kubernetes`

After that, build the manifests (remove OPTS="ROOTLESS" if you want to run kepler with privileged setup):
```bash
OPTS="ROOTLESS" make build-manifest
```

Then build images:
```bash
make build_containerized
make push-image
```

If successful, the manifests are at `_output/manifest/$CLUSTER_PROVIDER/`

Finally, push the manifests to your cluster:
```bash
make cluster-deploy
```

Or just simply build and deploy with:
```bash
make cluster-sync
```

## To run kepler externally to the cluster

This quick tutorial is for developing and testing Kepler locally but with access to kubelet

### Install bcc-devel and kernel-devel

Refer to the [builder Dockerfile](https://github.com/sustainable-computing-io/kepler/blob/main/build/Dockerfile.builder)

### Compile
Go to the root of the repo and do the following:

```bash
 make _build_local
```

If successful, the binary is at `_output/bin/_/kepler`

### Test

Create the k8s role and token, copy data files, this is only needed once.
```bash
cd dev/
./create_k8s_token.sh
./prepare_dev_env.sh
```

Then run the Kepler binary at `_output/bin/_/kepler`

Kepler metrics are available under <host_ip>:8888/metrics by default

## Build kepler and base multi-arch images
```bash
./hack/build-images.sh help
```
