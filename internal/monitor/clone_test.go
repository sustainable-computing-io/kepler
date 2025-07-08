// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package monitor

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sustainable-computing-io/kepler/internal/resource"
)

// fakeZone implements EnergyZone for testing
type fakeZone struct {
	name  string
	index int
}

func (z *fakeZone) Name() string            { return z.name }
func (z *fakeZone) Index() int              { return z.index }
func (z *fakeZone) Path() string            { return "/fake/path" }
func (z *fakeZone) Energy() (Energy, error) { return 0, nil }
func (z *fakeZone) MaxEnergy() Energy       { return 1000000 * Joule }

func TestNodeClone(t *testing.T) {
	t.Run("nil_safety", func(t *testing.T) {
		var node *Node
		clone := node.Clone()
		assert.Nil(t, clone, "Expected nil clone of nil Node")
	})

	t.Run("deep_copy", func(t *testing.T) {
		zone1 := &fakeZone{name: "package", index: 0}
		zone2 := &fakeZone{name: "core", index: 1}

		original := &Node{
			Timestamp:  time.Now(),
			UsageRatio: 0.75,
			Zones: NodeZoneUsageMap{
				zone1: NodeUsage{
					EnergyTotal:       1000 * Joule,
					Power:             50 * Watt,
					ActiveEnergyTotal: 800 * Joule,
					ActivePower:       40 * Watt,
					IdleEnergyTotal:   200 * Joule,
					IdlePower:         10 * Watt,
					activeEnergy:      800 * Joule,
				},
				zone2: NodeUsage{
					EnergyTotal: 500 * Joule,
					Power:       25 * Watt,
				},
			},
		}

		clone := original.Clone()
		require.NotNil(t, clone, "Clone should not be nil")

		// Verify all fields are copied
		assert.Equal(t, original.Timestamp, clone.Timestamp, "Timestamp should be copied")
		assert.Equal(t, original.UsageRatio, clone.UsageRatio, "UsageRatio should be copied")
		assert.Len(t, clone.Zones, len(original.Zones), "Zones length should match")

		// Verify initial zone values are copied
		assert.Equal(t, original.Zones[zone1], clone.Zones[zone1], "Zone1 values should be copied")
		assert.Equal(t, original.Zones[zone2], clone.Zones[zone2], "Zone2 values should be copied")

		// Modify clone and verify original is unchanged
		clone.UsageRatio = 0.9
		clone.Zones[zone1] = NodeUsage{EnergyTotal: 2000 * Joule, Power: 100 * Watt}

		assert.NotEqual(t, original.UsageRatio, clone.UsageRatio, "Original UsageRatio should be unchanged")
		assert.NotEqual(t, original.Zones[zone1].EnergyTotal, clone.Zones[zone1].EnergyTotal, "Original Zone values should be unchanged")

		// Verify clone has the modified values
		assert.Equal(t, 0.9, clone.UsageRatio, "Clone should have modified UsageRatio")
		assert.Equal(t, 2000*Joule, clone.Zones[zone1].EnergyTotal, "Clone should have modified EnergyTotal")
	})
}

