// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package redfish

import (
	"fmt"
	"log/slog"

	"github.com/stmcginnis/gofish"
	"github.com/sustainable-computing-io/kepler/internal/device"
)

// PowerReader handles reading power data from Redfish BMC
type PowerReader struct {
	logger   *slog.Logger
	client   *gofish.APIClient
	endpoint string // Store endpoint for logging
}

// NewPowerReader creates a new PowerReader with the given client
func NewPowerReader(logger *slog.Logger) *PowerReader {
	return &PowerReader{
		logger: logger,
	}
}

// SetClient sets the gofish client and endpoint for the power reader
func (pr *PowerReader) SetClient(client *gofish.APIClient) {
	pr.client = client
	if client != nil && client.Service != nil {
		pr.endpoint = client.Service.ODataID
	}
}

// ReadAll reads power consumption from all chassis with power data
func (pr *PowerReader) ReadAll() ([]Chassis, error) {
	if pr.client == nil {
		return nil, fmt.Errorf("BMC client is not connected")
	}

	service := pr.client.Service
	if service == nil {
		return nil, fmt.Errorf("BMC service is not available")
	}

	// Get chassis collection
	chassis, err := service.Chassis()
	if err != nil {
		return nil, fmt.Errorf("failed to get chassis collection: %w", err)
	}

	if len(chassis) == 0 {
		return nil, fmt.Errorf("no chassis found in BMC")
	}

	var chassisList []Chassis
	totalReadings := 0

	// Iterate
	for i, ch := range chassis {
		if ch == nil {
			pr.logger.Warn("Skipping nil chassis", "index", i)
			continue
		}

		// Extract chassis ID for metrics labeling
		chassisID := ch.ID
		if chassisID == "" {
			chassisID = fmt.Sprintf("chassis-%d", i)
		}

		power, err := ch.Power()
		if err != nil {
			pr.logger.Warn("Failed to get power information from chassis",
				"chassis_id", chassisID, "error", err)
			continue
		}

		if power == nil || len(power.PowerControl) == 0 {
			pr.logger.Debug("No power control information available for chassis",
				"chassis_id", chassisID)
			continue
		}

		// Collect all PowerControl entries for this chassis
		var readings []Reading
		for j, powerControl := range power.PowerControl {
			// Skip entries with zero power consumption
			if powerControl.PowerConsumedWatts == 0 {
				pr.logger.Debug("Power consumption reading is zero for PowerControl entry",
					"chassis_id", chassisID, "power_control_index", j, "member_id", powerControl.MemberID)
				continue
			}

			reading := Reading{
				ControlID: powerControl.MemberID,
				Name:      powerControl.Name,
				Power:     Power(powerControl.PowerConsumedWatts) * device.Watt,
			}

			readings = append(readings, reading)

			pr.logger.Debug("Successfully read power from PowerControl entry",
				"endpoint", pr.endpoint,
				"chassis_id", chassisID,
				"power_control_index", j,
				"member_id", powerControl.MemberID,
				"name", powerControl.Name,
				"physical_context", powerControl.PhysicalContext,
				"power_watts", powerControl.PowerConsumedWatts)
		}

		// Only add chassis if it has valid PowerControl readings
		if len(readings) > 0 {
			chassisData := Chassis{
				ID:       chassisID,
				Readings: readings,
			}
			chassisList = append(chassisList, chassisData)
			totalReadings += len(readings)
		}
	}

	if len(chassisList) == 0 {
		return nil, fmt.Errorf("no chassis with valid power readings found")
	}

	pr.logger.Info("Successfully collected PowerControl readings",
		"endpoint", pr.endpoint, "chassis_count", len(chassisList),
		"total_readings", totalReadings)

	return chassisList, nil
}
