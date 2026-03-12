// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package device

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	validIIOPath = "testdata/sys"
)

// TestIIOReaderInterface ensures sysfsIIOReader implements hwmonReader interface
func TestIIOReaderInterface(t *testing.T) {
	var _ hwmonReader = (*sysfsIIOReader)(nil)
}

// TestIIOPowerZoneInterface ensures iioPowerZone implements EnergyZone interface
func TestIIOPowerZoneInterface(t *testing.T) {
	var _ EnergyZone = (*iioPowerZone)(nil)
}

// TestIIOReaderDiscovery tests that the IIO reader discovers zones from testdata
func TestIIOReaderDiscovery(t *testing.T) {
	reader := &sysfsIIOReader{
		basePath: filepath.Join(validIIOPath, "bus", "iio", "devices"),
	}

	zones, err := reader.Zones()
	require.NoError(t, err, "IIO reader should discover zones from testdata")
	assert.Len(t, zones, 3, "Should find 3 power zones (POM_5V_IN, POM_5V_GPU, POM_5V_CPU)")

	// Verify zone names from rail_name files
	zoneNames := make(map[string]bool)
	for _, z := range zones {
		zoneNames[z.Name()] = true
	}
	assert.True(t, zoneNames["pom_5v_in"], "Should have POM_5V_IN zone")
	assert.True(t, zoneNames["pom_5v_gpu"], "Should have POM_5V_GPU zone")
	assert.True(t, zoneNames["pom_5v_cpu"], "Should have POM_5V_CPU zone")
}

// TestIIOPowerZoneReading tests power reading and mW to µW conversion
func TestIIOPowerZoneReading(t *testing.T) {
	reader := &sysfsIIOReader{
		basePath: filepath.Join(validIIOPath, "bus", "iio", "devices"),
	}

	zones, err := reader.Zones()
	require.NoError(t, err)

	for _, z := range zones {
		power, err := z.Power()
		require.NoError(t, err, "Power() should succeed for zone %s", z.Name())

		switch z.Name() {
		case "pom_5v_in":
			// testdata has 2363 mW → 2363000 µW
			assert.Equal(t, Power(2363000), power, "POM_5V_IN should be 2363 mW = 2363000 µW")
		case "pom_5v_gpu":
			// testdata has 0 mW → 0 µW
			assert.Equal(t, Power(0), power, "POM_5V_GPU should be 0 µW (idle)")
		case "pom_5v_cpu":
			// testdata has 761 mW → 761000 µW
			assert.Equal(t, Power(761000), power, "POM_5V_CPU should be 761 mW = 761000 µW")
		}
	}
}

// TestIIOPowerZoneEnergy verifies that Energy() returns error (not supported)
func TestIIOPowerZoneEnergy(t *testing.T) {
	zone := &iioPowerZone{
		name:  "test",
		index: 0,
		path:  "/nonexistent",
	}

	_, err := zone.Energy()
	assert.Error(t, err, "Energy() should return error for IIO power zones")
	assert.Equal(t, Energy(0), zone.MaxEnergy(), "MaxEnergy() should return 0")
}

// TestIIOReaderNoDevices tests behavior when IIO directory exists but has no devices
func TestIIOReaderNoDevices(t *testing.T) {
	tmpDir := t.TempDir()
	iioDir := filepath.Join(tmpDir, "bus", "iio", "devices")
	require.NoError(t, os.MkdirAll(iioDir, 0o755))

	reader := &sysfsIIOReader{
		basePath: iioDir,
	}

	zones, err := reader.Zones()
	assert.Error(t, err, "Should error when no IIO devices found")
	assert.Nil(t, zones)
}

// TestIIOReaderNonexistentPath tests behavior when IIO path doesn't exist
func TestIIOReaderNonexistentPath(t *testing.T) {
	reader := &sysfsIIOReader{
		basePath: "/nonexistent/bus/iio/devices",
	}

	zones, err := reader.Zones()
	assert.Error(t, err, "Should error when IIO path doesn't exist")
	assert.Nil(t, zones)
}

// TestIIOReaderNoPowerSensors tests device with name but no power files
func TestIIOReaderNoPowerSensors(t *testing.T) {
	tmpDir := t.TempDir()
	deviceDir := filepath.Join(tmpDir, "iio:device0")
	require.NoError(t, os.MkdirAll(deviceDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(deviceDir, "name"), []byte("some_chip"), 0o644))

	reader := &sysfsIIOReader{
		basePath: tmpDir,
	}

	zones, err := reader.Zones()
	assert.Error(t, err, "Should error when device has no power sensors")
	assert.Nil(t, zones)
}

