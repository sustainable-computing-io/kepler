// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package device

import (
	"fmt"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/utils/ptr"
)

func TestRaplPowerMeter_Init_WithMockReader(t *testing.T) {
	tests := []struct {
		name         string
		mockReader   raplReader
		expectedName string
		expectError  bool
	}{
		{
			name: "successful initialization with mock powercap reader",
			mockReader: &fakePowercapReader{
				available: true,
				zones:     createTestZones("powercap"),
				name:      "powercap",
			},
			expectedName: "powercap",
			expectError:  false,
		},
		{
			name: "successful initialization with mock MSR reader",
			mockReader: &fakeMSRReader{
				available: true,
				zones:     createTestZones("msr"),
				name:      "msr",
			},
			expectedName: "msr",
			expectError:  false,
		},
		{
			name: "initialization fails with reader that has no zones",
			mockReader: &fakePowercapReader{
				available: true,
				zones:     []EnergyZone{},
				name:      "empty",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pm, err := NewCPUPowerMeter(
				"/fake/sysfs",
				WithRaplReader(tt.mockReader),
			)
			require.NoError(t, err)

			err = pm.Init()

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedName, pm.reader.Name())
			}
		})
	}
}

func TestRaplPowerMeter_Name(t *testing.T) {
	tests := []struct {
		name     string
		useMSR   bool
		expected string
	}{
		{
			name:     "powercap reader",
			useMSR:   false,
			expected: "rapl-powercap",
		},
		{
			name:     "msr reader",
			useMSR:   true,
			expected: "rapl-msr",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pm := &raplPowerMeter{
				useMSR: tt.useMSR,
			}
			assert.Equal(t, tt.expected, pm.Name())
		})
	}
}

func TestRaplPowerMeter_Zones_WithFiltering(t *testing.T) {
	// Create test zones
	testZones := []EnergyZone{
		&fakeMSRZone{name: "package", index: 0, path: "/fake/package", energy: Energy(1000)},
		&fakeMSRZone{name: "core", index: 0, path: "/fake/core", energy: Energy(500)},
		&fakeMSRZone{name: "dram", index: 0, path: "/fake/dram", energy: Energy(300)},
	}

	tests := []struct {
		name       string
		zoneFilter []string
		expected   []string
	}{
		{
			name:       "no filter - all zones",
			zoneFilter: []string{},
			expected:   []string{"package", "core", "dram"},
		},
		{
			name:       "filter package only",
			zoneFilter: []string{"package"},
			expected:   []string{"package"},
		},
		{
			name:       "filter core and dram",
			zoneFilter: []string{"core", "dram"},
			expected:   []string{"core", "dram"},
		},
		{
			name:       "filter non-existent zone",
			zoneFilter: []string{"nonexistent"},
			expected:   []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockReader := &fakeMSRReader{
				available: true,
				zones:     testZones,
			}

			pm := &raplPowerMeter{
				reader:     mockReader,
				zoneFilter: tt.zoneFilter,
				logger:     slog.Default(),
			}

			zones, err := pm.Zones()
			if len(tt.expected) == 0 {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "no RAPL zones found after filtering")
			} else {
				assert.NoError(t, err)
				assert.Equal(t, len(tt.expected), len(zones))

				zoneNames := make([]string, len(zones))
				for i, zone := range zones {
					zoneNames[i] = zone.Name()
				}

				for _, expected := range tt.expected {
					assert.Contains(t, zoneNames, expected)
				}
			}
		})
	}
}

