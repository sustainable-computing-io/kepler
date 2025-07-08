// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package device

import (
	"fmt"
	"log/slog"
	"math/rand"
	"path/filepath"
	"strings"
	"sync"
)

// NOTE: This fake meter is not intended to be used in production and is for testing only
var defaultFakeZones = []Zone{ZonePackage, ZoneCore, ZoneDRAM}

const defaultRaplPath = "/sys/class/powercap/intel-rapl"

// fakeEnergyZone implements the EnergyZone interface
type fakeEnergyZone struct {
	name      string
	index     int
	path      string
	energy    Energy
	maxEnergy Energy
	mu        sync.Mutex

	// For generating fake values
	increment    Energy
	randomFactor float64
}

var _ EnergyZone = (*fakeEnergyZone)(nil)

// Name returns the zone name
func (z *fakeEnergyZone) Name() string {
	return z.name
}

// Index returns the index of the zone
func (z *fakeEnergyZone) Index() int {
	return z.index
}

// Path returns the path from which the energy usage value ie being read
func (z *fakeEnergyZone) Path() string {
	return z.path
}

// Energy returns energy consumed by the zone.
func (z *fakeEnergyZone) Energy() (Energy, error) {
	z.mu.Lock()
	defer z.mu.Unlock()

	randomComponent := Energy(rand.Float64() * float64(z.increment) * z.randomFactor)
	z.energy = (z.energy + z.increment + randomComponent) % z.maxEnergy

	return z.energy, nil
}

// MaxEnergy returns the maximum value of energy usage that can be read.
func (z *fakeEnergyZone) MaxEnergy() Energy {
	return z.maxEnergy
}

// fakeRaplMeter implements the CPUPowerMeter interface
type fakeRaplMeter struct {
	logger     *slog.Logger
	zones      []EnergyZone
	devicePath string
}

var _ CPUPowerMeter = (*fakeRaplMeter)(nil)

// FakeOptFn is a functional option for configuring FakeRaplMeter
type FakeOptFn func(*fakeRaplMeter)

// WithFakePath sets the base device path for the fake meter
func WithFakePath(path string) FakeOptFn {
	return func(m *fakeRaplMeter) {
		m.devicePath = path
		for _, z := range m.zones {
			if fz, ok := z.(*fakeEnergyZone); ok {
				fz.path = filepath.Join(path, fmt.Sprintf("energy_%s", fz.name))
			}
		}
	}
}

// WithFakeMaxEnergy sets the maximum energy value before wrap-around
func WithFakeMaxEnergy(e Energy) FakeOptFn {
	return func(m *fakeRaplMeter) {
		for _, z := range m.zones {
			if fz, ok := z.(*fakeEnergyZone); ok {
				fz.maxEnergy = e
			}
		}
	}
}

// WithFakeMaxEnergy sets the maximum energy value before wrap-around
func WithFakeLogger(l *slog.Logger) FakeOptFn {
	return func(m *fakeRaplMeter) {
		m.logger = l.With("meter", m.Name())
	}
}

// NewFakeCPUMeter creates a new fake CPU power meter
func NewFakeCPUMeter(zones []string, opts ...FakeOptFn) (CPUPowerMeter, error) {
	meter := &fakeRaplMeter{
		devicePath: defaultRaplPath,
		logger:     slog.Default().With("meter", "fake-cpu-meter"),
	}

	// nil and empty slices are equivalent
	if len(zones) == 0 {
		zones = defaultFakeZones
	}

	zoneIncrementFactor := map[Zone]int{
		ZonePackage: 12,
		ZoneCore:    8,
		ZoneDRAM:    5,
		ZoneUncore:  2,
	}

	meter.zones = make([]EnergyZone, 0, len(zones))

	for i, zoneName := range zones {
		meter.zones = append(meter.zones, &fakeEnergyZone{
			name:         zoneName,
			index:        i,
			path:         filepath.Join(defaultRaplPath, fmt.Sprintf("energy_%s", zoneName)),
			maxEnergy:    1000000,
			increment:    Energy(100 + zoneIncrementFactor[zoneName]),
			randomFactor: 0.5,
		})
	}

	for _, opt := range opts {
		opt(meter)
	}

	return meter, nil
}

func (m *fakeRaplMeter) Name() string {
	return "fake-cpu-meter"
}

func (m *fakeRaplMeter) Zones() ([]EnergyZone, error) {
	return m.zones, nil
}

// PrimaryEnergyZone returns the zone with the highest energy coverage/priority
func (m *fakeRaplMeter) PrimaryEnergyZone() (EnergyZone, error) {
	zones, err := m.Zones()
	if err != nil {
		return nil, err
	}

	if len(zones) == 0 {
		return nil, fmt.Errorf("no zones available in fake meter")
	}

	// For fake meter, prefer package if available, otherwise first zone
	for _, zone := range zones {
		if strings.Contains(strings.ToLower(zone.Name()), "package") {
			return zone, nil
		}
	}

	// Fallback to first zone
	return zones[0], nil
}
