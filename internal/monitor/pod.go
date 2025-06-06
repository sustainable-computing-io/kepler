// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package monitor

// firstPodRead initializes pod power data for the first time
func (pm *PowerMonitor) firstPodRead(snapshot *Snapshot) error {
	// Get the available zones to initialize each pod with the same zones
	zones, err := pm.cpu.Zones()
	if err != nil {
		return err
	}

	// Get the current running
	running := pm.resources.Pods().Running
	pods := make(Pods, len(running))

	// Add each pod with zero energy/power for each zone
	for id, pod := range running {
		// Create new pod power entry
		pod := &Pod{
			ID:           id,
			Name:         pod.Name,
			Namespace:    pod.Namespace,
			CPUTotalTime: pod.CPUTotalTime,
			Zones:        make(ZoneUsageMap, len(zones)),
		}

		// Initialize each zone with zero values
		for _, zone := range zones {
			pod.Zones[zone] = &Usage{
				Absolute: Energy(0),
				Delta:    Energy(0),
				Power:    Power(0),
			}
		}

		pods[id] = pod
	}

	// Store in snapshot
	snapshot.Pods = pods

	pm.logger.Debug("Initialized pod power tracking",
		"pods", len(pods),
		"zones_per_pod", len(zones))
	return nil
}

// calculatePodPower calculates pod power for each running pod
func (pm *PowerMonitor) calculatePodPower(prev, newSnapshot *Snapshot) error {
	// Get the current pods
	pods := pm.resources.Pods()

	// Skip if no pods
	if len(pods.Running) == 0 {
		pm.logger.Debug("No running pods found, skipping pod power calculation")
		return nil
	}

	pm.logger.Debug("Calculating pod power",
		"node-cputime", pods.NodeCPUTimeDelta,
		"running", len(pods.Running),
	)

	// Initialize pod map
	podMap := make(map[string]*Pod, len(pods.Running))

	// For each pod, calculate power for each zone separately
	for id, p := range pods.Running {
		// Create pod power entry with empty zones map
		pod := &Pod{
			ID:           id,
			Name:         p.Name,
			Namespace:    p.Namespace,
			CPUTotalTime: p.CPUTotalTime,
			Zones:        make(ZoneUsageMap),
		}

		// Calculate CPU time ratio for this pod

		// For each zone in the node, calculate pod's share
		for zone, nodeZoneUsage := range newSnapshot.Node.Zones {
			// Skip zones with zero power to avoid division by zero
			if nodeZoneUsage.Power == 0 || nodeZoneUsage.Delta == 0 || pods.NodeCPUTimeDelta == 0 {
				pod.Zones[zone] = &Usage{
					Power:    Power(0),
					Delta:    Energy(0),
					Absolute: Energy(0),
				}
				continue
			}

			cpuTimeRatio := p.CPUTimeDelta / pods.NodeCPUTimeDelta
			// Calculate pod's share of this zone's power and energy
			pod.Zones[zone] = &Usage{
				Power: Power(cpuTimeRatio * nodeZoneUsage.Power.MicroWatts()),
				Delta: Energy(cpuTimeRatio * float64(nodeZoneUsage.Delta)),
			}

			// If we have previous data for this pod and zone, add to absolute energy
			if prev, exists := prev.Pods[id]; exists {
				if prevUsage, hasZone := prev.Zones[zone]; hasZone {
					pod.Zones[zone].Absolute = prevUsage.Absolute + pod.Zones[zone].Delta
				} else {
					// TODO: unlikely; so add telemetry for this
					pod.Zones[zone].Absolute = pod.Zones[zone].Delta
				}
			} else {
				// New pod, starts with delta
				pod.Zones[zone].Absolute = pod.Zones[zone].Delta
			}
		}

		podMap[id] = pod
	}

	// Update the snapshot
	newSnapshot.Pods = podMap
	pm.logger.Debug("snapshot updated for pods", "pods", len(newSnapshot.Pods))

	return nil
}
