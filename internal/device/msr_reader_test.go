// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package device

/*
MSR Test Data Documentation

This test file uses mock MSR data to simulate Intel RAPL MSR registers for testing
the MSR reader implementation. The test data simulates the following registers:

MSR Register Values:
- 0x606: IA32_RAPL_POWER_UNIT - Power unit register containing scaling factors
- 0x611: IA32_PKG_ENERGY_STATUS - Package energy counter (32-bit, wraps around)
- 0x639: IA32_PP0_ENERGY_STATUS - Power Plane 0 (cores) energy counter
- 0x619: IA32_DRAM_ENERGY_STATUS - DRAM energy counter

File Format:
Each MSR register value is stored as 8 bytes (uint64) in little-endian format.
The test creates temporary MSR files and writes mock data at specific byte offsets
corresponding to the MSR register addresses.

Energy Unit Calculation:
The power unit register (0x606) contains scaling factors in specific bit fields:
- Bits 12:8 contain the energy unit value (e.g., value 16 means 1/(2^16) joules per LSB)
- Energy counters use this unit to convert raw MSR values to microjoules
- Example: energy_unit = 15.2587890625 microjoules (when unit value = 16)

Counter Overflow:
MSR energy counters are 32-bit values that wrap around at 2^32. The implementation
must handle this overflow correctly to maintain accurate energy measurements.
*/

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeMSRReader implements raplReader for testing
type fakeMSRReader struct {
	zones     []EnergyZone
	available bool
	initError error
	name      string
}

func (f *fakeMSRReader) Zones() ([]EnergyZone, error) {
	return f.zones, nil
}

func (f *fakeMSRReader) Available() bool {
	return f.available
}

func (f *fakeMSRReader) Init() error {
	return f.initError
}

func (f *fakeMSRReader) Close() error {
	return nil
}

func (f *fakeMSRReader) Name() string {
	if f.name == "" {
		return "fake-msr"
	}
	return f.name
}

// fakeMSRZone implements EnergyZone for testing
type fakeMSRZone struct {
	name      string
	index     int
	path      string
	energy    Energy
	maxEnergy Energy
	energyErr error
}

func (f *fakeMSRZone) Name() string {
	return f.name
}

func (f *fakeMSRZone) Index() int {
	return f.index
}

func (f *fakeMSRZone) Path() string {
	return f.path
}

func (f *fakeMSRZone) Energy() (Energy, error) {
	return f.energy, f.energyErr
}

func (f *fakeMSRZone) MaxEnergy() Energy {
	return f.maxEnergy
}

