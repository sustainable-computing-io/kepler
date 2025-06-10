// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package device

import (
	"fmt"
	"log/slog"
	"math/rand"
	"path/filepath"
	"sync"
	"time"

	"github.com/prometheus/procfs"
)

// NOTE: This fake meter is not intended to be used in production and is for testing only

type Zone = string

const (
	ZonePackage Zone = "package"
	ZoneCore    Zone = "core"
	ZoneDRAM    Zone = "dram"
	ZoneUncore  Zone = "uncore"
)

var defaultFakeZones = []Zone{ZonePackage, ZoneCore, ZoneDRAM}

const defaultRaplPath = "/sys/class/powercap/intel-rapl"

// cpuUsageReader interface for getting CPU usage
type cpuUsageReader interface {
	CPUUsageRatio() (float64, error)
}

// procFSCPUReader implements cpuUsageReader using procfs
type procFSCPUReader struct {
	fs       procfs.FS
	prevStat procfs.CPUStat
	mu       sync.Mutex
}

func (r *procFSCPUReader) CPUUsageRatio() (float64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	current, err := r.fs.Stat()
	if err != nil {
		return 0.5, nil // Return moderate CPU usage on error
	}

	prev := r.prevStat
	r.prevStat = current.CPUTotal

	// first time, return moderate usage
	if prev == (procfs.CPUStat{}) {
		return 0.3, nil
	}

	curr := current.CPUTotal

	// calculate delta for all components
	dUser := curr.User - prev.User
	dNice := curr.Nice - prev.Nice
	dSystem := curr.System - prev.System
	dIdle := curr.Idle - prev.Idle
	dIowait := curr.Iowait - prev.Iowait
	dIRQ := curr.IRQ - prev.IRQ
	dSoftIRQ := curr.SoftIRQ - prev.SoftIRQ
	dSteal := curr.Steal - prev.Steal

	total := dUser + dNice + dSystem + dIdle + dIowait + dIRQ + dSoftIRQ + dSteal
	if total == 0 {
		return 0.2, nil // Return low usage if no CPU time elapsed
	}

	active := total - (dIdle + dIowait)
	ratio := active / total
	return ratio, nil
}

// fakeCPUReader provides fake CPU usage for testing
type fakeCPUReader struct {
	baseUsage    float64
	randomFactor float64
	mu           sync.Mutex
}

func (f *fakeCPUReader) CPUUsageRatio() (float64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Generate realistic CPU usage between 0.1 and 0.9
	random := rand.Float64() * f.randomFactor
	usage := f.baseUsage + random - (f.randomFactor / 2)

	// Clamp between reasonable bounds
	if usage < 0.05 {
		usage = 0.05
	}
	if usage > 0.95 {
		usage = 0.95
	}

	return usage, nil
}

// fakeEnergyZone implements the EnergyZone interface
type fakeEnergyZone struct {
	name      string
	index     int
	path      string
	energy    Energy
	maxEnergy Energy
	mu        sync.Mutex

	// For generating fake values
	baseWatts    float64 // Base power consumption in watts
	randomFactor float64
	lastReadTime time.Time
	cpuReader    cpuUsageReader
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
// Energy consumption is calculated based on:
// 1. Time elapsed since last reading (more time = more energy)
// 2. Current CPU usage (higher usage = higher power consumption)
// 3. Base power consumption for the zone type
func (z *fakeEnergyZone) Energy() (Energy, error) {
	z.mu.Lock()
	defer z.mu.Unlock()

	now := time.Now()

	// Initialize timestamp on first call
	if z.lastReadTime.IsZero() {
		z.lastReadTime = now
		return z.energy, nil
	}

	// Calculate time elapsed in seconds
	duration := now.Sub(z.lastReadTime).Seconds()
	z.lastReadTime = now

	// Get current CPU usage (0.0 to 1.0)
	cpuUsage := 0.3 // Default moderate usage
	if z.cpuReader != nil {
		if usage, err := z.cpuReader.CPUUsageRatio(); err == nil {
			cpuUsage = usage
		}
	}

	// Calculate power consumption based on CPU usage
	// Power scales from 50% to 150% of base power based on CPU usage
	powerMultiplier := 0.5 + cpuUsage
	actualWatts := z.baseWatts * powerMultiplier

	// Add some randomness (±10% of base power)
	randomFactor := (rand.Float64() - 0.5) * z.randomFactor * z.baseWatts
	actualWatts += randomFactor

	// Ensure minimum power consumption
	if actualWatts < z.baseWatts*0.1 {
		actualWatts = z.baseWatts * 0.1
	}

	// Convert watts to microjoules: Watts × seconds × 1,000,000
	energyIncrement := Energy(actualWatts * duration * 1000000)

	// Update cumulative energy with wrap-around
	z.energy = (z.energy + energyIncrement) % z.maxEnergy

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

	// Base power consumption in watts for different zones
	// These are realistic values for modern CPUs
	zoneBasePower := map[Zone]float64{
		ZonePackage: 45.0, // Total package power
		ZoneCore:    25.0, // CPU cores
		ZoneDRAM:    8.0,  // Memory controller
		ZoneUncore:  12.0, // Uncore (caches, interconnect)
	}

	// Create CPU usage reader
	var cpuReader cpuUsageReader
	if fs, err := procfs.NewDefaultFS(); err == nil {
		cpuReader = &procFSCPUReader{fs: fs}
	} else {
		// Fallback to fake reader if procfs is not available
		cpuReader = &fakeCPUReader{
			baseUsage:    0.4,
			randomFactor: 0.3,
		}
	}

	meter.zones = make([]EnergyZone, 0, len(zones))

	for i, zoneName := range zones {
		basePower := zoneBasePower[zoneName]
		if basePower == 0 {
			// Default power for unknown zones
			basePower = 10.0
		}

		meter.zones = append(meter.zones, &fakeEnergyZone{
			name:         zoneName,
			index:        i,
			path:         filepath.Join(defaultRaplPath, fmt.Sprintf("energy_%s", zoneName)),
			maxEnergy:    1000000000000, // 1 trillion microjoules (about 278 kWh)
			baseWatts:    basePower,
			randomFactor: 0.2, // ±20% randomness
			cpuReader:    cpuReader,
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
