// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package device

import (
	"fmt"
	"math"
	"sync"
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
	mu        sync.RWMutex
}

func (m *mockEnergyZone) Name() string { return m.name }
func (m *mockEnergyZone) Index() int   { return m.index }
func (m *mockEnergyZone) Path() string { return m.path }
func (m *mockEnergyZone) Energy() (Energy, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.energy, m.err
}
func (m *mockEnergyZone) MaxEnergy() Energy { return m.maxEnergy }

// SetEnergy safely updates the energy value for testing
func (m *mockEnergyZone) SetEnergy(energy Energy) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.energy = energy
}

// TestNewAggregatedZone tests constructor validation and basic properties
func TestNewAggregatedZone(t *testing.T) {
	zones := []EnergyZone{
		&mockEnergyZone{name: "package", index: 0},
		&mockEnergyZone{name: "package", index: 1},
	}

	az := NewAggregatedZone(zones)

	assert.Equal(t, "package", az.Name())
	assert.Equal(t, -1, az.Index())
	assert.Equal(t, "aggregated-package", az.Path())
	assert.Len(t, az.zones, 2)
	assert.NotNil(t, az.lastReadings)

	// Test panic on empty zones
	assert.Panics(t, func() {
		NewAggregatedZone([]EnergyZone{})
	})
}

// TestAggregatedZone_EnergyAggregation tests core energy aggregation functionality
func TestAggregatedZone_EnergyAggregation(t *testing.T) {
	t.Run("BasicAggregation", func(t *testing.T) {
		zones := []EnergyZone{
			&mockEnergyZone{name: "package", index: 0, energy: 100, maxEnergy: 1000},
			&mockEnergyZone{name: "package", index: 1, energy: 200, maxEnergy: 1000},
		}

		az := NewAggregatedZone(zones)

		energy, err := az.Energy()
		require.NoError(t, err)
		assert.Equal(t, Energy(300), energy)          // 100 + 200
		assert.Equal(t, Energy(2000), az.MaxEnergy()) // 1000 + 1000
	})

	t.Run("SingleZone", func(t *testing.T) {
		zones := []EnergyZone{
			&mockEnergyZone{name: "package", index: 0, energy: 150, maxEnergy: 1000},
		}

		az := NewAggregatedZone(zones)

		energy, err := az.Energy()
		require.NoError(t, err)
		assert.Equal(t, Energy(150), energy)
	})

	t.Run("FirstReadingCorrectness", func(t *testing.T) {
		// Test that first reading doesn't double-count
		zone := &mockEnergyZone{name: "package", index: 0, energy: 100, maxEnergy: 1000}
		zones := []EnergyZone{zone}

		az := NewAggregatedZone(zones)

		// First reading should return current energy
		energy1, err := az.Energy()
		require.NoError(t, err)
		assert.Equal(t, Energy(100), energy1)

		// Second reading with same energy should still return 100 (no delta)
		energy2, err := az.Energy()
		require.NoError(t, err)
		assert.Equal(t, Energy(100), energy2)

		// Third reading with increased energy should show the increase
		zone.SetEnergy(150)
		energy3, err := az.Energy()
		require.NoError(t, err)
		assert.Equal(t, Energy(150), energy3)
	})
}

