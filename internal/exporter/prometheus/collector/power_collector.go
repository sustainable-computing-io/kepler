// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package collector

import (
	"fmt"
	"log/slog"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sustainable-computing-io/kepler/internal/monitor"
)

const (
	nodeRAPL = "node"
)

type PowerDataProvider = monitor.PowerDataProvider

// PowerCollector combines Node, Process, and CPU collectors to ensure data consistency
// by fetching all data in a single atomic operation during collection
type PowerCollector struct {
	pm     PowerDataProvider
	logger *slog.Logger

	// Lock to ensure thread safety during collection
	mutex sync.RWMutex

	// Node power metrics
	ready                 bool
	nodeJoulesDescriptors map[string]*prometheus.Desc
	nodeWattsDescriptors  map[string]*prometheus.Desc

	energyZoneDescriptor *prometheus.Desc
}

// NewPowerCollector creates a collector that provides consistent metrics
// by fetching all data in a single snapshot during collection
func NewPowerCollector(monitor PowerDataProvider, logger *slog.Logger) *PowerCollector {
	c := &PowerCollector{
		pm:     monitor,
		logger: logger.With("collector", "power"),

		nodeJoulesDescriptors: make(map[string]*prometheus.Desc),
		nodeWattsDescriptors:  make(map[string]*prometheus.Desc),
	}
	go c.updateDescriptors()
	return c
}

// updateDescriptors creates metric descriptors based on available zones
func (c *PowerCollector) updateDescriptors() {
	<-c.pm.DataChannel()
	zoneNames := c.pm.ZoneNames() // must be thread-safe

	c.mutex.Lock() // for write
	defer c.mutex.Unlock()
	for _, name := range zoneNames {
		zoneName := SanitizeMetricName(name)

		//  node metric descriptors
		if _, exists := c.nodeJoulesDescriptors[zoneName]; !exists {
			c.nodeJoulesDescriptors[zoneName] = prometheus.NewDesc(
				prometheus.BuildFQName(namespace, nodeRAPL, zoneName+"_joules_total"),
				"Energy consumption in joules for RAPL zone "+zoneName,
				[]string{"path"},
				nil,
			)
		}

		if _, exists := c.nodeWattsDescriptors[zoneName]; !exists {
			c.nodeWattsDescriptors[zoneName] = prometheus.NewDesc(
				prometheus.BuildFQName(namespace, nodeRAPL, zoneName+"_watts"),
				"Power consumption in watts for RAPL zone "+zoneName,
				[]string{"path"},
				nil,
			)
		}
	}
	c.energyZoneDescriptor = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, nodeRAPL, "energy_zone"),
		"Energy Zones from RAPL",
		[]string{"name", "index", "path"},
		nil,
	)
	c.ready = true
}

// Describe implements the prometheus.Collector interface
func (c *PowerCollector) Describe(ch chan<- *prometheus.Desc) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	if !c.ready {
		c.logger.Debug("Describe called before monitor is ready")
		return
	}

	for _, desc := range c.nodeJoulesDescriptors {
		ch <- desc
	}
	for _, desc := range c.nodeWattsDescriptors {
		ch <- desc
	}
	ch <- c.energyZoneDescriptor
}

func (c *PowerCollector) isReady() bool {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.ready
}

// Collect implements the prometheus.Collector interface
func (c *PowerCollector) Collect(ch chan<- prometheus.Metric) {
	if !c.isReady() {
		c.logger.Debug("Collect called before monitor is ready")
		return
	}

	c.logger.Info("Collecting unified power data")
	snapshot, err := c.pm.Snapshot() // snapshot is thread-safe
	if err != nil {
		c.logger.Error("Failed to collect power data", "error", err)
		return
	}

	c.collectNodeMetrics(ch, snapshot.Node)
}

// collectNodeMetrics collects node-level power metrics
func (c *PowerCollector) collectNodeMetrics(ch chan<- prometheus.Metric, node *monitor.Node) {
	c.mutex.RLock() // locking nodeJoulesDescriptors
	defer c.mutex.RUnlock()

	for zone, energy := range node.Zones {
		zoneName := SanitizeMetricName(zone.Name())
		// ensure both joules and watts descriptors exist
		joulesDesc, exists := c.nodeJoulesDescriptors[zoneName]
		if !exists {
			continue
		}

		wattsDesc, exists := c.nodeWattsDescriptors[zoneName]
		if !exists {
			continue
		}

		path := zone.Path()
		ch <- prometheus.MustNewConstMetric(
			joulesDesc,
			prometheus.CounterValue,
			energy.Absolute.Joules(),
			path,
		)

		ch <- prometheus.MustNewConstMetric(
			wattsDesc,
			prometheus.GaugeValue,
			energy.Power.Watts(),
			path,
		)

		ch <- prometheus.MustNewConstMetric(
			c.energyZoneDescriptor,
			prometheus.GaugeValue,
			1,
			zoneName,
			fmt.Sprintf("%d", zone.Index()),
			path,
		)
	}
}
