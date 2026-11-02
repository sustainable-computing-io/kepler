// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package device

import (
	"encoding/binary"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// pibBlock describes one reading block for buildPIB.
type pibBlock struct {
	format uint8
	vals   [3]uint64
	extra  int // padding bytes appended to the block
}

// buildPIB assembles a big-endian PIB exactly as firmware lays it out.
func buildPIB(intvTOD uint64, blocks ...pibBlock) []byte {
	const hlen = pibHeaderMinLen
	buf := make([]byte, hlen)
	buf[1] = uint8(len(blocks))
	buf[7] = hlen
	binary.BigEndian.PutUint64(buf[16:24], intvTOD)

	for _, b := range blocks {
		blen := pibBlockMinLen + b.extra
		blk := make([]byte, blen)
		blk[0] = b.format << 4
		blk[3] = uint8(blen)
		copy(blk[8:16], []byte{0xD3, 0xD7, 0xC1, 0xD9, 0x40, 0x40, 0x40, 0x40}) // "LPAR    " in EBCDIC
		for i, v := range b.vals {
			binary.BigEndian.PutUint64(blk[16+i*8:], v)
		}
		buf = append(buf, blk...)
	}
	binary.BigEndian.PutUint16(buf[2:4], uint16(len(buf)))
	// kernel buffer is usually larger than what firmware fills in
	return append(buf, make([]byte, 32)...)
}

// 1 second in TOD units (4096 units per microsecond)
const todOneSecond = 4096 * 1_000_000

func TestTodToDuration(t *testing.T) {
	assert.Equal(t, time.Second, todToDuration(todOneSecond))
	assert.Equal(t, time.Microsecond, todToDuration(4096))
	assert.Equal(t, time.Duration(0), todToDuration(0))
}

func TestParsePIB(t *testing.T) {
	buf := buildPIB(5*todOneSecond,
		pibBlock{format: pibFormatPartition, vals: [3]uint64{120_000_000, 30_000_000, 10_000_000}},
		pibBlock{format: pibFormatCPC, vals: [3]uint64{9_000_000_000, 500_000_000, 1_200_000_000}},
	)

	r, err := parsePIB(buf)
	require.NoError(t, err)
	assert.Equal(t, 5*time.Second, r.Interval)
	assert.True(t, r.LPARValid)
	assert.Equal(t, [3]uint64{120_000_000, 30_000_000, 10_000_000}, r.LPAR)
	assert.Equal(t, uint64(160_000_000), r.LPARTotal())
	assert.True(t, r.CPCValid)
	assert.Equal(t, uint64(9_000_000_000), r.CPC[pibCPCTotal])
	assert.Equal(t, uint64(500_000_000), r.CPC[pibCPCUnassigned])
	assert.Equal(t, uint64(1_200_000_000), r.CPC[pibCPCInfrastructure])
}

func TestParsePIB_PartitionOnly(t *testing.T) {
	buf := buildPIB(todOneSecond, pibBlock{format: pibFormatPartition, vals: [3]uint64{1, 2, 3}})
	r, err := parsePIB(buf)
	require.NoError(t, err)
	assert.True(t, r.LPARValid)
	assert.False(t, r.CPCValid, "CPC block absent without Global Performance Data authorization")
}

func TestParsePIB_SkipsUnknownFormatAndPadding(t *testing.T) {
	buf := buildPIB(todOneSecond,
		pibBlock{format: 7, vals: [3]uint64{99, 99, 99}, extra: 16},
		pibBlock{format: pibFormatPartition, vals: [3]uint64{4, 5, 6}, extra: 8},
	)
	r, err := parsePIB(buf)
	require.NoError(t, err)
	assert.True(t, r.LPARValid)
	assert.Equal(t, [3]uint64{4, 5, 6}, r.LPAR)
}

func TestParsePIB_Errors(t *testing.T) {
	valid := buildPIB(todOneSecond, pibBlock{format: pibFormatPartition, vals: [3]uint64{1, 2, 3}})

	tests := []struct {
		name   string
		mutate func([]byte) []byte
	}{
		{"too short", func(b []byte) []byte { return b[:10] }},
		{"hlen too small", func(b []byte) []byte { b[7] = 8; return b }},
		{"hlen beyond buffer", func(b []byte) []byte { b[7] = 0xff; return b[:40] }},
		{"block length zero", func(b []byte) []byte { b[pibHeaderMinLen+3] = 0; return b }},
		{"block truncated", func(b []byte) []byte {
			binary.BigEndian.PutUint16(b[2:4], pibHeaderMinLen+20)
			return b
		}},
		{"more blocks than data", func(b []byte) []byte { b[1] = 5; return b }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			b := append([]byte(nil), valid...)
			_, err := parsePIB(tc.mutate(b))
			assert.Error(t, err)
		})
	}
}

