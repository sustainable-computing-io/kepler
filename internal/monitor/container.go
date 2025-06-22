// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package monitor

import (
	"maps"

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
	// Get the current cntrs
	cntrs := pm.resources.Containers()

	// Copy existing terminated containers from previous snapshot if not exported
	if !pm.exported.Load() {
		// NOTE: no need to deep clone since already terminated containers won't be updated
		maps.Copy(newSnapshot.TerminatedContainers, prev.TerminatedContainers)
	}

	pm.logger.Debug("Processing terminated containers", "terminated", len(cntrs.Terminated))
	for id := range cntrs.Terminated {
		prevContainer, exists := prev.Containers[id]
		if !exists {
			continue
		}

		// Only include terminated containers that have consumed energy
		if prevContainer.Zones.HasZeroEnergy() {
			pm.logger.Debug("Filtering out terminated container with zero energy", "id", id)
			continue
		}
		pm.logger.Debug("Including terminated container with non-zero energy", "id", id)

		terminatedContainer := prevContainer.Clone()
		newSnapshot.TerminatedContainers[id] = terminatedContainer
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

	pm.logger.Debug("snapshot updated for containers",
		"running", len(newSnapshot.Containers),
		"terminated", len(newSnapshot.TerminatedContainers))

	return nil
}
