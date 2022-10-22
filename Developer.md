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

## Dev on Mac
Please notice that in kepler we use different build tags to adapt to different hardwares.
Please see keywords below for more details, as the code talks.
```
//go:build darwin
// +build darwin
or
if runtime.GOOS == "linux" {
```

## Commit message
ref to https://www.kubernetes.dev/docs/guide/pull-requests/#commit-message-guidelines
we have 3 rules as commit messages check to ensure commit message is meaningful.
- Try to keep the subject line to 50 characters or less; do not exceed 72 characters
- Providing additional context if I am right means formatting in topic":" something
- The first word in the commit message subject should be capitalized unless it starts with a lowercase symbol or other identifier