// TestAggregatedZone_WrapHandling tests hardware counter wrapping scenarios
func TestAggregatedZone_WrapHandling(t *testing.T) {
	t.Run("MultiZoneWrap", func(t *testing.T) {
		zone0 := &mockEnergyZone{name: "package", index: 0, energy: 900, maxEnergy: 1000}
		zone1 := &mockEnergyZone{name: "package", index: 1, energy: 800, maxEnergy: 1000}
		zones := []EnergyZone{zone0, zone1}

		az := NewAggregatedZone(zones)

		// First reading: 900 + 800 = 1700
		energy1, err := az.Energy()
		require.NoError(t, err)
		assert.Equal(t, Energy(1700), energy1)

		// Simulate wrap on zone0: 900 -> 100, zone1: 800 -> 850
		zone0.SetEnergy(100) // wrapped: delta = (1000-900) + 100 = 200
		zone1.SetEnergy(850) // normal: delta = 50

		energy2, err := az.Energy()
		require.NoError(t, err)
		assert.Equal(t, Energy(1950), energy2) // 1700 + 200 + 50
	})

	t.Run("MultipleWraps", func(t *testing.T) {
		zone := &mockEnergyZone{name: "package", index: 0, energy: 900, maxEnergy: 1000}
		zones := []EnergyZone{zone}

		az := NewAggregatedZone(zones)

		// First reading: 900
		energy1, err := az.Energy()
		require.NoError(t, err)
		assert.Equal(t, Energy(900), energy1)

		// First wrap: 900 -> 100 (delta: 200)
		// Total: 900 + 200 = 1100, wraps to 100 (1100 % 1000 = 100)
		zone.SetEnergy(100)
		energy2, err := az.Energy()
		require.NoError(t, err)
		assert.Equal(t, Energy(100), energy2) // 1100 % 1000 = 100

		// Second wrap: 100 -> 50 (delta: 950)
		// Total: 100 + 950 = 1050, wraps to 50 (1050 % 1000 = 50)
		zone.SetEnergy(50)
		energy3, err := az.Energy()
		require.NoError(t, err)
		assert.Equal(t, Energy(50), energy3) // 1050 % 1000 = 50
	})

	t.Run("BackwardReading", func(t *testing.T) {
		// Test handling of readings that go backward (faulty hardware)
		zone := &mockEnergyZone{name: "package", index: 0, energy: 500, maxEnergy: 1000}
		zones := []EnergyZone{zone}

		az := NewAggregatedZone(zones)

		energy1, err := az.Energy()
		require.NoError(t, err)
		assert.Equal(t, Energy(500), energy1)

		// Simulate backward reading (treated as wrap)
		zone.SetEnergy(400)
		energy2, err := az.Energy()
		require.NoError(t, err)
		// Treated as wrap: (1000-500) + 400 = 900, total = 500 + 900 = 1400
		// Wraps: 1400 % 1000 = 400
		assert.Equal(t, Energy(400), energy2)
	})

	t.Run("ZeroMaxEnergyHandling", func(t *testing.T) {
		// Test safe handling of zero MaxEnergy
		zones := []EnergyZone{
			&mockEnergyZone{name: "package", index: 0, energy: 100, maxEnergy: 0},
			&mockEnergyZone{name: "package", index: 1, energy: 200, maxEnergy: 0},
		}

		az := NewAggregatedZone(zones)
		assert.Equal(t, Energy(0), az.MaxEnergy())

		// Should not panic with zero MaxEnergy
		energy, err := az.Energy()
		require.NoError(t, err)
		assert.Equal(t, Energy(300), energy)
	})
}

// TestAggregatedZone_ErrorHandling tests error conditions
func TestAggregatedZone_ErrorHandling(t *testing.T) {
	t.Run("SingleZoneError", func(t *testing.T) {
		zones := []EnergyZone{
			&mockEnergyZone{name: "package", index: 0, energy: 100, maxEnergy: 1000, err: fmt.Errorf("read error")},
			&mockEnergyZone{name: "package", index: 1, energy: 200, maxEnergy: 1000},
		}

		az := NewAggregatedZone(zones)

		energy, err := az.Energy()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no valid energy readings")
		assert.Zero(t, energy)
	})

	t.Run("AllZonesError", func(t *testing.T) {
		zones := []EnergyZone{
			&mockEnergyZone{name: "package", index: 0, energy: 100, maxEnergy: 1000, err: fmt.Errorf("error1")},
			&mockEnergyZone{name: "package", index: 1, energy: 200, maxEnergy: 1000, err: fmt.Errorf("error2")},
		}

		az := NewAggregatedZone(zones)

		energy, err := az.Energy()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no valid energy readings")
		assert.Zero(t, energy)
	})
}

