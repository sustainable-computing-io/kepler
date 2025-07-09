// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package monitor

import (
	"fmt"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/sustainable-computing-io/kepler/internal/device"
)

// MockResource implements the Resource interface for testing
type MockResource struct {
	id    string
	zones ZoneUsageMap
}

func (mr *MockResource) StringID() string {
	return mr.id
}

func (mr *MockResource) ZoneUsage() ZoneUsageMap {
	return mr.zones
}

// Helper function to create a mock resource with specific energy in a zone
func createMockResource(id string, zone device.EnergyZone, energy Energy) *MockResource {
	zones := make(ZoneUsageMap)
	zones[zone] = Usage{
		EnergyTotal: energy,
		Power:       Power(0),
	}
	return &MockResource{
		id:    id,
		zones: zones,
	}
}

// Helper function to create a mock resource with energy in multiple zones
func createMockResourceMultiZone(id string, zoneEnergies map[device.EnergyZone]Energy) *MockResource {
	zones := make(ZoneUsageMap)
	for zone, energy := range zoneEnergies {
		zones[zone] = Usage{
			EnergyTotal: energy,
			Power:       Power(0),
		}
	}
	return &MockResource{
		id:    id,
		zones: zones,
	}
}

func TestTerminatedResourceTracker_New(t *testing.T) {
	zones := CreateTestZones()
	zone := zones[0]
	maxSize := 10

	tracker := NewTerminatedResourceTracker[*MockResource](zone, maxSize, 0*Joule, slog.Default().With("test", "tracker"))

	assert.NotNil(t, tracker)
	assert.Equal(t, 0, tracker.Size())
	assert.Equal(t, maxSize, tracker.MaxSize())
	assert.Equal(t, zone, tracker.EnergyZone())
	assert.Equal(t, 0, len(tracker.Items()))
}

func TestTerminatedResourceTracker_AddSingleResource(t *testing.T) {
	zones := CreateTestZones()
	zone := zones[0]
	tracker := NewTerminatedResourceTracker[*MockResource](zone, 5, 6*Joule, slog.Default())

	// Add a resource with energy
	resource := createMockResource("resource-1", zone, 1000*Joule)
	tracker.Add(resource)

	assert.Equal(t, 1, tracker.Size())
	items := tracker.Items()
	require.Len(t, items, 1)
	assert.Contains(t, items, "resource-1")
}

func TestTerminatedResourceTracker_AddResourceWithZeroEnergy(t *testing.T) {
	zones := CreateTestZones()
	zone := zones[0]
	tracker := NewTerminatedResourceTracker[*MockResource](zone, 5, 1*Joule, slog.Default())

	// Add a resource with zero energy - should be ignored due to threshold
	resource := createMockResource("resource-1", zone, 0*Joule)
	tracker.Add(resource)

	assert.Equal(t, 0, tracker.Size())
	assert.Equal(t, 0, len(tracker.Items()))
}

func TestTerminatedResourceTracker_AddResourceWithoutTrackedZone(t *testing.T) {
	zones := CreateTestZones()
	trackedZone := zones[0]
	otherZone := zones[1]
	tracker := NewTerminatedResourceTracker[*MockResource](trackedZone, 5, 1*Joule, slog.Default())

	// Add a resource that only has energy in a different zone
	resource := createMockResource("resource-1", otherZone, 1000*Joule)
	tracker.Add(resource)

	assert.Equal(t, 0, tracker.Size())
	assert.Equal(t, 0, len(tracker.Items()))
}

func TestTerminatedResourceTracker_AddMultipleResources(t *testing.T) {
	zones := CreateTestZones()
	zone := zones[0]
	tracker := NewTerminatedResourceTracker[*MockResource](zone, 5, 10*Joule, slog.Default())

	// Add multiple resources with different energies
	resources := []*MockResource{
		createMockResource("resource-1", zone, 1000*Joule),
		createMockResource("resource-2", zone, 2000*Joule),
		createMockResource("resource-3", zone, 500*Joule),
	}

	for _, resource := range resources {
		tracker.Add(resource)
	}

	assert.Equal(t, 3, tracker.Size())
	items := tracker.Items()
	assert.Len(t, items, 3)

	// Check all resources are present (order doesn't matter in Items())
	ids := make(map[string]bool)
	for _, item := range items {
		ids[item.StringID()] = true
	}
	assert.True(t, ids["resource-1"])
	assert.True(t, ids["resource-2"])
	assert.True(t, ids["resource-3"])
}

