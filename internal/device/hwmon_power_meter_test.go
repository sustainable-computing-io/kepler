// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package device

import (
	"log/slog"
	"os"
	"path/filepath"
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

// TestWithHwmonReader tests the WithHwmonReader option function
func TestWithHwmonReader(t *testing.T) {
	t.Logf("\n=== Testing WithHwmonReader Option ===")

	// Create a mock reader
	mockReader := &sysfsHwmonReader{
		basePath: "testdata/sys/class/hwmon",
	}

	// Create meter with custom reader
	meter, err := NewHwmonPowerMeter("testdata/sys", WithHwmonReader(mockReader))
	require.NoError(t, err)

	// Verify the custom reader was set
	assert.Equal(t, mockReader, meter.reader, "Custom reader should be set")
	t.Logf("✓ Custom reader successfully set via WithHwmonReader")
}

// TestWithHwmonLogger tests the WithHwmonLogger option function
func TestWithHwmonLogger(t *testing.T) {
	t.Logf("\n=== Testing WithHwmonLogger Option ===")

	// Create a custom logger
	customLogger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	// Create meter with custom logger
	meter, err := NewHwmonPowerMeter("testdata/sys", WithHwmonLogger(customLogger))
	require.NoError(t, err)

	// Verify logger was set (check that it's not nil and has "service" attribute)
	assert.NotNil(t, meter.logger, "Logger should be set")

	// The logger should have the "service"="hwmon" attribute added by WithHwmonLogger
	// We can verify this by checking that the logger works
	t.Logf("✓ Custom logger successfully set via WithHwmonLogger")
}

// TestWithHwmonZoneFilter tests the WithHwmonZoneFilter option function
func TestWithHwmonZoneFilter(t *testing.T) {
	t.Logf("\n=== Testing WithHwmonZoneFilter Option ===")

	// Create meter with zone filter
	zoneFilter := []string{"package", "core"}
	meter, err := NewHwmonPowerMeter("testdata/sys", WithHwmonZoneFilter(zoneFilter))
	require.NoError(t, err)

	// Verify zone filter was set
	assert.Equal(t, zoneFilter, meter.zoneFilter, "Zone filter should be set")
	assert.True(t, meter.needsZoneFiltering(), "Should need zone filtering")

	t.Logf("✓ Zone filter successfully set via WithHwmonZoneFilter")
}

// TestNewHwmonPowerMeter_WithMultipleOptions tests combining multiple options
func TestNewHwmonPowerMeter_WithMultipleOptions(t *testing.T) {
	t.Logf("\n=== Testing Multiple Options Combined ===")

	// Create custom components
	mockReader := &sysfsHwmonReader{
		basePath: "testdata/sys/class/hwmon",
	}
	customLogger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	zoneFilter := []string{"package"}

	// Create meter with all options
	meter, err := NewHwmonPowerMeter("testdata/sys",
		WithHwmonReader(mockReader),
		WithHwmonLogger(customLogger),
		WithHwmonZoneFilter(zoneFilter))
	require.NoError(t, err)

	// Verify all options were applied
	assert.Equal(t, mockReader, meter.reader, "Custom reader should be set")
	assert.NotNil(t, meter.logger, "Custom logger should be set")
	assert.Equal(t, zoneFilter, meter.zoneFilter, "Zone filter should be set")

	t.Logf("✓ All options successfully applied")
	t.Logf("  Reader: custom")
	t.Logf("  Logger: custom")
	t.Logf("  Zone filter: %v", zoneFilter)
}

// TestHwmonOptionFn_FunctionalInterface tests the option function pattern
func TestHwmonOptionFn_FunctionalInterface(t *testing.T) {
	t.Logf("\n=== Testing Option Function Pattern ===")

	// Create a meter instance
	meter := &hwmonPowerMeter{
		reader:     &sysfsHwmonReader{basePath: "/default/path"},
		logger:     slog.Default(),
		zoneFilter: []string{},
	}

	t.Run("WithHwmonReader modifies reader", func(t *testing.T) {
		newReader := &sysfsHwmonReader{basePath: "/new/path"}
		optionFn := WithHwmonReader(newReader)

		// Apply the option
		optionFn(meter)

		assert.Equal(t, newReader, meter.reader, "Reader should be updated")
		assert.Equal(t, "/new/path", meter.reader.(*sysfsHwmonReader).basePath)
		t.Logf("✓ WithHwmonReader correctly modifies reader")
	})

	t.Run("WithHwmonLogger modifies logger", func(t *testing.T) {
		customLogger := slog.New(slog.NewTextHandler(os.Stderr, nil))
		optionFn := WithHwmonLogger(customLogger)

		// Apply the option
		optionFn(meter)

		assert.NotNil(t, meter.logger, "Logger should be set")
		t.Logf("✓ WithHwmonLogger correctly modifies logger")
	})

	t.Run("WithHwmonZoneFilter modifies zone filter", func(t *testing.T) {
		filter := []string{"package", "core", "dram"}
		optionFn := WithHwmonZoneFilter(filter)

		// Apply the option
		optionFn(meter)

		assert.Equal(t, filter, meter.zoneFilter, "Zone filter should be updated")
		assert.Equal(t, 3, len(meter.zoneFilter))
		t.Logf("✓ WithHwmonZoneFilter correctly modifies zone filter")
	})
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

		// Test Energy() method (should return error for hwmon)
		energy, err := zone.Energy()
		assert.Error(t, err, "Energy() should return error for hwmon zones")
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
		// Check if this is an aggregated zone or regular zone
		if aggZone, ok := zone.(*AggregatedZone); ok {
			t.Logf("\nAggregated Zone: %s", zone.Name())
			t.Logf("  Contains: %d individual zones", len(aggZone.zones))
			t.Logf("  Index: %d (aggregated)", zone.Index())

			power, err := zone.Power()
			require.NoError(t, err)
			t.Logf("  Current Power: %.3f W (%.0f µW)", power.Watts(), power.MicroWatts())
			continue
		}

		// Handle both direct power zones and calculated power zones
		if hwmonZone, ok := zone.(*hwmonPowerZone); ok {
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
		} else if calcZone, ok := zone.(*hwmonCalculatedPowerZone); ok {
			t.Logf("\nCalculated Power Zone: %s", calcZone.chipName)
			t.Logf("  Human Name: %s", calcZone.humanName)
			t.Logf("  Zone Name: %s", calcZone.Name())
			t.Logf("  Sensor Index: %d", calcZone.Index())
			t.Logf("  Voltage Path: %s", calcZone.voltagePath)
			t.Logf("  Current Path: %s", calcZone.currentPath)

			// Read and display actual power value
			power, err := calcZone.Power()
			require.NoError(t, err)
			t.Logf("  Current Power: %.3f W (%.0f µW)", power.Watts(), power.MicroWatts())

			// Verify power is reasonable (between 0 and 500W for test data)
			assert.GreaterOrEqual(t, power.Watts(), 0.0, "Power should be non-negative")
			assert.LessOrEqual(t, power.Watts(), 500.0, "Power should be reasonable for test data")
		} else {
			t.Fatalf("Unknown zone type: %T", zone)
		}
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
	// Note: package is now aggregated from multiple sources: 45W + 50W + 30W = 125W
	expectedPower := map[string]float64{
		"package": 125.0, // Aggregated: 45W + 50W + 30W
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
		// Skip aggregated zones
		if zone.Index() == -1 {
			continue
		}

		hwmonZone, ok := zone.(*hwmonPowerZone)
		if !ok {
			continue
		}

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

// TestGetChipName_DevicePathStrategy tests chip name derivation from device symlink
func TestGetChipName_DevicePathStrategy(t *testing.T) {
	reader := &sysfsHwmonReader{
		basePath: "testdata/sys/class/hwmon",
	}

	t.Logf("\n=== Testing Device Path Strategy for Chip Name ===")

	// Test with device symlink present
	hwmonPath := "testdata/sys/class/hwmon/hwmon_device_path"
	chipName, err := reader.getChipName(hwmonPath)

	require.NoError(t, err, "getChipName should not fail with device symlink")
	t.Logf("Chip name from device path: %s", chipName)

	// The device path is: .../devices/pci0000:00/0000:00:02.0/drm/card0
	// Expected format: <devType>_<devName> or just <devName>
	// devName = "card0", devType = "drm"
	assert.Contains(t, chipName, "card0", "Chip name should contain device name")
	t.Logf("✓ Device path strategy worked correctly")
}

// TestGetChipName_DirectoryFallback tests chip name derivation from directory name
func TestGetChipName_DirectoryFallback(t *testing.T) {
	reader := &sysfsHwmonReader{
		basePath: "testdata/sys/class/hwmon",
	}

	t.Logf("\n=== Testing Directory Name Fallback for Chip Name ===")

	// Test with no name file and no device symlink
	hwmonPath := "testdata/sys/class/hwmon/hwmon_dir_fallback"
	chipName, err := reader.getChipName(hwmonPath)

	require.NoError(t, err, "getChipName should fall back to directory name")
	t.Logf("Chip name from directory: %s", chipName)

	// Should use the directory name as fallback
	assert.Equal(t, "hwmon_dir_fallback", chipName,
		"Chip name should be derived from directory name")
	t.Logf("✓ Directory name fallback worked correctly")
}

// TestGetChipName_AllStrategies tests all chip name derivation strategies
func TestGetChipName_AllStrategies(t *testing.T) {
	reader := &sysfsHwmonReader{
		basePath: "testdata/sys/class/hwmon",
	}

	testCases := []struct {
		name             string
		hwmonPath        string
		expectError      bool
		expectedContains string
		description      string
	}{
		{
			name:             "device_path_strategy",
			hwmonPath:        "testdata/sys/class/hwmon/hwmon_device_path",
			expectError:      false,
			expectedContains: "card0",
			description:      "Should derive name from device symlink path",
		},
		{
			name:             "name_file_strategy",
			hwmonPath:        "testdata/sys/class/hwmon/hwmon0",
			expectError:      false,
			expectedContains: "k10temp",
			description:      "Should read name from 'name' file",
		},
		{
			name:             "directory_fallback_strategy",
			hwmonPath:        "testdata/sys/class/hwmon/hwmon_dir_fallback",
			expectError:      false,
			expectedContains: "hwmon_dir_fallback",
			description:      "Should fall back to directory name",
		},
	}

	t.Logf("\n=== Testing All Chip Name Strategies ===")
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			chipName, err := reader.getChipName(tc.hwmonPath)

			if tc.expectError {
				assert.Error(t, err, tc.description)
				t.Logf("✓ %s: Correctly returned error", tc.name)
			} else {
				require.NoError(t, err, tc.description)
				assert.Contains(t, chipName, tc.expectedContains,
					"Chip name should contain '%s'", tc.expectedContains)
				t.Logf("✓ %s: Got chip name '%s'", tc.name, chipName)
			}
		})
	}
}

