// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package monitor

import (
	"github.com/sustainable-computing-io/kepler/internal/device/network"
)

// firstNICRead captures the initial NIC energy reading.
func (pm *PowerMonitor) firstNICRead(snapshot *Snapshot) {
	if pm.nicMeter == nil {
		return
	}
	zone, err := pm.nicMeter.Zone()
	if err != nil {
		pm.logger.Warn("Could not read NIC energy zone", "error", err)
		return
	}
	energy, err := zone.Energy()
	if err != nil {
		pm.logger.Warn("Could not read NIC energy", "error", err)
		return
	}
	snapshot.NICStats = &NICStats{
		EnergyTotal: energy,
		Path:        zone.Path(),
	}
	pm.logger.Info("First NIC energy read", "energy", energy, "path", zone.Path())

	// Capture initial flow snapshot for delta calculation
	pm.computePodNICStats(snapshot)
}

// calculateNICPower computes NIC power from the energy delta between snapshots.
func (pm *PowerMonitor) calculateNICPower(prev, newSnapshot *Snapshot) {
	if pm.nicMeter == nil {
		return
	}
	zone, err := pm.nicMeter.Zone()
	if err != nil {
		pm.logger.Warn("Could not read NIC energy zone", "error", err)
		return
	}
	energy, err := zone.Energy()
	if err != nil {
		pm.logger.Warn("Could not read NIC energy", "error", err)
		return
	}

	stats := &NICStats{
		EnergyTotal: energy,
		Path:        zone.Path(),
	}

	if prev.NICStats != nil {
		timeDiff := pm.clock.Now().Sub(prev.Timestamp).Seconds()
		if timeDiff > 0 {
			delta := calculateEnergyDelta(energy, prev.NICStats.EnergyTotal, zone.MaxEnergy())
			stats.Power = Power(float64(delta) / timeDiff)
		}
	}

	newSnapshot.NICStats = stats

	// Compute per-pod NIC attribution
	pm.computePodNICStats(newSnapshot)
}

// computePodNICStats reads NIC flows and conntrack, then attributes node NIC
// watts to individual pods based on their byte share.
func (pm *PowerMonitor) computePodNICStats(snapshot *Snapshot) {
	if pm.conntrackReader == nil {
		return
	}

	// Read NIC flows from eBPF output
	flowsData, err := network.ReadNICFlows("")
	if err != nil {
		pm.logger.Debug("Could not read NIC flows", "error", err)
		return
	}

	// Refresh conntrack NAT table
	if err := pm.conntrackReader.Refresh(); err != nil {
		pm.logger.Debug("Could not refresh conntrack", "error", err)
		return
	}

	// Attribute flows to pods: bridge IP → conntrack → pod IP.
	// The eBPF program only emits flows that traversed the physical NIC,
	// so every flow already represents real NIC energy cost.
	podUsage := network.AttributeFlowsToPods(flowsData.Flows, pm.conntrackReader.NATEntries())
	if len(podUsage) == 0 {
		return
	}

	// Calculate total attributed bytes for ratio
	var totalBytes uint64
	for _, u := range podUsage {
		totalBytes += u.TxBytes + u.RxBytes
	}
	if totalBytes == 0 {
		return
	}

	// Get node NIC watts for attribution
	var nodeNICWatts float64
	if snapshot.NICStats != nil {
		nodeNICWatts = snapshot.NICStats.Power.Watts()
	}

	// Attribute: pod_watts = node_nic_watts × (pod_bytes / total_bytes)
	podNICStats := make(map[string]*PodNICStats, len(podUsage))
	for podIP, u := range podUsage {
		podBytes := u.TxBytes + u.RxBytes
		ratio := float64(podBytes) / float64(totalBytes)

		podNICStats[podIP] = &PodNICStats{
			PodIP:   podIP,
			TxBytes: u.TxBytes,
			RxBytes: u.RxBytes,
			Watts:   nodeNICWatts * ratio,
		}
	}

	snapshot.PodNICStats = podNICStats

	pm.logger.Debug("Per-pod NIC attribution",
		"pods", len(podNICStats),
		"total_bytes", totalBytes,
		"node_nic_watts", nodeNICWatts,
	)
}