func TestProcessClone(t *testing.T) {
	t.Run("nil_safety", func(t *testing.T) {
		var process *Process
		clone := process.Clone()
		assert.Nil(t, clone, "Expected nil clone of nil Process")
	})

	t.Run("deep_copy", func(t *testing.T) {
		zone := &fakeZone{name: "package", index: 0}

		original := &Process{
			PID:              1234,
			Comm:             "test-process",
			Exe:              "/usr/bin/test",
			Type:             resource.RegularProcess,
			CPUTotalTime:     100.5,
			ContainerID:      "container-123",
			VirtualMachineID: "vm-456",
			Zones: ZoneUsageMap{
				zone: Usage{
					EnergyTotal: 500 * Joule,
					Power:       25 * Watt,
				},
			},
		}

		clone := original.Clone()
		require.NotNil(t, clone, "Clone should not be nil")

		// Verify all fields are copied
		assert.Equal(t, original.PID, clone.PID, "PID should be copied")
		assert.Equal(t, original.Comm, clone.Comm, "Comm should be copied")
		assert.Equal(t, original.Exe, clone.Exe, "Exe should be copied")
		assert.Equal(t, original.Type, clone.Type, "Type should be copied")
		assert.Equal(t, original.CPUTotalTime, clone.CPUTotalTime, "CPUTotalTime should be copied")
		assert.Equal(t, original.ContainerID, clone.ContainerID, "ContainerID should be copied")
		assert.Equal(t, original.VirtualMachineID, clone.VirtualMachineID, "VirtualMachineID should be copied")
		assert.Equal(t, original.Zones[zone], clone.Zones[zone], "Zone values should be copied")

		// Verify deep copy behavior
		clone.Comm = "modified-process"
		clone.Zones[zone] = Usage{EnergyTotal: 1000 * Joule, Power: 50 * Watt}

		assert.NotEqual(t, original.Comm, clone.Comm, "Original Comm should be unchanged")
		assert.NotEqual(t, original.Zones[zone].EnergyTotal, clone.Zones[zone].EnergyTotal, "Original Zone values should be unchanged")

		// Verify clone modifications
		assert.Equal(t, "modified-process", clone.Comm, "Clone should have modified Comm")
		assert.Equal(t, 1000*Joule, clone.Zones[zone].EnergyTotal, "Clone should have modified EnergyTotal")
	})
}

func TestContainerClone(t *testing.T) {
	t.Run("nil_safety", func(t *testing.T) {
		var container *Container
		clone := container.Clone()
		assert.Nil(t, clone, "Expected nil clone of nil Container")
	})

	t.Run("deep_copy", func(t *testing.T) {
		zone := &fakeZone{name: "core", index: 1}

		original := &Container{
			ID:           "container-123",
			Name:         "test-container",
			Runtime:      resource.DockerRuntime,
			CPUTotalTime: 200.5,
			PodID:        "pod-789",
			Zones: ZoneUsageMap{
				zone: Usage{
					EnergyTotal: 300 * Joule,
					Power:       15 * Watt,
				},
			},
		}

		clone := original.Clone()
		require.NotNil(t, clone, "Clone should not be nil")

		// Verify all fields are copied
		assert.Equal(t, original.ID, clone.ID, "ID should be copied")
		assert.Equal(t, original.Name, clone.Name, "Name should be copied")
		assert.Equal(t, original.Runtime, clone.Runtime, "Runtime should be copied")
		assert.Equal(t, original.CPUTotalTime, clone.CPUTotalTime, "CPUTotalTime should be copied")
		assert.Equal(t, original.PodID, clone.PodID, "PodID should be copied")
		assert.Equal(t, original.Zones[zone], clone.Zones[zone], "Zone values should be copied")

		// Verify deep copy behavior
		clone.Name = "modified-container"
		clone.Zones[zone] = Usage{EnergyTotal: 600 * Joule, Power: 30 * Watt}

		assert.NotEqual(t, original.Name, clone.Name, "Original Name should be unchanged")
		assert.NotEqual(t, original.Zones[zone].EnergyTotal, clone.Zones[zone].EnergyTotal, "Original Zone values should be unchanged")

		// Verify clone modifications
		assert.Equal(t, "modified-container", clone.Name, "Clone should have modified Name")
		assert.Equal(t, 600*Joule, clone.Zones[zone].EnergyTotal, "Clone should have modified EnergyTotal")
	})
}