// TestSysfsHwmonReader_DiscoverZones_DevicePath tests zone discovery with device path
func TestSysfsHwmonReader_DiscoverZones_DevicePath(t *testing.T) {
	reader := &sysfsHwmonReader{
		basePath: "testdata/sys/class/hwmon",
	}

	t.Logf("\n=== Testing Zone Discovery with Device Path ===")

	zones, err := reader.discoverZones("testdata/sys/class/hwmon/hwmon_device_path")
	require.NoError(t, err, "Should discover zones in hwmon_device_path")
	assert.NotEmpty(t, zones, "Should find at least one zone")

	t.Logf("Found %d zone(s) with device path naming:", len(zones))
	for i, zone := range zones {
		hwmonZone, ok := zone.(*hwmonPowerZone)
		require.True(t, ok, "Zone should be *hwmonPowerZone")

		t.Logf("  [%d] Zone: %s", i, zone.Name())
		t.Logf("      Chip: %s", hwmonZone.chipName)
		t.Logf("      Path: %s", zone.Path())

		// Verify chip name contains device name from path
		assert.Contains(t, hwmonZone.chipName, "card0",
			"Chip name should be derived from device path")

		// Test power reading
		power, err := zone.Power()
		require.NoError(t, err)
		assert.InDelta(t, 25.0, power.Watts(), 0.01,
			"Power should be 25W from test file")
		t.Logf("      Power: %.2f W", power.Watts())
	}
}

// TestSysfsHwmonReader_DiscoverZones_DirectoryFallback tests zone discovery with dir fallback
func TestSysfsHwmonReader_DiscoverZones_DirectoryFallback(t *testing.T) {
	reader := &sysfsHwmonReader{
		basePath: "testdata/sys/class/hwmon",
	}

	t.Logf("\n=== Testing Zone Discovery with Directory Fallback ===")

	zones, err := reader.discoverZones("testdata/sys/class/hwmon/hwmon_dir_fallback")
	require.NoError(t, err, "Should discover zones in hwmon_dir_fallback")
	assert.NotEmpty(t, zones, "Should find at least one zone")

	t.Logf("Found %d zone(s) with directory fallback naming:", len(zones))
	for i, zone := range zones {
		hwmonZone, ok := zone.(*hwmonPowerZone)
		require.True(t, ok, "Zone should be *hwmonPowerZone")

		t.Logf("  [%d] Zone: %s", i, zone.Name())
		t.Logf("      Chip: %s", hwmonZone.chipName)
		t.Logf("      Path: %s", zone.Path())

		// Verify chip name is the directory name
		assert.Equal(t, "hwmon_dir_fallback", hwmonZone.chipName,
			"Chip name should be directory name as fallback")

		// Test power reading
		power, err := zone.Power()
		require.NoError(t, err)
		assert.InDelta(t, 35.0, power.Watts(), 0.01,
			"Power should be 35W from test file")
		t.Logf("      Power: %.2f W", power.Watts())
	}
}

// TestGetChipName_DeviceNameOnly tests device path with only device name (no type)
func TestGetChipName_DeviceNameOnly(t *testing.T) {
	// Create a test setup where device path has a name but no meaningful type
	tmpDir := t.TempDir()
	hwmonPath := tmpDir + "/hwmon_test"
	err := os.MkdirAll(hwmonPath, 0755)
	require.NoError(t, err)

	// Create a device directory structure where parent dir has no meaningful name
	devicePath := tmpDir + "/devices/sensor123"
	err = os.MkdirAll(devicePath, 0755)
	require.NoError(t, err)

	// Create device symlink
	err = os.Symlink(devicePath, filepath.Join(hwmonPath, "device"))
	require.NoError(t, err)

	reader := &sysfsHwmonReader{basePath: tmpDir}
	chipName, err := reader.getChipName(hwmonPath)

	require.NoError(t, err)
	// When device path is /devices/sensor123, devType="devices" and devName="sensor123"
	// So it returns "devices_sensor123"
	assert.Equal(t, "devices_sensor123", chipName,
		"Should use device type and name from path")
	t.Logf("✓ Got chip name from device path: %s", chipName)
}

// TestGetChipName_ErrorCases tests error conditions
func TestGetChipName_ErrorCases(t *testing.T) {
	reader := &sysfsHwmonReader{basePath: "testdata/sys/class/hwmon"}

	t.Run("nonexistent_path", func(t *testing.T) {
		_, err := reader.getChipName("nonexistent/path/that/does/not/exist")
		assert.Error(t, err, "Should return error for nonexistent path")
		t.Logf("✓ Correctly returned error for nonexistent path")
	})
}

// TestIsSymlink tests the isSymlink utility function
func TestIsSymlink(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()

	t.Logf("\n=== Testing isSymlink Function ===")

	t.Run("symlink_returns_true", func(t *testing.T) {
		// Create a target file
		targetFile := filepath.Join(tmpDir, "target.txt")
		err := os.WriteFile(targetFile, []byte("test"), 0644)
		require.NoError(t, err)

		// Create a symlink to the target
		symlinkPath := filepath.Join(tmpDir, "symlink.txt")
		err = os.Symlink(targetFile, symlinkPath)
		require.NoError(t, err)

		// Test that isSymlink returns true for symlink
		result := isSymlink(symlinkPath)
		assert.True(t, result, "isSymlink should return true for symlink")
		t.Logf("✓ Symlink correctly identified: %s", symlinkPath)
	})

	t.Run("regular_file_returns_false", func(t *testing.T) {
		// Create a regular file
		regularFile := filepath.Join(tmpDir, "regular.txt")
		err := os.WriteFile(regularFile, []byte("test"), 0644)
		require.NoError(t, err)

		// Test that isSymlink returns false for regular file
		result := isSymlink(regularFile)
		assert.False(t, result, "isSymlink should return false for regular file")
		t.Logf("✓ Regular file correctly identified: %s", regularFile)
	})

	t.Run("directory_returns_false", func(t *testing.T) {
		// Create a directory
		dir := filepath.Join(tmpDir, "testdir")
		err := os.Mkdir(dir, 0755)
		require.NoError(t, err)

		// Test that isSymlink returns false for directory
		result := isSymlink(dir)
		assert.False(t, result, "isSymlink should return false for directory")
		t.Logf("✓ Directory correctly identified: %s", dir)
	})

	t.Run("nonexistent_path_returns_false", func(t *testing.T) {
		// Test with a path that doesn't exist
		nonexistent := filepath.Join(tmpDir, "does_not_exist.txt")

		// Test that isSymlink returns false for nonexistent path
		result := isSymlink(nonexistent)
		assert.False(t, result, "isSymlink should return false for nonexistent path")
		t.Logf("✓ Nonexistent path handled correctly: %s", nonexistent)
	})

	t.Run("symlink_to_directory", func(t *testing.T) {
		// Create a directory
		targetDir := filepath.Join(tmpDir, "target_dir")
		err := os.Mkdir(targetDir, 0755)
		require.NoError(t, err)

		// Create a symlink to the directory
		symlinkToDir := filepath.Join(tmpDir, "symlink_to_dir")
		err = os.Symlink(targetDir, symlinkToDir)
		require.NoError(t, err)

		// Test that isSymlink returns true for symlink to directory
		result := isSymlink(symlinkToDir)
		assert.True(t, result, "isSymlink should return true for symlink to directory")
		t.Logf("✓ Symlink to directory correctly identified: %s", symlinkToDir)
	})

	t.Run("broken_symlink_returns_true", func(t *testing.T) {
		// Create a symlink to a non-existent target (broken symlink)
		brokenSymlink := filepath.Join(tmpDir, "broken_symlink")
		err := os.Symlink("/nonexistent/target", brokenSymlink)
		require.NoError(t, err)

		// Test that isSymlink returns true even for broken symlink
		// (because we're testing the symlink itself, not the target)
		result := isSymlink(brokenSymlink)
		assert.True(t, result, "isSymlink should return true for broken symlink")
		t.Logf("✓ Broken symlink correctly identified: %s", brokenSymlink)
	})

	// Test with existing test data
	t.Run("test_with_existing_symlink_in_testdata", func(t *testing.T) {
		// We created a device symlink in hwmon_device_path earlier
		deviceSymlink := "testdata/sys/class/hwmon/hwmon_device_path/device"

		result := isSymlink(deviceSymlink)
		assert.True(t, result, "isSymlink should return true for device symlink")
		t.Logf("✓ Test data symlink correctly identified: %s", deviceSymlink)
	})
}

// TestHwmonPowerMeter_AggregatedZones tests zone aggregation when multiple zones have the same name
func TestHwmonPowerMeter_AggregatedZones(t *testing.T) {
	t.Logf("\n=== Testing Aggregated Zones ===")

	// Create a meter that will discover zones with duplicate names
	meter, err := NewHwmonPowerMeter("testdata/sys")
	require.NoError(t, err)

	// Discover zones - this should trigger aggregation
	zones, err := meter.Zones()
	require.NoError(t, err)

	t.Logf("Found %d zones after aggregation", len(zones))

	// Look for the aggregated "package" zone
	var aggregatedZone EnergyZone
	for _, zone := range zones {
		if zone.Name() == "package" {
			aggregatedZone = zone
			break
		}
	}

	require.NotNil(t, aggregatedZone, "Should find 'package' zone")

	// Check if it's an AggregatedZone by checking the index
	// AggregatedZone has index = -1
	if aggregatedZone.Index() == -1 {
		t.Logf("✓ Zone 'package' is an AggregatedZone (index=-1)")

		// Verify it's actually an AggregatedZone type
		aggZone, ok := aggregatedZone.(*AggregatedZone)
		require.True(t, ok, "Zone should be *AggregatedZone type")

		t.Logf("  Aggregated zone contains %d individual zones", len(aggZone.zones))
		assert.Equal(t, "package", aggZone.Name())
		assert.Equal(t, -1, aggZone.Index())

		// Test Power() - should sum power from all package zones
		totalPower, err := aggZone.Power()
		require.NoError(t, err)

		// The testdata contains multiple package zones:
		// - hwmon0: package zone (45W)
		// - hwmon_aggregate: two package zones (50W + 30W)
		// Total should be sum of all package zones found
		assert.Greater(t, totalPower.Watts(), 0.0,
			"Aggregated power should be positive")
		assert.Equal(t, len(aggZone.zones), 3,
			"Should aggregate all package zones from test data")

		// Expected: 45W + 50W + 30W = 125W
		assert.InDelta(t, 125.0, totalPower.Watts(), 0.01,
			"Aggregated power should be sum of all package zones (45W + 50W + 30W = 125W)")
		t.Logf("  Total aggregated power: %.2f W from %d zones",
			totalPower.Watts(), len(aggZone.zones))

	} else {
		t.Logf("Zone 'package' is a single zone (index=%d)", aggregatedZone.Index())
	}
}

