// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package device

import (
	"fmt"
	"log/slog"
	"strings"

	"k8s.io/utils/ptr"
)

// raplPowerMeter implements CPUPowerMeter with automatic MSR fallback support
type raplPowerMeter struct {
	reader      raplReader // Current active reader (powercap or MSR)
	cachedZones []EnergyZone
	logger      *slog.Logger
	zoneFilter  []string
	topZone     EnergyZone

	// Configuration for MSR fallback
	msrConfig MSRConfig
	sysfsPath string
	useMSR    bool // Track which backend is active
}

// MSRConfig holds MSR-specific configuration
type MSRConfig struct {
	Enabled    *bool
	Force      *bool
	DevicePath string
}

type OptionFn func(*raplPowerMeter)

// WithMSRConfig sets the MSR configuration for fallback behavior
func WithMSRConfig(msrConfig MSRConfig) OptionFn {
	return func(pm *raplPowerMeter) {
		pm.msrConfig = msrConfig
	}
}

// WithRaplReader sets a specific raplReader (for testing)
func WithRaplReader(r raplReader) OptionFn {
	return func(pm *raplPowerMeter) {
		pm.reader = r
	}
}

// WithRaplLogger sets the logger for raplPowerMeter
func WithRaplLogger(logger *slog.Logger) OptionFn {
	return func(pm *raplPowerMeter) {
		pm.logger = logger.With("service", "rapl")
	}
}

// WithZoneFilter sets zone names to include for monitoring
// If empty, all zones are included
func WithZoneFilter(zones []string) OptionFn {
	return func(pm *raplPowerMeter) {
		pm.zoneFilter = zones
	}
}

// NewCPUPowerMeter creates a new CPU power meter with MSR fallback support
func NewCPUPowerMeter(sysfsPath string, opts ...OptionFn) (*raplPowerMeter, error) {
	ret := &raplPowerMeter{
		logger:     slog.Default().With("service", "rapl"),
		zoneFilter: []string{},
		sysfsPath:  sysfsPath,
		// Default MSR configuration (disabled)
		msrConfig: MSRConfig{
			Enabled:    ptr.To(false),
			Force:      ptr.To(false),
			DevicePath: "/dev/cpu/%d/msr",
		},
	}

	for _, opt := range opts {
		opt(ret)
	}

	return ret, nil
}

func (r *raplPowerMeter) Name() string {
	if r.useMSR {
		return "rapl-msr"
	}
	return "rapl-powercap"
}

func (r *raplPowerMeter) Init() error {
	// Clear any cached state
	r.cachedZones = nil
	r.topZone = nil

	// If a specific reader is set (for testing), use it directly
	if r.reader != nil {
		r.logger.Info("Using provided power reader", "reader", r.reader.Name())
		return r.validateReader(r.reader)
	}

	// Determine which reader to use based on configuration and availability
	reader, useMSR, err := r.selectRaplReader()
	if err != nil {
		return fmt.Errorf("failed to select power reader: %w", err)
	}

	r.reader = reader
	r.useMSR = useMSR

	r.logger.Info("Selected power reader",
		"reader", r.reader.Name(),
		"msr_fallback", r.useMSR,
		"force_msr", ptr.Deref(r.msrConfig.Force, false))

	return r.validateReader(r.reader)
}

// selectRaplReader chooses the appropriate RAPL reader based on configuration and availability
func (r *raplPowerMeter) selectRaplReader() (raplReader, bool, error) {
	forceMSR := ptr.Deref(r.msrConfig.Force, false)
	enableFallback := ptr.Deref(r.msrConfig.Enabled, false)

	// If force MSR is enabled, use MSR directly (for testing)
	if forceMSR {
		r.logger.Info("MSR forced via configuration")
		msrReader := NewMSRReader(r.msrConfig.DevicePath, r.logger)
		if !msrReader.Available() {
			return nil, false, fmt.Errorf("MSR reader forced but not available")
		}
		if err := msrReader.Init(); err != nil {
			return nil, false, fmt.Errorf("failed to initialize forced MSR reader: %w", err)
		}
		return msrReader, true, nil
	}

	// Try powercap first (default behavior)
	powercapReader, err := NewPowercapReader(r.sysfsPath)
	if err == nil && powercapReader.Available() {
		if err := powercapReader.Init(); err == nil {
			r.logger.Debug("Using powercap reader")
			return powercapReader, false, nil
		} else {
			r.logger.Debug("Powercap reader initialization failed", "error", err)
		}
	} else {
		r.logger.Debug("Powercap reader not available", "error", err)
	}

	// If powercap failed and MSR fallback is enabled, try MSR
	if enableFallback {
		r.logger.Info("Attempting MSR fallback as powercap unavailable")

		// Log security warning for MSR usage
		r.logger.Warn("MSR fallback enabled - be aware of PLATYPUS attack vectors (CVE-2020-8694/8695)")

		msrReader := NewMSRReader(r.msrConfig.DevicePath, r.logger)
		if !msrReader.Available() {
			return nil, false, fmt.Errorf("neither powercap nor MSR readers are available")
		}
		if err := msrReader.Init(); err != nil {
			return nil, false, fmt.Errorf("MSR fallback failed to initialize: %w", err)
		}

		r.logger.Info("MSR fallback activated successfully")
		return msrReader, true, nil
	}

	// Neither powercap works nor MSR fallback is enabled
	return nil, false, fmt.Errorf("powercap unavailable and MSR fallback disabled")
}

