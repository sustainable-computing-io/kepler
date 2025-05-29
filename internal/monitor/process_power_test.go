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

	t.Run("firstProcessRead", func(t *testing.T) {
		// Setup mock to return test processes
		procs := CreateTestResources().Processes
		resourceInformer.On("Processes").Return(procs).Once()

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
			assert.Equal(t, Energy(0), usage.Absolute)
			assert.Equal(t, Energy(0), usage.Delta)
			assert.Equal(t, Power(0), usage.Power)
		}

		// Check process 789 (not in container)
		proc789 := snapshot.Processes[789]
		assert.Equal(t, "", proc789.ContainerID)

		resourceInformer.AssertExpectations(t)
	})

	t.Run("calculateProcessPower", func(t *testing.T) {
		// Setup previous snapshot with process data
		prevSnapshot := NewSnapshot()
		prevSnapshot.Node = createNodeSnapshot(zones, fakeClock.Now())

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
				Absolute: 25 * Joule,
				Delta:    Energy(0),
				Power:    Power(0),
			}
		}

		// Create new snapshot with updated node data
		newSnapshot := NewSnapshot()
		newSnapshot.Node = createNodeSnapshot(zones, fakeClock.Now().Add(time.Second))

		// Setup mock to return updated processes
		procs := CreateTestResources().Processes
		resourceInformer.On("Processes").Return(procs).Once()

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
			// Expected power = 0.3 * 50W = 15W
			expectedPower := 0.3 * 50 * Watt // 15W in microwatts
			assert.InDelta(t, expectedPower.MicroWatts(), usage.Power.MicroWatts(), 0.01)

			// Expected delta = 0.3 * 50J = 15J
			expectedDelta := 0.3 * 50 * Joule // 15J in microjoules
			assert.InDelta(t, expectedDelta.MicroJoules(), usage.Delta.MicroJoules(), 0.01)

			// Absolute should be previous + delta = 25J + 15J = 40J
			expectedAbsolute := 40 * Joule
			assert.InDelta(t, expectedAbsolute.MicroJoules(), usage.Absolute.MicroJoules(), 0.01)
		}

		// Check process 456 (new process)
		proc456 := newSnapshot.Processes[456]
		for _, zone := range zones {
			usage := proc456.Zones[zone]
			// CPU ratio = 20.0 / 100.0 = 0.5 (20%)
			expectedPower := Power(0.2 * 50_000_000) // 25W
			assert.InDelta(t, expectedPower.MicroWatts(), usage.Power.MicroWatts(), 0.01)

			expectedDelta := Energy(0.2 * 50_000_000) // 25J
			assert.InDelta(t, expectedDelta.MicroJoules(), usage.Delta.MicroJoules(), 0.01)
			// New process, so absolute = delta
			assert.Equal(t, usage.Delta, usage.Absolute)
		}

		// Check process 789 (not in container)
		proc789 := newSnapshot.Processes[789]
		for _, zone := range zones {
			usage := proc789.Zones[zone]
			// CPU ratio = 15.0 / 100.0 = 0.15 (15%)
			expectedPower := Power(0.15 * 50_000_000) // 10W
			assert.InDelta(t, expectedPower.MicroWatts(), usage.Power.MicroWatts(), 0.01)
		}

		resourceInformer.AssertExpectations(t)
	})

	t.Run("calculateProcessPower with zero node power", func(t *testing.T) {
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

		procs := CreateTestResources().Processes
		resourceInformer.On("Processes").Return(procs).Once()

		err := monitor.calculateProcessPower(prevSnapshot, newSnapshot)
		require.NoError(t, err)

		// All processes should have zero power
		for _, proc := range newSnapshot.Processes {
			for _, zone := range zones {
				usage := proc.Zones[zone]
				assert.Equal(t, Power(0), usage.Power)
				assert.Equal(t, Energy(0), usage.Delta)
				assert.Equal(t, Energy(0), usage.Absolute)
			}
		}

		resourceInformer.AssertExpectations(t)
	})

	t.Run("calculateProcessPower with no processes", func(t *testing.T) {
		prevSnapshot := NewSnapshot()
		newSnapshot := NewSnapshot()
		newSnapshot.Node = createNodeSnapshot(zones, fakeClock.Now())

		// Return empty processes
		emptyProcesses := &resource.Processes{
			NodeCPUTimeDelta: 0.0,
			Running:          map[int]*resource.Process{},
			Terminated:       map[int]*resource.Process{},
		}
		resourceInformer.On("Processes").Return(emptyProcesses).Once()

		err := monitor.calculateProcessPower(prevSnapshot, newSnapshot)
		require.NoError(t, err)

		assert.Empty(t, newSnapshot.Processes)

		resourceInformer.AssertExpectations(t)
	})

	t.Run("zero CPU time delta", func(t *testing.T) {
		prevSnapshot := NewSnapshot()
		newSnapshot := NewSnapshot()
		newSnapshot.Node = createNodeSnapshot(zones, fakeClock.Now())

		// Process with zero CPU time delta - this can happen when CPU time hasn't changed
		// or when NodeCPUTimeDelta is zero (no processes used CPU - theoretically possible)
		procs := &resource.Processes{
			NodeCPUTimeDelta: 0.0, // Zero total CPU time delta
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

		resourceInformer.On("Processes").Return(procs).Once()

		err := monitor.calculateProcessPower(prevSnapshot, newSnapshot)
		require.NoError(t, err)

		// Process should exist but with zero power/energy
		proc := newSnapshot.Processes[123]
		assert.NotNil(t, proc)

		// All zones should have zero values due to division by zero protection
		for _, zone := range zones {
			usage := proc.Zones[zone]
			assert.Equal(t, 0*Watt, usage.Power)
			assert.Equal(t, 0*Joule, usage.Delta)
			assert.Equal(t, 0*Joule, usage.Absolute)
		}

		resourceInformer.AssertExpectations(t)
	})

	t.Run("process missing in previous snapshot", func(t *testing.T) {
		// Create snapshots where previous doesn't have zone data for a process
		prevSnapshot := NewSnapshot()
		prevSnapshot.Node = createNodeSnapshot(zones, fakeClock.Now())

		// Add process without zone data for one zone
		prevSnapshot.Processes[123] = &Process{
			PID:   123,
			Comm:  "test-proc",
			Zones: make(ZoneUsageMap),
		}
		// Only add data for the first zone, missing the second
		prevSnapshot.Processes[123].Zones[zones[0]] = &Usage{
			Absolute: Energy(10_000_000),
			Delta:    Energy(0),
			Power:    Power(0),
		}

		newSnapshot := NewSnapshot()
		newSnapshot.Node = createNodeSnapshot(zones, fakeClock.Now().Add(time.Second))

		// Create process with all zones
		testProcesses := &resource.Processes{
			NodeCPUTimeDelta: 100.0,
			Running: map[int]*resource.Process{
				123: {
					PID:          123,
					Comm:         "test-proc",
					Exe:          "/usr/bin/test-proc",
					CPUTotalTime: 15.0,
					CPUTimeDelta: 50.0,
					Container:    nil,
				},
			},
			Terminated: map[int]*resource.Process{},
		}

		resourceInformer.On("Processes").Return(testProcesses).Once()

		err := monitor.calculateProcessPower(prevSnapshot, newSnapshot)
		require.NoError(t, err)

		// Process should have power calculated for all zones
		proc := newSnapshot.Processes[123]
		assert.Len(t, proc.Zones, 2)

		// First zone should have accumulated absolute value
		usage0 := proc.Zones[zones[0]]
		assert.Greater(t, usage0.Absolute.MicroJoules(), usage0.Delta.MicroJoules())

		// Second zone should have absolute = delta (new zone)
		usage1 := proc.Zones[zones[1]]
		assert.Equal(t, usage1.Absolute, usage1.Delta)

		resourceInformer.AssertExpectations(t)
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
		newSnapshot.Node = createNodeSnapshot(zones, fakeClock.Now())

		// Create processes with varying CPU usage
		testProcesses := &resource.Processes{
			NodeCPUTimeDelta: 100.0,
			Running: map[int]*resource.Process{
				1: {PID: 1, Comm: "proc1", Exe: "/bin/proc1", CPUTimeDelta: 25.0},
				2: {PID: 2, Comm: "proc2", Exe: "/bin/proc2", CPUTimeDelta: 35.0},
				3: {PID: 3, Comm: "proc3", Exe: "/bin/proc3", CPUTimeDelta: 40.0},
			},
			Terminated: map[int]*resource.Process{},
		}

		mockResourceInformer.On("Processes").Return(testProcesses).Once()

		err := monitor.calculateProcessPower(prevSnapshot, newSnapshot)
		require.NoError(t, err)

		// Verify power conservation for each zone
		for _, zone := range zones {
			nodePower := newSnapshot.Node.Zones[zone].Power.MicroWatts()
			totalProcessPower := float64(0)

			for _, proc := range newSnapshot.Processes {
				totalProcessPower += proc.Zones[zone].Power.MicroWatts()
			}

			// Total process power should equal node power (with small tolerance for floating point)
			assert.InDelta(t, nodePower, totalProcessPower, 1.0,
				"Power conservation failed for zone %s", zone.Name())
		}

		mockResourceInformer.AssertExpectations(t)
	})

	mockMeter.AssertExpectations(t)
}
