// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !goamdsmi

package esmi

// availableReaders returns the ordered list of backend candidates when the
// goamdsmi build tag is NOT active (default / CI build).
//
// Only the sysfs backend is available in this configuration.
// To also enable the goamdsmi backend, build with: -tags goamdsmi
func availableReaders(sysfsPath string) []backendCandidate {
	return []backendCandidate{
		sysfsBackend(sysfsPath),
	}
}
