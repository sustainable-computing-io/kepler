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

	t.Run("firstContainerRead", func(t *testing.T) {
		tr := CreateTestResources(createOnly(testContainers))
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
			assert.Equal(t, Energy(0), usage.Absolute)
			assert.Equal(t, Energy(0), usage.Delta)
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
			prevSnapshot.Containers["container-1"].Zones[zone] = &Usage{
				Absolute: 25 * Joule,
				Delta:    Energy(0),
				Power:    Power(0),
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

			// Delta should use used energy (since container.go:101 uses nodeZoneUsage.ActiveEnergy)
			expectedDelta := 0.4 * 50 * Joule // 20J in microjoules (0.4 * ActiveEnergy)
			assert.InDelta(t, expectedDelta.MicroJoules(), usage.Delta.MicroJoules(), 0.01)

			// Absolute should be previous + delta = 25J + 20J = 45J
			expectedAbsolute := 45 * Joule
			assert.InDelta(t, expectedAbsolute.MicroJoules(), usage.Absolute.MicroJoules(), 0.01)
		}

		// Check process 456 (new process)
		ctnr2 := newSnapshot.Containers["container-2"]
		for _, zone := range zones {
			usage := ctnr2.Zones[zone]
			// CPU ratio = 20.0 / 100.0 = 0.2 (20%)
			// ActivePower = 50W * 0.5 = 25W, so 0.2 * 25W = 5W
			expectedPower := 0.2 * 25 * Watt // 5W
			assert.InDelta(t, expectedPower.MicroWatts(), usage.Power.MicroWatts(), 0.01)

			// Delta uses used energy: 0.2 * 50J = 10J
			expectedDelta := Energy(0.2 * 50 * Joule) // 10J
			assert.InDelta(t, expectedDelta.MicroJoules(), usage.Delta.MicroJoules(), 0.01)
			// New process, so absolute = delta
			assert.Equal(t, usage.Delta, usage.Absolute)
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
			newSnapshot.Node.Zones[zone] = &NodeUsage{
				Absolute:     Energy(100_000_000),
				Delta:        Energy(50_000_000),
				ActiveEnergy: Energy(0),
				IdleEnergy:   Energy(0),
				Power:        Power(0), // Zero power
				ActivePower:  Power(0),
				IdlePower:    Power(0),
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
				assert.Equal(t, Energy(0), usage.Delta)
				assert.Equal(t, Energy(0), usage.Absolute)
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
			totalContainerPower := float64(0)

			for _, proc := range newSnapshot.Containers {
				totalContainerPower += proc.Zones[zone].Power.MicroWatts()
			}

			// Containers have total CPU delta of 100.0 out of node's 200.0 (50%)
			// So total container power should be 50% of node ActivePower
			nodeActivePower := newSnapshot.Node.Zones[zone].ActivePower.MicroWatts()
			expectedContainerPower := nodeActivePower * 0.5 // 50% of used power
			assert.InDelta(t, expectedContainerPower, totalContainerPower, 1.0,
				"Power conservation failed for zone %s", zone.Name())
		}

		mockResourceInformer.AssertExpectations(t)
	})

	mockMeter.AssertExpectations(t)
}