// TestAggregatedZone_MaxEnergyHandling tests MaxEnergy calculation and edge cases
func TestAggregatedZone_MaxEnergyHandling(t *testing.T) {
	t.Run("BasicMaxEnergy", func(t *testing.T) {
		zones := []EnergyZone{
			&mockEnergyZone{name: "package", index: 0, maxEnergy: 1000},
			&mockEnergyZone{name: "package", index: 1, maxEnergy: 1000},
		}

		az := NewAggregatedZone(zones)
		assert.Equal(t, Energy(2000), az.MaxEnergy())
	})

	t.Run("MaxEnergyCaching", func(t *testing.T) {
		zones := []EnergyZone{
			&mockEnergyZone{name: "package", index: 0, energy: 100, maxEnergy: 1000},
			&mockEnergyZone{name: "package", index: 1, energy: 200, maxEnergy: 1000},
			&mockEnergyZone{name: "package", index: 2, energy: 300, maxEnergy: 1000},
		}

		az := NewAggregatedZone(zones)
		expectedMaxEnergy := Energy(3000)

		// Multiple calls should return the same cached value
		assert.Equal(t, expectedMaxEnergy, az.MaxEnergy())
		assert.Equal(t, expectedMaxEnergy, az.MaxEnergy())
		assert.Equal(t, expectedMaxEnergy, az.MaxEnergy())

		// Verify cached value is used after Energy() calls
		_, err := az.Energy()
		require.NoError(t, err)
		assert.Equal(t, expectedMaxEnergy, az.MaxEnergy())
	})

	t.Run("MaxEnergyOverflow", func(t *testing.T) {
		// Test overflow protection in MaxEnergy calculation
		largeMaxEnergy := Energy(math.MaxUint64 / 2)
		zones := []EnergyZone{
			&mockEnergyZone{name: "package", index: 0, energy: 100, maxEnergy: largeMaxEnergy},
			&mockEnergyZone{name: "package", index: 1, energy: 200, maxEnergy: largeMaxEnergy},
			&mockEnergyZone{name: "package", index: 2, energy: 300, maxEnergy: largeMaxEnergy},
		}

		az := NewAggregatedZone(zones)

		// Should not panic and should handle overflow gracefully
		maxEnergy := az.MaxEnergy()
		t.Logf("MaxEnergy with potential overflow: %d", maxEnergy)

		assert.NotPanics(t, func() {
			_, _ = az.Energy()
		})
	})
}

// TestAggregatedZone_LargeValueHandling tests behavior with large Energy values
func TestAggregatedZone_LargeValueHandling(t *testing.T) {
	t.Run("MaximumValues", func(t *testing.T) {
		// Test with very large Energy values
		const maxUint64 = ^uint64(0)
		maxEnergy := Energy(maxUint64)

		zone0Energy := Energy(maxUint64/2 - 1000)
		zone1Energy := Energy(maxUint64/2 - 2000)

		zone0 := &mockEnergyZone{
			name: "package", index: 0,
			energy:    zone0Energy,
			maxEnergy: maxEnergy,
		}
		zone1 := &mockEnergyZone{
			name: "package", index: 1,
			energy:    zone1Energy,
			maxEnergy: maxEnergy,
		}
		zones := []EnergyZone{zone0, zone1}

		az := NewAggregatedZone(zones)

		// Should handle large values without overflow
		energy1, err := az.Energy()
		require.NoError(t, err)
		expected1 := zone0Energy + zone1Energy
		assert.Equal(t, expected1, energy1)

		// Test incremental updates
		zone0.SetEnergy(zone0Energy + 500)
		zone1.SetEnergy(zone1Energy + 500)
		energy2, err := az.Energy()
		require.NoError(t, err)
		expected2 := expected1 + 1000
		assert.Equal(t, expected2, energy2)
	})

	t.Run("LargeValueWrapping", func(t *testing.T) {
		const maxEnergy = Energy(1000000)

		zone := &mockEnergyZone{
			name: "package", index: 0,
			energy:    Energy(maxEnergy - 100),
			maxEnergy: maxEnergy,
		}
		zones := []EnergyZone{zone}

		az := NewAggregatedZone(zones)

		energy1, err := az.Energy()
		require.NoError(t, err)
		assert.Equal(t, Energy(maxEnergy-100), energy1)

		// Simulate wrap: (maxEnergy-100) -> 50
		zone.SetEnergy(50)
		energy2, err := az.Energy()
		require.NoError(t, err)
		// Delta: (maxEnergy - (maxEnergy-100)) + 50 = 150
		// Total: (maxEnergy-100) + 150 = maxEnergy + 50
		// Wraps: (maxEnergy + 50) % maxEnergy = 50
		expected := Energy(50)
		assert.Equal(t, expected, energy2)
	})

	t.Run("DeltaOverflow", func(t *testing.T) {
		// Test potential overflow in delta calculations
		largeMaxEnergy := Energy(math.MaxUint64 - 1000)
		zone := &mockEnergyZone{
			name: "package", index: 0,
			energy:    Energy(1000),
			maxEnergy: largeMaxEnergy,
		}
		zones := []EnergyZone{zone}

		az := NewAggregatedZone(zones)

		energy1, err := az.Energy()
		require.NoError(t, err)
		assert.Equal(t, Energy(1000), energy1)

		// Simulate wrap that could cause overflow in delta calculation
		zone.SetEnergy(100)
		energy2, err := az.Energy()

		// Should not panic, but result might be large due to wrap calculation
		if err != nil {
			t.Logf("Error on potential overflow: %v", err)
		} else {
			t.Logf("Energy after potential overflow: %d", energy2)
		}

		assert.NotPanics(t, func() {
			_, _ = az.Energy()
		})
	})
}

