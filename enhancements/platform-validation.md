---
title: Kepler Platform Validation
authors:
  - Jie Ren
reviewers:
  - Sam Yuan, Marcelo Carneiro do Amaral
approvers:
  - N/A
creation-date: 2023-05-27
last-updated: 2023-09-01
tracking-links:
  - https://github.com/sustainable-computing-io/kepler/issues/712
---

# Kepler Platform Validation Guide

## Summary

A quick start guide to validate Kepler support on specific hardware platform.

## Motivation

Currently, Kepler's test cases focus on code logic verification, lack of actual platform validation.

Kepler's [data](https://github.com/sustainable-computing-io/kepler/tree/main/data) files majorly contain static information, or lack of data sync mechanism.

Test cases and test report are not available, which could be the evidence or check criteria to below concerns:

1. Whether the specific CPU/GPU/FPGA/Accelerator model is well supported in Kepler?

2. Whether the data/metrics of specific power source are correctly exported?

3. Whether the power measurement accuracy is acceptable?

4. More...

### Goals

1. Define workflow to validate specific hardware platform support in Kepler.

2. Design test cases for such validation. (Platform agnostic and specific cases)

3. Define test report format and release process. (General test items result and hardware specific items)

### Non-Goals

1. Platforms which are not supported by Kepler yet should be out of this enhancement's scope.

2. Platforms which do not support power measurement could be low priority for this enhancement, since they depends on model train and the power consumption check criteria is TBD.

## Proposal

We would suggest to add platform validation test cases and define the template of the test report.

The specific tasks could be broken down as follows:

1. Platform validation guide document.

2. Platform validation test cases design and implementation.(Both platform agnostic and specific cases)

3. Platform validation test report format definition, report generation and release process definition.


### Workflow Description

As question 1 and 2 mentioned in `Open questions and action items` section below, there are various test scenarios. in this section, we are focusing on the major test scenario's workflow in Kepler: containerized deployment.

Step 1: Create a new local Kubernetes cluster on the test node with target hardware platform.

