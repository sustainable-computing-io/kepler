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

package node

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sustainable-computing-io/kepler/pkg/collector/stats"
	"github.com/sustainable-computing-io/kepler/pkg/config"
	"github.com/sustainable-computing-io/kepler/pkg/metrics/metricfactory"
	"github.com/sustainable-computing-io/kepler/pkg/metrics/utils"
	"github.com/sustainable-computing-io/kepler/pkg/sensors/components"
	"github.com/sustainable-computing-io/kepler/pkg/sensors/platform"
)

const (
	context = "node"
)

// collector implements prometheus.Collector. It collects metrics directly from BPF maps.
type collector struct {
	descriptions map[string]*prometheus.Desc
	collectors   map[string]metricfactory.PromMetric

	// NodeStats holds all node energy and resource usage metrics
	NodeStats *stats.NodeStats

	// Lock to syncronize the collector update with prometheus exporter
	Mx *sync.Mutex
}

func NewNodeCollector(nodeMetrics *stats.NodeStats, mx *sync.Mutex) prometheus.Collector {
	c := &collector{
		NodeStats:    nodeMetrics,
		descriptions: make(map[string]*prometheus.Desc),
		collectors:   make(map[string]metricfactory.PromMetric),
		Mx:           mx,
	}
	c.initMetrics()
	return c
}

// initMetrics creates prometheus metric description for node
func (c *collector) initMetrics() {
	// node exports different resource utilization metrics than process, container and vm
	for name, desc := range metricfactory.QATMetricsPromDesc(context) {
		c.descriptions[name] = desc
		c.collectors[name] = metricfactory.NewPromCounter(desc)
	}
	for name, desc := range metricfactory.NodeCPUFrequencyMetricsPromDesc(context) {
		c.descriptions[name] = desc
		c.collectors[name] = metricfactory.NewPromCounter(desc)
	}
	for name, desc := range metricfactory.EnergyMetricsPromDesc(context) {
		c.descriptions[name] = desc
		c.collectors[name] = metricfactory.NewPromCounter(desc)
	}

	// TODO: prometheus metric should be "node_info"
	desc := metricfactory.MetricsPromDesc(context, "", "info", "os", []string{
		"cpu_architecture", "components_power_source", "platform_power_source",
	})
	c.descriptions["info"] = desc
	c.collectors["info"] = metricfactory.NewPromCounter(desc)
}

func (c *collector) Describe(ch chan<- *prometheus.Desc) {
	for _, desc := range c.descriptions {
		ch <- desc
	}
}

func (c *collector) Collect(ch chan<- prometheus.Metric) {
	c.Mx.Lock()
	utils.CollectEnergyMetrics(ch, c.NodeStats, c.collectors)
	// we export different node resource utilization metrics than process, container and vms
	// TODO: verify if the resoruce utilization metrics are needed
	utils.CollectResUtil(ch, c.NodeStats, config.CPUFrequency, c.collectors[config.CPUFrequency])
	utils.CollectResUtil(ch, c.NodeStats, config.QATUtilization, c.collectors[config.QATUtilization])
	c.Mx.Unlock()

	// update node info
	ch <- c.collectors["info"].MustMetric(1,
		stats.NodeCPUArchitecture,
		components.GetSourceName(),
		platform.GetSourceName(),
	)
}
