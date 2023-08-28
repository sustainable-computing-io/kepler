---
title: Kepler CI/CD pipeline maintenance
authors:
  - Sam Yuan
reviewers:
  - n/A, though github
approvers:
  - n/A, though github
creation-date: 2023-05
last-updated: 2023-05
tracking-links: # link to related GitHub issues
  - https://github.com/orgs/sustainable-computing-io/projects/2/views/1?pane=issue&itemId=28750581
---

# Kepler CI/CD pipeline maintenance

## Summary

As we onboarded with CNCF sandbox, now it's time to review and open our mind on CI/CD pipeline.
For example, 
- Open to cloud service provider's contribute with k8s environment and we run our CI/CD pipeline on it.
- Open to hardware provider integrate kepler to verify kepler running on specific hardware well or not. Further see discussion at [#711](https://github.com/sustainable-computing-io/kepler/pull/711).
- To quick response to CI/CD breaks due to github action agent upgrade as cases happens in the past below:
  - github action agent upgrade, some network tips updated for container runtime. we updated kind version to fix it.
  - github action agent upgrade with their own kernel, conflicts with our hard code. we fixed up in code.

Hence, as infrastructure as code, we have motivations to make our CI/CD pipeline more flexible to face those challenges.

## Motivation

- Support development branch and stable branch for pipeline.
> For either development kepler or hardware provide verification usage.
- Quick response for github agent update.(or quick response for CI/CD breaking related with github agent)
- Quick update version of tools we used, for example kind/kubectl. 
> We should keep update our tooling version to avoid using any duplicated version.
- Flexible the k8s cluster adoption for micro shift/minikube or others.
> For could provider usage.

### Goals

- Keep today's feature as
1. Start up a k8s cluster with local development usage.
1. Start up a k8s cluster during pipeline as github action usage on github agent.

- Support version update for tooling as KIND.
- Support direct deploy on an existing k8s cluster.
- Support clean up local cluster.
- Support working branch and stable branch by Tag.
- Build up a mechanism to update version of CI/CD pipeline, with release cycle.

### Non-Goals

- Switch to other CI/CD tool. 
**Note**: no matter which CI/CD tool we are using, the infrastructure as code keep the same. Hence integration with specific CI/CD tool is no goal by treated as just invoke our infrastructure as code.

## Proposal

### Workflow Description
- In short term, we update current pipeline with Tag v0.0.0 base on today's version.
- For new features as
  - Add versioning tool support.
  - Add decouple k8s deployment.
  - Support clean up local cluster.
  - ...
- Once 0.6 release or in a specific lifecycle, create a new tag, for pipeline.
- Update pipeline with new tag.

### Implementation Details/Notes/Constraints [optional]
n/A, keep open at implementation details

### Risks and Mitigations
n/A

### Drawbacks
n/A

## Design Details
n/A, keep open at implementation details

### Open Questions [optional]
The frequence of version bump up for our tooling.

### Test Plan
Covered in [local-dev-cluster](https://github.com/sustainable-computing-io/local-dev-cluster)
and [github-action](https://github.com/sustainable-computing-io/kepler-action) for example checking cluster running. And as infrastructure as code, integration test should be covered.

## Implementation History
- [local-dev-cluster](https://github.com/sustainable-computing-io/local-dev-cluster)
- [github-action](https://github.com/sustainable-computing-io/kepler-action)

## Alternatives
n/A

## Infrastructure Needed [optional]
n/A
