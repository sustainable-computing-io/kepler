/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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
	Watts    Power  // Current power in watts
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
		Timestamp: time.Now(),
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
