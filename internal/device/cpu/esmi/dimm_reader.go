// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

//go:build goamdsmi

package esmi

// DimmReader provides per-DIMM power readings via the AMD E-SMI C library
// (libe_smi64.so) through CGo. It is compiled only with -tags goamdsmi.
//
// The E-SMI library exposes esmi_dimm_power_consumption_get(socket, dimmAddr, *power)
// which returns instantaneous DIMM power in milliwatts. DIMM addresses are
// enumerated per-socket; the library uses the SPD address (0x80–0x8b).
//
// Unlike the energy-counter backends, DimmReader zones only support Power()
// (instantaneous mW → W). Energy() returns an error directing callers to
// integrate Power() readings over time.
//
// Build requirements:
//   The E-SMI library (libe_smi64.so) and headers must be installed.
//   If installed in a non-standard location (e.g., /opt/e-sms/e_smi/),
//   update the CGO directives below with the correct paths.

/*
#cgo CFLAGS: -I/opt/e-sms/e_smi/include
#cgo LDFLAGS: -L/opt/e-sms/e_smi/lib -le_smi64

#include <stdint.h>
#include <e_smi/e_smi.h>

// esmi_dimm_power_read is a thin C helper that calls
// esmi_dimm_power_consumption_get and returns the power in milliwatts,
// or UINT32_MAX on any error.
static uint32_t esmi_dimm_power_read(uint32_t socket, uint8_t dimm_addr) {
    struct dimm_power dp = {0};
    esmi_status_t ret = esmi_dimm_power_consumption_get(socket, dimm_addr, &dp);
    if (ret != ESMI_SUCCESS) {
        return UINT32_MAX;
    }
    return dp.power; // milliwatts
}

// esmi_num_sockets returns the number of sockets or 0 on error.
static uint32_t esmi_num_sockets(void) {
    uint32_t n = 0;
    esmi_number_of_sockets_get(&n);
    return n;
}
*/
import "C"

import (
	"fmt"

	device "github.com/sustainable-computing-io/kepler/internal/device"
)

// Standard SPD DIMM addresses on the SMBus: 0x80 through 0x8b
// (up to 8 DIMMs per socket channel).
var dimmAddrs = []uint8{0x80, 0x81, 0x82, 0x83, 0x84, 0x85, 0x86, 0x87, 0x88, 0x89, 0x8a, 0x8b}

const dimmPowerFailure = uint32(0xFFFFFFFF)

// ── DimmReader ────────────────────────────────────────────────────────────────

// DimmReader enumerates all responding DIMM addresses across all sockets and
// returns one EsmiDimmZone per DIMM that responds to a power query.
type DimmReader struct{}

// NewDimmReader returns a DimmReader.
func NewDimmReader() *DimmReader { return &DimmReader{} }

// Zones initialises the e-smi library and probes all known DIMM SPD addresses
// on every socket. Returns one aggregated zone per socket that sums power from
// all responding DIMMs on that socket.
func (r *DimmReader) Zones() ([]device.EnergyZone, error) {
	if !esmiInit() {
		// Library or driver not available — return nil so the caller can
		// skip this reader without treating it as a hard error.
		return nil, nil
	}

	numSockets := int(C.esmi_num_sockets())
	if numSockets == 0 {
		return nil, nil
	}

	var zones []device.EnergyZone
	for socket := 0; socket < numSockets; socket++ {
		// Collect all responding DIMM addresses for this socket
		var dimmAddresses []uint8
		for _, addr := range dimmAddrs {
			pw := uint32(C.esmi_dimm_power_read(C.uint32_t(socket), C.uint8_t(addr)))
			if pw != dimmPowerFailure {
				dimmAddresses = append(dimmAddresses, addr)
			}
		}

		// Only create a zone if at least one DIMM responded
		if len(dimmAddresses) > 0 {
			zones = append(zones, &EsmiDimmZone{
				socketIdx:     socket,
				dimmAddresses: dimmAddresses,
			})
		}
	}
	return zones, nil
}

// ── EsmiDimmZone ──────────────────────────────────────────────────────────────

// EsmiDimmZone implements device.EnergyZone for aggregated DIMM power per socket.
// It sums power from all responding DIMMs on the socket.
// Only Power() is supported; Energy() returns an error.
type EsmiDimmZone struct {
	socketIdx     int
	dimmAddresses []uint8
}

func (z *EsmiDimmZone) Name() string {
	return fmt.Sprintf("dimm-%d", z.socketIdx)
}
func (z *EsmiDimmZone) Index() int { return z.socketIdx }
func (z *EsmiDimmZone) Path() string {
	return fmt.Sprintf("esmi://cpu/socket/%d/dimm", z.socketIdx)
}

// Energy is not supported for DIMM zones. The E-SMI library exposes only
// instantaneous power; integrate Power() readings over time to get energy.
func (z *EsmiDimmZone) Energy() (device.Energy, error) {
	return 0, fmt.Errorf("esmi/dimm: DIMM zones do not provide energy counters; integrate Power() readings over time")
}

// MaxEnergy returns 0 (not applicable).
func (z *EsmiDimmZone) MaxEnergy() device.Energy { return 0 }

// Power returns the aggregated instantaneous DIMM power for all DIMMs on this
// socket in watts. The E-SMI library returns milliwatts per DIMM.
func (z *EsmiDimmZone) Power() (device.Power, error) {
	var totalPowerMw uint64
	var readErrors int

	for _, addr := range z.dimmAddresses {
		pw := uint32(C.esmi_dimm_power_read(C.uint32_t(z.socketIdx), C.uint8_t(addr)))
		if pw == dimmPowerFailure {
			readErrors++
			continue
		}
		totalPowerMw += uint64(pw)
	}

	// Return error only if ALL DIMMs failed to read
	if readErrors == len(z.dimmAddresses) {
		return 0, fmt.Errorf("esmi/dimm: all DIMM reads failed for socket %d", z.socketIdx)
	}

	return device.Power(float64(totalPowerMw) / 1000.0), nil
}

// ── backend registration ──────────────────────────────────────────────────────

// dimmBackend returns a backend candidate for DIMM power readings.
// It is included in availableReaders() alongside the goamdsmi backend.
func dimmBackend() backendCandidate {
	return backendCandidate{name: "dimm", reader: NewDimmReader()}
}
