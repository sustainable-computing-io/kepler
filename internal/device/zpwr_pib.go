// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package device

import (
	"encoding/binary"
	"errors"
	"fmt"
	"time"
)

// This file decodes the Power Information Block (PIB) returned by the
// s390 DIAGNOSE 0x324 instruction, which the Linux kernel exposes through
// the DIAG324_GET_PIBBUF ioctl on /dev/diag (upstream since 6.14; also
// backported by distributions, e.g. RHEL 10 with 6.12). The layout mirrors
// `struct pib` / `struct pib_prologue` from s390-tools (zpwr/zpwr.h).
//
// The data is produced by firmware in big-endian byte order (the native order
// of s390x), so it is decoded explicitly with binary.BigEndian. This keeps the
// parser portable and lets it be unit tested on any architecture.
//
//	PIB header (hlen bytes, at least 24):
//	  byte  0      reserved
//	  byte  1      num   - number of reading blocks that follow
//	  bytes 2-3    len   - total length of the PIB in bytes
//	  bytes 4-6    reserved
//	  byte  7      hlen  - header length; first block starts at this offset
//	  bytes 8-15   reserved
//	  bytes 16-23  intv  - update interval in TOD clock units
//
//	Reading block (repeated num times, each `len` bytes long):
//	  byte  0      format (upper 4 bits): 0 = partition (LPAR), 1 = CPC
//	  byte  3      len    - length of this block in bytes
//	  bytes 4-7    reserved
//	  bytes 8-15   name (EBCDIC, unused here)
//	  bytes 16..   three big-endian uint64 readings in microwatts
//	               LPAR: cpu, storage (memory), I/O
//	               CPC:  total, unassigned resources, infrastructure

const (
	pibHeaderMinLen   = 24
	pibPrologueLen    = 8
	pibNameLen        = 8
	pibReadingsPerBlk = 3
	pibBlockMinLen    = pibPrologueLen + pibNameLen + pibReadingsPerBlk*8

	pibFormatPartition = 0
	pibFormatCPC       = 1
)

// Indices into pibReading.LPAR
const (
	pibLparCPU = iota
	pibLparStorage
	pibLparIO
)

// Indices into pibReading.CPC
const (
	pibCPCTotal = iota
	pibCPCUnassigned
	pibCPCInfrastructure
)

// errPIBBusy is returned by a pibSource when the firmware reports that a new
// power reading is not yet available (kernel returns EBUSY).
var errPIBBusy = errors.New("diag324: power reading temporarily unavailable")

// errPIBNotSupported is returned when the kernel or machine does not support
// DIAGNOSE 0x324 power readings.
var errPIBNotSupported = errors.New("diag324: power readings not supported on this system")

// pibReading is the decoded content of one PIB.
type pibReading struct {
	// Sequence is the kernel's reading sequence number; it only advances
	// when firmware delivers a fresh reading.
	Sequence uint64
	// Interval is how often firmware refreshes the readings.
	Interval time.Duration

	// LPAR holds partition readings (cpu, storage, io) in microwatts.
	LPAR      [pibReadingsPerBlk]uint64
	LPARValid bool

	// CPC holds machine-wide readings (total, unassigned, infrastructure) in
	// microwatts. Only present when the LPAR is authorized for
	// "Global Performance Data".
	CPC      [pibReadingsPerBlk]uint64
	CPCValid bool
}

// LPARTotal returns the sum of the partition readings in microwatts.
func (r *pibReading) LPARTotal() uint64 {
	return r.LPAR[pibLparCPU] + r.LPAR[pibLparStorage] + r.LPAR[pibLparIO]
}

// todToDuration converts s390 TOD clock units to a time.Duration.
// Bit 51 of the TOD clock is one microsecond, i.e. 4096 units == 1µs.
// Same arithmetic as the kernel's tod_to_ns().
func todToDuration(tod uint64) time.Duration {
	ns := (tod>>9)*125 + ((tod&0x1ff)*125)>>9
	return time.Duration(ns)
}

// parsePIB decodes a raw PIB buffer.
func parsePIB(buf []byte) (pibReading, error) {
	var r pibReading

	if len(buf) < pibHeaderMinLen {
		return r, fmt.Errorf("pib too short: %d bytes", len(buf))
	}

	num := int(buf[1])
	total := int(binary.BigEndian.Uint16(buf[2:4]))
	hlen := int(buf[7])
	r.Interval = todToDuration(binary.BigEndian.Uint64(buf[16:24]))

	// Never read past what firmware says it filled in, nor past the buffer.
	if total > 0 && total < len(buf) {
		buf = buf[:total]
	}
	if hlen < pibHeaderMinLen || hlen > len(buf) {
		return r, fmt.Errorf("invalid pib header length %d (buffer %d bytes)", hlen, len(buf))
	}

	off := hlen
	for i := 0; i < num; i++ {
		if off+pibPrologueLen > len(buf) {
			return r, fmt.Errorf("pib block %d at offset %d exceeds buffer (%d bytes)", i, off, len(buf))
		}
		format := buf[off] >> 4
		blen := int(buf[off+3])
		if blen < pibPrologueLen {
			return r, fmt.Errorf("pib block %d has invalid length %d", i, blen)
		}

		var dst *[pibReadingsPerBlk]uint64
		switch format {
		case pibFormatPartition:
			dst, r.LPARValid = &r.LPAR, true
		case pibFormatCPC:
			dst, r.CPCValid = &r.CPC, true
		default:
			// Unknown (newer) block format: skip it for forward compatibility.
			off += blen
			continue
		}

		if blen < pibBlockMinLen || off+pibBlockMinLen > len(buf) {
			return r, fmt.Errorf("pib block %d (format %d) truncated: len %d, offset %d, buffer %d",
				i, format, blen, off, len(buf))
		}
		vals := off + pibPrologueLen + pibNameLen
		for c := 0; c < pibReadingsPerBlk; c++ {
			dst[c] = binary.BigEndian.Uint64(buf[vals+c*8:])
		}
		off += blen
	}

	return r, nil
}