func TestTerminatedResourceTracker_DuplicatesIgnored(t *testing.T) {
	zones := CreateTestZones()
	zone := zones[0]
	tracker := NewTerminatedResourceTracker[*MockResource](zone, 5, 8*Joule, slog.Default())

	// Add initial resource
	resource1 := createMockResource("resource-1", zone, 1000*Joule)
	tracker.Add(resource1)
	assert.Equal(t, 1, tracker.Size())

	// NOTE: In normal operation, the same terminated resource should never
	// be added twice. Verify that if it happens, the duplicate is ignored.

	// Adding the same resource ID again should be ignored (safety check)
	resource2 := createMockResource("resource-1", zone, 2000*Joule)
	tracker.Add(resource2)
	assert.Equal(t, 1, tracker.Size()) // Still only one entry

	items := tracker.Items()
	require.Len(t, items, 1)
	// Original resource energy is preserved (duplicate was ignored)
	assert.Equal(t, Energy(1000*Joule), items["resource-1"].ZoneUsage()[zone].EnergyTotal)
}

// TestTerminatedResourceTracker_EvictOnCapactity validates that when the
// capacity is reached, the lowest energy resource is evicted.
func TestTerminatedResourceTracker_EvictOnCapactity(t *testing.T) {
	zones := CreateTestZones()
	zone := zones[0]
	maxSize := 3
	tracker := NewTerminatedResourceTracker[*MockResource](zone, maxSize, 7*Joule, slog.Default().With("test", "tracker"))

	// Fill to capacity with different energy levels
	resources := []*MockResource{
		createMockResource("low", zone, 100*Joule), // Lowest energy - should be evicted
		createMockResource("medium", zone, 500*Joule),
		createMockResource("high", zone, 1000*Joule), // Highest energy
	}

	for _, resource := range resources {
		tracker.Add(resource)
	}
	assert.Equal(t, maxSize, tracker.Size())

	// Add a new resource with higher energy than the lowest
	newResource := createMockResource("new-medium", zone, 300*Joule)
	tracker.Add(newResource)

	// Should still be at capacity
	assert.Equal(t, maxSize, tracker.Size())

	// Check that the lowest energy resource was evicted
	items := tracker.Items()
	ids := make(map[string]bool)
	for _, item := range items {
		ids[item.StringID()] = true
	}

	assert.False(t, ids["low"], "Lowest energy resource should be evicted")
	assert.True(t, ids["medium"], "Medium energy resource should remain")
	assert.True(t, ids["high"], "High energy resource should remain")
	assert.True(t, ids["new-medium"], "New medium energy resource should be added")
}

// TestTerminatedResourceTracker_CapacityEvictionWithLowerEnergy validates that
// when the capacity is reached, and a new low energy resource is asked to be added,
// it will be ignored.
func TestTerminatedResourceTracker_CapacityEvictionWithLowerEnergy(t *testing.T) {
	zones := CreateTestZones()
	zone := zones[0]
	maxSize := 2
	tracker := NewTerminatedResourceTracker[*MockResource](zone, maxSize, 9*Joule, slog.Default().With("test", "tracker"))

	// Fill to capacity
	resources := []*MockResource{
		createMockResource("high1", zone, 1000*Joule),
		createMockResource("high2", zone, 2000*Joule),
	}

	for _, resource := range resources {
		tracker.Add(resource)
	}
	assert.Equal(t, maxSize, tracker.Size())

	// Try to add a resource with lower energy than any existing
	lowResource := createMockResource("low", zone, 50*Joule)
	tracker.Add(lowResource)

	// Should still be at capacity and the low energy resource should not be added
	assert.Equal(t, maxSize, tracker.Size())

	items := tracker.Items()
	ids := make(map[string]bool)
	for _, item := range items {
		ids[item.StringID()] = true
	}

	assert.True(t, ids["high1"], "High energy resource 1 should remain")
	assert.True(t, ids["high2"], "High energy resource 2 should remain")
	assert.False(t, ids["low"], "Low energy resource should not be added")
}

