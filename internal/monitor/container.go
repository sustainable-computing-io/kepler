// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package monitor

// firstContainerRead initializes container power data for the first time
func (pm *PowerMonitor) firstContainerRead(snapshot *Snapshot) error {
	// Get the available zones to initialize each container with the same zones
	zones, err := pm.cpu.Zones()
	if err != nil {
		return err
	}

	// Get the current running
	running := pm.resources.Containers().Running
	containers := make(Containers, len(running))

	// Add each container with zero energy/power for each zone
	for id, ctnr := range running {
		// Create new container power entry
		container := &Container{
			ID:           id,
			Name:         ctnr.Name,
			Runtime:      ctnr.Runtime,
			CPUTotalTime: ctnr.CPUTotalTime,
			Zones:        make(ZoneUsageMap, len(zones)),
		}

		// Initialize each zone with zero values
		for _, zone := range zones {
			container.Zones[zone] = Usage{
				EnergyTotal: Energy(0),
				Power:       Power(0),
			}
		}

		containers[id] = container
	}

	// Store in snapshot
	snapshot.Containers = containers

	pm.logger.Debug("Initialized container power tracking",
		"containers", len(containers),
		"zones_per_container", len(zones))
	return nil
}

// calculateContainerPower calculates container power for each running container
func (pm *PowerMonitor) calculateContainerPower(prev, newSnapshot *Snapshot) error {
	// Get the current containers
	containers := pm.resources.Containers()

	// Skip if no containers
	if len(containers.Running) == 0 {
		pm.logger.Debug("No running containers found, skipping container power calculation")
		return nil
	}

	node := pm.resources.Node()
	nodeCPUTimeDelta := node.ProcessTotalCPUTimeDelta

	pm.logger.Debug("Calculating container power",
		"node.cpu.time", nodeCPUTimeDelta,
		"running", len(containers.Running),
	)

	// Initialize container map
	containerMap := make(map[string]*Container, len(containers.Running))

	// For each container, calculate power for each zone separately
	for id, c := range containers.Running {
		// Create container power entry with empty zones map
		container := &Container{
			ID:           id,
			Name:         c.Name,
			Runtime:      c.Runtime,
			CPUTotalTime: c.CPUTotalTime,
			Zones:        make(ZoneUsageMap),
		}

		// Calculate CPU time ratio for this container

		// For each zone in the node, calculate container's share
		for zone, nodeZoneUsage := range newSnapshot.Node.Zones {
			// Skip zones with zero power to avoid division by zero
			if nodeZoneUsage.ActivePower == 0 || nodeZoneUsage.activeEnergy == 0 || nodeCPUTimeDelta == 0 {
				container.Zones[zone] = Usage{
					Power:       Power(0),
					EnergyTotal: Energy(0),
				}
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
	pm.logger.Debug("snapshot updated for containers", "containers", len(newSnapshot.Containers))

	return nil
}