// TestAggregatedZone_ConcurrentAccess tests thread safety and concurrent access
func TestAggregatedZone_ConcurrentAccess(t *testing.T) {
	t.Run("BasicConcurrentReads", func(t *testing.T) {
		zones := []EnergyZone{
			&mockEnergyZone{name: "package", index: 0, energy: 100, maxEnergy: 1000},
			&mockEnergyZone{name: "package", index: 1, energy: 200, maxEnergy: 1000},
		}

		az := NewAggregatedZone(zones)

		// Test concurrent reads don't cause race conditions
		done := make(chan bool, 10)
		for i := 0; i < 10; i++ {
			go func() {
				_, err := az.Energy()
				assert.NoError(t, err)
				done <- true
			}()
		}

		for i := 0; i < 10; i++ {
			<-done
		}
	})

	t.Run("ConcurrentReadsWithUpdates", func(t *testing.T) {
		zone := &mockEnergyZone{name: "package", index: 0, energy: 100, maxEnergy: 1000}
		zones := []EnergyZone{zone}

		az := NewAggregatedZone(zones)

		// Initialize state
		_, err := az.Energy()
		require.NoError(t, err)

		// Concurrent access with zone energy updates
		done := make(chan bool, 20)
		results := make(chan Energy, 20)

		// Goroutines that update and read
		for i := 0; i < 10; i++ {
			go func(energyValue Energy) {
				zone.SetEnergy(energyValue)
				energy, err := az.Energy()
				if err == nil {
					results <- energy
				}
				done <- true
			}(Energy(100 + i*50))
		}

		// Goroutines that just read
		for i := 0; i < 10; i++ {
			go func() {
				energy, err := az.Energy()
				if err == nil {
					results <- energy
				}
				done <- true
			}()
		}

		// Wait for all goroutines
		for i := 0; i < 20; i++ {
			<-done
		}
		close(results)

		// Collect results
		var energyValues []Energy
		for energy := range results {
			energyValues = append(energyValues, energy)
		}

		t.Logf("Concurrent energy readings: %v", energyValues)
		assert.NotEmpty(t, energyValues)
		assert.NotPanics(t, func() {
			_, _ = az.Energy()
		})
	})
}

