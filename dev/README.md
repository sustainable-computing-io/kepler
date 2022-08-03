# Getting Started

A quick start guide to get Kepler up and running inside your container-based development cluster.

## I just want it built and run it on my cluster

First, point the `Makefile` to the container registry of your choice:

```bash
export IMAGE_REPO=index.docker.io/myrepo
export IMAGE_TAG=mybuild
```

We assume that you have logged in your container registry

Then point the `Makefile` to cluster provider to build the right manifests:
```bash
export CLUSTER_PROVIDER=kubernetes
```

By default we use the IMAGE_TAG=`devel` and CLUSTER_PROVIDER=`kubernetes`

After that, build the manifests and images:
```bash
make build-manifest
make _build_containerized
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

Create the k8s role and token, this is only needed once.
```bash
cd dev/
./create_k8s_token.sh
```

Then run the Kepler binary at `_output/bin/_/kepler`