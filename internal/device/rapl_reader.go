// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package device

// raplReader is an internal abstraction for different RAPL reading backends
// (powercap sysfs and MSR). This interface allows the raplPowerMeter to work
// with different RAPL reading mechanisms while maintaining a consistent API.
type raplReader interface {
	// Zones returns the list of energy zones available from this power reader
	Zones() ([]EnergyZone, error)

	// Available checks if the power reader can be used on the current system
	Available() bool

	// Init initializes the power reader and verifies it can read energy values
	Init() error

	// Close releases any resources held by the power reader
	Close() error

	// Name returns a human-readable name for the power reader implementation
	Name() string
}
