// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"strings"
	"testing"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStdoutExporterInterval(t *testing.T) {
	tests := []struct {
		name     string
		yamlData string
		args     []string
		want     time.Duration
	}{
		{
			name: "default",
			want: 2 * time.Second,
		},
		{
			name:     "enabled without interval",
			yamlData: "exporter:\n  stdout:\n    enabled: true\n",
			want:     2 * time.Second,
		},
		{
			name:     "YAML interval preserved without flag",
			yamlData: "exporter:\n  stdout:\n    interval: 10s\n",
			args:     []string{"--log.level=debug"},
			want:     10 * time.Second,
		},
		{
			name: "CLI interval",
			args: []string{"--exporter.stdout.interval=10s"},
			want: 10 * time.Second,
		},
		{
			name:     "CLI overrides YAML",
			yamlData: "exporter:\n  stdout:\n    interval: 5s\n",
			args:     []string{"--exporter.stdout.interval=10s"},
			want:     10 * time.Second,
		},
		{
			name:     "explicit default overrides YAML",
			yamlData: "exporter:\n  stdout:\n    interval: 10s\n",
			args:     []string{"--exporter.stdout.interval=2s"},
			want:     2 * time.Second,
		},
		{
			name: "subsecond interval",
			args: []string{"--exporter.stdout.interval=250ms"},
			want: 250 * time.Millisecond,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := Load(strings.NewReader(tc.yamlData))
			require.NoError(t, err)

			app := kingpin.New("test", "Test application")
			updateConfig := RegisterFlags(app)
			_, err = app.Parse(tc.args)
			require.NoError(t, err)
			require.NoError(t, updateConfig(cfg))
			assert.Equal(t, tc.want, cfg.Exporter.Stdout.Interval)
		})
	}
}

func TestStdoutExporterIntervalValidation(t *testing.T) {
	for _, interval := range []string{"0s", "-1s"} {
		t.Run("YAML/"+interval, func(t *testing.T) {
			cfg, err := Load(strings.NewReader("exporter:\n  stdout:\n    interval: " + interval + "\n"))
			require.ErrorContains(t, err, "invalid stdout exporter interval")
			assert.ErrorContains(t, err, "must be greater than zero")
			assert.Nil(t, cfg)
		})
		t.Run("CLI/"+interval, func(t *testing.T) {
			app := kingpin.New("test", "Test application")
			updateConfig := RegisterFlags(app)
			_, err := app.Parse([]string{"--exporter.stdout.interval=" + interval})
			require.NoError(t, err)
			err = updateConfig(DefaultConfig())
			require.ErrorContains(t, err, "invalid stdout exporter interval")
			assert.ErrorContains(t, err, "must be greater than zero")
		})
	}
}

func TestStdoutExporterIntervalInvalidDuration(t *testing.T) {
	t.Run("YAML", func(t *testing.T) {
		cfg, err := Load(strings.NewReader("exporter:\n  stdout:\n    interval: invalid\n"))
		require.Error(t, err)
		assert.Nil(t, cfg)
	})
	t.Run("CLI", func(t *testing.T) {
		app := kingpin.New("test", "Test application")
		RegisterFlags(app)
		_, err := app.Parse([]string{"--exporter.stdout.interval=invalid"})
		assert.Error(t, err)
	})
}

func TestStdoutExporterIntervalString(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Exporter.Stdout.Interval = 10 * time.Second
	assert.Contains(t, cfg.String(), "interval: 10s")
	assert.Contains(t, cfg.manualString(), "exporter.stdout.interval: 10s\n")
}
