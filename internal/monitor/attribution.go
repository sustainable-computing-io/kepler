// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package monitor

// attributeZone returns a workload's share of a node zone's active power and
// energy, in proportion to its CPU time, with the energy added to prevEnergy.
func attributeZone(nodeZoneUsage NodeUsage, prevEnergy Energy, cpuTimeDelta, nodeCPUTimeDelta float64) Usage {
	// Nothing to attribute this interval, but keep the running total so the
	// exported counter does not reset.
	if nodeZoneUsage.ActivePower == 0 || nodeZoneUsage.activeEnergy == 0 || nodeCPUTimeDelta == 0 {
		return Usage{EnergyTotal: prevEnergy}
	}

	cpuTimeRatio := cpuTimeDelta / nodeCPUTimeDelta
	return Usage{
		Power:       Power(cpuTimeRatio * nodeZoneUsage.ActivePower.MicroWatts()),
		EnergyTotal: prevEnergy + Energy(cpuTimeRatio*float64(nodeZoneUsage.activeEnergy)),
	}
}