func TestTerminatedResourceTracker_Clear(t *testing.T) {
	zones := CreateTestZones()
	zone := zones[0]
	tracker := NewTerminatedResourceTracker[*MockResource](zone, 5, 12*Joule, slog.Default())

	// Add some resources
	resources := []*MockResource{
		createMockResource("resource-1", zone, 1000*Joule),
		createMockResource("resource-2", zone, 2000*Joule),
	}

	for _, resource := range resources {
		tracker.Add(resource)
	}
	assert.Equal(t, 2, tracker.Size())

	// Clear the tracker
	tracker.Clear()

	assert.Equal(t, 0, tracker.Size())
	assert.Equal(t, 0, len(tracker.Items()))
}

func TestTerminatedResourceTracker_MultiZoneResource(t *testing.T) {
	zones := CreateTestZones()
	trackedZone := zones[0]
	otherZone := zones[1]
	tracker := NewTerminatedResourceTracker[*MockResource](trackedZone, 5, 15*Joule, slog.Default())

	// Create a resource with energy in multiple zones
	zoneEnergies := map[device.EnergyZone]Energy{
		trackedZone: 1000 * Joule, // This is the zone the tracker cares about
		otherZone:   5000 * Joule, // Higher energy, but tracker doesn't use this zone
	}
	resource := createMockResourceMultiZone("multi-zone", zoneEnergies)

	tracker.Add(resource)

	assert.Equal(t, 1, tracker.Size())
	items := tracker.Items()
	require.Len(t, items, 1)
	assert.Contains(t, items, "multi-zone")

	// Verify the resource has the expected energy values
	resourceZones := items["multi-zone"].ZoneUsage()
	assert.Equal(t, Energy(1000*Joule), resourceZones[trackedZone].EnergyTotal)
	assert.Equal(t, Energy(5000*Joule), resourceZones[otherZone].EnergyTotal)
}

func TestTerminatedResourceTracker_String(t *testing.T) {
	zones := CreateTestZones()
	zone := zones[0]

	t.Run("normal capacity", func(t *testing.T) {
		tracker := NewTerminatedResourceTracker[*MockResource](zone, 10, 18*Joule, slog.Default())

		// Add a few resources
		tracker.Add(createMockResource("resource-1", zone, 1000*Joule))
		tracker.Add(createMockResource("resource-2", zone, 2000*Joule))

		str := tracker.String()
		assert.Contains(t, str, "2/10")      // size/maxSize
		assert.Contains(t, str, zone.Name()) // zone name
	})

	t.Run("disabled capacity", func(t *testing.T) {
		tracker := NewTerminatedResourceTracker[*MockResource](zone, 0, 0*Joule, slog.Default())

		str := tracker.String()
		assert.Contains(t, str, "0/disabled") // size/maxSize for disabled
		assert.Contains(t, str, zone.Name())  // zone name
	})

	t.Run("unlimited capacity", func(t *testing.T) {
		tracker := NewTerminatedResourceTracker[*MockResource](zone, -1, 0*Joule, slog.Default())

		str := tracker.String()
		assert.Contains(t, str, "0/unlimited") // size/maxSize for unlimited
		assert.Contains(t, str, zone.Name())   // zone name
	})
}

