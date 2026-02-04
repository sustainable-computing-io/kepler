// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package device

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

// hwmonPowerMeter implements CPUPowerMeter using hwmon sysfs
type hwmonPowerMeter struct {
	reader      hwmonReader
	cachedZones []EnergyZone
	logger      *slog.Logger
	zoneFilter  []string
	topZone     EnergyZone
}

// HwmonOptionFn is a function that configures hwmonPowerMeter options
type HwmonOptionFn func(*hwmonPowerMeter)

// hwmonReader is an interface for reading hwmon data, used for mocking in tests
type hwmonReader interface {
	Zones() ([]EnergyZone, error)
}

// WithHwmonReader sets the hwmonReader to be used by hwmonPowerMeter
func WithHwmonReader(r hwmonReader) HwmonOptionFn {
	return func(pm *hwmonPowerMeter) {
		pm.reader = r
	}
}

// WithHwmonLogger sets the logger for hwmonPowerMeter
func WithHwmonLogger(logger *slog.Logger) HwmonOptionFn {
	return func(pm *hwmonPowerMeter) {
		pm.logger = logger.With("service", "hwmon")
	}
}

// WithHwmonZoneFilter sets zone names (power labels in hwmon) to include for monitoring
// If empty, all zones are included
func WithHwmonZoneFilter(zones []string) HwmonOptionFn {
	return func(pm *hwmonPowerMeter) {
		pm.zoneFilter = zones
	}
}

// NewHwmonPowerMeter creates a new hwmon-based power meter
func NewHwmonPowerMeter(sysfsPath string, opts ...HwmonOptionFn) (*hwmonPowerMeter, error) {
	ret := &hwmonPowerMeter{
		reader:     &sysfsHwmonReader{basePath: filepath.Join(sysfsPath, "class", "hwmon")},
		logger:     slog.Default().With("service", "hwmon"),
		zoneFilter: []string{},
	}

	for _, opt := range opts {
		opt(ret)
	}

	return ret, nil
}

func (h *hwmonPowerMeter) Name() string {
	return "hwmon"
}

func (h *hwmonPowerMeter) Init() error {
	// ensure hwmon zones can be read but don't cache them
	zones, err := h.reader.Zones()
	if err != nil {
		return err
	} else if len(zones) == 0 {
		return fmt.Errorf("no hwmon power zones found")
	}

	// try reading power from the first zone and return any error
	_, err = zones[0].Power()
	return err
}

func (h *hwmonPowerMeter) needsZoneFiltering() bool {
	return len(h.zoneFilter) != 0
}

func (h *hwmonPowerMeter) Zones() ([]EnergyZone, error) {
	// Return cached zones if already initialized
	if len(h.cachedZones) != 0 {
		return h.cachedZones, nil
	}

	zones, err := h.reader.Zones()
	if err != nil {
		return nil, err
	} else if len(zones) == 0 {
		return nil, fmt.Errorf("no hwmon zones found")
	}

	zones = h.filterZones(zones)
	if len(zones) == 0 {
		return nil, fmt.Errorf("no hwmon zones found after filtering")
	}

	// Group zones by name for potential aggregation
	h.cachedZones = h.groupZonesByName(zones)
	return h.cachedZones, nil
}

// filterZones applies zone filters
func (h *hwmonPowerMeter) filterZones(zones []EnergyZone) []EnergyZone {
	if !h.needsZoneFiltering() {
		return zones
	}

	zoneWanted := make(map[string]bool)
	for _, name := range h.zoneFilter {
		zoneWanted[strings.ToLower(name)] = true
	}

	var included, excluded []string
	filtered := make([]EnergyZone, 0, len(zones))

	for _, zone := range zones {
		// Check zone filter
		if !zoneWanted[strings.ToLower(zone.Name())] {
			excluded = append(excluded, zone.Name())
			continue
		}

		filtered = append(filtered, zone)
		included = append(included, zone.Name())
	}

	h.logger.Debug("Filtered hwmon zones", "included", included, "excluded", excluded)
	return filtered
}

