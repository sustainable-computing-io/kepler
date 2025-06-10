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

func TestProcessPowerCalculation(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	fakeClock := testingclock.NewFakeClock(time.Now())

	// Create mock CPU meter
	zones := CreateTestZones()
	mockMeter := &MockCPUPowerMeter{}
	mockMeter.On("Zones").Return(zones, nil)

	// Create mock resource informer
	resInformer := &MockResourceInformer{}

	// Create monitor with mocks
	monitor := &PowerMonitor{
		logger:    logger,
		cpu:       mockMeter,
		clock:     fakeClock,
		resources: resInformer,
	}

	err := monitor.initZones()
	require.NoError(t, err)

	t.Run("firstProcessRead", func(t *testing.T) {
		// Setup mock to return test processes
		tr := CreateTestResources(createOnly(testProcesses, testNode))
		procs := tr.Processes
		resInformer.SetExpectations(t, tr)

		snapshot := NewSnapshot()
		err := monitor.firstNodeRead(snapshot.Node)
		require.NoError(t, err)

		err = monitor.firstProcessRead(snapshot)
		require.NoError(t, err)

		// Verify processes were initialized
		assert.Len(t, snapshot.Processes, len(procs.Running))
		assert.Contains(t, snapshot.Processes, 123)
		assert.Contains(t, snapshot.Processes, 456)
		assert.Contains(t, snapshot.Processes, 789)

		// Check process 123 (in container-1)
		proc123 := snapshot.Processes[123]
		assert.Equal(t, 123, proc123.PID)
		assert.Equal(t, "process1", proc123.Comm)
		assert.Equal(t, "/usr/bin/process1", proc123.Exe)
		assert.Equal(t, "container-1", proc123.ContainerID)
		assert.Equal(t, 100.0, proc123.CPUTotalTime)

		// Verify zones are initialized with zero values
		assert.Len(t, proc123.Zones, 2)
		for _, zone := range zones {
			usage := proc123.Zones[zone]
			assert.Equal(t, Energy(0), usage.EnergyTotal)
			assert.Equal(t, Power(0), usage.Power)
		}

		// Check process 789 (not in container)
		proc789 := snapshot.Processes[789]
		assert.Equal(t, "", proc789.ContainerID)

		resInformer.AssertExpectations(t)
	})

	t.Run("calculateProcessPower", func(t *testing.T) {
		// Setup previous snapshot with process data
		prevSnapshot := NewSnapshot()
		prevSnapshot.Node = createNodeSnapshot(zones, fakeClock.Now(), 0.5)

		// Add existing process data to previous snapshot
		prevSnapshot.Processes[123] = &Process{
			PID:          123,
			Comm:         "process1",
			Exe:          "/usr/bin/process1",
			CPUTotalTime: 100.0,
			ContainerID:  "container-1",
			Zones:        make(ZoneUsageMap, len(zones)),
		}

		// Initialize zones for previous process
		for _, zone := range zones {
			prevSnapshot.Processes[123].Zones[zone] = &Usage{
				EnergyTotal: 25 * Joule,
				Power:       Power(0),
			}
		}

		// Create new snapshot with updated node data
		newSnapshot := NewSnapshot()
		newSnapshot.Node = createNodeSnapshot(zones, fakeClock.Now().Add(time.Second), 0.5)

		// Setup mock to return updated processes
		tr := CreateTestResources(createOnly(testNode, testProcesses))
		procs := tr.Processes
		resInformer.On("Node").Return(tr.Node, nil).Maybe()
		resInformer.On("Processes").Return(procs).Once()

		err = monitor.calculateProcessPower(prevSnapshot, newSnapshot)
		require.NoError(t, err)

		// Verify all processes are present
		assert.Len(t, newSnapshot.Processes, len(procs.Running))

		// Check process 123 power calculations
		inputProc123 := procs.Running[123]
		proc123 := newSnapshot.Processes[123]
		assert.Equal(t, inputProc123.CPUTotalTime, proc123.CPUTotalTime) // Updated CPU time

		for _, zone := range zones {
			usage := proc123.Zones[zone]

			// CPU ratio = 30.0 / 100.0 = 0.3 (30%)
			// ActivePower = 50W * 0.5 = 25W (node usage ratio is 0.5)
			// Expected power = 0.3 * 25W = 7.5W
			expectedPower := 0.3 * 25 * Watt // 7.5W in microwatts
			assert.InDelta(t, expectedPower.MicroWatts(), usage.Power.MicroWatts(), 0.01)

			// Absolute should be previous + delta = 25J + 15J = 40J
			expectedAbsolute := 40 * Joule
			assert.InDelta(t, expectedAbsolute.MicroJoules(), usage.EnergyTotal.MicroJoules(), 0.01)
		}

		// Check process 456 (new process)
		proc456 := newSnapshot.Processes[456]
		for _, zone := range zones {
			usage := proc456.Zones[zone]
			// CPU ratio = 20.0 / 100.0 = 0.2 (20%)
			// ActivePower = 50W * 0.5 = 25W, so 0.2 * 25W = 5W
			expectedPower := Power(0.2 * 25_000_000) // 5W
			assert.InDelta(t, expectedPower.MicroWatts(), usage.Power.MicroWatts(), 0.01)
		}

		// Check process 789 (not in container)
		proc789 := newSnapshot.Processes[789]
		for _, zone := range zones {
			usage := proc789.Zones[zone]
			// CPU ratio = 15.0 / 100.0 = 0.15 (15%)
			// ActivePower = 50W * 0.5 = 25W, so 0.15 * 25W = 3.75W
			expectedPower := Power(0.15 * 25_000_000) // 3.75W
			assert.InDelta(t, expectedPower.MicroWatts(), usage.Power.MicroWatts(), 0.01)
		}

		resInformer.AssertExpectations(t)
	})

	t.Run("calculateProcessPower with zero node power", func(t *testing.T) {
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
			newSnapshot.Node.Zones[zone] = &NodeUsage{
				EnergyTotal:       Energy(100_000_000),
				activeEnergy:      Energy(0),
				ActiveEnergyTotal: Energy(0),
				IdleEnergyTotal:   Energy(0),
				Power:             Power(0), // Zero power
				ActivePower:       Power(0),
				IdlePower:         Power(0),
			}
		}

		tr := CreateTestResources(createOnly(testNode, testProcesses))
		procs := tr.Processes
		resInformer.On("Node").Return(tr.Node, nil).Maybe()
		resInformer.On("Processes").Return(procs).Once()

		err := monitor.calculateProcessPower(prevSnapshot, newSnapshot)
		require.NoError(t, err)

		// All processes should have zero power
		for _, proc := range newSnapshot.Processes {
			for _, zone := range zones {
				usage := proc.Zones[zone]
				assert.Equal(t, Power(0), usage.Power)
				assert.Equal(t, Energy(0), usage.EnergyTotal)
			}
		}

		resInformer.AssertExpectations(t)
	})

	t.Run("calculateProcessPower with no processes", func(t *testing.T) {
		prevSnapshot := NewSnapshot()
		newSnapshot := NewSnapshot()
		newSnapshot.Node = createNodeSnapshot(zones, fakeClock.Now(), 0.5)

		// Return empty processes
		emptyProcesses := &resource.Processes{
			Running:    map[int]*resource.Process{},
			Terminated: map[int]*resource.Process{},
		}
		tr := CreateTestResources(createOnly(testNode))
		node := tr.Node
		resInformer.On("Node").Return(node, nil).Maybe()
		resInformer.On("Processes").Return(emptyProcesses).Once()

		err := monitor.calculateProcessPower(prevSnapshot, newSnapshot)
		require.NoError(t, err)

		assert.Empty(t, newSnapshot.Processes)

		resInformer.AssertExpectations(t)
	})

	t.Run("zero CPU time delta", func(t *testing.T) {
		prevSnapshot := NewSnapshot()
		newSnapshot := NewSnapshot()
		newSnapshot.Node = createNodeSnapshot(zones, fakeClock.Now(), 0.5)

		// Process with zero CPU time delta - this can happen when CPU time hasn't changed
		// or when NodeCPUTimeDelta is zero (no processes used CPU - theoretically possible)
		procs := &resource.Processes{
			Running: map[int]*resource.Process{
				123: {
					PID:          123,
					Comm:         "test-proc",
					Exe:          "/usr/bin/test-proc",
					CPUTotalTime: 10.0,
					CPUTimeDelta: 0.0, // NOTE: Zero delta
					Container:    nil,
				},
			},
			Terminated: map[int]*resource.Process{},
		}

		tr := CreateTestResources(createOnly(testNode))
		resInformer.On("Node").Return(tr.Node, nil).Maybe()
		resInformer.On("Processes").Return(procs).Once()

		err := monitor.calculateProcessPower(prevSnapshot, newSnapshot)
		require.NoError(t, err)

		// Process should exist but with zero power/energy
		proc := newSnapshot.Processes[123]
		assert.NotNil(t, proc)

		// All zones should have zero values due to division by zero protection
		for _, zone := range zones {
			usage := proc.Zones[zone]
			assert.Equal(t, 0*Watt, usage.Power)
			assert.Equal(t, 0*Joule, usage.EnergyTotal)
		}

		resInformer.AssertExpectations(t)
	})

	t.Run("new zone missing in previous snapshot", func(t *testing.T) {
		// Create snapshots where previous doesn't have zone data for a process
		prevSnapshot := NewSnapshot()
		now := fakeClock.Now()
		prevSnapshot.Node = createNodeSnapshot(zones, now, 0.5)

		// Add process without zone data for one zone
		prevSnapshot.Processes[123] = &Process{
			PID:   123,
			Comm:  "test-proc",
			Zones: make(ZoneUsageMap),
		}
		// Only add data for the first zone, missing the second
		prevSnapshot.Processes[123].Zones[zones[0]] = &Usage{
			EnergyTotal: 10 * Joule,
			Power:       5 * Watt,
		}

		newSnapshot := NewSnapshot()
		newSnapshot.Node = createNodeSnapshot(zones, now.Add(time.Second), 0.6)

		// Create process with all zones
		testProcesses := &resource.Processes{
			Running: map[int]*resource.Process{
				123: {
					PID:          123,
					Comm:         "test-proc",
					Exe:          "/usr/bin/test-proc",
					CPUTotalTime: 185.0,
					CPUTimeDelta: 50.0,
					Container:    nil,
				},
			},
			Terminated: map[int]*resource.Process{},
		}

		tr := CreateTestResources(createOnly(testNode))
		resInformer.On("Node").Return(tr.Node, nil).Maybe()
		resInformer.On("Processes").Return(testProcesses).Once()

		err := monitor.calculateProcessPower(prevSnapshot, newSnapshot)
		require.NoError(t, err)

		// Process should have power calculated for all zones
		proc := newSnapshot.Processes[123]
		assert.Len(t, proc.Zones, 2)

		// First zone should have accumulated absolute value
		usage0 := proc.Zones[zones[0]]

		// Second zone should have absolute = delta (new zone)
		usage1 := proc.Zones[zones[1]]

		nodeZones := newSnapshot.Node.Zones
		assert.Equal(t, 60.0, nodeZones[zones[0]].activeEnergy.Joules())
		assert.Equal(t, 60.0, nodeZones[zones[1]].activeEnergy.Joules())

		assert.Equal(t, 30.0, nodeZones[zones[0]].ActivePower.Watts())
		assert.Equal(t, 30.0, nodeZones[zones[1]].ActivePower.Watts())

		assert.Equal(t, 25.0, usage0.EnergyTotal.Joules()) // 10J + 15J
		assert.Equal(t, 15.0, usage1.EnergyTotal.Joules()) // 0J + 15J

		assert.Equal(t, 7.5, usage0.Power.Watts()) // 5W + 7.5W
		assert.Equal(t, 7.5, usage1.Power.Watts()) // 0W + 7.5W

		resInformer.AssertExpectations(t)
	})

	mockMeter.AssertExpectations(t)
}

