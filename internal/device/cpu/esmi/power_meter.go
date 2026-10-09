// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

// Package esmi implements a CPUPowerMeter for AMD processors.
//
// Two primary backends are supported and selected automatically at runtime:
//
//  1. goamdsmi (priority 1) — Go bindings over the AMD e-smi / HSMP C library
//     (github.com/ROCm/amdsmi). Provides socket and core energy counters plus
//     instantaneous socket power. Only compiled with the "goamdsmi" build tag.
//
//  2. sysfs amd_energy (priority 2) — pure-Go, zero CGo. Reads hwmon counters
//     from the amd_energy kernel module (CONFIG_AMD_ENERGY, Linux ≥ 5.8).
//     Provides socket and core energy counters only.
//
// One additive backend is always attempted independently:
//
//   - dimm — per-DIMM instantaneous power via the e-smi C library
//     (esmi_get_dimm_power). Compiled only with the "goamdsmi" build tag.
//     DIMM zones expose Power() only; Energy() returns an error.
//
// NewCPUPowerMeter selects the first primary backend that works, then merges
// in any DIMM zones from the dimm backend (if available). Zone types:
//
//	package – highest priority for PrimaryEnergyZone(); normalized from AMD
//	          "socket" to match RAPL naming; supports Power() via goamdsmi backend
//	core    – per-logical-thread energy
//	dimm    – per-DIMM power (Power() only, no Energy() counter)
package esmi

import (
	"fmt"
	"log/slog"
	"strings"

	device "github.com/sustainable-computing-io/kepler/internal/device"
)

// PowerMeter implements device.CPUPowerMeter for AMD CPUs.
type PowerMeter struct {
	reader      Reader
	cachedZones []device.EnergyZone
	logger      *slog.Logger
	zoneFilter  []string
	topZone     device.EnergyZone
	backendName string
}

// OptionFn is a functional option for PowerMeter.
type OptionFn func(*PowerMeter)

// WithReader forces a specific Reader backend, bypassing auto-detection.
// Primarily useful in tests.
func WithReader(r Reader) OptionFn {
	return func(pm *PowerMeter) { pm.reader = r }
}

// WithLogger sets a structured logger on the meter.
func WithLogger(logger *slog.Logger) OptionFn {
	return func(pm *PowerMeter) { pm.logger = logger.With("service", "esmi") }
}

// WithZoneFilter restricts monitoring to the named zone types.
// Valid names: "package", "core", "dimm", "l3cache". Empty means all zones are included.
// Note: "package" is the normalized name for AMD "socket" zones to align with RAPL.
func WithZoneFilter(zones []string) OptionFn {
	return func(pm *PowerMeter) { pm.zoneFilter = zones }
}

