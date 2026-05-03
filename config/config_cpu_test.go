// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/utils/ptr"
)

func TestCpuMetersDefault(t *testing.T) {
	cfg := DefaultConfig()
	assert.Equal(t, []string{"rapl", "hwmon"}, cfg.Cpu.Meters,
		"default cpu.meters should preserve the prior RAPL→hwmon fallback chain")
}

func TestLoadCpuMetersFromYAML(t *testing.T) {
	tt := []struct {
		name     string
		yamlData string
		want     []string
	}{
		{
			"explicit hwmon-first",
			`
cpu:
  meters: ["hwmon", "rapl"]
`,
			[]string{"hwmon", "rapl"},
		},
		{
			"single backend",
			`
cpu:
  meters: ["fake"]
`,
			[]string{"fake"},
		},
		{
			"empty list",
			`
cpu:
  meters: []
`,
			[]string{},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := Load(strings.NewReader(tc.yamlData))
			require.NoError(t, err)
			assert.Equal(t, tc.want, cfg.Cpu.Meters)
		})
	}
}

func TestApplyCpuMeterDeprecations(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*Config)
		want  []string
	}{
		{
			"fake-cpu-meter overrides cpu.meters",
			func(c *Config) {
				c.Dev.FakeCpuMeter.Enabled = ptr.To(true)
			},
			[]string{"fake"},
		},
		{
			"hwmon forceEnabled overrides cpu.meters",
			func(c *Config) {
				c.Experimental = &Experimental{}
				c.Experimental.Hwmon.ForceEnabled = ptr.To(true)
			},
			[]string{"hwmon"},
		},
		{
			"fake wins when both legacy keys are set",
			func(c *Config) {
				c.Dev.FakeCpuMeter.Enabled = ptr.To(true)
				c.Experimental = &Experimental{}
				c.Experimental.Hwmon.ForceEnabled = ptr.To(true)
			},
			[]string{"fake"},
		},
		{
			"no legacy: explicit cpu.meters preserved",
			func(c *Config) {
				c.Cpu.Meters = []string{"rapl"}
			},
			[]string{"rapl"},
		},
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := DefaultConfig()
			tc.setup(cfg)
			cfg.ApplyCpuMeterDeprecations(logger)
			assert.Equal(t, tc.want, cfg.Cpu.Meters)
		})
	}
}
