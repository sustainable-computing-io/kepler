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

func TestPodPowerCalculation(t *testing.T) {
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
		logger:        logger,
		cpu:           mockMeter,
		clock:         fakeClock,
		resources:     resInformer,
		maxTerminated: 500,
	}

	err := monitor.Init()
	require.NoError(t, err)

	t.Run("firstPodRead", func(t *testing.T) {
		tr := CreateTestResources(createOnly(testPods, testNode))
		require.NotNil(t, tr.Pods)

		resInformer.SetExpectations(t, tr)

		snapshot := NewSnapshot()
		err := monitor.firstNodeRead(snapshot.Node)
		require.NoError(t, err)

		err = monitor.firstPodRead(snapshot)
		require.NoError(t, err)

		// Verify pod power tracking was initialized
		assert.Len(t, snapshot.Pods, len(tr.Pods.Running))

		for id, pod := range snapshot.Pods {
			originalPod := tr.Pods.Running[id]
			assert.Equal(t, originalPod.ID, pod.ID)
			assert.Equal(t, originalPod.Name, pod.Name)
			assert.Equal(t, originalPod.Namespace, pod.Namespace)
			assert.Equal(t, originalPod.CPUTotalTime, pod.CPUTotalTime)

			// Verify zones were initialized with correct energy attribution
			assert.Len(t, pod.Zones, len(zones))

			// Calculate expected energy based on CPU ratio
			nodeCPUTimeDelta := tr.Node.ProcessTotalCPUTimeDelta
			cpuTimeRatio := originalPod.CPUTimeDelta / nodeCPUTimeDelta

			for _, zone := range zones {
				usage, exists := pod.Zones[zone]
				assert.True(t, exists, "Zone %s should exist in pod zones", zone.Name())

				nodeZoneUsage := snapshot.Node.Zones[zone]
				expectedEnergy := Energy(cpuTimeRatio * float64(nodeZoneUsage.activeEnergy))

				assert.Equal(t, expectedEnergy, usage.EnergyTotal, "Pod should get proportional share of node's active energy for zone %s", zone.Name())
				assert.Equal(t, Power(0), usage.Power, "Power should be 0 for first read")
			}
		}
	})

	t.Run("calculatePodPower", func(t *testing.T) {
		// Create previous snapshot with realistic node data and manually add pod data
		prevSnapshot := NewSnapshot()
		prevSnapshot.Node = createNodeSnapshot(zones, fakeClock.Now(), 0.5)

		// Add existing pod data to previous snapshot (similar to container test pattern)
		prevSnapshot.Pods["pod-1"] = &Pod{
			ID:           "pod-1",
			Name:         "test-pod-1",
			Namespace:    "default",
			CPUTotalTime: 5.0,
			Zones:        make(ZoneUsageMap, len(zones)),
		}

		// Initialize zones for previous pod
		for _, zone := range zones {
			prevSnapshot.Pods["pod-1"].Zones[zone] = Usage{
				EnergyTotal: 25 * Joule,
				Power:       Power(0),
			}
		}

		// Create new snapshot with updated node data
		fakeClock.Step(time.Second * 2)
		newSnapshot := NewSnapshot()
		newSnapshot.Node = createNodeSnapshot(zones, fakeClock.Now().Add(time.Second), 0.5)

		// Setup mock to return updated pods
		tr := CreateTestResources()
		require.NotNil(t, tr.Node)
		pods := tr.Pods
		resInformer.On("Node").Return(tr.Node, nil)
		resInformer.On("Pods").Return(pods)

		err = monitor.calculatePodPower(prevSnapshot, newSnapshot)
		require.NoError(t, err)

		// Verify pod power was calculated
		assert.Len(t, newSnapshot.Pods, len(pods.Running))

		totalNodeActivePower := calculateTotalNodeActivePower(newSnapshot.Node.Zones)
		totalPodPower := calculateTotalPodPower(newSnapshot.Pods)

		// Pod power should be distributed from node active power
		assert.Greater(t, totalPodPower, Power(0))
		assert.LessOrEqual(t, totalPodPower, totalNodeActivePower)

		for id, pod := range newSnapshot.Pods {
			// Verify power and energy values are reasonable
			for zone, usage := range pod.Zones {
				assert.GreaterOrEqual(t, usage.EnergyTotal, Energy(0),
					"Pod %s zone %s energy should be non-negative", id, zone.Name())
				assert.GreaterOrEqual(t, usage.Power, Power(0),
					"Pod %s zone %s power should be non-negative", id, zone.Name())
			}
		}
	})

	t.Run("calculatePodPower_with_zero_node_power", func(t *testing.T) {
		tr := CreateTestResources(createOnly(testPods, testNode))
		require.NotNil(t, tr.Pods)

		resInformer.SetExpectations(t, tr)

		// Create snapshots with zero node power
		prevSnapshot := NewSnapshot()
		prevSnapshot.Node = createNodeSnapshot(zones, fakeClock.Now(), 0.0) // Zero usage ratio
		err = monitor.firstPodRead(prevSnapshot)
		require.NoError(t, err)

		newSnapshot := NewSnapshot()
		newSnapshot.Node = createNodeSnapshot(zones, fakeClock.Now(), 0.0) // Zero usage ratio

		err = monitor.calculatePodPower(prevSnapshot, newSnapshot)
		require.NoError(t, err)

		// With zero node power, all pod power should be zero
		for id, pod := range newSnapshot.Pods {
			for zone, usage := range pod.Zones {
				assert.Equal(t, Power(0), usage.Power,
					"Pod %s zone %s power should be zero when node power is zero", id, zone.Name())
				assert.Equal(t, Energy(0), usage.EnergyTotal,
					"Pod %s zone %s energy should be zero when node power is zero", id, zone.Name())
			}
		}
	})

	t.Run("calculatePodPower_without_pods", func(t *testing.T) {
		// Create empty pods
		emptyPods := &resource.Pods{
			Running:    make(map[string]*resource.Pod),
			Terminated: make(map[string]*resource.Pod),
		}

		tr := CreateTestResources(createOnly(testNode))
		resInformer.On("Node").Return(tr.Node, nil).Once()
		resInformer.On("Pods").Return(emptyPods).Once()

		prevSnapshot := NewSnapshot()
		prevSnapshot.Node = createNodeSnapshot(zones, fakeClock.Now(), 0.5)

		newSnapshot := NewSnapshot()
		newSnapshot.Node = createNodeSnapshot(zones, fakeClock.Now(), 0.5)

		err = monitor.calculatePodPower(prevSnapshot, newSnapshot)
		require.NoError(t, err)

		// Should handle empty pods gracefully
		assert.Empty(t, newSnapshot.Pods)
	})
}

