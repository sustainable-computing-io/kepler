// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package redfish

import (
	"time"

	"github.com/sustainable-computing-io/kepler/internal/device"
)

type (
	Energy = device.Energy
	Power  = device.Power
)

// SourceType indicates the API source of the power reading
type SourceType string

const (
	// PowerSupplySource indicates data from PowerSubsystem → PowerSupplies (modern API)
	PowerSupplySource SourceType = "PowerSupply"
	// PowerControlSource indicates data from Power → PowerControl (deprecated API)
	PowerControlSource SourceType = "PowerControl"
)

// PowerAPIStrategy defines the power reading strategy
type PowerAPIStrategy string

const (
	// UnknownStrategy indicates that the strategy has not been determined yet
	UnknownStrategy PowerAPIStrategy = ""
	// PowerSubsystemStrategy uses the modern PowerSubsystem API
	PowerSubsystemStrategy PowerAPIStrategy = "PowerSubsystem"
	// PowerStrategy uses the deprecated Power API
	PowerStrategy PowerAPIStrategy = "Power"
)

// Reading represents a power measurement from either PowerSubsystem (PowerSupply) or Power (PowerControl)
type Reading struct {
	SourceID   string     // PowerSupply MemberID or PowerControl MemberID
	SourceName string     // PowerSupply Name or PowerControl Name (optional)
	SourceType SourceType // API source: PowerSupply or PowerControl
	Power      Power      // Current power output/consumption in watts
}

// Chassis represents a single chassis with its power readings (PowerSupply or PowerControl)
type Chassis struct {
	ID       string    // Chassis ID for identification
	Readings []Reading // Power readings from this chassis (PowerSupply or PowerControl)
}

// PowerReading represents a collection of chassis with their power measurements and a single timestamp
type PowerReading struct {
	Timestamp time.Time // When the readings were taken
	Chassis   []Chassis // Chassis with their power readings (PowerSupply or PowerControl)
}

// Clone creates a deep copy of PowerReading for safe concurrent usage
func (pr *PowerReading) Clone() *PowerReading {
	if pr == nil {
		return nil
	}

	// Copy all non-pointer fields at once (Timestamp)
	ret := *pr

	// Deep copy the chassis slice and their readings
	ret.Chassis = make([]Chassis, len(pr.Chassis))
	for i, chassis := range pr.Chassis {
		ret.Chassis[i] = Chassis{
			ID:       chassis.ID,
			Readings: make([]Reading, len(chassis.Readings)),
		}
		// Deep copy the readings slice
		copy(ret.Chassis[i].Readings, chassis.Readings)
	}

	return &ret
}
