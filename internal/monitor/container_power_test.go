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

func TestContainerPowerCalculation(t *testing.T) {
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
		logger:    logger,
		cpu:       mockMeter,
		clock:     fakeClock,
		resources: resInformer,
	}

	err := monitor.Init()
	require.NoError(t, err)

	t.Run("firstContainerRead", func(t *testing.T) {
		tr := CreateTestResources(createOnly(testContainers, testNode))
		require.NotNil(t, tr.Containers)

		resInformer.SetExpectations(t, tr)

		snapshot := NewSnapshot()
		err := monitor.firstNodeRead(snapshot.Node)
		require.NoError(t, err)

		err = monitor.firstContainerRead(snapshot)
		require.NoError(t, err)

		// Verify processes were initialized
		containers := tr.Containers
		assert.Len(t, snapshot.Containers, len(containers.Running))
		assert.Contains(t, snapshot.Containers, "container-1")
		assert.Contains(t, snapshot.Containers, "container-2")

		// Check process 123 (in container-1)
		cntr1 := snapshot.Containers["container-1"]
		assert.Equal(t, "container-1", cntr1.ID)
		assert.Equal(t, "test-container-1", cntr1.Name)
		assert.Equal(t, resource.DockerRuntime, cntr1.Runtime)
		assert.Equal(t, 200.0, cntr1.CPUTotalTime)

		// Verify zones are initialized with zero values
		assert.Len(t, cntr1.Zones, 2)
		for _, zone := range zones {
			usage := cntr1.Zones[zone]
			assert.Equal(t, Energy(0), usage.EnergyTotal)
			assert.Equal(t, Power(0), usage.Power)
		}

		resInformer.AssertExpectations(t)
	})

	t.Run("calculateContainerPower", func(t *testing.T) {
		// Setup previous snapshot with process data
		prevSnapshot := NewSnapshot()
		prevSnapshot.Node = createNodeSnapshot(zones, fakeClock.Now(), 0.5)

		// Add existing process data to previous snapshot
		prevSnapshot.Containers["container-1"] = &Container{
			ID:           "container-1",
			Name:         "test-container-1",
			Runtime:      resource.DockerRuntime,
			CPUTotalTime: 5.0,
			Zones:        make(ZoneUsageMap, len(zones)),
		}

		// Initialize zones for previous process
		for _, zone := range zones {
			prevSnapshot.Containers["container-1"].Zones[zone] = Usage{
				EnergyTotal: 25 * Joule,
				Power:       Power(0),
			}
		}

		// Create new snapshot with updated node data
		newSnapshot := NewSnapshot()
		newSnapshot.Node = createNodeSnapshot(zones, fakeClock.Now().Add(time.Second), 0.5)

		// Setup mock to return updated processes
		tr := CreateTestResources()
		require.NotNil(t, tr.Node)
		procs, containers := tr.Processes, tr.Containers
		resInformer.On("Node").Return(tr.Node, nil)
		resInformer.On("Containers").Return(containers)

		err = monitor.calculateContainerPower(prevSnapshot, newSnapshot)
		require.NoError(t, err)

		// Verify all processes are present
		assert.Len(t, newSnapshot.Containers, 2)

		// Check container-1 (which runs proc 123, 1231) power calculations
		inputProc123 := procs.Running[123]
		inputProc1231 := procs.Running[1231]
		ctnr1 := newSnapshot.Containers["container-1"]
		assert.Equal(t, inputProc123.CPUTotalTime+inputProc1231.CPUTotalTime, ctnr1.CPUTotalTime) // Updated CPU time

		for _, zone := range zones {
			usage := ctnr1.Zones[zone]

			// CPU ratio = 40.0 / 100.0 = 0.4 (40%)
			// ActivePower = 50W * 0.5 = 25W (node usage ratio is 0.5)
			// Expected power = 0.4 * 25W = 10W
			expectedPower := 0.4 * 25 * Watt // 10W in microwatts
			assert.InDelta(t, expectedPower.MicroWatts(), usage.Power.MicroWatts(), 0.01)

			// Absolute should be previous + delta = 25J + 20J = 45J
			expectedAbsolute := 45 * Joule
			assert.InDelta(t, expectedAbsolute.MicroJoules(), usage.EnergyTotal.MicroJoules(), 0.01)
		}

		// Check process 456 (new process)
		ctnr2 := newSnapshot.Containers["container-2"]
		for _, zone := range zones {
			usage := ctnr2.Zones[zone]
			// CPU ratio = 20.0 / 100.0 = 0.2 (20%)
			// ActivePower = 50W * 0.5 = 25W, so 0.2 * 25W = 5W
			expectedPower := 0.2 * 25 * Watt // 5W
			assert.InDelta(t, expectedPower.MicroWatts(), usage.Power.MicroWatts(), 0.01)
		}

		resInformer.AssertExpectations(t)
	})

	t.Run("calculateContainerPower with zero node power", func(t *testing.T) {
		// Create node with zero power
		prevSnapshot := NewSnapshot()
		newSnapshot := NewSnapshot()
		newSnapshot.Node = &Node{
			Timestamp:  fakeClock.Now(),
			UsageRatio: 0.5, // ratio of usage
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

		tr := CreateTestResources(createOnly(testNode, testContainers))
		resInformer.SetExpectations(t, tr)

		err := monitor.calculateContainerPower(prevSnapshot, newSnapshot)
		require.NoError(t, err)

		// All containers should have zero power
		for _, proc := range newSnapshot.Containers {
			for _, zone := range zones {
				usage := proc.Zones[zone]
				assert.Equal(t, Power(0), usage.Power)
				assert.Equal(t, Energy(0), usage.EnergyTotal)
			}
		}

		resInformer.AssertExpectations(t)
	})

	t.Run("calculateContainerPower without containers", func(t *testing.T) {
		prevSnapshot := NewSnapshot()
		newSnapshot := NewSnapshot()
		newSnapshot.Node = createNodeSnapshot(zones, fakeClock.Now(), 0.5)

		// Return empty processes
		emptyContaineres := &resource.Containers{
			Running:    map[string]*resource.Container{},
			Terminated: map[string]*resource.Container{},
		}
		tr := CreateTestResources(createOnly(testNode))
		resInformer.On("Node").Return(tr.Node, nil).Maybe()
		resInformer.On("Containers").Return(emptyContaineres).Once()

		err := monitor.calculateContainerPower(prevSnapshot, newSnapshot)
		require.NoError(t, err)

		assert.Empty(t, newSnapshot.Containers)

		resInformer.AssertExpectations(t)
	})
	mockMeter.AssertExpectations(t)
}

