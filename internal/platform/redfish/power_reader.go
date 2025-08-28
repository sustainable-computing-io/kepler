// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package redfish

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/stmcginnis/gofish"
	"github.com/sustainable-computing-io/kepler/internal/device"
)

type (
	Energy     = device.Energy
	Power      = device.Power
	EnergyZone = device.EnergyZone
)

// PowerReading represents a power measurement with timestamp
type PowerReading struct {
	Power     Power     // Current power consumption in watts
	Timestamp time.Time // When the reading was taken
	ChassisID string    // Chassis ID for metrics labeling
}

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

// ReadPower reads the current power consumption from the BMC
func (pr *PowerReader) ReadPower(ctx context.Context) (*PowerReading, error) {
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

	// Use the firstChassis chassis for power reading
	// In most single-node systems, there's typically only one chassis
	firstChassis := chassis[0]
	if firstChassis == nil {
		return nil, fmt.Errorf("first chassis is nil")
	}

	// Extract chassis ID for metrics labeling
	chassisID := firstChassis.ID
	if chassisID == "" {
		chassisID = "unknown"
	}

	power, err := firstChassis.Power()
	if err != nil {
		return nil, fmt.Errorf("failed to get power information from chassis: %w", err)
	}

	if power == nil {
		return nil, fmt.Errorf("power information is not available for chassis %s", chassisID)
	}

	if len(power.PowerControl) == 0 {
		return nil, fmt.Errorf("no power control information available")
	}

	// Get power consumption from the first power control
	powerControl := power.PowerControl[0]

	// PowerConsumedWatts is the current power consumption
	if powerControl.PowerConsumedWatts == 0 {
		pr.logger.Warn("Power consumption reading is zero", "endpoint", pr.endpoint)
	}

	reading := &PowerReading{
		Power:     Power(powerControl.PowerConsumedWatts) * device.Watt,
		ChassisID: chassisID,
		Timestamp: time.Now(),
	}

	pr.logger.Debug("Successfully read power from BMC",
		"endpoint", pr.endpoint,
		"power_watts", reading.Power,
		"timestamp", reading.Timestamp,
		"chassis_id", reading.ChassisID)

	return reading, nil
}

// ReadPowerWithRetry reads power with retry logic
func (pr *PowerReader) ReadPowerWithRetry(ctx context.Context, maxAttempts int, retryDelay time.Duration) (*PowerReading, error) {
	var lastErr error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		reading, err := pr.ReadPower(ctx)
		if err == nil {
			if attempt > 1 {
				pr.logger.Info("Power reading succeeded after retry",
					"endpoint", pr.endpoint,
					"attempt", attempt,
					"power_watts", reading.Power)
			}
			return reading, nil
		}

		lastErr = err
		pr.logger.Warn("Power reading failed",
			"endpoint", pr.endpoint,
			"attempt", attempt,
			"max_attempts", maxAttempts,
			"error", err)

		// Don't sleep on the last attempt
		if attempt < maxAttempts {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(retryDelay):
				// Continue to next attempt
			}
		}
	}

	return nil, fmt.Errorf("failed to read power after %d attempts, last error: %w", maxAttempts, lastErr)
}
