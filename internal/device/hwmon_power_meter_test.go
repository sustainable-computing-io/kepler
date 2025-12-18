// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package device

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	validHwmonPath = "testdata/sys"
	badHwmonPath   = "testdata/bad_sysfs"
)

// TestHwmonPowerMeterInterface ensures that hwmonPowerMeter properly implements the CPUPowerMeter interface
func TestHwmonPowerMeterInterface(t *testing.T) {
	var _ CPUPowerMeter = (*hwmonPowerMeter)(nil)
}

// TestNewHwmonPowerMeter tests the constructor
func TestNewHwmonPowerMeter(t *testing.T) {
	meter, err := NewHwmonPowerMeter(validHwmonPath)
	assert.NotNil(t, meter, "NewHwmonPowerMeter should not return nil")
	assert.NoError(t, err, "NewHwmonPowerMeter should not return error")
	assert.IsType(t, &hwmonPowerMeter{}, meter, "NewHwmonPowerMeter should return a *hwmonPowerMeter")
	assert.Equal(t, "hwmon", meter.Name())
}

// TestHwmonPowerMeter_Name tests the Name method
func TestHwmonPowerMeter_Name(t *testing.T) {
	meter := &hwmonPowerMeter{}
	name := meter.Name()
	assert.Equal(t, "hwmon", name, "Name() should return 'hwmon'")
}

// TestHwmonPowerMeter_Init tests initialization
func TestHwmonPowerMeter_Init(t *testing.T) {
	meter, err := NewHwmonPowerMeter(validHwmonPath)
	require.NoError(t, err, "NewHwmonPowerMeter should not return an error")

	t.Logf("Testing Init() for hwmon power meter...")
	err = meter.Init()
	assert.NoError(t, err, "Init() should not return an error")
	t.Logf("✓ Init() succeeded")
}

// TestHwmonPowerMeter_InitFail tests initialization failure
func TestHwmonPowerMeter_InitFail(t *testing.T) {
	meter, err := NewHwmonPowerMeter(badHwmonPath)
	require.NoError(t, err)

	t.Logf("Testing Init() with invalid path...")
	err = meter.Init()
	assert.Error(t, err, "Init() should return an error for invalid path")
	t.Logf("✓ Init() correctly failed with error: %v", err)
}

// TestHwmonPowerMeter_Zones tests zone discovery
func TestHwmonPowerMeter_Zones(t *testing.T) {
	meter, err := NewHwmonPowerMeter(validHwmonPath)
	require.NoError(t, err)

	t.Logf("\n=== Testing hwmon zone discovery ===")
	zones, err := meter.Zones()
	assert.NoError(t, err, "Zones() should not return an error")
	assert.NotNil(t, zones, "Zones() should return a non-nil slice")

	t.Logf("Found %d hwmon power zones:", len(zones))
	for i, zone := range zones {
		t.Logf("  [%d] Zone: %s", i, zone.Name())
		t.Logf("      Index: %d", zone.Index())
		t.Logf("      Path: %s", zone.Path())

		// Test Power() method
		power, err := zone.Power()
		if assert.NoError(t, err, "Power() should not return error for zone %s", zone.Name()) {
			t.Logf("      Power: %.2f W", power.Watts())
		}

		// Test Energy() method (should return 0 for hwmon)
		energy, err := zone.Energy()
		assert.EqualError(t, err, "hwmon zones do not provide energy readings")
		assert.Equal(t, Energy(0), energy, "Energy() should return 0 for hwmon zones")

		// Test MaxEnergy() method
		maxEnergy := zone.MaxEnergy()
		assert.Equal(t, Energy(0), maxEnergy, "MaxEnergy() should return 0 for hwmon zones")
	}

	// Verify we found expected zones
	zoneNames := make([]string, len(zones))
	for i, zone := range zones {
		zoneNames[i] = zone.Name()
	}

	t.Logf("\nZone names: %v", zoneNames)
	assert.Contains(t, zoneNames, "package", "Should find 'package' zone")
	assert.Contains(t, zoneNames, "core", "Should find 'core' zone")
	assert.Contains(t, zoneNames, "gpu", "Should find 'gpu' zone")
}

