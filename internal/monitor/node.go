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
		var deltaEnergy Energy
		var power Power
		var absEnergy Energy

		// Try to read energy first (for RAPL zones)
		energyReading, energyErr := zone.Energy()
		powerReading, powerErr := zone.Power()

		// Detect zone type based on MaxEnergy and energy reading
		// hwmon zones have MaxEnergy() == 0 and Energy() returns 0
		isEnergySensor := zone.MaxEnergy() > 0 || energyReading > 0

		if isEnergySensor {
			// energy sensor
			absEnergy = energyReading

			if energyErr != nil {
				retErr = errors.Join(energyErr)
				pm.logger.Warn("Could not read energy for zone", "zone", zone.Name(), "index", zone.Index(), "error", energyErr)
				continue
			}

			pm.logger.Debug("Processing energy zone",
				"zone", zone.Name(),
				"type", "energy",
				"abs_energy", absEnergy,
				"max_energy", zone.MaxEnergy())
		} else {
			// power sensor
			if powerErr != nil {
				retErr = errors.Join(powerErr)
				pm.logger.Warn("Could not read power for zone", "zone", zone.Name(), "index", zone.Index(), "error", powerErr)
				continue
			}

			// Power is already in microwatts
			power = powerReading

			pm.logger.Debug("Processing power zone",
				"zone", zone.Name(),
				"type", "power",
				"power_watts", powerReading.Watts(),
				"power_microwatts", power.MicroWatts())

			// For power zones, we don't have absolute energy
			// We'll calculate deltaEnergy below using power and timeDiff
			absEnergy = 0
		}

		// Calculate watts and joules diff if we have previous data for the zone
		var activeEnergy, activeEnergyTotal, idleEnergyTotal Energy
		var activePower, idlePower Power

		if prevZone, ok := prevZones[zone]; ok {

			if isEnergySensor {
				// energy sensor
				// RAPL: Calculate delta from cumulative energy counters
				// Absolute is a running total, so to find the current energy usage, calculate the delta
				// delta = current - previous
				deltaEnergy = calculateEnergyDelta(absEnergy, prevZone.EnergyTotal, zone.MaxEnergy())

				// Derive power from energy delta: P = ΔE / Δt
				powerF64 := float64(deltaEnergy) / float64(timeDiff)
				power = Power(powerF64)

				pm.logger.Debug("Energy zone delta calculation",
					"zone", zone.Name(),
					"current", absEnergy,
					"previous", prevZone.EnergyTotal,
					"delta_energy", deltaEnergy,
					"time_diff", timeDiff,
					"power", power)
			} else {
				// power sensor
				// hwmon: Calculate energy from instantaneous power reading
				// Energy = Power × Time
				// E (µJ) = P (µW) × t (s)
				deltaEnergy = Energy(float64(power) * timeDiff)

				// For energy accumulation, we need to maintain a cumulative counter
				// even though hwmon doesn't provide one natively
				absEnergy = prevZone.EnergyTotal + deltaEnergy

				pm.logger.Debug("Power zone energy integration",
					"zone", zone.Name(),
					"power", power,
					"time_diff", timeDiff,
					"delta_energy", deltaEnergy,
					"accumulated_energy", absEnergy)
			}

			// Idle and Dynamic Division
			// active = delta * cpuUsage
			// idle = delta - active
			activeEnergy = Energy(float64(deltaEnergy) * nodeCPUUsageRatio)
			idleEnergy := deltaEnergy - activeEnergy

			activeEnergyTotal = prevZone.ActiveEnergyTotal + activeEnergy
			idleEnergyTotal = prevZone.IdleEnergyTotal + idleEnergy

			activePower = Power(float64(power) * nodeCPUUsageRatio)
			idlePower = power - activePower
			pm.logger.Debug("Active and idle power/energy",
				"active_power", activePower,
				"idle_power", idlePower,
				"active_energy", activeEnergy,
				"idle_energy", idleEnergy,
			)
		} else {
			// initial reading
			pm.logger.Debug("First reading for zone",
				"zone", zone.Name(),
				"type", map[bool]string{true: "energy", false: "power"}[isEnergySensor])

			// For first reading, we can't calculate delta or power
			// For power zones, we could use the current power reading, but
			// we'll be conservative and wait for the next sample
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

		var energy Energy
		var power Power

		energyReading, energyErr := zone.Energy()
		powerReading, powerErr := zone.Power()

		// Detect if this is an energy zone or power zone
		isEnergySensor := zone.MaxEnergy() > 0 || energyReading > 0

		if isEnergySensor {
			// energy sensor
			if energyErr != nil {
				retErr = errors.Join(energyErr)
				pm.logger.Warn("Could not read energy for zone", "zone", zone.Name(), "index", zone.Index(), "error", energyErr)
				continue
			}
			energy = energyReading

			pm.logger.Info("First read - energy zone",
				"zone", zone.Name(),
				"energy", energy)
		} else {
			// power sensor
			if powerErr != nil {
				retErr = errors.Join(powerErr)
				pm.logger.Warn("Could not read power for zone", "zone", zone.Name(), "index", zone.Index(), "error", powerErr)
				continue
			}

			// For first reading, we start with 0 accumulated energy
			// Next reading will integrate power over time
			energy = 0
			power = powerReading

			pm.logger.Info("First read - power zone",
				"zone", zone.Name(),
				"power_watts", powerReading.Watts(),
				"power_microwatts", power.MicroWatts())
		}

		activeEnergy := Energy(float64(energy) * nodeCPUUsageRatio)
		idleEnergy := energy - activeEnergy

		node.Zones[zone] = NodeUsage{
			EnergyTotal:       energy,
			ActiveEnergyTotal: activeEnergy,
			IdleEnergyTotal:   idleEnergy,
			activeEnergy:      activeEnergy,
			Power:             power, // Will be 0 for energy zones on first read
			// Power can't be calculated for energy zones in the first read since we need Δt
			// For power zones, we set it immediately
		}
	}

	return retErr
}
