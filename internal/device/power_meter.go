// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package device

import "github.com/sustainable-computing-io/kepler/internal/service"

// PowerMeter is a hardware backend that reads energy or power from a class
// of hardware (CPU package, GPU device, etc).
//
// Many PowerMeters can be selected. Each contributes its own readings.
// Domain-specific methods live on subinterfaces that embed PowerMeter.
type PowerMeter interface {
	service.Service     // Name()
	service.Initializer // Init()
}

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

	// Power() returns the current power consumption by the zone.
	// This method is used for zones that provide instantaneous power readings.
	Power() (Power, error)
}
