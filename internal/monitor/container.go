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
			container.Zones[zone] = &Usage{
				Absolute: Energy(0),
				Delta:    Energy(0),
				Power:    Power(0),
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

	pm.logger.Debug("Calculating container power",
		"node-cputime", containers.NodeCPUTimeDelta,
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
			if nodeZoneUsage.Power == 0 || nodeZoneUsage.Delta == 0 || containers.NodeCPUTimeDelta == 0 {
				container.Zones[zone] = &Usage{
					Power:    Power(0),
					Delta:    Energy(0),
					Absolute: Energy(0),
				}
				continue
			}

			cpuTimeRatio := c.CPUTimeDelta / containers.NodeCPUTimeDelta
			// Calculate container's share of this zone's power and energy
			container.Zones[zone] = &Usage{
				Power: Power(cpuTimeRatio * nodeZoneUsage.Power.MicroWatts()),
				Delta: Energy(cpuTimeRatio * float64(nodeZoneUsage.Delta)),
			}

			// If we have previous data for this container and zone, add to absolute energy
			if prev, exists := prev.Containers[id]; exists {
				if prevUsage, hasZone := prev.Zones[zone]; hasZone {
					container.Zones[zone].Absolute = prevUsage.Absolute + container.Zones[zone].Delta
				} else {
					// TODO: unlikely; so add telemetry for this
					container.Zones[zone].Absolute = container.Zones[zone].Delta
				}
			} else {
				// New container, starts with delta
				container.Zones[zone].Absolute = container.Zones[zone].Delta
			}
		}

		containerMap[id] = container
	}

	// Update the snapshot
	newSnapshot.Containers = containerMap

	return nil
}
