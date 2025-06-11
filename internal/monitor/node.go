// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package monitor

import (
	"errors"
)

func (pm *PowerMonitor) calculateNodePower(prevNode, newNode *Node) error {
	// Get previous measurements for calculating watts
	prevReadTime := prevNode.Timestamp
	prevZones := prevNode.Zones

	now := pm.clock.Now()
	newNode.Timestamp = now

	// get zones first, before locking for read
	zones, err := pm.cpu.Zones()
	if err != nil {
		return err
	}

	nodeCPUTimeDelta := pm.resources.Node().ProcessTotalCPUTimeDelta
	nodeCPUUsageRatio := pm.resources.Node().CPUUsageRatio
	newNode.UsageRatio = nodeCPUUsageRatio

	pm.logger.Debug("Calculating Node power",
		"node.process-cpu.time", nodeCPUTimeDelta,
		"node.cpu.usage-ratio", nodeCPUUsageRatio,
	)

	// NOTE: energy is in MicroJoules and Power is in MicroWatts
	timeDiff := now.Sub(prevReadTime).Seconds()
	// Get the current energy

	var retErr error
	for _, zone := range zones {
		absEnergy, err := zone.Energy()
		if err != nil {
			retErr = errors.Join(err)
			pm.logger.Warn("Could not read energy for zone", "zone", zone.Name(), "index", zone.Index(), "error", err)
			continue
		}

		// Calculate watts and joules diff if we have previous data for the zone
		var activeEnergy, activeEnergyTotal, idleEnergyTotal Energy
		var power, activePower, idlePower Power

		if prevZone, ok := prevZones[zone]; ok {
			// Absolute is a running total, so to find the current energy usage, calculate the delta
			// delta = current - previous
			// active = delta * cpuUsage
			// idle = delta - active

			deltaEnergy := calculateEnergyDelta(absEnergy, prevZone.EnergyTotal, zone.MaxEnergy())

			activeEnergy = Energy(float64(deltaEnergy) * nodeCPUUsageRatio)
			idleEnergy := deltaEnergy - activeEnergy

			activeEnergyTotal = prevZone.ActiveEnergyTotal + activeEnergy
			idleEnergyTotal = prevZone.IdleEnergyTotal + idleEnergy

			powerF64 := float64(deltaEnergy) / float64(timeDiff)
			power = Power(powerF64)
			activePower = Power(powerF64 * nodeCPUUsageRatio)
			idlePower = power - activePower
		}

		newNode.Zones[zone] = NodeUsage{
			EnergyTotal: absEnergy,

			activeEnergy:      activeEnergy,
			ActiveEnergyTotal: activeEnergyTotal,
			IdleEnergyTotal:   idleEnergyTotal,

			Power:       power,
			ActivePower: activePower,
			IdlePower:   idlePower,
		}
	}

	return retErr
}

// Calculate joules difference handling wraparound
func calculateEnergyDelta(current, previous, maxJoules Energy) Energy {
	if current >= previous {
		return current - previous
	}

	// counter wraparound
	if maxJoules > 0 {
		return (maxJoules - previous) + current
	}

	return 0 // Unable to calculate delta
}

// firstNodeRead reads the energy for the first time
func (pm *PowerMonitor) firstNodeRead(node *Node) error {
	node.Timestamp = pm.clock.Now()

	zones, err := pm.cpu.Zones()
	if err != nil {
		return err
	}

	nodeCPUUsageRatio := pm.resources.Node().CPUUsageRatio
	var retErr error
	for _, zone := range zones {
		energy, err := zone.Energy()
		if err != nil {
			retErr = errors.Join(err)
			pm.logger.Warn("Could not read energy for zone", "zone", zone.Name(), "index", zone.Index(), "error", err)
			continue
		}
		activeEnergy := Energy(float64(energy) * nodeCPUUsageRatio)
		idleEnergy := energy - activeEnergy

		node.Zones[zone] = NodeUsage{
			EnergyTotal:       energy,
			ActiveEnergyTotal: activeEnergy,
			IdleEnergyTotal:   idleEnergy,
			activeEnergy:      activeEnergy,
			// Power can't be calculated in the first read since we need Δt
		}
	}

	return retErr
}
