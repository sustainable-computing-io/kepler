// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package device

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
)

// msrReader implements raplReader using Intel MSR (Model Specific Register) interface
type msrReader struct {
	msrFiles   map[int]*os.File // CPU ID -> MSR file handle
	zones      []EnergyZone     // Available energy zones
	energyUnit float64          // Energy unit in microjoules per LSB
	devicePath string           // MSR device path template
	logger     *slog.Logger
	mu         sync.RWMutex // Thread safety for zone operations
}

// MSR zone configuration mapping zone names to MSR offsets
var msrZoneConfig = map[string]uint32{
	ZonePackage: MSRPkgEnergyStatus,
	ZonePP0:     MSRPP0EnergyStatus, // Maps to "core" zone
	ZoneDRAM:    MSRDRAMEnergyStatus,
}

// zoneNameMapping maps MSR zone names to standard RAPL zone names
var zoneNameMapping = map[string]string{
	ZonePP0: ZoneCore, // PP0 (Power Plane 0) is the core domain
}

// NewMSRReader creates a new MSR reader using the specified device path template
func NewMSRReader(devicePath string, logger *slog.Logger) *msrReader {
	if logger == nil {
		logger = slog.Default()
	}

	return &msrReader{
		msrFiles:   make(map[int]*os.File),
		devicePath: devicePath,
		logger:     logger.With("service", "msr-reader"),
	}
}

// Name returns the name of this power reader implementation
func (m *msrReader) Name() string {
	return "msr"
}

// Available checks if MSR interface is available on this system
func (m *msrReader) Available() bool {
	// Derive CPU directory from devicePath (e.g., "/dev/cpu/%d/msr" -> "/dev/cpu")
	cpuDir := filepath.Dir(filepath.Dir(m.devicePath))

	// Check if CPU directory exists
	if _, err := os.Stat(cpuDir); os.IsNotExist(err) {
		m.logger.Debug("MSR not available: CPU directory does not exist", "dir", cpuDir)
		return false
	}

	// Check if we can find at least one CPU with MSR access
	// This validates that MSR interface is not just present but usable
	cpuIDs, err := m.findAvailableCPUs()
	if err != nil {
		m.logger.Debug("MSR not available: failed to scan for CPUs", "error", err)
		return false
	}

	if len(cpuIDs) == 0 {
		m.logger.Debug("MSR not available: no CPUs with MSR access found")
		return false
	}

	return true
}

// Init initializes the MSR reader and opens MSR files for all available CPUs
func (m *msrReader) Init() error {
	if !m.Available() {
		return fmt.Errorf("MSR interface not available")
	}

	// Find available CPUs
	cpuIDs, err := m.findAvailableCPUs()
	if err != nil {
		return fmt.Errorf("failed to find available CPUs: %w", err)
	}

	if len(cpuIDs) == 0 {
		return fmt.Errorf("no CPUs with MSR access found")
	}

	// Open MSR files for all CPUs
	for _, cpuID := range cpuIDs {
		msrPath := fmt.Sprintf(m.devicePath, cpuID)
		file, err := os.OpenFile(msrPath, os.O_RDONLY, 0)
		if err != nil {
			// Clean up any previously opened files
			if closeErr := m.Close(); closeErr != nil {
				m.logger.Warn("Failed to close MSR files", "error", closeErr)
			}
			return fmt.Errorf("failed to open MSR file %s: %w", msrPath, err)
		}
		m.msrFiles[cpuID] = file
	}

	// Read energy unit from the first CPU
	firstCPU := cpuIDs[0]
	energyUnit, err := readEnergyUnit(m.msrFiles[firstCPU])
	if err != nil {
		if closeErr := m.Close(); closeErr != nil {
			m.logger.Warn("Failed to close MSR files", "error", closeErr)
		}
		return fmt.Errorf("failed to read energy unit from CPU %d: %w", firstCPU, err)
	}
	m.energyUnit = energyUnit

	// Create zones for all available MSR energy counters
	if err := m.createZones(); err != nil {
		if closeErr := m.Close(); closeErr != nil {
			m.logger.Warn("Failed to close MSR files", "error", closeErr)
		}
		return fmt.Errorf("failed to create MSR zones: %w", err)
	}

	m.logger.Info("MSR reader initialized",
		"cpus", len(m.msrFiles),
		"zones", len(m.zones),
		"energy_unit_uj", m.energyUnit)

	return nil
}

