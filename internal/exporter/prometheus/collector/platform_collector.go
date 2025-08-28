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

// RedfishDataProvider defines the interface for getting platform power data
type RedfishDataProvider interface {
	LatestReading() (reading *redfish.PowerReading, nodeID string)
	BMCID() string
}

// PlatformCollector collects platform power metrics from Redfish BMC
type PlatformCollector struct {
	logger       *slog.Logger
	dataProvider RedfishDataProvider

	// Metric descriptors
	wattsDesc *prometheus.Desc
}

// NewRedfishCollector creates a new platform collector
func NewRedfishCollector(dataProvider RedfishDataProvider, logger *slog.Logger) *PlatformCollector {
	return &PlatformCollector{
		logger:       logger,
		dataProvider: dataProvider,
		wattsDesc: prometheus.NewDesc(
			prometheus.BuildFQName(keplerNS, platformSubsystem, "watts"),
			"Current platform power consumption in watts from BMC",
			[]string{"source", "node_name", "bmc", "chassis_id"},
			nil,
		),
	}
}

// Describe sends the descriptors of platform metrics to the provided channel
func (c *PlatformCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.wattsDesc
}

// Collect gathers platform power metrics and sends them to the provided channel
func (c *PlatformCollector) Collect(ch chan<- prometheus.Metric) {
	reading, nodeID := c.dataProvider.LatestReading()
	bmcID := c.dataProvider.BMCID()

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

	// Label order must match the descriptor: source, node_name, bmc, chassis_id
	labels := []string{"redfish", nodeID, bmcID, reading.ChassisID}

	// Emit current power consumption metric (power-only approach)
	ch <- prometheus.MustNewConstMetric(
		c.wattsDesc,
		prometheus.GaugeValue,
		float64(reading.Power.Watts()),
		labels...,
	)

	c.logger.Debug("Collected platform power metrics",
		"node_id", nodeID,
		"bmc_id", bmcID,
		"chassis_id", reading.ChassisID,
		"power_watts", reading.Power,
		"reading_age_seconds", time.Since(reading.Timestamp).Seconds())
}