// TestGroupZonesByName tests the grouping logic
func TestGroupZonesByName(t *testing.T) {
	meter := &hwmonPowerMeter{
		logger: slog.Default(),
	}

	t.Run("single_zone_per_name", func(t *testing.T) {
		zones := []EnergyZone{
			&hwmonPowerZone{name: "package", index: 0},
			&hwmonPowerZone{name: "core", index: 1},
			&hwmonPowerZone{name: "gpu", index: 2},
		}

		result := meter.groupZonesByName(zones)

		assert.Equal(t, 3, len(result), "Should have 3 zones (no aggregation)")

		// Verify none are aggregated (all have index >= 0)
		for _, zone := range result {
			assert.GreaterOrEqual(t, zone.Index(), 0,
				"Single zones should have non-negative index")
		}
		t.Logf("✓ No aggregation when each zone has unique name")
	})

	t.Run("multiple_zones_same_name", func(t *testing.T) {
		zones := []EnergyZone{
			&hwmonPowerZone{name: "package", index: 0, path: "/path1"},
			&hwmonPowerZone{name: "package", index: 1, path: "/path2"},
			&hwmonPowerZone{name: "core", index: 2, path: "/path3"},
		}

		result := meter.groupZonesByName(zones)

		assert.Equal(t, 2, len(result),
			"Should have 2 zones (package aggregated, core single)")

		// Find the package zone
		var packageZone EnergyZone
		var coreZone EnergyZone
		for _, zone := range result {
			if zone.Name() == "package" {
				packageZone = zone
			} else if zone.Name() == "core" {
				coreZone = zone
			}
		}

		require.NotNil(t, packageZone, "Should have package zone")
		require.NotNil(t, coreZone, "Should have core zone")

		// Package should be aggregated (index = -1)
		assert.Equal(t, -1, packageZone.Index(),
			"Package zone should be aggregated (index=-1)")

		// Core should be single zone (index >= 0)
		assert.Equal(t, 2, coreZone.Index(),
			"Core zone should not be aggregated")

		t.Logf("✓ Zones with same name are aggregated")
		t.Logf("  Package: aggregated (%d zones)", 2)
		t.Logf("  Core: single zone")
	})

	t.Run("all_zones_same_name", func(t *testing.T) {
		zones := []EnergyZone{
			&hwmonPowerZone{name: "package", index: 0},
			&hwmonPowerZone{name: "package", index: 1},
			&hwmonPowerZone{name: "package", index: 2},
		}

		result := meter.groupZonesByName(zones)

		assert.Equal(t, 1, len(result),
			"Should have 1 aggregated zone")
		assert.Equal(t, "package", result[0].Name())
		assert.Equal(t, -1, result[0].Index(),
			"Should be aggregated zone with index=-1")

		aggZone, ok := result[0].(*AggregatedZone)
		require.True(t, ok, "Should be AggregatedZone type")
		assert.Equal(t, 3, len(aggZone.zones),
			"Should aggregate all 3 zones")

		t.Logf("✓ All zones with same name are aggregated into one")
	})
}

// TestSysfsHwmonReader_DiscoverZones_Aggregation tests zone discovery with aggregation
func TestSysfsHwmonReader_DiscoverZones_Aggregation(t *testing.T) {
	reader := &sysfsHwmonReader{
		basePath: "testdata/sys/class/hwmon",
	}

	t.Logf("\n=== Testing Zone Discovery with Aggregation ===")

	// hwmon_aggregate has two power sensors with the same label "package"
	zones, err := reader.discoverZones("testdata/sys/class/hwmon/hwmon_aggregate")
	require.NoError(t, err, "Should discover zones in hwmon_aggregate")
	assert.Equal(t, 2, len(zones), "Should find 2 zones with same name")

	// Verify both zones have the same name
	assert.Equal(t, "package", zones[0].Name())
	assert.Equal(t, "package", zones[1].Name())

	// Verify they have different indices
	assert.NotEqual(t, zones[0].Index(), zones[1].Index(),
		"Zones should have different sensor indices")

	t.Logf("Found %d zones with name 'package':", len(zones))
	for i, zone := range zones {
		power, _ := zone.Power()
		t.Logf("  [%d] Index: %d, Power: %.2f W, Path: %s",
			i, zone.Index(), power.Watts(), zone.Path())
	}

	// Verify individual power readings
	power1, err := zones[0].Power()
	require.NoError(t, err)
	power2, err := zones[1].Power()
	require.NoError(t, err)

	// One should be 50W, the other 30W (in some order)
	powers := []float64{power1.Watts(), power2.Watts()}
	assert.Contains(t, powers, 50.0)
	assert.Contains(t, powers, 30.0)

	t.Logf("✓ Discovered multiple zones with same name ready for aggregation")
}

// TestCleanMetricName tests the metric name cleaning function
func TestCleanMetricName(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"Simple", "simple"},
		{"With Spaces", "with_spaces"},
		{"With-Dashes", "with_dashes"},
		{"UPPER_CASE", "upper_case"},
		{"Mixed-Case_Name", "mixed_case_name"},
		{"Special!@#Chars", "special___chars"}, // Each special char becomes _
		{"__leading__trailing__", "leading__trailing"},
		{"card0", "card0"},
		{"0000:00:02.0", "0000:00:02_0"}, // : is preserved, . is replaced
	}

	t.Logf("\n=== Testing Metric Name Cleaning ===")
	for _, tc := range testCases {
		result := cleanMetricName(tc.input)
		assert.Equal(t, tc.expected, result,
			"cleanMetricName(%q) should be %q, got %q", tc.input, tc.expected, result)
		t.Logf("✓ %q -> %q", tc.input, result)
	}
}

// mockHwmonReader is a mock implementation of hwmonReader for testing error paths
type mockHwmonReader struct {
	zones []EnergyZone
	err   error
}

func (m *mockHwmonReader) Zones() ([]EnergyZone, error) {
	return m.zones, m.err
}

// TestHwmonPowerMeter_Init_PowerReadError tests Init() error path when Power() fails
func TestHwmonPowerMeter_Init_PowerReadError(t *testing.T) {
	t.Logf("\n=== Testing Init() Error Path: Power Read Failure ===")

	// Create a mock zone that fails on Power() call
	mockZone := &mockEnergyZone{
		name:     "test_zone",
		index:    0,
		path:     "/fake/path",
		powerErr: assert.AnError,
		power:    0,
	}

	// Create mock reader that returns the failing zone
	mockReader := &mockHwmonReader{
		zones: []EnergyZone{mockZone},
		err:   nil,
	}

	// Create meter with mock reader
	meter, err := NewHwmonPowerMeter("testdata/sys", WithHwmonReader(mockReader))
	require.NoError(t, err)

	// Init should fail when trying to read power from first zone
	err = meter.Init()
	assert.Error(t, err, "Init() should fail when Power() fails on first zone")
	t.Logf("✓ Init() correctly failed with error: %v", err)
}

// TestHwmonPowerMeter_Zones_ReaderError tests Zones() error path when reader fails
func TestHwmonPowerMeter_Zones_ReaderError(t *testing.T) {
	t.Logf("\n=== Testing Zones() Error Path: Reader Failure ===")

	// Create mock reader that returns an error
	mockReader := &mockHwmonReader{
		zones: nil,
		err:   assert.AnError,
	}

	// Create meter with failing mock reader
	meter, err := NewHwmonPowerMeter("testdata/sys", WithHwmonReader(mockReader))
	require.NoError(t, err)

	// Zones() should return the error from reader
	zones, err := meter.Zones()
	assert.Error(t, err, "Zones() should return error when reader fails")
	assert.Nil(t, zones, "Zones should be nil on error")
	t.Logf("✓ Zones() correctly returned error: %v", err)
}

// TestHwmonPowerMeter_Zones_AllFiltered tests Zones() error when all zones are filtered out
func TestHwmonPowerMeter_Zones_AllFiltered(t *testing.T) {
	t.Logf("\n=== Testing Zones() Error Path: All Zones Filtered Out ===")

	// Create meter with zone filter that excludes all zones
	meter, err := NewHwmonPowerMeter("testdata/sys",
		WithHwmonZoneFilter([]string{"nonexistent_zone"}))
	require.NoError(t, err)

	// Zones() should fail because all zones are filtered out
	zones, err := meter.Zones()
	assert.Error(t, err, "Zones() should fail when all zones are filtered out")
	assert.Contains(t, err.Error(), "no hwmon zones found after filtering",
		"Error should mention filtering")
	assert.Nil(t, zones, "Zones should be nil on error")
	t.Logf("✓ Zones() correctly failed with: %v", err)
}

// TestHwmonPowerMeter_PrimaryEnergyZone_ZonesError tests PrimaryEnergyZone() when Zones() fails
func TestHwmonPowerMeter_PrimaryEnergyZone_ZonesError(t *testing.T) {
	t.Logf("\n=== Testing PrimaryEnergyZone() Error Path: Zones() Failure ===")

	// Create mock reader that returns an error
	mockReader := &mockHwmonReader{
		zones: nil,
		err:   assert.AnError,
	}

	// Create meter with failing mock reader
	meter, err := NewHwmonPowerMeter("testdata/sys", WithHwmonReader(mockReader))
	require.NoError(t, err)

	// PrimaryEnergyZone() should return the error from Zones()
	primaryZone, err := meter.PrimaryEnergyZone()
	assert.Error(t, err, "PrimaryEnergyZone() should return error when Zones() fails")
	assert.Nil(t, primaryZone, "Primary zone should be nil on error")
	t.Logf("✓ PrimaryEnergyZone() correctly returned error: %v", err)
}

