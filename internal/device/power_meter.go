// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package device

import "context"

// powerMeter is a generic interface for power meters which reads energy
// or power readings from hardware devices like CPU/GPU/DRAM etc
type powerMeter interface {
	// Name() returns a string identifying the power meter
	Name() string

	// Init() initializes the power meter and makes it ready for use. This method
	// is not required to be thread-safe
	Init(ctx context.Context) error

	// Run() power meter for reading energy or power
	Run(ctx context.Context) error

	// Stop() stops the power meter and releases any resources held
	Stop() error
}
