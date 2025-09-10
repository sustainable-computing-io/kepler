// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package redfish

import (
	"crypto/tls"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/stmcginnis/gofish"
	"github.com/stmcginnis/gofish/redfish"
	redfishcfg "github.com/sustainable-computing-io/kepler/config/redfish"
	"github.com/sustainable-computing-io/kepler/internal/device"
)

// PowerReader handles reading power data from Redfish BMC via PowerSubsystem with fallback to deprecated Power API
type PowerReader struct {
	logger *slog.Logger

	cfg    gofish.ClientConfig // gofish client configuration
	client *gofish.APIClient   // gofish client (managed internally)

	endpoint string           // Store endpoint for logging
	strategy PowerAPIStrategy // Determined power reading strategy

	once sync.Once
}

// NewPowerReader creates a new PowerReader with the given client
func NewPowerReader(bmc *redfishcfg.BMCDetail, httpTimeout time.Duration, logger *slog.Logger) *PowerReader {
	// Configure HTTP client with timeout and TLS configuration
	httpClient := &http.Client{
		Timeout: httpTimeout,
	}

	if bmc.Insecure {
		httpClient.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
	}

	// Create gofish client configuration
	cfg := gofish.ClientConfig{
		Endpoint:   bmc.Endpoint,
		Username:   bmc.Username,
		Password:   bmc.Password,
		HTTPClient: httpClient,
	}

	return &PowerReader{
		logger: logger,
		cfg:    cfg,
	}
}

// Init determines the power reading strategy by testing API availability with retry logic
func (pr *PowerReader) Init() error {
	// NOTE: Use Background() for client connection since gofish stores this context
	// and uses it for all subsequent HTTP requests. A timeout context causes
	// "context canceled" errors on later requests when the timeout expires.
	client, err := gofish.Connect(pr.cfg)
	if err != nil {
		return fmt.Errorf("failed to connect to BMC at %s: %w", pr.cfg.Endpoint, err)
	}

	// Ensure cleanup on failure after client is created
	needsCleanup := true
	defer func() {
		if !needsCleanup {
			return
		}

		pr.client.Logout()
		pr.client = nil
	}()

	pr.client = client
	if client.Service != nil {
		pr.endpoint = client.Service.ODataID
	}

	service := pr.client.Service
	if service == nil {
		return fmt.Errorf("BMC service is not available")
	}

	// Get chassis collection to test with
	chassis, err := service.Chassis()
	if err != nil {
		return fmt.Errorf("failed to get chassis collection: %w", err)
	}

	if len(chassis) == 0 {
		return fmt.Errorf("no chassis found in BMC")
	}

	strategy, err := pr.determineStrategy(chassis)
	if err != nil {
		return fmt.Errorf("failed to determine power reading strategy: %w", err)
	}

	pr.strategy = strategy
	pr.logger.Info("Power reading strategy determined",
		"endpoint", pr.endpoint, "strategy", string(strategy))

	// Success - don't cleanup the client
	needsCleanup = false
	return nil
}

// Close cleans up the PowerReader resources, including logging out from the BMC
func (pr *PowerReader) Close() {
	if pr.client == nil {
		return
	}

	pr.client.Logout()
	pr.client = nil
}

// determineStrategy tests chassis until it finds one with a supported API that has data
func (pr *PowerReader) determineStrategy(chassis []*redfish.Chassis) (PowerAPIStrategy, error) {
	if len(chassis) == 0 {
		return "", fmt.Errorf("no chassis available for testing")
	}

	for i, c := range chassis {
		if c == nil {
			pr.logger.Warn("Skipping nil chassis during strategy determination", "index", i)
			continue
		}

		if _, err := pr.readPowerSubsystem(c); err == nil {
			return PowerSubsystemStrategy, nil
		} else if _, err := pr.readPower(c); err == nil {
			return PowerStrategy, nil
		}
	}

	return UnknownStrategy, fmt.Errorf(
		"neither PowerSubsystem nor Power API is available on any chassis (tested %d chassis)",
		len(chassis))
}

