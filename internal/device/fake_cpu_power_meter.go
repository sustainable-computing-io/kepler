// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package device

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"path/filepath"
	"sync"
	"sync/atomic"
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
	maxEnergy Energy

	// Thread-safe precomputed energy value
	precomputedEnergy atomic.Uint64

	// For generating fake values
	baseWatts    float64 // Base power consumption in watts
	randomFactor float64

	// Ticker management
	ctx        context.Context
	cancel     context.CancelFunc
	ticker     *time.Ticker
	cpuReader  cpuUsageReader
	lastUpdate time.Time
	mu         sync.RWMutex // Protects lastUpdate time
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

// Energy returns the precomputed energy value
func (z *fakeEnergyZone) Energy() (Energy, error) {
	return Energy(z.precomputedEnergy.Load()), nil
}

// startTicker starts the background ticker that updates energy values
func (z *fakeEnergyZone) startTicker(interval time.Duration) {
	z.ticker = time.NewTicker(interval)
	go z.tickerLoop()
}

// tickerLoop runs the background energy computation
func (z *fakeEnergyZone) tickerLoop() {
	// Initialize timestamp
	z.mu.Lock()
	z.lastUpdate = time.Now()
	z.mu.Unlock()

	for {
		select {
		case <-z.ctx.Done():
			return
		case <-z.ticker.C:
			z.updateEnergy()
		}
	}
}

// updateEnergy computes and updates the precomputed energy value
func (z *fakeEnergyZone) updateEnergy() {
	z.mu.Lock()
	now := time.Now()
	duration := now.Sub(z.lastUpdate).Seconds()
	z.lastUpdate = now
	z.mu.Unlock()

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

	// Update cumulative energy with wrap-around using atomic operations
	for {
		current := z.precomputedEnergy.Load()
		newValue := (current + uint64(energyIncrement)) % uint64(z.maxEnergy)
		if z.precomputedEnergy.CompareAndSwap(current, newValue) {
			break
		}
	}
}

// stop stops the ticker and cancels the context
func (z *fakeEnergyZone) stop() {
	if z.cancel != nil {
		z.cancel()
	}
	if z.ticker != nil {
		z.ticker.Stop()
	}
}

// MaxEnergy returns the maximum value of energy usage that can be read.
func (z *fakeEnergyZone) MaxEnergy() Energy {
	return z.maxEnergy
}

// fakeRaplMeter implements the CPUPowerMeter interface
type fakeRaplMeter struct {
	logger         *slog.Logger
	zones          []EnergyZone
	devicePath     string
	tickerInterval time.Duration
	ctx            context.Context
	cancel         context.CancelFunc
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

// WithFakeLogger sets the logger for the fake meter
func WithFakeLogger(l *slog.Logger) FakeOptFn {
	return func(m *fakeRaplMeter) {
		m.logger = l.With("meter", m.Name())
	}
}

// WithTickerInterval sets the ticker interval for energy updates
func WithTickerInterval(interval time.Duration) FakeOptFn {
	return func(m *fakeRaplMeter) {
		m.tickerInterval = interval
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

	// Set default ticker interval if not specified
	if meter.tickerInterval == 0 {
		meter.tickerInterval = 100 * time.Millisecond // Default 100ms
	}

	// Create context for lifecycle management
	meter.ctx, meter.cancel = context.WithCancel(context.Background())

	meter.zones = make([]EnergyZone, 0, len(zones))

	for i, zoneName := range zones {
		basePower := zoneBasePower[zoneName]
		if basePower == 0 {
			// Default power for unknown zones
			basePower = 10.0
		}

		// Create zone context derived from meter context
		zoneCtx, zoneCancel := context.WithCancel(meter.ctx)

		zone := &fakeEnergyZone{
			name:         zoneName,
			index:        i,
			path:         filepath.Join(defaultRaplPath, fmt.Sprintf("energy_%s", zoneName)),
			maxEnergy:    1000000000000, // 1 trillion microjoules (about 278 kWh)
			baseWatts:    basePower,
			randomFactor: 0.2, // ±20% randomness
			cpuReader:    cpuReader,
			ctx:          zoneCtx,
			cancel:       zoneCancel,
		}

		// Start the ticker for this zone
		zone.startTicker(meter.tickerInterval)

		meter.zones = append(meter.zones, zone)
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

// Stop stops all background tickers and cleans up resources
func (m *fakeRaplMeter) Stop() {
	if m.cancel != nil {
		m.cancel()
	}
	// Stop all zone tickers
	for _, zone := range m.zones {
		if fz, ok := zone.(*fakeEnergyZone); ok {
			fz.stop()
		}
	}
}
