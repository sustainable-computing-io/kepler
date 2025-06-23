// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package device

import (
	"log/slog"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewFakeCPUMeter(t *testing.T) {
	meter, err := NewFakeCPUMeter(nil)
	assert.NoError(t, err)
	assert.NotNil(t, meter)
	assert.IsType(t, &fakeRaplMeter{}, meter)

	fakeRapl := meter.(*fakeRaplMeter)
	assert.Equal(t, defaultRaplPath, fakeRapl.devicePath)

	zones, err := meter.Zones()
	assert.NoError(t, err)
	assert.Equal(t, len(defaultFakeZones), len(zones))

	// check zone names match defaults
	zoneNames := make([]string, len(zones))
	for i, zone := range zones {
		zoneNames[i] = zone.Name()
	}
	for _, name := range defaultFakeZones {
		assert.Contains(t, zoneNames, name)
	}
}

func TestFakeRaplMeter_Name(t *testing.T) {
	meter, _ := NewFakeCPUMeter(nil)
	assert.Equal(t, "fake-cpu-meter", meter.Name())
}

func TestFakeEnergyZone_Basics(t *testing.T) {
	zone := &fakeEnergyZone{
		name:         "test-zone",
		index:        42,
		path:         "/fake/path/energy_test-zone",
		maxEnergy:    500000,
		increment:    100,
		randomFactor: 0.5,
	}

	assert.Equal(t, "test-zone", zone.Name())
	assert.Equal(t, 42, zone.Index())
	assert.Equal(t, "/fake/path/energy_test-zone", zone.Path())
	assert.Equal(t, Energy(500000), zone.MaxEnergy())
}

func TestFakeEnergyZone_Energy(t *testing.T) {
	zone := &fakeEnergyZone{
		name:         "test-zone",
		energy:       0,
		maxEnergy:    1000,
		increment:    100,
		randomFactor: 0, // No randomness
	}

	// First read should return the increment
	e1, err := zone.Energy()
	assert.NoError(t, err)
	assert.Equal(t, Energy(100), e1)

	// Second read should return double the increment
	e2, err := zone.Energy()
	assert.NoError(t, err)
	assert.Equal(t, Energy(200), e2)

	// Test wrap-around at maxEnergy
	zone.energy = 950
	e3, err := zone.Energy()
	assert.NoError(t, err)
	assert.Equal(t, Energy(50), e3) // Wrapped around: 950 + 100 = 1050, but 1050 % 1000 = 50
}

func TestWithFakeZones(t *testing.T) {
	customZones := []string{"package", "custom-zone"}
	meter, err := NewFakeCPUMeter(customZones)
	assert.NoError(t, err)

	zones, err := meter.Zones()
	assert.NoError(t, err)
	assert.Equal(t, len(customZones), len(zones))

	zoneNames := make([]string, len(zones))
	for i, zone := range zones {
		zoneNames[i] = zone.Name()
	}
	for _, name := range customZones {
		assert.Contains(t, zoneNames, name)
	}

	// empty zones should fallback to defaults
	meter, err = NewFakeCPUMeter(nil)
	assert.NoError(t, err)

	zones, err = meter.Zones()
	assert.NoError(t, err)
	assert.Equal(t, len(defaultFakeZones), len(zones))
}

func TestWithFakePath(t *testing.T) {
	customPath := "/custom/rapl/path"
	meter, err := NewFakeCPUMeter(nil, WithFakePath(customPath))
	assert.NoError(t, err)

	fakeRapl := meter.(*fakeRaplMeter)
	assert.Equal(t, customPath, fakeRapl.devicePath)

	zones, err := meter.Zones()
	assert.NoError(t, err)

	for _, zone := range zones {
		assert.Contains(t, zone.Path(), customPath)
		assert.Equal(t, filepath.Join(customPath, "energy_"+zone.Name()), zone.Path())
	}
}

func TestWithFakeMaxEnergy(t *testing.T) {
	customMax := Energy(999999)
	meter, err := NewFakeCPUMeter(nil, WithFakeMaxEnergy(customMax))
	assert.NoError(t, err)

	zones, err := meter.Zones()
	assert.NoError(t, err)
	assert.Len(t, zones, len(defaultFakeZones))

	for _, zone := range zones {
		fakeZone, ok := zone.(*fakeEnergyZone)
		assert.True(t, ok)
		assert.Equal(t, customMax, fakeZone.maxEnergy)
	}
}

func TestWithFakeLogger(t *testing.T) {
	logger := slog.Default().With("test", "logger")
	meter, err := NewFakeCPUMeter(nil, WithFakeLogger(logger))
	assert.NoError(t, err)

	fakeRapl := meter.(*fakeRaplMeter)
	assert.NotNil(t, fakeRapl.logger)
}

func TestMultipleOptions(t *testing.T) {
	customPath := "/custom/rapl/path"
	customMax := Energy(888888)
	customZones := []string{"custom1", "custom2"}
	logger := slog.Default().With("test", "logger")

	meter, err := NewFakeCPUMeter(
		customZones,
		WithFakePath(customPath),
		WithFakeMaxEnergy(customMax),
		WithFakeLogger(logger),
	)
	assert.NoError(t, err)

	fakeRapl := meter.(*fakeRaplMeter)
	assert.Equal(t, customPath, fakeRapl.devicePath)
	assert.NotNil(t, fakeRapl.logger)

	zones, err := meter.Zones()
	assert.NoError(t, err)
	assert.Equal(t, len(customZones), len(zones))

	for _, zone := range zones {
		assert.Contains(t, zone.Path(), customPath)
		fakeZone, ok := zone.(*fakeEnergyZone)
		assert.True(t, ok)
		assert.Equal(t, customMax, fakeZone.maxEnergy)
	}
}

// TestEnergyRandomness tests that the energy value changes with random component
func TestEnergyRandomness(t *testing.T) {
	zone := &fakeEnergyZone{
		name:         "test-zone",
		energy:       0,
		maxEnergy:    10000,
		increment:    100,
		randomFactor: 1.0, // Full randomness
	}

	// Read energy multiple times
	var readings []Energy
	for range 10 {
		e, err := zone.Energy()
		assert.NoError(t, err)
		readings = append(readings, e)
	}

	// with randomness, it's unlikely all readings follow exact increment
	exactIncrement := true
	for i := 1; i < len(readings); i++ {
		if readings[i]-readings[i-1] != zone.increment {
			exactIncrement = false
			break
		}
	}

	assert.False(t, exactIncrement, "Expected randomness in energy readings")
}

// TestMultiSocketEnergyStrategy tests energy strategy for multi-socket systems
func TestMultiSocketEnergyStrategy(t *testing.T) {
	tests := []struct {
		name             string
		zones            []string
		expectedStrategy string
		expectedZones    int
	}{{
		name:             "single socket - pkg+dram",
		zones:            []string{"package", "dram"},
		expectedStrategy: "package+dram",
		expectedZones:    2,
	}, {
		name:             "dual socket - pkg", // Must make use of AggregatedZone
		zones:            []string{"package"},
		expectedStrategy: "package",
		expectedZones:    1, // Uses first package found
	}, {
		name:             "quad socket with multiple cores",
		zones:            []string{"package", "core", "core", "core", "core"},
		expectedStrategy: "package",
		expectedZones:    1, // Uses first package
	}, {
		name:             "multi-socket with psys",
		zones:            []string{"psys", "package", "dram"},
		expectedStrategy: "psys",
		expectedZones:    1, // PSys takes priority
	}, {
		name:             "legacy multi-socket with pp0/pp1",
		zones:            []string{"pp0", "pp1", "dram"},
		expectedStrategy: "pp0+pp1+dram",
		expectedZones:    3,
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meter, err := NewFakeCPUMeter(tt.zones)
			assert.NoError(t, err)

			fakeRapl := meter.(*fakeRaplMeter)
			err = fakeRapl.Init()
			assert.NoError(t, err)

			assert.NotNil(t, fakeRapl.topologicalEnergy)
			assert.Equal(t, tt.expectedStrategy, fakeRapl.topologicalEnergy.strategy)
			assert.Equal(t, tt.expectedZones, len(fakeRapl.topologicalEnergy.zones))

			// Test TopologyEnergy works
			energy := meter.TopologyEnergy()
			assert.Greater(t, energy, Energy(0))
		})
	}
}

