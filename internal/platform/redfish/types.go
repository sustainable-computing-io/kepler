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

// Reading represents a single PowerControl entry measurement
type Reading struct {
	ControlID string // PowerControl MemberID
	Name      string // PowerControl Name (optional)
	Power     Power  // Current power consumption in watts
}

// Chassis represents a single chassis with its PowerControl readings
type Chassis struct {
	ID       string    // Chassis ID for identification
	Readings []Reading // PowerControl readings from this chassis
}

// PowerReading represents a collection of chassis with their power measurements and a single timestamp
type PowerReading struct {
	Timestamp time.Time // When the readings were taken
	Chassis   []Chassis // Chassis with their PowerControl readings
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