// TestAggregatedZone_StateManagement tests internal state tracking
func TestAggregatedZone_StateManagement(t *testing.T) {
	t.Run("BasicStateTracking", func(t *testing.T) {
		zone := &mockEnergyZone{name: "package", index: 0, energy: 500, maxEnergy: 1000}
		zones := []EnergyZone{zone}

		az := NewAggregatedZone(zones)

		// First reading should initialize last reading
		_, err := az.Energy()
		require.NoError(t, err)

		zoneID := zoneKey{"package", 0}
		lastReading := az.lastReadings[zoneID]
		assert.Equal(t, Energy(500), lastReading)

		// Update energy and verify state tracking
		zone.SetEnergy(600)
		_, err = az.Energy()
		require.NoError(t, err)

		lastReading = az.lastReadings[zoneID]
		assert.Equal(t, Energy(600), lastReading)
	})

	t.Run("ZoneIDGeneration", func(t *testing.T) {
		zone := &mockEnergyZone{name: "package", index: 0, energy: 100, maxEnergy: 1000}
		zones := []EnergyZone{zone}

		az := NewAggregatedZone(zones)

		// First call should create last reading entry
		_, err := az.Energy()
		require.NoError(t, err)
		assert.Len(t, az.lastReadings, 1)

		// Verify zone ID format
		expectedZoneID := zoneKey{"package", 0}
		_, exists := az.lastReadings[expectedZoneID]
		assert.True(t, exists, "Expected zone ID %v to exist", expectedZoneID)
	})

	t.Run("ConcurrentStateInitialization", func(t *testing.T) {
		zones := []EnergyZone{
			&mockEnergyZone{name: "package", index: 0, energy: 100, maxEnergy: 1000},
			&mockEnergyZone{name: "package", index: 1, energy: 200, maxEnergy: 1000},
		}

		az := NewAggregatedZone(zones)

		// Concurrent first-time access
		done := make(chan bool, 10)
		for i := 0; i < 10; i++ {
			go func() {
				defer func() { done <- true }()
				_, err := az.Energy()
				assert.NoError(t, err)
			}()
		}

		// Wait for all goroutines
		for i := 0; i < 10; i++ {
			<-done
		}

		// Should have exactly 2 last readings initialized
		assert.Len(t, az.lastReadings, 2)

		// Verify both zone IDs exist
		zoneKey0 := zoneKey{"package", 0}
		zoneKey1 := zoneKey{"package", 1}

		_, exists0 := az.lastReadings[zoneKey0]
		_, exists1 := az.lastReadings[zoneKey1]

		assert.True(t, exists0)
		assert.True(t, exists1)
	})
}

// TestAggregatedZone_EdgeCases tests edge cases and unusual scenarios
func TestAggregatedZone_EdgeCases(t *testing.T) {
	t.Run("InconsistentZoneState", func(t *testing.T) {
		// Test handling when zones exceed their own MaxEnergy
		zones := []EnergyZone{
			&mockEnergyZone{name: "package", index: 0, energy: 900, maxEnergy: 1000},
			&mockEnergyZone{name: "package", index: 1, energy: 900, maxEnergy: 1000},
		}

		az := NewAggregatedZone(zones)

		energy1, err := az.Energy()
		require.NoError(t, err)
		assert.Equal(t, Energy(1800), energy1)

		// Set zones to values that exceed their MaxEnergy (inconsistent state)
		zones[0].(*mockEnergyZone).SetEnergy(1100)
		zones[1].(*mockEnergyZone).SetEnergy(1100)

		energy2, err := az.Energy()
		require.NoError(t, err)

		// Should handle gracefully without panicking
		t.Logf("Energy with inconsistent state: %d", energy2)
		assert.NotPanics(t, func() {
			_, _ = az.Energy()
		})
	})

	t.Run("OverflowProtection", func(t *testing.T) {
		// Test protection against various overflow scenarios
		const largeMax = Energy(^uint64(0) >> 1)

		zone0 := &mockEnergyZone{
			name: "package", index: 0,
			energy:    largeMax - 1000,
			maxEnergy: largeMax,
		}
		zone1 := &mockEnergyZone{
			name: "package", index: 1,
			energy:    largeMax - 2000,
			maxEnergy: largeMax,
		}
		zones := []EnergyZone{zone0, zone1}

		az := NewAggregatedZone(zones)

		// Should work without overflow
		energy1, err := az.Energy()
		require.NoError(t, err)

		rawTotal := (largeMax - 1000) + (largeMax - 2000)
		assert.Equal(t, rawTotal, energy1)

		// Test incremental updates
		zone0.SetEnergy(largeMax - 500)
		zone1.SetEnergy(largeMax - 1500)
		energy2, err := az.Energy()
		require.NoError(t, err)

		expected2 := energy1 + 1000 // Total delta should be 1000
		assert.Equal(t, expected2, energy2)
	})
}
