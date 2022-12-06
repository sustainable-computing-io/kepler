<img align="right" width="250px" src="https://user-images.githubusercontent.com/17484350/138557170-d8079b94-a517-4366-ade8-8d473e3f3f1d.jpg">

![GitHub Workflow Status (event)](https://img.shields.io/github/workflow/status/sustainable-computing-io/kepler/Unit%20test?label=CI)
![Coverage](https://img.shields.io/badge/Coverage-40.1%25-yellow)
<!--
[![GoDoc](https://godoc.org/github.com/kubernetes/kube-state-metrics?status.svg)](https://godoc.org/github.com/kubernetes/kube-state-metrics)
-->

![GitHub](https://img.shields.io/github/license/sustainable-computing-io/kepler)

[![Twitter URL](https://img.shields.io/twitter/url/https/twitter.com/KeplerProject.svg?style=social&label=Follow%20%40KeplerProject)](https://twitter.com/KeplerProject)

# kepler
Kepler (Kubernetes Efficient Power Level Exporter) uses eBPF to probe energy related system stats and exports as Prometheus metrics

# Architecture
![Architecture](doc/kepler-arch.png)

# Talk and Demo
[Open Source Summit NA 2022 talk](doc/OSS-NA22.pdf) and [demo](https://www.youtube.com/watch?v=P5weULiBl60)

# Requirement
Kernel 4.18+

# Installation and Configuration for Prometheus
## Prerequisites
Need access to a Kubernetes cluster.

## Deploy the Kepler exporter
Deploying the Kepler exporter as a daemonset to run on all nodes. The following deployment will also create a service listening on
port 9102.
1. Build Manifest file
   -  kubectl v1.21.0 is minimum version that support build manifest
   -  manifest sources and outputs will be in  _output/generated-manifests by default
   -  Run:
        ```bash
        make build-manifest OPTS="<deployment options>"
        # minimum deployment: 
        # > make build-manifest
        # deployment with sidecar on openshift: 
        # > make build-manifest OPTS="ESTIMATOR_SIDECAR_DEPLOY OPENSHIFT_DEPLOY"
        ```

        Deployment Option|Description
        ---|---
        BM_DEPLOY|baremetal deployment patched with node selector feature.node.kubernetes.io/cpu-cpuid.HYPERVISOR to not exist
        OPENSHIFT_DEPLOY|patch openshift-specific attribute to kepler daemonset and deploy SecurityContextConstraints
        PROMETHEUS_DEPLOY|patch prometheus-related resource (ServiceMonitor, RBAC role, rolebinding) - require [prometheus deployment](README.md/#deploy-the-prometheus-operator-and-the-whole-monitoring-stack)
        CLUSTER_PREREQ_DEPLOY|deploy prerequisites for kepler on openshift cluster (only available when OPENSHIFT_DEPLOY set)
        CI_DEPLOY|deploy volumn mount for CI
        ESTIMATOR_SIDECAR_DEPLOY|patch estimator sidecar and corresponding configmap to kepler daemonset
        MODEL_SERVER_DEPLOY|deploy model server and corresponding configmap to kepler daemonset
        TRAIN_DEPLOY|patch online-trainer sidecar to model server (only available when MODEL_SERVER_DEPLOY set)

2. For deployment with cluster prerequisites:
    ```bash
    oc apply --kustomize _output/generated-manifests/config/cluster-prereqs
    # Check before proceeding that all nodes are Ready and Schedulable
    oc get nodes
    ```
    > Each node is decommissioned and rebooted - this may take ~20 minutes.
3. Deploy kustomized manifest
    ```bash
    kubectl create -f _output/generated-manifests/deployment.yaml
    ```
## Deploy the Prometheus operator and the whole monitoring stack
1. Clone the [kube-prometheus](https://github.com/prometheus-operator/kube-prometheus) project to your local folder.
```
# git clone https://github.com/prometheus-operator/kube-prometheus
```

1. Deploy the whole monitoring stack using the config in the `manifests` directory.
Create the namespace and CRDs, and then wait for them to be available before creating the remaining resources
```
# cd kube-prometheus
# kubectl apply --server-side -f manifests/setup
# until kubectl get servicemonitors --all-namespaces ; do date; sleep 1; echo ""; done
# kubectl apply -f manifests/
```

## Sample Grafana dashboard
Import the pre-generated [Kepler Dashboard](grafana-dashboards/Kepler-Exporter.json) into grafana
 ![Sample Grafana dashboard](doc/dashboard.png)

## To start developing Kepler
To set up a development environment please read our [Getting Started Guide](doc/dev/README.md)

## Community Meetings
[Download](https://us06web.zoom.us/meeting/tZ0sfuigqz8oHNQOn3yCDuBoEtPbXbZII5tH/ics?icsToken=98tyKuGhrzIrEtGRsh-HRpx5BYr4d_zwmClBgo1ssxG2GgN3dyH5E_ZyMIp9KvH5) the biweekly community meeting iCalendar file.
