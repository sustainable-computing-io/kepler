// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package monitor

import (
	"maps"
	"time"

	"github.com/sustainable-computing-io/kepler/internal/device"
)

type (
	Energy     = device.Energy
	Power      = device.Power
	EnergyZone = device.EnergyZone
)

// Usage contains energy consumption data
type Usage struct {
	Absolute Energy // Cumulative joules counter
	Delta    Energy // Difference since last measurement
	Power    Power  // Current power in watts
}

// ZoneUsageMap maps zones to energy data
type ZoneUsageMap map[EnergyZone]Usage

type Node struct {
	Timestamp time.Time    // Timestamp of the last measurement
	Zones     ZoneUsageMap // Map of zones to usage
}

func (n *Node) Clone() *Node {
	ret := &Node{
		Timestamp: n.Timestamp,
		Zones:     make(ZoneUsageMap, len(n.Zones)),
	}
	maps.Copy(ret.Zones, n.Zones)
	return ret
}

// Snapshot encapsulates power monitoring data
type Snapshot struct {
	Timestamp time.Time // Timestamp of the snapshot
	Node      *Node
}

// NewSnapshot creates a new Snapshot instance
func NewSnapshot() *Snapshot {
	return &Snapshot{
		// Timestamp: time.Time{}, // Zero value to indicate unset
		Node: &Node{
			Zones: make(ZoneUsageMap),
		},
	}
}

func (s *Snapshot) Clone() *Snapshot {
	return &Snapshot{
		Timestamp: s.Timestamp,
		Node:      s.Node.Clone(),
	}
}