// validateReader ensures the reader can provide valid energy readings
func (r *raplPowerMeter) validateReader(reader raplReader) error {
	zones, err := reader.Zones()
	if err != nil {
		return fmt.Errorf("failed to get zones from %s reader: %w", reader.Name(), err)
	}

	if len(zones) == 0 {
		return fmt.Errorf("no energy zones found from %s reader", reader.Name())
	}

	// Try reading energy from the first zone to verify functionality
	_, err = zones[0].Energy()
	if err != nil {
		return fmt.Errorf("failed to read energy from zone %s: %w", zones[0].Name(), err)
	}

	return nil
}

func (r *raplPowerMeter) needsFiltering() bool {
	return len(r.zoneFilter) != 0
}

// filterZones applies the configured zone filter
// If the filter is empty, all zones are returned
func (r *raplPowerMeter) filterZones(zones []EnergyZone) []EnergyZone {
	if !r.needsFiltering() {
		return zones
	}

	wanted := make(map[string]bool, len(r.zoneFilter))
	for _, name := range r.zoneFilter {
		wanted[strings.ToLower(name)] = true
	}
	var included, excluded []string
	filtered := make([]EnergyZone, 0, len(zones))
	for _, zone := range zones {
		if wanted[strings.ToLower(zone.Name())] {
			filtered = append(filtered, zone)
			included = append(included, zone.Name())
		} else {
			excluded = append(excluded, zone.Name())
		}
	}
	r.logger.Debug("Filtered RAPL zones", "included", included, "excluded", excluded)
	return filtered
}

func (r *raplPowerMeter) Zones() ([]EnergyZone, error) {
	// Return cached zones if already initialized
	if len(r.cachedZones) != 0 {
		return r.cachedZones, nil
	}

	if r.reader == nil {
		return nil, fmt.Errorf("power reader not initialized")
	}

	zones, err := r.reader.Zones()
	if err != nil {
		return nil, err
	} else if len(zones) == 0 {
		return nil, fmt.Errorf("no RAPL zones found")
	}

	zones = r.filterZones(zones)
	if len(zones) == 0 {
		return nil, fmt.Errorf("no RAPL zones found after filtering")
	}

	// filter out non-standard zones
	stdZoneMap := map[zoneKey]EnergyZone{}
	for _, zone := range zones {
		key := zoneKey{name: zone.Name(), index: zone.Index()}

		// ignore non-standard zones if a standard zone already exists
		if existingZone, exists := stdZoneMap[key]; exists && isStandardRaplPath(existingZone.Path()) {
			continue
		}
		stdZoneMap[key] = zone
	}

	// Group zones by name for aggregation
	r.cachedZones = r.groupZonesByName(stdZoneMap)
	return r.cachedZones, nil
}

// groupZonesByName groups zones by their base name and creates AggregatedZone
// instances when multiple zones share the same name (multi-socket systems)
func (r *raplPowerMeter) groupZonesByName(stdZoneMap map[zoneKey]EnergyZone) []EnergyZone {
	// Group zones by base name (e.g., "package", "dram")
	zoneGroups := make(map[string][]EnergyZone)

	for key, zone := range stdZoneMap {
		zoneGroups[key.name] = append(zoneGroups[key.name], zone)
	}

	// Create aggregated zones for duplicates, keep single zones as-is
	var result []EnergyZone
	for name, zones := range zoneGroups {
		if len(zones) == 1 {
			// Single zone - use as-is
			result = append(result, zones[0])
			continue

		}

		// Multiple zones with same name - create AggregatedZone
		aggregated := NewAggregatedZone(zones)
		result = append(result, aggregated)
		r.logger.Debug("Created aggregated zone",
			"name", name,
			"zone_count", len(zones),
			"zones", r.zoneNames(zones))
	}

	return result
}

// zoneNames returns a slice of zone names for logging
func (r *raplPowerMeter) zoneNames(zones []EnergyZone) []string {
	names := make([]string, len(zones))
	for i, zone := range zones {
		names[i] = fmt.Sprintf("%s-%d", zone.Name(), zone.Index())
	}
	return names
}

// PrimaryEnergyZone returns the zone with the highest energy coverage/priority
func (r *raplPowerMeter) PrimaryEnergyZone() (EnergyZone, error) {
	// Return cached zone if already initialized
	if r.topZone != nil {
		return r.topZone, nil
	}

	zones, err := r.Zones()
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

	// Priority hierarchy for RAPL zones (highest to lowest priority)
	priorityOrder := []string{"psys", "package", "core", "dram", "uncore"}

	// Find highest priority zone available
	for _, p := range priorityOrder {
		if zone, exists := zoneMap[p]; exists {
			r.topZone = zone
			return zone, nil
		}
	}

	// Fallback to first zone if none match our preferences
	r.topZone = zones[0]
	return zones[0], nil
}

// Close releases resources held by the power reader
func (r *raplPowerMeter) Close() error {
	if r.reader != nil {
		return r.reader.Close()
	}
	return nil
}

// isStandardRaplPath checks if a RAPL zone path is in the standard format
func isStandardRaplPath(path string) bool {
	// For powercap, check standard path format
	if strings.Contains(path, "/intel-rapl:") {
		return true
	}
	// For MSR, check MSR path format
	if strings.Contains(path, "/dev/cpu/") && strings.Contains(path, "/msr:") {
		return true
	}
	return false
}