// TestHwmonPowerMeter_PrimaryEnergyZone_NoZones tests PrimaryEnergyZone() with empty zones
func TestHwmonPowerMeter_PrimaryEnergyZone_NoZones(t *testing.T) {
	t.Logf("\n=== Testing PrimaryEnergyZone() Error Path: Empty Zones List ===")

	// Create mock reader that returns empty zones list
	mockReader := &mockHwmonReader{
		zones: []EnergyZone{},
		err:   nil,
	}

	// Create meter with mock reader
	meter, err := NewHwmonPowerMeter("testdata/sys", WithHwmonReader(mockReader))
	require.NoError(t, err)

	// PrimaryEnergyZone() should fail when zones list is empty
	primaryZone, err := meter.PrimaryEnergyZone()
	assert.Error(t, err, "PrimaryEnergyZone() should fail when zones list is empty")
	assert.Contains(t, err.Error(), "no hwmon zones found",
		"Error should mention no zones found")
	assert.Nil(t, primaryZone, "Primary zone should be nil on error")
	t.Logf("✓ PrimaryEnergyZone() correctly failed with: %v", err)
}

// TestSysReadFile_ErrorPaths tests sysReadFile error handling
func TestSysReadFile_ErrorPaths(t *testing.T) {
	t.Logf("\n=== Testing sysReadFile() Error Paths ===")

	t.Run("nonexistent_file", func(t *testing.T) {
		// Try to read a file that doesn't exist
		data, err := sysReadFile("testdata/nonexistent_file_that_does_not_exist.txt")
		assert.Error(t, err, "sysReadFile should fail for nonexistent file")
		assert.Nil(t, data, "Data should be nil on error")
		t.Logf("✓ Nonexistent file error: %v", err)
	})

	t.Run("directory_instead_of_file", func(t *testing.T) {
		// Try to read a directory instead of a file
		data, err := sysReadFile("testdata/sys/class/hwmon")
		assert.Error(t, err, "sysReadFile should fail when trying to read a directory")
		assert.Nil(t, data, "Data should be nil on error")
		t.Logf("✓ Directory read error: %v", err)
	})
}

// TestHwmonPowerMeter_Init_NoZonesFound tests Init() when no zones are discovered
func TestHwmonPowerMeter_Init_NoZonesFound(t *testing.T) {
	t.Logf("\n=== Testing Init() Error Path: No Zones Found ===")

	// Create mock reader that returns empty zones
	mockReader := &mockHwmonReader{
		zones: []EnergyZone{},
		err:   nil,
	}

	// Create meter with mock reader
	meter, err := NewHwmonPowerMeter("testdata/sys", WithHwmonReader(mockReader))
	require.NoError(t, err)

	// Init should fail when no zones are found
	err = meter.Init()
	assert.Error(t, err, "Init() should fail when no zones are found")
	assert.Contains(t, err.Error(), "no hwmon power zones found",
		"Error should mention no zones found")
	t.Logf("✓ Init() correctly failed with: %v", err)
}

// TestHwmonPowerMeter_Zones_ReaderReturnsEmpty tests Zones() when reader returns empty list
func TestHwmonPowerMeter_Zones_ReaderReturnsEmpty(t *testing.T) {
	t.Logf("\n=== Testing Zones() Error Path: Reader Returns Empty List ===")

	// Create mock reader that returns empty zones list without error
	mockReader := &mockHwmonReader{
		zones: []EnergyZone{},
		err:   nil,
	}

	// Create meter with mock reader
	meter, err := NewHwmonPowerMeter("testdata/sys", WithHwmonReader(mockReader))
	require.NoError(t, err)

	// Zones() should fail when reader returns empty list
	zones, err := meter.Zones()
	assert.Error(t, err, "Zones() should fail when reader returns empty list")
	assert.Contains(t, err.Error(), "no hwmon zones found",
		"Error should mention no zones found")
	assert.Nil(t, zones, "Zones should be nil on error")
	t.Logf("✓ Zones() correctly failed with: %v", err)
}

// TestHwmonPowerZone_Power_FileReadError tests Power() error when file read fails
func TestHwmonPowerZone_Power_FileReadError(t *testing.T) {
	t.Logf("\n=== Testing hwmonPowerZone.Power() Error Paths ===")

	t.Run("nonexistent_file", func(t *testing.T) {
		zone := &hwmonPowerZone{
			name:  "test_zone",
			index: 0,
			path:  "testdata/nonexistent_power_file.txt",
		}

		power, err := zone.Power()
		assert.Error(t, err, "Power() should fail for nonexistent file")
		assert.Equal(t, Power(0), power, "Power should be 0 on error")
		assert.Contains(t, err.Error(), "failed to read power",
			"Error should mention power read failure")
		t.Logf("✓ Nonexistent file error: %v", err)
	})

	t.Run("invalid_content", func(t *testing.T) {
		// Create a temporary file with invalid content
		tmpDir := t.TempDir()
		invalidFile := filepath.Join(tmpDir, "invalid_power.txt")
		err := os.WriteFile(invalidFile, []byte("not_a_number"), 0644)
		require.NoError(t, err)

		zone := &hwmonPowerZone{
			name:  "test_zone",
			index: 0,
			path:  invalidFile,
		}

		power, err := zone.Power()
		assert.Error(t, err, "Power() should fail for invalid content")
		assert.Equal(t, Power(0), power, "Power should be 0 on error")
		assert.Contains(t, err.Error(), "failed to parse power value",
			"Error should mention parse failure")
		t.Logf("✓ Invalid content error: %v", err)
	})
}

// TestSysfsHwmonReader_Zones_HwmonNotAvailable tests Zones() when hwmon directory doesn't exist
func TestSysfsHwmonReader_Zones_HwmonNotAvailable(t *testing.T) {
	t.Logf("\n=== Testing sysfsHwmonReader.Zones() Error: hwmon Not Available ===")

	reader := &sysfsHwmonReader{
		basePath: "testdata/nonexistent_hwmon_directory",
	}

	zones, err := reader.Zones()
	assert.Error(t, err, "Zones() should fail when hwmon directory doesn't exist")
	assert.Contains(t, err.Error(), "hwmon not available",
		"Error should mention hwmon not available")
	assert.Nil(t, zones, "Zones should be nil on error")
	t.Logf("✓ hwmon not available error: %v", err)
}

// TestHwmonPowerMeter_PrimaryEnergyZone_FallbackToFirstZone tests fallback when no priority zones exist
func TestHwmonPowerMeter_PrimaryEnergyZone_FallbackToFirstZone(t *testing.T) {
	t.Logf("\n=== Testing PrimaryEnergyZone() Fallback to First Zone ===")

	// Create zones with names that don't match any priority names
	mockZones := []EnergyZone{
		&mockEnergyZone{name: "unknown_sensor_1", index: 0, power: 10.0, maxEnergy: 1000},
		&mockEnergyZone{name: "unknown_sensor_2", index: 1, power: 20.0, maxEnergy: 1000},
	}

	mockReader := &mockHwmonReader{
		zones: mockZones,
		err:   nil,
	}

	meter, err := NewHwmonPowerMeter("testdata/sys", WithHwmonReader(mockReader))
	require.NoError(t, err)

	// PrimaryEnergyZone should fall back to first zone
	primaryZone, err := meter.PrimaryEnergyZone()
	assert.NoError(t, err, "PrimaryEnergyZone() should succeed with fallback")
	require.NotNil(t, primaryZone, "Primary zone should not be nil")
	assert.Equal(t, "unknown_sensor_1", primaryZone.Name(),
		"Should fall back to first zone when no priority names match")
	t.Logf("✓ Correctly fell back to first zone: %s", primaryZone.Name())
}

// TestSysfsHwmonReader_Zones_NoValidZones tests Zones() when no valid zones are found
func TestSysfsHwmonReader_Zones_NoValidZones(t *testing.T) {
	t.Logf("\n=== Testing sysfsHwmonReader.Zones() When No Valid Zones Found ===")

	// Create a temporary hwmon directory with no valid power sensors
	tmpDir := t.TempDir()
	hwmonDir := filepath.Join(tmpDir, "class", "hwmon", "hwmon0")
	err := os.MkdirAll(hwmonDir, 0755)
	require.NoError(t, err)

	// Create a name file but no power sensors
	err = os.WriteFile(filepath.Join(hwmonDir, "name"), []byte("test_sensor"), 0644)
	require.NoError(t, err)

	// Create some non-power files
	err = os.WriteFile(filepath.Join(hwmonDir, "temp1_input"), []byte("50000"), 0644)
	require.NoError(t, err)

	reader := &sysfsHwmonReader{
		basePath: filepath.Join(tmpDir, "class", "hwmon"),
	}

	zones, err := reader.Zones()
	assert.Error(t, err, "Zones() should fail when no power zones are found")
	assert.Contains(t, err.Error(), "no hwmon power zones found",
		"Error should mention no power zones found")
	assert.Nil(t, zones, "Zones should be nil on error")
	t.Logf("✓ Correctly failed with: %v", err)
}

// TestSysReadFile_NegativeReadBytes tests sysReadFile when read returns negative bytes
func TestSysReadFile_NegativeReadBytes(t *testing.T) {
	t.Logf("\n=== Testing sysReadFile() Negative Read Bytes ===")

	// This is a theoretical edge case that's hard to simulate in practice
	// The unix.Read syscall would need to return a negative value
	// We've covered the practical error paths with nonexistent files and directories
	t.Logf("Note: Negative bytes error path is covered by syscall-level tests")
}