func TestVirtualMachineClone(t *testing.T) {
	t.Run("nil_safety", func(t *testing.T) {
		var vm *VirtualMachine
		clone := vm.Clone()
		assert.Nil(t, clone, "Expected nil clone of nil VirtualMachine")
	})

	t.Run("deep_copy", func(t *testing.T) {
		zone := &fakeZone{name: "dram", index: 2}

		original := &VirtualMachine{
			ID:           "vm-456",
			Name:         "test-vm",
			Hypervisor:   resource.KVMHypervisor,
			CPUTotalTime: 300.5,
			Zones: ZoneUsageMap{
				zone: Usage{
					EnergyTotal: 400 * Joule,
					Power:       20 * Watt,
				},
			},
		}

		clone := original.Clone()
		require.NotNil(t, clone, "Clone should not be nil")

		// Verify all fields are copied
		assert.Equal(t, original.ID, clone.ID, "ID should be copied")
		assert.Equal(t, original.Name, clone.Name, "Name should be copied")
		assert.Equal(t, original.Hypervisor, clone.Hypervisor, "Hypervisor should be copied")
		assert.Equal(t, original.CPUTotalTime, clone.CPUTotalTime, "CPUTotalTime should be copied")
		assert.Equal(t, original.Zones[zone], clone.Zones[zone], "Zone values should be copied")

		// Verify deep copy behavior
		clone.Name = "modified-vm"
		clone.Zones[zone] = Usage{EnergyTotal: 800 * Joule, Power: 40 * Watt}

		assert.NotEqual(t, original.Name, clone.Name, "Original Name should be unchanged")
		assert.NotEqual(t, original.Zones[zone].EnergyTotal, clone.Zones[zone].EnergyTotal, "Original Zone values should be unchanged")

		// Verify clone modifications
		assert.Equal(t, "modified-vm", clone.Name, "Clone should have modified Name")
		assert.Equal(t, 800*Joule, clone.Zones[zone].EnergyTotal, "Clone should have modified EnergyTotal")
	})
}

func TestPodClone(t *testing.T) {
	t.Run("nil_safety", func(t *testing.T) {
		var pod *Pod
		clone := pod.Clone()
		assert.Nil(t, clone, "Expected nil clone of nil Pod")
	})

	t.Run("deep_copy", func(t *testing.T) {
		zone := &fakeZone{name: "uncore", index: 3}

		original := &Pod{
			ID:           "pod-789",
			Name:         "test-pod",
			Namespace:    "default",
			CPUTotalTime: 150.0,
			Zones: ZoneUsageMap{
				zone: Usage{
					EnergyTotal: 250 * Joule,
					Power:       12 * Watt,
				},
			},
		}

		clone := original.Clone()
		require.NotNil(t, clone, "Clone should not be nil")

		// Verify all fields are copied
		assert.Equal(t, original.ID, clone.ID, "ID should be copied")
		assert.Equal(t, original.Name, clone.Name, "Name should be copied")
		assert.Equal(t, original.Namespace, clone.Namespace, "Namespace should be copied")
		assert.Equal(t, original.CPUTotalTime, clone.CPUTotalTime, "CPUTotalTime should be copied")
		assert.Equal(t, original.Zones[zone], clone.Zones[zone], "Zone values should be copied")

		// Verify deep copy behavior
		clone.Name = "modified-pod"
		clone.Zones[zone] = Usage{EnergyTotal: 500 * Joule, Power: 25 * Watt}

		assert.NotEqual(t, original.Name, clone.Name, "Original Name should be unchanged")
		assert.NotEqual(t, original.Zones[zone].EnergyTotal, clone.Zones[zone].EnergyTotal, "Original Zone values should be unchanged")

		// Verify clone modifications
		assert.Equal(t, "modified-pod", clone.Name, "Clone should have modified Name")
		assert.Equal(t, 500*Joule, clone.Zones[zone].EnergyTotal, "Clone should have modified EnergyTotal")
	})
}

