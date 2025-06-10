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

// NodeUsage contains energy consumption data of a node. This is different to Usage in that it has idle/active split
type NodeUsage struct {
	EnergyTotal Energy // Cumulative joules counter
	Power       Power  // Current power in watts

	// Split of Delta Energy between Active and Idle
	ActiveEnergyTotal Energy // Cumulative energy counter for active workloads
	ActivePower       Power  // portion of the total power that is being used by the workload

	IdleEnergyTotal Energy // Cumulative energy counter for idle workloads
	IdlePower       Power  // portion of the total power that allocated to node idling

	// NOTE: activeEnergy is an internal variable that is used to calculate workload's energy
	activeEnergy Energy // Energy used by the workload running
}

// Usage contains energy consumption data of workloads (Process, Container, VM)
// This is different to NodeUsage in that it does not have idle/active split
type Usage struct {
	EnergyTotal Energy // Cumulative joules counter
	Power       Power  // Current power in watts
}

// ZoneUsageMap maps energy zones to basic usage data (absolute energy and power).
// Used by processes, containers, and VMs which only track their attributed energy consumption.
type ZoneUsageMap map[EnergyZone]*Usage

// NodeZoneUsageMap maps energy zones to node-specific usage data that includes idle/used breakdown.
// Used exclusively by Node to track total energy consumption with attribution between active workloads
// and idle system overhead, enabling proper power attribution calculations.
type NodeZoneUsageMap map[EnergyZone]*NodeUsage

type Node struct {
	Timestamp  time.Time        // Timestamp of the last measurement
	UsageRatio float64          // ratio of usage
	Zones      NodeZoneUsageMap // Map of zones to usage
}

func (n *Node) Clone() *Node {
	ret := &Node{
		Timestamp:  n.Timestamp,
		UsageRatio: n.UsageRatio,
		Zones:      make(NodeZoneUsageMap, len(n.Zones)),
	}
	maps.Copy(ret.Zones, n.Zones)
	return ret
}

// Process represents the power consumption of a process
type Process struct {
	PID  int
	Comm string
	Exe  string

	Type resource.ProcessType

	CPUTotalTime float64 // CPU time in seconds

	Zones ZoneUsageMap

	ContainerID      string // empty if not a container
	VirtualMachineID string // empty if not a virtual machine
}

func (p *Process) Clone() *Process {
	ret := &Process{
		PID:  p.PID,
		Comm: p.Comm,
		Exe:  p.Exe,
		Type: p.Type,

		CPUTotalTime: p.CPUTotalTime,
		Zones:        make(ZoneUsageMap, len(p.Zones)),

		ContainerID:      p.ContainerID,
		VirtualMachineID: p.VirtualMachineID,
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

type Hypervisor = resource.Hypervisor

// VirtualMachine represents the power consumption of a VM
type VirtualMachine struct {
	ID   string // VM ID
	Name string // VM name

	Hypervisor Hypervisor

	CPUTotalTime float64 // CPU time in seconds

	Zones ZoneUsageMap
}

func (vm *VirtualMachine) Clone() *VirtualMachine {
	ret := &VirtualMachine{
		ID:           vm.ID,
		Name:         vm.Name,
		Hypervisor:   vm.Hypervisor,
		CPUTotalTime: vm.CPUTotalTime,

		Zones: make(ZoneUsageMap, len(vm.Zones)),
	}
	maps.Copy(ret.Zones, vm.Zones)
	return ret
}

type (
	Processes       = map[int]*Process
	Containers      = map[string]*Container
	VirtualMachines = map[string]*VirtualMachine
)

// Snapshot encapsulates power monitoring data
type Snapshot struct {
	Timestamp time.Time // Timestamp of the snapshot
	Node      *Node     // Node power data

	Processes       Processes       // Process power data, keyed by PID
	Containers      Containers      // Container power data, keyed by container ID
	VirtualMachines VirtualMachines // VM power data, keyed by container ID
}

// NewSnapshot creates a new Snapshot instance
func NewSnapshot() *Snapshot {
	return &Snapshot{
		// Timestamp: time.Time{}, // Zero value to indicate unset
		Node: &Node{
			Zones: make(NodeZoneUsageMap),
		},
		Processes:       make(Processes),
		Containers:      make(Containers),
		VirtualMachines: make(VirtualMachines),
	}
}

func (s *Snapshot) Clone() *Snapshot {
	clone := &Snapshot{
		Timestamp:       s.Timestamp,
		Node:            s.Node.Clone(),
		Processes:       make(Processes, len(s.Processes)),
		Containers:      make(Containers, len(s.Containers)),
		VirtualMachines: make(VirtualMachines, len(s.VirtualMachines)),
	}

	// Deep copy the processes map
	for pid, src := range s.Processes {
		clone.Processes[pid] = src.Clone()
	}

	for id, src := range s.Containers {
		clone.Containers[id] = src.Clone()
	}

	for id, src := range s.VirtualMachines {
		clone.VirtualMachines[id] = src.Clone()
	}

	return clone
}
