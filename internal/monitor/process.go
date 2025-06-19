// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package monitor

import (
	"maps"

	"github.com/sustainable-computing-io/kepler/internal/resource"
)

// firstProcessRead initializes process power data for the first time
func (pm *PowerMonitor) firstProcessRead(snapshot *Snapshot) error {
	running := pm.resources.Processes().Running
	processes := make(Processes, len(running))

	zones := snapshot.Node.Zones
	nodeCPUTimeDelta := pm.resources.Node().ProcessTotalCPUTimeDelta

	for pid, proc := range running {
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

		processes[pid] = process
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
	procs := pm.resources.Processes()
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

	for pid, proc := range running {
		process := newProcess(proc, zones)

		// For each zone in the node, calculate process's share
		for zone, nodeZoneUsage := range zones {
			if nodeZoneUsage.ActivePower == 0 || nodeZoneUsage.activeEnergy == 0 || nodeCPUTimeDelta == 0 {
				continue
			}

			// Calculate CPU time ratio for process by calculating the cpuUsage as
			// the delta between 2 refreshes over the total delta for all processes on the node
			//
			// T -  Proc-1  Val   Delta
			// 1 ->  P1_t1   100
			// 2 ->  P1_t2   150   P1_t2 - P1_T1 = 50
			//
			//
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

		processMap[pid] = process
	}

	// Update the snapshot of running processes
	newSnapshot.Processes = processMap

	// Copy existing terminated processes from previous snapshot if not exported
	if !pm.exported.Load() {
		// NOTE: no need to deep clone since already terminated processes won't be updated
		maps.Copy(newSnapshot.TerminatedProcesses, prev.TerminatedProcesses)
	}

	pm.logger.Debug("Processing terminated processes", "terminated", len(procs.Terminated))
	for pid := range procs.Terminated {
		prevProcess, exists := prev.Processes[pid]
		if !exists {
			continue
		}

		// Only include terminated processes that have consumed energy
		if prevProcess.Zones.HasZeroEnergy() {
			pm.logger.Debug("Filtering out terminated process with zero energy", "pid", pid)
			continue
		}
		pm.logger.Debug("Including terminated process with non-zero energy", "pid", pid)

		terminatedProcess := prevProcess.Clone()
		newSnapshot.TerminatedProcesses[pid] = terminatedProcess
	}

	pm.logger.Debug("snapshot updated for process",
		"running", len(newSnapshot.Processes),
		"terminated", len(newSnapshot.TerminatedProcesses))

	return nil
}
