// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"maps"
	"slices"
)

// Clone returns a fully independent configuration. Callers must not mutate c
// concurrently with Clone.
func (c *Config) Clone() *Config {
	if c == nil {
		return nil
	}
	clone := *c
	clone.unknownFields = slices.Clone(c.unknownFields)
	clone.Cpu.PreferredMeters = slices.Clone(c.Cpu.PreferredMeters)
	clone.Rapl.Zones = slices.Clone(c.Rapl.Zones)
	clone.Web.ListenAddresses = slices.Clone(c.Web.ListenAddresses)
	clone.Exporter.Prometheus.DebugCollectors = slices.Clone(c.Exporter.Prometheus.DebugCollectors)
	clone.Exporter.Prometheus.Enabled = cloneBool(c.Exporter.Prometheus.Enabled)
	clone.Exporter.Stdout.Enabled = cloneBool(c.Exporter.Stdout.Enabled)
	clone.Debug.Pprof.Enabled = cloneBool(c.Debug.Pprof.Enabled)
	clone.Kube.Enabled = cloneBool(c.Kube.Enabled)
	clone.Dev.FakeCpuMeter.Enabled = cloneBool(c.Dev.FakeCpuMeter.Enabled)
	clone.Dev.FakeCpuMeter.Zones = slices.Clone(c.Dev.FakeCpuMeter.Zones)
	clone.Dev.FakeGPUMeter.Enabled = cloneBool(c.Dev.FakeGPUMeter.Enabled)
	if c.Experimental != nil {
		exp := *c.Experimental
		exp.Platform.Redfish.Enabled = cloneBool(exp.Platform.Redfish.Enabled)
		exp.GPU.Enabled = cloneBool(exp.GPU.Enabled)
		exp.Hwmon.ForceEnabled = cloneBool(exp.Hwmon.ForceEnabled)
		exp.Hwmon.Zones = slices.Clone(exp.Hwmon.Zones)
		exp.Hwmon.ChipRules = slices.Clone(exp.Hwmon.ChipRules)
		for i := range exp.Hwmon.ChipRules {
			rule := &exp.Hwmon.ChipRules[i]
			rule.Pairings = maps.Clone(rule.Pairings)
			rule.SkipVoltages = slices.Clone(rule.SkipVoltages)
			rule.SkipCurrents = slices.Clone(rule.SkipCurrents)
		}
		clone.Experimental = &exp
	}
	return &clone
}

func cloneBool(value *bool) *bool {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

// ValidateRuntime validates library configuration without requiring an HTTP
// listener or web configuration file. Host paths are checked at construction.
func (c *Config) ValidateRuntime() error {
	return c.Validate(SkipWebValidation)
}
