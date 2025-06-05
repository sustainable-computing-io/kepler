// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package monitor

// firstVMRead initializes VM power data for the first time
func (pm *PowerMonitor) firstVMRead(snapshot *Snapshot) error {
	// Get the available zones to initialize each VM with the same zones
	zones, err := pm.cpu.Zones()
	if err != nil {
		return err
	}

	// Get the current running
	running := pm.resources.VirtualMachines().Running
	vms := make(VirtualMachines, len(running))

	// Add each container with zero energy/power for each zone
	for id, vm := range running {
		// Create new vm power entry
		newVM := &VirtualMachine{
			ID:           id,
			Name:         vm.Name,
			Hypervisor:   vm.Hypervisor,
			CPUTotalTime: vm.CPUTotalTime,
			Zones:        make(ZoneUsageMap, len(zones)),
		}

		// Initialize each zone with zero values
		for _, zone := range zones {
			newVM.Zones[zone] = &Usage{
				Absolute: Energy(0),
				Delta:    Energy(0),
				Power:    Power(0),
			}
		}

		vms[id] = newVM
	}

	// Store in snapshot
	snapshot.VirtualMachines = vms

	pm.logger.Debug("Initialized VM power tracking",
		"vm", len(vms),
		"zones_per_vm", len(zones))
	return nil
}

// calculateVMPower calculates power for each running VM
func (pm *PowerMonitor) calculateVMPower(prev, newSnapshot *Snapshot) error {
	vms := pm.resources.VirtualMachines()

	if len(vms.Running) == 0 {
		pm.logger.Debug("No running VM found, skipping power calculation")
		return nil
	}

	nodeCPUTimeDelta := pm.resources.Node().CPUTimeDelta
	pm.logger.Debug("Calculating VM power",
		"node.cpu.time", nodeCPUTimeDelta,
		"running", len(vms.Running),
	)

	// Initialize VM map
	vmMap := make(VirtualMachines, len(vms.Running))

	// For each VM, calculate power for each zone separately
	for id, vm := range vms.Running {
		newVM := &VirtualMachine{
			ID:           id,
			Name:         vm.Name,
			Hypervisor:   vm.Hypervisor,
			CPUTotalTime: vm.CPUTotalTime,
			Zones:        make(ZoneUsageMap),
		}

		// For each zone in the node, calculate VM's share
		for zone, nodeZoneUsage := range newSnapshot.Node.Zones {
			// Skip zones with zero power to avoid division by zero
			if nodeZoneUsage.Power == 0 || nodeZoneUsage.Delta == 0 || nodeCPUTimeDelta == 0 {
				newVM.Zones[zone] = &Usage{
					Power:    Power(0),
					Delta:    Energy(0),
					Absolute: Energy(0),
				}
				continue
			}

			// Calculate VM's share of this zone's power and energy
			cpuTimeRatio := vm.CPUTimeDelta / nodeCPUTimeDelta
			newVM.Zones[zone] = &Usage{
				Power: Power(cpuTimeRatio * nodeZoneUsage.ActivePower.MicroWatts()),
				Delta: Energy(cpuTimeRatio * float64(nodeZoneUsage.ActiveEnergy)),
			}

			// If we have previous data for this VM and zone, add to absolute energy
			if prev, exists := prev.VirtualMachines[id]; exists {
				if prevUsage, hasZone := prev.Zones[zone]; hasZone {
					newVM.Zones[zone].Absolute = prevUsage.Absolute + newVM.Zones[zone].Delta
				} else {
					// TODO: unlikely; so add telemetry for this
					newVM.Zones[zone].Absolute = newVM.Zones[zone].Delta
				}
			} else {
				// New VM, starts with delta
				newVM.Zones[zone].Absolute = newVM.Zones[zone].Delta
			}
		}

		vmMap[id] = newVM
	}

	newSnapshot.VirtualMachines = vmMap
	return nil
}
