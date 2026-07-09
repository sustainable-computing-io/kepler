// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/utils/ptr"
)

// TestCpuPreferredMeters covers cfg.Cpu.PreferredMeters defaults, deprecation
// translation, and precedence. Each case sets up a Config and asserts the
// resulting PreferredMeters after ApplyCpuMeterDeprecations runs. Log output
// is discarded; behaviour is verified solely via cfg.Cpu.PreferredMeters.
func TestCpuPreferredMeters(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*Config)
		want  []string
	}{
		{
			name:  "default: rapl then hwmon",
			setup: func(*Config) {},
			want:  []string{"rapl", "hwmon"},
		},
		{
			name: "fake-cpu-meter overrides cpu.preferredMeters",
			setup: func(c *Config) {
				c.Dev.FakeCpuMeter.Enabled = ptr.To(true)
			},
			want: []string{"fake"},
		},
		{
			name: "hwmon forceEnabled overrides cpu.preferredMeters",
			setup: func(c *Config) {
				c.Experimental = &Experimental{}
				c.Experimental.Hwmon.ForceEnabled = ptr.To(true)
			},
			want: []string{"hwmon"},
		},
		{
			name: "fake wins when both legacy keys are set",
			setup: func(c *Config) {
				c.Dev.FakeCpuMeter.Enabled = ptr.To(true)
				c.Experimental = &Experimental{}
				c.Experimental.Hwmon.ForceEnabled = ptr.To(true)
			},
			want: []string{"fake"},
		},
		{
			name: "no legacy: explicit cpu.preferredMeters preserved",
			setup: func(c *Config) {
				c.Cpu.PreferredMeters = []string{"rapl"}
			},
			want: []string{"rapl"},
		},
	}

	logger := slog.New(slog.DiscardHandler)

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := DefaultConfig()
			tc.setup(cfg)
			cfg.ApplyCpuMeterDeprecations(logger)
			assert.Equal(t, tc.want, cfg.Cpu.PreferredMeters)
		})
	}
}

func TestCpuPreferredMetersValidation(t *testing.T) {
	tests := []struct {
		name    string
		meters  []string
		wantErr string
	}{
		{
			name:   "all known backends",
			meters: []string{"rapl", "hwmon", "fake"},
		},
		{
			name:    "unknown backend",
			meters:  []string{"rappl"},
			wantErr: `invalid cpu.preferredMeters entry "rappl"`,
		},
		{
			name:    "typo before valid backend still rejected",
			meters:  []string{"rappl", "hwmon"},
			wantErr: `invalid cpu.preferredMeters entry "rappl"`,
		},
		{
			name:   "empty list passes Validate (CreateCPUMeter handles emptiness)",
			meters: []string{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.Cpu.PreferredMeters = tc.meters
			err := cfg.Validate(SkipHostValidation)
			if tc.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

func TestLoadCpuPreferredMetersFromYAML(t *testing.T) {
	tests := []struct {
		name     string
		yamlData string
		want     []string
	}{
		{
			name:     "explicit hwmon-first",
			yamlData: "cpu:\n  preferredMeters: [hwmon, rapl]\n",
			want:     []string{"hwmon", "rapl"},
		},
		{
			name:     "single backend",
			yamlData: "cpu:\n  preferredMeters: [fake]\n",
			want:     []string{"fake"},
		},
		{
			name:     "empty list",
			yamlData: "cpu:\n  preferredMeters: []\n",
			want:     []string{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := Load(strings.NewReader(tc.yamlData))
			require.NoError(t, err)
			assert.Equal(t, tc.want, cfg.Cpu.PreferredMeters)
		})
	}
}
