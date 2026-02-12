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

	// Scan for power sensors
	files, err := os.ReadDir(hwmonPath)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve power sensor files: %w", err)
	}

	var zones []EnergyZone

	// Look for power sensors only
	powerSensors := r.findSensorsByType(files, "power")
	for sensorNum, sensorFiles := range powerSensors {
		zone, err := r.createPowerZone(hwmonPath, chipName, humanName, sensorNum, sensorFiles)
		if err == nil {
			zones = append(zones, zone)
		}
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