// fakePIBSource returns queued PIBs.
type fakePIBSource struct {
	bufs   [][]byte
	errs   []error
	calls  int
	closed bool
}

func (f *fakePIBSource) ReadPIB() ([]byte, uint64, error) {
	i := f.calls
	if i >= len(f.bufs) {
		i = len(f.bufs) - 1
	}
	f.calls++
	var err error
	if i < len(f.errs) {
		err = f.errs[i]
	}
	return f.bufs[i], uint64(i + 1), err
}

func (f *fakePIBSource) Close() error {
	f.closed = true
	return nil
}

type fakeClock struct{ t time.Time }

func (c *fakeClock) now() time.Time          { return c.t }
func (c *fakeClock) advance(d time.Duration) { c.t = c.t.Add(d) }

func lparPIB(cpu, mem, io uint64) []byte {
	return buildPIB(todOneSecond, pibBlock{format: pibFormatPartition, vals: [3]uint64{cpu, mem, io}})
}

func TestZpwrPowerMeter_ZonesAndPower(t *testing.T) {
	src := &fakePIBSource{bufs: [][]byte{
		lparPIB(100*uint64(Watt), 20*uint64(Watt), 5*uint64(Watt)),
		lparPIB(200*uint64(Watt), 20*uint64(Watt), 5*uint64(Watt)),
	}}
	clk := &fakeClock{t: time.Unix(1000, 0)}

	m, err := NewZpwrPowerMeter(WithZpwrLogger(discardLogger()), withZpwrSource(src), withZpwrClock(clk.now))
	require.NoError(t, err)
	assert.Equal(t, "zpwr", m.Name())
	require.NoError(t, m.Init())

	zones, err := m.Zones()
	require.NoError(t, err)
	require.Len(t, zones, 4)

	names := map[string]EnergyZone{}
	for _, z := range zones {
		names[z.Name()] = z
		assert.Equal(t, Energy(0), z.MaxEnergy(), "zpwr zones must be detected as power zones")
		e, err := z.Energy()
		assert.Error(t, err)
		assert.Equal(t, Energy(0), e)
	}

	primary, err := m.PrimaryEnergyZone()
	require.NoError(t, err)
	assert.Equal(t, ZoneLPAR, primary.Name())

	// First reading (taken during Init) is cached across zones.
	p, err := names[ZoneLPAR].Power()
	require.NoError(t, err)
	assert.InDelta(t, 125.0, p.Watts(), 1e-9)
	p, _ = names[ZoneLPARCPU].Power()
	assert.InDelta(t, 100.0, p.Watts(), 1e-9)
	p, _ = names[ZoneLPARMemory].Power()
	assert.InDelta(t, 20.0, p.Watts(), 1e-9)
	p, _ = names[ZoneLPARIO].Power()
	assert.InDelta(t, 5.0, p.Watts(), 1e-9)
	assert.Equal(t, 1, src.calls, "zones within one refresh window share a single ioctl")

	// After the refresh window a new reading is fetched.
	clk.advance(2 * time.Second)
	p, _ = names[ZoneLPARCPU].Power()
	assert.InDelta(t, 200.0, p.Watts(), 1e-9)
	p, _ = names[ZoneLPAR].Power()
	assert.InDelta(t, 225.0, p.Watts(), 1e-9)
	assert.Equal(t, 2, src.calls)

	require.NoError(t, m.Close())
	assert.True(t, src.closed)
}

func TestZpwrPowerMeter_BusyReusesLastReading(t *testing.T) {
	src := &fakePIBSource{
		bufs: [][]byte{lparPIB(50*uint64(Watt), 0, 0), make([]byte, 64)},
		errs: []error{nil, errPIBBusy},
	}
	clk := &fakeClock{t: time.Unix(0, 0)}
	m, _ := NewZpwrPowerMeter(WithZpwrLogger(discardLogger()), withZpwrSource(src), withZpwrClock(clk.now))
	require.NoError(t, m.Init())

	clk.advance(5 * time.Second)
	p, err := m.zones[0].Power()
	require.NoError(t, err)
	assert.InDelta(t, 50.0, p.Watts(), 1e-9)
}

