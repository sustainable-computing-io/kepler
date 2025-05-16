// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package monitor

import (
	"maps"
	"time"

	"github.com/sustainable-computing-io/kepler/internal/device"
	"github.com/sustainable-computing-io/kepler/internal/resource"
)

type (
	Energy     = device.Energy
	Power      = device.Power
	EnergyZone = device.EnergyZone
)

const (
	Joule = device.Joule
	Watt  = device.Watt
)

// Usage contains energy consumption data
type Usage struct {
	Absolute Energy // Cumulative joules counter
	Delta    Energy // Difference since last measurement
	Power    Power  // Current power in watts
}

// ZoneUsageMap maps zones to energy data
type ZoneUsageMap map[EnergyZone]*Usage

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

// Process represents the power consumption of a process
type Process struct {
	PID  int
	Comm string
	Exe  string

	CPUTotalTime float64 // CPU time in seconds

	// Replace single Usage with ZoneUsageMap
	Zones ZoneUsageMap

	ContainerID string
}

func (p *Process) Clone() *Process {
	ret := &Process{
		PID:  p.PID,
		Comm: p.Comm,
		Exe:  p.Exe,

		CPUTotalTime: p.CPUTotalTime,
		Zones:        make(ZoneUsageMap, len(p.Zones)),

		ContainerID: p.ContainerID,
	}
	maps.Copy(ret.Zones, p.Zones)
	return ret
}

type ContainerRuntime = resource.ContainerRuntime

// Container represents the power consumption of a container
type Container struct {
	ID   string // Container ID
	Name string // Container name

	Runtime ContainerRuntime // Container runtime

	CPUTotalTime float64 // CPU time in seconds

	// Replace single Usage with ZoneUsageMap
	Zones ZoneUsageMap
}

func (c *Container) Clone() *Container {
	ret := &Container{
		ID:           c.ID,
		Name:         c.Name,
		Runtime:      c.Runtime,
		CPUTotalTime: c.CPUTotalTime,

		Zones: make(ZoneUsageMap, len(c.Zones)),
	}
	maps.Copy(ret.Zones, c.Zones)
	return ret
}

type (
	Processes  = map[int]*Process
	Containers = map[string]*Container
)

// Snapshot encapsulates power monitoring data
type Snapshot struct {
	Timestamp time.Time // Timestamp of the snapshot
	Node      *Node     // Node power data

	Processes  Processes  // Process power data, keyed by PID
	Containers Containers // Container power data, keyed by container ID
}

// NewSnapshot creates a new Snapshot instance
func NewSnapshot() *Snapshot {
	return &Snapshot{
		// Timestamp: time.Time{}, // Zero value to indicate unset
		Node: &Node{
			Zones: make(ZoneUsageMap),
		},
		Processes:  make(map[int]*Process),
		Containers: make(map[string]*Container),
	}
}

func (s *Snapshot) Clone() *Snapshot {
	clone := &Snapshot{
		Timestamp:  s.Timestamp,
		Node:       s.Node.Clone(),
		Processes:  make(map[int]*Process, len(s.Processes)),
		Containers: make(map[string]*Container, len(s.Containers)),
	}

	// Deep copy the processes map
	for pid, src := range s.Processes {
		clone.Processes[pid] = src.Clone()
	}

	for id, src := range s.Containers {
		clone.Containers[id] = src.Clone()
	}

	return clone
}
