// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package device

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

// CPUPowerMeter implements powerMeter
type CPUPowerMeter interface {
	powerMeter

	// Zones() returns a slice of the energy measurement zones
	Zones() ([]EnergyZone, error)

	// PrimaryEnergyZone() returns the zone with the highest energy coverage/priority
	// This zone represents the most comprehensive energy measurement available
	// E.g. Psys > Package > Core > DRAM > Uncore
	PrimaryEnergyZone() (EnergyZone, error)
}
