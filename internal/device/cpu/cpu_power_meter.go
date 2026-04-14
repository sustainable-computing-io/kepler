// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package cpu

import (
	"github.com/sustainable-computing-io/kepler/internal/device"
)

// CPUPowerMeter implements powerMeter
type CPUPowerMeter interface {
	device.PowerMeter

	// Zones() returns a slice of the energy measurement zones
	Zones() ([]device.EnergyZone, error)

	// PrimaryEnergyZone() returns the zone with the highest energy coverage/priority
	// This zone represents the most comprehensive energy measurement available
	// E.g. Psys > Package > Core > DRAM > Uncore
	PrimaryEnergyZone() (device.EnergyZone, error)
}