func TestPodPowerConsistency(t *testing.T) {
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
		logger:        logger,
		cpu:           mockMeter,
		clock:         fakeClock,
		resources:     resInformer,
		maxTerminated: 500,
	}

	err := monitor.Init()
	require.NoError(t, err)

	t.Run("power_conservation_across_pods", func(t *testing.T) {
		tr := CreateTestResources(createOnly(testPods, testNode))
		require.NotNil(t, tr.Pods)

		resInformer.SetExpectations(t, tr)

		// Create and calculate power
		prevSnapshot := NewSnapshot()
		prevSnapshot.Node = createNodeSnapshot(zones, fakeClock.Now(), 0.6)
		err = monitor.firstPodRead(prevSnapshot)
		require.NoError(t, err)

		fakeClock.Step(time.Second * 2)
		newSnapshot := NewSnapshot()
		newSnapshot.Node = createNodeSnapshot(zones, fakeClock.Now(), 0.6)
		err = monitor.calculatePodPower(prevSnapshot, newSnapshot)
		require.NoError(t, err)

		// Verify power conservation: sum of pod power should not exceed node active power
		totalNodeActivePower := calculateTotalNodeActivePower(newSnapshot.Node.Zones)
		totalPodPower := calculateTotalPodPower(newSnapshot.Pods)

		assert.LessOrEqual(t, totalPodPower, totalNodeActivePower,
			"Total pod power should not exceed total node active power")

		// Verify energy conservation over time
		totalNodeActiveEnergy := calculateTotalNodeActiveEnergy(newSnapshot.Node.Zones)
		totalPodEnergy := calculateTotalPodEnergy(newSnapshot.Pods)

		assert.LessOrEqual(t, totalPodEnergy, totalNodeActiveEnergy,
			"Total pod energy should not exceed total node active energy")
	})
}