func TestSnapshotClone(t *testing.T) {
	t.Run("deep_copy", func(t *testing.T) {
		zone := &fakeZone{name: "package", index: 0}
		now := time.Now()

		original := &Snapshot{
			Timestamp: now,
			Node: &Node{
				Timestamp:  now,
				UsageRatio: 0.8,
				Zones: NodeZoneUsageMap{
					zone: NodeUsage{EnergyTotal: 1000 * Joule, Power: 50 * Watt},
				},
			},
			Processes: Processes{
				"1": &Process{
					PID:  1,
					Comm: "init",
					Zones: ZoneUsageMap{
						zone: Usage{EnergyTotal: 100 * Joule, Power: 5 * Watt},
					},
				},
			},
			TerminatedProcesses: Processes{
				"999": &Process{
					PID:  999,
					Comm: "terminated-proc",
					Exe:  "/bin/terminated",
					Zones: ZoneUsageMap{
						zone: Usage{EnergyTotal: 150 * Joule, Power: 8 * Watt},
					},
				},
			},
			Containers: Containers{
				"c1": &Container{
					ID:   "c1",
					Name: "container1",
					Zones: ZoneUsageMap{
						zone: Usage{EnergyTotal: 200 * Joule, Power: 10 * Watt},
					},
				},
			},
			VirtualMachines: VirtualMachines{
				"vm1": &VirtualMachine{
					ID:   "vm1",
					Name: "vm1",
					Zones: ZoneUsageMap{
						zone: Usage{EnergyTotal: 300 * Joule, Power: 15 * Watt},
					},
				},
			},
			Pods: Pods{
				"p1": &Pod{
					ID:   "p1",
					Name: "pod1",
					Zones: ZoneUsageMap{
						zone: Usage{EnergyTotal: 400 * Joule, Power: 20 * Watt},
					},
				},
			},
		}

		clone := original.Clone()
		require.NotNil(t, clone, "Clone should not be nil")

		// Verify top-level fields
		assert.Equal(t, original.Timestamp, clone.Timestamp, "Timestamp should be copied")
		assert.NotSame(t, original.Node, clone.Node, "Node should be a different instance")

		// Verify maps are different instances by checking that they don't share the same underlying slice
		assert.NotSame(t, &original.Processes, &clone.Processes, "Processes map should be a different instance")
		assert.NotSame(t, &original.TerminatedProcesses, &clone.TerminatedProcesses, "TerminatedProcesses map should be a different instance")
		assert.NotSame(t, &original.Containers, &clone.Containers, "Containers map should be a different instance")
		assert.NotSame(t, &original.VirtualMachines, &clone.VirtualMachines, "VirtualMachines map should be a different instance")
		assert.NotSame(t, &original.Pods, &clone.Pods, "Pods map should be a different instance")

		// Verify map lengths
		assert.Len(t, clone.Processes, len(original.Processes), "Processes length should match")
		assert.Len(t, clone.TerminatedProcesses, len(original.TerminatedProcesses), "TerminatedProcesses length should match")
		assert.Len(t, clone.Containers, len(original.Containers), "Containers length should match")
		assert.Len(t, clone.VirtualMachines, len(original.VirtualMachines), "VirtualMachines length should match")
		assert.Len(t, clone.Pods, len(original.Pods), "Pods length should match")

		// Store original values
		originalNodeRatio := original.Node.UsageRatio
		originalNodeEnergy := original.Node.Zones[zone].EnergyTotal
		originalProcessComm := original.Processes["1"].Comm
		originalTerminatedComm := original.TerminatedProcesses["999"].Comm
		originalTerminatedEnergy := original.TerminatedProcesses["999"].Zones[zone].EnergyTotal
		originalContainerName := original.Containers["c1"].Name
		originalVMName := original.VirtualMachines["vm1"].Name
		originalPodName := original.Pods["p1"].Name

		// Verify deep copy by modifying clone
		clone.Node.UsageRatio = 0.9
		clone.Node.Zones[zone] = NodeUsage{EnergyTotal: 2000 * Joule, Power: 100 * Watt}
		clone.Processes["1"].Comm = "modified"
		clone.TerminatedProcesses["999"].Comm = "modified-terminated"
		clone.TerminatedProcesses["999"].Zones[zone] = Usage{EnergyTotal: 999 * Joule, Power: 99 * Watt}
		clone.Containers["c1"].Name = "modified"
		clone.VirtualMachines["vm1"].Name = "modified"
		clone.Pods["p1"].Name = "modified"

		// Verify original is unchanged
		assert.Equal(t, originalNodeRatio, original.Node.UsageRatio, "Original Node UsageRatio should be unchanged")
		assert.Equal(t, originalNodeEnergy, original.Node.Zones[zone].EnergyTotal, "Original Node zones should be unchanged")
		assert.Equal(t, originalProcessComm, original.Processes["1"].Comm, "Original Process should be unchanged")
		assert.Equal(t, originalTerminatedComm, original.TerminatedProcesses["999"].Comm, "Original TerminatedProcess should be unchanged")
		assert.Equal(t, originalTerminatedEnergy, original.TerminatedProcesses["999"].Zones[zone].EnergyTotal, "Original TerminatedProcess zones should be unchanged")
		assert.Equal(t, originalContainerName, original.Containers["c1"].Name, "Original Container should be unchanged")
		assert.Equal(t, originalVMName, original.VirtualMachines["vm1"].Name, "Original VirtualMachine should be unchanged")
		assert.Equal(t, originalPodName, original.Pods["p1"].Name, "Original Pod should be unchanged")

		// Verify clone has modified values
		assert.Equal(t, 0.9, clone.Node.UsageRatio, "Clone Node should have modified UsageRatio")
		assert.Equal(t, 2000*Joule, clone.Node.Zones[zone].EnergyTotal, "Clone Node should have modified EnergyTotal")
		assert.Equal(t, "modified", clone.Processes["1"].Comm, "Clone Process should have modified Comm")
		assert.Equal(t, "modified-terminated", clone.TerminatedProcesses["999"].Comm, "Clone TerminatedProcess should have modified Comm")
		assert.Equal(t, 999*Joule, clone.TerminatedProcesses["999"].Zones[zone].EnergyTotal, "Clone TerminatedProcess should have modified EnergyTotal")
		assert.Equal(t, "modified", clone.Containers["c1"].Name, "Clone Container should have modified Name")
		assert.Equal(t, "modified", clone.VirtualMachines["vm1"].Name, "Clone VirtualMachine should have modified Name")
		assert.Equal(t, "modified", clone.Pods["p1"].Name, "Clone Pod should have modified Name")
	})
}

