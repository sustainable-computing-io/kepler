// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package device

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// ErrVoltageCurrentNoLabels is returned when voltage and current sensors exist
// but cannot be matched because they lack labels
var ErrVoltageCurrentNoLabels = fmt.Errorf("voltage and current sensors found but no matching labels available for power calculation")

// discoverVoltageCurrentZones finds voltage/current sensor pairs that can be used
// to calculate power when direct power sensors are not available.
//
// Matching priority (per Linux hwmon Power Sensor Reference):
//  1. Label-based matching - most robust, driver-agnostic
//  2. Known-chip lookup - uses chip-specific pairing rules from driver table
//  3. Same-index fallback - works for majority of drivers
//
// Returns nil if no voltage/current sensors exist.
// Only returns error if sensors exist but cannot be matched.
func (r *sysfsHwmonReader) discoverVoltageCurrentZones(
	hwmonPath, chipName, humanName string,
	files []os.DirEntry,
) ([]EnergyZone, error) {
	// Find voltage sensors (in*) and current sensors (curr*)
	voltageSensors := r.findSensorsByType(files, "in")
	currentSensors := r.findSensorsByType(files, "curr")

	if len(voltageSensors) == 0 || len(currentSensors) == 0 {
		return nil, nil
	}

	// Build voltage sensor info maps (by index and by label)
	voltageByIndex := make(map[int]voltageSensorInfo)
	voltageLabelMap := make(map[string]voltageSensorInfo)

	for sensorNum, sensorFiles := range voltageSensors {
		// Need at least input file
		inputFile, hasInput := sensorFiles["input"]
		if !hasInput {
			continue
		}

		// Skip disabled voltage channels (in*_enable = 0)
		if !isSensorEnabled(hwmonPath, "in", sensorNum, r.logger) {
			if r.logger != nil {
				r.logger.Debug("skipping disabled voltage channel", "index", sensorNum)
			}
			continue
		}

		info := voltageSensorInfo{
			index:     sensorNum,
			inputPath: filepath.Join(hwmonPath, inputFile),
		}

		// Store average path if available
		if averageFile, hasAverage := sensorFiles["average"]; hasAverage {
			info.averagePath = filepath.Join(hwmonPath, averageFile)
		}

		// Try to get label
		if labelFile, hasLabel := sensorFiles["label"]; hasLabel {
			labelData, err := os.ReadFile(filepath.Join(hwmonPath, labelFile))
			if err == nil {
				label := strings.TrimSpace(string(labelData))
				cleanLabel := cleanMetricName(label)
				if cleanLabel != "" {
					info.label = cleanLabel
					voltageLabelMap[cleanLabel] = info
				}
			}
		}

		voltageByIndex[sensorNum] = info
	}

	// Build current sensor info maps (by index and by label)
	currentByIndex := make(map[int]currentSensorInfo)
	currentLabelMap := make(map[string]currentSensorInfo)

	for sensorNum, sensorFiles := range currentSensors {
		// Need at least input file
		inputFile, hasInput := sensorFiles["input"]
		if !hasInput {
			continue
		}

		// Skip disabled current channels (curr*_enable = 0)
		if !isSensorEnabled(hwmonPath, "curr", sensorNum, r.logger) {
			if r.logger != nil {
				r.logger.Debug("skipping disabled current channel", "index", sensorNum)
			}
			continue
		}

		info := currentSensorInfo{
			index:     sensorNum,
			inputPath: filepath.Join(hwmonPath, inputFile),
		}

		// Store average path if available
		if averageFile, hasAverage := sensorFiles["average"]; hasAverage {
			info.averagePath = filepath.Join(hwmonPath, averageFile)
		}

		// Try to get label
		if labelFile, hasLabel := sensorFiles["label"]; hasLabel {
			labelData, err := os.ReadFile(filepath.Join(hwmonPath, labelFile))
			if err == nil {
				label := strings.TrimSpace(string(labelData))
				cleanLabel := cleanMetricName(label)
				if cleanLabel != "" {
					info.label = cleanLabel
					currentLabelMap[cleanLabel] = info
				}
			}
		}

		currentByIndex[sensorNum] = info
	}

	var zones []EnergyZone

	// PRIORITY 1: Label-based matching
	// This is the most robust method - labels explicitly identify matching sensors
	zones = r.matchByLabel(chipName, humanName, voltageLabelMap, currentLabelMap)
	if len(zones) > 0 {
		return zones, nil
	}

	// PRIORITY 2: Known-chip lookup
	// Use chip-specific pairing rules from the driver table
	zones = r.matchByChipRule(chipName, humanName, voltageByIndex, currentByIndex)
	if len(zones) > 0 {
		return zones, nil
	}

	// PRIORITY 3: Same-index fallback
	// Works for the majority of drivers that use matching indices
	zones = r.matchBySameIndex(chipName, humanName, voltageByIndex, currentByIndex)
	if len(zones) > 0 {
		return zones, nil
	}

	// No matching strategy worked - return error
	return nil, ErrVoltageCurrentNoLabels
}

// matchByLabel matches voltage and current sensors by their labels.
// This is the most robust method as labels explicitly identify matching sensors.
func (r *sysfsHwmonReader) matchByLabel(
	chipName, humanName string,
	voltageLabelMap map[string]voltageSensorInfo,
	currentLabelMap map[string]currentSensorInfo,
) []EnergyZone {
	var zones []EnergyZone

	for label, voltageInfo := range voltageLabelMap {
		currentInfo, found := currentLabelMap[label]
		if !found {
			continue
		}

		zone := r.createCalculatedZone(
			chipName, humanName,
			label, currentInfo.index,
			voltageInfo, currentInfo,
		)
		zones = append(zones, zone)
	}

	return zones
}

