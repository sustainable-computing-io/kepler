// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package device

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPowercapReader_Available(t *testing.T) {
	tests := []struct {
		name              string
		sysfsPath         string
		expectedAvailable bool
	}{
		{
			name:              "powercap available with valid sysfs",
			sysfsPath:         validSysFSPath,
			expectedAvailable: true,
		},
		{
			name:              "powercap unavailable with bad sysfs",
			sysfsPath:         badSysFSPath,
			expectedAvailable: false,
		},
		{
			name:              "powercap unavailable with nonexistent path",
			sysfsPath:         "/nonexistent",
			expectedAvailable: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader, err := NewPowercapReader(tt.sysfsPath)
			if err != nil {
				// If reader creation fails, powercap is not available
				assert.False(t, tt.expectedAvailable)
				return
			}

			available := reader.Available()
			assert.Equal(t, tt.expectedAvailable, available)
		})
	}
}

func TestPowercapReader_Init(t *testing.T) {
	tests := []struct {
		name        string
		sysfsPath   string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "successful initialization",
			sysfsPath:   validSysFSPath,
			expectError: false,
		},
		{
			name:        "initialization fails without powercap",
			sysfsPath:   badSysFSPath,
			expectError: true,
			errorMsg:    "powercap interface not available",
		},
		{
			name:        "initialization fails with invalid path",
			sysfsPath:   "/nonexistent",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader, err := NewPowercapReader(tt.sysfsPath)
			if err != nil {
				// Reader creation failed
				if tt.expectError {
					return
				}
				t.Fatalf("Unexpected error creating reader: %v", err)
			}

			err = reader.Init()
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestPowercapReader_Zones(t *testing.T) {
	reader, err := NewPowercapReader(validSysFSPath)
	require.NoError(t, err)
	require.NoError(t, reader.Init())

	zones, err := reader.Zones()
	require.NoError(t, err)

	assert.Greater(t, len(zones), 0)

	// Test that we have at least one zone and can read its properties
	zone := zones[0]

	// Zone should have a name
	assert.NotEmpty(t, zone.Name())
	// Zone should have a valid index (>= 0)
	assert.GreaterOrEqual(t, zone.Index(), 0)
	// Zone should have a path
	assert.NotEmpty(t, zone.Path())

	// Test energy reading
	energy, err := zone.Energy()
	assert.NoError(t, err)
	assert.Greater(t, uint64(energy), uint64(0)) // Should have some energy value
}

func TestPowercapReader_Name(t *testing.T) {
	reader, err := NewPowercapReader("/tmp")
	require.NoError(t, err)
	assert.Equal(t, "powercap", reader.Name())
}

func TestPowercapReader_Close(t *testing.T) {
	reader, err := NewPowercapReader("/tmp")
	require.NoError(t, err)

	err = reader.Close()
	assert.NoError(t, err)
}

func TestSysfsRaplZone_Implementation(t *testing.T) {
	reader, err := NewPowercapReader(validSysFSPath)
	require.NoError(t, err)

	zones, err := reader.Zones()
	require.NoError(t, err)
	require.Greater(t, len(zones), 0)

	// Test the first zone's EnergyZone interface methods
	zone := zones[0]

	// Test all EnergyZone interface methods
	assert.NotEmpty(t, zone.Name())           // Should have a name
	assert.GreaterOrEqual(t, zone.Index(), 0) // Should have a valid index
	assert.NotEmpty(t, zone.Path())           // Should have a path

	energy, err := zone.Energy()
	assert.NoError(t, err)
	assert.Greater(t, uint64(energy), uint64(0)) // Should have some energy value

	maxEnergy := zone.MaxEnergy()
	assert.Greater(t, uint64(maxEnergy), uint64(0)) // Should have some max energy value
}
