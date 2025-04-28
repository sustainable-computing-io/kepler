// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package device

import (
	"fmt"
	"strings"

	"github.com/prometheus/procfs/sysfs"
)

// raplPowerMeter implements CPUPowerMeter using sysfs
type raplPowerMeter struct {
	reader      sysfsReader
	cachedZones []EnergyZone
}

type OptionFn func(*raplPowerMeter)

// sysfsReader is an interface for a sysfs filesystem used by raplPowerMeter to mock for testing
type sysfsReader interface {
	Zones() ([]EnergyZone, error)
}

// WithSysFSReader sets the sysfsReader used by raplPowerMeter
func WithSysFSReader(r sysfsReader) OptionFn {
	return func(pm *raplPowerMeter) {
		pm.reader = r
	}
}

// NewCPUPowerMeter creates a new CPU power meter
func NewCPUPowerMeter(sysfsPath string, opts ...OptionFn) (*raplPowerMeter, error) {
	fs, err := sysfs.NewFS(sysfsPath)
	if err != nil {
		return nil, err
	}

	ret := &raplPowerMeter{
		reader: sysfsRaplReader{fs: fs},
	}

	for _, opt := range opts {
		opt(ret)
	}

	return ret, nil
}

func (r *raplPowerMeter) Name() string {
	return "rapl"
}

func (r *raplPowerMeter) Init() error {
	// ensure zones can be read but don't cache them
	zones, err := r.reader.Zones()
	if err != nil {
		return err
	} else if len(zones) == 0 {
		return fmt.Errorf("no RAPL zones found")
	}

	// try reading the first zone and return the error
	_, err = zones[0].Energy()
	return err
}

func (r *raplPowerMeter) Zones() ([]EnergyZone, error) {
	// Return cached zones if already initialized
	if len(r.cachedZones) != 0 {
		return r.cachedZones, nil
	}

	zones, err := r.reader.Zones()
	if err != nil {
		return nil, err
	} else if len(zones) == 0 {
		return nil, fmt.Errorf("no RAPL zones found")
	}

	stdZoneMap := map[string]EnergyZone{}
	for _, zone := range zones {
		// key -> zone-name + index
		key := fmt.Sprintf("%s-%d", zone.Name(), zone.Index())

		// ignore non-standard zones if a standard zone already exists
		if existingZone, exists := stdZoneMap[key]; exists && isStandardRaplPath(existingZone.Path()) {
			continue
		}
		stdZoneMap[key] = zone
	}

	r.cachedZones = make([]EnergyZone, 0, len(stdZoneMap))
	for _, zone := range stdZoneMap {
		r.cachedZones = append(r.cachedZones, zone)
	}
	return r.cachedZones, nil
}

// isStandardRaplPath checks if a RAPL zone path is in the standard format
func isStandardRaplPath(path string) bool {
	return strings.Contains(path, "/intel-rapl:")
}

type sysfsRaplReader struct {
	fs sysfs.FS
}

func (r sysfsRaplReader) Zones() ([]EnergyZone, error) {
	raplZones, err := sysfs.GetRaplZones(r.fs)
	if err != nil {
		return nil, fmt.Errorf("failed to read rapl zones: %w", err)
	}

	// convert sysfs.RaplZones to EnergyZones
	energyZones := make([]EnergyZone, 0, len(raplZones))
	for _, zone := range raplZones {
		energyZones = append(energyZones, sysfsRaplZone{zone})
	}

	return energyZones, nil
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
