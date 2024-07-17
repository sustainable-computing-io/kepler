/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package metrics

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sustainable-computing-io/kepler/pkg/bpf"
	"github.com/sustainable-computing-io/kepler/pkg/collector/stats"
	"github.com/sustainable-computing-io/kepler/pkg/config"
	"github.com/sustainable-computing-io/kepler/pkg/metrics/container"
	"github.com/sustainable-computing-io/kepler/pkg/metrics/node"
	"github.com/sustainable-computing-io/kepler/pkg/metrics/process"
	"github.com/sustainable-computing-io/kepler/pkg/metrics/virtualmachine"
	"k8s.io/klog/v2"
)

var (
	registryOnce sync.Once
	registry     *prometheus.Registry
)

// PrometheusExporter holds the list of prometheus metric collectors
type PrometheusExporter struct {
	ProcessStatsCollector   prometheus.Collector
	ContainerStatsCollector prometheus.Collector
	VMStatsCollector        prometheus.Collector
	NodeStatsCollector      prometheus.Collector

	// Lock to syncronize the collector update with prometheus exporter
	Mx sync.Mutex

	bpfSupportedMetrics bpf.SupportedMetrics
}

// NewPrometheusExporter creates a new prometheus exporter
func NewPrometheusExporter(bpfSupportedMetrics bpf.SupportedMetrics) *PrometheusExporter {
	return &PrometheusExporter{
		bpfSupportedMetrics: bpfSupportedMetrics,
	}
}

// NewProcessCollector creates a new prometheus collector for process metrics
func (e *PrometheusExporter) NewProcessCollector(processMetrics map[uint64]*stats.ProcessStats) {
	e.ProcessStatsCollector = process.NewProcessCollector(processMetrics, &e.Mx, e.bpfSupportedMetrics)
}

// NewContainerCollector creates a new prometheus collector for container metrics
func (e *PrometheusExporter) NewContainerCollector(containerMetrics map[string]*stats.ContainerStats) {
	e.ContainerStatsCollector = container.NewContainerCollector(containerMetrics, &e.Mx, e.bpfSupportedMetrics)
}

// NewVMCollector creates a new prometheus collector for vm metrics
func (e *PrometheusExporter) NewVMCollector(vmMetrics map[string]*stats.VMStats) {
	e.VMStatsCollector = virtualmachine.NewVMCollector(vmMetrics, &e.Mx, e.bpfSupportedMetrics)
}

// NewNodeCollector creates a new prometheus collector for node metrics
func (e *PrometheusExporter) NewNodeCollector(nodeMetrics *stats.NodeStats) {
	e.NodeStatsCollector = node.NewNodeCollector(nodeMetrics, &e.Mx)
}

func GetRegistry() *prometheus.Registry {
	registryOnce.Do(func() {
		registry = prometheus.NewRegistry()
	})
	return registry
}

func (e *PrometheusExporter) RegisterMetrics() *prometheus.Registry {
	r := GetRegistry()

	if config.IsExposeProcessStatsEnabled() {
		r.MustRegister(e.ProcessStatsCollector)
		klog.Infoln("Registered Process Prometheus metrics")
	}

	if config.IsExposeContainerStatsEnabled() {
		r.MustRegister(e.ContainerStatsCollector)
		klog.Infoln("Registered Container Prometheus metrics")
	}

	if config.IsExposeVMStatsEnabled() {
		r.MustRegister(e.VMStatsCollector)
		klog.Infoln("Registered VM Prometheus metrics")
	}

	r.MustRegister(e.NodeStatsCollector)
	klog.Infoln("Registered Node Prometheus metrics")

	// log prometheus errors
	_, err := r.Gather()
	if err != nil {
		klog.Errorln(err)
	}

	return r
}