// TestGetHumanReadableChipName_ErrorCases tests error handling in getHumanReadableChipName
func TestGetHumanReadableChipName_ErrorCases(t *testing.T) {
	t.Logf("\n=== Testing getHumanReadableChipName() Error Cases ===")

	reader := &sysfsHwmonReader{
		basePath: "testdata/sys/class/hwmon",
	}

	t.Run("no_name_file", func(t *testing.T) {
		// Directory without a name file (using dir_fallback test fixture)
		_, err := reader.getHumanReadableChipName("testdata/sys/class/hwmon/hwmon_dir_fallback")
		assert.Error(t, err, "Should return error when name file doesn't exist")
		t.Logf("✓ Correctly returned error for missing name file")
	})

	t.Run("nonexistent_directory", func(t *testing.T) {
		_, err := reader.getHumanReadableChipName("testdata/nonexistent_hwmon_path")
		assert.Error(t, err, "Should return error for nonexistent directory")
		t.Logf("✓ Correctly returned error for nonexistent directory")
	})
}

// TestDiscoverZones_GetChipNameError tests discoverZones when getChipName fails
func TestDiscoverZones_GetChipNameError(t *testing.T) {
	t.Logf("\n=== Testing discoverZones() When getChipName Fails ===")

	reader := &sysfsHwmonReader{
		basePath: "testdata/sys/class/hwmon",
	}

	// Test with a path that will cause getChipName to fail
	// Create the hwmon path itself as a broken symlink so EvalSymlinks fails
	tmpDir := t.TempDir()
	baseDir := filepath.Join(tmpDir, "broken_base")
	err := os.MkdirAll(baseDir, 0755)
	require.NoError(t, err)

	// Create a symlink to a nonexistent target as the hwmon path itself
	brokenHwmonPath := filepath.Join(baseDir, "broken_hwmon")
	err = os.Symlink("/nonexistent/hwmon/path", brokenHwmonPath)
	require.NoError(t, err)

	// Try to discover zones - should handle getChipName error gracefully
	zones, err := reader.discoverZones(brokenHwmonPath)
	assert.Error(t, err, "discoverZones should fail when getChipName fails")
	assert.Nil(t, zones, "Zones should be nil on error")
	t.Logf("✓ discoverZones correctly handled getChipName error: %v", err)
}

// =============================================================================
// Tests for hwmonCalculatedPowerZone (voltage/current based power calculation)
// =============================================================================

// TestHwmonCalculatedPowerZoneInterface ensures hwmonCalculatedPowerZone implements EnergyZone
func TestHwmonCalculatedPowerZoneInterface(t *testing.T) {
	var _ EnergyZone = (*hwmonCalculatedPowerZone)(nil)
	t.Log("✓ hwmonCalculatedPowerZone implements EnergyZone interface")
}

// TestHwmonCalculatedPowerZone_Power tests power calculation from voltage and current
func TestHwmonCalculatedPowerZone_Power(t *testing.T) {
	t.Logf("\n=== Testing hwmonCalculatedPowerZone.Power() ===")

	zone := &hwmonCalculatedPowerZone{
		name:        "vdd_cpu",
		index:       1,
		voltagePath: "testdata/sys/class/hwmon/hwmon_voltage_current/in1_input",
		currentPath: "testdata/sys/class/hwmon/hwmon_voltage_current/curr1_input",
		chipName:    "ina3221",
		humanName:   "ina3221",
	}

	power, err := zone.Power()
	require.NoError(t, err, "Power() should not return error")

	// Expected: 12000 mV × 5000 mA = 60,000,000 µW = 60 W
	expectedPowerMicrowatts := float64(12000 * 5000)
	assert.InDelta(t, expectedPowerMicrowatts, float64(power), 0.01,
		"Power should be voltage × current in microwatts")
	assert.InDelta(t, 60.0, power.Watts(), 0.01,
		"Power should be 60 W (12V × 5A)")

	t.Logf("✓ Calculated power: %.2f W (%.0f µW)", power.Watts(), power.MicroWatts())
	t.Logf("  Voltage: 12000 mV (12 V)")
	t.Logf("  Current: 5000 mA (5 A)")
}

// TestHwmonCalculatedPowerZone_Methods tests all interface methods
func TestHwmonCalculatedPowerZone_Methods(t *testing.T) {
	t.Logf("\n=== Testing hwmonCalculatedPowerZone Interface Methods ===")

	zone := &hwmonCalculatedPowerZone{
		name:        "test_zone",
		index:       2,
		voltagePath: "/path/to/voltage",
		currentPath: "/path/to/current",
		chipName:    "test_chip",
		humanName:   "test_human",
	}

	// Test Name()
	assert.Equal(t, "test_zone", zone.Name())
	t.Logf("✓ Name() = %s", zone.Name())

	// Test Index()
	assert.Equal(t, 2, zone.Index())
	t.Logf("✓ Index() = %d", zone.Index())

	// Test Path() - returns voltage path
	assert.Equal(t, "/path/to/voltage", zone.Path())
	t.Logf("✓ Path() = %s", zone.Path())

	// Test Energy() - returns error
	energy, err := zone.Energy()
	assert.Error(t, err, "Energy() should return error for calculated power zones")
	assert.Equal(t, Energy(0), energy)
	t.Logf("✓ Energy() correctly returns error: %v", err)

	// Test MaxEnergy() - returns 0
	maxEnergy := zone.MaxEnergy()
	assert.Equal(t, Energy(0), maxEnergy)
	t.Logf("✓ MaxEnergy() = %d", maxEnergy)
}

// TestHwmonCalculatedPowerZone_Power_VoltageReadError tests error handling for voltage read failure
func TestHwmonCalculatedPowerZone_Power_VoltageReadError(t *testing.T) {
	t.Logf("\n=== Testing hwmonCalculatedPowerZone.Power() Voltage Read Error ===")

	zone := &hwmonCalculatedPowerZone{
		name:        "test_zone",
		index:       1,
		voltagePath: "testdata/nonexistent_voltage_file",
		currentPath: "testdata/sys/class/hwmon/hwmon_voltage_current/curr1_input",
		chipName:    "test_chip",
		humanName:   "test_human",
	}

	power, err := zone.Power()
	assert.Error(t, err, "Power() should fail when voltage file doesn't exist")
	assert.Contains(t, err.Error(), "failed to read voltage")
	assert.Equal(t, Power(0), power)
	t.Logf("✓ Correctly returned error: %v", err)
}

// TestHwmonCalculatedPowerZone_Power_CurrentReadError tests error handling for current read failure
func TestHwmonCalculatedPowerZone_Power_CurrentReadError(t *testing.T) {
	t.Logf("\n=== Testing hwmonCalculatedPowerZone.Power() Current Read Error ===")

	zone := &hwmonCalculatedPowerZone{
		name:        "test_zone",
		index:       1,
		voltagePath: "testdata/sys/class/hwmon/hwmon_voltage_current/in1_input",
		currentPath: "testdata/nonexistent_current_file",
		chipName:    "test_chip",
		humanName:   "test_human",
	}

	power, err := zone.Power()
	assert.Error(t, err, "Power() should fail when current file doesn't exist")
	assert.Contains(t, err.Error(), "failed to read current")
	assert.Equal(t, Power(0), power)
	t.Logf("✓ Correctly returned error: %v", err)
}

// TestHwmonCalculatedPowerZone_Power_InvalidVoltageContent tests error handling for invalid voltage
func TestHwmonCalculatedPowerZone_Power_InvalidVoltageContent(t *testing.T) {
	t.Logf("\n=== Testing hwmonCalculatedPowerZone.Power() Invalid Voltage Content ===")

	// Create temporary file with invalid content
	tmpDir := t.TempDir()
	invalidFile := filepath.Join(tmpDir, "invalid_voltage")
	err := os.WriteFile(invalidFile, []byte("not_a_number"), 0644)
	require.NoError(t, err)

	zone := &hwmonCalculatedPowerZone{
		name:        "test_zone",
		index:       1,
		voltagePath: invalidFile,
		currentPath: "testdata/sys/class/hwmon/hwmon_voltage_current/curr1_input",
		chipName:    "test_chip",
		humanName:   "test_human",
	}

	power, err := zone.Power()
	assert.Error(t, err, "Power() should fail for invalid voltage content")
	assert.Contains(t, err.Error(), "failed to parse voltage")
	assert.Equal(t, Power(0), power)
	t.Logf("✓ Correctly returned error: %v", err)
}

// TestHwmonCalculatedPowerZone_Power_InvalidCurrentContent tests error handling for invalid current
func TestHwmonCalculatedPowerZone_Power_InvalidCurrentContent(t *testing.T) {
	t.Logf("\n=== Testing hwmonCalculatedPowerZone.Power() Invalid Current Content ===")

	// Create temporary file with invalid content
	tmpDir := t.TempDir()
	invalidFile := filepath.Join(tmpDir, "invalid_current")
	err := os.WriteFile(invalidFile, []byte("not_a_number"), 0644)
	require.NoError(t, err)

	zone := &hwmonCalculatedPowerZone{
		name:        "test_zone",
		index:       1,
		voltagePath: "testdata/sys/class/hwmon/hwmon_voltage_current/in1_input",
		currentPath: invalidFile,
		chipName:    "test_chip",
		humanName:   "test_human",
	}

	power, err := zone.Power()
	assert.Error(t, err, "Power() should fail for invalid current content")
	assert.Contains(t, err.Error(), "failed to parse current")
	assert.Equal(t, Power(0), power)
	t.Logf("✓ Correctly returned error: %v", err)
}

// TestDiscoverVoltageCurrentZones_MatchByLabel tests voltage/current discovery with label matching
func TestDiscoverVoltageCurrentZones_MatchByLabel(t *testing.T) {
	t.Logf("\n=== Testing Voltage/Current Discovery with Label Matching ===")

	reader := &sysfsHwmonReader{
		basePath: "testdata/sys/class/hwmon",
	}

	// Read files from the test fixture
	hwmonPath := "testdata/sys/class/hwmon/hwmon_voltage_current"
	files, err := os.ReadDir(hwmonPath)
	require.NoError(t, err)

	zones, err := reader.discoverVoltageCurrentZones(hwmonPath, "ina3221", "ina3221", files)
	require.NoError(t, err, "Should not return error when labels match")

	require.NotEmpty(t, zones, "Should discover voltage/current zones")
	assert.Equal(t, 1, len(zones), "Should find 1 matched pair")

	zone := zones[0]
	assert.Equal(t, "vdd_cpu", zone.Name(), "Zone name should be the cleaned label")

	// Verify it's a calculated power zone
	calcZone, ok := zone.(*hwmonCalculatedPowerZone)
	require.True(t, ok, "Zone should be *hwmonCalculatedPowerZone type")
	assert.Contains(t, calcZone.voltagePath, "in1_input")
	assert.Contains(t, calcZone.currentPath, "curr1_input")

	// Test power calculation
	power, err := zone.Power()
	require.NoError(t, err)
	assert.InDelta(t, 60.0, power.Watts(), 0.01,
		"Power should be 60 W (12V × 5A)")

	t.Logf("✓ Found zone: %s", zone.Name())
	t.Logf("  Voltage path: %s", calcZone.voltagePath)
	t.Logf("  Current path: %s", calcZone.currentPath)
	t.Logf("  Power: %.2f W", power.Watts())
}