// TestHwmonPowerMeter_ZoneDetails provides detailed zone information
func TestHwmonPowerMeter_ZoneDetails(t *testing.T) {
	meter, err := NewHwmonPowerMeter(validHwmonPath)
	require.NoError(t, err)

	zones, err := meter.Zones()
	require.NoError(t, err)

	t.Logf("\n=== Detailed Zone Information ===")
	for _, zone := range zones {
		hwmonZone, ok := zone.(*hwmonPowerZone)
		require.True(t, ok, "Zone should be *hwmonPowerZone type")

		t.Logf("\nChip: %s", hwmonZone.chipName)
		t.Logf("  Human Name: %s", hwmonZone.humanName)
		t.Logf("  Zone Name: %s", hwmonZone.Name())
		t.Logf("  Sensor Index: %d", hwmonZone.Index())
		t.Logf("  Sysfs Path: %s", hwmonZone.Path())

		// Read and display actual power value
		power, err := hwmonZone.Power()
		require.NoError(t, err)
		t.Logf("  Current Power: %.3f W (%.0f µW)", power.Watts(), power.MicroWatts())

		// Verify power is reasonable (between 0 and 500W for test data)
		assert.GreaterOrEqual(t, power.Watts(), 0.0, "Power should be non-negative")
		assert.LessOrEqual(t, power.Watts(), 500.0, "Power should be reasonable for test data")
	}
}

// TestHwmonPowerMeter_PowerReadings verifies power readings from power*_input files
func TestHwmonPowerMeter_PowerReadings(t *testing.T) {
	meter, err := NewHwmonPowerMeter(validHwmonPath)
	require.NoError(t, err)

	zones, err := meter.Zones()
	require.NoError(t, err)

	t.Logf("\n=== Verifying Power Readings ===")

	// Map of expected power values (zone name -> watts)
	expectedPower := map[string]float64{
		"package": 45.0,  // 45,000,000 µW
		"core":    15.0,  // 15,000,000 µW
		"gpu":     118.0, // 118,000,000 µW (should prefer power1_average)
	}

	for _, zone := range zones {
		power, err := zone.Power()
		require.NoError(t, err, "Failed to read power for zone %s", zone.Name())

		expected, ok := expectedPower[zone.Name()]
		if ok {
			assert.InDelta(t, expected, power.Watts(), 0.01,
				"Zone %s: expected %.2f W, got %.2f W", zone.Name(), expected, power.Watts())
			t.Logf("✓ Zone '%s': %.2f W (matches expected)", zone.Name(), power.Watts())
		} else {
			t.Logf("  Zone '%s': %.2f W", zone.Name(), power.Watts())
		}
	}
}

// TestHwmonPowerMeter_PrimaryEnergyZone tests the primary zone selection
func TestHwmonPowerMeter_PrimaryEnergyZone(t *testing.T) {
	meter, err := NewHwmonPowerMeter(validHwmonPath)
	require.NoError(t, err)

	t.Logf("\n=== Testing Primary Energy Zone Selection ===")
	primaryZone, err := meter.PrimaryEnergyZone()
	assert.NoError(t, err, "PrimaryEnergyZone() should not return error")
	require.NotNil(t, primaryZone, "Primary zone should not be nil")

	t.Logf("Primary zone: %s", primaryZone.Name())
	t.Logf("  Path: %s", primaryZone.Path())

	power, err := primaryZone.Power()
	require.NoError(t, err)
	t.Logf("  Power: %.2f W", power.Watts())

	// Primary zone should be 'package' based on priority order
	assert.Equal(t, "package", primaryZone.Name(),
		"Primary zone should be 'package' based on priority hierarchy")
}

// TestHwmonPowerMeter_ZoneFilter tests zone filtering
func TestHwmonPowerMeter_ZoneFilter(t *testing.T) {
	t.Logf("\n=== Testing Zone Filtering ===")

	// Filter for only 'package' zone
	meter, err := NewHwmonPowerMeter(validHwmonPath,
		WithHwmonZoneFilter([]string{"package"}))
	require.NoError(t, err)

	zones, err := meter.Zones()
	require.NoError(t, err)

	t.Logf("With zone filter 'package', found %d zones:", len(zones))
	assert.Equal(t, 1, len(zones), "Should only find 1 zone with filter")

	for _, zone := range zones {
		t.Logf("  - %s", zone.Name())
		assert.Equal(t, "package", zone.Name(), "Zone should be 'package'")
	}
}