func TestTerminatedResourceTracker_EdgeCases(t *testing.T) {
	zones := CreateTestZones()
	zone := zones[0]

	t.Run("zero capacity tracker - feature disabled", func(t *testing.T) {
		tracker := NewTerminatedResourceTracker[*MockResource](zone, 0, 0*Joule, slog.Default())
		resource := createMockResource("resource-1", zone, 1000*Joule)

		tracker.Add(resource)

		assert.Equal(t, 0, tracker.Size())
		assert.Equal(t, 0, len(tracker.Items()))
	})

	t.Run("negative capacity tracker is unlimited", func(t *testing.T) {
		tracker := NewTerminatedResourceTracker[*MockResource](zone, -5, 0*Joule, slog.Default())

		// Add many resources beyond what would normally be capacity
		for i := 0; i < 100; i++ {
			resource := createMockResource(fmt.Sprintf("resource-%d", i), zone, Energy(i+1)*Joule)
			tracker.Add(resource)
		}

		assert.Equal(t, 100, tracker.Size())
		assert.Equal(t, 100, len(tracker.Items()))
		assert.Equal(t, -5, tracker.MaxSize())
	})

	t.Run("unlimited capacity tracker", func(t *testing.T) {
		tracker := NewTerminatedResourceTracker[*MockResource](zone, -1, 0*Joule, slog.Default())

		// Add many resources beyond what would normally be capacity
		for i := 0; i < 100; i++ {
			resource := createMockResource(fmt.Sprintf("resource-%d", i), zone, Energy(i+1)*Joule)
			tracker.Add(resource)
		}

		assert.Equal(t, 100, tracker.Size())
		assert.Equal(t, 100, len(tracker.Items()))
		assert.Equal(t, -1, tracker.MaxSize())
	})

	t.Run("capacity of 1", func(t *testing.T) {
		tracker := NewTerminatedResourceTracker[*MockResource](zone, 1, 0*Joule, slog.Default())

		// Add first resource
		resource1 := createMockResource("resource-1", zone, 1000*Joule)
		tracker.Add(resource1)
		assert.Equal(t, 1, tracker.Size())

		// Add second resource with higher energy - should replace first
		resource2 := createMockResource("resource-2", zone, 2000*Joule)
		tracker.Add(resource2)
		assert.Equal(t, 1, tracker.Size())

		items := tracker.Items()
		require.Len(t, items, 1)
		assert.Contains(t, items, "resource-2")
	})

	t.Run("empty resource ID", func(t *testing.T) {
		tracker := NewTerminatedResourceTracker[*MockResource](zone, 5, 0*Joule, slog.Default())
		resource := createMockResource("", zone, 1000*Joule)

		tracker.Add(resource)

		assert.Equal(t, 1, tracker.Size())
		items := tracker.Items()
		require.Len(t, items, 1)
		assert.Contains(t, items, "")
	})
}

func TestTerminatedResourceTracker_HeapIntegrity(t *testing.T) {
	zones := CreateTestZones()
	zone := zones[0]
	tracker := NewTerminatedResourceTracker[*MockResource](zone, 5, 20*Joule, slog.Default())

	// Add resources in various orders to test heap integrity
	energies := []Energy{500 * Joule, 1000 * Joule, 100 * Joule, 2000 * Joule, 300 * Joule}

	for i, energy := range energies {
		resource := createMockResource(
			fmt.Sprintf("resource-%d", i),
			zone,
			energy,
		)
		tracker.Add(resource)
	}

	assert.Equal(t, 5, tracker.Size())
	assert.Equal(t, 5, len(tracker.Items()))

	// Test that all resources are retrievable
	items := tracker.Items()
	totalEnergy := Energy(0)
	for _, item := range items {
		totalEnergy += item.ZoneUsage()[zone].EnergyTotal
	}

	expectedTotal := Energy(500+1000+100+2000+300) * Joule
	assert.Equal(t, expectedTotal, totalEnergy)
}

// Integration test that mimics the real usage pattern
func TestTerminatedResourceTracker_RealWorldScenario(t *testing.T) {
	zones := CreateTestZones()
	zone := zones[0]
	maxTerminated := 100
	tracker := NewTerminatedResourceTracker[*MockResource](zone, maxTerminated, 25*Joule, slog.Default())

	// Simulate adding many terminated processes over time
	processCount := 0

	// First batch: Add 50 processes with varying energy
	for i := 0; i < 50; i++ {
		energy := Energy((i + 1) * 100 * int(Joule)) // 100, 200, 300, ... 5000 Joules
		resource := createMockResource(
			fmt.Sprintf("process-%d", processCount),
			zone,
			energy,
		)
		tracker.Add(resource)
		processCount++
	}

	assert.Equal(t, 50, tracker.Size())

	// Second batch: Add 75 more processes (some will evict lower energy ones)
	for i := 0; i < 75; i++ {
		energy := Energy((i + 1) * 50 * int(Joule)) // 50, 100, 150, ... 3750 Joules
		resource := createMockResource(
			fmt.Sprintf("process-%d", processCount),
			zone,
			energy,
		)
		tracker.Add(resource)
		processCount++
	}

	// Should be at max capacity
	assert.Equal(t, maxTerminated, tracker.Size())

	// Verify that the highest energy processes are retained
	items := tracker.Items()
	minEnergy := Energy(^uint64(0)) // Max uint64 as starting point for finding min
	for _, item := range items {
		energy := item.ZoneUsage()[zone].EnergyTotal
		if energy < minEnergy {
			minEnergy = energy
		}
	}

	// The minimum energy in the tracker should be reasonably high
	// (exact value depends on the eviction logic, but it should be > 0)
	assert.Greater(t, minEnergy, Energy(0))

	// Clear and verify
	tracker.Clear()
	assert.Equal(t, 0, tracker.Size())
	assert.Equal(t, 0, len(tracker.Items()))
}

