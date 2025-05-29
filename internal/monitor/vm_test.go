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

	// Create mock resource informer
	resourceInformer := &MockResourceInformer{}

	// Create monitor with mocks
	monitor := &PowerMonitor{
		logger:    logger,
		cpu:       mockMeter,
		clock:     fakeClock,
		resources: resourceInformer,
	}

	err := monitor.initZones()
	require.NoError(t, err)

	t.Run("firstVMRead", func(t *testing.T) {
		vms := CreateTestVMs()
		resourceInformer.On("VirtualMachines").Return(vms).Once()

		snapshot := NewSnapshot()
		err := monitor.firstNodeRead(snapshot.Node)
		require.NoError(t, err)

		err = monitor.firstVMRead(snapshot)
		require.NoError(t, err)

		// Verify VMs were initialized
		assert.Len(t, snapshot.VirtualMachines, len(vms.Running))
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
			assert.Equal(t, Energy(0), usage.Absolute)
			assert.Equal(t, Energy(0), usage.Delta)
			assert.Equal(t, Power(0), usage.Power)
		}

		resourceInformer.AssertExpectations(t)
	})

	t.Run("calculateVMPower", func(t *testing.T) {
		// Setup previous snapshot with VM data
		prevSnapshot := NewSnapshot()
		prevSnapshot.Node = createNodeSnapshot(zones, fakeClock.Now())

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
			prevSnapshot.VirtualMachines["vm-1"].Zones[zone] = &Usage{
				Absolute: 30 * Joule,
				Delta:    Energy(0),
				Power:    Power(0),
			}
		}

		// Create new snapshot with updated node data
		newSnapshot := NewSnapshot()
		newSnapshot.Node = createNodeSnapshot(zones, fakeClock.Now().Add(time.Second))

		// Setup mock to return updated VMs
		vms := CreateTestVMs()
		resourceInformer.On("VirtualMachines").Return(vms).Once()

		err = monitor.calculateVMPower(prevSnapshot, newSnapshot)
		require.NoError(t, err)

		// Verify all VMs are present
		assert.Len(t, newSnapshot.VirtualMachines, 2)

		// Check VM-1 power calculations
		inputVM1 := vms.Running["vm-1"]
		vm1 := newSnapshot.VirtualMachines["vm-1"]
		assert.Equal(t, inputVM1.CPUTotalTime, vm1.CPUTotalTime) // Updated CPU time

		for _, zone := range zones {
			usage := vm1.Zones[zone]

			// CPU ratio = 60.0 / 100.0 = 0.6 (60%)
			// Expected power = 0.6 * 50W = 30W
			expectedPower := 0.6 * 50 * Watt // 30W in microwatts
			assert.InDelta(t, expectedPower.MicroWatts(), usage.Power.MicroWatts(), 0.01)

			expectedDelta := 0.6 * 50 * Joule // 30J in microjoules
			assert.InDelta(t, expectedDelta.MicroJoules(), usage.Delta.MicroJoules(), 0.01)

			// Absolute should be previous + delta = 30J + 30J = 60J
			expectedAbsolute := 60 * Joule
			assert.InDelta(t, expectedAbsolute.MicroJoules(), usage.Absolute.MicroJoules(), 0.01)
		}

		// Check VM-2 (new VM)
		vm2 := newSnapshot.VirtualMachines["vm-2"]
		for _, zone := range zones {
			usage := vm2.Zones[zone]
			// CPU ratio = 40.0 / 100.0 = 0.4 (40%)
			expectedPower := 0.4 * 50 * Watt // 20W
			assert.InDelta(t, expectedPower.MicroWatts(), usage.Power.MicroWatts(), 0.01)

			expectedDelta := Energy(0.4 * 50 * Joule) // 20J
			assert.InDelta(t, expectedDelta.MicroJoules(), usage.Delta.MicroJoules(), 0.01)
			// New VM, so absolute = delta
			assert.Equal(t, usage.Delta, usage.Absolute)
		}

		resourceInformer.AssertExpectations(t)
	})

	t.Run("calculateVMPower with zero node power", func(t *testing.T) {
		// Create node with zero power
		prevSnapshot := NewSnapshot()
		newSnapshot := NewSnapshot()
		newSnapshot.Node = &Node{
			Timestamp: fakeClock.Now(),
			Zones:     make(ZoneUsageMap),
		}

		// Set zero power for all zones
		for _, zone := range zones {
			newSnapshot.Node.Zones[zone] = &Usage{
				Absolute: Energy(100_000_000),
				Delta:    Energy(50_000_000),
				Power:    Power(0), // Zero power
			}
		}

		vms := CreateTestVMs()
		resourceInformer.On("VirtualMachines").Return(vms).Once()

		err := monitor.calculateVMPower(prevSnapshot, newSnapshot)
		require.NoError(t, err)

		// All VMs should have zero power
		for _, vm := range newSnapshot.VirtualMachines {
			for _, zone := range zones {
				usage := vm.Zones[zone]
				assert.Equal(t, Power(0), usage.Power)
				assert.Equal(t, Energy(0), usage.Delta)
				assert.Equal(t, Energy(0), usage.Absolute)
			}
		}

		resourceInformer.AssertExpectations(t)
	})

	t.Run("calculateVMPower without VMs", func(t *testing.T) {
		prevSnapshot := NewSnapshot()
		newSnapshot := NewSnapshot()
		newSnapshot.Node = createNodeSnapshot(zones, fakeClock.Now())

		// Return empty VMs
		emptyVMs := &resource.VirtualMachines{
			NodeCPUTimeDelta: 0.0,
			Running:          map[string]*resource.VirtualMachine{},
			Terminated:       map[string]*resource.VirtualMachine{},
		}
		resourceInformer.On("VirtualMachines").Return(emptyVMs).Once()

		err := monitor.calculateVMPower(prevSnapshot, newSnapshot)
		require.NoError(t, err)

		assert.Empty(t, newSnapshot.VirtualMachines)

		resourceInformer.AssertExpectations(t)
	})

	t.Run("calculateVMPower with zero CPU time delta", func(t *testing.T) {
		prevSnapshot := NewSnapshot()
		newSnapshot := NewSnapshot()
		newSnapshot.Node = createNodeSnapshot(zones, fakeClock.Now())

		// VM with zero CPU time delta - this can happen when CPU time hasn't changed
		vms := &resource.VirtualMachines{
			NodeCPUTimeDelta: 0.0, // Zero total CPU time delta
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
			assert.Equal(t, 0*Joule, usage.Delta)
			assert.Equal(t, 0*Joule, usage.Absolute)
		}

		resourceInformer.AssertExpectations(t)
	})

	t.Run("VM missing in previous snapshot", func(t *testing.T) {
		// Create snapshots where previous doesn't have zone data for a VM
		prevSnapshot := NewSnapshot()
		prevSnapshot.Node = createNodeSnapshot(zones, fakeClock.Now())

		// Add VM without zone data for one zone
		prevSnapshot.VirtualMachines["vm-1"] = &VirtualMachine{
			ID:    "vm-1",
			Name:  "test-vm",
			Zones: make(ZoneUsageMap),
		}
		// Only add data for the first zone, missing the second
		prevSnapshot.VirtualMachines["vm-1"].Zones[zones[0]] = &Usage{
			Absolute: Energy(15_000_000),
			Delta:    Energy(0),
			Power:    Power(0),
		}

		newSnapshot := NewSnapshot()
		newSnapshot.Node = createNodeSnapshot(zones, fakeClock.Now().Add(time.Second))

		// Create VM with all zones
		testVMs := &resource.VirtualMachines{
			NodeCPUTimeDelta: 100.0,
			Running: map[string]*resource.VirtualMachine{
				"vm-1": {
					ID:           "vm-1",
					Name:         "test-vm",
					Hypervisor:   resource.KVMHypervisor,
					CPUTotalTime: 20.0,
					CPUTimeDelta: 75.0,
				},
			},
			Terminated: map[string]*resource.VirtualMachine{},
		}

		resourceInformer.On("VirtualMachines").Return(testVMs).Once()

		err := monitor.calculateVMPower(prevSnapshot, newSnapshot)
		require.NoError(t, err)

		// VM should have power calculated for all zones
		vm := newSnapshot.VirtualMachines["vm-1"]
		assert.Len(t, vm.Zones, 2)

		// First zone should have accumulated absolute value
		usage0 := vm.Zones[zones[0]]
		assert.Greater(t, usage0.Absolute.MicroJoules(), usage0.Delta.MicroJoules())

		// Second zone should have absolute = delta (new zone)
		usage1 := vm.Zones[zones[1]]
		assert.Equal(t, usage1.Absolute, usage1.Delta)

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
		newSnapshot.Node = createNodeSnapshot(zones, fakeClock.Now())

		// Create VMs with varying CPU usage that consumes all node CPU
		testVMs := &resource.VirtualMachines{
			NodeCPUTimeDelta: 100.0, // Total node CPU time delta
			Running: map[string]*resource.VirtualMachine{
				"vm1": {ID: "vm1", Name: "vm1", Hypervisor: resource.KVMHypervisor, CPUTimeDelta: 25.0},
				"vm2": {ID: "vm2", Name: "vm2", Hypervisor: resource.KVMHypervisor, CPUTimeDelta: 35.0},
				"vm3": {ID: "vm3", Name: "vm3", Hypervisor: resource.KVMHypervisor, CPUTimeDelta: 40.0},
			},
			Terminated: map[string]*resource.VirtualMachine{},
		}

		mockResourceInformer.On("VirtualMachines").Return(testVMs).Once()

		err := monitor.calculateVMPower(prevSnapshot, newSnapshot)
		require.NoError(t, err)

		// Verify power conservation for each zone
		for _, zone := range zones {
			nodePower := newSnapshot.Node.Zones[zone].Power.MicroWatts()
			totalVMPower := float64(0)

			for _, vm := range newSnapshot.VirtualMachines {
				totalVMPower += vm.Zones[zone].Power.MicroWatts()
			}

			// Total VM power should equal node power (with small tolerance for floating point)
			assert.InDelta(t, nodePower, totalVMPower, 1.0,
				"Power conservation failed for zone %s", zone.Name())
		}

		mockResourceInformer.AssertExpectations(t)
	})

	t.Run("mixed workload power distribution", func(t *testing.T) {
		// Test power distribution when VMs only consume part of the node CPU
		prevSnapshot := NewSnapshot()
		newSnapshot := NewSnapshot()
		newSnapshot.Node = createNodeSnapshot(zones, fakeClock.Now())

		// VMs consume only 60% of node CPU (remaining 40% could be host processes)
		testVMs := &resource.VirtualMachines{
			NodeCPUTimeDelta: 100.0, // Total node CPU including host processes
			Running: map[string]*resource.VirtualMachine{
				"vm1": {ID: "vm1", Name: "vm1", Hypervisor: resource.KVMHypervisor, CPUTimeDelta: 20.0}, // 20%
				"vm2": {ID: "vm2", Name: "vm2", Hypervisor: resource.KVMHypervisor, CPUTimeDelta: 40.0}, // 40%
			},
			Terminated: map[string]*resource.VirtualMachine{},
		}

		mockResourceInformer.On("VirtualMachines").Return(testVMs).Once()

		err := monitor.calculateVMPower(prevSnapshot, newSnapshot)
		require.NoError(t, err)

		// Verify VM power ratios are correct
		for _, zone := range zones {
			nodePower := newSnapshot.Node.Zones[zone].Power.MicroWatts()

			vm1Power := newSnapshot.VirtualMachines["vm1"].Zones[zone].Power.MicroWatts()
			vm2Power := newSnapshot.VirtualMachines["vm2"].Zones[zone].Power.MicroWatts()

			// VM1 should consume 20% of node power
			expectedVM1Power := 0.20 * nodePower
			assert.InDelta(t, expectedVM1Power, vm1Power, 1.0,
				"VM1 power should be 20%% of node power for zone %s", zone.Name())

			// VM2 should consume 40% of node power
			expectedVM2Power := 0.40 * nodePower
			assert.InDelta(t, expectedVM2Power, vm2Power, 1.0,
				"VM2 power should be 40%% of node power for zone %s", zone.Name())

			// Total VM power should be 60% of node power
			totalVMPower := vm1Power + vm2Power
			expectedTotalVMPower := 0.60 * nodePower
			assert.InDelta(t, expectedTotalVMPower, totalVMPower, 1.0,
				"Total VM power should be 60%% of node power for zone %s", zone.Name())
		}

		mockResourceInformer.AssertExpectations(t)
	})

	mockMeter.AssertExpectations(t)
}

// CreateTestVMs creates test VMs with CPU time deltas for testing
func CreateTestVMs() *resource.VirtualMachines {
	return &resource.VirtualMachines{
		NodeCPUTimeDelta: 100.0, // Total node CPU time delta
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
	}
}
