// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package device

// powerMeter is a generic interface for power meters which reads energy
// or power readings from hardware devices like CPU/GPU/DRAM etc
type powerMeter interface {
	// Name() returns a string identifying the power meter
	Name() string
}
