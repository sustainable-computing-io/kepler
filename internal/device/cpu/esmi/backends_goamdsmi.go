// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

//go:build goamdsmi

package esmi

// availableReaders returns all backend candidates when the goamdsmi build tag
// is active.
//
// Primary backends (mutually exclusive — first working one wins):
//  1. goamdsmi — socket + core energy counters, instantaneous socket power
//  2. sysfs    — pure-Go socket + core energy counters via amd_energy driver
//
// Additive backends (always attempted, zones merged alongside primary):
//   - dimm — per-DIMM instantaneous power via e-smi C library
func availableReaders(sysfsPath string) []backendCandidate {
	return []backendCandidate{
		goamdsmiBackend(),
		{name: "dimm", reader: NewDimmReader(), additive: true},
		sysfsBackend(sysfsPath),
	}
}
