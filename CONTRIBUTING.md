# Contribute to Kepler

Welcome to the Kepler community and thank you for contributing to Kepler!
May this guide help you with your 1st contribution.

There are multiple ways to contribute, including new feature requests and implementations, bug reports and fixes, PR reviews, doc updates, refactoring, unit and integration tests, web design, etc.

1. Before opening a new issue or PR, search for any existing issues [here](https://github.com/sustainable-computing-io/kepler/issues) to avoid duplication.
2. For any code contribution, please read the documents below carefully:
   -  [License](./LICENSE)
   -  [DCO](./DCO)

If you are good with our [License](./LICENSE) and [DCO](./DCO), follow these steps to start with your 1st code contribution:
1. Fork & clone Kepler
2. We use [ginkgo](https://onsi.github.io/ginkgo/#getting-started) as a test framework. Please add units tests that cover your code changes.
3. For any new feature design, or feature level changes, please create an issue 1st, then submit a PR following document and steps [here](./enhancements/README.md) before code implementation.

Once you are ready to start working on an issue, follow the steps to set up your [local development environment](#local-development-environment).

Here is a checklist for when you are ready to open a Pull Request:
1. Add [unit tests](#unit-tests) that cover your changes
2. Ensure that all unit tests are successful
3. Run the [integration tests](#integration-tests) locally
4. [Sign](#signed-commits) your commits
5. [Format](#commit-messages) your commit messages

Once a PR is open, Kepler [reviewers](./Contributors.md) will review the PR. Thank you for contributing to Kepler!

## Local Development Environment
To set up a development environment, please follow the steps [here](./doc/dev/README.md).

### MacOS
Please notice that in Kepler we use different build tags to adapt to different hardwares.
Please see the keywords below for more details:
```
//go:build darwin
// +build darwin
or
if runtime.GOOS == "linux" {
```

## Testing

### CI Tests
The Kepler CI tests perform the following checks:
- [unit tests](./.github/workflows/unit_test.yml)
- [integration tests](./.github/workflows/integration_test.yml)

### Unit Tests
We run Go tests based on specific build tags for different conditions.
Please don't break other build tags, otherwise CI may fail.

To run the unit tests:
```
make test
```

For MacOS, use the following command instead:
```
make test-mac-verbose
```

### Integration Tests
Integration tests should be based on the miminal scope of a unit test needed to succeeded.

The GitHub Actions workflow for integration tests and in-depth steps can be found [here](./.github/workflows/integration_test.yml). The end-to-end testing suite can be found [here](./e2e/).

The logic is as follows:
- Ensure Kepler requirements are met e.g. kernel headers are installed.
- Download required tools, such as kubectl.
- Build kepler image with specific PR code.
----------------------------------------------------------------
Based on different k8s cluster, for example, [kind](https://kind.sigs.k8s.io/):
- Start kind cluster locally with a local image registry.
- Upload kepler image build by specific PR code to local image registry for kind cluster.
--------------------------------------------------------------------------------
Back to common checking process:
- Deploy Kepler to a local kind cluster with image stored at local image registry.
- Use kubectl to check that the change was applied as intended.
  
## Benchmark
steps:
```
go test -cpuprofile cpu.prof -memprofile mem.prof -bench .
pprof -http=":8091" ./cpu.prof
```
ref https://dave.cheney.net/2014/06/07/five-things-that-make-go-fast

## Sign Commits

Please sign and commit your changes with `git commit -s`. More information on how to sign commits can be found [here](https://docs.github.com/en/authentication/managing-commit-signature-verification/signing-commits).

## Commit Messages
Please refer to the Kubernetes commit message guidelines that can be found [here](https://www.kubernetes.dev/docs/guide/pull-requests/#commit-message-guidelines).

We have 3 rules as commit messages check to ensure a commit message is meaningful:
- Try to keep the subject line to 50 characters or less; do not exceed 72 characters
- Providing additional context with the following formatting: `<topic>: <something>`
- The first word in the commit message subject should be capitalized unless it starts with a lowercase symbol or other identifier

For example:
```
Doc: update Developer.md
```

## Stale Issues
We enabled [stale bot](https://github.com/probot/stale) for house keeping. An Issue or Pull Request becomes stale if no any inactivity for 60 days.