func TestSnapshotTerminatedProcessesClone(t *testing.T) {
	t.Run("terminated_processes_deep_copy", func(t *testing.T) {
		zone1 := &fakeZone{name: "package", index: 0}
		zone2 := &fakeZone{name: "core", index: 1}

		original := &Snapshot{
			Timestamp: time.Now(),
			Node:      &Node{Zones: make(NodeZoneUsageMap)},
			Processes: make(Processes),
			TerminatedProcesses: Processes{
				"100": &Process{
					PID:          100,
					Comm:         "term-proc-1",
					Exe:          "/usr/bin/term-proc-1",
					Type:         resource.ContainerProcess,
					CPUTotalTime: 125.5,
					ContainerID:  "container-abc",
					Zones: ZoneUsageMap{
						zone1: Usage{EnergyTotal: 500 * Joule, Power: 25 * Watt},
						zone2: Usage{EnergyTotal: 300 * Joule, Power: 15 * Watt},
					},
				},
				"200": &Process{
					PID:              200,
					Comm:             "term-proc-2",
					Exe:              "/usr/bin/term-proc-2",
					Type:             resource.RegularProcess,
					CPUTotalTime:     89.2,
					VirtualMachineID: "vm-xyz",
					Zones: ZoneUsageMap{
						zone1: Usage{EnergyTotal: 750 * Joule, Power: 40 * Watt},
						zone2: Usage{EnergyTotal: 450 * Joule, Power: 22 * Watt},
					},
				},
			},
			Containers:      make(Containers),
			VirtualMachines: make(VirtualMachines),
			Pods:            make(Pods),
		}

		clone := original.Clone()
		require.NotNil(t, clone, "Clone should not be nil")

		// Verify TerminatedProcesses map independence
		assert.NotSame(t, &original.TerminatedProcesses, &clone.TerminatedProcesses, "TerminatedProcesses map should be different instance")
		assert.Len(t, clone.TerminatedProcesses, 2, "Clone should have same number of terminated processes")

		// Verify each terminated process is deeply cloned
		for id, originalProc := range original.TerminatedProcesses {
			clonedProc := clone.TerminatedProcesses[id]

			require.NotNil(t, clonedProc, "Cloned terminated process %d should exist", originalProc.PID)
			assert.NotSame(t, originalProc, clonedProc, "Terminated process %d should be different instance", originalProc.PID)

			// Verify all fields are copied correctly
			assert.Equal(t, originalProc.PID, clonedProc.PID)
			assert.Equal(t, originalProc.Comm, clonedProc.Comm)
			assert.Equal(t, originalProc.Exe, clonedProc.Exe)
			assert.Equal(t, originalProc.Type, clonedProc.Type)
			assert.Equal(t, originalProc.CPUTotalTime, clonedProc.CPUTotalTime)
			assert.Equal(t, originalProc.ContainerID, clonedProc.ContainerID)
			assert.Equal(t, originalProc.VirtualMachineID, clonedProc.VirtualMachineID)

			// Verify zones map independence and content
			assert.NotSame(t, &originalProc.Zones, &clonedProc.Zones, "Process %d zones map should be different instance", originalProc.PID)
			assert.Len(t, clonedProc.Zones, len(originalProc.Zones), "Process %d should have same number of zones", originalProc.PID)

			for zone, originalUsage := range originalProc.Zones {
				clonedUsage, exists := clonedProc.Zones[zone]
				require.True(t, exists, "Process %d should have zone %s", originalProc.PID, zone.Name())
				assert.Equal(t, originalUsage, clonedUsage, "Process %d zone %s usage should be identical", originalProc.PID, zone.Name())
			}
		}

		// Test deep copy isolation by modifying clone
		clone.TerminatedProcesses["100"].Comm = "modified-terminated"
		clone.TerminatedProcesses["100"].CPUTotalTime = 999.9
		clone.TerminatedProcesses["100"].Zones[zone1] = Usage{EnergyTotal: 9999 * Joule, Power: 888 * Watt}

		// Add new terminated process to clone
		newProcess := &Process{
			PID:  300,
			Comm: "new-terminated",
			Zones: ZoneUsageMap{
				zone1: Usage{EnergyTotal: 111 * Joule, Power: 11 * Watt},
			},
		}
		clone.TerminatedProcesses[newProcess.StringID()] = newProcess

		// Verify original is completely unchanged
		assert.Equal(t, "term-proc-1", original.TerminatedProcesses["100"].Comm, "Original process comm should be unchanged")
		assert.Equal(t, 125.5, original.TerminatedProcesses["100"].CPUTotalTime, "Original process CPU time should be unchanged")
		assert.Equal(t, 500*Joule, original.TerminatedProcesses["100"].Zones[zone1].EnergyTotal, "Original process zone energy should be unchanged")
		assert.Equal(t, 25*Watt, original.TerminatedProcesses["100"].Zones[zone1].Power, "Original process zone power should be unchanged")
		assert.Len(t, original.TerminatedProcesses, 2, "Original should not have new terminated process")

		// Verify clone has modified values
		assert.Equal(t, "modified-terminated", clone.TerminatedProcesses["100"].Comm, "Clone process should have modified comm")
		assert.Equal(t, 999.9, clone.TerminatedProcesses["100"].CPUTotalTime, "Clone process should have modified CPU time")
		assert.Equal(t, 9999*Joule, clone.TerminatedProcesses["100"].Zones[zone1].EnergyTotal, "Clone process should have modified energy")
		assert.Equal(t, 888*Watt, clone.TerminatedProcesses["100"].Zones[zone1].Power, "Clone process should have modified power")
		assert.Len(t, clone.TerminatedProcesses, 3, "Clone should have new terminated process")
	})

	t.Run("nil_terminated_processes", func(t *testing.T) {
		original := &Snapshot{
			Timestamp:           time.Now(),
			Node:                &Node{Zones: make(NodeZoneUsageMap)},
			Processes:           make(Processes),
			TerminatedProcesses: nil, // Explicitly nil
			Containers:          make(Containers),
			VirtualMachines:     make(VirtualMachines),
			Pods:                make(Pods),
		}

		clone := original.Clone()
		require.NotNil(t, clone, "Clone should not be nil")
		assert.Len(t, clone.TerminatedProcesses, 0, "Clone should have empty TerminatedProcesses slice when original is nil")
	})

	t.Run("empty_terminated_processes", func(t *testing.T) {
		original := &Snapshot{
			Timestamp:           time.Now(),
			Node:                &Node{Zones: make(NodeZoneUsageMap)},
			Processes:           make(Processes),
			TerminatedProcesses: make(Processes), // Empty but not nil
			Containers:          make(Containers),
			VirtualMachines:     make(VirtualMachines),
			Pods:                make(Pods),
		}

		clone := original.Clone()
		require.NotNil(t, clone, "Clone should not be nil")
		assert.Len(t, clone.TerminatedProcesses, 0, "Clone should have empty TerminatedProcesses slice")
		assert.NotSame(t, &original.TerminatedProcesses, &clone.TerminatedProcesses, "Even empty slices should be different instances")
	})
}

