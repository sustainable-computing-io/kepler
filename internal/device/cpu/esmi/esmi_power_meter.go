// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package esmi

import (
	"fmt"
	"github.com/sustainable-computing-io/kepler/internal/device"
	"github.com/sustainable-computing-io/kepler/internal/device/cpu"
	"log/slog"
	"strings"
	"sync"
	"time"
)

// NOTE: This esmi meter is not intended to be used in production and is for testing only
var defaultEsmiZones = []device.Zone{device.ZonePackage, device.ZoneCore, device.ZoneDRAM}

// EsmiMeter implements the CPUPowerMeter interface
type EsmiMeter struct {
	logger     *slog.Logger
	zones      []device.EnergyZone
	lastSample map[string]time.Time
	lastEnergy map[string]device.Energy
	mu         sync.Mutex

	coreCount   int
	socketCount int
}

var _ cpu.CPUPowerMeter = (*EsmiMeter)(nil)

// EsmiOptFn is a functional option for configuring EsmiRaplMeter
type EsmiOptFn func(*EsmiMeter)

// EsmiEnergyZone implements the EnergyZone interface
type EsmiEnergyZone struct {
	name      string
	index     int
	energy    device.Energy
	maxEnergy device.Energy
	mu        sync.Mutex

	meter *EsmiMeter
}

//var _ device.EnergyZone = (*EsmiEnergyZone)(nil)

// Name returns the zone name
func (z *EsmiEnergyZone) Name() string {
	return z.name
}

// Index returns the index of the zone
func (z *EsmiEnergyZone) Index() int {
	return z.index
}

func (z *EsmiEnergyZone) Path() string {
	return fmt.Sprintf("esmi/%s/%d", z.name, z.index)
}

// Energy returns energy consumed by the zone.
func (z *EsmiEnergyZone) Energy() (device.Energy, error) {
	z.mu.Lock()
	defer z.mu.Unlock()

	m := z.meter

	m.mu.Lock()
	defer m.mu.Unlock()

	var totalPower float64

	for s := 0; s < m.socketCount; s++ {
		var p float64
		var err error

		switch z.name {
		case device.ZonePackage:
			p, err = GetSocketPower(s)

		case device.ZoneDRAM:
			p, err = GetDramPower(s)

		case device.ZoneCore:
			p, err = GetCorePower(s)

		default:
			continue
		}

		if err != nil {
			return 0, err
		}

		totalPower += p
	}

	now := time.Now()
	key := string(z.name)

	lastT, ok := m.lastSample[key]
	if !ok {
		m.lastSample[key] = now
		return 0, nil
	}

	delta := now.Sub(lastT).Seconds()
	m.lastSample[key] = now

	// Energy = Power * Time
	energy := device.Energy(totalPower * delta * 1000) // mJ

	m.lastEnergy[key] += energy
	return m.lastEnergy[key], nil
}

// MaxEnergy returns the maximum value of energy usage that can be read.
func (z *EsmiEnergyZone) MaxEnergy() device.Energy {
	return z.maxEnergy
}

