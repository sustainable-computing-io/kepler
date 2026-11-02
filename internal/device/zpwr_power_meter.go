// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package device

import (
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// zpwr is a CPU power meter for IBM Z and LinuxONE (linux/s390x).
//
// s390x has no RAPL and normally no hwmon power sensors. Instead, IBM z17 /
// LinuxONE 5 and later machines provide partition-level power readings
// through the DIAGNOSE 0x324 instruction. The kernel exposes it via
// /dev/diag (upstream since Linux 6.14; also backported by distributions,
// e.g. RHEL 10 with 6.12). This is the same interface used by the `zpwr`
// tool in s390-tools >= 2.38.
//
// Firmware reports instantaneous power (microwatts) rather than cumulative
// energy, so these zones behave like hwmon power zones: Power() returns a
// reading and the monitor integrates it over time.

const (
	defaultDiagDevicePath = "/dev/diag"
	zpwrMeterName         = "zpwr"

	// zpwrMinRefresh bounds how often the device is queried. Every zone is
	// read in the same monitor cycle, so they share one snapshot.
	zpwrMinRefresh = time.Second

	// zpwrMinBusyGrace is the minimum time the last successful reading may be
	// reused while firmware reports that a new reading is unavailable (EBUSY).
	// The effective grace is the larger of this and three firmware update
	// intervals; after it, Power() returns errPIBBusy instead of stale data.
	zpwrMinBusyGrace = 30 * time.Second
)

// zpwrDevicePath is the diag device used by the "zpwr" backend that
// CreateCPUMeter builds. It is a variable so tests can point it at a missing
// path and exercise fallback deterministically, even on IBM Z hosts.
var zpwrDevicePath = defaultDiagDevicePath

// Zone names reported by the zpwr meter.
const (
	ZoneLPAR       Zone = "lpar"   // sum of cpu + memory + io for this partition
	ZoneLPARCPU    Zone = "cpu"    // processors assigned to this partition
	ZoneLPARMemory Zone = "memory" // main storage assigned to this partition
	ZoneLPARIO     Zone = "io"     // I/O resources assigned to this partition
)

// pibSource abstracts /dev/diag so the meter can be tested on any platform.
type pibSource interface {
	// ReadPIB returns a raw PIB buffer and the kernel sequence number.
	// On errPIBBusy the returned buffer may hold a stale reading.
	ReadPIB() ([]byte, uint64, error)
	Close() error
}

type zpwrPowerMeter struct {
	logger     *slog.Logger
	devicePath string
	openFn     func(string) (pibSource, error)
	now        func() time.Time
	minRefresh time.Duration

	mu        sync.Mutex
	src       pibSource
	last      *pibReading
	lastFetch time.Time // last time the device was polled
	lastGood  time.Time // last time a fresh (non-busy) reading was decoded

	zones []EnergyZone
}

var _ CPUPowerMeter = (*zpwrPowerMeter)(nil)

// ZpwrOptionFn configures a zpwr power meter.
type ZpwrOptionFn func(*zpwrPowerMeter)

// WithZpwrLogger sets the logger.
func WithZpwrLogger(logger *slog.Logger) ZpwrOptionFn {
	return func(m *zpwrPowerMeter) {
		m.logger = logger.With("service", zpwrMeterName)
	}
}

// WithZpwrDevicePath overrides the diag device path (default /dev/diag).
func WithZpwrDevicePath(path string) ZpwrOptionFn {
	return func(m *zpwrPowerMeter) {
		m.devicePath = path
	}
}

// withZpwrSource injects a PIB source; used by tests.
func withZpwrSource(src pibSource) ZpwrOptionFn {
	return func(m *zpwrPowerMeter) {
		m.openFn = func(string) (pibSource, error) { return src, nil }
	}
}

// withZpwrClock injects a clock; used by tests.
func withZpwrClock(now func() time.Time) ZpwrOptionFn {
	return func(m *zpwrPowerMeter) {
		m.now = now
	}
}

// NewZpwrPowerMeter creates a power meter backed by s390 DIAGNOSE 0x324.
// The device is not opened until Init is called.
func NewZpwrPowerMeter(opts ...ZpwrOptionFn) (*zpwrPowerMeter, error) {
	m := &zpwrPowerMeter{
		logger:     slog.Default().With("service", zpwrMeterName),
		devicePath: defaultDiagDevicePath,
		openFn:     openPIBSource,
		now:        time.Now,
		minRefresh: zpwrMinRefresh,
	}
	for _, opt := range opts {
		opt(m)
	}

	m.zones = []EnergyZone{
		&zpwrZone{meter: m, name: ZoneLPAR, component: -1},
		&zpwrZone{meter: m, name: ZoneLPARCPU, component: pibLparCPU},
		&zpwrZone{meter: m, name: ZoneLPARMemory, component: pibLparStorage},
		&zpwrZone{meter: m, name: ZoneLPARIO, component: pibLparIO},
	}
	return m, nil
}

func (m *zpwrPowerMeter) Name() string {
	return zpwrMeterName
}

// Init opens the diag device and verifies that partition readings are available.
func (m *zpwrPowerMeter) Init() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.src == nil {
		src, err := m.openFn(m.devicePath)
		if err != nil {
			return err
		}
		m.src = src
	}

	r, err := m.refreshLocked(true)
	if err != nil {
		_ = m.closeLocked()
		return err
	}
	if !r.LPARValid {
		_ = m.closeLocked()
		return errors.New("diag324: firmware returned no partition power readings")
	}

	m.logger.Info("zpwr power readings available",
		"device", m.devicePath,
		"update-interval", r.Interval,
		"lpar-watts", Power(r.LPARTotal()).Watts(),
		"cpc-readings", r.CPCValid)
	if !r.CPCValid {
		m.logger.Debug("CPC-level power readings not available; enable 'Global Performance Data' in the LPAR activation profile to see them with zpwr")
	}
	return nil
}

