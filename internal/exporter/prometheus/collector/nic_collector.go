// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package collector

import (
	"log/slog"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sustainable-computing-io/kepler/internal/k8s/pod"
	"github.com/sustainable-computing-io/kepler/internal/monitor"
)

// PodIPResolver resolves pod IPs to pod name/namespace. Optional.
type PodIPResolver interface {
	Resolve(ip string) (pod.PodMeta, bool)
}

// NICCollector exports node-level and per-pod NIC energy metrics.
type NICCollector struct {
	pm       monitor.PowerDataProvider
	logger   *slog.Logger
	resolver PodIPResolver // optional, nil if no k8s info available

	mutex sync.RWMutex
	ready bool

	// Node-level
	joulesDesc *prometheus.Desc
	wattsDesc  *prometheus.Desc

	// Per-pod
	podNICWattsDesc   *prometheus.Desc
	podNICTxBytesDesc *prometheus.Desc
	podNICRxBytesDesc *prometheus.Desc
}

// NewNICCollector creates a collector for node and per-pod NIC metrics.
// resolver is optional — if provided, metrics carry pod_name/pod_namespace labels.
func NewNICCollector(pm monitor.PowerDataProvider, nodeName string, resolver PodIPResolver, logger *slog.Logger) *NICCollector {
	podLabels := []string{"pod_ip", "pod_name", "pod_namespace"}

	c := &NICCollector{
		pm:       pm,
		logger:   logger.With("collector", "nic"),
		resolver: resolver,

		// Node-level metrics
		joulesDesc: prometheus.NewDesc(
			prometheus.BuildFQName(keplerNS, "node", "nic_joules_total"),
			"Cumulative NIC energy consumption in joules",
			[]string{"path"}, prometheus.Labels{nodeNameLabel: nodeName},
		),
		wattsDesc: prometheus.NewDesc(
			prometheus.BuildFQName(keplerNS, "node", "nic_watts"),
			"Current NIC power consumption in watts",
			[]string{"path"}, prometheus.Labels{nodeNameLabel: nodeName},
		),

		// Per-pod NIC metrics
		podNICWattsDesc: prometheus.NewDesc(
			prometheus.BuildFQName(keplerNS, "pod", "nic_watts"),
			"Attributed NIC power consumption per pod in watts",
			podLabels, prometheus.Labels{nodeNameLabel: nodeName},
		),
		podNICTxBytesDesc: prometheus.NewDesc(
			prometheus.BuildFQName(keplerNS, "pod", "nic_tx_bytes_total"),
			"Total bytes transmitted through physical NIC attributed to pod",
			podLabels, prometheus.Labels{nodeNameLabel: nodeName},
		),
		podNICRxBytesDesc: prometheus.NewDesc(
			prometheus.BuildFQName(keplerNS, "pod", "nic_rx_bytes_total"),
			"Total bytes received through physical NIC attributed to pod",
			podLabels, prometheus.Labels{nodeNameLabel: nodeName},
		),
	}

	go c.waitForData()
	return c
}

func (c *NICCollector) waitForData() {
	<-c.pm.DataChannel()
	c.mutex.Lock()
	c.ready = true
	c.mutex.Unlock()
}

func (c *NICCollector) isReady() bool {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.ready
}

// Describe implements prometheus.Collector.
func (c *NICCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.joulesDesc
	ch <- c.wattsDesc
	ch <- c.podNICWattsDesc
	ch <- c.podNICTxBytesDesc
	ch <- c.podNICRxBytesDesc
}

// Collect implements prometheus.Collector.
func (c *NICCollector) Collect(ch chan<- prometheus.Metric) {
	if !c.isReady() {
		return
	}

	snapshot, err := c.pm.Snapshot()
	if err != nil {
		c.logger.Error("Failed to collect NIC data", "error", err)
		return
	}

	// Node-level NIC metrics
	if snapshot.NICStats != nil {
		stats := snapshot.NICStats
		ch <- prometheus.MustNewConstMetric(
			c.joulesDesc, prometheus.CounterValue,
			stats.EnergyTotal.Joules(), stats.Path,
		)
		ch <- prometheus.MustNewConstMetric(
			c.wattsDesc, prometheus.GaugeValue,
			stats.Power.Watts(), stats.Path,
		)
	}

	// Per-pod NIC metrics
	for _, p := range snapshot.PodNICStats {
		name, namespace := "", ""
		if c.resolver != nil {
			if meta, ok := c.resolver.Resolve(p.PodIP); ok {
				name, namespace = meta.Name, meta.Namespace
			}
		}

		ch <- prometheus.MustNewConstMetric(
			c.podNICWattsDesc, prometheus.GaugeValue,
			p.Watts, p.PodIP, name, namespace,
		)
		ch <- prometheus.MustNewConstMetric(
			c.podNICTxBytesDesc, prometheus.CounterValue,
			float64(p.TxBytes), p.PodIP, name, namespace,
		)
		ch <- prometheus.MustNewConstMetric(
			c.podNICRxBytesDesc, prometheus.CounterValue,
			float64(p.RxBytes), p.PodIP, name, namespace,
		)
	}
}
