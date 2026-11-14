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

	"golang.org/x/sys/unix"
)

// sysfsHwmonReader implements hwmonReader by reading directly from sysfs
type sysfsHwmonReader struct {
	basePath        string // /sys/class/hwmon
	logger          *slog.Logger
	configChipRules []ConfigChipRule // User-configurable chip pairing rules (take precedence over hardcoded)
}

var (
	hwmonInvalidMetricChars = regexp.MustCompile("[^a-z0-9:_]")
)

func (r *sysfsHwmonReader) Zones() ([]EnergyZone, error) {
	hwmonDirs, err := os.ReadDir(r.basePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("hwmon not available: %w", err)
		}
		return nil, fmt.Errorf("failed to read hwmon directory: %w", err)
	}

	var zones []EnergyZone
	for _, entry := range hwmonDirs {
		// check for valid hwmon devices
		if !entry.IsDir() && !isSymlink(filepath.Join(r.basePath, entry.Name())) {
			continue
		}

		hwmonPath := filepath.Join(r.basePath, entry.Name())
		hwmonZones, err := r.discoverZones(hwmonPath)
		if err != nil {
			// Log but continue with other hwmon devices
			if r.logger != nil {
				r.logger.Warn("failed to discover zones for hwmon device", "path", hwmonPath, "error", err)
			}
			continue
		}
		zones = append(zones, hwmonZones...)
	}

	if len(zones) == 0 {
		return nil, fmt.Errorf("no hwmon power zones found")
	}

	return zones, nil
}

func (r *sysfsHwmonReader) discoverZones(hwmonPath string) ([]EnergyZone, error) {
	// Get chip name
	chipName, err := r.getChipName(hwmonPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get hardware monitor name: %w", err)
	}

	// Get human-readable chip name
	humanName, _ := r.getHumanReadableChipName(hwmonPath)

	// Scan for sensors
	files, err := os.ReadDir(hwmonPath)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve sensor files: %w", err)
	}

	var zones []EnergyZone

	// Tier 1: Energy sensors (preferred — cumulative µJ, like RAPL)
	energySensors := r.findSensorsByType(files, "energy")
	for sensorNum, sensorFiles := range energySensors {
		if !isSensorEnabled(hwmonPath, "energy", sensorNum, r.logger) {
			if r.logger != nil {
				r.logger.Debug("skipping disabled energy sensor", "chip", chipName, "sensor", sensorNum)
			}
			continue
		}
		zone, err := r.createEnergyZone(hwmonPath, chipName, humanName, sensorNum, sensorFiles)
		if err == nil {
			zones = append(zones, zone)
		}
	}

	// Tier 2: Direct power sensors (instantaneous µW)
	if len(zones) == 0 {
		powerSensors := r.findSensorsByType(files, "power")
		for sensorNum, sensorFiles := range powerSensors {
			// Skip disabled power channels (power*_enable = 0)
			if !isSensorEnabled(hwmonPath, "power", sensorNum, r.logger) {
				if r.logger != nil {
					r.logger.Debug("skipping disabled power channel", "index", sensorNum)
				}
				continue
			}

			zone, err := r.createPowerZone(hwmonPath, chipName, humanName, sensorNum, sensorFiles)
			if err == nil {
				zones = append(zones, zone)
			}
		}
	}

	// Tier 3: Voltage × current calculated power (fallback)
	if len(zones) == 0 {
		if r.logger != nil {
			r.logger.Info("trying current,voltage sensors", "chip", chipName)
		}
		calculatedZones, err := r.discoverVoltageCurrentZones(hwmonPath, chipName, humanName, files)
		if err != nil {
			// Return the error (e.g., ErrVoltageCurrentNoLabels)
			return nil, err
		}
		zones = append(zones, calculatedZones...)
	}

	return zones, nil
}

// findSensorsByType finds all sensors of a given type (e.g., "power")
// Returns a map of sensor number -> sensor files
func (r *sysfsHwmonReader) findSensorsByType(files []os.DirEntry, sensorType string) map[int]map[string]string {
	sensors := make(map[int]map[string]string)
	pattern := regexp.MustCompile(fmt.Sprintf(`^%s(\d+)_(.+)$`, sensorType))

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		matches := pattern.FindStringSubmatch(file.Name())
		if len(matches) != 3 {
			continue
		}

		sensorNum, _ := strconv.Atoi(matches[1])
		property := matches[2]

		if sensors[sensorNum] == nil {
			sensors[sensorNum] = make(map[string]string)
		}
		sensors[sensorNum][property] = file.Name()
	}

	return sensors
}

