// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package device

import (
	"fmt"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockEnergyZone implements EnergyZone for testing
type mockEnergyZone struct {
	name      string
	index     int
	path      string
	energy    Energy
	maxEnergy Energy
	err       error
}

func (m *mockEnergyZone) Name() string            { return m.name }
func (m *mockEnergyZone) Index() int              { return m.index }
func (m *mockEnergyZone) Path() string            { return m.path }
func (m *mockEnergyZone) Energy() (Energy, error) { return m.energy, m.err }
func (m *mockEnergyZone) MaxEnergy() Energy       { return m.maxEnergy }

func TestNewAggregatedZone(t *testing.T) {
	zones := []EnergyZone{
		&mockEnergyZone{name: "package", index: 0},
		&mockEnergyZone{name: "package", index: 1},
	}

	az := NewAggregatedZone("package", zones)

	assert.Equal(t, "package", az.Name())
	assert.Equal(t, -1, az.Index())
	assert.Equal(t, "aggregated-package", az.Path())
	assert.Len(t, az.zones, 2)
	assert.NotNil(t, az.zoneStates)
}

func TestAggregatedZone_Energy_BasicAggregation(t *testing.T) {
	zones := []EnergyZone{
		&mockEnergyZone{name: "package", index: 0, energy: 100, maxEnergy: 1000},
		&mockEnergyZone{name: "package", index: 1, energy: 200, maxEnergy: 1000},
	}

	az := NewAggregatedZone("package", zones)

	energy, err := az.Energy()
	require.NoError(t, err)
	assert.Equal(t, Energy(300), energy) // 100 + 200
}

func TestAggregatedZone_Energy_SingleZone(t *testing.T) {
	zones := []EnergyZone{
		&mockEnergyZone{name: "package", index: 0, energy: 150, maxEnergy: 1000},
	}

	az := NewAggregatedZone("package", zones)

	energy, err := az.Energy()
	require.NoError(t, err)
	assert.Equal(t, Energy(150), energy)
}

func TestAggregatedZone_Energy_WrapDetection(t *testing.T) {
	zone0 := &mockEnergyZone{name: "package", index: 0, energy: 900, maxEnergy: 1000}
	zone1 := &mockEnergyZone{name: "package", index: 1, energy: 800, maxEnergy: 1000}
	zones := []EnergyZone{zone0, zone1}

	az := NewAggregatedZone("package", zones)

	// First reading: 900 + 800 = 1700 (initial accumulated energy)
	energy1, err := az.Energy()
	require.NoError(t, err)
	assert.Equal(t, Energy(1700), energy1)

	// Simulate wrap on zone0: 900 -> 100 (wrapped)
	// Delta for zone0: (1000-900) + 100 = 200
	// Zone1 continues: 800 -> 850 (delta: 50)
	zone0.energy = 100
	zone1.energy = 850

	// Second reading: (900+200) + (800+50) = 1950
	energy2, err := az.Energy()
	require.NoError(t, err)
	assert.Equal(t, Energy(1950), energy2)
}

func TestAggregatedZone_Energy_MultipleWraps(t *testing.T) {
	zone := &mockEnergyZone{name: "package", index: 0, energy: 900, maxEnergy: 1000}
	zones := []EnergyZone{zone}

	az := NewAggregatedZone("package", zones)

	// First reading: 900 (initial accumulated energy)
	energy1, err := az.Energy()
	require.NoError(t, err)
	assert.Equal(t, Energy(900), energy1)

	// First wrap: 900 -> 100
	// Delta: (1000-900) + 100 = 200
	// Total: 900 + 200 = 1100, but MaxEnergy is 1000, so wraps to 100
	zone.energy = 100
	energy2, err := az.Energy()
	require.NoError(t, err)
	assert.Equal(t, Energy(100), energy2) // 1100 % 1000 = 100

	// Second wrap: 100 -> 50
	// Delta: (1000-100) + 50 = 950
	// Total: 100 + 950 = 1050, wraps to 50
	zone.energy = 50
	energy3, err := az.Energy()
	require.NoError(t, err)
	assert.Equal(t, Energy(50), energy3) // 1050 % 1000 = 50
}

func TestAggregatedZone_Energy_ErrorHandling(t *testing.T) {
	zones := []EnergyZone{
		&mockEnergyZone{name: "package", index: 0, energy: 100, maxEnergy: 1000, err: fmt.Errorf("read error")},
		&mockEnergyZone{name: "package", index: 1, energy: 200, maxEnergy: 1000},
	}

	az := NewAggregatedZone("package", zones)

	// Should continue with valid zones when one fails
	energy, err := az.Energy()
	require.NoError(t, err)
	assert.Equal(t, Energy(200), energy) // Only zone1 contributes
}

func TestAggregatedZone_Energy_AllZonesError(t *testing.T) {
	zones := []EnergyZone{
		&mockEnergyZone{name: "package", index: 0, energy: 100, maxEnergy: 1000, err: fmt.Errorf("error1")},
		&mockEnergyZone{name: "package", index: 1, energy: 200, maxEnergy: 1000, err: fmt.Errorf("error2")},
	}

	az := NewAggregatedZone("package", zones)

	energy, err := az.Energy()
	assert.Error(t, err)
	assert.Equal(t, Energy(0), energy)
	assert.Contains(t, err.Error(), "no valid energy readings")
}

func TestAggregatedZone_MaxEnergy(t *testing.T) {
	zones := []EnergyZone{
		&mockEnergyZone{name: "package", index: 0, maxEnergy: 1000},
		&mockEnergyZone{name: "package", index: 1, maxEnergy: 1000},
	}

	az := NewAggregatedZone("package", zones)

	// Should return sum of all zone MaxEnergy values
	assert.Equal(t, Energy(2000), az.MaxEnergy())
}

func TestAggregatedZone_MaxEnergy_EmptyZones(t *testing.T) {
	az := NewAggregatedZone("package", []EnergyZone{})
	assert.Equal(t, Energy(0), az.MaxEnergy())
}

func TestAggregatedZone_ConcurrentAccess(t *testing.T) {
	zones := []EnergyZone{
		&mockEnergyZone{name: "package", index: 0, energy: 100, maxEnergy: 1000},
		&mockEnergyZone{name: "package", index: 1, energy: 200, maxEnergy: 1000},
	}

	az := NewAggregatedZone("package", zones)

	// Test concurrent access doesn't cause race conditions
	done := make(chan bool, 10)
	for range 10 {
		go func() {
			_, err := az.Energy()
			assert.NoError(t, err)
			done <- true
		}()
	}

	for range 10 {
		<-done
	}
}

func TestAggregatedZone_ZoneIDGeneration(t *testing.T) {
	zone := &mockEnergyZone{name: "package", index: 0, energy: 100, maxEnergy: 1000}
	zones := []EnergyZone{zone}

	az := NewAggregatedZone("package", zones)

	// First call should create state
	_, err := az.Energy()
	require.NoError(t, err)
	assert.Len(t, az.zoneStates, 1)

	// Verify zone ID format
	expectedZoneID := "package-0"
	_, exists := az.zoneStates[expectedZoneID]
	assert.True(t, exists, "Expected zone ID %s to exist", expectedZoneID)
}

func TestAggregatedZone_StateManagement(t *testing.T) {
	zone := &mockEnergyZone{name: "package", index: 0, energy: 500, maxEnergy: 1000}
	zones := []EnergyZone{zone}

	az := NewAggregatedZone("package", zones)

	// First reading should initialize state
	_, err := az.Energy()
	require.NoError(t, err)

	zoneID := "package-0"
	state := az.zoneStates[zoneID]
	assert.Equal(t, Energy(500), state.lastReading)
	assert.Equal(t, Energy(500), state.accumulatedEnergy) // Initial accumulated energy

	// Update energy and read again
	zone.energy = 600
	_, err = az.Energy()
	require.NoError(t, err)

	// State should be updated
	assert.Equal(t, Energy(600), state.lastReading)
	assert.Equal(t, Energy(600), state.accumulatedEnergy) // 500 + 100 delta

	// Simulate wrap
	zone.energy = 100
	_, err = az.Energy()
	require.NoError(t, err)

	// State should reflect wrap
	assert.Equal(t, Energy(100), state.lastReading)
	assert.Equal(t, Energy(1100), state.accumulatedEnergy) // 600 + 500 delta from wrap
}

func TestAggregatedZone_Energy_AggregatedWrapping(t *testing.T) {
	// Test case where aggregated total wraps at combined MaxEnergy
	zone0 := &mockEnergyZone{name: "package", index: 0, energy: 800, maxEnergy: 1000}
	zone1 := &mockEnergyZone{name: "package", index: 1, energy: 900, maxEnergy: 1000}
	zones := []EnergyZone{zone0, zone1}

	az := NewAggregatedZone("package", zones)

	// First reading: 800 + 900 = 1700
	energy1, err := az.Energy()
	require.NoError(t, err)
	assert.Equal(t, Energy(1700), energy1)

	// Update both zones: 800->900 (delta:100) + 900->950 (delta:50)
	// Total: 1700 + 150 = 1850
	zone0.energy = 900
	zone1.energy = 950
	energy2, err := az.Energy()
	require.NoError(t, err)
	assert.Equal(t, Energy(1850), energy2)

	// Update to cause aggregated wrap: total = 1850 + 200 = 2050
	// MaxEnergy = 2000, so should wrap to 50
	zone0.energy = 950 // delta: 50
	zone1.energy = 100 // wrapped: delta = (1000-950) + 100 = 150
	energy3, err := az.Energy()
	require.NoError(t, err)
	assert.Equal(t, Energy(50), energy3) // 2050 % 2000 = 50
}

func TestAggregatedZone_Energy_MaximumValues(t *testing.T) {
	// Test with very large Energy values to ensure proper handling
	const maxUint64 = ^uint64(0) // Maximum uint64 value
	maxEnergy := Energy(maxUint64)

	// Create zones with maximum possible MaxEnergy and high current energy
	// Use values that won't cause overflow in test calculations
	zone0Energy := Energy(maxUint64/2 - 1000) // Large but safe value
	zone1Energy := Energy(maxUint64/2 - 2000) // Large but safe value

	zone0 := &mockEnergyZone{
		name:      "package",
		index:     0,
		energy:    zone0Energy,
		maxEnergy: maxEnergy,
	}
	zone1 := &mockEnergyZone{
		name:      "package",
		index:     1,
		energy:    zone1Energy,
		maxEnergy: maxEnergy,
	}
	zones := []EnergyZone{zone0, zone1}

	az := NewAggregatedZone("package", zones)

	// First reading: should handle large values without overflow
	energy1, err := az.Energy()
	require.NoError(t, err)
	// The sum should be manageable and not cause overflow
	expected1 := zone0Energy + zone1Energy
	assert.Equal(t, expected1, energy1)

	// Simulate small increments on both zones
	zone0.energy = zone0Energy + 500 // delta: 500
	zone1.energy = zone1Energy + 500 // delta: 500
	energy2, err := az.Energy()
	require.NoError(t, err)

	// The accumulated energy should properly handle the deltas
	// Each zone accumulated 500 more, so total delta is 1000
	expected2 := expected1 + 1000
	assert.Equal(t, expected2, energy2)
}

func TestAggregatedZone_Energy_MaximumValueWrapping(t *testing.T) {
	// Test wrapping behavior when individual zones are at very high values
	const maxUint64 = math.MaxUint64
	maxEnergy := Energy(maxUint64) // Use a reasonable max energy for testing

	zone := &mockEnergyZone{
		name:      "package",
		index:     0,
		energy:    Energy(maxEnergy - 100), // Very close to zone maximum
		maxEnergy: maxEnergy,
	}
	zones := []EnergyZone{zone}

	az := NewAggregatedZone("package", zones)

	// First reading
	energy1, err := az.Energy()
	require.NoError(t, err)
	assert.Equal(t, Energy(maxEnergy-100), energy1)

	// Simulate wrap: (maxEnergy-100) -> 50
	// Delta should be: (maxEnergy - (maxEnergy-100)) + 50 = 100 + 50 = 150
	zone.energy = 50
	energy2, err := az.Energy()
	require.NoError(t, err)

	// Total: (maxEnergy-100) + 150 = maxEnergy + 50
	// Since this exceeds maxEnergy, it should wrap: (maxEnergy + 50) % maxEnergy = 50
	expected := Energy(50)
	assert.Equal(t, expected, energy2)
}

func TestAggregatedZone_Energy_OverflowProtection(t *testing.T) {
	// Test that aggregation handles potential overflow scenarios
	const largeMax = Energy(^uint64(0) >> 1) // Half of max uint64 to avoid overflow in calculations

	// Create two zones with large MaxEnergy values
	zone0 := &mockEnergyZone{
		name:      "package",
		index:     0,
		energy:    largeMax - 1000,
		maxEnergy: largeMax,
	}
	zone1 := &mockEnergyZone{
		name:      "package",
		index:     1,
		energy:    largeMax - 2000,
		maxEnergy: largeMax,
	}
	zones := []EnergyZone{zone0, zone1}

	az := NewAggregatedZone("package", zones)

	// First reading should work without overflow
	energy1, err := az.Energy()
	require.NoError(t, err)

	// The total should be properly calculated and wrapped if needed
	combinedMax := 2 * largeMax
	rawTotal := (largeMax - 1000) + (largeMax - 2000) // This should not overflow
	expected := rawTotal % combinedMax
	assert.Equal(t, expected, energy1)

	// Test incremental updates
	zone0.energy = largeMax - 500  // delta: 500
	zone1.energy = largeMax - 1500 // delta: 500
	energy2, err := az.Energy()
	require.NoError(t, err)

	// Total delta should be 1000, properly wrapped
	expected2 := (expected + 1000) % combinedMax
	assert.Equal(t, expected2, energy2)
}

func TestAggregatedZone_CachedMaxEnergy(t *testing.T) {
	// Test that MaxEnergy returns the correct cached value
	zones := []EnergyZone{
		&mockEnergyZone{name: "package", index: 0, energy: 100, maxEnergy: 1000},
		&mockEnergyZone{name: "package", index: 1, energy: 200, maxEnergy: 1000},
		&mockEnergyZone{name: "package", index: 2, energy: 300, maxEnergy: 1000},
	}

	az := NewAggregatedZone("package", zones)

	// MaxEnergy should return the sum of all zone MaxEnergy values
	expectedMaxEnergy := Energy(3000) // 1000 + 1000 + 1000

	// Multiple calls should return the same cached value consistently
	maxEnergy1 := az.MaxEnergy()
	maxEnergy2 := az.MaxEnergy()
	maxEnergy3 := az.MaxEnergy()

	// All calls should return the same cached value
	assert.Equal(t, expectedMaxEnergy, maxEnergy1)
	assert.Equal(t, expectedMaxEnergy, maxEnergy2)
	assert.Equal(t, expectedMaxEnergy, maxEnergy3)

	// Verify the cached value is used in Energy() wrapping calculation
	_, err := az.Energy()
	require.NoError(t, err)

	// MaxEnergy should still return the same cached value after Energy() calls
	maxEnergyAfter := az.MaxEnergy()
	assert.Equal(t, expectedMaxEnergy, maxEnergyAfter)
}

func TestAggregatedZone_MaxEnergyCaching_EmptyZones(t *testing.T) {
	// Test that empty zones list results in zero MaxEnergy
	az := NewAggregatedZone("package", []EnergyZone{})

	// Should return 0 for empty zones
	assert.Equal(t, Energy(0), az.MaxEnergy())

	// Multiple calls should consistently return 0
	assert.Equal(t, Energy(0), az.MaxEnergy())
	assert.Equal(t, Energy(0), az.MaxEnergy())
}