// TestHwmonPowerMeter_MultipleReadings tests reading power multiple times
func TestHwmonPowerMeter_MultipleReadings(t *testing.T) {
	meter, err := NewHwmonPowerMeter(validHwmonPath)
	require.NoError(t, err)

	zones, err := meter.Zones()
	require.NoError(t, err)

	t.Logf("\n=== Testing Multiple Power Readings ===")
	packageZone := zones[0]

	t.Logf("Reading power from '%s' zone 5 times:", packageZone.Name())
	for i := 0; i < 5; i++ {
		power, err := packageZone.Power()
		require.NoError(t, err)
		t.Logf("  Reading %d: %.2f W", i+1, power.Watts())
	}
}

// TestHwmonPowerMeter_FallbackNaming tests zones without labels
func TestHwmonPowerMeter_FallbackNaming(t *testing.T) {
	meter, err := NewHwmonPowerMeter(validHwmonPath)
	require.NoError(t, err)

	zones, err := meter.Zones()
	require.NoError(t, err)

	t.Logf("\n=== Testing Fallback Naming (zones without labels) ===")

	// Look for zones from platform_sensor which has no label
	found := false
	for _, zone := range zones {
		hwmonZone := zone.(*hwmonPowerZone)
		if hwmonZone.chipName == "platform_sensor" {
			found = true
			t.Logf("Found zone without label:")
			t.Logf("  Zone name: %s", zone.Name())
			t.Logf("  Should follow pattern: <chipname>_power<N>")
			assert.Contains(t, zone.Name(), "platform_sensor_power",
				"Zone name should contain chip name and 'power'")

			power, err := zone.Power()
			require.NoError(t, err)
			t.Logf("  Power: %.2f W", power.Watts())
			assert.InDelta(t, 30.0, power.Watts(), 0.01, "Power should be 30W")
		}
	}

	assert.True(t, found, "Should find at least one zone from platform_sensor")
}

// TestHwmonPowerMeter_PreferAverage tests that power*_average is preferred over power*_input
func TestHwmonPowerMeter_PreferAverage(t *testing.T) {
	meter, err := NewHwmonPowerMeter(validHwmonPath)
	require.NoError(t, err)

	zones, err := meter.Zones()
	require.NoError(t, err)

	t.Logf("\n=== Testing power*_average Preference ===")

	// Find the GPU zone which has both power1_input (120W) and power1_average (118W)
	for _, zone := range zones {
		if zone.Name() == "gpu" {
			power, err := zone.Power()
			require.NoError(t, err)

			t.Logf("GPU zone has:")
			t.Logf("  power1_input:   120.0 W")
			t.Logf("  power1_average: 118.0 W")
			t.Logf("  Actual reading: %.2f W", power.Watts())

			assert.InDelta(t, 118.0, power.Watts(), 0.01,
				"Should prefer power1_average (118W) over power1_input (120W)")
			t.Logf("✓ Correctly preferred power1_average")
			return
		}
	}

	t.Fatal("GPU zone not found")
}

// TestHwmonPowerZone_Interface tests the hwmonPowerZone type directly
func TestHwmonPowerZone_Interface(t *testing.T) {
	zone := &hwmonPowerZone{
		name:      "test_zone",
		index:     1,
		path:      "testdata/sys/class/hwmon/hwmon0/power1_input",
		chipName:  "test_chip",
		humanName: "test_human",
	}

	t.Logf("\n=== Testing hwmonPowerZone Interface ===")
	assert.Equal(t, "test_zone", zone.Name())
	assert.Equal(t, 1, zone.Index())
	assert.Equal(t, "testdata/sys/class/hwmon/hwmon0/power1_input", zone.Path())

	// Test Energy() returns 0
	energy, err := zone.Energy()
	assert.EqualError(t, err, "hwmon zones do not provide energy readings")
	assert.Equal(t, Energy(0), energy, "Energy() should return 0")

	// Test MaxEnergy() returns 0
	maxEnergy := zone.MaxEnergy()
	assert.Equal(t, Energy(0), maxEnergy, "MaxEnergy() should return 0")

	// Test Power() reads actual value
	power, err := zone.Power()
	assert.NoError(t, err)
	assert.InDelta(t, 45.0, power.Watts(), 0.01, "Power should be 45W from test file")
	t.Logf("✓ Zone methods work correctly")
	t.Logf("  Name: %s", zone.Name())
	t.Logf("  Power: %.2f W", power.Watts())
}