Bring the cluster up with following [repo](https://github.com/sustainable-computing-io/local-dev-cluster) or just `make cluster-up`

Step 2: Deploy Kepler in the cluster.

Step 3: Run test cases and get test result.

Step 4: Generate test report and save to somewhere.

Step 5: Delete the local cluster and recover the original environment on the test node.

Note:
1. There are three kinds of deployment available in Kepler:

* Deploy using Helm Chart. Follow guide [here](https://github.com/sustainable-computing-io/kepler-helm-chart/blob/main/README.md).

* Deploy using Kepler Operator. Follow guide [here](https://github.com/sustainable-computing-io/kepler-operator/blob/v1alpha1/README.md).

* Deploy from Kepler Source Code. Follow guide [here](https://sustainable-computing.io/installation/kepler/).


2. Above steps cover the whole test lifecycle.

* Test setup: Step 1 & 2.

* Test execution: Step 3 & 4.

* Test cleanup: Step 5.

All test cases could share same Test setup/cleanup and focus on their own execution logic.
In order to cover three deployment scenario above, we could implement different scripts or code module for test cases' pick up.

All the scenarios should be automated, but no need to do 1:1 test for each test case. See details in `Test Plan` section below.


### Implementation Details/Notes/Constraints

Our validation test is local-dev based. Currently Kepler only support [Kind](https://kind.sigs.k8s.io/) for local deployment, before supporting [more cluster types](https://github.com/sustainable-computing-io/kepler/issues/331), we define and implement test cases based on Kind.

There are two ways to check metrics exported by Kepler:

* Check on Kepler Exporter.

Directly check the Kepler exposed metrics by port forwarding port 9102 to host port.

Pros: Check metrics on the very beginning of dataflow, direct evaluation on Kepler's collector and exporter functionalities.

Cons: Kepler exporter metrics can be regarded as snapshots per three seconds(current defautl scrape interval), it may not be intuitive to those observability concepts of power consumption and power attribution. 

* Check on Prometheus.

Deploy Prometheus service and Kepler specific Service Monitor in the cluster.

Expose Prometheus service also through the `port forwarding` way.

Pros: Codes/scripts could retrieve what they needed data from Prometheus using either REST API calls or PromQL queries.

Cons: Need to deploy Prometheus and Kepler Service Monitor.

A typical but not only way could be found in `Test Plan` section below.

### Risks and Mitigations

* Risk

  Some test cases may not be applicable on specific platforms.

* Mitigation

  Propose test case through Github PR one by one after it is fully reviewed and tested.

### Drawbacks
N/A

## Design Details

### Test Plan

#### Test Methodogy

The current local-dev-cluster setup job will by default bring up Kind cluster and deploy basic Prometheus service(without Grafana).

The current three Kepler deployment scenarios:

* Deploy from source.

It supports Kepler-Prometheus association by binding Kepler's specific service monitor to Prometheus:

(1) Deploy Kepler with Prometheus.

```
make build-manifest OPTS="PROMETHEUS_DEPLOY"
```

(2) Prometheus community provides [REST APIs](https://prometheus.io/docs/prometheus/latest/querying/api/) and [client-go](https://github.com/prometheus/client_golang/blob/main/api/prometheus/v1/api.go) libraries for specific queries.


* Deploy from kepler-helm-chart.

It by default does not support deploy kepler service monitor yet.

* Deploy from kepler-operator.

Kepler operator feature has some refactoring in 0.6.0 release period, we may cover this scenario in platform validation framework in later releases.

We could leverage the `port forwarding` mechanism to expose accessible ports of `kepler-exporter` and `prometheus-k8s` sevices to the automation codes.

```
kubectl port-forward --address localhost -n kepler service/kepler-exporter 9102:9102 &

kubectl port-forward --address localhost -n monitoring service/prometheus-k8s 9090:9090 &
```

To support validation cases comparison base, we are introducing an indepentent RAPL-based energy collection and power consumption calculation tool called `validator` on Intel X86 BareMetal platforms.

Test cases could use the `valiator` sampling and calculation results to compare with Kepler exported and Promethes aggregated query results.

For other platforms, developers may use other specific measurement methods and tools to implement similar validation targets.

#### Test Cases

Till 0.6.0 release, we have been introducing four automation platform validation cases on Intel X86 BareMetal platforms.

Case 1. CPU architecture recognition check.

(1) Use `validator` built-in `cpuid` tool to achieve the current platform CPU architecture info.

(2) Compare it with the Kepler exported metrics' `cpu_archiecture` label info.


Case 2. Platform specific power source components validity check.

(1) Use `validator` tool to detect the RAPL components support status at the beginning of the test.

(2) Check Kepler exported specific components metrics' validity.


Case 3. Prometheus side node level components power stats accuracy check.

(1) Use `validator` tool to do node-level sampling and power calculation for the node level power stats data.

(2) Query with Prometheus for the same data as comparison.


Case 4. Prometheus side namespace level components power stats accuracy check.

(1) Use `validator` tool to do node-level sampling and power calculation in cluster up different phases

(2) Query the observed APP's related namespace container power sum to compare with the power stats deltas from `validator`.

(3) The by default APP could be `kind cluster`(refer to three namespaces) and `kepler`(refer to one namespace)

Note: There might be interference processes in the same node for this test, for example, other tenants deployed workloads, other running VMs, etc. So we can better understand the Kepler's current accuracy on BareMetal Power Ratio Modeling.

Besides above automation test cases, with the help of the tools such as `validator`, we can also trigger manual(or automation) tests to check below scenarios:

1. Typical workloads power consumption measurement (carbon footprint) accuracy check on specific platform.

2. Carbon footprint accuracy check in scaling scenarios (VPA/HPA) on specific platform.


## Implementation History

* 2023/5/27, Jie Ren (jie.ren@intel.com), initial draft for ideas and scope.

* 2023/6/6, Jie Ren (jie.ren@intel.com), change per review comments and add new section for open questions and action items.

* 2023/9/1, Jie Ren (jie.ren@intel.com), adjust description in `Implementation Details` and `Test Plan` sections per platform-validation framework initial code commit in community. Also update in `Open questions and action items` section for resolved issues.

## Alternatives
N/A

## Infrastructure Needed

1. Github runner machines which cover the Kepler suppported platforms. On-prem machines, CSP's BMs/VMs, etc.

2. Test cases should follow current ginkgo framework and merged into e2e/ directory code files.

3. Test report release process needs further investigation, may be rely on Ginkgo's Reporting mechanism. Whether needs to integrate test report into [SBOM](https://github.com/sustainable-computing-io/kepler/pull/702) is an open.

## Open questions and action items.

1. Local dev cluster choices.

Three solutions mentioned in previous comments: Kind, k3d/k3s, microshift.

* Kind is current available and widely-used solution in Kelper.

* Microshift has also been supported in 2023 Q3. It is beneficial supplement to Kind and especially for local deployment verfication for Openshift scenarios.

* k3d/k3s will not be community choice for local dev in short term.

AI:

(1) The initial proposed platform validation framework code only covers BM platforms scenarios, we still design and execute test cases based on Kind. If needed, we can try other solutions, such as microshift later.

(2) Add more BM platforms validation cases, such as s390x and arm64 platforms, adjust platform validation framework if needed.

2. Test scenarios coverage.

AI:

(1) BM vs Container vs VM scenarios.
* For BM scenario,
  `kepler binary` built from source and `kepler rpm package` included in Kepler official release should be considered.

* For Container scenario, it is just Kepler's current focus, because Kepler is originated from Kubernetes and as a CNCF project, its focus should be `Cloud Native`.

  Three deployment methods available in community:

  Deploy from source.

  Deploy from kepler-helm-chart.

  Deploy from kepler-operator.

* For VM scenario, needs more investigations and TBD.

3. Some test cases' feasibility.

AI:

(1) Test plan and cases are open in community.

(2) Test code and execution workflow should be reviewed before merge. Separate PRs needed case by case in the future.

4. The content/format/release process of the test report.

AI:

(1) some cases could be merged into current integration test scope, they are Ginkgo based, so need to investigate the report output and release process. Some refernces are [here](https://onsi.github.io/ginkgo/#reporting-and-profiling-suites)

(2) Other cases are open and TBD.

5. Test cases execution workflow.

AI:

(1) Define GHA for test cases execution, platform validatio workflows should happen on specific self-hosted runners, manual or release based one-shot executions are appropriate.

(2) PR triggerred cases should be merged into current integration test scope; others may live under e2e directory also but handled by different GHA workflow ymls.

(3) Separate PRs needed for change here.

6. More test cases?

AI:

(1) Marcelo introduced `stress-ng` for stress test. Need investigation whether it is applicable in this PR scope.

(2) Some further cases are introduced in 0.6.0 release, see `Test Plan` section above.

(3) More brainstorming ideas are needed and welcomed in community, as long as they are `Platform Validation` related. Ideas could be added here, test cases should be seperate PRs.
