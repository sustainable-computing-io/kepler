// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package esmi

import (
	"log/slog"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
        "github.com/sustainable-computing-io/kepler/internal/device/cpu"
)

func TestNewEsmiCPUMeter(t *testing.T) {
	meter, err := NewEsmiCPUMeter(nil)
	assert.NoError(t, err)
	assert.NotNil(t, meter)
	assert.IsType(t, &fakeRaplMeter{}, meter)

	fakeRapl := meter.(*fakeRaplMeter)
	assert.Equal(t, defaultRaplPath, fakeRapl.devicePath)

	zones, err := meter.Zones()
	assert.NoError(t, err)
	assert.Equal(t, len(defaultEsmiZones), len(zones))

	// check zone names match defaults
	zoneNames := make([]string, len(zones))
	for i, zone := range zones {
		zoneNames[i] = zone.Name()
	}
	for _, name := range defaultEsmiZones {
		assert.Contains(t, zoneNames, name)
	}
}

func TestEsmiRaplMeter_Name(t *testing.T) {
	meter, _ := NewEsmiCPUMeter(nil)
	assert.Equal(t, "fake-cpu-meter", meter.Name())
}

func TestEsmiEnergyZone_Basics(t *testing.T) {
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

func TestEsmiEnergyZone_Energy(t *testing.T) {
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

func TestWithEsmiZones(t *testing.T) {
	customZones := []string{"package", "custom-zone"}
	meter, err := NewEsmiCPUMeter(customZones)
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
	meter, err = NewEsmiCPUMeter(nil)
	assert.NoError(t, err)

	zones, err = meter.Zones()
	assert.NoError(t, err)
	assert.Equal(t, len(defaultEsmiZones), len(zones))
}

//func TestWithEsmiPath(t *testing.T) {
//	customPath := "/custom/rapl/path"
//	meter, err := NewEsmiCPUMeter(nil, WithEsmiPath(customPath))
//	assert.NoError(t, err)

//	fakeRapl := meter.(*fakeRaplMeter)
//	assert.Equal(t, customPath, fakeRapl.devicePath)

//	zones, err := meter.Zones()
//	assert.NoError(t, err)

//	for _, zone := range zones {
//		assert.Contains(t, zone.Path(), customPath)
//		assert.Equal(t, filepath.Join(customPath, "energy_"+zone.Name()), zone.Path())
//	}
//}

func TestWithEsmiMaxEnergy(t *testing.T) {
	customMax := Energy(999999)
	meter, err := NewEsmiCPUMeter(nil, WithEsmiMaxEnergy(customMax))
	assert.NoError(t, err)

	zones, err := meter.Zones()
	assert.NoError(t, err)
	assert.Len(t, zones, len(defaultEsmiZones))

	for _, zone := range zones {
		fakeZone, ok := zone.(*fakeEnergyZone)
		assert.True(t, ok)
		assert.Equal(t, customMax, fakeZone.maxEnergy)
	}
}

func TestWithEsmiLogger(t *testing.T) {
	logger := slog.Default().With("test", "logger")
	meter, err := NewEsmiCPUMeter(nil, WithEsmiLogger(logger))
	assert.NoError(t, err)

	fakeRapl := meter.(*fakeRaplMeter)
	assert.NotNil(t, fakeRapl.logger)
}

func TestMultipleOptions(t *testing.T) {
	customPath := "/custom/rapl/path"
	customMax := Energy(888888)
	customZones := []string{"custom1", "custom2"}
	logger := slog.Default().With("test", "logger")

	meter, err := NewEsmiCPUMeter(
		customZones,
		//WithEsmiPath(customPath),
		WithEsmiMaxEnergy(customMax),
		WithEsmiLogger(logger),
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
