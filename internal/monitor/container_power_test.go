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

	t.Run("firstContainerRead", func(t *testing.T) {
		containers := CreateTestResources().Containers
		resourceInformer.On("Containers").Return(containers).Once()

		snapshot := NewSnapshot()
		err := monitor.firstNodeRead(snapshot.Node)
		require.NoError(t, err)

		err = monitor.firstContainerRead(snapshot)
		require.NoError(t, err)

		// Verify processes were initialized
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

		resourceInformer.AssertExpectations(t)
	})

	t.Run("calculateContainerPower", func(t *testing.T) {
		// Setup previous snapshot with process data
		prevSnapshot := NewSnapshot()
		prevSnapshot.Node = createNodeSnapshot(zones, fakeClock.Now())

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
		newSnapshot.Node = createNodeSnapshot(zones, fakeClock.Now().Add(time.Second))

		// Setup mock to return updated processes
		tr := CreateTestResources()
		procs, containers := tr.Processes, tr.Containers
		resourceInformer.On("Containers").Return(containers).Once()

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
			// Expected power = 0.4 * 50W = 20W
			expectedPower := 0.4 * 50 * Watt // 20W in microwatts
			assert.InDelta(t, expectedPower.MicroWatts(), usage.Power.MicroWatts(), 0.01)

			expectedDelta := 0.4 * 50 * Joule // 20J in microjoules
			assert.InDelta(t, expectedDelta.MicroJoules(), usage.Delta.MicroJoules(), 0.01)

			// Absolute should be previous + delta = 25J + 20J = 45J
			expectedAbsolute := 45 * Joule
			assert.InDelta(t, expectedAbsolute.MicroJoules(), usage.Absolute.MicroJoules(), 0.01)
		}

		// Check process 456 (new process)
		ctnr2 := newSnapshot.Containers["container-2"]
		for _, zone := range zones {
			usage := ctnr2.Zones[zone]
			// CPU ratio = 40.0 / 100.0 = 0.4 (40%)
			expectedPower := 0.2 * 50 * Watt // 20W
			assert.InDelta(t, expectedPower.MicroWatts(), usage.Power.MicroWatts(), 0.01)

			expectedDelta := Energy(0.2 * 50 * Joule) // 25J
			assert.InDelta(t, expectedDelta.MicroJoules(), usage.Delta.MicroJoules(), 0.01)
			// New process, so absolute = delta
			assert.Equal(t, usage.Delta, usage.Absolute)
		}
		resourceInformer.AssertExpectations(t)
	})

	t.Run("calculateContainerPower with zero node power", func(t *testing.T) {
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

		containers := CreateTestResources().Containers
		resourceInformer.On("Containers").Return(containers).Once()

		err := monitor.calculateContainerPower(prevSnapshot, newSnapshot)
		require.NoError(t, err)

		// All processes should have zero power
		for _, proc := range newSnapshot.Containers {
			for _, zone := range zones {
				usage := proc.Zones[zone]
				assert.Equal(t, Power(0), usage.Power)
				assert.Equal(t, Energy(0), usage.Delta)
				assert.Equal(t, Energy(0), usage.Absolute)
			}
		}

		resourceInformer.AssertExpectations(t)
	})

	t.Run("calculateContainerPower without containers", func(t *testing.T) {
		prevSnapshot := NewSnapshot()
		newSnapshot := NewSnapshot()
		newSnapshot.Node = createNodeSnapshot(zones, fakeClock.Now())

		// Return empty processes
		emptyContaineres := &resource.Containers{
			NodeCPUTimeDelta: 0.0,
			Running:          map[string]*resource.Container{},
			Terminated:       map[string]*resource.Container{},
		}
		resourceInformer.On("Containers").Return(emptyContaineres).Once()

		err := monitor.calculateContainerPower(prevSnapshot, newSnapshot)
		require.NoError(t, err)

		assert.Empty(t, newSnapshot.Containers)

		resourceInformer.AssertExpectations(t)
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
		newSnapshot.Node = createNodeSnapshot(zones, fakeClock.Now())

		// Create processes with varying CPU usage
		testContaineres := &resource.Containers{
			NodeCPUTimeDelta: 100.0,
			Running: map[string]*resource.Container{
				"c1": {ID: "c1", Name: "container-1", Runtime: resource.PodmanRuntime, CPUTimeDelta: 25.0},
				"c2": {ID: "c2", Name: "container-2", Runtime: resource.PodmanRuntime, CPUTimeDelta: 35.0},
				"c3": {ID: "c3", Name: "container-3", Runtime: resource.PodmanRuntime, CPUTimeDelta: 40.0},
			},
			Terminated: map[string]*resource.Container{},
		}

		mockResourceInformer.On("Containers").Return(testContaineres).Once()

		err := monitor.calculateContainerPower(prevSnapshot, newSnapshot)
		require.NoError(t, err)

		// Verify power conservation for each zone
		for _, zone := range zones {
			nodePower := newSnapshot.Node.Zones[zone].Power.MicroWatts()
			totalContainerPower := float64(0)

			for _, proc := range newSnapshot.Containers {
				totalContainerPower += proc.Zones[zone].Power.MicroWatts()
			}

			// Total process power should equal node power (with small tolerance for floating point)
			assert.InDelta(t, nodePower, totalContainerPower, 1.0,
				"Power conservation failed for zone %s", zone.Name())
		}

		mockResourceInformer.AssertExpectations(t)
	})

	mockMeter.AssertExpectations(t)
}