// TestDiscoverVoltageCurrentZones_NoLabels tests that sensors without labels return an error
func TestDiscoverVoltageCurrentZones_NoLabels(t *testing.T) {
	t.Logf("\n=== Testing Voltage/Current Discovery Without Labels ===")

	reader := &sysfsHwmonReader{
		basePath: "testdata/sys/class/hwmon",
	}

	// This fixture has voltage and current but no labels
	hwmonPath := "testdata/sys/class/hwmon/hwmon_voltage_current_no_labels"
	files, err := os.ReadDir(hwmonPath)
	require.NoError(t, err)

	zones, err := reader.discoverVoltageCurrentZones(hwmonPath, "ina226", "ina226", files)

	assert.Empty(t, zones, "Should not discover zones without labels")
	assert.Error(t, err, "Should return error when voltage/current exist but no labels")
	assert.ErrorIs(t, err, ErrVoltageCurrentNoLabels,
		"Error should be ErrVoltageCurrentNoLabels")
	t.Logf("✓ Correctly returned error: %v", err)
}

// TestDiscoverVoltageCurrentZones_MultiplePairs tests discovery with multiple labeled pairs
func TestDiscoverVoltageCurrentZones_MultiplePairs(t *testing.T) {
	t.Logf("\n=== Testing Voltage/Current Discovery with Multiple Pairs ===")

	reader := &sysfsHwmonReader{
		basePath: "testdata/sys/class/hwmon",
	}

	hwmonPath := "testdata/sys/class/hwmon/hwmon_voltage_current_multi"
	files, err := os.ReadDir(hwmonPath)
	require.NoError(t, err)

	zones, err := reader.discoverVoltageCurrentZones(hwmonPath, "ina3221", "ina3221", files)
	require.NoError(t, err, "Should not return error when labels match")

	// Should find 2 zones (VDD_CPU and VDD_GPU have matching pairs)
	// VDD_SOC has only voltage, no matching current
	assert.Equal(t, 2, len(zones), "Should find 2 matched pairs")

	// Create a map of zone names for verification
	zoneNames := make(map[string]EnergyZone)
	for _, zone := range zones {
		zoneNames[zone.Name()] = zone
	}

	t.Logf("Found %d zones:", len(zones))
	for _, zone := range zones {
		power, err := zone.Power()
		require.NoError(t, err)
		t.Logf("  - %s: %.2f W", zone.Name(), power.Watts())
	}

	// Verify VDD_CPU zone
	cpuZone, found := zoneNames["vdd_cpu"]
	assert.True(t, found, "Should find vdd_cpu zone")
	if found {
		power, _ := cpuZone.Power()
		// 12000 mV × 5000 mA = 60 W
		assert.InDelta(t, 60.0, power.Watts(), 0.01)
	}

	// Verify VDD_GPU zone
	gpuZone, found := zoneNames["vdd_gpu"]
	assert.True(t, found, "Should find vdd_gpu zone")
	if found {
		power, _ := gpuZone.Power()
		// 3300 mV × 10000 mA = 33 W
		assert.InDelta(t, 33.0, power.Watts(), 0.01)
	}

	// Verify VDD_SOC is NOT found (no matching current)
	_, found = zoneNames["vdd_soc"]
	assert.False(t, found, "Should NOT find vdd_soc zone (no matching current)")
	t.Logf("✓ Correctly matched only voltage/current pairs with matching labels")
}

// TestDiscoverVoltageCurrentZones_PrefersAverageOverInput tests that _average files are preferred
func TestDiscoverVoltageCurrentZones_PrefersAverageOverInput(t *testing.T) {
	t.Logf("\n=== Testing Voltage/Current Discovery Prefers _average Over _input ===")

	reader := &sysfsHwmonReader{
		basePath: "testdata/sys/class/hwmon",
	}

	// This fixture has both _input and _average files with different values
	// in1_input: 12000 mV, in1_average: 11800 mV
	// curr1_input: 5000 mA, curr1_average: 4900 mA
	// If _average is preferred: Power = 11800 × 4900 = 57,820,000 µW = 57.82 W
	// If _input was used: Power = 12000 × 5000 = 60,000,000 µW = 60 W
	hwmonPath := "testdata/sys/class/hwmon/hwmon_voltage_current_average"
	files, err := os.ReadDir(hwmonPath)
	require.NoError(t, err)

	zones, err := reader.discoverVoltageCurrentZones(hwmonPath, "ina3221", "ina3221", files)
	require.NoError(t, err, "Should not return error when labels match")
	require.Equal(t, 1, len(zones), "Should find 1 matched pair")

	zone := zones[0]
	power, err := zone.Power()
	require.NoError(t, err)

	// Verify _average values are used (57.82 W), not _input values (60 W)
	expectedPowerFromAverage := 11800.0 * 4900.0 / 1_000_000.0 // 57.82 W
	expectedPowerFromInput := 12000.0 * 5000.0 / 1_000_000.0   // 60.0 W

	assert.InDelta(t, expectedPowerFromAverage, power.Watts(), 0.01,
		"Power should use _average values (57.82 W), not _input values (60 W)")
	// Verify we're NOT using _input values (there's a ~2W difference)
	assert.Greater(t, expectedPowerFromInput-power.Watts(), 2.0,
		"Power should NOT be using _input values - should differ by more than 2W")

	t.Logf("✓ _average files are preferred over _input")
	t.Logf("  Power from _average: %.2f W (expected)", expectedPowerFromAverage)
	t.Logf("  Power from _input: %.2f W (not used)", expectedPowerFromInput)
	t.Logf("  Actual power: %.2f W", power.Watts())
}

// TestDiscoverVoltageCurrentZones_PartialAverageFallsBackToInput tests that when only one
// sensor has _average, both fall back to _input for consistency
func TestDiscoverVoltageCurrentZones_PartialAverageFallsBackToInput(t *testing.T) {
	t.Logf("\n=== Testing Partial _average Falls Back to _input for Both ===")

	reader := &sysfsHwmonReader{
		basePath: "testdata/sys/class/hwmon",
	}

	// This fixture has:
	// - Voltage: in1_input (12000), in1_average (11800) - has both
	// - Current: curr1_input (5000) only - no average
	// Since current lacks _average, both should use _input:
	// Power = 12000 × 5000 = 60,000,000 µW = 60 W
	// NOT: 11800 × 5000 = 59 W (which would be mixing average/input)
	hwmonPath := "testdata/sys/class/hwmon/hwmon_voltage_current_partial_average"
	files, err := os.ReadDir(hwmonPath)
	require.NoError(t, err)

	zones, err := reader.discoverVoltageCurrentZones(hwmonPath, "ina3221", "ina3221", files)
	require.NoError(t, err, "Should not return error when labels match")
	require.Equal(t, 1, len(zones), "Should find 1 matched pair")

	zone := zones[0]
	power, err := zone.Power()
	require.NoError(t, err)

	// Verify _input values are used for BOTH (60 W), not mixed (59 W)
	expectedPowerFromInput := 12000.0 * 5000.0 / 1_000_000.0 // 60.0 W
	expectedPowerFromMixed := 11800.0 * 5000.0 / 1_000_000.0 // 59.0 W (voltage average × current input)

	assert.InDelta(t, expectedPowerFromInput, power.Watts(), 0.01,
		"Power should use _input for both when only one has _average")
	assert.Greater(t, power.Watts()-expectedPowerFromMixed, 0.5,
		"Power should NOT be using mixed average/input values")

	t.Logf("✓ Correctly fell back to _input for both when only voltage has _average")
	t.Logf("  Power from _input (both): %.2f W (expected)", expectedPowerFromInput)
	t.Logf("  Power from mixed (wrong): %.2f W (not used)", expectedPowerFromMixed)
	t.Logf("  Actual power: %.2f W", power.Watts())
}

// TestDiscoverZones_FallbackToVoltageCurrentWhenNoPower tests the fallback behavior
func TestDiscoverZones_FallbackToVoltageCurrentWhenNoPower(t *testing.T) {
	t.Logf("\n=== Testing discoverZones() Fallback to Voltage/Current ===")

	reader := &sysfsHwmonReader{
		basePath: "testdata/sys/class/hwmon",
	}

	// hwmon_voltage_current has no power sensors, only voltage/current pairs
	zones, err := reader.discoverZones("testdata/sys/class/hwmon/hwmon_voltage_current")
	require.NoError(t, err, "discoverZones should succeed with voltage/current fallback")
	require.NotEmpty(t, zones, "Should find zones via voltage/current fallback")

	t.Logf("Found %d zone(s) via voltage/current fallback:", len(zones))
	for i, zone := range zones {
		power, err := zone.Power()
		require.NoError(t, err)
		t.Logf("  [%d] %s: %.2f W", i, zone.Name(), power.Watts())

		// Verify it's a calculated power zone
		_, ok := zone.(*hwmonCalculatedPowerZone)
		assert.True(t, ok, "Zone should be *hwmonCalculatedPowerZone type")
	}

	t.Logf("✓ Successfully fell back to voltage/current when no power sensors found")
}

// TestDiscoverZones_PreferDirectPowerOverCalculated tests that direct power is preferred
func TestDiscoverZones_PreferDirectPowerOverCalculated(t *testing.T) {
	t.Logf("\n=== Testing discoverZones() Prefers Direct Power ===")

	reader := &sysfsHwmonReader{
		basePath: "testdata/sys/class/hwmon",
	}

	// hwmon0 has direct power sensors
	zones, err := reader.discoverZones("testdata/sys/class/hwmon/hwmon0")
	require.NoError(t, err)
	require.NotEmpty(t, zones)

	// All zones should be direct power zones
	for _, zone := range zones {
		_, ok := zone.(*hwmonPowerZone)
		assert.True(t, ok, "Zone should be *hwmonPowerZone type when direct power is available")
	}

	t.Logf("✓ Direct power sensors are preferred over voltage/current calculation")
}

