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

package process

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sustainable-computing-io/kepler/pkg/collector/stats"
	"github.com/sustainable-computing-io/kepler/pkg/config"
	"github.com/sustainable-computing-io/kepler/pkg/metrics/metricfactory"
	"github.com/sustainable-computing-io/kepler/pkg/metrics/utils"
)

const (
	context = "process"
)

// collector implements prometheus.Collector. It collects metrics directly from process maps.
type collector struct {
	descriptions map[string]*prometheus.Desc
	collectors   map[string]metricfactory.PromMetric

	// ProcessStats holds all processes energy and resource usage metrics
	ProcessStats map[uint64]*stats.ProcessStats

	// Lock to syncronize the collector update with prometheus exporter
	Mx *sync.Mutex
}

func NewProcessCollector(processMetrics map[uint64]*stats.ProcessStats, mx *sync.Mutex) prometheus.Collector {
	c := &collector{
		ProcessStats: processMetrics,
		descriptions: make(map[string]*prometheus.Desc),
		collectors:   make(map[string]metricfactory.PromMetric),
		Mx:           mx,
	}
	c.initMetrics()
	return c
}

// initMetrics creates prometheus metric description for process
func (c *collector) initMetrics() {
	if !config.IsExposeProcessStatsEnabled() {
		return
	}
	for name, desc := range metricfactory.HCMetricsPromDesc(context) {
		c.descriptions[name] = desc
		c.collectors[name] = metricfactory.NewPromCounter(desc)
	}
	for name, desc := range metricfactory.SCMetricsPromDesc(context) {
		c.descriptions[name] = desc
		c.collectors[name] = metricfactory.NewPromCounter(desc)
	}
	for name, desc := range metricfactory.IRQMetricsPromDesc(context) {
		c.descriptions[name] = desc
		c.collectors[name] = metricfactory.NewPromCounter(desc)
	}
	for name, desc := range metricfactory.CGroupMetricsPromDesc(context) {
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
}

func (c *collector) Describe(ch chan<- *prometheus.Desc) {
	for _, desc := range c.descriptions {
		ch <- desc
	}
}

func (c *collector) Collect(ch chan<- prometheus.Metric) {
	c.Mx.Lock()
	for _, process := range c.ProcessStats {
		utils.CollectEnergyMetrics(ch, process, c.collectors)
		utils.CollectResUtilizationMetrics(ch, process, c.collectors)
	}
	c.Mx.Unlock()
}