// TestMultiSocketTopologyEnergySum tests that multi-socket energy is summed correctly
func TestMultiSocketTopologyEnergySum(t *testing.T) {
	// Create dual socket with package zones
	zones := []string{"package-0", "package-1"}
	meter, err := NewFakeCPUMeter(zones)
	assert.NoError(t, err)

	fakeRapl := meter.(*fakeRaplMeter)
	err = fakeRapl.Init()
	assert.NoError(t, err)

	// Strategy should use only first package (pkg strategy)
	assert.Equal(t, "pkg", fakeRapl.topologicalEnergy.strategy)
	assert.Equal(t, 1, len(fakeRapl.topologicalEnergy.zones))

	// Verify the selected zone is package-0 (first one found)
	selectedZone := fakeRapl.topologicalEnergy.zones[0]
	assert.Equal(t, "package-0", selectedZone.Name())

	// Test energy reading
	energy1 := meter.TopologyEnergy()
	energy2 := meter.TopologyEnergy()

	// Energy should increase over time
	assert.Greater(t, energy2, energy1)
}

// TestMultiSocketPkgDramStrategy tests PKG+DRAM strategy with multiple sockets
func TestMultiSocketPkgDramStrategy(t *testing.T) {
	// Create system with multiple packages and dram
	zones := []string{"package-0", "package-1", "dram-0", "dram-1"}
	meter, err := NewFakeCPUMeter(zones)
	assert.NoError(t, err)

	fakeRapl := meter.(*fakeRaplMeter)
	err = fakeRapl.Init()
	assert.NoError(t, err)

	// Should use pkg+dram strategy with first package and first dram
	assert.Equal(t, "pkg+dram", fakeRapl.topologicalEnergy.strategy)
	assert.Equal(t, 2, len(fakeRapl.topologicalEnergy.zones))

	// Verify zone names
	zoneNames := make([]string, len(fakeRapl.topologicalEnergy.zones))
	for i, zone := range fakeRapl.topologicalEnergy.zones {
		zoneNames[i] = zone.Name()
	}
	assert.Contains(t, zoneNames, "package-0")
	assert.Contains(t, zoneNames, "dram-0")

	// Test energy is sum of package and dram
	// Note: Each call to Energy() increments the value, so we need to capture at the same time
	energy1 := meter.TopologyEnergy()
	energy2 := meter.TopologyEnergy()

	// Verify energy increases over time (sum of both zones)
	assert.Greater(t, energy2, energy1)
	assert.Greater(t, energy1, Energy(0))
}

