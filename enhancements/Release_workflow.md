---
title: Kepler dev and release schedule
authors:
  - Sam Yuan
reviewers:
  - n/A, though github
approvers:
  - n/A, though github
creation-date: 2023-07
last-updated: 2023-07
tracking-links: # link to related GitHub issues
  - https://github.com/sustainable-computing-io/kepler/pull/759
  - https://github.com/sustainable-computing-io/kepler-action/issues/50
  - https://github.com/sustainable-computing-io/local-dev-cluster/issues/22
  - https://github.com/sustainable-computing-io/kepler/pull/760
---

# Kepler dev and release schedule

## Summary

At Jul 2023, during development of [CICDv1](./CICDv1.md) we found we'd better have a release schedule as guidance to handle breaking changes between repos. Major for our (customer github action)[https://github.com/sustainable-computing-io/kepler-action], but also for breaking changes in kepler which may influence kepler-operator, model server and other repos.

## Motivation

This document is created for our next steps integration works, as in [CICDv1](./CICDv1.md) we discussed about flexible pipelines with paramterable to reduce the influence of version bump up. In this rfc, we are going to discuss how to make CI, dashboard, bi-weekly meeting together.

### Goals

1. Create a pre release page as enhancement of this [PR](https://github.com/sustainable-computing-io/kepler/pull/760).
1. To further considering reducing CVE by automatic build, enhance CI to support build latest code on default branch and latest release branch bi weekly? Hence provide something as security patch bi weekly.
1. To avoid breaking changes in customer CI breaks other Repos. Make a schedule for customer CI and other repos release.

### Non-Goals

1. Self host github action integration. Considering with privilege issue on self hosted BM... mark out of scope. 
1. Test scope as metric of OS, CPU arch, k8s platform is collecting in our google doc as meeting minutes, and some OS/CPU arch may need to test manually or by other CI tooling support, hence mark it out of scope today.

## Proposal

As we have kepler release in each 3 months.

| Timeline | Action |
|---| --- | 
| 1~2 month | kepler-action development, maybe just bump version in regular |
| 2 month | kepler-action release |
| 2~3 month | kepler, kepler-helm-chat, kepler-operator... default branch switch to kepler action |
| 3 month | kepler release |
| 3~3+1 month | kepler-helm-chat, kepler-operator... default branch switch to latest kepler release |

and we also provide pre release for kepler, kepler operator etc... to make a regular build as security patch for CVE fixing.

### Workflow Description
n/A I suppose this rfc itself is a proposal as workflow.

### Implementation Details/Notes/Constraints [optional]
n/A

### Risks and Mitigations
n/A

### Drawbacks
n/A

## Design Details
We can use https://docs.github.com/en/actions/managing-issues-and-pull-requests/scheduling-issue-creation to schedule those works.

### Open Questions [optional]
n/A, leave to PR to discussion.

### Test Plan
n/A

## Implementation History
n/A

## Alternatives
n/A

## Infrastructure Needed [optional]
n/A