func TestContainerPowerConsistency(t *testing.T) {
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

	t.Run("power conservation across processes", func(t *testing.T) {
		// Test that sum of process powers equals node power assuming
		// no processes are running outside of containers
		prevSnapshot := NewSnapshot()
		newSnapshot := NewSnapshot()
		newSnapshot.Node = createNodeSnapshot(zones, fakeClock.Now(), 0.5)

		// Create containers with varying CPU usage that adds up to node's CPU usage
		// Node has CPUTimeDelta: 200.0, so containers should sum to 100.0 (50% of 200.0 = node usage ratio * node delta)
		testContaineres := &resource.Containers{
			Running: map[string]*resource.Container{
				"c1": {ID: "c1", Name: "container-1", Runtime: resource.PodmanRuntime, CPUTimeDelta: 30.0},
				"c2": {ID: "c2", Name: "container-2", Runtime: resource.PodmanRuntime, CPUTimeDelta: 35.0},
				"c3": {ID: "c3", Name: "container-3", Runtime: resource.PodmanRuntime, CPUTimeDelta: 35.0},
			},
			Terminated: map[string]*resource.Container{},
		}

		tr := CreateTestResources()
		mockResourceInformer.On("Node").Return(tr.Node, nil).Once()
		mockResourceInformer.On("Containers").Return(testContaineres).Once()

		err := monitor.calculateContainerPower(prevSnapshot, newSnapshot)
		require.NoError(t, err)

		// Verify power conservation for each zone
		for _, zone := range zones {
			ctnrZonePowerTotal := Power(0)

			for _, ctnr := range newSnapshot.Containers {
				ctnrZonePowerTotal += ctnr.Zones[zone].Power
			}

			// Containers have total CPU delta of 100.0 out of node's 200.0 (50%)
			// So total container power should be 50% of node ActivePower
			nodeActivePower := newSnapshot.Node.Zones[zone].ActivePower
			expectedContainerPower := nodeActivePower * 0.5 // 50% of used power
			assert.Equal(t, expectedContainerPower, ctnrZonePowerTotal,
				"Power conservation failed for zone %s", zone.Name())
		}

		mockResourceInformer.AssertExpectations(t)
	})

	mockMeter.AssertExpectations(t)
}

