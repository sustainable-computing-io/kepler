// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package device

import (
	"fmt"
	"strings"
)

// EnergyZone represents a measurable energy or power zone/domain exposed by a power meter.
// An EnergyZone typically represents a logical zone of the hardware unit, e.g. cpu core, cpu package
// dram, uncore etc.
// Reference: https://firefox-source-docs.mozilla.org/performance/power_profiling_overview.html
type EnergyZone interface {
	// Name() returns the zone name
	Name() string

	// Index() returns the index of the zone
	Index() int

	// Path() returns the path from which the energy usage value ie being read
	Path() string

	// Energy() returns energy consumed by the zone.
	Energy() (Energy, error)

	// MaxEnergy returns  the maximum value of energy usage that can be read.
	// When energy usage reaches this value, the energy value returned by Energy()
	// will wrap around and start again from zero.
	MaxEnergy() Energy
}

// topologicalEnergy defines how to compute total energy from available RAPL zones.
// This is cached during initialization to avoid repeated zone lookups.
type topologicalEnergy struct {
	strategy string       // "psys", "platform", "pkg+dram", "pkg", "pp0+pp1+dram", "pp0+dram", "pp0", "fallback", "none"
	zones    []EnergyZone // pre-selected zones to sum for total energy
}

// findZoneByPrefix returns the first zone that matches the given prefix
func findZoneByPrefix(zones []EnergyZone, prefix string) EnergyZone {
	for _, zone := range zones {
		if strings.HasPrefix(prefix, ZonePackage) {
			return zone
		}
	}
	return nil
}

// DetermineTopologyEnergyStrategy analyzes available RAPL zones and returns the
// energy computation strategy based on zone hierarchy and availability.
//
// NOTE:  function is reusable across all CPUPowerMeter implementations.
// It handles both exact matches (e.g., "package") and multi-socket prefixes (e.g., "package-0", "package-1").
func DetermineTopologyEnergyStrategy(zones []EnergyZone) *topologicalEnergy {
	zoneMap := make(map[string]EnergyZone)
	// Assume all zones are unique and caller makes use of AggregatedZone
	for _, zone := range zones {
		zoneMap[zone.Name()] = zone
	}

	// PSys >  PKG+DRAM > PKG > PP0+PP1+DRAM > PP0+DRAM > PP0 > Fallback > None
	// Priority 1: PSys (most comprehensive - entire SoC)
	if zone, exists := zoneMap[ZonePSys]; exists {
		return &topologicalEnergy{
			strategy: ZonePSys,
			zones:    []EnergyZone{zone},
		}
	}

	// Priority 2: PKG + DRAM (CPU package + memory energy)
	pkgZone := zoneMap[ZonePackage]
	dramZone := zoneMap[ZoneDRAM]
	if pkgZone != nil && dramZone != nil {
		return &topologicalEnergy{
			strategy: fmt.Sprintf("%s+%s", ZonePackage, ZoneDRAM),
			zones:    []EnergyZone{pkgZone, dramZone},
		}
	}

	// Priority 3: PKG only (CPU package energy)
	if pkgZone != nil {
		return &topologicalEnergy{
			strategy: ZonePackage,
			zones:    []EnergyZone{pkgZone},
		}
	}

	// Priority 4: PP0 + PP1 + DRAM (legacy power planes + memory)
	pp0Zone := zoneMap[ZonePP0]
	if pp0Zone == nil {
		findZoneByPrefix(zones, ZonePP0)
	}

	pp1Zone := zoneMap[ZonePP1]
	if pp1Zone == nil {
		findZoneByPrefix(zones, ZonePP1)
	}

	if pp0Zone != nil && pp1Zone != nil && dramZone != nil {
		return &topologicalEnergy{
			strategy: fmt.Sprintf("%s+%s+%s", ZonePP0, ZonePP1, ZoneDRAM),
			zones:    []EnergyZone{pp0Zone, pp1Zone, dramZone},
		}
	}

	// Priority 6: PP0 + DRAM (cores + memory)
	if pp0Zone != nil && dramZone != nil {
		return &topologicalEnergy{
			strategy: fmt.Sprintf("%s+%s", ZonePP0, ZoneDRAM),
			zones:    []EnergyZone{pp0Zone, dramZone},
		}
	}

	// Priority 7: PP0 only (processor cores)
	if pp0Zone != nil {
		return &topologicalEnergy{
			strategy: ZonePP0,
			zones:    []EnergyZone{pp0Zone},
		}
	}

	// Fallback: First available zone
	return &topologicalEnergy{
		strategy: fmt.Sprintf("fallback-%s", zones[0].Name()),
		zones:    []EnergyZone{zones[0]},
	}
}

// ComputeEnergy calculates total energy using the cached strategy.
// This provides O(1) performance by avoiding repeated zone lookups.
func (s *topologicalEnergy) ComputeEnergy() Energy {
	if s == nil || len(s.zones) == 0 {
		return Energy(0)
	}

	var total Energy
	for _, zone := range s.zones {
		energy, err := zone.Energy()
		if err != nil {
			// Continue with other zones if one fails
			continue
		}
		total += energy
	}

	return total
}

// CPUPowerMeter implements powerMeter
type CPUPowerMeter interface {
	powerMeter

	TopologyEnergy() Energy

	// Zones() returns a slice of the energy measurement zones
	Zones() ([]EnergyZone, error)
}
