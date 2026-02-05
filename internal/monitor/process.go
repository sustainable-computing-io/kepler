// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package monitor

import (
	"fmt"

	"github.com/sustainable-computing-io/kepler/internal/resource"
)

// firstProcessRead initializes process power data for the first time
func (pm *PowerMonitor) firstProcessRead(snapshot *Snapshot) error {
	// Collect GPU device stats on first read from all GPU meters
	if len(pm.gpuMeters) > 0 {
		var gpuStats []GPUDeviceStats
		for _, meter := range pm.gpuMeters {
			devices := meter.Devices()
			for _, dev := range devices {
				stats, err := meter.GetDevicePowerStats(dev.Index)
				if err != nil {
					pm.logger.Debug("Failed to get GPU device stats", "device", dev.Index, "error", err)
					continue
				}
				energy, energyErr := meter.GetTotalEnergy(dev.Index)
				if energyErr != nil {
					pm.logger.Debug("Failed to get GPU energy", "device", dev.Index, "error", energyErr)
				}
				gpuStats = append(gpuStats, GPUDeviceStats{
					DeviceIndex: dev.Index,
					UUID:        dev.UUID,
					Name:        dev.Name,
					Vendor:      string(dev.Vendor),
					TotalPower:  stats.TotalPower,
					IdlePower:   stats.IdlePower,
					ActivePower: stats.ActivePower,
					EnergyTotal: energy,
				})
			}
		}
		snapshot.GPUStats = gpuStats
		pm.logger.Info("GPU stats collected on first read", "devices", len(gpuStats))
		for _, s := range gpuStats {
			pm.logger.Debug("GPU device stats", "device", s.DeviceIndex, "uuid", s.UUID, "total", s.TotalPower, "idle", s.IdlePower, "active", s.ActivePower)
		}
	}

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

	// Get GPU power attribution from all GPU meters
	gpuPowerByPID := make(map[uint32]float64)
	if len(pm.gpuMeters) > 0 {
		var gpuStats []GPUDeviceStats
		for _, meter := range pm.gpuMeters {
			// Get process power from this meter
			power, err := meter.GetProcessPower()
			if err != nil {
				pm.logger.Warn("Failed to get GPU process power", "vendor", meter.Vendor(), "error", err)
				continue
			}
			// Collect power from this meter. In practice, nodes have homogeneous GPUs
			// (single vendor), and a process uses only one GPU type (CUDA or ROCm),
			// so there's no PID overlap between meters.
			for pid, watts := range power {
				gpuPowerByPID[pid] = watts
			}

			// Collect GPU device stats for debugging/monitoring
			devices := meter.Devices()
			for _, dev := range devices {
				stats, err := meter.GetDevicePowerStats(dev.Index)
				if err != nil {
					pm.logger.Debug("Failed to get GPU device stats", "device", dev.Index, "error", err)
					continue
				}
				energy, energyErr := meter.GetTotalEnergy(dev.Index)
				if energyErr != nil {
					pm.logger.Debug("Failed to get GPU energy", "device", dev.Index, "error", energyErr)
				}
				gpuStats = append(gpuStats, GPUDeviceStats{
					DeviceIndex: dev.Index,
					UUID:        dev.UUID,
					Name:        dev.Name,
					Vendor:      string(dev.Vendor),
					TotalPower:  stats.TotalPower,
					IdlePower:   stats.IdlePower,
					ActivePower: stats.ActivePower,
					EnergyTotal: energy,
				})
			}
		}
		newSnapshot.GPUStats = gpuStats
		pm.logger.Debug("GPU process power", "gpu_processes", len(gpuPowerByPID))
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

		// Add GPU power attribution if available
		if gpuPower, hasGPU := gpuPowerByPID[uint32(proc.PID)]; hasGPU {
			process.GPUPower = gpuPower
		}

		// Accumulate GPU energy: energy = power × time
		if prevProc, exists := prev.Processes[pid]; exists {
			process.GPUEnergyTotal = prevProc.GPUEnergyTotal
			if process.GPUPower > 0 {
				timeDelta := newSnapshot.Node.Timestamp.Sub(prev.Node.Timestamp).Seconds()
				if timeDelta > 0 {
					process.GPUEnergyTotal += Energy(process.GPUPower * timeDelta * float64(Joule))
				}
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