func TestCloneConcurrency(t *testing.T) {
	t.Run("concurrent_node_clones", func(t *testing.T) {
		zone := &fakeZone{name: "package", index: 0}

		original := &Node{
			Timestamp:  time.Now(),
			UsageRatio: 0.75,
			Zones: NodeZoneUsageMap{
				zone: NodeUsage{
					EnergyTotal: 1000 * Joule,
					Power:       50 * Watt,
				},
			},
		}

		const numGoroutines = 100
		var wg sync.WaitGroup
		clones := make([]*Node, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				clones[index] = original.Clone()
				// Modify the clone to verify independence
				clones[index].UsageRatio = float64(index)
				clones[index].Zones[zone] = NodeUsage{
					EnergyTotal: Energy(index) * Joule,
					Power:       Power(index) * Watt,
				}
			}(i)
		}

		wg.Wait()

		// Verify original is unchanged
		assert.Equal(t, 0.75, original.UsageRatio, "Original Node should be unchanged")
		assert.Equal(t, 1000*Joule, original.Zones[zone].EnergyTotal, "Original Node zones should be unchanged")

		// Verify all clones are unique and have expected values
		for i, clone := range clones {
			require.NotNil(t, clone, "Clone %d should not be nil", i)
			assert.Equal(t, float64(i), clone.UsageRatio, "Clone %d should have correct UsageRatio", i)
			assert.Equal(t, Energy(i)*Joule, clone.Zones[zone].EnergyTotal, "Clone %d should have correct EnergyTotal", i)
		}
	})

	t.Run("concurrent_snapshot_clones", func(t *testing.T) {
		zone := &fakeZone{name: "package", index: 0}

		original := &Snapshot{
			Timestamp: time.Now(),
			Node: &Node{
				Zones: NodeZoneUsageMap{
					zone: NodeUsage{EnergyTotal: 1000 * Joule, Power: 50 * Watt},
				},
			},
			Processes: Processes{
				"1": &Process{PID: 1, Zones: ZoneUsageMap{zone: Usage{EnergyTotal: 100 * Joule}}},
			},
		}

		const numGoroutines = 50
		var wg sync.WaitGroup
		clones := make([]*Snapshot, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				clones[index] = original.Clone()
				// Modify the clone
				clones[index].Node.Zones[zone] = NodeUsage{EnergyTotal: Energy(index) * Joule}
			}(i)
		}

		wg.Wait()

		// Verify original is unchanged
		assert.Equal(t, 1000*Joule, original.Node.Zones[zone].EnergyTotal, "Original snapshot should be unchanged")

		// Verify all clones have expected values
		for i, clone := range clones {
			require.NotNil(t, clone, "Clone %d should not be nil", i)
			assert.Equal(t, Energy(i)*Joule, clone.Node.Zones[zone].EnergyTotal, "Clone %d should have correct EnergyTotal", i)
		}
	})
}
