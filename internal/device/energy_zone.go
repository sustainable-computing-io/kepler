// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package device

import (
	"fmt"
	"math"
	"sync"
)

type Zone = string

const (
	ZonePackage Zone = "package"
	ZoneCore    Zone = "core"
	ZoneDRAM    Zone = "dram"
	ZoneUncore  Zone = "uncore"
	ZonePSys    Zone = "psys"
	ZonePP0     Zone = "pp0" // Power Plane 0 - processor cores
	ZonePP1     Zone = "pp1" // Power Plane 1 - uncore (e.g., integrated GPU)
)

// zoneKey uniquely identifies a zone by name and index
type zoneKey struct {
	name  string
	index int
}

// AggregatedZone implements EnergyZone interface by aggregating multiple zones
// of the same type (e.g., multiple package zones in multi-socket systems).
// It handles energy counter wrapping for each individual zone and provides
// a single consolidated energy reading.
type AggregatedZone struct {
	name          string
	index         int
	zones         []EnergyZone
	lastReadings  map[zoneKey]Energy
	currentEnergy Energy // Aggregated energy counter
	maxEnergy     Energy // Cached sum of all zone MaxEnergy values
	mu            sync.RWMutex
}

// NewAggregatedZone creates a new AggregatedZone for zones of the same type
// The name is taken from the first zone
// Panics if zones is empty or nil
func NewAggregatedZone(zones []EnergyZone) *AggregatedZone {
	// Panic on invalid inputs
	if len(zones) == 0 {
		panic("NewAggregatedZone: zones cannot be empty")
	}

	// Use the first zone's name as the aggregated zone name
	name := zones[0].Name()
	// Calculate and cache the combined MaxEnergy during construction
	// Check for overflow when summing MaxEnergy values
	var totalMax Energy
	for _, zone := range zones {
		zoneMax := zone.MaxEnergy()
		// Check for overflow before adding
		if totalMax > 0 && zoneMax > math.MaxUint64-totalMax {
			// Overflow would occur, use MaxUint64 as safe maximum
			totalMax = Energy(math.MaxUint64)
			break
		}
		totalMax += zoneMax
	}

	return &AggregatedZone{
		name:          name,
		index:         -1, // Indicates this is an aggregated zone
		zones:         zones,
		lastReadings:  make(map[zoneKey]Energy),
		currentEnergy: 0,
		maxEnergy:     totalMax, // Cache the combined MaxEnergy
	}
}

// Name returns the zone name
func (az *AggregatedZone) Name() string {
	return az.name
}

// Index returns the zone index (-1 for aggregated zones)
func (az *AggregatedZone) Index() int {
	return az.index
}

// Path returns path for the aggregated zone
func (az *AggregatedZone) Path() string {
	// TODO: decide if all the paths should be returned
	return fmt.Sprintf("aggregated-%s", az.name)
}

// Energy returns the total energy consumption across all aggregated zones,
// handling wrap-around for each individual zone
func (az *AggregatedZone) Energy() (Energy, error) {
	az.mu.Lock()
	defer az.mu.Unlock()

	var totalDelta Energy

	for _, zone := range az.zones {
		currentReading, err := zone.Energy()
		if err != nil {
			return 0, fmt.Errorf("no valid energy readings from aggregated zones - %s: %w", zone.Name(), err)
		}

		zoneID := zoneKey{zone.Name(), zone.Index()}

		if lastReading, exists := az.lastReadings[zoneID]; exists {

			// Calculate delta since last reading
			var delta Energy
			if currentReading >= lastReading {
				// Normal case: no wrap
				delta = currentReading - lastReading
			} else {
				// Wrap occurred: calculate delta across wrap boundary
				// Only if zone has valid MaxEnergy (> 0)
				if zone.MaxEnergy() > 0 {
					delta = (zone.MaxEnergy() - lastReading) + currentReading
				} else {
					// Invalid MaxEnergy, treat as normal delta (might be negative)
					delta = currentReading - lastReading
				}
			}
			totalDelta += delta
		} else {
			// First reading: use current reading as initial energy
			totalDelta += currentReading
		}

		// Update last reading
		az.lastReadings[zoneID] = currentReading
	}

	// Update aggregated energy counter
	az.currentEnergy += totalDelta

	// Wrap at maxEnergy boundary to match hardware counter behavior
	// This is required for the power attribution algorithm's calculateEnergyDelta()
	if az.maxEnergy > 0 {
		az.currentEnergy %= az.maxEnergy
	}

	return az.currentEnergy, nil
}

// MaxEnergy returns the cached sum of maximum energy values across all zones
// This provides the correct wrap boundary for delta calculations
func (az *AggregatedZone) MaxEnergy() Energy {
	return az.maxEnergy
}
