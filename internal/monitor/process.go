// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package monitor

import (
	"github.com/sustainable-computing-io/kepler/internal/resource"
)

// firstProcessRead initializes process power data for the first time
func (pm *PowerMonitor) firstProcessRead(snapshot *Snapshot) error {
	running := pm.resources.Processes().Running
	processes := make(Processes, len(running))
	for pid, proc := range running {
		processes[pid] = newProcess(proc, snapshot.Node.Zones)
	}
	snapshot.Processes = processes

	pm.logger.Debug("Initialized process power tracking",
		"processes", len(processes),
	)
	return nil
}

func newProcess(proc *resource.Process, zones ZoneUsageMap) *Process {
	process := &Process{
		PID:          proc.PID,
		Comm:         proc.Comm,
		Exe:          proc.Exe,
		CPUTotalTime: proc.CPUTotalTime,
		Zones:        make(ZoneUsageMap, len(zones)),
	}

	// Initialize each zone with zero values
	for zone := range zones {
		process.Zones[zone] = &Usage{
			Absolute: Energy(0),
			Delta:    Energy(0),
			Power:    Power(0),
		}
	}

	// Add the container ID if available
	if proc.Container != nil {
		process.ContainerID = proc.Container.ID
	}
	return process
}

// calculateProcessPower calculates process power for each running process
func (pm *PowerMonitor) calculateProcessPower(prev, newSnapshot *Snapshot) error {
	procs := pm.resources.Processes()
	running := procs.Running
	if len(running) == 0 {
		pm.logger.Debug("No running processes found, skipping process power calculation")
		return nil
	}

	zones := newSnapshot.Node.Zones
	nodeCPUTimeDelta := procs.NodeCPUTimeDelta

	// Initialize process map
	processMap := make(Processes, len(running))

	for pid, proc := range running {
		process := newProcess(proc, zones)

		// For each zone in the node, calculate process's share
		for zone, usage := range zones {
			if usage.Power == 0 || usage.Delta == 0 || nodeCPUTimeDelta == 0 {
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
			cpuRatio := proc.CPUTimeDelta / nodeCPUTimeDelta

			// Calculate process's share of this zone's power and energy

			energyDelta := Energy(cpuRatio * float64(usage.Delta))
			process.Zones[zone] = &Usage{
				Power: Power(cpuRatio * usage.Power.MicroWatts()),
				Delta: energyDelta,
			}

			// If we have previous data for this process and zone, add to absolute energy
			if prev, exists := prev.Processes[pid]; exists {
				if prevUsage, hasZone := prev.Zones[zone]; hasZone {
					process.Zones[zone].Absolute = prevUsage.Absolute + energyDelta
				} else {
					// TODO: unlikely; so add telemetry for this
					process.Zones[zone].Absolute = process.Zones[zone].Delta
				}
			} else {
				// New process, starts with delta
				process.Zones[zone].Absolute = process.Zones[zone].Delta
			}
		}

		processMap[pid] = process
	}

	// Update the snapshot
	newSnapshot.Processes = processMap

	return nil
}