// NewCPUPowerMeter constructs a PowerMeter, auto-detecting the best available
// backends. sysfsPath is the sysfs root (normally "/sys").
func NewCPUPowerMeter(sysfsPath string, opts ...OptionFn) (*PowerMeter, error) {
	pm := &PowerMeter{
		logger:     slog.Default().With("service", "esmi"),
		zoneFilter: []string{},
	}
	for _, opt := range opts {
		opt(pm)
	}

	// WithReader injected a backend (e.g. in tests) — use it directly.
	if pm.reader != nil {
		pm.backendName = "injected"
		return pm, nil
	}

	candidates := availableReaders(sysfsPath)

	// Split candidates into primary (energy-capable) and additive (power-only).
	var primaryCandidates, additiveCandidates []backendCandidate
	for _, c := range candidates {
		if c.additive {
			additiveCandidates = append(additiveCandidates, c)
		} else {
			primaryCandidates = append(primaryCandidates, c)
		}
	}

	// Select the first working primary backend.
	var primaryReader Reader
	for _, candidate := range primaryCandidates {
		zones, err := candidate.reader.Zones()
		if err != nil {
			pm.logger.Debug("esmi primary backend failed",
				"backend", candidate.name, "err", err)
			continue
		}
		if len(zones) == 0 {
			pm.logger.Debug("esmi primary backend found no zones",
				"backend", candidate.name)
			continue
		}
		// Probe the first zone for either Energy() or Power() support
		_, errEnergy := zones[0].Energy()
		_, errPower := zones[0].Power()
		if errEnergy != nil && errPower != nil {
			pm.logger.Debug("esmi primary backend probe failed",
				"backend", candidate.name,
				"energy_err", errEnergy,
				"power_err", errPower)
			continue
		}
		primaryReader = candidate.reader
		pm.backendName = candidate.name
		pm.logger.Info("esmi primary backend selected", "backend", candidate.name)
		break
	}

	if primaryReader == nil {
		return nil, fmt.Errorf(
			"esmi: no working primary backend found; " +
				"load the amd_energy kernel module or install libamd_smi.so " +
				"and rebuild with -tags goamdsmi",
		)
	}

	// Probe additive backends (e.g. DIMM) and merge their zones.
	// These are probed via Power() rather than Energy().
	var additiveReaders []Reader
	for _, candidate := range additiveCandidates {
		zones, err := candidate.reader.Zones()
		if err != nil || len(zones) == 0 {
			pm.logger.Debug("esmi additive backend unavailable",
				"backend", candidate.name, "err", err)
			continue
		}
		if _, err := zones[0].Power(); err != nil {
			pm.logger.Debug("esmi additive backend probe failed",
				"backend", candidate.name, "err", err)
			continue
		}
		additiveReaders = append(additiveReaders, candidate.reader)
		pm.logger.Info("esmi additive backend available", "backend", candidate.name)
	}

	if len(additiveReaders) == 0 {
		pm.reader = primaryReader
	} else {
		pm.reader = &mergedReader{
			primary:  primaryReader,
			additive: additiveReaders,
		}
	}

	return pm, nil
}

// backendCandidate pairs a display name with a Reader for the detection loop.
type backendCandidate struct {
	name   string
	reader Reader
	// additive marks backends whose zones supplement (rather than replace) the
	// primary backend. Additive zones are probed via Power(), not Energy().
	additive bool
}

// mergedReader combines a primary Reader with one or more additive Readers.
type mergedReader struct {
	primary  Reader
	additive []Reader
}

func (m *mergedReader) Zones() ([]device.EnergyZone, error) {
	zones, err := m.primary.Zones()
	if err != nil {
		return nil, err
	}
	for _, r := range m.additive {
		extra, err := r.Zones()
		if err != nil {
			continue // additive failures are non-fatal
		}
		zones = append(zones, extra...)
	}
	return zones, nil
}

// Reader enumerates ESMI energy zones.
type Reader interface {
	Zones() ([]device.EnergyZone, error)
}

// ── device.CPUPowerMeter interface ───────────────────────────────────────────

// Name identifies the meter and the active backend.
func (pm *PowerMeter) Name() string { return "esmi/" + pm.backendName }

// Init validates that the active backend can enumerate and read at least one zone.
func (pm *PowerMeter) Init() error {
	zones, err := pm.reader.Zones()
	if err != nil {
		return fmt.Errorf("esmi: failed to enumerate zones: %w", err)
	}
	if len(zones) == 0 {
		return fmt.Errorf("esmi: no energy zones found (backend: %s)", pm.backendName)
	}
	// Find the first energy-capable or power-capable zone to probe.
	for _, z := range zones {
		// Try Energy() first (preferred)
		if _, err = z.Energy(); err == nil {
			return nil
		}
		// Fall back to Power() for power-only zones (e.g., HSMP)
		if _, err = z.Power(); err == nil {
			return nil
		}
	}
	return fmt.Errorf("esmi: no readable energy or power zone found (backend: %s)", pm.backendName)
}

// Zones returns the filtered and aggregated list of energy zones.
// Results are cached after the first successful call.
func (pm *PowerMeter) Zones() ([]device.EnergyZone, error) {
	if len(pm.cachedZones) != 0 {
		return pm.cachedZones, nil
	}

	zones, err := pm.reader.Zones()
	if err != nil {
		return nil, err
	}
	if len(zones) == 0 {
		return nil, fmt.Errorf("esmi: no energy zones found")
	}

	zones = pm.filterZones(zones)
	if len(zones) == 0 {
		return nil, fmt.Errorf("esmi: no energy zones remaining after filtering")
	}

	pm.cachedZones = pm.groupZonesByName(zones)
	return pm.cachedZones, nil
}