func TestMSRReader_Available(t *testing.T) {
	tests := []struct {
		name           string
		setupDevDir    bool
		createMSRFile  bool
		expectedResult bool
	}{
		{
			name:           "MSR available with dev directory and msr file",
			setupDevDir:    true,
			createMSRFile:  true,
			expectedResult: true,
		},
		{
			name:           "MSR unavailable without dev directory",
			setupDevDir:    false,
			createMSRFile:  false,
			expectedResult: false,
		},
		{
			name:           "MSR unavailable without msr file",
			setupDevDir:    true,
			createMSRFile:  false,
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary directory structure
			tempDir := t.TempDir()
			var devicePath string

			if tt.setupDevDir {
				// Create /dev/cpu/0 directory
				cpuDir := filepath.Join(tempDir, "dev", "cpu", "0")
				require.NoError(t, os.MkdirAll(cpuDir, 0755))

				devicePath = filepath.Join(tempDir, "dev", "cpu", "%d", "msr")

				if tt.createMSRFile {
					msrFile := filepath.Join(cpuDir, "msr")
					file, err := os.Create(msrFile)
					require.NoError(t, err)
					_ = file.Close()
				}
			} else {
				devicePath = filepath.Join(tempDir, "nonexistent", "cpu", "%d", "msr")
			}

			reader := NewMSRReader(devicePath, slog.Default())
			result := reader.Available()

			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestMSRReader_Init(t *testing.T) {
	tests := []struct {
		name        string
		setupMSRs   func(tempDir string) string
		expectError bool
		errorMsg    string
	}{
		{
			name: "successful initialization",
			setupMSRs: func(tempDir string) string {
				// Create CPU 0 with MSR file containing mock data
				cpuDir := filepath.Join(tempDir, "dev", "cpu", "0")
				require.NoError(t, os.MkdirAll(cpuDir, 0755))

				msrFile := filepath.Join(cpuDir, "msr")
				createMockMSRFile(t, msrFile)

				return filepath.Join(tempDir, "dev", "cpu", "%d", "msr")
			},
			expectError: false,
		},
		{
			name: "initialization fails with no CPUs",
			setupMSRs: func(tempDir string) string {
				// Create empty dev directory
				require.NoError(t, os.MkdirAll(filepath.Join(tempDir, "dev", "cpu"), 0755))
				return filepath.Join(tempDir, "dev", "cpu", "%d", "msr")
			},
			expectError: true,
			errorMsg:    "MSR interface not available",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			devicePath := tt.setupMSRs(tempDir)

			reader := NewMSRReader(devicePath, slog.Default())
			err := reader.Init()

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}

			// Clean up
			_ = reader.Close()
		})
	}
}

func TestMSRReader_Zones(t *testing.T) {
	tempDir := t.TempDir()

	// Create CPU 0 and CPU 1 with MSR files
	for i := 0; i < 2; i++ {
		cpuDir := filepath.Join(tempDir, "dev", "cpu", fmt.Sprintf("%d", i))
		require.NoError(t, os.MkdirAll(cpuDir, 0755))

		msrFile := filepath.Join(cpuDir, "msr")
		createMockMSRFile(t, msrFile)
	}

	devicePath := filepath.Join(tempDir, "dev", "cpu", "%d", "msr")
	reader := NewMSRReader(devicePath, slog.Default())

	require.NoError(t, reader.Init())
	t.Cleanup(func() {
		assert.NoError(t, reader.Close())
	})

	zones, err := reader.Zones()
	require.NoError(t, err)

	// Should have zones for package, core (pp0), and dram
	// On a 2-CPU system, we should get aggregated zones
	assert.Greater(t, len(zones), 0)

	// Verify zone names
	zoneNames := make(map[string]bool)
	for _, zone := range zones {
		zoneNames[zone.Name()] = true

		// Test that each zone can provide energy readings
		energy, err := zone.Energy()
		assert.NoError(t, err)
		assert.GreaterOrEqual(t, energy, Energy(0))
	}

	// Should have at least package zone
	assert.True(t, zoneNames["package"] || zoneNames["core"] || zoneNames["dram"],
		"Expected at least one MSR zone type")
}

func TestMSRReader_Name(t *testing.T) {
	reader := NewMSRReader("/dev/cpu/%d/msr", slog.Default())
	assert.Equal(t, "msr", reader.Name())
}

func TestMSRReader_Close(t *testing.T) {
	tempDir := t.TempDir()

	// Create CPU 0 with MSR file
	cpuDir := filepath.Join(tempDir, "dev", "cpu", "0")
	require.NoError(t, os.MkdirAll(cpuDir, 0755))

	msrFile := filepath.Join(cpuDir, "msr")
	createMockMSRFile(t, msrFile)

	devicePath := filepath.Join(tempDir, "dev", "cpu", "%d", "msr")
	reader := NewMSRReader(devicePath, slog.Default())

	require.NoError(t, reader.Init())

	// Verify it has zones before closing
	zones, err := reader.Zones()
	require.NoError(t, err)
	assert.Greater(t, len(zones), 0)

	// Close should not error
	err = reader.Close()
	assert.NoError(t, err)

	// After closing, zones should be cleared
	_, err = reader.Zones()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "MSR reader not initialized")
}