func TestTerminatedPodTracking(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	fakeClock := testingclock.NewFakeClock(time.Now())
	zones := CreateTestZones()

	t.Run("terminated_pod_energy_accumulation", func(t *testing.T) {
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

		// Setup previous snapshot with pod data (similar to container test pattern)
		prevSnapshot := NewSnapshot()
		prevSnapshot.Node = createNodeSnapshot(zones, fakeClock.Now(), 0.7)

		// Add existing pod data to previous snapshot
		prevSnapshot.Pods["pod-1"] = &Pod{
			ID:           "pod-1",
			Name:         "test-pod-1",
			Namespace:    "default",
			CPUTotalTime: 5.0,
			Zones:        make(ZoneUsageMap, len(zones)),
		}
		prevSnapshot.Pods["pod-2"] = &Pod{
			ID:           "pod-2",
			Name:         "test-pod-2",
			Namespace:    "default",
			CPUTotalTime: 3.0,
			Zones:        make(ZoneUsageMap, len(zones)),
		}

		// Initialize zones for previous pods
		for _, zone := range zones {
			prevSnapshot.Pods["pod-1"].Zones[zone] = Usage{
				EnergyTotal: 25 * Joule,
				Power:       15,
			}
			prevSnapshot.Pods["pod-2"].Zones[zone] = Usage{
				EnergyTotal: 15 * Joule,
				Power:       10,
			}
		}

		// Setup pods where pod-1 terminates
		pods := &resource.Pods{
			Running: map[string]*resource.Pod{
				"pod-2": {ID: "pod-2", Name: "test-pod-2", Namespace: "default", CPUTimeDelta: 20.0},
			},
			Terminated: map[string]*resource.Pod{
				"pod-1": {ID: "pod-1", Name: "test-pod-1", Namespace: "default", CPUTimeDelta: 30.0},
			},
		}

		tr := CreateTestResources(createOnly(testNode))
		resInformer.On("Node").Return(tr.Node, nil)
		resInformer.On("Pods").Return(pods)

		// Create new snapshot with updated node data
		fakeClock.Step(time.Second * 2)
		newSnapshot := NewSnapshot()
		newSnapshot.Node = createNodeSnapshot(zones, fakeClock.Now().Add(time.Second), 0.7)

		err = monitor.calculatePodPower(prevSnapshot, newSnapshot)
		require.NoError(t, err)

		// Populate terminated resources from trackers (normally done by refreshSnapshot)
		newSnapshot.TerminatedPods = monitor.terminatedPodsTracker.Items()

		// Verify terminated pod tracking
		assert.Len(t, newSnapshot.TerminatedPods, 1, "Terminated pods should be tracked")

		// Find terminated pod by ID
		var terminatedPod *Pod
		for _, pod := range newSnapshot.TerminatedPods {
			if pod.ID == "pod-1" {
				terminatedPod = pod
				break
			}
		}
		require.NotNil(t, terminatedPod, "Pod pod-1 should exist in terminated pods")
		originalPod := prevSnapshot.Pods["pod-1"]

		// Verify terminated pod preserves energy and power from last running state
		assert.Equal(t, originalPod.ID, terminatedPod.ID)
		assert.Equal(t, originalPod.Name, terminatedPod.Name)
		assert.Equal(t, originalPod.Namespace, terminatedPod.Namespace)

		expectedEnergy := make(map[string]Energy)
		expectedPower := make(map[string]Power)
		for zoneName, usage := range originalPod.Zones {
			expectedEnergy[zoneName.Name()] = usage.EnergyTotal
			expectedPower[zoneName.Name()] = usage.Power
		}

		for zoneName, terminatedUsage := range terminatedPod.Zones {
			assert.Equal(t, expectedEnergy[zoneName.Name()], terminatedUsage.EnergyTotal,
				"Terminated pod energy should be preserved from last running state for zone %s", zoneName.Name())
			assert.Equal(t, expectedPower[zoneName.Name()], terminatedUsage.Power,
				"Terminated pod power should be preserved from last running state for zone %s", zoneName.Name())
		}

		// Verify running pods still exist
		assert.Len(t, newSnapshot.Pods, 1)
		assert.Contains(t, newSnapshot.Pods, "pod-2")
	})

	t.Run("terminated_pod_cleanup_after_export", func(t *testing.T) {
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

		// Start with terminated pod already in system
		pods1 := &resource.Pods{
			Running: map[string]*resource.Pod{
				"pod-2": {ID: "pod-2", Name: "test-pod-2", Namespace: "default", CPUTimeDelta: 20.0},
			},
			Terminated: map[string]*resource.Pod{
				"pod-1": {ID: "pod-1", Name: "test-pod-1", Namespace: "default", CPUTimeDelta: 30.0},
			},
		}

		tr1 := CreateTestResources(createOnly(testNode))
		resInformer.On("Node").Return(tr1.Node, nil).Times(3)
		resInformer.On("Pods").Return(pods1).Times(3)

		// Create snapshot with terminated pod data
		snapshot1 := NewSnapshot()
		oldTerminatedPod := &Pod{
			ID:        "pod-1",
			Name:      "test-pod-1",
			Namespace: "default",
			Zones:     make(ZoneUsageMap),
		}

		// Initialize zones with energy above threshold to pass energy filtering
		for _, zone := range zones {
			oldTerminatedPod.Zones[zone] = Usage{
				EnergyTotal: 50 * Joule,
				Power:       10,
			}
		}

		snapshot1.Pods = map[string]*Pod{"pod-1": oldTerminatedPod} // Add to previous pods so it can be found

		// Before export - terminated pods should be preserved
		monitor.exported.Store(false)
		snapshot1.Node = createNodeSnapshot(zones, fakeClock.Now(), 0.5)

		newSnapshot := NewSnapshot()
		newSnapshot.Node = createNodeSnapshot(zones, fakeClock.Now(), 0.5)
		err = monitor.calculatePodPower(snapshot1, newSnapshot)
		require.NoError(t, err)

		// Populate terminated resources from trackers (normally done by refreshSnapshot)
		newSnapshot.TerminatedPods = monitor.terminatedPodsTracker.Items()

		assert.Len(t, newSnapshot.TerminatedPods, 1, "Terminated pods should be preserved before export")

		// After export - terminated pods should be cleaned up
		monitor.exported.Store(true)
		snapshot3 := NewSnapshot()
		snapshot3.Node = createNodeSnapshot(zones, fakeClock.Now(), 0.5)
		err = monitor.calculatePodPower(newSnapshot, snapshot3)
		require.NoError(t, err)

		// Populate terminated resources from trackers (normally done by refreshSnapshot)
		snapshot3.TerminatedPods = monitor.terminatedPodsTracker.Items()

		assert.Len(t, snapshot3.TerminatedPods, 0, "Terminated pods should be cleaned up after export")
	})

	t.Run("multiple_terminated_pods_accumulation", func(t *testing.T) {
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

		// Create initial snapshot with multiple running pods
		snapshot1 := NewSnapshot()
		snapshot1.Node = createNodeSnapshot(zones, fakeClock.Now(), 0.5)

		podsInitial := &resource.Pods{
			Running: map[string]*resource.Pod{
				"pod-1": {ID: "pod-1", Name: "test-pod-1", Namespace: "default", CPUTimeDelta: 10.0},
				"pod-2": {ID: "pod-2", Name: "test-pod-2", Namespace: "default", CPUTimeDelta: 20.0},
				"pod-3": {ID: "pod-3", Name: "test-pod-3", Namespace: "default", CPUTimeDelta: 30.0},
			},
			Terminated: map[string]*resource.Pod{},
		}

		tr1 := CreateTestResources(createOnly(testNode))
		resInformer.On("Node").Return(tr1.Node, nil).Maybe()
		resInformer.On("Pods").Return(podsInitial).Once()

		err = monitor.calculatePodPower(NewSnapshot(), snapshot1)
		require.NoError(t, err)

		// Step 2: First pod terminates
		snapshot2 := NewSnapshot()
		snapshot2.Node = createNodeSnapshot(zones, fakeClock.Now().Add(time.Second), 0.5)

		pods2 := &resource.Pods{
			Running: map[string]*resource.Pod{
				"pod-2": {ID: "pod-2", Name: "test-pod-2", Namespace: "default", CPUTimeDelta: 20.0},
				"pod-3": {ID: "pod-3", Name: "test-pod-3", Namespace: "default", CPUTimeDelta: 30.0},
			},
			Terminated: map[string]*resource.Pod{
				"pod-1": {ID: "pod-1", Name: "test-pod-1", Namespace: "default", CPUTimeDelta: 10.0},
			},
		}

		tr2 := CreateTestResources(createOnly(testNode))
		resInformer.On("Node").Return(tr2.Node, nil).Maybe()
		resInformer.On("Pods").Return(pods2).Once()

		err = monitor.calculatePodPower(snapshot1, snapshot2)
		require.NoError(t, err)

		// Populate terminated resources from trackers (normally done by refreshSnapshot)
		snapshot2.TerminatedPods = monitor.terminatedPodsTracker.Items()

		assert.Len(t, snapshot2.TerminatedPods, 1, "Should have 1 terminated pod")
		// Check pod-1 is in terminated pods
		found := false
		for _, pod := range snapshot2.TerminatedPods {
			if pod.ID == "pod-1" {
				found = true
				break
			}
		}
		assert.True(t, found, "pod-1 should be in terminated pods")

		// Step 3: Second pod terminates
		snapshot3 := NewSnapshot()
		snapshot3.Node = createNodeSnapshot(zones, fakeClock.Now().Add(2*time.Second), 0.5)

		pods3 := &resource.Pods{
			Running: map[string]*resource.Pod{
				"pod-3": {ID: "pod-3", Name: "test-pod-3", Namespace: "default", CPUTimeDelta: 30.0},
			},
			Terminated: map[string]*resource.Pod{
				"pod-2": {ID: "pod-2", Name: "test-pod-2", Namespace: "default", CPUTimeDelta: 20.0},
			},
		}

		tr3 := CreateTestResources(createOnly(testNode))
		resInformer.On("Node").Return(tr3.Node, nil).Maybe()
		resInformer.On("Pods").Return(pods3).Once()

		err = monitor.calculatePodPower(snapshot2, snapshot3)
		require.NoError(t, err)

		// Populate terminated resources from trackers (normally done by refreshSnapshot)
		snapshot3.TerminatedPods = monitor.terminatedPodsTracker.Items()

		// Should now have 2 terminated pods accumulated
		assert.Len(t, snapshot3.TerminatedPods, 2, "Should have 2 terminated pods accumulated")

		// Check that both terminated pods are present
		podIDs := make(map[string]bool)
		for _, pod := range snapshot3.TerminatedPods {
			podIDs[pod.ID] = true
		}
		assert.True(t, podIDs["pod-1"], "First terminated pod should still be present")
		assert.True(t, podIDs["pod-2"], "Second terminated pod should be added")

		assert.Len(t, snapshot3.Pods, 1, "Should have 1 running pod")
		assert.Contains(t, snapshot3.Pods, "pod-3")
	})

	t.Run("terminated_pod_with_zero_energy_filtering", func(t *testing.T) {
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
			minTerminatedEnergyThreshold: 1 * Joule, // Set threshold to filter zero-energy pods
		}

		err := monitor.Init()
		require.NoError(t, err)

		// Pod with zero energy should be filtered out
		pods1 := &resource.Pods{
			Running: map[string]*resource.Pod{
				"pod-1": {ID: "pod-1", Name: "test-pod-1", Namespace: "default", CPUTimeDelta: 0.0}, // Zero CPU time
			},
			Terminated: map[string]*resource.Pod{},
		}

		tr1 := CreateTestResources(createOnly(testNode))
		resInformer.On("Node").Return(tr1.Node, nil).Once()
		resInformer.On("Pods").Return(pods1).Once()

		snapshot1 := NewSnapshot()
		snapshot1.Node = createNodeSnapshot(zones, fakeClock.Now(), 0.0) // Zero usage for zero energy
		err = monitor.firstPodRead(snapshot1)
		require.NoError(t, err)

		// Pod terminates with zero energy
		pods2 := &resource.Pods{
			Running: map[string]*resource.Pod{},
			Terminated: map[string]*resource.Pod{
				"pod-1": {ID: "pod-1", Name: "test-pod-1", Namespace: "default", CPUTimeDelta: 0.0},
			},
		}

		tr2 := CreateTestResources(createOnly(testNode))
		resInformer.On("Node").Return(tr2.Node, nil).Once()
		resInformer.On("Pods").Return(pods2).Once()

		newSnapshot := NewSnapshot()
		newSnapshot.Node = createNodeSnapshot(zones, fakeClock.Now(), 0.0) // Keep zero usage
		err = monitor.calculatePodPower(snapshot1, newSnapshot)
		require.NoError(t, err)

		// Populate terminated resources from trackers (normally done by refreshSnapshot)
		newSnapshot.TerminatedPods = monitor.terminatedPodsTracker.Items()

		// Pod with zero energy should be filtered out from terminated tracking
		assert.Len(t, newSnapshot.TerminatedPods, 0, "Pods with zero energy should be filtered out")
	})
}

// Helper functions
func calculateTotalPodPower(pods Pods) Power {
	var total Power
	for _, pod := range pods {
		for _, usage := range pod.Zones {
			total += usage.Power
		}
	}
	return total
}

func calculateTotalPodEnergy(pods Pods) Energy {
	var total Energy
	for _, pod := range pods {
		for _, usage := range pod.Zones {
			total += usage.EnergyTotal
		}
	}
	return total
}

func calculateTotalNodeActivePower(zones NodeZoneUsageMap) Power {
	var total Power
	for _, usage := range zones {
		total += usage.ActivePower
	}
	return total
}

func calculateTotalNodeActiveEnergy(zones NodeZoneUsageMap) Energy {
	var total Energy
	for _, usage := range zones {
		total += usage.activeEnergy
	}
	return total
}