// PrimaryEnergyZone returns the highest-priority energy-capable zone.
// Priority: package > core > l3cache > first available.
// Zone names are normalized to match RAPL conventions (socket → package).
// DIMM zones are excluded since they have no energy counter.
func (pm *PowerMeter) PrimaryEnergyZone() (device.EnergyZone, error) {
	if pm.topZone != nil {
		return pm.topZone, nil
	}

	zones, err := pm.Zones()
	if err != nil {
		return nil, err
	}
	if len(zones) == 0 {
		return nil, fmt.Errorf("esmi: no energy zones available")
	}

	zoneMap := make(map[string]device.EnergyZone, len(zones))
	for _, z := range zones {
		// Only index energy-capable zones for primary selection.
		if _, err := z.Energy(); err == nil {
			zoneMap[strings.ToLower(z.Name())] = z
		}
	}

	// Priority order aligned with RAPL: package > core > l3cache
	for _, name := range []string{"package", "core", "l3cache"} {
		if z, ok := zoneMap[name]; ok {
			pm.topZone = z
			return z, nil
		}
	}

	// Fallback: first energy-capable zone.
	for _, z := range zones {
		if _, err := z.Energy(); err == nil {
			pm.topZone = z
			return z, nil
		}
	}

	// Last resort: accept power-only zones (e.g., HSMP)
	// Power can be integrated over time to derive energy
	for _, z := range zones {
		if _, err := z.Power(); err == nil {
			pm.logger.Info("using power-only zone as primary",
				"zone", z.Name(),
				"note", "energy will be derived from power readings")
			pm.topZone = z
			return z, nil
		}
	}

	return nil, fmt.Errorf("esmi: no energy-capable or power-capable zone found")
}

// ── internal helpers ──────────────────────────────────────────────────────────

func (pm *PowerMeter) needsFiltering() bool { return len(pm.zoneFilter) != 0 }

func (pm *PowerMeter) filterZones(zones []device.EnergyZone) []device.EnergyZone {
	if !pm.needsFiltering() {
		return zones
	}
	wanted := make(map[string]bool, len(pm.zoneFilter))
	for _, name := range pm.zoneFilter {
		wanted[strings.ToLower(name)] = true
	}
	var included, excluded []string
	filtered := make([]device.EnergyZone, 0, len(zones))
	for _, z := range zones {
		if wanted[strings.ToLower(z.Name())] {
			filtered = append(filtered, z)
			included = append(included, z.Name())
		} else {
			excluded = append(excluded, z.Name())
		}
	}
	pm.logger.Debug("filtered ESMI zones", "included", included, "excluded", excluded)
	return filtered
}

func (pm *PowerMeter) groupZonesByName(zones []device.EnergyZone) []device.EnergyZone {
	groups := make(map[string][]device.EnergyZone)
	for _, z := range zones {
		groups[z.Name()] = append(groups[z.Name()], z)
	}
	result := make([]device.EnergyZone, 0, len(zones))

	for name, grp := range groups {
		if len(grp) == 1 {
			// Single zone: add as-is (matches RAPL behavior for unique names like package-0, package-1)
			result = append(result, grp[0])
			continue
		}

		// Multiple zones with same name: create aggregated zone
		// This handles cases where multiple sensors share the same name
		agg, err := device.NewAggregatedZone(grp)
		if err != nil {
			pm.logger.Warn("failed to create aggregated zone",
				"name", name, "err", err)
			continue
		}
		result = append(result, agg)
	}
	return result
}

func zoneNames(zones []device.EnergyZone) []string {
	names := make([]string, len(zones))
	for i, z := range zones {
		names[i] = fmt.Sprintf("%s-%d", z.Name(), z.Index())
	}
	return names
}
