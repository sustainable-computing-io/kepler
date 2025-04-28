// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRaplConfig(t *testing.T) {
	cfg := DefaultConfig()
	assert.Empty(t, cfg.Rapl.Zones, "rapl zones should be empty by default")
}

// TestLoadRaplConfigFromYAML tests loading rapl configuration from YAML
func TestLoadRaplConfigFromYAML(t *testing.T) {
	tt := []struct {
		name     string
		yamlData string
		zones    []string
	}{
		{"empty", ``, []string{}},
		{
			"only-pkg",
			`
rapl:
  zones:
  - package
`,
			[]string{"package"},
		},
		{
			"2 zones",
			`
rapl:
  zones:
  - package
  - core
`,
			[]string{"package", "core"},
		},
		{
			"sanitize",
			`
rapl:
  zones:
  - "package  "
  - "  core"
`,
			[]string{"package", "core"},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			reader := strings.NewReader(tc.yamlData)
			cfg, err := Load(reader)
			assert.NoError(t, err)
			assert.Equal(t, tc.zones, cfg.Rapl.Zones)
		})
	}
}

// TestEmptyRaplConfigFromYAML tests loading an empty rapl configuration (keeps defaults)
func TestEmptyRaplConfigFromYAML(t *testing.T) {
	reader := strings.NewReader(``)
	cfg, err := Load(reader)
	assert.NoError(t, err)

	// Verify all values are defaults
	defaultCfg := DefaultConfig()
	assert.Equal(t, defaultCfg.Rapl.Zones, cfg.Rapl.Zones)
}

// TestComplexRaplConfig tests loading complex rapl configuration
func TestComplexRaplConfig(t *testing.T) {
	yamlData := `
rapl:
  zones:
    - package
    - core
    - dram
`
	// Load config from YAML
	reader := strings.NewReader(yamlData)
	cfg, err := Load(reader)
	assert.NoError(t, err)

	// Verify configuration values
	assert.Equal(t, []string{"package", "core", "dram"}, cfg.Rapl.Zones)
	assert.Len(t, cfg.Rapl.Zones, 3)
}

func TestRaplConfigString(t *testing.T) {
	cfg := &Config{
		Rapl: Rapl{
			Zones: []string{"package", "core"},
		},
	}

	str := cfg.String()
	assert.Contains(t, str, "rapl:")
	assert.Contains(t, str, "- package")
	assert.Contains(t, str, "- core")

	// Test manual string method
	manualStr := cfg.manualString()
	assert.Contains(t, manualStr, "rapl.zones: package, core")
}