// TestTerminatedResourceTracker_MinimumThreshold tests the minimum energy threshold functionality
func TestTerminatedResourceTracker_MinimumThreshold(t *testing.T) {
	zones := CreateTestZones()
	zone := zones[0]

	t.Run("resources below threshold are rejected", func(t *testing.T) {
		minThreshold := 100 * Joule
		tracker := NewTerminatedResourceTracker[*MockResource](zone, 10, minThreshold, slog.Default())

		// Add resources below threshold - should be rejected
		belowThresholdResources := []*MockResource{
			createMockResource("below-1", zone, 50*Joule), // Below threshold
			createMockResource("below-2", zone, 99*Joule), // Just below threshold
			createMockResource("below-3", zone, 0*Joule),  // Zero energy
		}

		for _, resource := range belowThresholdResources {
			tracker.Add(resource)
		}

		// All resources should be rejected
		assert.Equal(t, 0, tracker.Size())
		assert.Equal(t, 0, len(tracker.Items()))
	})

	t.Run("resources above threshold are accepted", func(t *testing.T) {
		minThreshold := 100 * Joule
		tracker := NewTerminatedResourceTracker[*MockResource](zone, 10, minThreshold, slog.Default())

		// Add resources above threshold - should be accepted
		aboveThresholdResources := []*MockResource{
			createMockResource("above-1", zone, 100*Joule), // Exactly at threshold
			createMockResource("above-2", zone, 101*Joule), // Just above threshold
			createMockResource("above-3", zone, 500*Joule), // Well above threshold
		}

		for _, resource := range aboveThresholdResources {
			tracker.Add(resource)
		}

		// All resources should be accepted
		assert.Equal(t, 3, tracker.Size())
		items := tracker.Items()
		assert.Len(t, items, 3)

		// Verify all resources are present
		expectedIDs := []string{"above-1", "above-2", "above-3"}
		for _, id := range expectedIDs {
			assert.Contains(t, items, id)
		}
	})

	t.Run("mixed resources - some accepted, some rejected", func(t *testing.T) {
		minThreshold := 200 * Joule
		tracker := NewTerminatedResourceTracker[*MockResource](zone, 10, minThreshold, slog.Default())

		// Add mix of resources above and below threshold
		mixedResources := []*MockResource{
			createMockResource("reject-1", zone, 50*Joule),   // Below - reject
			createMockResource("accept-1", zone, 250*Joule),  // Above - accept
			createMockResource("reject-2", zone, 199*Joule),  // Below - reject
			createMockResource("accept-2", zone, 200*Joule),  // At threshold - accept
			createMockResource("reject-3", zone, 0*Joule),    // Zero - reject
			createMockResource("accept-3", zone, 1000*Joule), // Above - accept
		}

		for _, resource := range mixedResources {
			tracker.Add(resource)
		}

		// Only 3 resources should be accepted
		assert.Equal(t, 3, tracker.Size())
		items := tracker.Items()
		assert.Len(t, items, 3)

		// Verify only the accepted resources are present
		expectedAcceptedIDs := []string{"accept-1", "accept-2", "accept-3"}
		for _, id := range expectedAcceptedIDs {
			assert.Contains(t, items, id)
		}

		// Verify rejected resources are not present
		rejectedIDs := []string{"reject-1", "reject-2", "reject-3"}
		for _, id := range rejectedIDs {
			assert.NotContains(t, items, id)
		}
	})

	t.Run("threshold with capacity eviction", func(t *testing.T) {
		minThreshold := 100 * Joule
		maxCapacity := 2
		tracker := NewTerminatedResourceTracker[*MockResource](zone, maxCapacity, minThreshold, slog.Default())

		// Add resources that meet threshold but test capacity eviction
		resources := []*MockResource{
			createMockResource("low-valid", zone, 120*Joule), // Above threshold, but lowest
			createMockResource("high", zone, 500*Joule),      // High energy
			createMockResource("medium", zone, 300*Joule),    // Medium energy
		}

		for _, resource := range resources {
			tracker.Add(resource)
		}

		// Should be at capacity, with lowest energy resource evicted
		assert.Equal(t, maxCapacity, tracker.Size())
		items := tracker.Items()
		assert.Len(t, items, 2)

		// Verify the highest energy resources are retained
		assert.Contains(t, items, "high")
		assert.Contains(t, items, "medium")
		assert.NotContains(t, items, "low-valid")
	})

	t.Run("threshold with multi-zone resource", func(t *testing.T) {
		trackedZone := zones[0]
		otherZone := zones[1]
		minThreshold := 150 * Joule
		tracker := NewTerminatedResourceTracker[*MockResource](trackedZone, 10, minThreshold, slog.Default())

		// Create resources with energy in multiple zones
		tt := []struct {
			name          string
			trackedEnergy Energy
			otherEnergy   Energy
			shouldBeAdded bool
		}{
			{
				name:          "tracked-zone-above-threshold",
				trackedEnergy: 200 * Joule, // Above threshold in tracked zone
				otherEnergy:   50 * Joule,  // Below threshold in other zone
				shouldBeAdded: true,
			},
			{
				name:          "tracked-zone-below-threshold",
				trackedEnergy: 100 * Joule,  // Below threshold in tracked zone
				otherEnergy:   1000 * Joule, // Above threshold in other zone
				shouldBeAdded: false,
			},
			{
				name:          "tracked-zone-at-threshold",
				trackedEnergy: 150 * Joule, // At threshold in tracked zone
				otherEnergy:   0 * Joule,   // Zero in other zone
				shouldBeAdded: true,
			},
		}

		for _, tc := range tt {
			t.Run(tc.name, func(t *testing.T) {
				// Clear tracker for each sub-test
				tracker.Clear()

				zoneEnergies := map[device.EnergyZone]Energy{
					trackedZone: tc.trackedEnergy,
					otherZone:   tc.otherEnergy,
				}
				resource := createMockResourceMultiZone(tc.name, zoneEnergies)
				tracker.Add(resource)

				if tc.shouldBeAdded {
					assert.Equal(t, 1, tracker.Size())
					items := tracker.Items()
					assert.Contains(t, items, tc.name)
				} else {
					assert.Equal(t, 0, tracker.Size())
					items := tracker.Items()
					assert.NotContains(t, items, tc.name)
				}
			})
		}
	})
}

