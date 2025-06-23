// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package device

import (
	"fmt"
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

// zoneState tracks the energy state of an individual zone for delta calculation
type zoneState struct {
	lastReading       Energy
	accumulatedEnergy Energy
}

// AggregatedZone implements EnergyZone interface by aggregating multiple zones
// of the same type (e.g., multiple package zones in multi-socket systems).
// It handles energy counter wrapping for each individual zone and provides
// a single consolidated energy reading.
type AggregatedZone struct {
	name       string
	index      int
	zones      []EnergyZone
	zoneStates map[string]*zoneState
	maxEnergy  Energy // Cached sum of all zone MaxEnergy values
	mu         sync.RWMutex
}

// NewAggregatedZone creates a new AggregatedZone for zones of the same type
func NewAggregatedZone(name string, zones []EnergyZone) *AggregatedZone {
	// Calculate and cache the combined MaxEnergy during construction
	var totalMax Energy
	for _, zone := range zones {
		totalMax += zone.MaxEnergy()
	}

	return &AggregatedZone{
		name:       name,
		index:      -1, // Indicates this is an aggregated zone
		zones:      zones,
		zoneStates: make(map[string]*zoneState),
		maxEnergy:  totalMax, // Cache the combined MaxEnergy
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

	var totalEnergy Energy
	hasValidReading := false

	for _, zone := range az.zones {
		currentReading, err := zone.Energy()
		if err != nil {
			// Continue with other zones if one fails
			continue
		}

		zoneID := fmt.Sprintf("%s-%d", zone.Name(), zone.Index())
		state := az.stateForID(zoneID, currentReading)

		// Calculate energy delta since last reading
		var delta Energy
		if currentReading >= state.lastReading {
			// Normal case: no wrap
			delta = currentReading - state.lastReading
		} else {
			// Wrap occurred: calculate delta across wrap boundary
			delta = (zone.MaxEnergy() - state.lastReading) + currentReading
		}

		// Accumulate delta and add to total
		state.accumulatedEnergy += delta
		totalEnergy += state.accumulatedEnergy

		// Update state for next reading
		state.lastReading = currentReading
		hasValidReading = true
	}

	if !hasValidReading {
		return 0, fmt.Errorf("no valid energy readings from aggregated zones")
	}

	// Wrap totalEnergy at MaxEnergy boundary for consistent delta calculation
	if az.maxEnergy > 0 {
		totalEnergy = totalEnergy % az.maxEnergy
	}

	return totalEnergy, nil
}

// MaxEnergy returns the cached sum of maximum energy values across all zones
// This provides the correct wrap boundary for delta calculations
func (az *AggregatedZone) MaxEnergy() Energy {
	return az.maxEnergy
}

// stateForID returns the existing state or creates the state for a specific zone
func (az *AggregatedZone) stateForID(zoneID string, currentReading Energy) *zoneState {
	if state, exists := az.zoneStates[zoneID]; exists {
		return state
	}

	// Initialize state with current reading as baseline
	az.zoneStates[zoneID] = &zoneState{
		lastReading:       currentReading,
		accumulatedEnergy: currentReading, // Start with current reading as initial energy
	}
	return az.zoneStates[zoneID]
}
