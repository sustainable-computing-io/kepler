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

package manager

import (
	"time"

	"github.com/sustainable-computing-io/kepler/pkg/collector"
	"github.com/sustainable-computing-io/kepler/pkg/config"
	"github.com/sustainable-computing-io/kepler/pkg/kubernetes"
	exporter "github.com/sustainable-computing-io/kepler/pkg/metrics"
)

var (
	samplePeriod = time.Duration(config.SamplePeriodSec * 1000 * uint64(time.Millisecond))
)

type CollectorManager struct {
	// StatsCollector is resposible to collect resource and energy consumption metrics and calculate them when needed
	StatsCollector *collector.Collector

	// PrometheusCollector implements the external Collector interface provided by the Prometheus client
	PrometheusCollector *exporter.PrometheusExporter

	// Watcher register in the kubernetes apiserver to watch for pod events to add or remove it from the ContainerStats map
	Watcher *kubernetes.ObjListWatcher
}

func New() *CollectorManager {
	manager := &CollectorManager{}
	manager.StatsCollector = collector.NewCollector()
	manager.PrometheusCollector = exporter.NewPrometheusExporter()
	// the collector and prometheusExporter share structures and collections
	manager.PrometheusCollector.NewProcessCollector(manager.StatsCollector.ProcessStats)
	manager.PrometheusCollector.NewContainerCollector(manager.StatsCollector.ContainerStats)
	manager.PrometheusCollector.NewVMCollector(manager.StatsCollector.VMStats)
	manager.PrometheusCollector.NewNodeCollector(&manager.StatsCollector.NodeStats)
	// configure the wather
	manager.Watcher = kubernetes.NewObjListWatcher()
	manager.Watcher.Mx = &manager.PrometheusCollector.Mx
	manager.Watcher.ContainerStats = &manager.StatsCollector.ContainerStats
	manager.Watcher.Run()
	return manager
}

func (m *CollectorManager) Start() error {
	if err := m.StatsCollector.Initialize(); err != nil {
		return err
	}

	go func() {
		ticker := time.NewTicker(samplePeriod)
		for {
			// wait x seconds before updating the metrics
			<-ticker.C

			// acquire the lock to wait prometheus finish the metric collection before updating the metrics
			m.PrometheusCollector.Mx.Lock()
			m.StatsCollector.Update()
			m.PrometheusCollector.Mx.Unlock()
		}
	}()

	return nil
}