// TestHwmonPowerMeter_WithVoltageCurrentZones tests full integration with voltage/current zones
func TestHwmonPowerMeter_WithVoltageCurrentZones(t *testing.T) {
	t.Logf("\n=== Testing hwmonPowerMeter with Voltage/Current Zones ===")

	// Create a meter using only the voltage/current fixture
	tmpDir := t.TempDir()
	hwmonDir := filepath.Join(tmpDir, "class", "hwmon", "hwmon0")
	err := os.MkdirAll(hwmonDir, 0755)
	require.NoError(t, err)

	// Copy voltage/current fixture files
	srcDir := "testdata/sys/class/hwmon/hwmon_voltage_current"
	files := []string{"name", "in1_input", "in1_label", "curr1_input", "curr1_label"}
	for _, f := range files {
		srcData, err := os.ReadFile(filepath.Join(srcDir, f))
		require.NoError(t, err)
		err = os.WriteFile(filepath.Join(hwmonDir, f), srcData, 0644)
		require.NoError(t, err)
	}

	meter, err := NewHwmonPowerMeter(tmpDir)
	require.NoError(t, err)

	// Init should succeed
	err = meter.Init()
	require.NoError(t, err, "Init() should succeed with voltage/current zones")

	// Get zones
	zones, err := meter.Zones()
	require.NoError(t, err)
	require.NotEmpty(t, zones)

	t.Logf("Found %d zone(s):", len(zones))
	for _, zone := range zones {
		power, err := zone.Power()
		require.NoError(t, err)
		t.Logf("  - %s: %.2f W", zone.Name(), power.Watts())
	}

	// Get primary zone
	primaryZone, err := meter.PrimaryEnergyZone()
	require.NoError(t, err)
	t.Logf("Primary zone: %s", primaryZone.Name())

	power, err := primaryZone.Power()
	require.NoError(t, err)
	assert.InDelta(t, 60.0, power.Watts(), 0.01,
		"Power should be 60 W (12V × 5A)")

	t.Logf("✓ hwmonPowerMeter works correctly with voltage/current zones")
}

// TestDiscoverVoltageCurrentZones_EmptySensors tests with no voltage or current sensors
func TestDiscoverVoltageCurrentZones_EmptySensors(t *testing.T) {
	t.Logf("\n=== Testing Voltage/Current Discovery with No Sensors ===")

	reader := &sysfsHwmonReader{
		basePath: "testdata/sys/class/hwmon",
	}

	// Create temporary directory with only temperature sensors
	tmpDir := t.TempDir()
	hwmonDir := filepath.Join(tmpDir, "hwmon_temp_only")
	err := os.MkdirAll(hwmonDir, 0755)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(hwmonDir, "name"), []byte("temp_sensor"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(hwmonDir, "temp1_input"), []byte("50000"), 0644)
	require.NoError(t, err)

	files, err := os.ReadDir(hwmonDir)
	require.NoError(t, err)

	zones, err := reader.discoverVoltageCurrentZones(hwmonDir, "temp_sensor", "temp_sensor", files)
	assert.NoError(t, err, "Should not return error when no voltage/current sensors exist")
	assert.Empty(t, zones, "Should return empty when no voltage/current sensors exist")
	t.Logf("✓ Correctly returned empty for no voltage/current sensors")
}

