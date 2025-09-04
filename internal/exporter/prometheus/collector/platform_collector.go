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
	Power() (*redfish.PowerReading, error) // On-demand method for all chassis
	NodeName() string                      // Node name
	BMCID() string                         // BMC identifier
}

// PlatformCollector collects platform power metrics from Redfish BMC
type PlatformCollector struct {
	logger  *slog.Logger
	redfish RedfishDataProvider

	// Static metadata
	nodeName string // Node identifier
	bmcID    string // BMC identifier

	// Metric descriptors
	wattsDesc *prometheus.Desc
}

// NewRedfishCollector creates a new platform collector
func NewRedfishCollector(redfish RedfishDataProvider, logger *slog.Logger) *PlatformCollector {
	if redfish == nil {
		panic("RedfishDataProvider cannot be nil - platform collector requires a data provider to function")
	}
	if logger == nil {
		logger = slog.Default()
	}

	return &PlatformCollector{
		logger:   logger,
		redfish:  redfish,
		nodeName: redfish.NodeName(),
		bmcID:    redfish.BMCID(),
		wattsDesc: prometheus.NewDesc(
			prometheus.BuildFQName(keplerNS, platformSubsystem, "watts"),
			"Current platform power consumption in watts from BMC PowerControl entries",
			[]string{"source", "node_name", "bmc_id", "chassis_id", "power_control_id", "power_control_name"},
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
	// Get all chassis power readings using the new simplified interface
	powerReading, err := c.redfish.Power()
	if err != nil {
		c.logger.Error("Failed to get chassis power readings", "error", err)
		return
	}

	// If no power reading is available, don't emit metrics
	if powerReading == nil || len(powerReading.Chassis) == 0 {
		c.logger.Debug("No platform power readings available")
		return
	}

	// Emit metrics for each PowerControl reading in each chassis
	for _, chassis := range powerReading.Chassis {
		for _, reading := range chassis.Readings {
			// Label order must match the descriptor: source, node_name, bmc_id, chassis_id, power_control_id, power_control_name
			labels := []string{"redfish", c.nodeName, c.bmcID, chassis.ID, reading.ControlID, reading.Name}

			// Emit current power consumption metric (power-only approach)
			ch <- prometheus.MustNewConstMetric(
				c.wattsDesc,
				prometheus.GaugeValue,
				float64(reading.Power.Watts()),
				labels...,
			)

			c.logger.Debug("Collected platform power metrics",
				"node.name", c.nodeName,
				"bmc.id", c.bmcID,
				"chassis.id", chassis.ID,
				"power_control.id", reading.ControlID,
				"power_control.name", reading.Name,
				"power.watts", reading.Power,
				"age", time.Since(powerReading.Timestamp).Seconds())
		}
	}
}
