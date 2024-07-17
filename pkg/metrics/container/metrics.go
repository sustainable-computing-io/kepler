/*
Copyright 2023.

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

package container

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sustainable-computing-io/kepler/pkg/bpf"
	"github.com/sustainable-computing-io/kepler/pkg/collector/stats"
	"github.com/sustainable-computing-io/kepler/pkg/config"
	"github.com/sustainable-computing-io/kepler/pkg/metrics/consts"
	"github.com/sustainable-computing-io/kepler/pkg/metrics/metricfactory"
	"github.com/sustainable-computing-io/kepler/pkg/metrics/utils"
)

const (
	context = "container"
)

// collector implements prometheus.Collector. It collects metrics directly from container maps.
type collector struct {
	descriptions map[string]*prometheus.Desc
	collectors   map[string]metricfactory.PromMetric

	// ContainerStats holds all containers energy and resource usage metrics
	ContainerStats map[string]*stats.ContainerStats

	// Lock to synchronize the collector update with prometheus exporter
	Mx *sync.Mutex

	bpfSupportedMetrics bpf.SupportedMetrics
}

func NewContainerCollector(containerMetrics map[string]*stats.ContainerStats, mx *sync.Mutex, bpfSupportedMetrics bpf.SupportedMetrics) prometheus.Collector {
	c := &collector{
		ContainerStats:      containerMetrics,
		descriptions:        make(map[string]*prometheus.Desc),
		collectors:          make(map[string]metricfactory.PromMetric),
		Mx:                  mx,
		bpfSupportedMetrics: bpfSupportedMetrics,
	}
	c.initMetrics()
	return c
}

// initMetrics creates prometheus metric description for container
func (c *collector) initMetrics() {
	if !config.IsExposeContainerStatsEnabled() {
		return
	}
	for name, desc := range metricfactory.HCMetricsPromDesc(context, c.bpfSupportedMetrics) {
		c.descriptions[name] = desc
		c.collectors[name] = metricfactory.NewPromCounter(desc)
	}
	for name, desc := range metricfactory.SCMetricsPromDesc(context, c.bpfSupportedMetrics) {
		c.descriptions[name] = desc
		c.collectors[name] = metricfactory.NewPromCounter(desc)
	}
	for name, desc := range metricfactory.EnergyMetricsPromDesc(context) {
		c.descriptions[name] = desc
		c.collectors[name] = metricfactory.NewPromCounter(desc)
	}
	for name, desc := range metricfactory.GPUUsageMetricsPromDesc(context) {
		c.descriptions[name] = desc
		c.collectors[name] = metricfactory.NewPromCounter(desc)
	}

	desc := metricfactory.MetricsPromDesc(context, "joules", "_total", "", consts.ContainerEnergyLabels)
	c.descriptions["total"] = desc
	c.collectors["total"] = metricfactory.NewPromCounter(desc)
}

func (c *collector) Describe(ch chan<- *prometheus.Desc) {
	for _, desc := range c.descriptions {
		ch <- desc
	}
}

func (c *collector) Collect(ch chan<- prometheus.Metric) {
	c.Mx.Lock()
	for _, container := range c.ContainerStats {
		utils.CollectEnergyMetrics(ch, container, c.collectors)
		utils.CollectResUtilizationMetrics(ch, container, c.collectors, c.bpfSupportedMetrics)
		// update container total joules
		utils.CollectTotalEnergyMetrics(ch, container, c.collectors)
	}
	c.Mx.Unlock()
}
