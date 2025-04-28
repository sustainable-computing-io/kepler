// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package device

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRaplZoneFiltering(t *testing.T) {
	// Create mock zones for testing
	packageZone := &MockRaplZone{
		name:  "package",
		path:  "/sys/class/powercap/intel-rapl/intel-rapl:0",
		index: 0,
	}
	coreZone := &MockRaplZone{
		name:  "core",
		path:  "/sys/class/powercap/intel-rapl/intel-rapl:0:0",
		index: 1,
	}
	dramZone := &MockRaplZone{
		name:  "dram",
		path:  "/sys/class/powercap/intel-rapl/intel-rapl:0:2",
		index: 2,
	}
	uncoreZone := &MockRaplZone{
		name:  "uncore",
		path:  "/sys/class/powercap/intel-rapl/intel-rapl:0:3",
		index: 3,
	}

	allZones := []EnergyZone{packageZone, coreZone, dramZone, uncoreZone}

	tests := []struct {
		name          string
		filterZones   []string
		expectedZones []string
	}{
		{
			name:          "No filter - all zones included",
			filterZones:   []string{},
			expectedZones: []string{"package", "core", "dram", "uncore"},
		},
		{
			name:          "Filter single zone",
			filterZones:   []string{"core"},
			expectedZones: []string{"core"},
		},
		{
			name:          "Filter multiple zones",
			filterZones:   []string{"package", "dram"},
			expectedZones: []string{"package", "dram"},
		},
		{
			name:          "Case-insensitive filtering",
			filterZones:   []string{"PACKAGE", "Core"},
			expectedZones: []string{"package", "core"},
		},
		{
			name:          "Non-existent zone in filter",
			filterZones:   []string{"package", "nonexistent"},
			expectedZones: []string{"package"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockReader := &mockRaplReader{}
			mockReader.On("Zones").Return(allZones, nil)

			logger := slog.Default().With("test", "zone-filtering")
			meter := &raplPowerMeter{
				reader:     mockReader,
				logger:     logger,
				zoneFilter: tc.filterZones,
			}

			// Filter zones directly to test the filtering logic
			filteredZones := meter.filterZones(allZones)

			// Verify only expected zones are included
			assert.Equal(t, len(tc.expectedZones), len(filteredZones),
				"Filtered zones length mismatch")

			// Create a map of zone names for easy checking
			zoneNames := make(map[string]bool)
			for _, zone := range filteredZones {
				zoneNames[zone.Name()] = true
			}

			// Verify each expected zone is present
			for _, name := range tc.expectedZones {
				assert.True(t, zoneNames[name],
					"Expected zone %s not found in filtered zones", name)
			}
		})
	}
}

// Test that zone filtering applies during Init
func TestRaplZoneFiltering_Init(t *testing.T) {
	packageZone := &MockRaplZone{
		name:           "package",
		path:           "/sys/class/powercap/intel-rapl/intel-rapl:0",
		index:          0,
		maxMicroJoules: 1000000,
		energy:         100000,
	}
	coreZone := &MockRaplZone{
		name:           "core",
		path:           "/sys/class/powercap/intel-rapl/intel-rapl:0:0",
		index:          1,
		maxMicroJoules: 1000000,
		energy:         50000,
	}

	allZones := []EnergyZone{packageZone, coreZone}

	t.Run("Init succeeds with valid filter", func(t *testing.T) {
		mockReader := &mockRaplReader{}
		mockReader.On("Zones").Return(allZones, nil)

		meter := &raplPowerMeter{
			reader:     mockReader,
			logger:     slog.Default(),
			zoneFilter: []string{"package"},
		}

		err := meter.Init()
		assert.NoError(t, err)
	})

	t.Run("Init does not fails with unknown zones", func(t *testing.T) {
		mockReader := &mockRaplReader{}
		mockReader.On("Zones").Return(allZones, nil)

		meter := &raplPowerMeter{
			reader:     mockReader,
			logger:     slog.Default(),
			zoneFilter: []string{"nonexistent"},
		}

		err := meter.Init()
		assert.NoError(t, err)
	})
}

// Test that Zones() properly applies the filter
func TestRaplZoneFiltering_Zones(t *testing.T) {
	packageZone := &MockRaplZone{
		name:           "package",
		path:           "/sys/class/powercap/intel-rapl/intel-rapl:0",
		index:          0,
		maxMicroJoules: 1000000,
		energy:         100000,
	}
	coreZone := &MockRaplZone{
		name:           "core",
		path:           "/sys/class/powercap/intel-rapl/intel-rapl:0:0",
		index:          1,
		maxMicroJoules: 1000000,
		energy:         50000,
	}

	allZones := []EnergyZone{packageZone, coreZone}

	tests := []struct {
		name          string
		filter        []string
		expectedZones int
		expectError   bool
	}{
		{
			name:          "No filter",
			filter:        []string{},
			expectedZones: 2,
			expectError:   false,
		}, {
			name:          "Filter package",
			filter:        []string{"package"},
			expectedZones: 1,
			expectError:   false,
		}, {
			name:          "Filter core",
			filter:        []string{"core"},
			expectedZones: 1,
			expectError:   false,
		}, {
			name:          "nonexistent zone",
			filter:        []string{"nonexistent"},
			expectedZones: 0,
			expectError:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockReader := &mockRaplReader{}
			mockReader.On("Zones").Return(allZones, nil)

			meter := &raplPowerMeter{
				reader:     mockReader,
				logger:     slog.Default(),
				zoneFilter: tc.filter,
			}

			zones, err := meter.Zones()

			if tc.expectError {
				assert.Error(t, err)
				assert.Nil(t, zones)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedZones, len(zones))
			}
		})
	}
}

// Test integration with the configuration options
func TestRaplZoneFiltering_WithOptions(t *testing.T) {
	// Mock sysfs reader for this test
	mockReader := &mockRaplReader{}
	packageZone := &MockRaplZone{
		name:           "package",
		path:           "/sys/class/powercap/intel-rapl/intel-rapl:0",
		index:          0,
		maxMicroJoules: 1000000,
		energy:         100000,
	}
	coreZone := &MockRaplZone{
		name:           "core",
		path:           "/sys/class/powercap/intel-rapl/intel-rapl:0:0",
		index:          1,
		maxMicroJoules: 1000000,
		energy:         50000,
	}
	mockReader.On("Zones").Return([]EnergyZone{packageZone, coreZone}, nil)

	// Create meter with WithZoneFilter option
	meter, err := NewCPUPowerMeter(
		validSysFSPath,
		WithSysFSReader(mockReader),
		WithZoneFilter([]string{"core"}),
	)
	assert.NoError(t, err)

	// Check that filtering was applied
	zones, err := meter.Zones()
	assert.NoError(t, err)
	assert.Equal(t, 1, len(zones))
	assert.Equal(t, "core", zones[0].Name())
}
