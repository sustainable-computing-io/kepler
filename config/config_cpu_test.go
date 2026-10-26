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
			name: "prometheus config keeps explicit fallback order",
			setup: func(c *Config) {
				c.Cpu.PreferredMeters = []string{"prometheus", "rapl"}
				c.Experimental = &Experimental{}
				c.Experimental.PrometheusPower.BaseURL = "https://vm.example.org:8428"
				c.Experimental.PrometheusPower.Query = "up"
			},
			want: []string{"prometheus", "rapl"},
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
			meters: []string{"rapl", "hwmon", "fake", "prometheus"},
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

func TestLoadPrometheusViaPreferredMeters(t *testing.T) {
	yamlData := `
cpu:
  preferredMeters: [rapl, prometheus]
experimental:
  prometheus-power:
    baseURL: " https://vm.example.org:8428 "
    query: sum(pdu_power_watts{node="n1"})
`
	cfg, err := Load(strings.NewReader(yamlData))
	require.NoError(t, err)
	cfg.ApplyCpuMeterDeprecations(slog.New(slog.DiscardHandler))

	assert.Equal(t, []string{"rapl", "prometheus"}, cfg.Cpu.PreferredMeters)
	require.NotNil(t, cfg.Experimental, "experimental section must survive sanitize when prometheus is a preferred meter")
	assert.Equal(t, "https://vm.example.org:8428", cfg.Experimental.PrometheusPower.BaseURL)
	assert.Equal(t, `sum(pdu_power_watts{node="n1"})`, cfg.Experimental.PrometheusPower.Query)
}

func TestLoadPrometheusPowerNotSelected(t *testing.T) {
	yamlData := `
experimental:
  prometheus-power:
    baseURL: https://vm.example.org:8428
    query: up
`
	cfg, err := Load(strings.NewReader(yamlData))
	require.NoError(t, err)
	assert.Nil(t, cfg.Experimental, "unused experimental section is hidden")
}

func TestConfigStringRedactsPrometheusPowerCredentials(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Experimental = &Experimental{}
	cfg.Experimental.PrometheusPower.BaseURL = "https://kepler:s3cret@vm.example.org:8428"

	str := cfg.String()
	assert.NotContains(t, str, "s3cret")
	assert.Contains(t, str, "https://kepler:xxxxx@vm.example.org:8428")
	assert.Equal(t, "https://kepler:s3cret@vm.example.org:8428", cfg.Experimental.PrometheusPower.BaseURL,
		"String must not modify the config")
}

func TestRedactURL(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "empty", in: "", want: ""},
		{name: "no userinfo", in: "https://vm.example.org:8428", want: "https://vm.example.org:8428"},
		{name: "user and password", in: "https://u:p@vm.example.org", want: "https://u:xxxxx@vm.example.org"},
		{name: "user only", in: "https://u@vm.example.org", want: "https://u@vm.example.org"},
		{name: "unparseable", in: "https://u:p@vm.example.org:port", want: "<redacted: unparseable URL>"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, RedactURL(tc.in))
		})
	}
}