func TestTerminatedContainerTracking(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	fakeClock := testingclock.NewFakeClock(time.Now())
	zones := CreateTestZones()

	t.Run("terminated container energy accumulation", func(t *testing.T) {
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
		// Step 1: Create initial snapshot with running containers
		// We need a previous snapshot with some energy to have proper power attribution
		prevSnapshot := NewSnapshot()
		prevSnapshot.Node = createNodeSnapshot(zones, fakeClock.Now().Add(-time.Second), 0.5)

		// Add existing containers with some energy
		prevSnapshot.Containers["container-1"] = &Container{
			ID:           "container-1",
			Name:         "test-container-1",
			Runtime:      resource.DockerRuntime,
			CPUTotalTime: 70.0, // Previous CPU time
			Zones:        make(ZoneUsageMap, len(zones)),
		}
		prevSnapshot.Containers["container-2"] = &Container{
			ID:           "container-2",
			Name:         "test-container-2",
			Runtime:      resource.PodmanRuntime,
			CPUTotalTime: 130.0, // Previous CPU time
			Zones:        make(ZoneUsageMap, len(zones)),
		}
		// Initialize previous energy values
		for _, zone := range zones {
			prevSnapshot.Containers["container-1"].Zones[zone] = Usage{
				EnergyTotal: 15 * Joule,
				Power:       Power(0),
			}
			prevSnapshot.Containers["container-2"].Zones[zone] = Usage{
				EnergyTotal: 10 * Joule,
				Power:       Power(0),
			}
		}

		snapshot1 := NewSnapshot()
		snapshot1.Node = createNodeSnapshot(zones, fakeClock.Now(), 0.5)

		// Create initial containers - all running
		cntrsInitial := &resource.Containers{
			Running: map[string]*resource.Container{
				"container-1": {ID: "container-1", Name: "test-container-1", Runtime: resource.DockerRuntime, CPUTotalTime: 100.0, CPUTimeDelta: 30.0},
				"container-2": {ID: "container-2", Name: "test-container-2", Runtime: resource.PodmanRuntime, CPUTotalTime: 150.0, CPUTimeDelta: 20.0},
			},
			Terminated: map[string]*resource.Container{},
		}

		tr1 := CreateTestResources(createOnly(testNode))
		resInformer.On("Node").Return(tr1.Node, nil).Maybe()
		resInformer.On("Containers").Return(cntrsInitial).Once()

		err = monitor.calculateContainerPower(prevSnapshot, snapshot1)
		require.NoError(t, err)

		// Populate terminated resources from trackers (normally done by refreshSnapshot)
		snapshot1.TerminatedContainers = monitor.terminatedContainersTracker.Items()

		runningCtnr1 := snapshot1.Containers["container-1"]
		require.NotNil(t, runningCtnr1, "Container container-1 should exist in running containers")

		// Store expected values for terminated container validation
		expectedEnergy := make(map[string]Energy)
		expectedPower := make(map[string]Power)
		for zone, usage := range runningCtnr1.Zones {
			expectedEnergy[zone.Name()] = usage.EnergyTotal
			expectedPower[zone.Name()] = usage.Power
		}

		// Step 2: Create second snapshot where container-1 terminates
		snapshot2 := NewSnapshot()
		snapshot2.Node = createNodeSnapshot(zones, fakeClock.Now().Add(time.Second), 0.5)

		// container-1 is now terminated, container-2 still running
		containers2 := &resource.Containers{
			Running: map[string]*resource.Container{
				"container-2": {ID: "container-2", Name: "test-container-2", Runtime: resource.PodmanRuntime, CPUTotalTime: 170.0, CPUTimeDelta: 20.0},
			},
			Terminated: map[string]*resource.Container{
				"container-1": {ID: "container-1", Name: "test-container-1", Runtime: resource.DockerRuntime, CPUTotalTime: 130.0, CPUTimeDelta: 30.0},
			},
		}

		tr2 := CreateTestResources(createOnly(testNode))
		resInformer.On("Node").Return(tr2.Node, nil).Maybe()
		resInformer.On("Containers").Return(containers2).Once()

		err = monitor.calculateContainerPower(snapshot1, snapshot2)
		require.NoError(t, err)

		// Populate terminated resources from trackers (normally done by refreshSnapshot)
		snapshot2.TerminatedContainers = monitor.terminatedContainersTracker.Items()

		// Step 3: Validate running containers
		assert.Len(t, snapshot2.Containers, 1)
		assert.Contains(t, snapshot2.Containers, "container-2")
		assert.NotContains(t, snapshot2.Containers, "container-1", "Container container-1 should no longer be in running containers")

		// Step 4: Validate terminated containers - CORE BUSINESS LOGIC
		assert.Len(t, snapshot2.TerminatedContainers, 1, "Terminated containers should be tracked")

		// Find terminated container by ID
		var terminatedCntr1 *Container
		for _, cntr := range snapshot2.TerminatedContainers {
			if cntr.ID == "container-1" {
				terminatedCntr1 = cntr
				break
			}
		}
		require.NotNil(t, terminatedCntr1, "Container container-1 should exist in terminated containers")

		// TEST: Energy and power values are EXACTLY preserved from when container was running
		for _, zone := range zones {
			terminatedUsage := terminatedCntr1.Zones[zone]
			zoneName := zone.Name()

			// The terminated container should have EXACTLY the same energy as when it was last running
			assert.Equal(t, expectedEnergy[zoneName], terminatedUsage.EnergyTotal,
				"Terminated container energy should be preserved from last running state for zone %s", zoneName)

			// The terminated container should have EXACTLY the same power as when it was last running
			assert.Equal(t, expectedPower[zoneName], terminatedUsage.Power,
				"Terminated container power should be preserved from last running state for zone %s", zoneName)
		}

		// Step 5: Validate container metadata is preserved
		assert.Equal(t, runningCtnr1.ID, terminatedCntr1.ID)
		assert.Equal(t, runningCtnr1.Name, terminatedCntr1.Name)
		assert.Equal(t, runningCtnr1.Runtime, terminatedCntr1.Runtime)
		assert.Equal(t, runningCtnr1.PodID, terminatedCntr1.PodID)

		resInformer.AssertExpectations(t)
	})

	t.Run("terminated container cleanup after export", func(t *testing.T) {
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

		// Create snapshot with terminated container
		snapshot1 := NewSnapshot()
		snapshot1.Node = createNodeSnapshot(zones, fakeClock.Now(), 0.5)

		// Add a terminated container from a previous calculation
		oldTerminated := &Container{
			ID:      "old-terminated",
			Name:    "old-terminated-container",
			Runtime: resource.DockerRuntime,
			Zones:   make(ZoneUsageMap),
		}
		for _, zone := range zones {
			oldTerminated.Zones[zone] = Usage{
				EnergyTotal: 50 * Joule,
				Power:       10 * Watt,
			}
		}
		snapshot1.TerminatedContainers[oldTerminated.StringID()] = oldTerminated

		// Also add to the internal tracker (simulating previous processing)
		monitor.terminatedContainersTracker.Add(oldTerminated)

		// Create next snapshot with no new terminated containers
		snapshot2 := NewSnapshot()
		snapshot2.Node = createNodeSnapshot(zones, fakeClock.Now().Add(time.Second), 0.5)

		containers := &resource.Containers{
			Running: map[string]*resource.Container{
				"running-container": {ID: "running-container", Name: "running", Runtime: resource.DockerRuntime, CPUTimeDelta: 10.0},
			},
			Terminated: map[string]*resource.Container{}, // No new terminated containers
		}

		tr := CreateTestResources(createOnly(testNode))
		resInformer.On("Node").Return(tr.Node, nil).Maybe()
		resInformer.On("Containers").Return(containers).Once()

		// Before export terminated containers should be preserved
		monitor.exported.Store(false)
		err = monitor.calculateContainerPower(snapshot1, snapshot2)
		require.NoError(t, err)

		// Populate terminated resources from trackers (normally done by refreshSnapshot)
		snapshot2.TerminatedContainers = monitor.terminatedContainersTracker.Items()

		assert.Len(t, snapshot2.TerminatedContainers, 1, "Terminated containers should be preserved before export")
		// Verify old-terminated container is still present
		found := false
		for _, cntr := range snapshot2.TerminatedContainers {
			if cntr.ID == "old-terminated" {
				found = true
				break
			}
		}
		assert.True(t, found, "old-terminated container should be present")

		// After export, terminated containers should be cleaned up
		monitor.exported.Store(true)
		snapshot3 := NewSnapshot()
		snapshot3.Node = createNodeSnapshot(zones, fakeClock.Now().Add(2*time.Second), 0.5)

		resInformer.On("Node").Return(tr.Node, nil).Maybe()
		resInformer.On("Containers").Return(containers).Once()

		err = monitor.calculateContainerPower(snapshot2, snapshot3)
		require.NoError(t, err)

		// Populate terminated resources from trackers (normally done by refreshSnapshot)
		snapshot3.TerminatedContainers = monitor.terminatedContainersTracker.Items()

		assert.Len(t, snapshot3.TerminatedContainers, 0, "Terminated containers should be cleaned up after export")

		resInformer.AssertExpectations(t)
	})

	t.Run("multiple terminated containers accumulation", func(t *testing.T) {
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

		// Create initial snapshot with multiple running containers
		snapshot1 := NewSnapshot()
		snapshot1.Node = createNodeSnapshot(zones, fakeClock.Now(), 0.5)

		cntrsInitial := &resource.Containers{
			Running: map[string]*resource.Container{
				"container-1": {ID: "container-1", Name: "cont1", Runtime: resource.DockerRuntime, CPUTimeDelta: 10.0},
				"container-2": {ID: "container-2", Name: "cont2", Runtime: resource.PodmanRuntime, CPUTimeDelta: 20.0},
				"container-3": {ID: "container-3", Name: "cont3", Runtime: resource.CrioRuntime, CPUTimeDelta: 30.0},
			},
			Terminated: map[string]*resource.Container{},
		}

		tr1 := CreateTestResources(createOnly(testNode))
		resInformer.On("Node").Return(tr1.Node, nil).Maybe()
		resInformer.On("Containers").Return(cntrsInitial).Once()

		err = monitor.calculateContainerPower(NewSnapshot(), snapshot1)
		require.NoError(t, err)

		// Populate terminated resources from trackers (normally done by refreshSnapshot)
		snapshot1.TerminatedContainers = monitor.terminatedContainersTracker.Items()

		// Step 2: First container terminates
		snapshot2 := NewSnapshot()
		snapshot2.Node = createNodeSnapshot(zones, fakeClock.Now().Add(time.Second), 0.5)

		containers2 := &resource.Containers{
			Running: map[string]*resource.Container{
				"container-2": {ID: "container-2", Name: "cont2", Runtime: resource.PodmanRuntime, CPUTimeDelta: 20.0},
				"container-3": {ID: "container-3", Name: "cont3", Runtime: resource.CrioRuntime, CPUTimeDelta: 30.0},
			},
			Terminated: map[string]*resource.Container{
				"container-1": {ID: "container-1", Name: "cont1", Runtime: resource.DockerRuntime, CPUTimeDelta: 10.0},
			},
		}

		tr2 := CreateTestResources(createOnly(testNode))
		resInformer.On("Node").Return(tr2.Node, nil).Maybe()
		resInformer.On("Containers").Return(containers2).Once()

		err = monitor.calculateContainerPower(snapshot1, snapshot2)
		require.NoError(t, err)

		// Populate terminated resources from trackers (normally done by refreshSnapshot)
		snapshot2.TerminatedContainers = monitor.terminatedContainersTracker.Items()

		assert.Len(t, snapshot2.TerminatedContainers, 1, "Should have 1 terminated container")
		// Check that container-1 is in terminated containers
		foundContainer1 := false
		for _, cntr := range snapshot2.TerminatedContainers {
			if cntr.ID == "container-1" {
				foundContainer1 = true
				break
			}
		}
		assert.True(t, foundContainer1, "container-1 should be in terminated containers")

		// Step 3: Second container terminates
		snapshot3 := NewSnapshot()
		snapshot3.Node = createNodeSnapshot(zones, fakeClock.Now().Add(2*time.Second), 0.5)

		containers3 := &resource.Containers{
			Running: map[string]*resource.Container{
				"container-3": {ID: "container-3", Name: "cont3", Runtime: resource.CrioRuntime, CPUTimeDelta: 30.0},
			},
			Terminated: map[string]*resource.Container{
				"container-2": {ID: "container-2", Name: "cont2", Runtime: resource.PodmanRuntime, CPUTimeDelta: 20.0},
			},
		}

		tr3 := CreateTestResources(createOnly(testNode))
		resInformer.On("Node").Return(tr3.Node, nil).Maybe()
		resInformer.On("Containers").Return(containers3).Once()

		err = monitor.calculateContainerPower(snapshot2, snapshot3)
		require.NoError(t, err)

		// Populate terminated resources from trackers (normally done by refreshSnapshot)
		snapshot3.TerminatedContainers = monitor.terminatedContainersTracker.Items()

		// Should now have 2 terminated containers accumulated
		assert.Len(t, snapshot3.TerminatedContainers, 2, "Should have 2 terminated containers accumulated")

		// Check that both terminated containers are present
		containerIDs := make(map[string]bool)
		for _, cntr := range snapshot3.TerminatedContainers {
			containerIDs[cntr.ID] = true
		}
		assert.True(t, containerIDs["container-1"], "First terminated container should still be present")
		assert.True(t, containerIDs["container-2"], "Second terminated container should be added")

		assert.Len(t, snapshot3.Containers, 1, "Should have 1 running container")
		assert.Contains(t, snapshot3.Containers, "container-3")

		resInformer.AssertExpectations(t)
	})

	t.Run("terminated container with zero energy filtering", func(t *testing.T) {
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
			minTerminatedEnergyThreshold: 1 * Joule, // Set threshold to filter zero-energy containers
		}

		err := monitor.Init()
		require.NoError(t, err)

		// Reset monitor state
		monitor.exported.Store(false)

		// Create snapshot with container that has zero energy
		snapshot1 := NewSnapshot()
		snapshot1.Node = createNodeSnapshot(zones, fakeClock.Now(), 0.5)

		// Create container with zero energy in all zones
		zeroEnergyContainer := &Container{
			ID:      "zero-energy-container",
			Name:    "zero-container",
			Runtime: resource.DockerRuntime,
			Zones:   make(ZoneUsageMap),
		}
		for _, zone := range zones {
			zeroEnergyContainer.Zones[zone] = Usage{
				EnergyTotal: Energy(0), // Zero energy
				Power:       Power(0),
			}
		}
		snapshot1.Containers["zero-energy-container"] = zeroEnergyContainer

		// Create second snapshot where the zero-energy container terminates
		snapshot2 := NewSnapshot()
		snapshot2.Node = createNodeSnapshot(zones, fakeClock.Now().Add(time.Second), 0.5)

		containers := &resource.Containers{
			Running: map[string]*resource.Container{},
			Terminated: map[string]*resource.Container{
				"zero-energy-container": {ID: "zero-energy-container", Name: "zero-container", Runtime: resource.DockerRuntime},
			},
		}

		tr := CreateTestResources(createOnly(testNode))
		resInformer.On("Node").Return(tr.Node, nil).Maybe()
		resInformer.On("Containers").Return(containers).Once()

		err = monitor.calculateContainerPower(snapshot1, snapshot2)
		require.NoError(t, err)

		// Populate terminated resources from trackers (normally done by refreshSnapshot)
		snapshot2.TerminatedContainers = monitor.terminatedContainersTracker.Items()

		// Zero-energy terminated containers should be filtered out
		assert.Len(t, snapshot2.TerminatedContainers, 0, "Containers with zero energy should be filtered out from terminated containers")
		assert.Len(t, snapshot2.Containers, 0, "No running containers")

		resInformer.AssertExpectations(t)
	})
}
