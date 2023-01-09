---
title: upgarde to ginkgo v2
authors:
  - Sam Yuan
reviewers:
  - n/A
approvers:
  - n/A
creation-date: 2022-11-02
last-updated: 2022-11-02
tracking-links: # link to related GitHub issues
  - [link](https://github.com/sustainable-computing-io/kepler/issues/341)
---

# Upgrade ginkgo to v2 version

## Summary

Upgrade test framework ginkgo to v2 version, so that able to use [vscode plugins](https://marketplace.visualstudio.com/items?itemName=joselitofilho.ginkgotestexplorer) to see test coverage in visual way.

## Motivation

Same as Summary

### Goals

- update go mod to ginkgo v2
- update go vendor
- update test codes
- document as sample
- pass CI

### Non-Goals

Test coverage up to 50%

### Workflow Description

![usage](images/ginkgov2_vscode.png)
after this change, we are able to 
- Click at left side bar to run all test case at local.
- For any package, open a file click run package tests.
- Find test coverage in visual way, green for covered, red for not.

### Drawbacks
n/A
