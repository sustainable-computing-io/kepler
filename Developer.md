# Contribute to Kepler

Welcome anyone contribute to Kepler.
May this guide help you with your 1st contribution.

1. Search your issue/contribute at [here](https://github.com/sustainable-computing-io/kepler/issues) to avoid duplication.
1. For any code contribution please read documents below carefully.
- [License](./LICENSE)
- [DCO](./DCO)

If you are good with our [License](./LICENSE) and [DCO](./DCO), then contents would help you with your 1st code contribution.

1. Fork Kepler
1. For any do code changes(as nit fix or feature implementation), we use ginkgo as test framework, please submit test case with feature code together.
1. For any new feature design, or feature level changes, please create an issue 1st, then submit a PR following document and steps [here](./enhancements/README.md) before code implementation.

## Unit test
We run go test base on specific build tag for different conditions.
Please don't break others build tag, otherwise you will see CI failure.

## Dev on Mac
Please notice that in kepler we use different build tags to adapt to different hardwares.
Please see keywords below for more details, as the code talks.
```
//go:build darwin
// +build darwin
or
if runtime.GOOS == "linux" {
```

## Commit your change by `git commit -s`

## Commit message
ref to https://www.kubernetes.dev/docs/guide/pull-requests/#commit-message-guidelines
we have 3 rules as commit messages check to ensure commit message is meaningful.
- Try to keep the subject line to 50 characters or less; do not exceed 72 characters
- Providing additional context if I am right means formatting in topic":" something
- The first word in the commit message subject should be capitalized unless it starts with a lowercase symbol or other identifier

For example:
```
Doc: update Developer.md
```

## CI test
our CI test has two phases for now
- unit test
- integration test

## Integration test
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

## Stale
We enabled [stale bot](https://github.com/probot/stale) for house keeping, an Issue or Pull Request becomes stale if no any inactivity for 60 days.
