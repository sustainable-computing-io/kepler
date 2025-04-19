// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package monitor

import (
	"errors"
)

func (pm *PowerMonitor) calculateNodePower(snapshot *Snapshot, prevNode *Node) error {
	if prevNode == nil || prevNode.Timestamp.IsZero() {
		// No previous data, nothing to do
		return pm.firstNodeRead(snapshot)
	}
	// Get previous measurements for calculating watts
	prevReadTime := prevNode.Timestamp
	prevZones := prevNode.Zones

	now := pm.clock.Now()
	snapshot.Node.Timestamp = now

	// get zones first, before locking for read
	zones, err := pm.cpu.Zones()
	if err != nil {
		return err
	}

	// NOTE: energy is in MicroJoules and Power is in MicroWatts
	timeDiff := now.Sub(prevReadTime).Seconds()

	var retErr error
	for _, zone := range zones {
		absEnergy, err := zone.Energy()
		if err != nil {
			retErr = errors.Join(err)
			pm.logger.Warn("Could not read energy for zone", "zone", zone.Name(), "index", zone.Index(), "error", err)
			continue
		}

		// Calculate watts and joules diff if we have previous data for the zone
		var deltaEnergy Energy
		var power Power

		if prevZone, ok := prevZones[zone]; ok {
			deltaEnergy = calculateEnergyDelta(absEnergy, prevZone.Absolute, zone.MaxEnergy())
			power = Power(float64(deltaEnergy) / float64(timeDiff))
		}

		snapshot.Node.Zones[zone] = Usage{
			Absolute: absEnergy,
			Delta:    deltaEnergy,
			Power:    power,
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
func (pm *PowerMonitor) firstNodeRead(snapshot *Snapshot) error {
	snapshot.Node.Timestamp = pm.clock.Now()

	zones, err := pm.cpu.Zones()
	if err != nil {
		return err
	}

	var retErr error
	for _, zone := range zones {
		energy, err := zone.Energy()
		if err != nil {
			retErr = errors.Join(err)
			pm.logger.Warn("Could not read energy for zone", "zone", zone.Name(), "index", zone.Index(), "error", err)
			continue
		}

		snapshot.Node.Zones[zone] = Usage{
			Absolute: energy,
		}
	}

	return retErr
}
