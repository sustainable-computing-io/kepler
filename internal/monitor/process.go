// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package monitor

import (
	"fmt"

	"github.com/sustainable-computing-io/kepler/internal/resource"
)

// firstProcessRead initializes process power data for the first time
func (pm *PowerMonitor) firstProcessRead(snapshot *Snapshot) error {
	running := pm.resources.Processes().Running
	processes := make(Processes, len(running))

	zones := snapshot.Node.Zones
	nodeCPUTimeDelta := pm.resources.Node().ProcessTotalCPUTimeDelta

	for _, proc := range running {
		process := newProcess(proc, zones)

		// Calculate initial energy based on CPU ratio * nodeActiveEnergy
		for zone, nodeZoneUsage := range zones {
			if nodeZoneUsage.ActivePower == 0 || nodeZoneUsage.activeEnergy == 0 || nodeCPUTimeDelta == 0 {
				continue
			}

			cpuTimeRatio := proc.CPUTimeDelta / nodeCPUTimeDelta
			activeEnergy := Energy(cpuTimeRatio * float64(nodeZoneUsage.activeEnergy))

			process.Zones[zone] = Usage{
				Power:       Power(0), // No power in first read - no delta time to calculate rate
				EnergyTotal: activeEnergy,
			}
		}

		processes[process.StringID()] = process
	}
	snapshot.Processes = processes

	pm.logger.Debug("Initialized process power tracking",
		"processes", len(processes),
	)
	return nil
}

func newProcess(proc *resource.Process, zones NodeZoneUsageMap) *Process {
	process := &Process{
		PID:          proc.PID,
		Comm:         proc.Comm,
		Exe:          proc.Exe,
		Type:         proc.Type,
		CPUTotalTime: proc.CPUTotalTime,
		Zones:        make(ZoneUsageMap, len(zones)),
	}

	// Initialize each zone with zero values
	for zone := range zones {
		process.Zones[zone] = Usage{
			EnergyTotal: Energy(0),
			Power:       Power(0),
		}
	}

	// Add the container ID if available
	if proc.Container != nil {
		process.ContainerID = proc.Container.ID
	}

	// Add the VM ID if available
	if proc.VirtualMachine != nil {
		process.VirtualMachineID = proc.VirtualMachine.ID
	}
	return process
}

// calculateProcessPower calculates process power for each running process
func (pm *PowerMonitor) calculateProcessPower(prev, newSnapshot *Snapshot) error {
	// Clear terminated workloads if snapshot has been exported
	if pm.exported.Load() {
		pm.logger.Debug("Clearing terminated processes after export")
		pm.terminatedProcessesTracker.Clear()
	}

	procs := pm.resources.Processes()

	pm.logger.Debug("Processing terminated processes", "terminated", len(procs.Terminated))
	for pid := range procs.Terminated {
		pidStr := fmt.Sprintf("%d", pid)
		prevProcess, exists := prev.Processes[pidStr]
		if !exists {
			continue
		}

		// Add to internal tracker (which will handle priority-based retention)
		// NOTE: Each terminated process is only added once since a process cannot be terminated twice
		pm.terminatedProcessesTracker.Add(prevProcess.Clone())
	}

	running := procs.Running

	zones := newSnapshot.Node.Zones
	nodeCPUTimeDelta := pm.resources.Node().ProcessTotalCPUTimeDelta
	pm.logger.Debug("Calculating Process power",
		"node.cpu.time", nodeCPUTimeDelta,
		"running", len(running),
	)

	// Initialize process map
	processMap := make(Processes, len(running))

	if len(running) == 0 {
		// this is odd!
		pm.logger.Warn("No running processes found, skipping running process power calculation")
	}

	for _, proc := range running {
		process := newProcess(proc, zones)
		pid := process.StringID() // to string

		// For each zone in the node, calculate process's share
		for zone, nodeZoneUsage := range zones {
			if nodeZoneUsage.ActivePower == 0 || nodeZoneUsage.activeEnergy == 0 || nodeCPUTimeDelta == 0 {
				continue
			}

			cpuTimeRatio := proc.CPUTimeDelta / nodeCPUTimeDelta
			// Calculate energy  for this interval
			activeEnergy := Energy(cpuTimeRatio * float64(nodeZoneUsage.activeEnergy))

			// Calculate absolute energy based on previous data
			absoluteEnergy := activeEnergy
			if prev, exists := prev.Processes[pid]; exists {
				if prevUsage, hasZone := prev.Zones[zone]; hasZone {
					absoluteEnergy += prevUsage.EnergyTotal
				}
			}

			// Calculate process's share of this zone's power and energy
			process.Zones[zone] = Usage{
				Power:       Power(cpuTimeRatio * nodeZoneUsage.ActivePower.MicroWatts()),
				EnergyTotal: absoluteEnergy,
			}
		}

		processMap[process.StringID()] = process
	}

	// Update the snapshot of running processes
	newSnapshot.Processes = processMap

	// Populate terminated processes from tracker
	newSnapshot.TerminatedProcesses = pm.terminatedProcessesTracker.Items()
	pm.logger.Debug("snapshot updated for process",
		"running", len(newSnapshot.Processes),
		"terminated", len(newSnapshot.TerminatedProcesses),
	)

	return nil
}
