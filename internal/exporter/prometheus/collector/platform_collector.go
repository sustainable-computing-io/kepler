// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package collector

import (
	"log/slog"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/sustainable-computing-io/kepler/internal/platform/redfish"
)

const (
	// Prometheus namespace for Kepler platform metrics
	platformSubsystem = "platform"
)

// PlatformDataProvider defines the interface for getting platform power data
type PlatformDataProvider interface {
	GetLatestReading() (reading *redfish.PowerReading, totalEnergyJ float64, nodeID string)
}

// PlatformCollector collects platform power metrics from Redfish BMC
type PlatformCollector struct {
	logger       *slog.Logger
	dataProvider PlatformDataProvider

	// Metric descriptors
	wattsDesc  *prometheus.Desc
	joulesDesc *prometheus.Desc
}

// NewPlatformCollector creates a new platform collector
func NewPlatformCollector(dataProvider PlatformDataProvider, logger *slog.Logger) *PlatformCollector {
	return &PlatformCollector{
		logger:       logger,
		dataProvider: dataProvider,
		wattsDesc: prometheus.NewDesc(
			prometheus.BuildFQName(keplerNS, platformSubsystem, "watts"),
			"Current platform power consumption in watts",
			[]string{"source", "node_name"},
			nil,
		),
		joulesDesc: prometheus.NewDesc(
			prometheus.BuildFQName(keplerNS, platformSubsystem, "joules_total"),
			"Total platform energy consumption in joules",
			[]string{"source", "node_name"},
			nil,
		),
	}
}

// Describe sends the descriptors of platform metrics to the provided channel
func (c *PlatformCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.wattsDesc
	ch <- c.joulesDesc
}

// Collect gathers platform power metrics and sends them to the provided channel
func (c *PlatformCollector) Collect(ch chan<- prometheus.Metric) {
	reading, totalEnergyJ, nodeID := c.dataProvider.GetLatestReading()

	// If no reading is available, don't emit metrics
	if reading == nil {
		c.logger.Debug("No platform power reading available")
		return
	}

	// Check if the reading is too old (more than 60 seconds)
	if time.Since(reading.Timestamp) > 60*time.Second {
		c.logger.Warn("Platform power reading is stale, skipping metrics",
			"age_seconds", time.Since(reading.Timestamp).Seconds(),
			"node_id", nodeID)
		return
	}

	labels := []string{"redfish", nodeID}

	// Emit current power consumption metric
	ch <- prometheus.MustNewConstMetric(
		c.wattsDesc,
		prometheus.GaugeValue,
		reading.PowerWatts,
		labels...,
	)

	// Emit total energy consumption metric
	ch <- prometheus.MustNewConstMetric(
		c.joulesDesc,
		prometheus.CounterValue,
		totalEnergyJ,
		labels...,
	)

	c.logger.Debug("Collected platform metrics",
		"node_id", nodeID,
		"power_watts", reading.PowerWatts,
		"total_energy_j", totalEnergyJ,
		"reading_age_seconds", time.Since(reading.Timestamp).Seconds())
}