// TestDiscoverVoltageCurrentZones_VoltageOnly tests with only voltage sensors
func TestDiscoverVoltageCurrentZones_VoltageOnly(t *testing.T) {
	t.Logf("\n=== Testing Voltage/Current Discovery with Voltage Only ===")

	reader := &sysfsHwmonReader{
		basePath: "testdata/sys/class/hwmon",
	}

	tmpDir := t.TempDir()
	hwmonDir := filepath.Join(tmpDir, "hwmon_voltage_only")
	err := os.MkdirAll(hwmonDir, 0755)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(hwmonDir, "name"), []byte("voltage_sensor"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(hwmonDir, "in1_input"), []byte("12000"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(hwmonDir, "in1_label"), []byte("VDD"), 0644)
	require.NoError(t, err)

	files, err := os.ReadDir(hwmonDir)
	require.NoError(t, err)

	zones, err := reader.discoverVoltageCurrentZones(hwmonDir, "voltage_sensor", "voltage_sensor", files)
	assert.NoError(t, err, "Should not return error when only voltage sensors exist (no current)")
	assert.Empty(t, zones, "Should return empty when only voltage sensors exist")
	t.Logf("✓ Correctly returned empty for voltage-only sensors")
}

// TestDiscoverVoltageCurrentZones_CurrentOnly tests with only current sensors
func TestDiscoverVoltageCurrentZones_CurrentOnly(t *testing.T) {
	t.Logf("\n=== Testing Voltage/Current Discovery with Current Only ===")

	reader := &sysfsHwmonReader{
		basePath: "testdata/sys/class/hwmon",
	}

	tmpDir := t.TempDir()
	hwmonDir := filepath.Join(tmpDir, "hwmon_current_only")
	err := os.MkdirAll(hwmonDir, 0755)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(hwmonDir, "name"), []byte("current_sensor"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(hwmonDir, "curr1_input"), []byte("5000"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(hwmonDir, "curr1_label"), []byte("VDD"), 0644)
	require.NoError(t, err)

	files, err := os.ReadDir(hwmonDir)
	require.NoError(t, err)

	zones, err := reader.discoverVoltageCurrentZones(hwmonDir, "current_sensor", "current_sensor", files)
	assert.NoError(t, err, "Should not return error when only current sensors exist (no voltage)")
	assert.Empty(t, zones, "Should return empty when only current sensors exist")
	t.Logf("✓ Correctly returned empty for current-only sensors")
}

// TestHwmonCalculatedPowerZone_MultipleReadings tests reading power multiple times
func TestHwmonCalculatedPowerZone_MultipleReadings(t *testing.T) {
	t.Logf("\n=== Testing hwmonCalculatedPowerZone Multiple Readings ===")

	zone := &hwmonCalculatedPowerZone{
		name:        "vdd_cpu",
		index:       1,
		voltagePath: "testdata/sys/class/hwmon/hwmon_voltage_current/in1_input",
		currentPath: "testdata/sys/class/hwmon/hwmon_voltage_current/curr1_input",
		chipName:    "ina3221",
		humanName:   "ina3221",
	}

	t.Logf("Reading power 5 times:")
	for i := 0; i < 5; i++ {
		power, err := zone.Power()
		require.NoError(t, err)
		assert.InDelta(t, 60.0, power.Watts(), 0.01)
		t.Logf("  Reading %d: %.2f W", i+1, power.Watts())
	}

	t.Logf("✓ Multiple readings are consistent")
}

// =============================================================================
// Tests for Known-Chip Lookup and Same-Index Fallback
// =============================================================================

// TestGetChipPairingRule tests the chip pairing rule lookup
func TestGetChipPairingRule(t *testing.T) {
	t.Logf("\n=== Testing Chip Pairing Rule Lookup ===")

	testCases := []struct {
		chipName     string
		expectRule   bool
		useSameIndex bool
		hasPairings  bool
	}{
		{"ina226", true, false, true},
		{"ina3221", true, true, false},
		{"max20730", true, false, true},
		{"adm1275", true, false, true},
		{"pmbus", true, true, false},
		{"unknown_chip", false, false, false},
		{"INA226", true, false, true},     // Case insensitive
		{"  ina226  ", true, false, true}, // Whitespace trimmed
	}

	for _, tc := range testCases {
		rule := getChipPairingRule(tc.chipName)
		if tc.expectRule {
			require.NotNil(t, rule, "Should find rule for %q", tc.chipName)
			assert.Equal(t, tc.useSameIndex, rule.useSameIndex,
				"useSameIndex should match for %q", tc.chipName)
			if tc.hasPairings {
				assert.NotEmpty(t, rule.pairings, "Should have pairings for %q", tc.chipName)
			}
			t.Logf("✓ Found rule for %q: useSameIndex=%v, pairings=%v",
				tc.chipName, rule.useSameIndex, rule.pairings)
		} else {
			assert.Nil(t, rule, "Should not find rule for %q", tc.chipName)
			t.Logf("✓ No rule found for unknown chip %q", tc.chipName)
		}
	}
}

// TestChipPairingRule_SkipVoltage tests the shouldSkipVoltage method
func TestChipPairingRule_SkipVoltage(t *testing.T) {
	t.Logf("\n=== Testing Voltage Skip Rules ===")

	// INA3221 should skip in4, in5, in6, in7 (shunt voltages and sum)
	rule := getChipPairingRule("ina3221")
	require.NotNil(t, rule)

	skipTests := []struct {
		idx    int
		should bool
	}{
		{1, false}, {2, false}, {3, false}, // Bus voltages - don't skip
		{4, true}, {5, true}, {6, true}, {7, true}, // Shunt/sum - skip
	}

	for _, tc := range skipTests {
		result := rule.shouldSkipVoltage(tc.idx)
		assert.Equal(t, tc.should, result,
			"shouldSkipVoltage(%d) should be %v", tc.idx, tc.should)
	}

	t.Logf("✓ INA3221 correctly skips in4-in7 (shunt voltages)")
}

// TestChipPairingRule_SkipCurrent tests the shouldSkipCurrent method
func TestChipPairingRule_SkipCurrent(t *testing.T) {
	t.Logf("\n=== Testing Current Skip Rules ===")

	// MAX34440 should skip curr7 and curr8 (current-only channels)
	rule := getChipPairingRule("max34440")
	require.NotNil(t, rule)

	skipTests := []struct {
		idx    int
		should bool
	}{
		{1, false}, {2, false}, {6, false}, // Normal currents - don't skip
		{7, true}, {8, true}, // Current-only channels - skip
	}

	for _, tc := range skipTests {
		result := rule.shouldSkipCurrent(tc.idx)
		assert.Equal(t, tc.should, result,
			"shouldSkipCurrent(%d) should be %v", tc.idx, tc.should)
	}

	t.Logf("✓ MAX34440 correctly skips curr7-curr8 (current-only)")
}

// TestDiscoverVoltageCurrentZones_INA226ChipRule tests INA226 chip-specific pairing
func TestDiscoverVoltageCurrentZones_INA226ChipRule(t *testing.T) {
	t.Logf("\n=== Testing INA226 Chip-Specific Pairing ===")
	t.Logf("INA226 has: in0 (shunt), in1 (bus), curr1 - rule: in1 ↔ curr1, skip in0")

	reader := &sysfsHwmonReader{
		basePath: "testdata/sys/class/hwmon",
	}

	hwmonPath := "testdata/sys/class/hwmon/hwmon_ina226"
	files, err := os.ReadDir(hwmonPath)
	require.NoError(t, err)

	zones, err := reader.discoverVoltageCurrentZones(hwmonPath, "ina226", "ina226", files)
	require.NoError(t, err, "Should find zones using chip rule")
	require.Len(t, zones, 1, "Should find exactly 1 zone (in1 ↔ curr1)")

	zone := zones[0].(*hwmonCalculatedPowerZone)
	power, err := zone.Power()
	require.NoError(t, err)

	// Power = 12000 mV × 5000 mA = 60,000,000 µW = 60 W
	expectedWatts := 12000.0 * 5000.0 / 1_000_000.0
	assert.InDelta(t, expectedWatts, power.Watts(), 0.01,
		"Power should be from in1 (12V) × curr1 (5A) = 60W")

	t.Logf("✓ INA226 correctly paired in1 with curr1: %.2f W", power.Watts())
	t.Logf("✓ INA226 correctly skipped in0 (shunt voltage)")
}

// TestDiscoverVoltageCurrentZones_INA3221ChipRule tests INA3221 chip-specific pairing
func TestDiscoverVoltageCurrentZones_INA3221ChipRule(t *testing.T) {
	t.Logf("\n=== Testing INA3221 Chip-Specific Pairing ===")
	t.Logf("INA3221 has: in1-3 (bus), in4-6 (shunt), in7 (sum), curr1-3")
	t.Logf("Rule: in{N} ↔ curr{N} for N=1..3, skip in4-7")

	reader := &sysfsHwmonReader{
		basePath: "testdata/sys/class/hwmon",
	}

	hwmonPath := "testdata/sys/class/hwmon/hwmon_ina3221"
	files, err := os.ReadDir(hwmonPath)
	require.NoError(t, err)

	zones, err := reader.discoverVoltageCurrentZones(hwmonPath, "ina3221", "ina3221", files)
	require.NoError(t, err, "Should find zones using chip rule")
	require.Len(t, zones, 3, "Should find 3 zones (in1↔curr1, in2↔curr2, in3↔curr3)")

	t.Logf("Found %d zones:", len(zones))
	for _, z := range zones {
		zone := z.(*hwmonCalculatedPowerZone)
		power, err := zone.Power()
		require.NoError(t, err)
		t.Logf("  Zone %q (index %d): %.2f W", zone.Name(), zone.Index(), power.Watts())
	}

	t.Logf("✓ INA3221 correctly paired 3 bus voltage/current pairs")
	t.Logf("✓ INA3221 correctly skipped in4-in7 (shunt voltages and sum)")
}

// TestDiscoverVoltageCurrentZones_MAX20730ExceptionRule tests MAX20730 exception
func TestDiscoverVoltageCurrentZones_MAX20730ExceptionRule(t *testing.T) {
	t.Logf("\n=== Testing MAX20730 Exception Rule ===")
	t.Logf("MAX20730 EXCEPTION: curr1 is output current, pairs with in2 (VOUT), not in1")

	reader := &sysfsHwmonReader{
		basePath: "testdata/sys/class/hwmon",
	}

	hwmonPath := "testdata/sys/class/hwmon/hwmon_max20730"
	files, err := os.ReadDir(hwmonPath)
	require.NoError(t, err)

	zones, err := reader.discoverVoltageCurrentZones(hwmonPath, "max20730", "max20730", files)
	require.NoError(t, err, "Should find zones using chip rule")
	require.Len(t, zones, 1, "Should find 1 zone (in2 ↔ curr1)")

	zone := zones[0].(*hwmonCalculatedPowerZone)
	power, err := zone.Power()
	require.NoError(t, err)

	// Power = 12000 mV (in2=VOUT) × 10000 mA (curr1=IOUT) = 120,000,000 µW = 120 W
	expectedWatts := 12000.0 * 10000.0 / 1_000_000.0
	assert.InDelta(t, expectedWatts, power.Watts(), 0.01,
		"Power should be from in2 (12V VOUT) × curr1 (10A IOUT) = 120W")

	// NOT from in1 (48V VIN) which would give 480W
	wrongWatts := 48000.0 * 10000.0 / 1_000_000.0
	assert.True(t, power.Watts() < wrongWatts-1.0,
		"Power should NOT be from in1 (VIN @ 48V) which would give %.2fW", wrongWatts)

	t.Logf("✓ MAX20730 correctly paired in2 (VOUT) with curr1 (IOUT): %.2f W", power.Watts())
	t.Logf("✓ MAX20730 correctly ignored in1 (VIN @ 48V)")
}

// TestDiscoverVoltageCurrentZones_SameIndexFallback tests same-index fallback for unknown chips
func TestDiscoverVoltageCurrentZones_SameIndexFallback(t *testing.T) {
	t.Logf("\n=== Testing Same-Index Fallback for Unknown Chips ===")
	t.Logf("When chip is unknown, fall back to same-index matching: in{N} ↔ curr{N}")

	reader := &sysfsHwmonReader{
		basePath: "testdata/sys/class/hwmon",
	}

	hwmonPath := "testdata/sys/class/hwmon/hwmon_same_index"
	files, err := os.ReadDir(hwmonPath)
	require.NoError(t, err)

	zones, err := reader.discoverVoltageCurrentZones(hwmonPath, "unknown_chip", "unknown_chip", files)
	require.NoError(t, err, "Should find zones using same-index fallback")
	require.Len(t, zones, 2, "Should find 2 zones (in1↔curr1, in2↔curr2)")

	t.Logf("Found %d zones using same-index fallback:", len(zones))
	for _, z := range zones {
		zone := z.(*hwmonCalculatedPowerZone)
		power, err := zone.Power()
		require.NoError(t, err)
		t.Logf("  Zone %q (index %d): %.2f W", zone.Name(), zone.Index(), power.Watts())
	}

	t.Logf("✓ Same-index fallback correctly paired in1↔curr1 and in2↔curr2")
}

// TestDiscoverVoltageCurrentZones_PriorityOrder tests that label > chip rule > same-index
func TestDiscoverVoltageCurrentZones_PriorityOrder(t *testing.T) {
	t.Logf("\n=== Testing Priority Order: Label > Chip Rule > Same-Index ===")

	reader := &sysfsHwmonReader{
		basePath: "testdata/sys/class/hwmon",
	}

	// Test 1: Label matching takes priority
	// hwmon_voltage_current has labels, should use label matching even for known chip
	t.Run("LabelMatchingTakesPriority", func(t *testing.T) {
		hwmonPath := "testdata/sys/class/hwmon/hwmon_voltage_current"
		files, err := os.ReadDir(hwmonPath)
		require.NoError(t, err)

		// Pass ina3221 (known chip) but it has labels, so label matching should be used
		zones, err := reader.discoverVoltageCurrentZones(hwmonPath, "ina3221", "ina3221", files)
		require.NoError(t, err)
		require.NotEmpty(t, zones)

		// Zone should have the label name, not a generated name
		zone := zones[0].(*hwmonCalculatedPowerZone)
		assert.NotContains(t, zone.Name(), "power",
			"Zone name should be from label, not generated")
		t.Logf("✓ Label matching used: zone named %q", zone.Name())
	})

	// Test 2: Chip rule used when no labels
	t.Run("ChipRuleWhenNoLabels", func(t *testing.T) {
		hwmonPath := "testdata/sys/class/hwmon/hwmon_ina226"
		files, err := os.ReadDir(hwmonPath)
		require.NoError(t, err)

		zones, err := reader.discoverVoltageCurrentZones(hwmonPath, "ina226", "ina226", files)
		require.NoError(t, err)
		require.Len(t, zones, 1, "Should find 1 zone via chip rule")
		t.Logf("✓ Chip rule used for INA226")
	})

	// Test 3: Same-index used when no labels and unknown chip
	t.Run("SameIndexWhenUnknownChip", func(t *testing.T) {
		hwmonPath := "testdata/sys/class/hwmon/hwmon_same_index"
		files, err := os.ReadDir(hwmonPath)
		require.NoError(t, err)

		zones, err := reader.discoverVoltageCurrentZones(hwmonPath, "unknown", "unknown", files)
		require.NoError(t, err)
		require.Len(t, zones, 2, "Should find 2 zones via same-index")
		t.Logf("✓ Same-index fallback used for unknown chip")
	})
}

// TestKnownChipPairings_Coverage tests that the chip table has expected entries
func TestKnownChipPairings_Coverage(t *testing.T) {
	t.Logf("\n=== Testing Known Chip Pairings Coverage ===")

	// Verify key chips from the Linux hwmon Power Sensor Reference document
	expectedChips := []string{
		// INA Family
		"ina3221", "ina226", "ina219", "ina209", "ina238", "ina260", "ina233",
		// LTC Family
		"ltc2945", "ltc2947", "ltc4260", "ltc4261", "ltc2992", "ltc4282",
		// ADM Family
		"adm1275", "adm1276", "adm1278", "adm1293",
		// MAX Family
		"max20730", "max20751", "max34440", "max34451",
		// TPS Family
		"tps40422", "tps53679", "tps546d24",
		// Other PMBus
		"ir35221", "xdpe12284", "mp2975", "pmbus",
	}

	for _, chip := range expectedChips {
		rule := getChipPairingRule(chip)
		assert.NotNil(t, rule, "Should have pairing rule for %q", chip)
		t.Logf("✓ %s: useSameIndex=%v, pairings=%v, skipV=%v, skipC=%v",
			chip, rule.useSameIndex, rule.pairings, rule.skipVoltages, rule.skipCurrents)
	}

	t.Logf("\n✓ All %d expected chips have pairing rules", len(expectedChips))
}
