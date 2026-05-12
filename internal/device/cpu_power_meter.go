// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package device

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/sustainable-computing-io/kepler/config"
)

// EnergyZone represents a measurable energy or power zone/domain exposed by a power meter.
// An EnergyZone typically represents a logical zone of the hardware unit, e.g. cpu core, cpu package
// dram, uncore etc.
// Reference: https://firefox-source-docs.mozilla.org/performance/power_profiling_overview.html
type EnergyZone interface {
	// Name() returns the zone name
	Name() string

	// Index() returns the index of the zone
	Index() int

	// Path() returns the path from which the energy usage value ie being read
	Path() string

	// Energy() returns energy consumed by the zone.
	Energy() (Energy, error)

	// MaxEnergy returns  the maximum value of energy usage that can be read.
	// When energy usage reaches this value, the energy value returned by Energy()
	// will wrap around and start again from zero.
	MaxEnergy() Energy

	// Power() returns the current power consumption by the zone.
	// This method is used for zones that provide instantaneous power readings.
	Power() (Power, error)
}

// CPUPowerMeter is the interface for CPU power measurement.
// It embeds PowerMeter and adds CPU-specific methods.
type CPUPowerMeter interface {
	PowerMeter

	// Zones() returns a slice of the energy measurement zones
	Zones() ([]EnergyZone, error)

	// PrimaryEnergyZone() returns the zone with the highest energy coverage/priority
	// This zone represents the most comprehensive energy measurement available
	// E.g. Psys > Package > Core > DRAM > Uncore
	PrimaryEnergyZone() (EnergyZone, error)
}

// CreateCPUMeter walks cfg.Cpu.PreferredMeters in preference order, builds
// each backend, runs Init(), and returns the first meter that reports zones.
//
// Failure modes per backend (all aggregated into the returned error):
//   - factory error or Init() error: real failure.
//   - empty zones: backend ran but found nothing usable on this host.
//
// Returns the joined error only if no backend produced a usable meter.
// CPU is mandatory; callers treat any returned error as fatal.
func CreateCPUMeter(logger *slog.Logger, cfg *config.Config) (CPUPowerMeter, error) {
	if len(cfg.Cpu.PreferredMeters) == 0 {
		return nil, errors.New("cpu.preferredMeters is empty")
	}
	var errs []error
	for _, name := range cfg.Cpu.PreferredMeters {
		meter, err := buildCPUMeter(name, logger, cfg)
		if err != nil {
			logger.Warn("cpu meter not available, trying next backend", "meter", name, "error", err)
			errs = append(errs, fmt.Errorf("cpu meter %q: %w", name, err))
			continue
		}
		if err := meter.Init(); err != nil {
			logger.Warn("cpu meter init failed, trying next backend", "meter", name, "error", err)
			errs = append(errs, fmt.Errorf("cpu meter %q: init: %w", name, err))
			continue
		}
		zones, _ := meter.Zones()
		if len(zones) == 0 {
			logger.Info("cpu meter reports no zones, trying next backend", "meter", name)
			errs = append(errs, fmt.Errorf("cpu meter %q: reported no zones", name))
			continue
		}
		logger.Info("using cpu power meter", "meter", name)
		return meter, nil
	}

	return nil, errors.Join(errs...)
}

// buildCPUMeter dispatches to the constructor for a named CPU backend.
// Add a new backend by adding a case here and a constructor in its source file.
func buildCPUMeter(name string, logger *slog.Logger, cfg *config.Config) (CPUPowerMeter, error) {
	switch name {
	case "rapl":
		if len(cfg.Rapl.Zones) > 0 {
			logger.Info("rapl zones are filtered", "zones-enabled", cfg.Rapl.Zones)
		}
		return NewCPUPowerMeter(
			cfg.Host.SysFS,
			WithRaplLogger(logger),
			WithZoneFilter(cfg.Rapl.Zones),
		)

	case "hwmon":
		var zones []string
		var rules []ConfigChipRule
		if cfg.Experimental != nil {
			h := cfg.Experimental.Hwmon
			zones = h.Zones
			for _, cr := range h.ChipRules {
				rules = append(rules, ConfigChipRule{
					Name:         cr.Name,
					Pairings:     cr.Pairings,
					SkipVoltages: cr.SkipVoltages,
					SkipCurrents: cr.SkipCurrents,
					UseSameIndex: cr.UseSameIndex,
				})
			}
		}
		if len(zones) > 0 {
			logger.Info("hwmon zones are filtered", "zones-enabled", zones)
		}
		if len(rules) > 0 {
			logger.Info("hwmon chip rules configured", "count", len(rules))
		}
		return NewHwmonPowerMeter(
			cfg.Host.SysFS,
			WithHwmonLogger(logger),
			WithHwmonZoneFilter(zones),
			WithHwmonChipRules(rules),
		)

	case "fake":
		return NewFakeCPUMeter(cfg.Dev.FakeCpuMeter.Zones, WithFakeLogger(logger))

	default:
		return nil, fmt.Errorf("unknown cpu meter %q", name)
	}
}