// TestHwmonPowerMeter_NoZones tests behavior when no power zones are found
func TestHwmonPowerMeter_NoZones(t *testing.T) {
	// Create a temporary empty hwmon directory
	tmpDir := t.TempDir()
	hwmonDir := tmpDir + "/class/hwmon"
	err := os.MkdirAll(hwmonDir, 0755)
	require.NoError(t, err)

	meter, err := NewHwmonPowerMeter(tmpDir)
	require.NoError(t, err)

	t.Logf("\n=== Testing Empty hwmon Directory ===")
	err = meter.Init()
	assert.Error(t, err, "Init() should fail when no zones are found")
	t.Logf("✓ Correctly failed with: %v", err)
}

// TestHwmonPowerMeter_RealSystem tests against the actual /sys filesystem
// This test is skipped by default. Run with: go test -v -run TestHwmonPowerMeter_RealSystem
func TestHwmonPowerMeter_RealSystem(t *testing.T) {
	realSysPath := "/sys"

	// Skip if /sys/class/hwmon doesn't exist or is not accessible
	if _, err := os.Stat(realSysPath + "/class/hwmon"); os.IsNotExist(err) {
		t.Skip("Skipping real system test: /sys/class/hwmon not found")
	}

	t.Logf("\n================================================================================")
	t.Logf("TESTING AGAINST REAL SYSTEM: /sys/class/hwmon")
	t.Logf("================================================================================")

	meter, err := NewHwmonPowerMeter(realSysPath)
	if err != nil {
		t.Fatalf("Failed to create hwmon power meter: %v", err)
	}

	// Test Init
	t.Logf("\n--- Initializing hwmon power meter ---")
	err = meter.Init()
	if err != nil {
		t.Logf("⚠ Init failed: %v", err)
		t.Logf("This might mean your system doesn't have power sensors exposed via hwmon")
		t.Skip("No power zones found on this system")
	}
	t.Logf("✓ Init successful")

	// Get all zones
	t.Logf("\n--- Discovering power zones ---")
	zones, err := meter.Zones()
	if err != nil {
		t.Fatalf("Failed to get zones: %v", err)
	}

	t.Logf("\n================================================================================")
	t.Logf("FOUND %d POWER ZONES ON YOUR SYSTEM", len(zones))
	t.Logf("================================================================================")

	for i, zone := range zones {
		t.Logf("\n[Zone %d/%d]", i+1, len(zones))
		t.Logf("  Zone Name: %s", zone.Name())
		t.Logf("  Index: %d", zone.Index())
		t.Logf("  Sysfs Path: %s", zone.Path())

		// Get detailed info from hwmonPowerZone
		if hwmonZone, ok := zone.(*hwmonPowerZone); ok {
			t.Logf("  Chip Name: %s", hwmonZone.chipName)
			t.Logf("  Human Name: %s", hwmonZone.humanName)
		}

		// Read current power
		power, err := zone.Power()
		if err != nil {
			t.Logf("  ⚠ Power read failed: %v", err)
		} else {
			t.Logf("  ⚡ Current Power: %.3f W (%.0f µW)", power.Watts(), power.MicroWatts())
		}

		// Verify this is reading from power*_input
		if strings.Contains(zone.Path(), "power") && strings.Contains(zone.Path(), "_input") {
			t.Logf("  ✓ Reading from power*_input file")
		} else if strings.Contains(zone.Path(), "power") && strings.Contains(zone.Path(), "_average") {
			t.Logf("  ✓ Reading from power*_average file (preferred)")
		}
	}

	// Get primary zone
	t.Logf("\n================================================================================")
	t.Logf("PRIMARY ENERGY ZONE")
	t.Logf("================================================================================")
	primaryZone, err := meter.PrimaryEnergyZone()
	if err != nil {
		t.Logf("⚠ Could not determine primary zone: %v", err)
	} else {
		t.Logf("Primary Zone: %s", primaryZone.Name())
		t.Logf("Path: %s", primaryZone.Path())
		power, err := primaryZone.Power()
		if err == nil {
			t.Logf("Power: %.3f W", power.Watts())
		}
	}

	// List all zone names
	t.Logf("\n================================================================================")
	t.Logf("SUMMARY")
	t.Logf("================================================================================")
	t.Logf("Total zones found: %d", len(zones))
	t.Logf("Zone names:")
	for _, zone := range zones {
		power, _ := zone.Power()
		t.Logf("  - %s: %.2f W", zone.Name(), power.Watts())
	}

	t.Logf("\n================================================================================")
	t.Logf("✓ Real system test completed successfully")
	t.Logf("================================================================================\n")
}
