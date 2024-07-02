# Getting Started

<!--toc:start-->
- [Getting Started](#getting-started)
  - [Pre-requisites](#pre-requisites)
  - [Create a new ephemeral local kubernetes cluster](#create-a-new-ephemeral-local-kubernetes-cluster)
  - [Build and run kepler on your cluster](#build-and-run-kepler-on-your-cluster)
  - [To run kepler externally to the cluster](#to-run-kepler-externally-to-the-cluster)
    - [Install bcc-devel and kernel-devel](#install-bcc-devel-and-kernel-devel)
    - [Compile](#compile)
    - [Test](#test)
  - [Build kepler and base multi-arch images](#build-kepler-and-base-multi-arch-images)
<!--toc:end-->

A quick start guide to get Kepler up and running.

## Pre-requisites

This guide assumes you have the following installed:

- [Docker](https://docs.docker.com/get-docker/) or [Podman](https://podman.io/getting-started/installation)
- [Go](https://golang.org/doc/install)
- [Kind](https://kind.sigs.k8s.io/docs/user/quick-start/)

In order to make contributions to Kepler, you need to have the following installed:

- [Pre-commit](https://pre-commit.com/#install)

You can install pre-commit by running the following command:

```bash
pip install pre-commit
```

After installing pre-commit, you need to install the pre-commit hooks by running the following command:

```bash
pre-commit install
```

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
### Profiling Kepler

`kepler` exposes [standard go profiling](https://pkg.go.dev/net/http/pprof)
endpoint at `/debug/pprof/profile`. Following examples show how to
profile kepler with `go tool pprof`.

**NOTE:** These examples assume `kepler` is running (or
port-forwarded in case of k8s) on `localhost:8888`.

Detailed  information about using pprof can be found in their
[documentation](https://github.com/google/pprof/tree/main/doc).

#### CPU Profiling

```bash
go tool pprof 'http://localhost:8888/debug/pprof/profile?seconds=30'
```

#### Memory (heap) Profiling

```bash
go tool pprof 'http://localhost:8888/debug/pprof/heap?seconds=30'
```

#### Visualizing pprof

`-http` option can be used to visualize existing pprof in

```bash
go tool pprof -http 0.0.0.0:8000 \
	'http://localhost:8888/debug/pprof/heap?seconds=30'

```
or

```bash
go tool pprof -http 0.0.0.0:8000 <path/to/pprof-capture>.pb.gz

```

## Logging Kepler with Klog
While logging Kepler with Klog, please follow the guidelines from this [best practice article](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-instrumentation/logging.md).

Highlights of the guidelines are in the following:

### What method to use?

* `klog.ErrorS` - Errors should be used to indicate unexpected behaviours in code, like unexpected errors returned by subroutine function calls.
Logs generated by `ErrorS` command may be enhanced with additional debug information (by logging library). Calling `ErrorS` with `nil` as error may be acceptable if there is error condition that deserves a stack trace at this origin point.

* `klog.InfoS` -  Structured logs to the INFO log. `InfoS` should be used for routine logging. It can also be used to log warnings for expected errors (errors that can happen during routine operations).
  Depending on log severity it's important to pick a proper verbosity level to ensure that consumer is neither under nor overwhelmed by log volume:
  * `klog.V(0).InfoS` = `klog.InfoS` - Generally useful for this to **always** be visible to a cluster operator
    * Programmer errors
    * Logging extra info about a panic
    * CLI argument handling
  * `klog.V(1).InfoS` - A reasonable default log level if you don't want verbosity.
    * Information about config (listening on X, watching Y)
    * Errors that repeat frequently that relate to conditions that can be corrected (pod detected as unhealthy)
  * `klog.V(2).InfoS` - Useful steady state information about the service and important log messages that may correlate to significant changes in the system.  This is the recommended default log level for most systems.
    * Logging HTTP requests and their exit code
    * System state changing (killing pod)
    * Controller state change events (starting pods)
    * Scheduler log messages
  * `klog.V(3).InfoS` - Extended information about changes
    * More info about system state changes
  * `klog.V(4).InfoS` - Debug level verbosity
    * Logging in particularly thorny parts of code where you may want to come back later and check it
  * `klog.V(5).InfoS` - Trace level verbosity
    * Context to understand the steps leading up to errors and warnings
    * More information for troubleshooting reported issues

As per the comments, the practical default level is V(2). Developers and QE
environments may wish to run at V(3) or V(4). If you wish to change the log
level, you can pass in `-v=X` where X is the desired maximum level to log.