// Zones returns the partition power zones.
func (m *zpwrPowerMeter) Zones() ([]EnergyZone, error) {
	return m.zones, nil
}

// PrimaryEnergyZone returns the whole-partition zone.
func (m *zpwrPowerMeter) PrimaryEnergyZone() (EnergyZone, error) {
	return m.zones[0], nil
}

// Close releases the diag device.
func (m *zpwrPowerMeter) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.closeLocked()
}

func (m *zpwrPowerMeter) closeLocked() error {
	if m.src == nil {
		return nil
	}
	err := m.src.Close()
	m.src = nil
	return err
}

// snapshot returns a reading no older than minRefresh.
func (m *zpwrPowerMeter) snapshot() (*pibReading, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.refreshLocked(false)
}

func (m *zpwrPowerMeter) refreshLocked(force bool) (*pibReading, error) {
	now := m.now()
	if !force && m.last != nil && now.Sub(m.lastFetch) < m.minRefresh {
		return m.cachedLocked(now)
	}
	if m.src == nil {
		return nil, errors.New("zpwr meter not initialized")
	}

	buf, seq, err := m.src.ReadPIB()
	busy := errors.Is(err, errPIBBusy)
	if err != nil && !busy {
		return nil, err
	}

	if busy && m.last != nil {
		// Briefly keep serving the last good reading until firmware has a
		// new one; busy responses never renew lastGood.
		m.lastFetch = now
		m.logger.Debug("diag324 reading busy, reusing previous reading",
			"sequence", m.last.Sequence, "age", now.Sub(m.lastGood))
		return m.cachedLocked(now)
	}

	r, perr := parsePIB(buf)
	if perr != nil {
		return nil, fmt.Errorf("decode power information block: %w", perr)
	}
	if busy && !r.LPARValid {
		return nil, err
	}
	r.Sequence = seq
	m.last = &r
	m.lastFetch = now
	// A busy first reading still carries the PIB the kernel last copied; it
	// starts the grace window but is never renewed by later busy responses.
	m.lastGood = now
	return m.last, nil
}

// cachedLocked returns the last reading, or errPIBBusy once it is older than
// the busy grace period, so the monitor does not integrate stale power into
// energy during a prolonged firmware outage.
func (m *zpwrPowerMeter) cachedLocked(now time.Time) (*pibReading, error) {
	age := now.Sub(m.lastGood)
	if grace := m.busyGraceLocked(); age > grace {
		return nil, fmt.Errorf("%w: no fresh reading for %s (grace %s)", errPIBBusy, age.Round(time.Second), grace)
	}
	return m.last, nil
}

// busyGraceLocked returns how long the last good reading may be reused while
// firmware reports EBUSY: three firmware update intervals, at least
// zpwrMinBusyGrace.
func (m *zpwrPowerMeter) busyGraceLocked() time.Duration {
	grace := zpwrMinBusyGrace
	if m.last != nil && 3*m.last.Interval > grace {
		grace = 3 * m.last.Interval
	}
	return grace
}

// zpwrZone is one partition power reading exposed as an EnergyZone.
type zpwrZone struct {
	meter     *zpwrPowerMeter
	name      string
	component int // index into pibReading.LPAR, or -1 for the sum
}

var _ EnergyZone = (*zpwrZone)(nil)

func (z *zpwrZone) Name() string { return z.name }

func (z *zpwrZone) Index() int { return 0 }

func (z *zpwrZone) Path() string { return z.meter.devicePath }

// Energy is not provided by firmware; the monitor integrates Power() instead.
func (z *zpwrZone) Energy() (Energy, error) {
	return 0, fmt.Errorf("zpwr zones do not provide energy readings")
}

// MaxEnergy returns 0, marking this as a power (not energy-counter) zone.
func (z *zpwrZone) MaxEnergy() Energy { return 0 }

// Power returns the current reading in microwatts.
func (z *zpwrZone) Power() (Power, error) {
	r, err := z.meter.snapshot()
	if err != nil {
		return 0, err
	}
	if !r.LPARValid {
		return 0, fmt.Errorf("zpwr: no partition reading for zone %s", z.name)
	}
	if z.component < 0 {
		return Power(r.LPARTotal()), nil
	}
	return Power(r.LPAR[z.component]), nil
}
