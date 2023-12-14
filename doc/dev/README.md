# Getting Started

<!--toc:start-->
- [Getting Started](#getting-started)
  - [Create a new ephemeral local kubernetes cluster](#create-a-new-ephemeral-local-kubernetes-cluster)
  - [Build and run kepler on your cluster](#build-and-run-kepler-on-your-cluster)
  - [To run kepler externally to the cluster](#to-run-kepler-externally-to-the-cluster)
    - [Install bcc-devel and kernel-devel](#install-bcc-devel-and-kernel-devel)
    - [Compile](#compile)
    - [Test](#test)
  - [Build kepler and base multi-arch images](#build-kepler-and-base-multi-arch-images)
<!--toc:end-->

A quick start guide to get Kepler up and running inside your container-based development cluster.

## Create a new ephemeral local kubernetes cluster

Use `make cluster-up` to setup a local development cluster running in Kind.

The make target `cluster-up` works by cloning [local-dev-cluster repo](https://github.com/sustainable-computing-io/local-dev-cluster)
locally and using the scripts in the repo to setup a Kubernetes running locally
using `Kind`.

**NOTE**: Considering that your local environment is different to CI, we strongly
recommend that you check [prerequisites](https://github.com/sustainable-computing-io/local-dev-cluster#prerequisites)
and [start up](https://github.com/sustainable-computing-io/local-dev-cluster#startup) to
customize your own local development environment.

You can find technical discussions within the community regarding this topic by
following [this enhancements proposal](../../enhancements/CICDv1.md) and
[related issue](https://github.com/sustainable-computing-io/kepler/issues/721).

## Build and run Kepler on your cluster

```bash
make cluster-deploy
```

Make target `cluster-deploy` does the following

- Build the necessary manifests.
- Build and Push the Kepler image to local registry.
- Deploys Kepler with the newly created image.
- Validate Kepler installation.

If you want to use container registry of your choice:

```bash
make cluster-deploy IMAGE_REPO=index.docker.io/myrepo IMAGE_TAG=mybuild NO_BUILD=true
```

If you want to run Kepler with privileged setup:

```bash
make cluster-deploy IMAGE_REPO=index.docker.io/myrepo IMAGE_TAG=mybuild OPTS=ROOTLESS NO_BUILD=true
```

## To run Kepler externally to the cluster

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
