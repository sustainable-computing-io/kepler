<img align="right" width="250px" src="https://user-images.githubusercontent.com/17484350/138557170-d8079b94-a517-4366-ade8-8d473e3f3f1d.jpg">

![GitHub Workflow Status (event)](https://img.shields.io/github/actions/workflow/status/sustainable-computing-io/kepler/unit_test.yml?branch=main&label=CI)
![Coverage](https://img.shields.io/badge/Coverage-43.1%25-yellow)
<!--
[![GoDoc](https://godoc.org/github.com/kubernetes/kube-state-metrics?status.svg)](https://godoc.org/github.com/kubernetes/kube-state-metrics)
-->

![GitHub](https://img.shields.io/github/license/sustainable-computing-io/kepler)

[![Twitter URL](https://img.shields.io/twitter/url/https/twitter.com/KeplerProject.svg?style=social&label=Follow%20%40KeplerProject)](https://twitter.com/KeplerProject)

# Kepler
Kepler (Kubernetes Efficient Power Level Exporter) uses eBPF to probe energy-related system stats and exports them as Prometheus metrics.

## Architecture
Kepler Exporter exposes a variety of [metrics](https://sustainable-computing.io/design/metrics/) about the energy consumption of Kubernetes components such as Pods and Nodes. 

![Architecture](doc/kepler-arch.png)

## Install Kepler
Instructions to install Kepler can be found in the [Kepler docs](https://sustainable-computing.io/installation/kepler/).

## Visualise Kepler metrics with Grafana
To visualise the power consumption metrics made available by the Kepler Exporter, import the pre-generated [Kepler Dashboard](grafana-dashboards/Kepler-Exporter.json) into Grafana:
 ![Sample Grafana dashboard](doc/dashboard.png)

## Contribute to Kepler
Interested in contributing to Kepler? Follow the [Contributing Guide](CONTRIBUTING.md) to get started!

## Talks & Demos
- [Kepler Demo](https://www.youtube.com/watch?v=P5weULiBl60)
- ["Sustainability the Container Native Way" - Open Source Summit NA 2022](doc/OSS-NA22.pdf)

A full list of talks and demos about Kepler can be found [here](https://github.com/sustainable-computing-io/kepler-doc/tree/main/demos).

## Community Meetings
Please join the biweekly community meeting, starting on 8-9AM US ET (UTC−05:00) every other Tuesdays. 

[Download](https://us06web.zoom.us/meeting/tZApfumrrzgpHdwUQbkSOj0zpoGveKMMvxaD/ics?icsToken=98tyKuGsqjIqGdWWsx6PRpwIBor4c-nzmHZegqdKqS7nCBN8aFPsJu1lJLdUPfXG) the biweekly community meeting iCalendar file.