// groupZonesByName groups zones by their base name and creates AggregatedZone
// instances when multiple zones share the same name
func (h *hwmonPowerMeter) groupZonesByName(zones []EnergyZone) []EnergyZone {
	// Group zones by base name
	zoneGroups := make(map[string][]EnergyZone)

	for _, zone := range zones {
		name := zone.Name()
		zoneGroups[name] = append(zoneGroups[name], zone)
	}

	// Create aggregated zones for duplicates, keep single zones as-is
	var result []EnergyZone
	for name, zones := range zoneGroups {
		if len(zones) == 1 {
			result = append(result, zones[0])
			continue
		}

		// Multiple zones with same name - create AggregatedZone
		// LIMITATION: aggregation occurs when the devices are different with coincidentally
		// the same labels. This should not happen. Ideally, Kepler identifies whether the zones with same
		// name occur due to multi-socket CPU or independent devices.
		aggregated := NewAggregatedZone(zones)
		result = append(result, aggregated)
		h.logger.Debug("Created aggregated zone",
			"name", name,
			"zone_count", len(zones))
	}

	// Sort by name for deterministic ordering
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name() < result[j].Name()
	})

	return result
}

// PrimaryEnergyZone returns the zone with the highest energy coverage/priority
func (h *hwmonPowerMeter) PrimaryEnergyZone() (EnergyZone, error) {
	// Return cached zone if already initialized
	if h.topZone != nil {
		return h.topZone, nil
	}

	zones, err := h.Zones()
	if err != nil {
		return nil, err
	}

	if len(zones) == 0 {
		return nil, fmt.Errorf("no energy zones available")
	}

	zoneMap := map[string]EnergyZone{}
	for _, zone := range zones {
		zoneMap[strings.ToLower(zone.Name())] = zone
	}

	// Priority hierarchy for common hwmon zones (highest to lowest priority)
	// This may need adjustment based on actual hwmon device naming
	// Needs to be modified to include CPU_Power and IO power
	priorityOrder := []string{
		"package", "socket", "cpu", "core",
		"ppt", "ppt0", "ppt1", // AMD PPT (Package Power Tracking)
		"core_power", "io_power",
		"soc", "platform",
		"gpu", "vddgfx",
	} // likely needs to be modified

	// Find highest priority zone available
	for _, p := range priorityOrder {
		if zone, exists := zoneMap[p]; exists {
			h.topZone = zone
			return zone, nil
		}
	}

	// Fallback to first zone if none match our preferences
	h.topZone = zones[0]
	return zones[0], nil
}

// sysfsHwmonReader implements hwmonReader by reading directly from sysfs
type sysfsHwmonReader struct {
	basePath string // /sys/class/hwmon
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

	// First, look for direct power sensors (preferred)
	powerSensors := r.findSensorsByType(files, "power")
	for sensorNum, sensorFiles := range powerSensors {
		zone, err := r.createPowerZone(hwmonPath, chipName, humanName, sensorNum, sensorFiles)
		if err == nil {
			zones = append(zones, zone)
		}
	}

	// If no direct power sensors found, try voltage/current pairs as fallback
	if len(zones) == 0 {
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
	// Get the pairing rule for this chip
	rule := getChipPairingRule(humanName)
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
) *hwmonCalculatedPowerZone {
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

	return &hwmonCalculatedPowerZone{
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

// currentSensorInfo holds information about a current sensor for matching
type currentSensorInfo struct {
	index       int
	inputPath   string // curr*_input path
	averagePath string // curr*_average path (may be empty)
	label       string
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

// hwmonPowerZone implements EnergyZone for hwmon power sensors
// It provides direct power readings without time integration
type hwmonPowerZone struct {
	name      string
	index     int
	path      string
	chipName  string
	humanName string
}

// hwmonCalculatedPowerZone implements EnergyZone by calculating power from
// voltage and current sensors when direct power readings are not available.
// Power is calculated as: voltage (mV) × current (mA) = power (µW)
type hwmonCalculatedPowerZone struct {
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

func (z *hwmonCalculatedPowerZone) Name() string {
	return z.name
}

func (z *hwmonCalculatedPowerZone) Index() int {
	return z.index
}

func (z *hwmonCalculatedPowerZone) Path() string {
	// Return voltage path as the primary identifier
	return z.voltagePath
}

func (z *hwmonCalculatedPowerZone) Energy() (Energy, error) {
	// Calculated power zones do not provide energy readings
	return 0, fmt.Errorf("hwmon calculated power zones do not provide energy readings")
}

func (z *hwmonCalculatedPowerZone) MaxEnergy() Energy {
	// No maximum for calculated power zones
	return 0
}

func (z *hwmonCalculatedPowerZone) Power() (Power, error) {
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