// ReadAll reads power consumption from all chassis via PowerSubsystem with fallback to deprecated Power API
func (pr *PowerReader) ReadAll() ([]Chassis, error) {
	var initErr error
	// Ensure for the first time that client is initialized by
	// validating the client and that the strategy has been determined
	pr.once.Do(func() {
		if pr.client == nil {
			initErr = fmt.Errorf("BMC client is not initialized")
			return
		}

		service := pr.client.Service
		if service == nil {
			initErr = fmt.Errorf("BMC service is not available")
		}

		// Check if strategy has been determined
		if pr.strategy == UnknownStrategy {
			initErr = fmt.Errorf("power reading strategy not determined; call Init() first")
		}
	})

	if initErr != nil {
		return nil, initErr
	}

	// Get chassis collection
	chassis, err := pr.client.Service.Chassis()
	if err != nil {
		return nil, fmt.Errorf("failed to get chassis collection: %w", err)
	}

	if len(chassis) == 0 {
		return nil, fmt.Errorf("no chassis found in BMC")
	}

	var chassisList []Chassis
	totalReadings := 0

	// Iterate through all chassis using pre-determined strategy
	for i, ch := range chassis {
		if ch == nil {
			pr.logger.Warn("Skipping nil chassis", "index", i)
			continue
		}

		// Use pre-determined strategy
		var readings []Reading
		switch pr.strategy {
		case PowerSubsystemStrategy:
			readings, err = pr.readPowerSubsystem(ch)
		case PowerStrategy:
			readings, err = pr.readPower(ch)
		default:
			return nil, fmt.Errorf("unknown power reading strategy: %s", pr.strategy)
		}

		if err != nil {
			pr.logger.Warn("Failed to read power data from chassis",
				"chassis_id", ch.ID, "strategy", pr.strategy, "error", err)
			continue
		}

		if len(readings) > 0 {
			chassisData := Chassis{
				ID:       ch.ID,
				Readings: readings,
			}
			chassisList = append(chassisList, chassisData)
			totalReadings += len(readings)
		}
	}

	if len(chassisList) == 0 {
		return nil, fmt.Errorf("no chassis with valid power readings found")
	}

	pr.logger.Info("Successfully collected power readings",
		"endpoint", pr.endpoint, "strategy", pr.strategy,
		"chassis_count", len(chassisList), "total_readings", totalReadings)

	return chassisList, nil
}

// readPowerSubsystem attempts to read power data via PowerSubsystem API (modern approach)
func (pr *PowerReader) readPowerSubsystem(chassis *redfish.Chassis) ([]Reading, error) {
	// Get PowerSubsystem for this chassis
	powerSubsystem, err := chassis.PowerSubsystem()
	if err != nil {
		return nil, fmt.Errorf("failed to get power subsystem: %w", err)
	}

	if powerSubsystem == nil {
		return nil, fmt.Errorf("no power subsystem available")
	}

	// Get power supplies from the power subsystem
	powerSupplies, err := powerSubsystem.PowerSupplies()
	if err != nil {
		return nil, fmt.Errorf("failed to get power supplies: %w", err)
	}

	if len(powerSupplies) == 0 {
		return nil, fmt.Errorf("no power supplies found")
	}

	// Collect all power supply readings for this chassis
	var readings []Reading
	for j, powerSupply := range powerSupplies {
		// Skip power supplies with zero power output
		if powerSupply.PowerOutputWatts == 0 {
			pr.logger.Debug("Power output reading is zero for power supply",
				"chassis_id", chassis.ID, "power_supply_index", j, "member_id", powerSupply.ID)
			continue
		}

		reading := Reading{
			SourceID:   powerSupply.ID,
			SourceName: powerSupply.Name,
			SourceType: PowerSupplySource,
			Power:      Power(powerSupply.PowerOutputWatts) * device.Watt,
		}

		readings = append(readings, reading)

		pr.logger.Debug("Successfully read power from power supply",
			"endpoint", pr.endpoint,
			"chassis_id", chassis.ID,
			"power_supply_index", j,
			"member_id", powerSupply.ID,
			"name", powerSupply.Name,
			"power_output_watts", powerSupply.PowerOutputWatts,
			"power_input_watts", powerSupply.PowerInputWatts,
			"efficiency_percent", powerSupply.EfficiencyPercent)
	}

	if len(readings) == 0 {
		return nil, fmt.Errorf("no valid power readings found from power supplies")
	}

	return readings, nil
}

// readPower attempts to read power data via deprecated Power API (fallback)
func (pr *PowerReader) readPower(chassis *redfish.Chassis) ([]Reading, error) {
	power, err := chassis.Power()
	if err != nil {
		return nil, fmt.Errorf("failed to get power information: %w", err)
	}

	if power == nil || len(power.PowerControl) == 0 {
		return nil, fmt.Errorf("no power control information available")
	}

	// Collect all PowerControl entries for this chassis
	var readings []Reading
	for j, powerControl := range power.PowerControl {
		// Skip entries with zero power consumption
		if powerControl.PowerConsumedWatts == 0 {
			pr.logger.Debug("Power consumption reading is zero for PowerControl entry",
				"chassis_id", chassis.ID, "power_control_index", j, "member_id", powerControl.MemberID)
			continue
		}

		reading := Reading{
			SourceID:   powerControl.MemberID,
			SourceName: powerControl.Name,
			SourceType: PowerControlSource,
			Power:      Power(powerControl.PowerConsumedWatts) * device.Watt,
		}

		readings = append(readings, reading)

		pr.logger.Debug("Successfully read power from PowerControl entry",
			"endpoint", pr.endpoint,
			"chassis_id", chassis.ID,
			"power_control_index", j,
			"member_id", powerControl.MemberID,
			"name", powerControl.Name,
			"physical_context", powerControl.PhysicalContext,
			"power_watts", powerControl.PowerConsumedWatts)
	}

	if len(readings) == 0 {
		return nil, fmt.Errorf("no valid power readings found from power controls")
	}

	return readings, nil
}
