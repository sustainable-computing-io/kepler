// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package device

import (
	"fmt"

	"github.com/prometheus/procfs/sysfs"
)

// powercapReader implements raplReader using the Linux powercap sysfs interface
type powercapReader struct {
	fs sysfs.FS
}

// NewPowercapReader creates a new powercap reader using the specified sysfs path
func NewPowercapReader(sysfsPath string) (*powercapReader, error) {
	fs, err := sysfs.NewFS(sysfsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create sysfs filesystem: %w", err)
	}

	return &powercapReader{
		fs: fs,
	}, nil
}

// Name returns the name of this power reader implementation
func (p *powercapReader) Name() string {
	return "powercap"
}

// Available checks if powercap interface is available on this system
func (p *powercapReader) Available() bool {
	// Try to read RAPL zones to verify functionality
	_, err := sysfs.GetRaplZones(p.fs)
	return err == nil
}

// Init initializes the powercap reader and verifies it can read energy values
func (p *powercapReader) Init() error {
	if !p.Available() {
		return fmt.Errorf("powercap interface not available")
	}

	// Try reading zones and test the first zone
	zones, err := p.Zones()
	if err != nil {
		return fmt.Errorf("failed to read RAPL zones: %w", err)
	}

	if len(zones) == 0 {
		return fmt.Errorf("no RAPL zones found")
	}

	// Try reading energy from the first zone to verify functionality
	_, err = zones[0].Energy()
	if err != nil {
		return fmt.Errorf("failed to read energy from zone %s: %w", zones[0].Name(), err)
	}

	return nil
}

// Zones returns the list of RAPL energy zones available from powercap
func (p *powercapReader) Zones() ([]EnergyZone, error) {
	raplZones, err := sysfs.GetRaplZones(p.fs)
	if err != nil {
		return nil, fmt.Errorf("failed to read rapl zones: %w", err)
	}

	// Convert sysfs.RaplZones to EnergyZones
	energyZones := make([]EnergyZone, 0, len(raplZones))
	for _, zone := range raplZones {
		energyZones = append(energyZones, sysfsRaplZone{zone})
	}

	return energyZones, nil
}

// Close releases any resources held by the powercap reader
func (p *powercapReader) Close() error {
	// No resources to close for powercap reader
	return nil
}

// sysfsRaplZone implements EnergyZone using sysfs.RaplZone.
// It is an adapter for the EnergyZone interface
type sysfsRaplZone struct {
	zone sysfs.RaplZone
}

// Name returns the name of the zone
func (s sysfsRaplZone) Name() string {
	return s.zone.Name
}

// Index returns the index of the zone
func (s sysfsRaplZone) Index() int {
	return s.zone.Index
}

// Path returns the path of the zone
func (s sysfsRaplZone) Path() string {
	return s.zone.Path
}

// Energy returns the current energy value
func (s sysfsRaplZone) Energy() (Energy, error) {
	mj, err := s.zone.GetEnergyMicrojoules()
	return Energy(mj), err
}

// MaxEnergy returns the maximum energy value before wraparound
func (s sysfsRaplZone) MaxEnergy() Energy {
	return Energy(s.zone.MaxMicrojoules)
}
