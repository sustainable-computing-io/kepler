// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package device

import (
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/prometheus/procfs/sysfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TestCPUPowerMeterInterface ensures that raplPowerMeter properly implements the CPUPowerMeter interface
func TestCPUPowerMeterInterface(t *testing.T) {
	var _ CPUPowerMeter = (*raplPowerMeter)(nil)
}

func TestNewCPUPowerMeter(t *testing.T) {
	meter, err := NewCPUPowerMeter("testdata/sys")
	assert.NotNil(t, meter, "NewCPUPowerMeter should not return nil")
	assert.NoError(t, err, "NewCPUPowerMeter should not return error")
	assert.IsType(t, &raplPowerMeter{}, meter, "NewCPUPowerMeter should return a *cpuPowerMeter")
}

func TestCPUPowerMeter_Name(t *testing.T) {
	meter := &raplPowerMeter{}
	name := meter.Name()
	assert.Equal(t, "rapl", name, "Name() should return 'rapl'")
}

func TestCPUPowerMeter_Init(t *testing.T) {
	meter, err := NewCPUPowerMeter(validSysFSPath)
	assert.NoError(t, err, "NewCPUPowerMeter should not return an error")

	err = meter.Init()
	assert.NoError(t, err, "Start() should not return an error")
}

func TestCPUPowerMeter_Zones(t *testing.T) {
	meter := &raplPowerMeter{
		reader: sysfsRaplReader{fs: validSysFSFixtures(t)},
		logger: slog.Default().With("service", "rapl"),
	}
	zones, err := meter.Zones()
	assert.NoError(t, err, "Zones() should not return an error")
	assert.NotNil(t, zones, "Zones() should return a non-nil slice")

	names := make([]string, len(zones))
	for i, zone := range zones {
		names[i] = zone.Name()
	}
	assert.Contains(t, names, "package")
	assert.Contains(t, names, "core")
}

// TestSysFSRaplZoneInterface ensures that sysfsRaplZone properly implements the EnergyZone interface
func TestSysFSRaplZoneInterface(t *testing.T) {
	pkg := sysfs.RaplZone{
		Name:           "package",
		Index:          0,
		Path:           "/sys/class/powercap/intel-rapl/intel-rapl:0",
		MaxMicrojoules: 1_000_000,
	}

	zone := sysfsRaplZone{zone: pkg}

	// Test that all interface methods return the expected values
	assert.Equal(t, 0, zone.Index())
	assert.Equal(t, "/sys/class/powercap/intel-rapl/intel-rapl:0", zone.Path())
	assert.Equal(t, "package", zone.Name())
	assert.Equal(t, 1.0, zone.MaxEnergy().Joules())
}

func TestSysFSRaplPowerMeterInit(t *testing.T) {
	rapl := raplPowerMeter{
		reader: sysfsRaplReader{fs: validSysFSFixtures(t)},
		logger: slog.Default().With("service", "rapl"),
	}
	err := rapl.Init()
	assert.NoError(t, err)
}

func TestSysFSRaplPowerMeterInitFail(t *testing.T) {
	rapl := raplPowerMeter{reader: sysfsRaplReader{fs: invalidSysFSFixtures(t)}}
	err := rapl.Init()
	assert.Error(t, err)
}

// TestSysFSRaplPowerMeter tests the sysfsRaplZone implementation using test fixtures
func TestSysFSRaplPowerMeter(t *testing.T) {
	fs := validSysFSFixtures(t)
	actualZones, err := sysfs.GetRaplZones(fs)
	assert.NoError(t, err)
	assert.Equal(t, 4, len(actualZones), "Expected to find 4 zones in test fixtures")

	// realRaplReader should filter out non-standard zones
	rapl := raplPowerMeter{
		reader: sysfsRaplReader{fs: fs},
		logger: slog.Default().With("service", "rapl"),
	}
	zones, err := rapl.Zones()

	// Test that each zone implements the interface correctly
	assert.NoError(t, err)
	// With aggregation: two package zones become one AggregatedZone + one core zone = 2 total
	assert.Equal(t, 2, len(zones), "find 2 zones after aggregation (package + core)")
	assert.Equal(t, []string{"core", "package"}, sortedZoneNames(zones),
		"Expected to find aggregated zones in test fixtures")

	for _, zone := range zones {
		assert.NotEmpty(t, zone.Name(), "Zone name should not be empty")
		assert.NotEmpty(t, zone.Path(), "Zone path should not be empty")
		assert.GreaterOrEqual(t, zone.MaxEnergy(), 1000.0*Joule, "Max energy should not be negative")

		// Zone could be either sysfsRaplZone or AggregatedZone
		switch z := zone.(type) {
		case sysfsRaplZone:
			// Individual zone
			assert.NotNil(t, z)
		case *AggregatedZone:
			// Aggregated zone
			assert.NotNil(t, z)
			assert.Equal(t, -1, z.Index(), "AggregatedZone should have index -1")
		default:
			t.Fatalf("Unexpected zone type: %T", zone)
		}

		// Skip the original assertion since we now support both zone types
		_ = zone

		energy, err := zone.Energy()
		assert.NoError(t, err, zone.Path())
		assert.GreaterOrEqual(t, energy, 1000.0*Joule, "Energy should not be negative")
	}
}

func TestAggregatedZoneIntegration(t *testing.T) {
	// Test that RAPL reader creates AggregatedZone for multiple zones with same name
	mockReader := &mockSysFSReader{
		response: []EnergyZone{
			// Two package zones with same name but different indices and one core zone
			mockZone{name: "package", index: 0, path: "/intel-rapl:0", energy: 1000, maxEnergy: 100000},
			mockZone{name: "package", index: 1, path: "/intel-rapl:1", energy: 2000, maxEnergy: 100000},
			mockZone{name: "core", index: 0, path: "/intel-rapl:0:0", energy: 500, maxEnergy: 50000},
		},
	}

	rapl := &raplPowerMeter{
		reader: mockReader,
		logger: slog.Default(),
	}

	zones, err := rapl.Zones()
	require.NoError(t, err)

	// Should have 2 zones: 1 aggregated package zone + 1 core zone
	assert.Equal(t, 2, len(zones), "Expected 2 zones after aggregation")

	// Find the package zone - should be AggregatedZone
	var packageZone EnergyZone
	var coreZone EnergyZone
	for _, zone := range zones {
		if zone.Name() == "package" {
			packageZone = zone
		} else if zone.Name() == "core" { // Single zone keeps original name
			coreZone = zone
		}
	}

	// Verify package zone is aggregated
	require.NotNil(t, packageZone, "Package zone should exist")
	aggregated, isAggregated := packageZone.(*AggregatedZone)
	assert.True(t, isAggregated, "Package zone should be AggregatedZone")
	assert.Equal(t, "package", aggregated.Name())
	assert.Equal(t, -1, aggregated.Index())
	assert.Equal(t, Energy(200000), aggregated.MaxEnergy()) // Sum of both package zones

	// Verify core zone is not aggregated
	require.NotNil(t, coreZone, "Core zone should exist")
	_, isNotAggregated := coreZone.(mockZone)
	assert.True(t, isNotAggregated, "Core zone should remain as individual zone")

	// Test energy aggregation
	packageEnergy, err := packageZone.Energy()
	require.NoError(t, err)
	assert.Equal(t, Energy(3000), packageEnergy) // 1000 + 2000 from both package zones
}

type mockZone struct {
	name      string
	index     int
	path      string
	energy    Energy
	maxEnergy Energy
}

func (m mockZone) Name() string            { return m.name }
func (m mockZone) Index() int              { return m.index }
func (m mockZone) Path() string            { return m.path }
func (m mockZone) Energy() (Energy, error) { return m.energy, nil }
func (m mockZone) MaxEnergy() Energy       { return m.maxEnergy }

type mockSysFSReader struct {
	response []EnergyZone
	err      error
}

func (m *mockSysFSReader) Zones() ([]EnergyZone, error) {
	return m.response, m.err
}

// TestRAPLPowerMeterFromFixtures tests the realRaplReader with filtering using test fixtures
func TestRAPLPowerMeterFromFixtures(t *testing.T) {
	fs := validSysFSFixtures(t)

	raplMeter := raplPowerMeter{
		reader: sysfsRaplReader{fs: fs},
		logger: slog.Default().With("service", "rapl"),
	}
	allZones, err := raplMeter.Zones()
	assert.NoError(t, err)
	assert.NotEmpty(t, allZones, "Expected to find RAPL zones in test fixtures")

	mmioZones := 0
	for _, zone := range allZones {
		if strings.Contains(zone.Path(), "mmio") {
			mmioZones++
		}
	}
	assert.Equal(t, mmioZones, 0, "all non-standard RAPL zones should be filtered")
}

// TestStandardRaplPath tests that standard paths are preferred over non-standard ones
func TestStandardRaplPaths(t *testing.T) {
	tt := []struct {
		path       string
		isStandard bool
	}{
		{"/sys/class/powercap/intel-rapl", false},
		{"/sys/class/powercap/intel-rapl-mmio", false},
		{"/sys/class/powercap/intel-rapl-mmio/intel-rapl-mmio:0", false},
		{"/sys/class/powercap/intel-rapl-mmio:0", false},
		{"/sys/class/powercap/intel-rapl/intel-rapl:0", true},
		{"/sys/class/powercap/intel-rapl:0", true},
		{"/sys/class/powercap/intel-rapl:0:0", true},
		{"/sys/class/powercap/intel-rapl:0:1", true},
		{"/sys/class/powercap/intel-rapl:1", true},
	}

	for _, test := range tt {
		assert.Equal(t, test.isStandard, isStandardRaplPath(test.path), test.path)
	}
}

type mockRaplReader struct {
	mock.Mock
}

func (m *mockRaplReader) Zones() ([]EnergyZone, error) {
	args := m.Called()
	return args.Get(0).([]EnergyZone), args.Error(1)
}

// TestStandardPathPreference tests that standard paths are preferred over non-standard ones
func TestStandardPathPreference(t *testing.T) {
	// Create test zones with both standard and non-standard paths
	mmio := &MockRaplZone{
		name:  "package",
		path:  "/sys/class/powercap/intel-rapl-mmio/intel-rapl-mmio:0",
		index: 0,
	}
	stdPkg := &MockRaplZone{
		name:  "package",
		path:  "/sys/class/powercap/intel-rapl/intel-rapl:0",
		index: 0,
	}
	tt := []struct {
		zones    []EnergyZone
		expected EnergyZone
	}{
		{[]EnergyZone{stdPkg}, stdPkg},
		{[]EnergyZone{mmio}, mmio},
		{[]EnergyZone{mmio, stdPkg}, stdPkg},
		{[]EnergyZone{stdPkg, mmio}, stdPkg},
	}

	for _, test := range tt {
		mockReader := &mockRaplReader{}
		mockReader.On("Zones").Return(test.zones, nil)

		rapl, err := NewCPUPowerMeter(validSysFSPath, WithSysFSReader(mockReader))
		assert.NoError(t, err)

		zones, err := rapl.Zones()
		assert.NoError(t, err)

		// We should have only one package zone
		assert.Equal(t, 1, len(zones), "Should have 1 zone after filtering mmio")

		// The package zone should be the standard path version
		pkg := zones[0]
		expected := test.expected

		// It should be the standard path version
		assert.Equal(t, "package", expected.Name())
		assert.Equal(t, pkg.Path(), expected.Path(),
			"Should prefer standard path over non-standard path")

		mockReader.AssertExpectations(t)
	}
}

// TestZoneCaching tests that zones are cached and called only once
func TestZoneCaching(t *testing.T) {
	// Create test zones with both standard and non-standard paths
	pkg := &MockRaplZone{
		name:  "package",
		path:  "/sys/class/powercap/intel-rapl/intel-rapl:0",
		index: 0,
	}
	core := &MockRaplZone{
		name:  "core",
		path:  "/sys/class/powercap/intel-rapl/intel-rapl:0:0",
		index: 1,
	}
	raplZones := []EnergyZone{pkg, core}

	mockReader := &mockRaplReader{}
	mockReader.On("Zones").Return(raplZones, nil).Once()

	rapl, err := NewCPUPowerMeter(validSysFSPath, WithSysFSReader(mockReader))
	assert.NoError(t, err)

	// Get zones multiple times to test that "Zone" is called only once
	for range 3 {
		zones, err := rapl.Zones()
		assert.NoError(t, err)
		assert.Equal(t, 2, len(zones), "Should have both zones")
	}

	mockReader.AssertExpectations(t)
}

// TestZoneCaching_Error tests that zones are not cached when there is an error
func TestZoneCaching_Error(t *testing.T) {
	mockReader := &mockRaplReader{}
	rapl, err := NewCPUPowerMeter(validSysFSPath, WithSysFSReader(mockReader))

	t.Run("Zone Read Error", func(t *testing.T) {
		mockReader.On("Zones").Return([]EnergyZone(nil), errors.New("error")).Once()
		assert.NoError(t, err)
		zones, err := rapl.Zones()
		assert.Error(t, err)
		assert.Nil(t, zones)
		mockReader.AssertExpectations(t)
	})

	// Create test zones with both standard and non-standard paths
	pkg := &MockRaplZone{
		name:  "package",
		path:  "/sys/class/powercap/intel-rapl/intel-rapl:0",
		index: 0,
	}
	core := &MockRaplZone{
		name:  "core",
		path:  "/sys/class/powercap/intel-rapl/intel-rapl:0:0",
		index: 1,
	}
	raplZones := []EnergyZone{pkg, core}
	t.Run("Zone Read Succeeds", func(t *testing.T) {
		mockReader.On("Zones").Return(raplZones, nil).Once()
		for range 3 {
			zones, err := rapl.Zones()
			assert.NoError(t, err)
			assert.Equal(t, 2, len(zones))

		}
		mockReader.AssertExpectations(t)
	})
}

// TestZone_None tests that zones error when none are found
func TestZone_None(t *testing.T) {
	mockReader := &mockRaplReader{}
	rapl, err := NewCPUPowerMeter(validSysFSPath, WithSysFSReader(mockReader))
	assert.NoError(t, err)

	mockReader.On("Zones").Return([]EnergyZone(nil), nil).Once()
	zones, err := rapl.Zones()
	assert.Error(t, err)
	assert.Equal(t, 0, len(zones))
	mockReader.AssertExpectations(t)
}

// TestNewCPUPowerMeter_InvalidPath tests that NewCPUPowerMeter returns an error with an invalid sysfs path
func TestNewCPUPowerMeter_InvalidPath(t *testing.T) {
	meter, err := NewCPUPowerMeter("/nonexistent/path")
	assert.Error(t, err, "Should return an error with an invalid path")
	assert.Nil(t, meter, "Should not return a meter with an invalid path")
}

// TestCPUPowerMeter_ZonesError tests that the Zones method correctly handles errors from the reader
func TestCPUPowerMeter_ZonesError(t *testing.T) {
	mockReader := &mockRaplReader{}
	expectedErr := errors.New("error")
	mockReader.On("Zones").Return([]EnergyZone{}, expectedErr)

	meter := &raplPowerMeter{reader: mockReader}
	zones, err := meter.Zones()

	assert.Error(t, err, "Should return an error when the reader fails")
	assert.Equal(t, expectedErr, err, "Should return the error from the reader")
	assert.Nil(t, zones, "Should return nil zones when there's an error")
	mockReader.AssertExpectations(t)
}

// TestCPUPowerMeter_NoZones tests that Zones returns an error when no zones are found
func TestCPUPowerMeter_NoZones(t *testing.T) {
	mockReader := &mockRaplReader{}
	mockReader.On("Zones").Return([]EnergyZone{}, nil)

	meter := &raplPowerMeter{reader: mockReader}
	zones, err := meter.Zones()

	assert.Error(t, err, "Should return an error when no zones are found")
	assert.Equal(t, "no RAPL zones found", err.Error(), "Should return a specific error message")
	assert.Nil(t, zones, "Should return nil zones when no zones are found")
	mockReader.AssertExpectations(t)
}

// TestCPUPowerMeter_InitNoZones tests that Start returns an error when no zones are found
func TestCPUPowerMeter_InitNoZones(t *testing.T) {
	mockReader := &mockRaplReader{}
	mockReader.On("Zones").Return([]EnergyZone{}, nil)

	meter := &raplPowerMeter{reader: mockReader}
	err := meter.Init()

	assert.Error(t, err, "Start() should return an error when no zones are found")
	assert.Equal(t, "no RAPL zones found", err.Error(), "Start() should return a specific error message")
	mockReader.AssertExpectations(t)
}

// TestPrimaryEnergyZone tests the PrimaryEnergyZone method
func TestPrimaryEnergyZone(t *testing.T) {
	t.Run("Priority hierarchy", func(t *testing.T) {
		tests := []struct {
			name     string
			zones    []EnergyZone
			expected string
		}{{
			name: "psys has highest priority",
			zones: []EnergyZone{
				mockZone{name: "package", index: 0},
				mockZone{name: "psys", index: 0},
				mockZone{name: "core", index: 0},
			},
			expected: "psys",
		}, {
			name: "package has priority over core",
			zones: []EnergyZone{
				mockZone{name: "core", index: 0},
				mockZone{name: "package", index: 0},
				mockZone{name: "dram", index: 0},
			},
			expected: "package",
		}, {
			name: "core has priority over dram",
			zones: []EnergyZone{
				mockZone{name: "dram", index: 0},
				mockZone{name: "core", index: 0},
				mockZone{name: "uncore", index: 0},
			},
			expected: "core",
		}, {
			name: "dram has priority over uncore",
			zones: []EnergyZone{
				mockZone{name: "uncore", index: 0},
				mockZone{name: "dram", index: 0},
			},
			expected: "dram",
		}}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				mockReader := &mockRaplReader{}
				mockReader.On("Zones").Return(tt.zones, nil)

				meter := &raplPowerMeter{reader: mockReader, logger: slog.Default()}
				zone, err := meter.PrimaryEnergyZone()

				assert.NoError(t, err)
				assert.Equal(t, tt.expected, zone.Name())
				mockReader.AssertExpectations(t)
			})
		}
	})

	t.Run("Case insensitive matching", func(t *testing.T) {
		mockReader := &mockRaplReader{}
		mockReader.On("Zones").Return([]EnergyZone{
			mockZone{name: "PACKAGE", index: 0},
			mockZone{name: "Core", index: 0},
		}, nil)

		meter := &raplPowerMeter{reader: mockReader, logger: slog.Default()}
		zone, err := meter.PrimaryEnergyZone()

		assert.NoError(t, err)
		assert.Equal(t, "PACKAGE", zone.Name())
		mockReader.AssertExpectations(t)
	})

	t.Run("Fallback to first zone", func(t *testing.T) {
		zones := []EnergyZone{
			mockZone{name: "unknown1", index: 0},
			mockZone{name: "unknown2", index: 1},
		}
		mockReader := &mockRaplReader{}
		mockReader.On("Zones").Return(zones, nil)

		meter := &raplPowerMeter{reader: mockReader, logger: slog.Default()}
		zone, err := meter.PrimaryEnergyZone()

		assert.NoError(t, err)
		// NOTE: since reader.Zones() does not guarantee the order after filtering,
		// we cannot assert zone.Name() == "unknown1", thus assert the zone returned
		// any of the zones passed as input
		zoneName := zone.Name()
		assert.Contains(t, []string{"unknown1", "unknown2"}, zoneName)
		mockReader.AssertExpectations(t)
	})

	t.Run("Caching behavior", func(t *testing.T) {
		mockReader := &mockRaplReader{}
		mockReader.On("Zones").Return([]EnergyZone{
			mockZone{name: "package", index: 0},
		}, nil).Once()

		meter := &raplPowerMeter{reader: mockReader, logger: slog.Default()}

		// First call should read from zones and cache topZone
		zone1, err := meter.PrimaryEnergyZone()
		assert.NoError(t, err)
		assert.Equal(t, "package", zone1.Name())

		// Second call should use cached topZone directly
		zone2, err := meter.PrimaryEnergyZone()
		assert.NoError(t, err)
		assert.Equal(t, "package", zone2.Name())

		mockReader.AssertExpectations(t)
	})

	t.Run("Error handling", func(t *testing.T) {
		t.Run("Zones() returns error", func(t *testing.T) {
			mockReader := &mockRaplReader{}
			mockReader.On("Zones").Return([]EnergyZone{}, errors.New("zones error"))

			meter := &raplPowerMeter{reader: mockReader, logger: slog.Default()}
			zone, err := meter.PrimaryEnergyZone()

			assert.Error(t, err)
			assert.Nil(t, zone)
			assert.Contains(t, err.Error(), "zones error")
			mockReader.AssertExpectations(t)
		})

		t.Run("Empty zones list", func(t *testing.T) {
			mockReader := &mockRaplReader{}
			mockReader.On("Zones").Return([]EnergyZone{}, nil)

			meter := &raplPowerMeter{reader: mockReader, logger: slog.Default()}
			zone, err := meter.PrimaryEnergyZone()

			assert.Error(t, err)
			assert.Nil(t, zone)
			assert.Contains(t, err.Error(), "no RAPL zones found")
			mockReader.AssertExpectations(t)
		})
	})
}
