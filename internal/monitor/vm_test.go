// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package monitor

import (
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/sustainable-computing-io/kepler/internal/resource"
	testingclock "k8s.io/utils/clock/testing"
)

func TestVMPowerCalculation(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	fakeClock := testingclock.NewFakeClock(time.Now())

	// Create mock CPU meter
	zones := CreateTestZones()
	mockMeter := &MockCPUPowerMeter{}
	mockMeter.On("Zones").Return(zones, nil)
	mockMeter.On("PrimaryEnergyZone").Return(zones[0], nil)

	// Create mock resource informer
	resourceInformer := &MockResourceInformer{}

	// Create monitor with mocks
	monitor := &PowerMonitor{
		logger:                       logger,
		cpu:                          mockMeter,
		clock:                        fakeClock,
		resources:                    resourceInformer,
		maxTerminated:                500,
		minTerminatedEnergyThreshold: 1 * Joule, // Set threshold to filter zero-energy VMs
	}

	err := monitor.Init()
	require.NoError(t, err)

	t.Run("firstVMRead", func(t *testing.T) {
		testData := CreateTestVMs()
		resourceInformer.On("Node").Return(testData.Node, nil).Maybe()
		resourceInformer.On("Node").Return(testData.Node, nil).Maybe() // Additional call for firstVMRead
		resourceInformer.On("VirtualMachines").Return(testData.VMs).Once()

		snapshot := NewSnapshot()
		err := monitor.firstNodeRead(snapshot.Node)
		require.NoError(t, err)

		err = monitor.firstVMRead(snapshot)
		require.NoError(t, err)

		// Verify VMs were initialized
		assert.Len(t, snapshot.VirtualMachines, len(testData.VMs.Running))
		assert.Contains(t, snapshot.VirtualMachines, "vm-1")
		assert.Contains(t, snapshot.VirtualMachines, "vm-2")

		// Check VM 1 details
		vm1 := snapshot.VirtualMachines["vm-1"]
		assert.Equal(t, "vm-1", vm1.ID)
		assert.Equal(t, "test-vm-1", vm1.Name)
		assert.Equal(t, resource.KVMHypervisor, vm1.Hypervisor)
		assert.Equal(t, 150.0, vm1.CPUTotalTime)

		// Verify zones are initialized with zero values
		assert.Len(t, vm1.Zones, 2)
		for _, zone := range zones {
			usage := vm1.Zones[zone]
			assert.Equal(t, Energy(0), usage.EnergyTotal)
			assert.Equal(t, Power(0), usage.Power)
		}

		resourceInformer.AssertExpectations(t)
	})

	t.Run("calculateVMPower", func(t *testing.T) {
		// Setup previous snapshot with VM data
		prevSnapshot := NewSnapshot()
		prevSnapshot.Node = createNodeSnapshot(zones, fakeClock.Now(), 0.5)

		// Add existing VM data to previous snapshot
		prevSnapshot.VirtualMachines["vm-1"] = &VirtualMachine{
			ID:           "vm-1",
			Name:         "test-vm-1",
			Hypervisor:   resource.KVMHypervisor,
			CPUTotalTime: 150.0,
			Zones:        make(ZoneUsageMap, len(zones)),
		}

		// Initialize zones for previous VM
		for _, zone := range zones {
			prevSnapshot.VirtualMachines["vm-1"].Zones[zone] = Usage{
				EnergyTotal: 30 * Joule,
				Power:       Power(0),
			}
		}

		// Create new snapshot with updated node data
		newSnapshot := NewSnapshot()
		newSnapshot.Node = createNodeSnapshot(zones, fakeClock.Now().Add(time.Second), 0.5)

		// Setup mock to return updated VMs
		testData := CreateTestVMs()
		resourceInformer.On("Node").Return(testData.Node, nil).Maybe()
		resourceInformer.On("VirtualMachines").Return(testData.VMs).Once()

		err = monitor.calculateVMPower(prevSnapshot, newSnapshot)
		require.NoError(t, err)

		// Verify all VMs are present
		assert.Len(t, newSnapshot.VirtualMachines, 2)

		// Check VM-1 power calculations
		inputVM1 := testData.VMs.Running["vm-1"]
		vm1 := newSnapshot.VirtualMachines["vm-1"]
		assert.Equal(t, inputVM1.CPUTotalTime, vm1.CPUTotalTime) // Updated CPU time

		for _, zone := range zones {
			usage := vm1.Zones[zone]

			// CPU ratio = 60.0 / 200.0 = 0.3 (30% of node CPU)
			// VM power = cpuTimeRatio * nodeZoneUsage.ActivePower
			// Expected power = 0.3 * 25W = 7.5W
			expectedPower := 0.3 * 25 * Watt // 7.5W in microwatts
			assert.InDelta(t, expectedPower.MicroWatts(), usage.Power.MicroWatts(), 0.01)

			// Absolute should be previous + delta = 30J + 15J = 45J
			expectedAbsolute := 45 * Joule
			assert.InDelta(t, expectedAbsolute.MicroJoules(), usage.EnergyTotal.MicroJoules(), 0.01)
		}

		// Check VM-2 (new VM)
		vm2 := newSnapshot.VirtualMachines["vm-2"]
		for _, zone := range zones {
			usage := vm2.Zones[zone]
			// CPU ratio = 40.0 / 200.0 = 0.2 (20% of node CPU)
			// Expected power = 0.2 * 25W = 5W
			expectedPower := 0.2 * 25
			assert.Equal(t, expectedPower, usage.Power.Watts())

			// New VM, so absolute = delta
			// Expected power = 0.2 * 50J = 10J
			expectedEnergyTotal := 0.2 * 50
			assert.Equal(t, expectedEnergyTotal, usage.EnergyTotal.Joules())
		}

		resourceInformer.AssertExpectations(t)
	})

	t.Run("calculateVMPower with zero node power", func(t *testing.T) {
		// Create node with zero power
		prevSnapshot := NewSnapshot()
		newSnapshot := NewSnapshot()
		newSnapshot.Node = &Node{
			Timestamp:  fakeClock.Now(),
			UsageRatio: 0.5,
			Zones:      make(NodeZoneUsageMap),
		}

		// Set zero power for all zones
		for _, zone := range zones {
			newSnapshot.Node.Zones[zone] = NodeUsage{
				EnergyTotal:       Energy(100_000_000),
				activeEnergy:      Energy(0),
				ActiveEnergyTotal: Energy(0),
				IdleEnergyTotal:   Energy(0),

				Power:       Power(0), // Zero power
				ActivePower: Power(0),
				IdlePower:   Power(0),
			}
		}

		testData := CreateTestVMs()
		resourceInformer.On("Node").Return(testData.Node, nil).Maybe()
		resourceInformer.On("VirtualMachines").Return(testData.VMs).Once()

		err := monitor.calculateVMPower(prevSnapshot, newSnapshot)
		require.NoError(t, err)

		// All VMs should have zero power
		for _, vm := range newSnapshot.VirtualMachines {
			for _, zone := range zones {
				usage := vm.Zones[zone]
				assert.Equal(t, Power(0), usage.Power)
				assert.Equal(t, Energy(0), usage.EnergyTotal)
			}
		}

		resourceInformer.AssertExpectations(t)
	})

	t.Run("calculateVMPower without VMs", func(t *testing.T) {
		prevSnapshot := NewSnapshot()
		newSnapshot := NewSnapshot()
		newSnapshot.Node = createNodeSnapshot(zones, fakeClock.Now(), 0.5)

		// Return empty VMs
		emptyVMs := &resource.VirtualMachines{
			Running:    map[string]*resource.VirtualMachine{},
			Terminated: map[string]*resource.VirtualMachine{},
		}
		tr := CreateTestResources(createOnly(testNode))
		resourceInformer.On("Node").Return(tr.Node, nil).Maybe()
		resourceInformer.On("VirtualMachines").Return(emptyVMs).Once()

		err := monitor.calculateVMPower(prevSnapshot, newSnapshot)
		require.NoError(t, err)

		assert.Empty(t, newSnapshot.VirtualMachines)

		resourceInformer.AssertExpectations(t)
	})

	t.Run("calculateVMPower with zero CPU time delta", func(t *testing.T) {
		prevSnapshot := NewSnapshot()
		newSnapshot := NewSnapshot()
		newSnapshot.Node = createNodeSnapshot(zones, fakeClock.Now(), 0.5)

		// VM with zero CPU time delta - this can happen when CPU time hasn't changed
		vms := &resource.VirtualMachines{
			Running: map[string]*resource.VirtualMachine{
				"vm-zero": {
					ID:           "vm-zero",
					Name:         "test-vm-zero",
					Hypervisor:   resource.KVMHypervisor,
					CPUTotalTime: 10.0,
					CPUTimeDelta: 0.0, // Zero delta
				},
			},
			Terminated: map[string]*resource.VirtualMachine{},
		}

		tr := CreateTestResources()
		resourceInformer.On("Node").Return(tr.Node, nil).Maybe()
		resourceInformer.On("VirtualMachines").Return(vms).Once()

		err := monitor.calculateVMPower(prevSnapshot, newSnapshot)
		require.NoError(t, err)

		// VM should exist but with zero power/energy
		vm := newSnapshot.VirtualMachines["vm-zero"]
		assert.NotNil(t, vm)

		// All zones should have zero values due to division by zero protection
		for _, zone := range zones {
			usage := vm.Zones[zone]
			assert.Equal(t, 0*Watt, usage.Power)
			assert.Equal(t, 0*Joule, usage.EnergyTotal)
		}

		resourceInformer.AssertExpectations(t)
	})

	t.Run("VM missing zone in previous snapshot", func(t *testing.T) {
		// Create snapshots where previous doesn't have zone data for a VM
		prevSnapshot := NewSnapshot()
		prevSnapshot.Node = createNodeSnapshot(zones, fakeClock.Now(), 0.5)

		// Add VM without zone data for one zone
		prevSnapshot.VirtualMachines["vm-1"] = &VirtualMachine{
			ID:    "vm-1",
			Name:  "test-vm",
			Zones: make(ZoneUsageMap),
		}
		// Only add data for the first zone, missing the second
		prevSnapshot.VirtualMachines["vm-1"].Zones[zones[0]] = Usage{
			EnergyTotal: 15 * Joule,
			Power:       3 * Watt,
		}

		newSnapshot := NewSnapshot()
		newSnapshot.Node = createNodeSnapshot(zones, fakeClock.Now().Add(time.Second), 0.6)

		// Create VM with all zones
		testVMs := &resource.VirtualMachines{
			Running: map[string]*resource.VirtualMachine{
				"vm-1": {
					ID:           "vm-1",
					Name:         "test-vm",
					Hypervisor:   resource.KVMHypervisor,
					CPUTotalTime: 20.0,
					CPUTimeDelta: 60.0,
				},
			},
			Terminated: map[string]*resource.VirtualMachine{},
		}

		tr := CreateTestResources(createOnly(testNode))
		resourceInformer.On("Node").Return(tr.Node, nil).Maybe()
		resourceInformer.On("VirtualMachines").Return(testVMs).Once()

		err := monitor.calculateVMPower(prevSnapshot, newSnapshot)
		require.NoError(t, err)

		// VM should have power calculated for all zones
		vm := newSnapshot.VirtualMachines["vm-1"]
		assert.Len(t, vm.Zones, 2)

		nodeZones := newSnapshot.Node.Zones
		assert.Equal(t, 60.0, nodeZones[zones[0]].activeEnergy.Joules())
		assert.Equal(t, 60.0, nodeZones[zones[1]].activeEnergy.Joules())

		assert.Equal(t, 30.0, nodeZones[zones[0]].ActivePower.Watts())
		assert.Equal(t, 30.0, nodeZones[zones[1]].ActivePower.Watts())

		// First zone should have accumulated absolute value
		usage0 := vm.Zones[zones[0]]
		usage1 := vm.Zones[zones[1]]

		assert.Equal(t, 33.0, usage0.EnergyTotal.Joules()) // 5J + 18J
		assert.Equal(t, 18.0, usage1.EnergyTotal.Joules()) // 0J + 18J

		assert.Equal(t, 9.0, usage0.Power.Watts()) // 5W + 7.5W
		assert.Equal(t, 9.0, usage1.Power.Watts()) // 0W + 7.5W

		resourceInformer.AssertExpectations(t)
	})

	t.Run("calculateVMPower with terminated VMs", func(t *testing.T) {
		// Create previous snapshot with VMs that will be terminated
		prevSnapshot := NewSnapshot()
		prevSnapshot.Node = createNodeSnapshot(zones, fakeClock.Now(), 0.5)

		// Add VMs to previous snapshot
		prevSnapshot.VirtualMachines["vm-running"] = &VirtualMachine{
			ID:           "vm-running",
			Name:         "running-vm",
			Hypervisor:   resource.KVMHypervisor,
			CPUTotalTime: 100.0,
			Zones:        make(ZoneUsageMap, len(zones)),
		}
		prevSnapshot.VirtualMachines["vm-terminated"] = &VirtualMachine{
			ID:           "vm-terminated",
			Name:         "terminated-vm",
			Hypervisor:   resource.KVMHypervisor,
			CPUTotalTime: 80.0,
			Zones:        make(ZoneUsageMap, len(zones)),
		}
		prevSnapshot.VirtualMachines["vm-zero-energy"] = &VirtualMachine{
			ID:           "vm-zero-energy",
			Name:         "zero-energy-vm",
			Hypervisor:   resource.KVMHypervisor,
			CPUTotalTime: 60.0,
			Zones:        make(ZoneUsageMap, len(zones)),
		}

		// Set energy for VMs - terminated VM has energy, zero-energy VM has zero
		for _, zone := range zones {
			prevSnapshot.VirtualMachines["vm-running"].Zones[zone] = Usage{
				EnergyTotal: 50 * Joule,
				Power:       10 * Watt,
			}
			prevSnapshot.VirtualMachines["vm-terminated"].Zones[zone] = Usage{
				EnergyTotal: 30 * Joule, // Non-zero energy
				Power:       5 * Watt,
			}
			prevSnapshot.VirtualMachines["vm-zero-energy"].Zones[zone] = Usage{
				EnergyTotal: 0 * Joule, // Zero energy - should be filtered out
				Power:       0 * Watt,
			}
		}

		// Create new snapshot
		newSnapshot := NewSnapshot()
		newSnapshot.Node = createNodeSnapshot(zones, fakeClock.Now().Add(time.Second), 0.5)

		// Only vm-running is still running, others are terminated
		vmsWithTerminated := &resource.VirtualMachines{
			Running: map[string]*resource.VirtualMachine{
				"vm-running": {
					ID:           "vm-running",
					Name:         "running-vm",
					Hypervisor:   resource.KVMHypervisor,
					CPUTotalTime: 120.0,
					CPUTimeDelta: 20.0,
				},
			},
			Terminated: map[string]*resource.VirtualMachine{
				"vm-terminated": {
					ID:           "vm-terminated",
					Name:         "terminated-vm",
					Hypervisor:   resource.KVMHypervisor,
					CPUTotalTime: 80.0,
					CPUTimeDelta: 0.0,
				},
				"vm-zero-energy": {
					ID:           "vm-zero-energy",
					Name:         "zero-energy-vm",
					Hypervisor:   resource.KVMHypervisor,
					CPUTotalTime: 60.0,
					CPUTimeDelta: 0.0,
				},
			},
		}

		tr := CreateTestResources()
		resourceInformer.On("Node").Return(tr.Node, nil).Maybe()
		resourceInformer.On("VirtualMachines").Return(vmsWithTerminated).Once()

		err := monitor.calculateVMPower(prevSnapshot, newSnapshot)
		require.NoError(t, err)

		// Populate terminated resources from trackers (normally done by refreshSnapshot)
		newSnapshot.TerminatedVirtualMachines = monitor.terminatedVMsTracker.Items()

		// Verify running VM is still present
		assert.Len(t, newSnapshot.VirtualMachines, 1)
		assert.Contains(t, newSnapshot.VirtualMachines, "vm-running")

		// Verify terminated VMs - only the one with non-zero energy should be included
		assert.Len(t, newSnapshot.TerminatedVirtualMachines, 1)

		// Find terminated VM by ID
		var terminatedVM *VirtualMachine
		for _, vm := range newSnapshot.TerminatedVirtualMachines {
			if vm.ID == "vm-terminated" {
				terminatedVM = vm
				break
			}
		}
		require.NotNil(t, terminatedVM, "vm-terminated should exist in terminated VMs")

		// Ensure vm-zero-energy is not in terminated VMs
		foundZeroEnergy := false
		for _, vm := range newSnapshot.TerminatedVirtualMachines {
			if vm.ID == "vm-zero-energy" {
				foundZeroEnergy = true
				break
			}
		}
		assert.False(t, foundZeroEnergy, "vm-zero-energy should not be in terminated VMs")

		// Check that terminated VM retains its energy from previous snapshot
		assert.Equal(t, "vm-terminated", terminatedVM.ID)
		assert.Equal(t, "terminated-vm", terminatedVM.Name)
		assert.Equal(t, 80.0, terminatedVM.CPUTotalTime)

		for _, zone := range zones {
			usage := terminatedVM.Zones[zone]
			assert.Equal(t, 30*Joule, usage.EnergyTotal)
			assert.Equal(t, 5*Watt, usage.Power)
		}

		resourceInformer.AssertExpectations(t)
	})

	mockMeter.AssertExpectations(t)
}