// Zones returns the list of MSR-based energy zones
func (m *msrReader) Zones() ([]EnergyZone, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.zones) == 0 {
		return nil, fmt.Errorf("MSR reader not initialized or no zones available")
	}

	// Return a copy to prevent external modification
	zones := make([]EnergyZone, len(m.zones))
	copy(zones, m.zones)
	return zones, nil
}

// Close closes all MSR files and releases resources
func (m *msrReader) Close() error {
	var lastErr error

	for cpuID, file := range m.msrFiles {
		if err := file.Close(); err != nil {
			lastErr = err
			m.logger.Warn("Failed to close MSR file", "cpu", cpuID, "error", err)
		}
	}

	// Clear the map
	m.msrFiles = make(map[int]*os.File)
	m.zones = nil

	return lastErr
}

// findAvailableCPUs finds all CPUs that have MSR device files
func (m *msrReader) findAvailableCPUs() ([]int, error) {
	// Derive CPU directory from devicePath (e.g., "/dev/cpu/%d/msr" -> "/dev/cpu")
	cpuDir := filepath.Dir(filepath.Dir(m.devicePath))
	entries, err := os.ReadDir(cpuDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read CPU directory %s: %w", cpuDir, err)
	}

	var cpuIDs []int
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Parse CPU ID from directory name
		cpuID, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue // Skip non-numeric directories
		}

		// Check if MSR file exists for this CPU
		msrPath := fmt.Sprintf(m.devicePath, cpuID)
		if _, err := os.Stat(msrPath); err == nil {
			cpuIDs = append(cpuIDs, cpuID)
		}
	}

	// Sort CPU IDs for consistent ordering
	sort.Ints(cpuIDs)

	return cpuIDs, nil
}

// createZones creates MSR-based energy zones for all available MSR counters
func (m *msrReader) createZones() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.zones = nil

	// Get sorted CPU IDs for consistent zone ordering
	var cpuIDs []int
	for cpuID := range m.msrFiles {
		cpuIDs = append(cpuIDs, cpuID)
	}
	sort.Ints(cpuIDs)

	// Group zones by name for potential aggregation
	zoneGroups := make(map[string][]*msrZone)

	// Create zones for each MSR counter on each CPU
	for _, cpuID := range cpuIDs {
		msrFile := m.msrFiles[cpuID]

		for zoneName, msrOffset := range msrZoneConfig {
			// Test if this MSR register is readable on this CPU
			if !m.isRegisterReadable(msrFile, msrOffset) {
				m.logger.Debug("MSR register not readable, skipping zone",
					"cpu", cpuID, "zone", zoneName, "msr", fmt.Sprintf("0x%x", msrOffset))
				continue
			}

			// Map internal zone names to standard RAPL names if needed
			displayName := zoneName
			if mappedName, exists := zoneNameMapping[zoneName]; exists {
				displayName = mappedName
			}

			// Create MSR zone
			zone := NewMSRZone(displayName, cpuID, cpuID, msrOffset, m.energyUnit, msrFile)
			zoneGroups[displayName] = append(zoneGroups[displayName], zone)

			m.logger.Debug("Created MSR zone",
				"name", displayName, "cpu", cpuID, "msr", fmt.Sprintf("0x%x", msrOffset))
		}
	}

	// Convert zone groups to EnergyZone interfaces
	// For multi-socket systems, aggregate zones with the same name
	for name, zones := range zoneGroups {
		if len(zones) == 1 {
			// Single zone - use directly
			m.zones = append(m.zones, zones[0])
		} else {
			// Multiple zones - create aggregated zone
			var energyZones []EnergyZone
			for _, zone := range zones {
				energyZones = append(energyZones, zone)
			}
			aggregated := NewAggregatedZone(energyZones)
			m.zones = append(m.zones, aggregated)

			m.logger.Debug("Created aggregated MSR zone",
				"name", name, "zone_count", len(zones))
		}
	}

	if len(m.zones) == 0 {
		return fmt.Errorf("no readable MSR energy counters found")
	}

	return nil
}

// isRegisterReadable tests if an MSR register can be read without error
func (m *msrReader) isRegisterReadable(msrFile *os.File, msrOffset uint32) bool {
	// Try to seek to the register
	_, err := msrFile.Seek(int64(msrOffset), 0)
	if err != nil {
		return false
	}

	// Try to read 8 bytes from the register
	buf := make([]byte, 8)
	_, err = msrFile.Read(buf)
	return err == nil
}
