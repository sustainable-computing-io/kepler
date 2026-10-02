// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

//go:build goamdsmi

package esmi

// This file is only compiled when built with -tags goamdsmi.
// It requires:
//   - E-SMI C library (libe_smi64.so) installed (typically /opt/e-sms/e_smi/)
//   - amd_hsmp kernel driver loaded
//
// Zone types provided by this backend:
//   - socket : accumulated energy (µJ) + instantaneous power (W) per socket
//   - core   : accumulated energy (µJ) per logical thread/core

/*
#cgo CFLAGS: -I/opt/e-sms/e_smi/include
#cgo LDFLAGS: -L/opt/e-sms/e_smi/lib -le_smi64

#include <stdint.h>
#include <e_smi/e_smi.h>

// Wrapper functions to safely call E-SMI library functions
static uint32_t esmi_num_sockets(void) {
    uint32_t n = 0;
    esmi_number_of_sockets_get(&n);
    return n;
}

static uint32_t esmi_num_cpus(void) {
    uint32_t n = 0;
    esmi_number_of_cpus_get(&n);
    return n;
}

static uint32_t esmi_threads_per_core_get_safe(void) {
    uint32_t n = 0;
    esmi_threads_per_core_get(&n);
    return n;
}

// Returns 0 on success, non-zero on failure
static int esmi_socket_energy_read(uint32_t socket_idx, uint64_t *energy) {
    return (int)esmi_socket_energy_get(socket_idx, energy);
}

// Returns 0 on success, non-zero on failure
static int esmi_socket_power_read(uint32_t socket_idx, uint32_t *power) {
    return (int)esmi_socket_power_get(socket_idx, power);
}

// Returns 0 on success, non-zero on failure
static int esmi_core_energy_read(uint32_t core_ind, uint64_t *energy) {
    return (int)esmi_core_energy_get(core_ind, energy);
}
*/
import "C"

import (
	"fmt"

	device "github.com/sustainable-computing-io/kepler/internal/device"
)

// ── GoamdsmiReader ────────────────────────────────────────────────────────────

// GoamdsmiReader implements Reader using the AMD E-SMI C library.
// It returns one "socket" zone per physical CPU socket and one "core" zone
// per logical thread exposed by the library.
type GoamdsmiReader struct{}

// NewGoamdsmiReader returns a GoamdsmiReader.
func NewGoamdsmiReader() *GoamdsmiReader { return &GoamdsmiReader{} }

// Zones initialises the E-SMI library and returns all available
// energy zones. Returns nil (no error) when the library cannot initialise so
// NewCPUPowerMeter falls through to the next backend.
func (r *GoamdsmiReader) Zones() ([]device.EnergyZone, error) {
	if !esmiInit() {
		return nil, nil
	}

	var zones []device.EnergyZone

	// ── socket zones ─────────────────────────────────────────────────────────
	numSockets := int(C.esmi_num_sockets())
	for i := 0; i < numSockets; i++ {
		var energy C.uint64_t
		if C.esmi_socket_energy_read(C.uint32_t(i), &energy) != 0 {
			continue
		}
		zones = append(zones, &GoamdsmiPackageZone{socketIdx: i})
	}

	// ── core zones ────────────────────────────────────────────────────────────
	// The library indexes cores by CPU (logical thread index).
	numCPUs := int(C.esmi_num_cpus())
	threadsPerCore := int(C.esmi_threads_per_core_get_safe())
	if threadsPerCore < 1 {
		threadsPerCore = 1
	}
	for i := 0; i < numCPUs; i++ {
		var energy C.uint64_t
		if C.esmi_core_energy_read(C.uint32_t(i), &energy) != 0 {
			continue
		}
		zones = append(zones, &GoamdsmiCoreZone{
			threadIdx: i,
			coreIdx:   i / threadsPerCore,
		})
	}

	return zones, nil
}

// ── GoamdsmiPackageZone ────────────────────────────────────────────────────────

// GoamdsmiPackageZone implements device.EnergyZone for one AMD CPU socket.
// Uniquely among ESMI zones, it also supports Power() for instantaneous watts.
type GoamdsmiPackageZone struct {
	socketIdx int
}

func (z *GoamdsmiPackageZone) Name() string {
	return fmt.Sprintf("package-%d", z.socketIdx)
}
func (z *GoamdsmiPackageZone) Index() int { return z.socketIdx }
func (z *GoamdsmiPackageZone) Path() string {
	return fmt.Sprintf("esmi://cpu/socket/%d", z.socketIdx)
}

// Energy returns accumulated socket energy in microjoules.
func (z *GoamdsmiPackageZone) Energy() (device.Energy, error) {
	var energy C.uint64_t
	if C.esmi_socket_energy_read(C.uint32_t(z.socketIdx), &energy) != 0 {
		return 0, fmt.Errorf("esmi: esmi_socket_energy_get failed for socket %d", z.socketIdx)
	}
	return device.Energy(energy), nil
}

// MaxEnergy returns 0 — the library does not expose a counter maximum.
func (z *GoamdsmiPackageZone) MaxEnergy() device.Energy { return 0 }

// Power returns instantaneous socket power in watts.
// esmi_socket_power_get returns milliwatts.
func (z *GoamdsmiPackageZone) Power() (device.Power, error) {
	var power C.uint32_t
	if C.esmi_socket_power_read(C.uint32_t(z.socketIdx), &power) != 0 {
		return 0, fmt.Errorf("esmi: esmi_socket_power_get failed for socket %d", z.socketIdx)
	}
	return device.Power(float64(power) / 1000.0), nil
}

// ── GoamdsmiCoreZone ──────────────────────────────────────────────────────────

// GoamdsmiCoreZone implements device.EnergyZone for a single logical core
// (SMT thread). Energy is cumulative µJ; instantaneous power is not available
// at per-core granularity via this library.
type GoamdsmiCoreZone struct {
	threadIdx int // library index (SMT thread)
	coreIdx   int // physical core index (threadIdx / threadsPerCore)
}

func (z *GoamdsmiCoreZone) Name() string {
	return fmt.Sprintf("core-%d", z.threadIdx)
}
func (z *GoamdsmiCoreZone) Index() int { return z.threadIdx }
func (z *GoamdsmiCoreZone) Path() string {
	return fmt.Sprintf("esmi://cpu/core/%d", z.threadIdx)
}

// Energy returns accumulated core energy in microjoules.
func (z *GoamdsmiCoreZone) Energy() (device.Energy, error) {
	var energy C.uint64_t
	if C.esmi_core_energy_read(C.uint32_t(z.threadIdx), &energy) != 0 {
		return 0, fmt.Errorf("esmi: esmi_core_energy_get failed for thread %d", z.threadIdx)
	}
	return device.Energy(energy), nil
}

// MaxEnergy returns 0 — no counter maximum is exposed.
func (z *GoamdsmiCoreZone) MaxEnergy() device.Energy { return 0 }

// Power is not available at per-core granularity via E-SMI.
// Use socket-level Power() or derive from successive Energy() readings.
func (z *GoamdsmiCoreZone) Power() (device.Power, error) {
	return 0, fmt.Errorf("esmi: per-core instantaneous power is not available; derive from successive Energy() calls")
}

// ── backend registration ──────────────────────────────────────────────────────

func goamdsmiBackend() backendCandidate {
	return backendCandidate{name: "goamdsmi", reader: NewGoamdsmiReader()}
}