func TestVMPowerConsistency(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	fakeClock := testingclock.NewFakeClock(time.Now())

	// Create mock CPU meter
	mockMeter := &MockCPUPowerMeter{}
	zones := CreateTestZones()
	mockMeter.On("Zones").Return(zones, nil)
	mockMeter.On("PrimaryEnergyZone").Return(zones[0], nil)

	// Create mock resource informer
	mockResourceInformer := &MockResourceInformer{}

	// Create monitor with mocks
	monitor := &PowerMonitor{
		logger:    logger,
		cpu:       mockMeter,
		clock:     fakeClock,
		resources: mockResourceInformer,
	}

	err := monitor.Init()
	require.NoError(t, err)

	t.Run("power conservation across VMs", func(t *testing.T) {
		// Test that sum of VM powers equals node power when VMs consume all CPU
		prevSnapshot := NewSnapshot()
		newSnapshot := NewSnapshot()
		newSnapshot.Node = createNodeSnapshot(zones, fakeClock.Now(), 0.5)

		// Create VMs with varying CPU usage that consumes all node CPU
		testVMs := &resource.VirtualMachines{
			Running: map[string]*resource.VirtualMachine{
				"vm1": {ID: "vm1", Name: "vm1", Hypervisor: resource.KVMHypervisor, CPUTimeDelta: 25.0},
				"vm2": {ID: "vm2", Name: "vm2", Hypervisor: resource.KVMHypervisor, CPUTimeDelta: 35.0},
				"vm3": {ID: "vm3", Name: "vm3", Hypervisor: resource.KVMHypervisor, CPUTimeDelta: 40.0},
			},
			Terminated: map[string]*resource.VirtualMachine{},
		}

		tr := CreateTestResources()
		mockResourceInformer.On("Node").Return(tr.Node, nil).Maybe()
		mockResourceInformer.On("VirtualMachines").Return(testVMs).Once()

		err := monitor.calculateVMPower(prevSnapshot, newSnapshot)
		require.NoError(t, err)

		// Verify power conservation for each zone
		for _, zone := range zones {
			totalVMPower := float64(0)

			for _, vm := range newSnapshot.VirtualMachines {
				totalVMPower += vm.Zones[zone].Power.MicroWatts()
			}

			// VMs have total CPU delta of 100.0 out of node's 200.0 (50%)
			// So total VM power should be 50% of node ActivePower
			nodeActivePower := newSnapshot.Node.Zones[zone].ActivePower.MicroWatts()
			expectedTotalVMPower := nodeActivePower * 0.5 // 50% of used power
			assert.InDelta(t, expectedTotalVMPower, totalVMPower, 1.0,
				"Power conservation failed for zone %s", zone.Name())
		}

		mockResourceInformer.AssertExpectations(t)
	})

	t.Run("mixed workload power distribution", func(t *testing.T) {
		// Test power distribution when VMs only consume part of the node CPU
		prevSnapshot := NewSnapshot()
		newSnapshot := NewSnapshot()
		newSnapshot.Node = createNodeSnapshot(zones, fakeClock.Now(), 0.5)

		// VMs consume only 60% of node CPU (remaining 40% could be host processes)
		testVMs := &resource.VirtualMachines{
			Running: map[string]*resource.VirtualMachine{
				"vm1": {ID: "vm1", Name: "vm1", Hypervisor: resource.KVMHypervisor, CPUTimeDelta: 20.0}, // 20%
				"vm2": {ID: "vm2", Name: "vm2", Hypervisor: resource.KVMHypervisor, CPUTimeDelta: 40.0}, // 40%
			},
			Terminated: map[string]*resource.VirtualMachine{},
		}

		tr := CreateTestResources()
		mockResourceInformer.On("Node").Return(tr.Node, nil).Maybe()
		mockResourceInformer.On("VirtualMachines").Return(testVMs).Once()

		err := monitor.calculateVMPower(prevSnapshot, newSnapshot)
		require.NoError(t, err)

		// Verify VM power ratios are correct
		for _, zone := range zones {
			vm1Power := newSnapshot.VirtualMachines["vm1"].Zones[zone].Power.MicroWatts()
			vm2Power := newSnapshot.VirtualMachines["vm2"].Zones[zone].Power.MicroWatts()

			// VM1 has 20.0 CPU delta out of 200.0 total node CPU delta = 10% of node usage
			nodeActivePower := newSnapshot.Node.Zones[zone].ActivePower.MicroWatts()
			expectedVM1Power := 0.10 * nodeActivePower // 10% of used power
			assert.InDelta(t, expectedVM1Power, vm1Power, 1.0,
				"VM1 power should be 10%% of node used power for zone %s", zone.Name())

			// VM2 has 40.0 CPU delta out of 200.0 total node CPU delta = 20% of node usage
			expectedVM2Power := 0.20 * nodeActivePower // 20% of used power
			assert.InDelta(t, expectedVM2Power, vm2Power, 1.0,
				"VM2 power should be 20%% of node used power for zone %s", zone.Name())

			// Total VM power should be 30% of node used power (10% + 20%)
			totalVMPower := vm1Power + vm2Power
			expectedTotalVMPower := 0.30 * nodeActivePower // 30% of used power
			assert.InDelta(t, expectedTotalVMPower, totalVMPower, 1.0,
				"Total VM power should be 30%% of node used power for zone %s", zone.Name())
		}

		mockResourceInformer.AssertExpectations(t)
	})

	mockMeter.AssertExpectations(t)
}

// VMTestData holds test data for VM tests
type VMTestData struct {
	Node *resource.Node
	VMs  *resource.VirtualMachines
}

// CreateTestVMs creates test VMs with CPU time deltas and a test node for testing
func CreateTestVMs() *VMTestData {
	return &VMTestData{
		Node: &resource.Node{
			CPUUsageRatio:            0.5,
			ProcessTotalCPUTimeDelta: 200.0,
		},
		VMs: &resource.VirtualMachines{
			Running: map[string]*resource.VirtualMachine{
				"vm-1": {
					ID:           "vm-1",
					Name:         "test-vm-1",
					Hypervisor:   resource.KVMHypervisor,
					CPUTotalTime: 150.0,
					CPUTimeDelta: 60.0, // 60% of total CPU time
				},
				"vm-2": {
					ID:           "vm-2",
					Name:         "test-vm-2",
					Hypervisor:   resource.KVMHypervisor,
					CPUTotalTime: 80.0,
					CPUTimeDelta: 40.0, // 40% of total CPU time
				},
			},
			Terminated: map[string]*resource.VirtualMachine{},
		},
	}
}
