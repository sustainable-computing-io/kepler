// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package device

import (
	"log/slog"
	"path/filepath"
	"testing"
	"time"

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
		baseWatts:    10.0,
		randomFactor: 0.5,
	}

	assert.Equal(t, "test-zone", zone.Name())
	assert.Equal(t, 42, zone.Index())
	assert.Equal(t, "/fake/path/energy_test-zone", zone.Path())
	assert.Equal(t, Energy(500000), zone.MaxEnergy())
}

func TestFakeEnergyZone_Energy(t *testing.T) {
	// Create a fake CPU reader for consistent testing
	fakeCPU := &fakeCPUReader{
		baseUsage:    0.5, // 50% CPU usage
		randomFactor: 0,   // No randomness for predictable tests
	}

	zone := &fakeEnergyZone{
		name:         "test-zone",
		energy:       0,
		maxEnergy:    1000000, // Large enough to avoid wrap-around in test
		baseWatts:    10.0,    // 10 watts base power
		randomFactor: 0,       // No randomness
		cpuReader:    fakeCPU,
	}

	// First read should initialize timestamp and return 0
	e1, err := zone.Energy()
	assert.NoError(t, err)
	assert.Equal(t, Energy(0), e1)

	// Sleep briefly to ensure time passes
	time.Sleep(10 * time.Millisecond)

	// Second read should return energy based on time elapsed and CPU usage
	e2, err := zone.Energy()
	assert.NoError(t, err)
	assert.Greater(t, e2, Energy(0), "Energy should increase over time")

	// Third read after more time should show further increase
	time.Sleep(10 * time.Millisecond)
	e3, err := zone.Energy()
	assert.NoError(t, err)
	assert.Greater(t, e3, e2, "Energy should continue to increase")

	// Test wrap-around at maxEnergy
	zone.energy = zone.maxEnergy - 100
	e4, err := zone.Energy()
	assert.NoError(t, err)
	assert.Less(t, e4, zone.maxEnergy, "Energy should wrap around at maxEnergy")
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
	// Create a fake CPU reader for consistent testing
	fakeCPU := &fakeCPUReader{
		baseUsage:    0.5, // 50% CPU usage
		randomFactor: 0.3, // Some randomness in CPU usage
	}

	zone := &fakeEnergyZone{
		name:         "test-zone",
		energy:       0,
		maxEnergy:    10000000, // Large enough to avoid wrap-around
		baseWatts:    10.0,     // 10 watts base power
		randomFactor: 0.3,      // 30% randomness
		cpuReader:    fakeCPU,
	}

	// Initialize with first read
	_, err := zone.Energy()
	assert.NoError(t, err)

	// Read energy multiple times with small delays
	var readings []Energy
	for range 10 {
		time.Sleep(1 * time.Millisecond) // Small delay to ensure time passes
		e, err := zone.Energy()
		assert.NoError(t, err)
		readings = append(readings, e)
	}

	// With randomness, energy increments shouldn't be exactly the same
	// Check that we have some variation in the differences
	var diffs []Energy
	for i := 1; i < len(readings); i++ {
		diffs = append(diffs, readings[i]-readings[i-1])
	}

	// At least some differences should be different (due to randomness)
	allSame := true
	if len(diffs) > 1 {
		firstDiff := diffs[0]
		for _, diff := range diffs[1:] {
			if diff != firstDiff {
				allSame = false
				break
			}
		}
	}

	// Note: Due to randomness, this test might occasionally pass even with randomness
	// but over multiple runs it should show variation
	if len(diffs) > 3 {
		assert.False(t, allSame, "Expected some variation in energy increments due to randomness")
	}
}