// TestTerminatedResourceTracker_ThresholdEdgeCases tests edge cases and boundary conditions for the threshold
func TestTerminatedResourceTracker_ThresholdEdgeCases(t *testing.T) {
	zones := CreateTestZones()
	zone := zones[0]

	t.Run("zero threshold accepts all resources", func(t *testing.T) {
		tracker := NewTerminatedResourceTracker[*MockResource](zone, 10, 0*Joule, slog.Default())

		// Add resources with various energies including zero
		resources := []*MockResource{
			createMockResource("zero", zone, 0*Joule),
			createMockResource("tiny", zone, 1*Joule),
			createMockResource("small", zone, 10*Joule),
			createMockResource("large", zone, 1000*Joule),
		}

		for _, resource := range resources {
			tracker.Add(resource)
		}

		// All resources should be accepted with zero threshold
		assert.Equal(t, 4, tracker.Size())
		items := tracker.Items()
		assert.Len(t, items, 4)

		for _, resource := range resources {
			assert.Contains(t, items, resource.StringID())
		}
	})

	t.Run("very high threshold rejects most resources", func(t *testing.T) {
		veryHighThreshold := 10000 * Joule
		tracker := NewTerminatedResourceTracker[*MockResource](zone, 10, veryHighThreshold, slog.Default())

		// Add resources with various energies
		resources := []*MockResource{
			createMockResource("low", zone, 1000*Joule),        // Below threshold
			createMockResource("medium", zone, 5000*Joule),     // Below threshold
			createMockResource("high", zone, 9999*Joule),       // Just below threshold
			createMockResource("accepted", zone, 10000*Joule),  // At threshold
			createMockResource("very-high", zone, 15000*Joule), // Above threshold
		}

		for _, resource := range resources {
			tracker.Add(resource)
		}

		// Only 2 resources should be accepted
		assert.Equal(t, 2, tracker.Size())
		items := tracker.Items()
		assert.Len(t, items, 2)

		// Verify only the high-energy resources are present
		assert.Contains(t, items, "accepted")
		assert.Contains(t, items, "very-high")
		assert.NotContains(t, items, "low")
		assert.NotContains(t, items, "medium")
		assert.NotContains(t, items, "high")
	})

	t.Run("threshold with disabled tracker", func(t *testing.T) {
		tracker := NewTerminatedResourceTracker[*MockResource](zone, 0, 100*Joule, slog.Default())

		// Add a resource that would meet the threshold
		resource := createMockResource("valid", zone, 200*Joule)
		tracker.Add(resource)

		// Should still be rejected because tracker is disabled (capacity = 0)
		assert.Equal(t, 0, tracker.Size())
		assert.Equal(t, 0, len(tracker.Items()))
	})

	t.Run("threshold with unlimited capacity", func(t *testing.T) {
		tracker := NewTerminatedResourceTracker[*MockResource](zone, -1, 50*Joule, slog.Default())

		// Add many resources, some above and some below threshold
		var acceptedCount int
		for i := 0; i < 100; i++ {
			energy := Energy(i * 10 * int(Joule)) // 0, 10, 20, ... 990 Joules
			resource := createMockResource(fmt.Sprintf("resource-%d", i), zone, energy)
			tracker.Add(resource)

			if energy >= 50*Joule {
				acceptedCount++
			}
		}

		// Should accept all resources that meet the threshold
		assert.Equal(t, acceptedCount, tracker.Size())
		assert.Equal(t, acceptedCount, len(tracker.Items()))
	})

	t.Run("exact threshold boundary values", func(t *testing.T) {
		exactThreshold := 500 * Joule
		tracker := NewTerminatedResourceTracker[*MockResource](zone, 10, exactThreshold, slog.Default())

		// Test boundary values around the exact threshold
		tt := []struct {
			name         string
			energy       Energy
			shouldAccept bool
		}{
			{"below-by-1", 499 * Joule, false},
			{"exact-threshold", 500 * Joule, true},
			{"above-by-1", 501 * Joule, true},
		}

		for _, tc := range tt {
			t.Run(tc.name, func(t *testing.T) {
				// Clear tracker for each sub-test
				tracker.Clear()

				resource := createMockResource(tc.name, zone, tc.energy)
				tracker.Add(resource)

				if tc.shouldAccept {
					assert.Equal(t, 1, tracker.Size(), "Resource with energy %v should be accepted", tc.energy)
					items := tracker.Items()
					assert.Contains(t, items, tc.name)
				} else {
					assert.Equal(t, 0, tracker.Size(), "Resource with energy %v should be rejected", tc.energy)
					items := tracker.Items()
					assert.NotContains(t, items, tc.name)
				}
			})
		}
	})

	t.Run("threshold validation with resource missing tracked zone", func(t *testing.T) {
		trackedZone := zones[0]
		otherZone := zones[1]
		tracker := NewTerminatedResourceTracker[*MockResource](trackedZone, 10, 100*Joule, slog.Default())

		// Create a resource that only has energy in the non-tracked zone
		resource := createMockResource("missing-zone", otherZone, 1000*Joule)
		tracker.Add(resource)

		// Should be rejected because it has no energy in the tracked zone
		assert.Equal(t, 0, tracker.Size())
		assert.Equal(t, 0, len(tracker.Items()))
	})
}