// TestDetermineEnergyStrategy tests the energy strategy logic directly
func TestDetermineEnergyStrategy(t *testing.T) {
	tests := []struct {
		name             string
		zoneNames        []string
		expectedStrategy string
		expectedZones    int
		shouldPanic      bool
	}{{
		name:             "psys priority - exact match",
		zoneNames:        []string{"psys", "package", "dram"},
		expectedStrategy: "psys",
		expectedZones:    1,
	}, {
		name:             "pkg+dram strategy - exact match",
		zoneNames:        []string{"package", "dram", "core"},
		expectedStrategy: "package+dram",
		expectedZones:    2,
	}, {
		name:             "pkg only - no dram",
		zoneNames:        []string{"package", "core"},
		expectedStrategy: "package",
		expectedZones:    1,
	}, {
		name:             "pp0+pp1+dram strategy - legacy",
		zoneNames:        []string{"pp0", "pp1", "dram", "uncore"},
		expectedStrategy: "pp0+pp1+dram",
		expectedZones:    3,
	}, {
		name:             "pp0+dram strategy - no pp1",
		zoneNames:        []string{"pp0", "dram", "uncore"},
		expectedStrategy: "pp0+dram",
		expectedZones:    2,
	}, {
		name:             "pp0 only strategy",
		zoneNames:        []string{"pp0", "uncore"},
		expectedStrategy: "pp0",
		expectedZones:    1,
	}, {
		name:             "fallback strategy",
		zoneNames:        []string{"unknown", "custom"},
		expectedStrategy: "fallback-unknown",
		expectedZones:    1,
	}, {
		name:             "no zones",
		zoneNames:        []string{},
		expectedStrategy: "none",
		expectedZones:    0,
		shouldPanic:      true,
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fake zones
			var zones []EnergyZone
			for i, name := range tt.zoneNames {
				zones = append(zones, &fakeEnergyZone{
					name:      name,
					index:     i,
					path:      "/fake/" + name,
					maxEnergy: 1000000,
				})
			}

			if tt.shouldPanic {
				assert.Panics(t, func() {
					DetermineTopologyEnergyStrategy(zones)
				})
				return
			}

			strategy := DetermineTopologyEnergyStrategy(zones)
			assert.NotNil(t, strategy)
			assert.Equal(t, tt.expectedStrategy, strategy.strategy)
			assert.Equal(t, tt.expectedZones, len(strategy.zones))

			// Test ComputeEnergy works
			energy := strategy.ComputeEnergy()
			if len(strategy.zones) > 0 {
				assert.GreaterOrEqual(t, energy, Energy(0))
			} else {
				assert.Equal(t, Energy(0), energy)
			}
		})
	}
}
