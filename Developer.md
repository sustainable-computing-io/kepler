# CI test
our CI test has two phases for now
- unit test
- integration test

## unit test
We run go test base on specific build tag for different conditions.

## integration test
It should base on the miminal scope of unit test succeeded.
the logic is
- download tools as kubectl.
- build kepler image with specific pr code.
----------------------------------------------------------------
base on different k8s cluster, for example kind(k8s in docker)
- start KIND cluster locally with a local image registry.
- upload kepler image build by specific pr code to local image registry for kind cluster.
--------------------------------------------------------------------------------
back to common checking process
- deploy a kepler at local kind cluster with image stored at local image registry.
- check via kubectl command for ...
