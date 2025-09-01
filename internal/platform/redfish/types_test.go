// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package redfish

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/sustainable-computing-io/kepler/internal/device"
)

func TestPowerReadingCloneNil(t *testing.T) {
	var pr *PowerReading = nil

	result := pr.Clone()

	assert.Nil(t, result)
}

func TestPowerReadingCloneSuccess(t *testing.T) {
	timestamp := time.Now()
	original := &PowerReading{
		Timestamp: timestamp,
		Chassis: []Chassis{
			{
				ID: "chassis-1",
				Readings: []Reading{
					{
						ControlID: "PC1",
						Name:      "Chassis 1 Power Control",
						Power:     100.5 * device.Watt,
					},
				},
			},
			{
				ID: "chassis-2",
				Readings: []Reading{
					{
						ControlID: "PC1",
						Name:      "Chassis 2 Power Control",
						Power:     200.3 * device.Watt,
					},
				},
			},
		},
	}

	cloned := original.Clone()

	// Verify cloned is not nil and is a different instance
	assert.NotNil(t, cloned)
	assert.NotSame(t, original, cloned)

	// Verify timestamp is copied correctly
	assert.Equal(t, timestamp, cloned.Timestamp)

	// Verify chassis slice is copied correctly
	assert.Len(t, cloned.Chassis, 2)
	assert.Equal(t, original.Chassis[0].ID, cloned.Chassis[0].ID)
	assert.Equal(t, original.Chassis[0].Readings[0].Power, cloned.Chassis[0].Readings[0].Power)
	assert.Equal(t, original.Chassis[0].Readings[0].ControlID, cloned.Chassis[0].Readings[0].ControlID)
	assert.Equal(t, original.Chassis[1].Readings[0].Power, cloned.Chassis[1].Readings[0].Power)
	assert.Equal(t, original.Chassis[1].Readings[0].ControlID, cloned.Chassis[1].Readings[0].ControlID)

	// Verify it's a deep copy - modifying original shouldn't affect clone
	original.Chassis[0].Readings[0].Power = 999 * device.Watt
	original.Chassis[0].Readings[0].ControlID = "modified"

	assert.Equal(t, 100.5*device.Watt, cloned.Chassis[0].Readings[0].Power)
	assert.Equal(t, "PC1", cloned.Chassis[0].Readings[0].ControlID)
}

func TestPowerReadingCloneEmpty(t *testing.T) {
	timestamp := time.Now()
	original := &PowerReading{
		Timestamp: timestamp,
		Chassis:   []Chassis{}, // empty slice
	}

	cloned := original.Clone()

	// Verify cloned is not nil and is a different instance
	assert.NotNil(t, cloned)
	assert.NotSame(t, original, cloned)

	// Verify timestamp is copied correctly
	assert.Equal(t, timestamp, cloned.Timestamp)

	// Verify empty chassis slice is handled correctly
	assert.NotNil(t, cloned.Chassis)
	assert.Len(t, cloned.Chassis, 0)
}
