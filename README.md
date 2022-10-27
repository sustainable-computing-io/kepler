<img align="right" width="250px" src="https://user-images.githubusercontent.com/17484350/138557170-d8079b94-a517-4366-ade8-8d473e3f3f1d.jpg">

![GitHub Workflow Status (event)](https://img.shields.io/github/workflow/status/sustainable-computing-io/kepler/Unit%20test?label=CI)
![Coverage](https://img.shields.io/badge/Coverage-28.4%25-red)
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
```
# build manifests file for VM+Baremetal and Baremetal only
# manifests are created in  _output/manifests/kubernetes/generated/ by default
# kubectl v1.21.0 is minimum version that support build manifest
# make build-manifest
```

if you are running with Baremetal only
```
kubectl create -f _output/manifests/kubernetes/generated/bm/deployment.yaml
```

if you are running with Baremetal and/or VM
```
kubectl create -f _output/manifests/kubernetes/generated/vm/deployment.yaml
```

## Deploy the Prometheus operator and the whole monitoring stack
1. Clone the [kube-prometheus](https://github.com/prometheus-operator/kube-prometheus) project to your local folder.
```
# git clone https://github.com/prometheus-operator/kube-prometheus
```

2. Deploy the whole monitoring stack using the config in the `manifests` directory.
Create the namespace and CRDs, and then wait for them to be available before creating the remaining resources
```
# cd kube-prometheus
# kubectl apply --server-side -f manifests/setup
# until kubectl get servicemonitors --all-namespaces ; do date; sleep 1; echo ""; done
# kubectl apply -f manifests/
```

## Configure Prometheus to scrape Kepler-exporter endpoints.
```
# cd ../kepler
# kubectl create -f manifests/kubernetes/keplerExporter-serviceMonitor.yaml
```

## Sample Grafana dashboard
Import the pre-generated [Kepler Dashboard](grafana-dashboards/Kepler-Exporter.json) into grafana
 ![Sample Grafana dashboard](doc/dashboard.png)


## To start developing Kepler
To set up a development environment please read our [Getting Started Guide](dev/README.md)
