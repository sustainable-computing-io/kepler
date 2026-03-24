// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package monitor

import (
	"github.com/sustainable-computing-io/kepler/internal/resource"
)

// firstPodRead initializes pod power data for the first time
func (pm *PowerMonitor) firstPodRead(snapshot *Snapshot) error {
	running := pm.resources.Pods().Running
	pods := make(Pods, len(running))

	zones := snapshot.Node.Zones
	nodeCPUTimeDelta := pm.resources.Node().ProcessTotalCPUTimeDelta

	for id, p := range running {
		pod := newPod(p, zones)

		// Calculate initial energy based on CPU ratio * nodeActiveEnergy
		for zone, nodeZoneUsage := range zones {
			if nodeZoneUsage.ActivePower == 0 || nodeZoneUsage.activeEnergy == 0 || nodeCPUTimeDelta == 0 {
				continue
			}

			cpuTimeRatio := p.CPUTimeDelta / nodeCPUTimeDelta
			activeEnergy := Energy(cpuTimeRatio * float64(nodeZoneUsage.activeEnergy))

			pod.Zones[zone] = Usage{
				Power:       Power(0), // No power in first read - no delta time to calculate rate
				EnergyTotal: activeEnergy,
			}
		}

		pods[id] = pod
	}
	// Aggregate GPU power and energy from containers into pods
	for _, container := range snapshot.Containers {
		if container.PodID == "" {
			continue
		}
		if pod, ok := pods[container.PodID]; ok {
			pod.GPUPower += container.GPUPower
			pod.GPUEnergyTotal += container.GPUEnergyTotal
		}
	}

	snapshot.Pods = pods

	pm.logger.Debug("Initialized pod power tracking",
		"pods", len(pods))
	return nil
}

// calculatePodPower calculates pod power for each running pod and handles terminated pods
func (pm *PowerMonitor) calculatePodPower(prev, newSnapshot *Snapshot) error {
	// Clear terminated workloads if snapshot has been exported
	if pm.exported.Load() {
		pm.logger.Debug("Clearing terminated pods after export")
		pm.terminatedPodsTracker.Clear()
	}

	// Get the current pods
	pods := pm.resources.Pods()

	// Handle terminated pods
	//
	// When a pod terminates, its processes are gone from /proc and it no longer
	// consumes CPU. However, the resource layer preserves the pod's last non-zero
	// CPUTimeDelta (frozen at termination), so we cannot simply re-run the power
	// formula to get zero — it would compute a stale non-zero value.
	//
	// We explicitly zero the instantaneous power (watts) because:
	//   1. The pod has no running processes, so its real power consumption is 0W.
	//   2. The cumulative energy (joules) is preserved — it represents total
	//      energy consumed over the pod's lifetime and must not be lost.
	//   3. When Prometheus scrapes this zero value before the series goes stale,
	//      the staleness marker holds 0W instead of the last non-zero power,
	//      preventing phantom power readings in dashboards.
	//
	// GPU power is also zeroed for the same reason — the GPU workload has ended.
	pm.logger.Debug("Processing terminated pods", "terminated", len(pods.Terminated))
	for id := range pods.Terminated {
		prevPod, exists := prev.Pods[id]
		if !exists {
			continue
		}

		clone := prevPod.Clone()

		// Zero instantaneous power: the pod is no longer running, so its
		// real-time power draw is 0W. We preserve EnergyTotal (cumulative
		// joules) since that represents the pod's historical consumption.
		for zone, usage := range clone.Zones {
			clone.Zones[zone] = Usage{
				EnergyTotal: usage.EnergyTotal,
				Power:       Power(0),
			}
		}
		clone.GPUPower = 0

		pm.terminatedPodsTracker.Add(clone)
	}

	// Skip if no running pods
	if len(pods.Running) == 0 {
		pm.logger.Debug("No running pods found, skipping pod power calculation")
		return nil
	}

	node := pm.resources.Node()
	nodeCPUTimeDelta := node.ProcessTotalCPUTimeDelta

	pm.logger.Debug("Calculating pod power",
		"node-cputime", nodeCPUTimeDelta,
		"running", len(pods.Running),
	)

	// Initialize pod map
	podMap := make(map[string]*Pod, len(pods.Running))

	// For each pod, calculate power for each zone separately
	for id, p := range pods.Running {
		// Create pod power entry with node zones
		pod := newPod(p, newSnapshot.Node.Zones)

		// Calculate CPU time ratio for this pod

		// For each zone in the node, calculate pod's share
		for zone, nodeZoneUsage := range newSnapshot.Node.Zones {
			// Skip zones with zero power to avoid division by zero
			if nodeZoneUsage.Power == 0 || nodeZoneUsage.activeEnergy == 0 || nodeCPUTimeDelta == 0 {
				continue
			}

			cpuTimeRatio := p.CPUTimeDelta / nodeCPUTimeDelta
			// Calculate pod's share of this zone's power and energy
			activeEnergy := Energy(float64(nodeZoneUsage.activeEnergy) * cpuTimeRatio)
			absoluteEnergy := activeEnergy

			// If we have previous data for this pod and zone, add to absolute energy
			if prev, exists := prev.Pods[id]; exists {
				if prevUsage, hasZone := prev.Zones[zone]; hasZone {
					absoluteEnergy += prevUsage.EnergyTotal
				}
			}
			pod.Zones[zone] = Usage{
				EnergyTotal: absoluteEnergy,
				Power:       Power(cpuTimeRatio * float64(nodeZoneUsage.ActivePower)),
			}
		}

		podMap[id] = pod
	}

	// Aggregate GPU power and energy from containers into pods
	for _, container := range newSnapshot.Containers {
		if container.PodID == "" {
			continue
		}
		if pod, ok := podMap[container.PodID]; ok {
			pod.GPUPower += container.GPUPower
			pod.GPUEnergyTotal += container.GPUEnergyTotal
		}
	}

	// Update the snapshot
	newSnapshot.Pods = podMap

	// Populate terminated pods from tracker
	newSnapshot.TerminatedPods = pm.terminatedPodsTracker.Items()
	pm.logger.Debug("snapshot updated for pods",
		"running", len(newSnapshot.Pods),
		"terminated", len(newSnapshot.TerminatedPods),
	)

	return nil
}

// newPod creates a new Pod struct with initialized zones from resource.Pod
func newPod(pod *resource.Pod, zones NodeZoneUsageMap) *Pod {
	p := &Pod{
		ID:           pod.ID,
		Name:         pod.Name,
		Namespace:    pod.Namespace,
		CPUTotalTime: pod.CPUTotalTime,
		Zones:        make(ZoneUsageMap, len(zones)),
	}

	// Initialize each zone with zero values
	for zone := range zones {
		p.Zones[zone] = Usage{
			EnergyTotal: Energy(0),
			Power:       Power(0),
		}
	}

	return p
}