func TestMSRZone_Energy(t *testing.T) {
	tests := []struct {
		name          string
		msrData       []byte
		energyUnit    float64
		expectedRange [2]Energy // min, max range
	}{
		{
			name: "normal energy reading",
			msrData: []byte{
				0x00, 0x00, 0x10, 0x00, // 0x100000 in lower 32 bits
				0x00, 0x00, 0x00, 0x00, // upper 32 bits
			},
			energyUnit:    15.2587890625,                                 // 1000000 / 2^16
			expectedRange: [2]Energy{Energy(15999998), Energy(16000000)}, // Approximately 16.0 J
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary MSR file with specific data
			tempDir := t.TempDir()
			msrFile := filepath.Join(tempDir, "msr")

			file, err := os.Create(msrFile)
			require.NoError(t, err)
			t.Cleanup(func() {
				assert.NoError(t, file.Close())
			})

			// Write mock MSR data at different offsets
			_, err = file.WriteAt(tt.msrData, int64(MSRPkgEnergyStatus))
			require.NoError(t, err)

			// Create MSR zone
			zone := NewMSRZone("package", 0, 0, MSRPkgEnergyStatus, tt.energyUnit, file)

			energy, err := zone.Energy()
			require.NoError(t, err)

			// Check energy is within expected range
			assert.GreaterOrEqual(t, energy, tt.expectedRange[0])
			assert.LessOrEqual(t, energy, tt.expectedRange[1])
		})
	}
}

func TestMSRZone_MaxEnergy(t *testing.T) {
	energyUnit := 15.2587890625 // 1000000 / 2^16

	zone := NewMSRZone("package", 0, 0, MSRPkgEnergyStatus, energyUnit, nil)
	maxEnergy := zone.MaxEnergy()

	// For 32-bit counter, max should be 2^32 * energyUnit
	expectedMax := Energy(float64(0xFFFFFFFF) * energyUnit)
	assert.Equal(t, expectedMax, maxEnergy)
}

// Helper functions

// createMockMSRFile creates a mock MSR device file with test data
// The file simulates reading from /dev/cpu/N/msr with realistic RAPL register values
func createMockMSRFile(t *testing.T, path string) {
	file, err := os.Create(path)
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, file.Close())
	}()

	// Write power unit register at offset 0x606 (IA32_RAPL_POWER_UNIT)
	// This register contains scaling factors for energy measurements
	// Bits 12:8 = energy unit: 16 means 1/(2^16) = 15.2587890625 microjoules per LSB
	powerUnitData := []byte{
		0x00, 0x10, 0x00, 0x00, // Energy unit = 16 in bits 12:8
		0x00, 0x00, 0x00, 0x00, // Upper 32 bits (unused)
	}
	_, err = file.WriteAt(powerUnitData, int64(MSRPowerUnit))
	require.NoError(t, err)

	// Write package energy counter at offset 0x611 (IA32_PKG_ENERGY_STATUS)
	// This is a 32-bit counter that accumulates package energy consumption
	// Raw value: 0x100000 = 1048576 LSB → ~16.0 Joules with energy unit 15.26 μJ/LSB
	pkgEnergyData := []byte{
		0x00, 0x00, 0x10, 0x00, // 32-bit energy counter value
		0x00, 0x00, 0x00, 0x00, // Upper 32 bits (reserved/unused)
	}
	_, err = file.WriteAt(pkgEnergyData, int64(MSRPkgEnergyStatus))
	require.NoError(t, err)

	// Write PP0 energy counter at offset 0x639 (IA32_PP0_ENERGY_STATUS)
	// PP0 represents Power Plane 0 (CPU cores) energy consumption
	// Raw value: 0x80000 = 524288 LSB → ~8.0 Joules
	pp0EnergyData := []byte{
		0x00, 0x00, 0x08, 0x00, // 32-bit energy counter value
		0x00, 0x00, 0x00, 0x00, // Upper 32 bits (reserved/unused)
	}
	_, err = file.WriteAt(pp0EnergyData, int64(MSRPP0EnergyStatus))
	require.NoError(t, err)

	// Write DRAM energy counter at offset 0x619 (IA32_DRAM_ENERGY_STATUS)
	// This counter tracks memory subsystem energy consumption
	// Raw value: 0x40000 = 262144 LSB → ~4.0 Joules
	dramEnergyData := []byte{
		0x00, 0x00, 0x04, 0x00, // 32-bit energy counter value
		0x00, 0x00, 0x00, 0x00, // Upper 32 bits (reserved/unused)
	}
	_, err = file.WriteAt(dramEnergyData, int64(MSRDRAMEnergyStatus))
	require.NoError(t, err)
}