// TestIIOReaderFallbackZoneName tests zone naming when rail_name is missing
func TestIIOReaderFallbackZoneName(t *testing.T) {
	tmpDir := t.TempDir()
	deviceDir := filepath.Join(tmpDir, "iio:device0")
	require.NoError(t, os.MkdirAll(deviceDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(deviceDir, "name"), []byte("test_chip"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(deviceDir, "in_power0_input"), []byte("1000"), 0o644))
	// No rail_name_0 file

	reader := &sysfsIIOReader{
		basePath: tmpDir,
	}

	zones, err := reader.Zones()
	require.NoError(t, err)
	assert.Len(t, zones, 1)
	assert.Equal(t, "test_chip_power0", zones[0].Name(), "Should fallback to chip_name_powerN")
}

// TestCompositeReaderHwmonOnly tests composite reader when only hwmon has zones
func TestCompositeReaderHwmonOnly(t *testing.T) {
	// The existing testdata has hwmon zones but also now has IIO zones
	// Test with a mock to isolate hwmon-only behavior
	mockZones := []EnergyZone{
		&hwmonPowerZone{name: "test_zone", index: 0, path: "/fake"},
	}
	mockReader := &mockHwmonReader{zones: mockZones}
	failReader := &mockHwmonReader{err: assert.AnError}

	composite := &compositeReader{
		readers: []hwmonReader{mockReader, failReader},
	}

	zones, err := composite.Zones()
	require.NoError(t, err)
	assert.Len(t, zones, 1)
	assert.Equal(t, "test_zone", zones[0].Name())
}

// TestCompositeReaderIIOFallback tests composite reader falling back to IIO
func TestCompositeReaderIIOFallback(t *testing.T) {
	iioZones := []EnergyZone{
		&iioPowerZone{name: "pom_5v_in", index: 0, path: "/fake"},
	}
	failReader := &mockHwmonReader{err: assert.AnError}
	iioMock := &mockHwmonReader{zones: iioZones}

	composite := &compositeReader{
		readers: []hwmonReader{failReader, iioMock},
	}

	zones, err := composite.Zones()
	require.NoError(t, err)
	assert.Len(t, zones, 1)
	assert.Equal(t, "pom_5v_in", zones[0].Name())
}

// TestCompositeReaderBothSources tests composite reader combining both sources
func TestCompositeReaderBothSources(t *testing.T) {
	hwmonZones := []EnergyZone{
		&hwmonPowerZone{name: "package", index: 0, path: "/fake/hwmon"},
	}
	iioZones := []EnergyZone{
		&iioPowerZone{name: "pom_5v_in", index: 0, path: "/fake/iio"},
	}

	composite := &compositeReader{
		readers: []hwmonReader{
			&mockHwmonReader{zones: hwmonZones},
			&mockHwmonReader{zones: iioZones},
		},
	}

	zones, err := composite.Zones()
	require.NoError(t, err)
	assert.Len(t, zones, 2, "Should combine zones from both readers")
}

// TestCompositeReaderAllFail tests composite reader when all readers fail
func TestCompositeReaderAllFail(t *testing.T) {
	composite := &compositeReader{
		readers: []hwmonReader{
			&mockHwmonReader{err: assert.AnError},
			&mockHwmonReader{err: assert.AnError},
		},
	}

	zones, err := composite.Zones()
	assert.Error(t, err)
	assert.Nil(t, zones)
}

// TestNewHwmonPowerMeterWithIIOTestdata tests that NewHwmonPowerMeter picks up IIO zones
// from testdata (which has both hwmon and IIO data)
func TestNewHwmonPowerMeterWithIIOTestdata(t *testing.T) {
	meter, err := NewHwmonPowerMeter(validIIOPath)
	require.NoError(t, err)

	zones, err := meter.Zones()
	require.NoError(t, err)

	// Should find zones from both hwmon and IIO testdata
	zoneNames := make(map[string]bool)
	for _, z := range zones {
		zoneNames[z.Name()] = true
	}

	// IIO zones should be present
	assert.True(t, zoneNames["pom_5v_in"], "Should include IIO zone POM_5V_IN")
	assert.True(t, zoneNames["pom_5v_cpu"], "Should include IIO zone POM_5V_CPU")
}

// mockHwmonReader is defined in hwmon_power_meter_test.go — reused here