func (z *EsmiEnergyZone) Power() (device.Power, error) {
	z.mu.Lock()
	defer z.mu.Unlock()

	m := z.meter

	m.mu.Lock()
	defer m.mu.Unlock()

	var totalPower float64

	switch z.name {

	case device.ZonePackage:
		for s := 0; s < m.socketCount; s++ {
			p, err := GetSocketPower(s)
			if err != nil {
				return 0, err
			}
			totalPower += p
		}
		return device.Power(totalPower * 1000), nil //mW

	case device.ZoneDRAM:
		for s := 0; s < m.socketCount; s++ {
			p, err := GetDramPower(s)
			if err != nil {
				return 0, err
			}
			totalPower += p
		}
		return device.Power(totalPower), nil

	// CORE → only energy API available → approximate power
	case device.ZoneCore:
		var totalEnergy float64

		for c := 0; c < m.coreCount; c++ {
			e, err := GetCoreEnergy(c)
			if err != nil {
				return 0, err
			}
			totalEnergy += e
		}

		now := time.Now()
		key := string(z.name)

		lastT, ok := m.lastSample[key]
		lastE := m.lastEnergy[key]

		// First sample → initialize
		if !ok {
			m.lastSample[key] = now
			m.lastEnergy[key] = device.Energy(totalEnergy)
			return 0, nil
		}

		deltaT := now.Sub(lastT).Seconds()
		if deltaT <= 0 {
			return 0, nil
		}

		deltaE := totalEnergy - float64(lastE)
		if deltaE < 0 {
			deltaE = 0 // handle counter reset
		}

		// update state
		m.lastSample[key] = now
		m.lastEnergy[key] = device.Energy(totalEnergy)

		// Power = ΔE / Δt
		watts := deltaE / deltaT

		return device.Power(watts), nil

	default:
		return 0, fmt.Errorf("unsupported zone: %s", z.name)
	}
}

// WithEsmiPath sets the base device path for the esmi meter
//func WithEsmiPath(path string) EsmiOptFn {
//	return func(m *EsmiMeter) {
//		m.devicePath = path
//		for _, z := range m.zones {
//			if fz, ok := z.(*EsmiEnergyZone); ok {
//				fz.path = filepath.Join(path, fmt.Sprintf("energy_%s", fz.name))
//			}
//		}
//	}
//}

// WithEsmiMaxEnergy sets the maximum energy value before wrap-around
//func WithEsmiMaxEnergy(e device.Energy) EsmiOptFn {
//	return func(m *EsmiMeter) {
//		for _, z := range m.zones {
//			if fz, ok := z.(*EsmiEnergyZone); ok {
//				fz.maxEnergy = e
//			}
//		}
//	}
//}

func WithEsmiLogger(l *slog.Logger) EsmiOptFn {
	return func(m *EsmiMeter) {
		m.logger = l.With("meter", m.Name())
	}
}

// NewEsmiCPUMeter creates a new esmi CPU power meter
func NewEsmiCPUMeter(zones []string, opts ...EsmiOptFn) (cpu.CPUPowerMeter, error) {
	if err := Init(); err != nil {
		return nil, err
	}

	socketCount, err := GetSocketCount()
	if err != nil {
		return nil, err
	}

	coreCount, err := GetCoreCount()
	if err != nil {
		return nil, err
	}

	meter := &EsmiMeter{
		socketCount: socketCount,
		coreCount:   coreCount,
		lastSample:  make(map[string]time.Time),
		lastEnergy:  make(map[string]device.Energy),
		logger:      slog.Default().With("meter", "esmi-meter"),
	}

	// nil and empty slices are equivalent
	if len(zones) == 0 {
		zones = defaultEsmiZones
	}

	meter.zones = make([]device.EnergyZone, 0, len(zones))

	for i, zoneName := range zones {
		meter.zones = append(meter.zones, &EsmiEnergyZone{
			name:      zoneName,
			index:     i,
			energy:    0,
			maxEnergy: 0,
			meter:     meter,
		})
	}

	for _, opt := range opts {
		opt(meter)
	}

	return meter, nil
}

func (m *EsmiMeter) Name() string {
	return "esmi-meter"
}

func (m *EsmiMeter) Zones() ([]device.EnergyZone, error) {
	return m.zones, nil
}

// PrimaryEnergyZone returns the zone with the highest energy coverage/priority
func (m *EsmiMeter) PrimaryEnergyZone() (device.EnergyZone, error) {
	zones, err := m.Zones()
	if err != nil {
		return nil, err
	}

	if len(zones) == 0 {
		return nil, fmt.Errorf("no zones available in esmi meter")
	}

	// For esmi meter, prefer package if available, otherwise first zone
	for _, zone := range zones {
		if strings.Contains(strings.ToLower(zone.Name()), "package") {
			return zone, nil
		}
	}

	// Fallback to first zone
	return zones[0], nil
}
