// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package device

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// sysfsIIOReader implements hwmonReader by reading power data from the Linux
// IIO (Industrial I/O) subsystem. This provides a fallback for devices where
// power monitoring chips are bound to an IIO driver instead of hwmon.
//
// Primary use case: NVIDIA Jetson Nano (L4T R32) where the INA3221 power
// monitor is bound to the "ina3221x" IIO driver rather than the upstream
// "ina3221" hwmon driver.
//
// IIO power sensors expose data under /sys/bus/iio/devices/iio:deviceN/
// with attributes like in_power{N}_input (milliwatts), rail_name_{N}, etc.
type sysfsIIOReader struct {
	basePath string // /sys/bus/iio/devices
	logger   *slog.Logger
}

var iioDevicePattern = regexp.MustCompile(`^iio:device\d+$`)

func (r *sysfsIIOReader) Zones() ([]EnergyZone, error) {
	entries, err := os.ReadDir(r.basePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("iio not available: %w", err)
		}
		return nil, fmt.Errorf("failed to read iio directory: %w", err)
	}

	var zones []EnergyZone
	for _, entry := range entries {
		name := entry.Name()
		if !iioDevicePattern.MatchString(name) {
			// Also check if it's a symlink to an iio device
			if !isSymlink(filepath.Join(r.basePath, name)) {
				continue
			}
			if !iioDevicePattern.MatchString(name) {
				continue
			}
		}

		devicePath := filepath.Join(r.basePath, name)
		deviceZones, err := r.discoverIIOZones(devicePath)
		if err != nil {
			if r.logger != nil {
				r.logger.Debug("skipping iio device", "path", devicePath, "error", err)
			}
			continue
		}
		zones = append(zones, deviceZones...)
	}

	if len(zones) == 0 {
		return nil, fmt.Errorf("no iio power zones found")
	}

	return zones, nil
}

// discoverIIOZones scans an IIO device directory for power sensors.
// It looks for in_power{N}_input files and uses rail_name_{N} for zone labels.
func (r *sysfsIIOReader) discoverIIOZones(devicePath string) ([]EnergyZone, error) {
	// Verify this is a power monitoring device by checking for name file
	nameData, err := os.ReadFile(filepath.Join(devicePath, "name"))
	if err != nil {
		return nil, fmt.Errorf("no name file: %w", err)
	}
	chipName := strings.TrimSpace(string(nameData))
	if chipName == "" {
		return nil, fmt.Errorf("empty chip name")
	}

	// Scan for in_power{N}_input files
	files, err := os.ReadDir(devicePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read device directory: %w", err)
	}

	powerPattern := regexp.MustCompile(`^in_power(\d+)_input$`)
	var zones []EnergyZone

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		matches := powerPattern.FindStringSubmatch(file.Name())
		if len(matches) != 2 {
			continue
		}

		sensorIdx, err := strconv.Atoi(matches[1])
		if err != nil {
			continue
		}

		// Try to get rail name for this channel
		zoneName := r.getRailName(devicePath, sensorIdx)
		if zoneName == "" {
			zoneName = fmt.Sprintf("%s_power%d", cleanMetricName(chipName), sensorIdx)
		}

		powerPath := filepath.Join(devicePath, file.Name())
		zones = append(zones, &iioPowerZone{
			name:     zoneName,
			index:    sensorIdx,
			path:     powerPath,
			chipName: chipName,
		})
	}

	if len(zones) == 0 {
		return nil, fmt.Errorf("no power sensors found in iio device %s", chipName)
	}

	if r.logger != nil {
		names := make([]string, len(zones))
		for i, z := range zones {
			names[i] = z.Name()
		}
		r.logger.Info("discovered iio power zones",
			"chip", chipName,
			"zones", names,
			"count", len(zones))
	}

	return zones, nil
}

// getRailName reads the rail_name_{N} file for a given sensor index.
func (r *sysfsIIOReader) getRailName(devicePath string, index int) string {
	railFile := filepath.Join(devicePath, fmt.Sprintf("rail_name_%d", index))
	data, err := os.ReadFile(railFile)
	if err != nil {
		return ""
	}
	name := strings.TrimSpace(string(data))
	if name == "" {
		return ""
	}
	return cleanMetricName(name)
}

// iioPowerZone implements EnergyZone for IIO power sensors.
// IIO power sensors report instantaneous power in milliwatts (mW),
// unlike hwmon which uses microwatts (µW). The Power() method
// converts to microwatts for consistency with the rest of Kepler.
type iioPowerZone struct {
	name     string
	index    int
	path     string // in_power{N}_input file path
	chipName string
}

func (z *iioPowerZone) Name() string {
	return z.name
}

func (z *iioPowerZone) Index() int {
	return z.index
}

func (z *iioPowerZone) Path() string {
	return z.path
}

func (z *iioPowerZone) Energy() (Energy, error) {
	return 0, fmt.Errorf("iio power zones do not provide energy readings")
}

func (z *iioPowerZone) MaxEnergy() Energy {
	return 0
}

// Power reads the IIO power sensor and returns the value in microwatts.
// IIO in_power{N}_input reports milliwatts; this method converts to
// microwatts (×1000) to match Kepler's internal Power type (µW).
func (z *iioPowerZone) Power() (Power, error) {
	data, err := sysReadFile(z.path)
	if err != nil {
		return 0, fmt.Errorf("failed to read iio power from %s: %w", z.path, err)
	}

	valueStr := strings.TrimSpace(string(data))
	powerMilliwatts, err := strconv.ParseUint(valueStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse iio power value from %s: %w", z.path, err)
	}

	// Convert milliwatts to microwatts: 1 mW = 1000 µW
	return Power(powerMilliwatts) * MilliWatt, nil
}