func TestProcessPowerConsistency(t *testing.T) {
	// Test to ensure process power calculations follow conservation laws
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

	t.Run("power conservation across processes", func(t *testing.T) {
		// Test that sum of process powers equals node power
		prevSnapshot := NewSnapshot()
		newSnapshot := NewSnapshot()
		// Create processes with varying CPU usage
		testProcesses := &resource.Processes{
			Running: map[int]*resource.Process{
				1: {PID: 1, Comm: "proc1", Exe: "/bin/proc1", CPUTimeDelta: 25.0},
				2: {PID: 2, Comm: "proc2", Exe: "/bin/proc2", CPUTimeDelta: 35.0},
				3: {PID: 3, Comm: "proc3", Exe: "/bin/proc3", CPUTimeDelta: 40.0},
			},
			Terminated: map[int]*resource.Process{},
		}

		// Create node with 100% CPU usage and matching CPU time delta (25+35+40=100)
		tr := CreateTestResources(createOnly(testNode), withNodeCpuUsage(1.0), withNodeCpuTimeDelta(100.0))
		newSnapshot.Node = createNodeSnapshot(zones, fakeClock.Now(), 0.5)
		mockResourceInformer.On("Node").Return(tr.Node, nil).Maybe()
		mockResourceInformer.On("Processes").Return(testProcesses).Once()

		err := monitor.calculateProcessPower(prevSnapshot, newSnapshot)
		require.NoError(t, err)

		// Verify power conservation for each zone
		for _, zone := range zones {
			totalProcessPower := float64(0)

			for _, proc := range newSnapshot.Processes {
				totalProcessPower += proc.Zones[zone].Power.MicroWatts()
			}

			// Total process power should equal node ActivePower (with small tolerance for floating point)
			nodeActivePower := newSnapshot.Node.Zones[zone].ActivePower.MicroWatts()
			assert.InDelta(t, nodeActivePower, totalProcessPower, 1.0,
				"Power conservation failed for zone %s", zone.Name())
		}

		mockResourceInformer.AssertExpectations(t)
	})

	mockMeter.AssertExpectations(t)
}