func TestRaplPowerMeter_PrimaryEnergyZone(t *testing.T) {
	tests := []struct {
		name           string
		availableZones []string
		expectedZone   string
	}{
		{
			name:           "psys has highest priority",
			availableZones: []string{"core", "package", "psys", "dram"},
			expectedZone:   "psys",
		},
		{
			name:           "package has second priority",
			availableZones: []string{"core", "package", "dram"},
			expectedZone:   "package",
		},
		{
			name:           "core has third priority",
			availableZones: []string{"core", "dram"},
			expectedZone:   "core",
		},
		{
			name:           "fallback to first zone if no priority match",
			availableZones: []string{"uncore", "other"},
			expectedZone:   "uncore",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var testZones []EnergyZone
			for i, name := range tt.availableZones {
				testZones = append(testZones, &fakeMSRZone{
					name:  name,
					index: i,
					path:  fmt.Sprintf("/fake/%s", name),
				})
			}

			mockReader := &fakeMSRReader{
				available: true,
				zones:     testZones,
			}

			pm := &raplPowerMeter{
				reader: mockReader,
				logger: slog.Default(),
			}

			primaryZone, err := pm.PrimaryEnergyZone()
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedZone, primaryZone.Name())

			// Test caching - call again and should get same result
			primaryZone2, err := pm.PrimaryEnergyZone()
			assert.NoError(t, err)
			assert.Equal(t, primaryZone, primaryZone2)
		})
	}
}

func TestRaplPowerMeter_Close(t *testing.T) {
	mockReader := &fakeMSRReader{
		available: true,
		zones:     createTestZones("test"),
	}

	pm := &raplPowerMeter{
		reader: mockReader,
		logger: slog.Default(),
	}

	err := pm.Close()
	assert.NoError(t, err)

	// Test closing when reader is nil
	pm.reader = nil
	err = pm.Close()
	assert.NoError(t, err)
}

func TestNewCPUPowerMeter(t *testing.T) {
	sysfsPath := "/fake/sysfs"

	pm, err := NewCPUPowerMeter(sysfsPath)
	require.NoError(t, err)

	assert.Equal(t, sysfsPath, pm.sysfsPath)
	assert.NotNil(t, pm.logger)
	assert.Equal(t, []string{}, pm.zoneFilter)

	// Test MSR config defaults
	assert.Equal(t, ptr.To(false), pm.msrConfig.Enabled)
	assert.Equal(t, ptr.To(false), pm.msrConfig.Force)
	assert.Equal(t, "/dev/cpu/%d/msr", pm.msrConfig.DevicePath)
}

func TestNewCPUPowerMeter_WithOptions(t *testing.T) {
	sysfsPath := "/fake/sysfs"

	testLogger := slog.Default().With("test", "meter")
	testZoneFilter := []string{"package", "core"}
	testMSRConfig := MSRConfig{
		Enabled:    ptr.To(true),
		Force:      ptr.To(false),
		DevicePath: "/custom/cpu/%d/msr",
	}

	pm, err := NewCPUPowerMeter(
		sysfsPath,
		WithRaplLogger(testLogger),
		WithZoneFilter(testZoneFilter),
		WithMSRConfig(testMSRConfig),
	)
	require.NoError(t, err)

	assert.Equal(t, sysfsPath, pm.sysfsPath)
	assert.Equal(t, testZoneFilter, pm.zoneFilter)
	assert.Equal(t, testMSRConfig, pm.msrConfig)
}

// Helper types and functions

type fakePowercapReader struct {
	zones     []EnergyZone
	available bool
	initError error
	name      string
}

func (f *fakePowercapReader) Zones() ([]EnergyZone, error) {
	return f.zones, nil
}

func (f *fakePowercapReader) Available() bool {
	return f.available
}

func (f *fakePowercapReader) Init() error {
	return f.initError
}

func (f *fakePowercapReader) Close() error {
	return nil
}

func (f *fakePowercapReader) Name() string {
	if f.name == "" {
		return "fake-powercap"
	}
	return f.name
}

func createTestZones(prefix string) []EnergyZone {
	return []EnergyZone{
		&fakeMSRZone{name: "package", index: 0, path: fmt.Sprintf("/%s/package", prefix), energy: Energy(1000)},
		&fakeMSRZone{name: "core", index: 0, path: fmt.Sprintf("/%s/core", prefix), energy: Energy(500)},
		&fakeMSRZone{name: "dram", index: 0, path: fmt.Sprintf("/%s/dram", prefix), energy: Energy(300)},
	}
}
