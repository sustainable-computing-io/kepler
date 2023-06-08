---
title: Kepler Platform Validation
authors:
  - Jie Ren
reviewers:
  - Sam Yuan, Marcelo Carneiro do Amaral
approvers:
  - N/A
creation-date: 2023-05-27
last-updated: 2023-06-06
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

3. Whether the power measurement accuracy is accpetable?

4. More...

### Goals

1. Define workflow to validate specific hardward platform support in Kepler.

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

Step 5: Delete the local cluster and recover the orignal environment on the test node.

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

Cons: `kubectl port-forward` needs to be run in background process, this is inconvenient for automation and weak in side effect.

* Check on Prometheus.

Deploy Prometheus service and Kepler specific Service Monitor in the cluster.

Expose Prometheus service as `NodePort` type.

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

The first test case for this PR: CPU architecture recognition.

The current local-dev-cluster setup job will by default bring up Kind cluster and deploy basic Prometheus service(without Grafana).

The current three Kepler deployment scenarios:

* Deploy from source.

It supports Kepler-Prometheus association by binding Kepler's specific service monitor to Prometheus:

(1) Deploy Kepler with Prometheus.

```
make build-manifest OPTS="PROMETHEUS_DEPLOY"
```

(2) Change the Prometheus service as `NodePort` type and get the accessible port.

```
kubectl patch svc prometheus-k8s -n monitoring --type='json' -p '[{"op":"replace","path":"/spec/type","value":"NodePort"}]'
```
```
$kubectl get svc prometheus-k8s -n monitoring
NAME             TYPE       CLUSTER-IP    EXTERNAL-IP   PORT(S)                         AGE
prometheus-k8s   NodePort   10.96.205.9   <none>        9090:32517/TCP,8080:30331/TCP   5d9h
```

Tools such as `awk` could retrieve the accessible port:
```
$kubectl get svc prometheus-k8s -n monitoring | grep 9090 | awk '{print $5}' | awk -F ',' '{print $1}' | awk -F '/' '{print substr($1,6)}'
32517
```

(3) Get the test node's IP.

```
$ kubectl get nodes -o wide
NAME                 STATUS   ROLES           AGE     VERSION   INTERNAL-IP   EXTERNAL-IP   OS-IMAGE             KERNEL-VERSION      CONTAINER-RUNTIME
kind-control-plane   Ready    control-plane   5d10h   v1.25.3   172.19.0.2    <none>        Ubuntu 22.04.1 LTS   5.19.0-41-generic   containerd://1.6.9
```

```
$ kubectl get nodes -o wide |grep control-plane | awk '{print $6}'
172.19.0.2
```

(4) Retrieve the metrics with `cpu_architecture` label in Prometheus.

```
$ curl --noproxy "*" -s http://172.19.0.2:32517/api/v1/label/cpu_architecture/values | jq -r ".data[]" | sort
Sapphire Rapids
```

* Deploy from kepler-helm-chart.

It by default does not support deploy kepler service monitor yet.

* Deploy from kepler-operator.

It also by default does not support Kepler-Prometheus association.

Without `servcie monitor`, we have to use `port forwarding` mechanism to check metrics directly on Kepler Exporter:
```
kubectl port-forward --namespace=kepler service/kepler 9103:9102 &
```

So we could implement `port-forward` test logic in the latter two and implement service `nodePort type` patch logic in the first one.

After retrieve the exported metrics, we could check the actual `cpu_architure` info on the node:

Per current Kepler code,

(1) For x86 platforms, we use `cpuid` command to fetch the cpu arch info:

```
$ cpuid -1 |grep uarch
   (uarch synth) = Intel Sapphire Rapids {Golden Cove}, Intel 7
```

Note: we need to add prerequisite check for cpuid version on the test machine. Upgrade it if necessary.

Take Intel 4th Gen Xeon Scalable Processor (Code name: Sapphire Rapids, first released at 2023/1/10) as example, cpuid could not correctly recognize its full characteristics until `20230505` version, this version is available in RHEL distro's epel, but not available in latest Ubuntu APT repositories.

On Ubuntu test machine, the result may like this:

```
$ cpuid -1 |grep uarch
   (uarch synth) = Intel Sapphire Rapids {Sunny Cove}, 10nm+
```


(2) For ARM platforms, we use `archspec` Python tool, follow the usage guide in this [repo](https://github.com/archspec/archspec)

(3) For s390x platforms, we use `lscpu` command to check the machine type:
```
$ lscpu |grep "Machine type"
Machine type:        3906
```


The further test cases include not but limited to:

1. Typical workloads power consumption measurement (carbon footprint) accuracy check on specific platform.

2. Carbon footprint accuracy check in scaling scenarios (VPA/HPA) on specific platform.

## Implementation History

* 2023/5/27, Jie Ren (jie.ren@intel.com), initial draft for ideas and scope.

* 2023/6/6, Jie Ren (jie.ren@intel.com), change per review comments and add new section for open questions and action items.

## Alternatives
N/A

## Infrastructure Needed

1. Github runner machines which cover the Kepler suppported platforms. On-prem machines, CSP's BMs/VMs, etc.

2. Test cases should follow current ginkgo framework and merged into e2e/ directory code files.

3. Test report release process needs further investigation, may be rely on Ginkgo's Reporting mechanism. Whether needs to integrate test report into [SBOM](https://github.com/sustainable-computing-io/kepler/pull/702) is an open.

## Open questions and action items.

1. Local dev cluster choices.

Three solutions mentioned in previous comments: Kind, k3d/k3s, microshift.

* Kind is current available solution in Kelper. It works well on BM, but has known issues in VM.

* Microshift efforts and progress is tracked in Issue #182.

* k3d/k3s works are tracked in PR #313.

AI:

(1) Before more local dev solutions are officially supported in Kepler, we still design and execute platform validation test cases based on Kind.

(2) Once more solutions supported, such as microshift. Add option parameters support in both test cases and Kepler Makefile cluster-xxx operations.

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

(2) Test image build for different platforms. See existed PR [here](https://github.com/sustainable-computing-io/kepler/pull/587)

3. Some test cases' feasibility.

AI:

(1) Test plan and cases are open in community.

(2) Test code and execution workflow should be reviewed before merge. Seperate PRs needed case by case in the future.

4. The content/format/release process of the test report.

AI:

(1) some cases could be merged into current integration test scope, they are Ginkgo based, so need to investigate the report output and release process. Some refernces are [here](https://onsi.github.io/ginkgo/#reporting-and-profiling-suites)

(2) Other cases are open and TBD.

5. Test cases execution workflow.

AI:

(1) Define GHA for test cases execution, PR triggerred and one-shot execution are both needed.

(2) PR triggerred cases could be merged into current integration test scope; others may live under e2e directory also but handled by different GHA workflow ymls.

(3) Seperate PRs needed for change here.

6. More test cases?

AI:

(1) Marcelo introduced `stress-ng` for stress test. Need investigation whether it is applicable in this PR scope.

(2) More brainstorming ideas are needed and welcomed in community, as long as they are `Platform Validation` related. Ideas could be added here, test cases should be seperate PRs.
