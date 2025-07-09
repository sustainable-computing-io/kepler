// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package monitor

import (
	"maps"
	"strconv"
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
	ActivePower       Power  // portion of the total power that is being used by the Resource

	IdleEnergyTotal Energy // Cumulative energy counter for idle workloads
	IdlePower       Power  // portion of the total power that allocated to node idling

	// NOTE: activeEnergy is an internal variable that is used to calculate Resource's energy
	activeEnergy Energy // Energy used by the Resource running
}

// Usage contains energy consumption data of workloads (Process, Container, VM)
// This is different to NodeUsage in that it does not have idle/active split
type Usage struct {
	EnergyTotal Energy // Cumulative joules counter
	Power       Power  // Current power in watts
}

// ZoneUsageMap maps energy zones to basic usage data (absolute energy and power).
// Used by processes, containers, and VMs which only track their attributed energy consumption.
type ZoneUsageMap map[EnergyZone]Usage

// NodeZoneUsageMap maps energy zones to node-specific usage data that includes idle/used breakdown.
// Used exclusively by Node to track total energy consumption with attribution between active workloads
// and idle system overhead, enabling proper power attribution calculations.
type NodeZoneUsageMap map[EnergyZone]NodeUsage

type Node struct {
	Timestamp  time.Time        // Timestamp of the last measurement
	UsageRatio float64          // ratio of usage
	Zones      NodeZoneUsageMap // Map of zones to usage
}

func (n *Node) Clone() *Node {
	if n == nil {
		return nil
	}
	ret := *n
	ret.Zones = make(NodeZoneUsageMap, len(n.Zones))
	maps.Copy(ret.Zones, n.Zones)
	return &ret
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
	if p == nil {
		return nil
	}

	ret := *p
	ret.Zones = make(ZoneUsageMap, len(p.Zones))
	maps.Copy(ret.Zones, p.Zones)
	return &ret
}

// ZonesUsage implements the Resource interface
func (p *Process) ZoneUsage() ZoneUsageMap {
	return p.Zones
}

// StringID implements the Resource interface
func (p *Process) StringID() string {
	return strconv.Itoa(p.PID)
}

type ContainerRuntime = resource.ContainerRuntime

// Container represents the power consumption of a container
type Container struct {
	ID   string // Container ID
	Name string // Container name

	Runtime ContainerRuntime // Container runtime

	CPUTotalTime float64 // CPU time in seconds

	Zones ZoneUsageMap

	// pod id is empty if the container is not a pod
	PodID string
}

func (c *Container) Clone() *Container {
	if c == nil {
		return nil
	}

	ret := *c
	ret.Zones = make(ZoneUsageMap, len(c.Zones))
	maps.Copy(ret.Zones, c.Zones)
	return &ret
}

// ZoneUsage implements the Resource interface
func (c *Container) ZoneUsage() ZoneUsageMap {
	return c.Zones
}

// StringID implements the Resource interface
func (c *Container) StringID() string {
	return c.ID
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
	if vm == nil {
		return nil
	}

	ret := *vm
	ret.Zones = make(ZoneUsageMap, len(vm.Zones))
	maps.Copy(ret.Zones, vm.Zones)
	return &ret
}

// ZoneUsage implements the Resource interface
func (vm *VirtualMachine) ZoneUsage() ZoneUsageMap {
	return vm.Zones
}

// StringID implements the Resource interface
func (vm *VirtualMachine) StringID() string {
	return vm.ID
}

type Pod struct {
	ID        string // Pod UUID
	Name      string // Pod Name
	Namespace string // Pod Namespace

	CPUTotalTime float64 // CPU time in seconds

	// Replace single Usage with ZoneUsageMap
	Zones ZoneUsageMap
}

func (p *Pod) Clone() *Pod {
	if p == nil {
		return nil
	}

	ret := *p
	ret.Zones = make(ZoneUsageMap, len(p.Zones))
	maps.Copy(ret.Zones, p.Zones)
	return &ret
}

// ZoneUsage implements the Resource interface
func (p *Pod) ZoneUsage() ZoneUsageMap {
	return p.Zones
}

// StringID implements the Resource interface
func (p *Pod) StringID() string {
	return p.ID
}

type (
	Processes       = map[string]*Process
	Containers      = map[string]*Container
	VirtualMachines = map[string]*VirtualMachine
	Pods            = map[string]*Pod
)

// Snapshot encapsulates power monitoring data
type Snapshot struct {
	Timestamp time.Time // Timestamp of the snapshot
	Node      *Node     // Node power data

	Processes           Processes // Process power data, keyed by PID
	TerminatedProcesses Processes // Terminated processes with highest energy consumption

	Containers           Containers // Container power data, keyed by container ID
	TerminatedContainers Containers // Terminated containers with highest energy consumption

	VirtualMachines           VirtualMachines // VM power data, keyed by container ID
	TerminatedVirtualMachines VirtualMachines // Terminated VMs with highest energy consumption
	Pods                      Pods            // Pod power data, keyed by pod ID
	TerminatedPods            Pods            // Terminated pods with highest energy consumption
}

// NewSnapshot creates a new Snapshot instance
func NewSnapshot() *Snapshot {
	return &Snapshot{
		// Timestamp: time.Time{}, // Zero value to indicate unset
		Node: &Node{
			Zones: make(NodeZoneUsageMap),
		},
		Processes:                 make(Processes),
		TerminatedProcesses:       make(Processes),
		Containers:                make(Containers),
		TerminatedContainers:      make(Containers),
		VirtualMachines:           make(VirtualMachines),
		TerminatedVirtualMachines: make(VirtualMachines),
		Pods:                      make(Pods),
		TerminatedPods:            make(Pods),
	}
}

func (s *Snapshot) Clone() *Snapshot {
	clone := &Snapshot{
		Timestamp:                 s.Timestamp,
		Node:                      s.Node.Clone(),
		Processes:                 make(Processes, len(s.Processes)),
		TerminatedProcesses:       make(Processes, len(s.TerminatedProcesses)),
		Containers:                make(Containers, len(s.Containers)),
		TerminatedContainers:      make(Containers, len(s.TerminatedContainers)),
		VirtualMachines:           make(VirtualMachines, len(s.VirtualMachines)),
		TerminatedVirtualMachines: make(VirtualMachines, len(s.TerminatedVirtualMachines)),
		Pods:                      make(Pods, len(s.Pods)),
		TerminatedPods:            make(Pods, len(s.TerminatedPods)),
	}

	// Deep copy the processes map
	for pid, src := range s.Processes {
		clone.Processes[pid] = src.Clone()
	}

	// Deep copy terminated processes map
	for id, src := range s.TerminatedProcesses {
		clone.TerminatedProcesses[id] = src.Clone()
	}

	for id, src := range s.Containers {
		clone.Containers[id] = src.Clone()
	}

	// Deep copy terminated containers map
	for id, src := range s.TerminatedContainers {
		clone.TerminatedContainers[id] = src.Clone()
	}

	for id, src := range s.VirtualMachines {
		clone.VirtualMachines[id] = src.Clone()
	}

	// Deep copy terminated VMs map
	for id, src := range s.TerminatedVirtualMachines {
		clone.TerminatedVirtualMachines[id] = src.Clone()
	}

	for id, src := range s.Pods {
		clone.Pods[id] = src.Clone()
	}

	// Deep copy terminated pods map
	for id, src := range s.TerminatedPods {
		clone.TerminatedPods[id] = src.Clone()
	}

	return clone
}
