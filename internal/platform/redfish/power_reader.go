// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package redfish

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// PowerReading represents a power measurement with timestamp
type PowerReading struct {
	PowerWatts float64   // Current power consumption in watts
	Timestamp  time.Time // When the reading was taken
}

// PowerReader handles reading power data from Redfish BMC
type PowerReader struct {
	logger *slog.Logger
	client GoFishClient
}

// NewPowerReader creates a new PowerReader with the given client
func NewPowerReader(client GoFishClient, logger *slog.Logger) *PowerReader {
	return &PowerReader{
		logger: logger,
		client: client,
	}
}

// ReadPower reads the current power consumption from the BMC
func (pr *PowerReader) ReadPower(ctx context.Context) (*PowerReading, error) {
	if !pr.client.IsConnected() {
		return nil, fmt.Errorf("BMC client is not connected")
	}

	apiClient := pr.client.GetAPIClient()
	service := apiClient.Service

	// Get chassis collection
	chassis, err := service.Chassis()
	if err != nil {
		return nil, fmt.Errorf("failed to get chassis collection: %w", err)
	}

	if len(chassis) == 0 {
		return nil, fmt.Errorf("no chassis found in BMC")
	}

	// Use the first chassis for power reading
	// In most single-node systems, there's typically only one chassis
	firstChassis := chassis[0]

	power, err := firstChassis.Power()
	if err != nil {
		return nil, fmt.Errorf("failed to get power information from chassis: %w", err)
	}

	if len(power.PowerControl) == 0 {
		return nil, fmt.Errorf("no power control information available")
	}

	// Get power consumption from the first power control
	powerControl := power.PowerControl[0]

	// PowerConsumedWatts is the current power consumption
	if powerControl.PowerConsumedWatts == 0 {
		pr.logger.Warn("Power consumption reading is zero", "endpoint", pr.client.Endpoint())
	}

	reading := &PowerReading{
		PowerWatts: float64(powerControl.PowerConsumedWatts),
		Timestamp:  time.Now(),
	}

	pr.logger.Debug("Successfully read power from BMC",
		"endpoint", pr.client.Endpoint(),
		"power_watts", reading.PowerWatts,
		"timestamp", reading.Timestamp)

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
					"endpoint", pr.client.Endpoint(),
					"attempt", attempt,
					"power_watts", reading.PowerWatts)
			}
			return reading, nil
		}

		lastErr = err
		pr.logger.Warn("Power reading failed",
			"endpoint", pr.client.Endpoint(),
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