func TestZpwrPowerMeter_BusyBeyondGraceReturnsError(t *testing.T) {
	// good, busy (within grace), busy (beyond grace), good again
	good := lparPIB(50*uint64(Watt), 0, 0)
	src := &fakePIBSource{
		bufs: [][]byte{good, make([]byte, 64), make([]byte, 64), lparPIB(60*uint64(Watt), 0, 0)},
		errs: []error{nil, errPIBBusy, errPIBBusy, nil},
	}
	clk := &fakeClock{t: time.Unix(0, 0)}
	m, _ := NewZpwrPowerMeter(WithZpwrLogger(discardLogger()), withZpwrSource(src), withZpwrClock(clk.now))
	require.NoError(t, m.Init())
	zone := m.zones[0]

	// Within grace: the last good reading is reused.
	clk.advance(10 * time.Second)
	p, err := zone.Power()
	require.NoError(t, err)
	assert.InDelta(t, 50.0, p.Watts(), 1e-9)

	// Busy responses do not extend the grace window: 40s after the last
	// good reading (grace is 30s for a 1s firmware interval) Power() fails.
	clk.advance(30 * time.Second)
	_, err = zone.Power()
	require.Error(t, err)
	assert.True(t, errors.Is(err, errPIBBusy))
	// Other zones read in the same cycle (served from the refresh cache)
	// must not get the stale reading either.
	_, err = m.zones[1].Power()
	assert.True(t, errors.Is(err, errPIBBusy))

	// A fresh reading recovers.
	clk.advance(2 * time.Second)
	p, err = zone.Power()
	require.NoError(t, err)
	assert.InDelta(t, 60.0, p.Watts(), 1e-9)
}

func TestZpwrPowerMeter_BusyGraceScalesWithInterval(t *testing.T) {
	m, _ := NewZpwrPowerMeter(WithZpwrLogger(discardLogger()))
	assert.Equal(t, zpwrMinBusyGrace, m.busyGraceLocked(), "no reading yet")

	m.last = &pibReading{Interval: time.Second}
	assert.Equal(t, zpwrMinBusyGrace, m.busyGraceLocked(), "short interval uses the minimum")

	m.last = &pibReading{Interval: 20 * time.Second}
	assert.Equal(t, 60*time.Second, m.busyGraceLocked(), "three firmware intervals")
}

func TestZpwrPowerMeter_InitFailures(t *testing.T) {
	t.Run("open fails", func(t *testing.T) {
		m, _ := NewZpwrPowerMeter(WithZpwrLogger(discardLogger()))
		m.openFn = func(string) (pibSource, error) { return nil, errPIBNotSupported }
		err := m.Init()
		require.Error(t, err)
		assert.True(t, errors.Is(err, errPIBNotSupported))
	})

	t.Run("read fails", func(t *testing.T) {
		src := &fakePIBSource{bufs: [][]byte{nil}, errs: []error{errors.New("boom")}}
		m, _ := NewZpwrPowerMeter(WithZpwrLogger(discardLogger()), withZpwrSource(src))
		require.Error(t, m.Init())
		assert.True(t, src.closed)
	})

	t.Run("no partition block", func(t *testing.T) {
		src := &fakePIBSource{bufs: [][]byte{
			buildPIB(todOneSecond, pibBlock{format: pibFormatCPC, vals: [3]uint64{1, 2, 3}}),
		}}
		m, _ := NewZpwrPowerMeter(WithZpwrLogger(discardLogger()), withZpwrSource(src))
		require.Error(t, m.Init())
	})

	t.Run("busy with no prior reading", func(t *testing.T) {
		src := &fakePIBSource{bufs: [][]byte{make([]byte, 64)}, errs: []error{errPIBBusy}}
		m, _ := NewZpwrPowerMeter(WithZpwrLogger(discardLogger()), withZpwrSource(src))
		require.Error(t, m.Init())
	})
}

func TestZpwrPowerMeter_UnsupportedPlatformFallsThrough(t *testing.T) {
	// On non-s390x the real openPIBSource always reports "not supported".
	// On s390x without /dev/diag it does too, so this is safe everywhere
	// as long as the path does not exist.
	m, err := NewZpwrPowerMeter(WithZpwrLogger(discardLogger()), WithZpwrDevicePath("/nonexistent/diag"))
	require.NoError(t, err)
	assert.Error(t, m.Init())
}