func (r *sysfsHwmonReader) createPowerZone(
	hwmonPath, chipName, humanName string,
	sensorNum int,
	sensorFiles map[string]string,
) (EnergyZone, error) {
	// Determine the zone name from label or generate one
	var zoneName string
	if labelFile, hasLabel := sensorFiles["label"]; hasLabel {
		labelData, err := os.ReadFile(filepath.Join(hwmonPath, labelFile))
		if err == nil {
			zoneName = strings.TrimSpace(string(labelData))
			zoneName = cleanMetricName(zoneName)
		}
	}

	// Fallback to chip name + power + sensor number - node exporter strategy
	if zoneName == "" {
		zoneName = fmt.Sprintf("%s_power%d", chipName, sensorNum)
	}

	// Get the input file - prefer "average" over "input" if available - can be changed
	var inputFile string
	if file, ok := sensorFiles["average"]; ok {
		inputFile = file
	} else if file, ok := sensorFiles["input"]; ok {
		inputFile = file
	} else {
		return nil, fmt.Errorf("no input file for power sensor")
	}

	inputPath := filepath.Join(hwmonPath, inputFile)

	return &hwmonPowerZone{
		name:      zoneName,
		index:     sensorNum,
		path:      inputPath,
		chipName:  chipName,
		humanName: humanName,
	}, nil
}

func (r *sysfsHwmonReader) createEnergyZone(
	hwmonPath, chipName, humanName string,
	sensorNum int,
	sensorFiles map[string]string,
) (EnergyZone, error) {
	// Determine the zone name from label or generate one
	var zoneName string
	if labelFile, hasLabel := sensorFiles["label"]; hasLabel {
		labelData, err := os.ReadFile(filepath.Join(hwmonPath, labelFile))
		if err == nil {
			zoneName = strings.TrimSpace(string(labelData))
			zoneName = cleanMetricName(zoneName)
		}
	}

	// Fallback to chip name + energy + sensor number
	if zoneName == "" {
		zoneName = fmt.Sprintf("%s_energy%d", chipName, sensorNum)
	}

	// Require energy*_input file
	inputFile, hasInput := sensorFiles["input"]
	if !hasInput {
		return nil, fmt.Errorf("no input file for energy sensor")
	}

	inputPath := filepath.Join(hwmonPath, inputFile)

	return &hwmonEnergyZone{
		name:      zoneName,
		index:     sensorNum,
		path:      inputPath,
		chipName:  chipName,
		humanName: humanName,
	}, nil
}

func (r *sysfsHwmonReader) getChipName(hwmonPath string) (string, error) {
	// Strategy from node_exporter:
	// 1. Try to construct name from device path (most stable)
	// 2. Fall back to "name" file
	// 3. Fall back to hwmon directory name

	devicePath, err := filepath.EvalSymlinks(filepath.Join(hwmonPath, "device"))
	if err == nil {
		devPathPrefix, devName := filepath.Split(devicePath)
		_, devType := filepath.Split(strings.TrimRight(devPathPrefix, "/"))

		cleanDevName := cleanMetricName(devName)
		cleanDevType := cleanMetricName(devType)

		if cleanDevType != "" && cleanDevName != "" {
			return cleanDevType + "_" + cleanDevName, nil
		}

		if cleanDevName != "" {
			return cleanDevName, nil
		}
	}

	// Try name file
	nameData, err := os.ReadFile(filepath.Join(hwmonPath, "name"))
	if err == nil && len(nameData) > 0 {
		cleanName := cleanMetricName(string(nameData))
		if cleanName != "" {
			return cleanName, nil
		}
	}

	// Fall back to directory name
	realDir, err := filepath.EvalSymlinks(hwmonPath)
	if err != nil {
		return "", err
	}

	_, name := filepath.Split(realDir)
	cleanName := cleanMetricName(name)
	if cleanName != "" {
		return cleanName, nil
	}

	return "", fmt.Errorf("could not derive chip name for %s", hwmonPath)
}

func (r *sysfsHwmonReader) getHumanReadableChipName(hwmonPath string) (string, error) {
	nameData, err := os.ReadFile(filepath.Join(hwmonPath, "name"))
	if err != nil {
		return "", err
	}

	if len(nameData) > 0 {
		cleanName := cleanMetricName(string(nameData))
		if cleanName != "" {
			return cleanName, nil
		}
	}

	return "", fmt.Errorf("could not derive human-readable chip name for %s", hwmonPath)
}

func cleanMetricName(name string) string {
	lower := strings.ToLower(name)
	replaced := hwmonInvalidMetricChars.ReplaceAllLiteralString(lower, "_")
	cleaned := strings.Trim(replaced, "_")
	return cleaned
}

func isSymlink(path string) bool {
	fi, err := os.Lstat(path)
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeSymlink != 0
}

// sysReadFile is a simplified os.ReadFile that invokes syscall.Read directly.
func sysReadFile(file string) ([]byte, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	// On some machines, hwmon drivers are broken and return EAGAIN.  This causes
	// Go's os.ReadFile implementation to poll forever.
	//
	// Since we either want to read data or bail immediately, do the simplest
	// possible read using system call directly.
	b := make([]byte, 128)
	n, err := unix.Read(int(f.Fd()), b)
	if err != nil {
		return nil, err
	}
	if n < 0 {
		return nil, fmt.Errorf("failed to read file: %q, read returned negative bytes value: %d", file, n)
	}

	return b[:n], nil
}
