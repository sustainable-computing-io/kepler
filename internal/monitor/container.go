// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package monitor

import (
	"github.com/sustainable-computing-io/kepler/internal/resource"
)

// firstContainerRead initializes container power data for the first time
func (pm *PowerMonitor) firstContainerRead(snapshot *Snapshot) error {
	running := pm.resources.Containers().Running
	containers := make(Containers, len(running))

	zones := snapshot.Node.Zones
	nodeCPUTimeDelta := pm.resources.Node().ProcessTotalCPUTimeDelta

	for id, cntr := range running {
		container := newContainer(cntr, zones)

		// Calculate initial energy based on CPU ratio * nodeActiveEnergy
		for zone, nodeZoneUsage := range zones {
			if nodeZoneUsage.ActivePower == 0 || nodeZoneUsage.activeEnergy == 0 || nodeCPUTimeDelta == 0 {
				continue
			}

			cpuTimeRatio := cntr.CPUTimeDelta / nodeCPUTimeDelta
			activeEnergy := Energy(cpuTimeRatio * float64(nodeZoneUsage.activeEnergy))

			container.Zones[zone] = Usage{
				Power:       Power(0), // No power in first read - no delta time to calculate rate
				EnergyTotal: activeEnergy,
			}
		}

		containers[id] = container
	}
	snapshot.Containers = containers

	pm.logger.Debug("Initialized container power tracking",
		"containers", len(containers))
	return nil
}

func newContainer(cntr *resource.Container, zones NodeZoneUsageMap) *Container {
	container := &Container{
		ID:           cntr.ID,
		Name:         cntr.Name,
		Runtime:      cntr.Runtime,
		CPUTotalTime: cntr.CPUTotalTime,
		Zones:        make(ZoneUsageMap, len(zones)),
	}

	// Initialize each zone with zero values
	for zone := range zones {
		container.Zones[zone] = Usage{
			EnergyTotal: Energy(0),
			Power:       Power(0),
		}
	}

	// Add the pod ID if available
	if cntr.Pod != nil {
		container.PodID = cntr.Pod.ID
	}

	return container
}

// calculateContainerPower calculates container power for each running container
func (pm *PowerMonitor) calculateContainerPower(prev, newSnapshot *Snapshot) error {
	// Clear terminated workloads if snapshot has been exported
	if pm.exported.Load() {
		pm.logger.Debug("Clearing terminated containers after export")
		pm.terminatedContainersTracker.Clear()
	}

	// Get the current cntrs
	cntrs := pm.resources.Containers()

	pm.logger.Debug("Processing terminated containers", "terminated", len(cntrs.Terminated))
	for id := range cntrs.Terminated {
		prevContainer, exists := prev.Containers[id]
		if !exists {
			continue
		}

		// Add to internal tracker (which will handle priority-based retention)
		// NOTE: Each terminated container is only added once since a container cannot be terminated twice
		pm.terminatedContainersTracker.Add(prevContainer.Clone())
	}

	// process running containers
	zones := newSnapshot.Node.Zones
	node := pm.resources.Node()
	nodeCPUTimeDelta := node.ProcessTotalCPUTimeDelta

	pm.logger.Debug("Calculating container power",
		"node.cpu.time", nodeCPUTimeDelta,
		"running", len(cntrs.Running),
	)

	containerMap := make(map[string]*Container, len(cntrs.Running))

	// For each container, calculate power for each zone separately
	for id, c := range cntrs.Running {
		container := newContainer(c, zones)

		// Calculate CPU time ratio for this container

		// For each zone in the node, calculate container's share
		for zone, nodeZoneUsage := range zones {
			// Skip zones with zero power to avoid division by zero
			if nodeZoneUsage.ActivePower == 0 || nodeZoneUsage.activeEnergy == 0 || nodeCPUTimeDelta == 0 {
				continue
			}

			cpuTimeRatio := c.CPUTimeDelta / nodeCPUTimeDelta

			// Calculate energy delta for this interval
			activeEnergy := Energy(cpuTimeRatio * float64(nodeZoneUsage.activeEnergy))

			// Calculate absolute energy based on previous data
			// New container, starts with delta
			absoluteEnergy := activeEnergy
			if prev, exists := prev.Containers[id]; exists {
				if prevUsage, hasZone := prev.Zones[zone]; hasZone {
					absoluteEnergy += prevUsage.EnergyTotal
				}
			}

			// Calculate container's share of this zone's power and energy
			container.Zones[zone] = Usage{
				Power:       Power(cpuTimeRatio * nodeZoneUsage.ActivePower.MicroWatts()),
				EnergyTotal: absoluteEnergy,
			}
		}

		containerMap[id] = container
	}

	// Update the snapshot
	newSnapshot.Containers = containerMap

	// Populate terminated containers from tracker
	newSnapshot.TerminatedContainers = pm.terminatedContainersTracker.Items()
	pm.logger.Debug("snapshot updated for containers",
		"running", len(newSnapshot.Containers),
		"terminated", len(newSnapshot.TerminatedContainers),
	)

	return nil
}
