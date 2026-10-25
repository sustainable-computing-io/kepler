// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package device

import "github.com/sustainable-computing-io/kepler/internal/service"

// PowerMeter is a hardware backend that reads energy or power from a class
// of hardware (CPU package, GPU device, etc).
//
// Many PowerMeters can be selected. Each contributes its own readings.
// Domain-specific methods live on subinterfaces that embed PowerMeter.
type PowerMeter interface {
	service.Service     // Name()
	service.Initializer // Init()
}
