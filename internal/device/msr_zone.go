// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package device

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
)

// MSR Register offsets for Intel RAPL energy counters
const (
	// IA32_RAPL_POWER_UNIT - Power unit register containing scaling factors
	MSRPowerUnit = 0x606

	// Energy counters (32-bit, wraparound at ~4 billion)
	MSRPkgEnergyStatus  = 0x611 // Package energy counter
	MSRPP0EnergyStatus  = 0x639 // Power Plane 0 (cores) energy counter
	MSRDRAMEnergyStatus = 0x619 // DRAM energy counter
)

// msrZone implements EnergyZone interface for MSR-based energy reading
type msrZone struct {
	name       string
	index      int
	cpuID      int
	msrOffset  uint32
	energyUnit float64 // Energy unit in microjoules per LSB
	msrFile    *os.File
}

// NewMSRZone creates a new MSR-based energy zone
func NewMSRZone(name string, index, cpuID int, msrOffset uint32, energyUnit float64, msrFile *os.File) *msrZone {
	return &msrZone{
		name:       name,
		index:      index,
		cpuID:      cpuID,
		msrOffset:  msrOffset,
		energyUnit: energyUnit,
		msrFile:    msrFile,
	}
}

// Name returns the zone name (package, pp0, dram)
func (m *msrZone) Name() string {
	return m.name
}

// Index returns the zone index (CPU socket/package index)
func (m *msrZone) Index() int {
	return m.index
}

// Path returns the MSR device path for this zone
func (m *msrZone) Path() string {
	return fmt.Sprintf("/dev/cpu/%d/msr:0x%x", m.cpuID, m.msrOffset)
}

// Energy reads the current energy value from the MSR register
func (m *msrZone) Energy() (Energy, error) {
	if m.msrFile == nil {
		return 0, fmt.Errorf("MSR file not opened for CPU %d", m.cpuID)
	}

	// Read 64-bit MSR register at the specified offset
	_, err := m.msrFile.Seek(int64(m.msrOffset), 0)
	if err != nil {
		return 0, fmt.Errorf("failed to seek to MSR offset 0x%x: %w", m.msrOffset, err)
	}

	var msrValue uint64
	err = binary.Read(m.msrFile, binary.LittleEndian, &msrValue)
	if err != nil {
		return 0, fmt.Errorf("failed to read MSR 0x%x from CPU %d: %w", m.msrOffset, m.cpuID, err)
	}

	// Extract the 32-bit energy counter from the MSR value
	// Energy counters are in the lower 32 bits
	energyCounter := uint32(msrValue & 0xFFFFFFFF)

	// Convert to microjoules using the energy unit
	energyMicroJoules := float64(energyCounter) * m.energyUnit

	return Energy(energyMicroJoules), nil
}

// MaxEnergy returns the maximum energy value before wraparound
// MSR energy counters are 32-bit, so they wrap at 2^32
func (m *msrZone) MaxEnergy() Energy {
	// 32-bit counter maximum value converted to microjoules
	maxCounter := uint64(math.MaxUint32)
	maxEnergyMicroJoules := float64(maxCounter) * m.energyUnit
	return Energy(maxEnergyMicroJoules)
}

// readEnergyUnit reads the energy unit from the IA32_RAPL_POWER_UNIT MSR
// Returns the energy unit in microjoules per LSB
func readEnergyUnit(msrFile *os.File) (float64, error) {
	if msrFile == nil {
		return 0, fmt.Errorf("MSR file not opened")
	}

	// Seek to the power unit MSR
	_, err := msrFile.Seek(int64(MSRPowerUnit), 0)
	if err != nil {
		return 0, fmt.Errorf("failed to seek to MSR power unit register: %w", err)
	}

	var powerUnit uint64
	err = binary.Read(msrFile, binary.LittleEndian, &powerUnit)
	if err != nil {
		return 0, fmt.Errorf("failed to read MSR power unit register: %w", err)
	}

	// Energy unit is in bits 12:8 of the power unit register
	energyUnitBits := (powerUnit >> 8) & 0x1F

	// Energy unit = 1 / (2^energyUnitBits) joules
	// Convert to microjoules: multiply by 1,000,000
	energyUnit := 1000000.0 / float64(uint64(1)<<energyUnitBits)

	return energyUnit, nil
}
