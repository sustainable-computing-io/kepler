// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package gpu

import (
	"log/slog"
	"sync"
)

// Factory is a function that creates a GPUPowerMeter for a specific vendor.
// It receives a logger and returns a meter or an error if the vendor's
// hardware/drivers are not available.
type Factory func(logger *slog.Logger) (GPUPowerMeter, error)

var (
	registry   = make(map[Vendor]Factory)
	registryMu sync.RWMutex
)

// Register adds a GPU backend factory for the given vendor.
func Register(vendor Vendor, factory Factory) {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry[vendor] = factory
}

// DiscoverAll probes all registered GPU backends and returns meters for
// vendors with available hardware. Backends that fail to initialize or
// have no devices are silently skipped.
//
// Returns an empty slice if no GPUs are found.
func DiscoverAll(logger *slog.Logger) []GPUPowerMeter {
	vendors := RegisteredVendors()

	var meters []GPUPowerMeter
	for _, vendor := range vendors {
		if meter := Discover(vendor, logger); meter != nil {
			meters = append(meters, meter)
		}
	}

	return meters
}

// Discover returns a GPUPowerMeter for a specific vendor, or nil if
// the vendor is not registered or has no available hardware.
func Discover(vendor Vendor, logger *slog.Logger) GPUPowerMeter {
	registryMu.RLock()
	factory, ok := registry[vendor]
	registryMu.RUnlock()

	if !ok {
		logger.Debug("GPU vendor not registered", "vendor", vendor)
		return nil
	}

	meter, err := factory(logger)
	if err != nil {
		logger.Debug("GPU vendor factory failed",
			"vendor", vendor,
			"error", err)
		return nil
	}

	if err := meter.Init(); err != nil {
		logger.Debug("GPU vendor init failed",
			"vendor", vendor,
			"error", err)
		return nil
	}

	if len(meter.Devices()) == 0 {
		logger.Debug("GPU vendor has no devices", "vendor", vendor)
		_ = meter.Shutdown()
		return nil
	}

	return meter
}

// RegisteredVendors returns a list of all registered GPU vendors.
// Useful for debugging and logging available backends.
func RegisteredVendors() []Vendor {
	registryMu.RLock()
	defer registryMu.RUnlock()

	vendors := make([]Vendor, 0, len(registry))
	for vendor := range registry {
		vendors = append(vendors, vendor)
	}
	return vendors
}

// ClearRegistry removes all registered vendors.
// This is primarily useful for testing.
func ClearRegistry() {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry = make(map[Vendor]Factory)
}