// matchByChipRule uses chip-specific pairing rules to match sensors.
// This handles known edge cases where indices don't match directly.
func (r *sysfsHwmonReader) matchByChipRule(
	chipName, humanName string,
	voltageByIndex map[int]voltageSensorInfo,
	currentByIndex map[int]currentSensorInfo,
) []EnergyZone {
	// Get the pairing rule for this chip (config rules take precedence over hardcoded)
	rule := getChipPairingRule(humanName, r.configChipRules)
	if rule == nil {
		return nil
	}

	var zones []EnergyZone

	if rule.useSameIndex {
		// Same-index pairing with skip rules
		for vIdx, voltageInfo := range voltageByIndex {
			if rule.shouldSkipVoltage(vIdx) {
				continue
			}

			currentInfo, found := currentByIndex[vIdx]
			if !found {
				continue
			}

			if rule.shouldSkipCurrent(vIdx) {
				continue
			}

			zoneName := r.generateZoneName(chipName, vIdx, voltageInfo.label)
			zone := r.createCalculatedZone(
				chipName, humanName,
				zoneName, vIdx,
				voltageInfo, currentInfo,
			)
			zones = append(zones, zone)
		}
	} else if rule.pairings != nil {
		// Explicit pairings from the rule table
		for vIdx, cIdx := range rule.pairings {
			voltageInfo, vFound := voltageByIndex[vIdx]
			if !vFound {
				continue
			}

			currentInfo, cFound := currentByIndex[cIdx]
			if !cFound {
				continue
			}

			zoneName := r.generateZoneName(chipName, vIdx, voltageInfo.label)
			zone := r.createCalculatedZone(
				chipName, humanName,
				zoneName, vIdx,
				voltageInfo, currentInfo,
			)
			zones = append(zones, zone)
		}
	}

	return zones
}

// matchBySameIndex matches voltage and current sensors by their index numbers.
// This is the fallback method that works for the majority of hwmon drivers.
func (r *sysfsHwmonReader) matchBySameIndex(
	chipName, humanName string,
	voltageByIndex map[int]voltageSensorInfo,
	currentByIndex map[int]currentSensorInfo,
) []EnergyZone {
	var zones []EnergyZone

	for idx, voltageInfo := range voltageByIndex {
		currentInfo, found := currentByIndex[idx]
		if !found {
			continue
		}

		zoneName := r.generateZoneName(chipName, idx, voltageInfo.label)
		zone := r.createCalculatedZone(
			chipName, humanName,
			zoneName, idx,
			voltageInfo, currentInfo,
		)
		zones = append(zones, zone)
	}

	return zones
}

// generateZoneName creates a zone name from label or chip name + index
func (r *sysfsHwmonReader) generateZoneName(chipName string, idx int, label string) string {
	if label != "" {
		return label
	}
	return fmt.Sprintf("%s_power%d", chipName, idx)
}

// createCalculatedZone creates a calculated power zone from voltage and current sensor info.
// It handles the logic of choosing between input and average files.
func (r *sysfsHwmonReader) createCalculatedZone(
	chipName, humanName, zoneName string,
	index int,
	voltageInfo voltageSensorInfo,
	currentInfo currentSensorInfo,
) *hwmonVoltageCurrentZone {
	// Determine which file type to use for both sensors
	// Use "average" only if BOTH voltage and current have it, otherwise use "input" for both
	var voltagePath, currentPath string
	voltageHasAverage := voltageInfo.averagePath != ""
	currentHasAverage := currentInfo.averagePath != ""

	if voltageHasAverage && currentHasAverage {
		// Both have average - use average for both
		voltagePath = voltageInfo.averagePath
		currentPath = currentInfo.averagePath
	} else {
		// Use input for both (consistent readings)
		voltagePath = voltageInfo.inputPath
		currentPath = currentInfo.inputPath
	}

	return &hwmonVoltageCurrentZone{
		name:        zoneName,
		index:       index,
		voltagePath: voltagePath,
		currentPath: currentPath,
		chipName:    chipName,
		humanName:   humanName,
	}
}

// voltageSensorInfo holds information about a voltage sensor for label matching
type voltageSensorInfo struct {
	index       int
	inputPath   string // in*_input path
	averagePath string // in*_average path (may be empty)
	label       string
}

// isSensorEnabled checks if a sensor channel is enabled.
// Returns true if:
// - The enable file doesn't exist (followup check of the channel will confirm if it is accessible)
// - The enable file exists and contains "1"
// - The enable file exists but cannot be read (continue to test if the channel itself can be accessed)
// Returns false if the enable file exists and contains "0" (disabled)
func isSensorEnabled(hwmonPath, sensorType string, sensorNum int, logger *slog.Logger) bool {
	enableFile := filepath.Join(hwmonPath, fmt.Sprintf("%s%d_enable", sensorType, sensorNum))
	data, err := os.ReadFile(enableFile)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if logger != nil {
				logger.Warn("enable file not present, assuming sensor is enabled",
					"path", enableFile)
			}
			return true
		}
		// File exists but cannot be read (e.g., permission denied)
		// Fail open: assume enabled so the downstream data read can be attempted
		if logger != nil {
			logger.Warn("could not read enable file, assuming sensor is enabled",
				"path", enableFile, "error", err)
		}
		return true
	}

	value := strings.TrimSpace(string(data))
	// Channel is enabled if value is "1", disabled if "0"
	return value != "0"
}

// currentSensorInfo holds information about a current sensor for matching
type currentSensorInfo struct {
	index       int
	inputPath   string // curr*_input path
	averagePath string // curr*_average path (may be empty)
	label       string
}
