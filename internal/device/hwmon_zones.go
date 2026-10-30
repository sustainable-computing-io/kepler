// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package device

import (
	"fmt"
	"strconv"
	"strings"
)

// hwmonPowerZone implements EnergyZone for hwmon power sensors
// It provides direct power readings without time integration
type hwmonPowerZone struct {
	name      string
	index     int
	path      string
	chipName  string
	humanName string
}

// hwmonVoltageCurrentZone implements EnergyZone by calculating power from
// voltage and current sensors when direct power readings are not available.
// Power is calculated as: voltage (mV) × current (mA) = power (µW)
type hwmonVoltageCurrentZone struct {
	name        string
	index       int
	voltagePath string // Path to in*_input file (millivolts)
	currentPath string // Path to curr*_input file (milliamperes)
	chipName    string
	humanName   string
}

func (z *hwmonPowerZone) Name() string {
	return z.name
}

func (z *hwmonPowerZone) Index() int {
	return z.index
}

func (z *hwmonPowerZone) Path() string {
	return z.path
}

func (z *hwmonPowerZone) Energy() (Energy, error) {
	// hwmon provides power, not energy
	// Return 0 for interface compatibility
	return 0, fmt.Errorf("hwmon zones do not provide energy readings")
}

func (z *hwmonPowerZone) MaxEnergy() Energy {
	// No maximum for power sensors
	return 0
}

func (z *hwmonPowerZone) Power() (Power, error) {
	// Read current power value using direct syscall to avoid EAGAIN polling issues
	data, err := sysReadFile(z.path)
	if err != nil {
		return 0, fmt.Errorf("failed to read power from %s: %w", z.path, err)
	}

	valueStr := strings.TrimSpace(string(data))
	powerMicrowatts, err := strconv.ParseUint(valueStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse power value from %s: %w", z.path, err)
	}

	// Power type represents microwatts
	return Power(powerMicrowatts), nil
}

func (z *hwmonVoltageCurrentZone) Name() string {
	return z.name
}

func (z *hwmonVoltageCurrentZone) Index() int {
	return z.index
}

func (z *hwmonVoltageCurrentZone) Path() string {
	// Return voltage path as the primary identifier
	return z.voltagePath
}

func (z *hwmonVoltageCurrentZone) Energy() (Energy, error) {
	// Calculated power zones do not provide energy readings
	return 0, fmt.Errorf("hwmon calculated power zones do not provide energy readings")
}

func (z *hwmonVoltageCurrentZone) MaxEnergy() Energy {
	// No maximum for calculated power zones
	return 0
}

// hwmonEnergyZone implements EnergyZone for hwmon energy sensors.
// These provide cumulative energy readings (energy*_input in microjoules),
// functionally identical to RAPL energy counters.
type hwmonEnergyZone struct {
	name      string
	index     int
	path      string // energy*_input file path
	chipName  string
	humanName string
}

func (z *hwmonEnergyZone) Name() string {
	return z.name
}

func (z *hwmonEnergyZone) Index() int {
	return z.index
}

func (z *hwmonEnergyZone) Path() string {
	return z.path
}

func (z *hwmonEnergyZone) Energy() (Energy, error) {
	data, err := sysReadFile(z.path)
	if err != nil {
		return 0, fmt.Errorf("failed to read energy from %s: %w", z.path, err)
	}

	valueStr := strings.TrimSpace(string(data))
	microjoules, err := strconv.ParseUint(valueStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse energy value from %s: %w", z.path, err)
	}

	return Energy(microjoules), nil
}

func (z *hwmonEnergyZone) MaxEnergy() Energy {
	// hwmon energy sensors have no max_energy_range equivalent.
	// The monitor detects energy zones via energyReading > 0 as fallback.
	return 0
}

func (z *hwmonEnergyZone) Power() (Power, error) {
	// Energy zones provide cumulative energy, not instantaneous power
	return 0, fmt.Errorf("hwmon energy zones do not provide instantaneous power readings")
}

func (z *hwmonVoltageCurrentZone) Power() (Power, error) {
	// Read voltage in millivolts
	voltageData, err := sysReadFile(z.voltagePath)
	if err != nil {
		return 0, fmt.Errorf("failed to read voltage from %s: %w", z.voltagePath, err)
	}

	voltageStr := strings.TrimSpace(string(voltageData))
	voltageMV, err := strconv.ParseUint(voltageStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse voltage value from %s: %w", z.voltagePath, err)
	}

	// Read current in milliamperes
	currentData, err := sysReadFile(z.currentPath)
	if err != nil {
		return 0, fmt.Errorf("failed to read current from %s: %w", z.currentPath, err)
	}

	currentStr := strings.TrimSpace(string(currentData))
	currentMA, err := strconv.ParseUint(currentStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse current value from %s: %w", z.currentPath, err)
	}

	// Calculate power: voltage (mV) × current (mA) = power (µW)
	// Example: 12000 mV × 5000 mA = 60,000,000 µW = 60 W
	powerMicrowatts := voltageMV * currentMA

	return Power(powerMicrowatts), nil
}
