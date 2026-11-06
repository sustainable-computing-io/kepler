// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package device

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"sort"
	"strings"
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
		// Also update the reader's logger if it's the default sysfs reader
		if reader, ok := pm.reader.(*sysfsHwmonReader); ok {
			reader.logger = pm.logger
		}
	}
}

// WithHwmonZoneFilter sets zone names (power labels in hwmon) to include for monitoring
// If empty, all zones are included
func WithHwmonZoneFilter(zones []string) HwmonOptionFn {
	return func(pm *hwmonPowerMeter) {
		pm.zoneFilter = zones
	}
}

// ConfigChipRule defines a user-configurable chip pairing rule.
// This allows users to override hardcoded chip rules or add new chips via configuration.
type ConfigChipRule struct {
	// Name is the chip driver name (from hwmon "name" file), e.g., "ina3221", "ltc2945"
	Name string
	// Pairings maps voltage sensor index to current sensor index for explicit pairings
	// Key: voltage sensor index (in{N}), Value: current sensor index (curr{N})
	Pairings map[int]int
	// SkipVoltages lists voltage indices to skip (shunt voltages, aux inputs, etc.)
	SkipVoltages []int
	// SkipCurrents lists current indices to skip (current-only channels, per-phase, etc.)
	SkipCurrents []int
	// UseSameIndex indicates this chip uses same-index pairing (in{N} ↔ curr{N})
	UseSameIndex bool
}

// WithHwmonChipRules sets user-configurable chip pairing rules.
// These rules take precedence over hardcoded defaults.
func WithHwmonChipRules(rules []ConfigChipRule) HwmonOptionFn {
	return func(pm *hwmonPowerMeter) {
		// Update the reader's chip rules if it's the default sysfs reader
		if reader, ok := pm.reader.(*sysfsHwmonReader); ok {
			reader.configChipRules = rules
		}
	}
}

// NewHwmonPowerMeter creates a new hwmon-based power meter
func NewHwmonPowerMeter(sysfsPath string, opts ...HwmonOptionFn) (*hwmonPowerMeter, error) {
	logger := slog.Default().With("service", "hwmon")
	ret := &hwmonPowerMeter{
		reader: &sysfsHwmonReader{
			basePath: filepath.Join(sysfsPath, "class", "hwmon"),
			logger:   logger,
		},
		logger:     logger,
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

	// try reading from the first zone — try Power() first, then Energy()
	// Energy-only zones (like hwmon energy sensors) return error from Power()
	_, powerErr := zones[0].Power()
	if powerErr == nil {
		return nil
	}

	_, energyErr := zones[0].Energy()
	if energyErr == nil {
		return nil
	}

	return fmt.Errorf("first zone is not readable: power: %w, energy: %v", powerErr, energyErr)
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
		aggregated, _ := NewAggregatedZone(zones)
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
