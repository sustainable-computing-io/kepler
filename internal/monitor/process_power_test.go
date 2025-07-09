// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package monitor

import (
	"fmt"
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
	mockMeter.On("PrimaryEnergyZone").Return(zones[0], nil)

	// Create mock resource informer
	resInformer := &MockResourceInformer{}

	// Create monitor with mocks
	monitor := &PowerMonitor{
		logger:                       logger,
		cpu:                          mockMeter,
		clock:                        fakeClock,
		resources:                    resInformer,
		maxTerminated:                500,
		minTerminatedEnergyThreshold: 1 * Joule, // Set threshold to filter zero-energy processes
	}

	err := monitor.Init()
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
		assert.Contains(t, snapshot.Processes, "123")
		assert.Contains(t, snapshot.Processes, "456")
		assert.Contains(t, snapshot.Processes, "789")

		// Check process 123 (in container-1)
		proc123 := snapshot.Processes["123"]
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
		proc789 := snapshot.Processes["789"]
		assert.Equal(t, "", proc789.ContainerID)

		resInformer.AssertExpectations(t)
	})

	t.Run("calculateProcessPower", func(t *testing.T) {
		// Setup previous snapshot with process data
		prevSnapshot := NewSnapshot()
		prevSnapshot.Node = createNodeSnapshot(zones, fakeClock.Now(), 0.5)

		// Add existing process data to previous snapshot
		prevSnapshot.Processes["123"] = &Process{
			PID:          123,
			Comm:         "process1",
			Exe:          "/usr/bin/process1",
			CPUTotalTime: 100.0,
			ContainerID:  "container-1",
			Zones:        make(ZoneUsageMap, len(zones)),
		}

		// Initialize zones for previous process
		for _, zone := range zones {
			prevSnapshot.Processes["123"].Zones[zone] = Usage{
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
		proc123 := newSnapshot.Processes["123"]
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
		proc456 := newSnapshot.Processes["456"]
		for _, zone := range zones {
			usage := proc456.Zones[zone]
			// CPU ratio = 20.0 / 100.0 = 0.2 (20%)
			// ActivePower = 50W * 0.5 = 25W, so 0.2 * 25W = 5W
			expectedPower := Power(0.2 * 25_000_000) // 5W
			assert.InDelta(t, expectedPower.MicroWatts(), usage.Power.MicroWatts(), 0.01)
		}

		// Check process 789 (not in container)
		proc789 := newSnapshot.Processes["789"]
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
			newSnapshot.Node.Zones[zone] = NodeUsage{
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
		proc := newSnapshot.Processes["123"]
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
		prevSnapshot.Processes["123"] = &Process{
			PID:   123,
			Comm:  "test-proc",
			Zones: make(ZoneUsageMap),
		}
		// Only add data for the first zone, missing the second
		prevSnapshot.Processes["123"].Zones[zones[0]] = Usage{
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
		proc := newSnapshot.Processes["123"]
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
	mockMeter.On("PrimaryEnergyZone").Return(zones[0], nil)

	// Create mock resource informer
	mockResourceInformer := &MockResourceInformer{}

	// Create monitor with mocks
	monitor := &PowerMonitor{
		logger:        logger,
		cpu:           mockMeter,
		clock:         fakeClock,
		resources:     mockResourceInformer,
		maxTerminated: 500,
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

func TestTerminatedProcessTracking(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	fakeClock := testingclock.NewFakeClock(time.Now())
	zones := CreateTestZones()

	t.Run("terminated process energy accumulation", func(t *testing.T) {
		mockMeter := &MockCPUPowerMeter{}
		mockMeter.On("Zones").Return(zones, nil)
		mockMeter.On("PrimaryEnergyZone").Return(zones[0], nil)
		resInformer := &MockResourceInformer{}

		monitor := &PowerMonitor{
			logger:        logger,
			cpu:           mockMeter,
			clock:         fakeClock,
			resources:     resInformer,
			maxTerminated: 500,
		}

		err := monitor.Init()
		require.NoError(t, err)
		// Step 1: Create initial snapshot with running processes
		snapshot1 := NewSnapshot()
		snapshot1.Node = createNodeSnapshot(zones, fakeClock.Now(), 0.5)

		// Create initial processes - all running
		procs1 := &resource.Processes{
			Running: map[int]*resource.Process{
				123: {PID: 123, Comm: "process1", Exe: "/usr/bin/process1", CPUTotalTime: 100.0, CPUTimeDelta: 30.0},
				456: {PID: 456, Comm: "process2", Exe: "/usr/bin/process2", CPUTotalTime: 150.0, CPUTimeDelta: 20.0},
			},
			Terminated: map[int]*resource.Process{},
		}

		tr1 := CreateTestResources(createOnly(testNode))
		resInformer.On("Node").Return(tr1.Node, nil).Maybe()
		resInformer.On("Processes").Return(procs1).Once()

		err = monitor.calculateProcessPower(NewSnapshot(), snapshot1)
		require.NoError(t, err)

		// Populate terminated resources from trackers (normally done by refreshSnapshot)
		snapshot1.TerminatedProcesses = monitor.terminatedProcessesTracker.Items()

		runningProc123 := snapshot1.Processes["123"]
		require.NotNil(t, runningProc123, "Process 123 should exist in running processes")

		// Store expected values for terminated process validation
		expectedEnergyValues := make(map[string]Energy)
		expectedPowerValues := make(map[string]Power)
		for zone, usage := range runningProc123.Zones {
			expectedEnergyValues[zone.Name()] = usage.EnergyTotal
			expectedPowerValues[zone.Name()] = usage.Power
		}

		// Step 2: Create second snapshot where process 123 terminates
		snapshot2 := NewSnapshot()
		snapshot2.Node = createNodeSnapshot(zones, fakeClock.Now().Add(time.Second), 0.5)

		// Process 123 is now terminated, process 456 still running
		procs2 := &resource.Processes{
			Running: map[int]*resource.Process{
				456: {PID: 456, Comm: "process2", Exe: "/usr/bin/process2", CPUTotalTime: 170.0, CPUTimeDelta: 20.0},
			},
			Terminated: map[int]*resource.Process{
				123: {PID: 123, Comm: "process1", Exe: "/usr/bin/process1", CPUTotalTime: 130.0, CPUTimeDelta: 30.0},
			},
		}

		tr2 := CreateTestResources(createOnly(testNode))
		resInformer.On("Node").Return(tr2.Node, nil).Maybe()
		resInformer.On("Processes").Return(procs2).Once()

		err = monitor.calculateProcessPower(snapshot1, snapshot2)
		require.NoError(t, err)

		// Populate terminated resources from trackers (normally done by refreshSnapshot)
		snapshot2.TerminatedProcesses = monitor.terminatedProcessesTracker.Items()

		// Step 3: Validate running processes
		assert.Len(t, snapshot2.Processes, 1)
		assert.Contains(t, snapshot2.Processes, "456")
		assert.NotContains(t, snapshot2.Processes, "123", "Process 123 should no longer be in running processes")

		// Step 4: Validate terminated processes - CORE BUSINESS LOGIC
		assert.Len(t, snapshot2.TerminatedProcesses, 1, "Terminated processes should be tracked")

		// Find terminated process by PID
		var terminatedProc123 *Process
		for _, proc := range snapshot2.TerminatedProcesses {
			if proc.PID == 123 {
				terminatedProc123 = proc
				break
			}
		}
		require.NotNil(t, terminatedProc123, "Process 123 should exist in terminated processes")

		// TEST: Energy and power values are EXACTLY preserved from when process was running
		for _, zone := range zones {
			terminatedUsage := terminatedProc123.Zones[zone]
			zoneName := zone.Name()

			// The terminated process should have EXACTLY the same energy as when it was last running
			assert.Equal(t, expectedEnergyValues[zoneName], terminatedUsage.EnergyTotal,
				"Terminated process energy should be preserved from last running state for zone %s", zoneName)

			// The terminated process should have EXACTLY the same power as when it was last running
			assert.Equal(t, expectedPowerValues[zoneName], terminatedUsage.Power,
				"Terminated process power should be preserved from last running state for zone %s", zoneName)
		}

		// Step 5: Validate process metadata is preserved
		assert.Equal(t, runningProc123.PID, terminatedProc123.PID)
		assert.Equal(t, runningProc123.Comm, terminatedProc123.Comm)
		assert.Equal(t, runningProc123.Exe, terminatedProc123.Exe)
		assert.Equal(t, runningProc123.ContainerID, terminatedProc123.ContainerID)

		resInformer.AssertExpectations(t)
	})

	t.Run("terminated process cleanup after export", func(t *testing.T) {
		mockMeter := &MockCPUPowerMeter{}
		mockMeter.On("Zones").Return(zones, nil)
		mockMeter.On("PrimaryEnergyZone").Return(zones[0], nil)
		resInformer := &MockResourceInformer{}

		monitor := &PowerMonitor{
			logger:        logger,
			cpu:           mockMeter,
			clock:         fakeClock,
			resources:     resInformer,
			maxTerminated: 500,
		}

		err := monitor.Init()
		require.NoError(t, err)

		// Reset monitor state
		monitor.exported.Store(false)

		// Create snapshot with terminated process
		snapshot1 := NewSnapshot()
		snapshot1.Node = createNodeSnapshot(zones, fakeClock.Now(), 0.5)

		// Add a terminated process from a previous calculation
		oldTerminated := &Process{
			PID:   999,
			Comm:  "old-terminated",
			Zones: make(ZoneUsageMap),
		}
		for _, zone := range zones {
			oldTerminated.Zones[zone] = Usage{
				EnergyTotal: 50 * Joule,
				Power:       10 * Watt,
			}
		}
		snapshot1.TerminatedProcesses[oldTerminated.StringID()] = oldTerminated

		// Also add to the internal tracker (simulating previous processing)
		monitor.terminatedProcessesTracker.Add(oldTerminated)

		// Create next snapshot with no new terminated processes
		snapshot2 := NewSnapshot()
		snapshot2.Node = createNodeSnapshot(zones, fakeClock.Now().Add(time.Second), 0.5)

		procs := &resource.Processes{
			Running: map[int]*resource.Process{
				100: {PID: 100, Comm: "running", CPUTimeDelta: 10.0},
			},
			Terminated: map[int]*resource.Process{}, // No new terminated processes
		}

		tr := CreateTestResources(createOnly(testNode))
		resInformer.On("Node").Return(tr.Node, nil).Maybe()
		resInformer.On("Processes").Return(procs).Once()

		// Before export terminated processes should be preserved
		monitor.exported.Store(false)
		err = monitor.calculateProcessPower(snapshot1, snapshot2)
		require.NoError(t, err)

		// Populate terminated resources from trackers (normally done by refreshSnapshot)
		snapshot2.TerminatedProcesses = monitor.terminatedProcessesTracker.Items()

		assert.Len(t, snapshot2.TerminatedProcesses, 1, "Terminated processes should be preserved before export")
		// Verify old-terminated process is still present
		found := false
		for _, proc := range snapshot2.TerminatedProcesses {
			if proc.PID == 999 {
				found = true
				break
			}
		}
		assert.True(t, found, "old-terminated process should be present")

		// After export, terminated processes should be cleaned up
		monitor.exported.Store(true)
		snapshot3 := NewSnapshot()
		snapshot3.Node = createNodeSnapshot(zones, fakeClock.Now().Add(2*time.Second), 0.5)

		resInformer.On("Node").Return(tr.Node, nil).Maybe()
		resInformer.On("Processes").Return(procs).Once()

		err = monitor.calculateProcessPower(snapshot2, snapshot3)
		require.NoError(t, err)

		// Populate terminated resources from trackers (normally done by refreshSnapshot)
		snapshot3.TerminatedProcesses = monitor.terminatedProcessesTracker.Items()

		assert.Len(t, snapshot3.TerminatedProcesses, 0, "Terminated processes should be cleaned up after export")

		resInformer.AssertExpectations(t)
	})

	t.Run("multiple terminated processes accumulation", func(t *testing.T) {
		mockMeter := &MockCPUPowerMeter{}
		mockMeter.On("Zones").Return(zones, nil)
		mockMeter.On("PrimaryEnergyZone").Return(zones[0], nil)
		resInformer := &MockResourceInformer{}

		monitor := &PowerMonitor{
			logger:        logger,
			cpu:           mockMeter,
			clock:         fakeClock,
			resources:     resInformer,
			maxTerminated: 500,
		}

		err := monitor.Init()
		require.NoError(t, err)

		// Reset monitor state
		monitor.exported.Store(false)

		// Create initial snapshot with multiple running processes
		snapshot1 := NewSnapshot()
		snapshot1.Node = createNodeSnapshot(zones, fakeClock.Now(), 0.5)

		procs1 := &resource.Processes{
			Running: map[int]*resource.Process{
				100: {PID: 100, Comm: "proc1", CPUTimeDelta: 10.0},
				200: {PID: 200, Comm: "proc2", CPUTimeDelta: 20.0},
				300: {PID: 300, Comm: "proc3", CPUTimeDelta: 30.0},
			},
			Terminated: map[int]*resource.Process{},
		}

		tr1 := CreateTestResources(createOnly(testNode))
		resInformer.On("Node").Return(tr1.Node, nil).Maybe()
		resInformer.On("Processes").Return(procs1).Once()

		err = monitor.calculateProcessPower(NewSnapshot(), snapshot1)
		require.NoError(t, err)

		// Populate terminated resources from trackers (normally done by refreshSnapshot)
		snapshot1.TerminatedProcesses = monitor.terminatedProcessesTracker.Items()

		// Capture running process values
		runningValues := make(map[string]map[string]Usage)
		for pid, proc := range snapshot1.Processes {
			runningValues[pid] = make(map[string]Usage)
			for zone, usage := range proc.Zones {
				runningValues[pid][zone.Name()] = usage
			}
		}

		// Create second snapshot where processes 100 and 200 terminate
		snapshot2 := NewSnapshot()
		snapshot2.Node = createNodeSnapshot(zones, fakeClock.Now().Add(time.Second), 0.5)

		procs2 := &resource.Processes{
			Running: map[int]*resource.Process{
				300: {PID: 300, Comm: "proc3", CPUTimeDelta: 35.0}, // Still running
			},
			Terminated: map[int]*resource.Process{
				100: {PID: 100, Comm: "proc1", CPUTimeDelta: 10.0},
				200: {PID: 200, Comm: "proc2", CPUTimeDelta: 20.0},
			},
		}

		tr2 := CreateTestResources(createOnly(testNode))
		resInformer.On("Node").Return(tr2.Node, nil).Maybe()
		resInformer.On("Processes").Return(procs2).Once()

		err = monitor.calculateProcessPower(snapshot1, snapshot2)
		require.NoError(t, err)

		// Populate terminated resources from trackers (normally done by refreshSnapshot)
		snapshot2.TerminatedProcesses = monitor.terminatedProcessesTracker.Items()

		// Validate multiple terminated processes
		assert.Len(t, snapshot2.TerminatedProcesses, 2, "Both terminated processes should be tracked")
		// Check that both terminated processes are present
		processPIDs := make(map[int]bool)
		for _, proc := range snapshot2.TerminatedProcesses {
			processPIDs[proc.PID] = true
		}
		assert.True(t, processPIDs[100], "Process 100 should be in terminated processes")
		assert.True(t, processPIDs[200], "Process 200 should be in terminated processes")
		assert.Len(t, snapshot2.Processes, 1, "Only one process should still be running")
		assert.Contains(t, snapshot2.Processes, "300")

		// Validate each terminated process preserves exact values
		for _, pid := range []int{100, 200} {
			var terminatedProc *Process
			for _, proc := range snapshot2.TerminatedProcesses {
				if proc.PID == pid {
					terminatedProc = proc
					break
				}
			}
			require.NotNil(t, terminatedProc, "Terminated process %d should exist", pid)

			for _, zone := range zones {
				terminatedUsage := terminatedProc.Zones[zone]
				expectedUsage := runningValues[fmt.Sprintf("%d", pid)][zone.Name()]

				assert.Equal(t, expectedUsage.EnergyTotal, terminatedUsage.EnergyTotal,
					"Process %d zone %s energy should be preserved", pid, zone.Name())
				assert.Equal(t, expectedUsage.Power, terminatedUsage.Power,
					"Process %d zone %s power should be preserved", pid, zone.Name())
			}
		}

		resInformer.AssertExpectations(t)
	})

	t.Run("terminated process with zero energy filtering", func(t *testing.T) {
		mockMeter := &MockCPUPowerMeter{}
		mockMeter.On("Zones").Return(zones, nil)
		mockMeter.On("PrimaryEnergyZone").Return(zones[0], nil)
		resInformer := &MockResourceInformer{}

		monitor := &PowerMonitor{
			logger:                       logger,
			cpu:                          mockMeter,
			clock:                        fakeClock,
			resources:                    resInformer,
			maxTerminated:                500,
			minTerminatedEnergyThreshold: 1 * Joule, // Set threshold to filter zero-energy processes
		}

		err := monitor.Init()
		require.NoError(t, err)

		// NOTE: Reset monitor state
		monitor.exported.Store(false)

		// Step 1: Create initial snapshot and populate it using calculateProcessPower
		snapshot1 := NewSnapshot()
		snapshot1.Node = createNodeSnapshot(zones, fakeClock.Now(), 0.5)

		// Create initial processes - all running
		procs1 := &resource.Processes{
			Running: map[int]*resource.Process{
				100: {PID: 100, Comm: "active-process", Exe: "/usr/bin/active", CPUTotalTime: 100.0, CPUTimeDelta: 30.0},
				200: {PID: 200, Comm: "idle-process", Exe: "/usr/bin/idle", CPUTotalTime: 50.0, CPUTimeDelta: 0.0}, // Zero CPU delta = zero energy
			},
			Terminated: map[int]*resource.Process{},
		}

		tr1 := CreateTestResources(createOnly(testNode))
		resInformer.On("Node").Return(tr1.Node, nil).Maybe()
		resInformer.On("Processes").Return(procs1).Once()

		err = monitor.calculateProcessPower(NewSnapshot(), snapshot1)
		require.NoError(t, err)

		// Populate terminated resources from trackers (normally done by refreshSnapshot)
		snapshot1.TerminatedProcesses = monitor.terminatedProcessesTracker.Items()

		// Verify initial state: process 100 has energy, process 200 has zero energy
		require.Contains(t, snapshot1.Processes, "100")
		require.Contains(t, snapshot1.Processes, "200")

		process100 := snapshot1.Processes["100"]
		process200 := snapshot1.Processes["200"]

		// Process 100 should have energy due to CPU usage
		// Check that process 100 has energy in at least one zone
		hasEnergy := false
		for _, usage := range process100.Zones {
			if usage.EnergyTotal >= 1*Joule {
				hasEnergy = true
				break
			}
		}
		assert.True(t, hasEnergy, "Process 100 should have significant energy")

		// Process 200 should have zero energy due to zero CPU delta
		// Check that process 200 has minimal energy in all zones
		hasMinimalEnergy := true
		for _, usage := range process200.Zones {
			if usage.EnergyTotal >= 1*Joule {
				hasMinimalEnergy = false
				break
			}
		}
		assert.True(t, hasMinimalEnergy, "Process 200 should have minimal energy")

		// Step 2: Create snapshot where both processes terminate
		snapshot2 := NewSnapshot()
		snapshot2.Node = createNodeSnapshot(zones, fakeClock.Now().Add(time.Second), 0.5)

		// Both processes are now terminated
		procs2 := &resource.Processes{
			Running: map[int]*resource.Process{}, // No running processes
			Terminated: map[int]*resource.Process{
				100: {PID: 100, Comm: "active-process", Exe: "/usr/bin/active"},
				200: {PID: 200, Comm: "idle-process", Exe: "/usr/bin/idle"},
			},
		}

		tr2 := CreateTestResources(createOnly(testNode))
		resInformer.On("Node").Return(tr2.Node, nil).Maybe()
		resInformer.On("Processes").Return(procs2).Once()

		err = monitor.calculateProcessPower(snapshot1, snapshot2)
		require.NoError(t, err)

		// Populate terminated resources from trackers (normally done by refreshSnapshot)
		snapshot2.TerminatedProcesses = monitor.terminatedProcessesTracker.Items()

		// TEST:  Only process with non-zero energy should be in terminated processes
		assert.Len(t, snapshot2.TerminatedProcesses, 1, "Only process with non-zero energy should be in terminated processes")
		// Check that process 100 is in terminated processes but not process 200
		foundProc100 := false
		foundProc200 := false
		for _, proc := range snapshot2.TerminatedProcesses {
			if proc.PID == 100 {
				foundProc100 = true
			}
			if proc.PID == 200 {
				foundProc200 = true
			}
		}
		assert.True(t, foundProc100, "Process 100 (with energy) should be in terminated processes")
		assert.False(t, foundProc200, "Process 200 (zero energy) should be filtered out")

		// TEST: verify the included process has correct energy values
		var terminatedProc100 *Process
		for _, proc := range snapshot2.TerminatedProcesses {
			if proc.PID == 100 {
				terminatedProc100 = proc
				break
			}
		}
		require.NotNil(t, terminatedProc100)

		for _, zone := range zones {
			usage := terminatedProc100.Zones[zone]
			assert.Greater(t, usage.EnergyTotal.Joules(), 0.0, "Process 100 should have non-zero energy")
		}

		resInformer.AssertExpectations(t)
	})
}
