// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/utils/ptr"
)

// Hwmon-related tests for improved coverage

func TestDefaultHwmonConfig(t *testing.T) {
	hwmon := defaultHwmonConfig()
	assert.Equal(t, ptr.To(false), hwmon.Enabled)
	assert.Empty(t, hwmon.Zones)
}

func TestApplyHwmonFlags(t *testing.T) {
	tests := []struct {
		name     string
		hwmon    *Hwmon
		flagsSet map[string]bool
		enabled  *bool
		zones    *[]string
		expected *Hwmon
	}{{
		name:     "no flags set",
		hwmon:    &Hwmon{},
		flagsSet: map[string]bool{},
		enabled:  ptr.To(true),
		zones:    &[]string{"package", "core"},
		expected: &Hwmon{},
	}, {
		name:  "enabled flag set",
		hwmon: &Hwmon{},
		flagsSet: map[string]bool{
			ExperimentalHwmonEnabledFlag: true,
		},
		enabled: ptr.To(true),
		zones:   &[]string{"package", "core"},
		expected: &Hwmon{
			Enabled: ptr.To(true),
		},
	}, {
		name:  "zones flag set",
		hwmon: &Hwmon{},
		flagsSet: map[string]bool{
			ExperimentalHwmonZonesFlag: true,
		},
		enabled: ptr.To(true),
		zones:   &[]string{"package", "core"},
		expected: &Hwmon{
			Zones: []string{"package", "core"},
		},
	}, {
		name:  "all flags set",
		hwmon: &Hwmon{},
		flagsSet: map[string]bool{
			ExperimentalHwmonEnabledFlag: true,
			ExperimentalHwmonZonesFlag:   true,
		},
		enabled: ptr.To(true),
		zones:   &[]string{"package", "core"},
		expected: &Hwmon{
			Enabled: ptr.To(true),
			Zones:   []string{"package", "core"},
		},
	}, {
		name:  "enabled false flag",
		hwmon: &Hwmon{},
		flagsSet: map[string]bool{
			ExperimentalHwmonEnabledFlag: true,
		},
		enabled: ptr.To(false),
		zones:   &[]string{},
		expected: &Hwmon{
			Enabled: ptr.To(false),
		},
	}, {
		name:  "empty zones",
		hwmon: &Hwmon{},
		flagsSet: map[string]bool{
			ExperimentalHwmonZonesFlag: true,
		},
		enabled: ptr.To(true),
		zones:   &[]string{},
		expected: &Hwmon{
			Zones: []string{},
		},
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			applyHwmonFlags(tc.hwmon, tc.flagsSet, tc.enabled, tc.zones)
			assert.Equal(t, tc.expected, tc.hwmon)
		})
	}
}

func TestHasHwmonFlags(t *testing.T) {
	tests := []struct {
		name     string
		flagsSet map[string]bool
		expected bool
	}{{
		name:     "no hwmon flags",
		flagsSet: map[string]bool{},
		expected: false,
	}, {
		name: "enabled flag set",
		flagsSet: map[string]bool{
			ExperimentalHwmonEnabledFlag: true,
		},
		expected: true,
	}, {
		name: "zones flag set",
		flagsSet: map[string]bool{
			ExperimentalHwmonZonesFlag: true,
		},
		expected: true,
	}, {
		name: "multiple hwmon flags set",
		flagsSet: map[string]bool{
			ExperimentalHwmonEnabledFlag: true,
			ExperimentalHwmonZonesFlag:   true,
		},
		expected: true,
	}, {
		name: "other experimental flags set (not hwmon)",
		flagsSet: map[string]bool{
			ExperimentalPlatformRedfishEnabledFlag: true,
		},
		expected: false,
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := hasHwmonFlags(tc.flagsSet)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestApplyHwmonConfig(t *testing.T) {
	tests := []struct {
		name        string
		cfg         *Config
		flagsSet    map[string]bool
		enabled     *bool
		zones       *[]string
		expectError bool
	}{{
		name:     "no hwmon flags and no experimental config",
		cfg:      &Config{},
		flagsSet: map[string]bool{},
		enabled:  ptr.To(false),
		zones:    &[]string{},
	}, {
		name: "has hwmon flags",
		cfg:  &Config{},
		flagsSet: map[string]bool{
			ExperimentalHwmonEnabledFlag: true,
		},
		enabled: ptr.To(true),
		zones:   &[]string{"package"},
	}, {
		name: "experimental config already exists",
		cfg: &Config{
			Experimental: &Experimental{
				Hwmon: Hwmon{
					Enabled: ptr.To(false),
					Zones:   []string{"core"},
				},
			},
		},
		flagsSet: map[string]bool{
			ExperimentalHwmonEnabledFlag: true,
		},
		enabled: ptr.To(true),
		zones:   &[]string{"package"},
	}, {
		name: "zones flag overrides config",
		cfg: &Config{
			Experimental: &Experimental{
				Hwmon: Hwmon{
					Zones: []string{"old-zone"},
				},
			},
		},
		flagsSet: map[string]bool{
			ExperimentalHwmonZonesFlag: true,
		},
		enabled: ptr.To(false),
		zones:   &[]string{"new-zone1", "new-zone2"},
	}, {
		name: "all hwmon flags override config",
		cfg: &Config{
			Experimental: &Experimental{
				Hwmon: Hwmon{
					Enabled: ptr.To(false),
					Zones:   []string{"old-zone"},
				},
			},
		},
		flagsSet: map[string]bool{
			ExperimentalHwmonEnabledFlag: true,
			ExperimentalHwmonZonesFlag:   true,
		},
		enabled: ptr.To(true),
		zones:   &[]string{"package", "core"},
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := applyHwmonConfig(tc.cfg, tc.flagsSet, tc.enabled, tc.zones)

			if tc.expectError {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)

			// Verify experimental section exists if hwmon flags were set
			if hasHwmonFlags(tc.flagsSet) {
				assert.NotNil(t, tc.cfg.Experimental)

				// Verify enabled flag was applied if set
				if tc.flagsSet[ExperimentalHwmonEnabledFlag] {
					assert.Equal(t, tc.enabled, tc.cfg.Experimental.Hwmon.Enabled)
				}

				// Verify zones were applied if set
				if tc.flagsSet[ExperimentalHwmonZonesFlag] {
					assert.Equal(t, *tc.zones, tc.cfg.Experimental.Hwmon.Zones)
				}
			}
		})
	}
}

func TestIsFeatureEnabled_Hwmon(t *testing.T) {
	tests := []struct {
		name     string
		config   *Config
		expected bool
	}{{
		name: "hwmon feature enabled",
		config: &Config{
			Experimental: &Experimental{
				Hwmon: Hwmon{
					Enabled: ptr.To(true),
				},
			},
		},
		expected: true,
	}, {
		name: "hwmon feature disabled",
		config: &Config{
			Experimental: &Experimental{
				Hwmon: Hwmon{
					Enabled: ptr.To(false),
				},
			},
		},
		expected: false,
	}, {
		name:     "hwmon feature nil experimental",
		config:   &Config{},
		expected: false,
	}, {
		name: "hwmon feature nil enabled pointer",
		config: &Config{
			Experimental: &Experimental{
				Hwmon: Hwmon{
					Enabled: nil,
				},
			},
		},
		expected: false,
	}, {
		name: "hwmon feature with zones but disabled",
		config: &Config{
			Experimental: &Experimental{
				Hwmon: Hwmon{
					Enabled: ptr.To(false),
					Zones:   []string{"package", "core"},
				},
			},
		},
		expected: false,
	}, {
		name: "hwmon feature enabled with zones",
		config: &Config{
			Experimental: &Experimental{
				Hwmon: Hwmon{
					Enabled: ptr.To(true),
					Zones:   []string{"package", "core"},
				},
			},
		},
		expected: true,
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.config.IsFeatureEnabled(ExperimentalHwmonFeature)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestExperimentalFeatureEnabled_Hwmon(t *testing.T) {
	tests := []struct {
		name     string
		config   *Config
		expected bool
	}{{
		name: "hwmon enabled",
		config: &Config{
			Experimental: &Experimental{
				Hwmon: Hwmon{
					Enabled: ptr.To(true),
				},
			},
		},
		expected: true,
	}, {
		name: "hwmon disabled",
		config: &Config{
			Experimental: &Experimental{
				Hwmon: Hwmon{
					Enabled: ptr.To(false),
				},
			},
		},
		expected: false,
	}, {
		name:     "no experimental config",
		config:   &Config{},
		expected: false,
	}, {
		name: "hwmon and redfish both enabled",
		config: &Config{
			Experimental: &Experimental{
				Hwmon: Hwmon{
					Enabled: ptr.To(true),
				},
				Platform: Platform{
					Redfish: Redfish{
						Enabled: ptr.To(true),
					},
				},
			},
		},
		expected: true,
	}, {
		name: "only redfish enabled, hwmon disabled",
		config: &Config{
			Experimental: &Experimental{
				Hwmon: Hwmon{
					Enabled: ptr.To(false),
				},
				Platform: Platform{
					Redfish: Redfish{
						Enabled: ptr.To(true),
					},
				},
			},
		},
		expected: true,
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.config.experimentalFeatureEnabled()
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestSanitize_HwmonFields(t *testing.T) {
	tests := []struct {
		name            string
		config          *Config
		expectedZones   []string
		experimentalNil bool
	}{{
		name: "sanitize zones with whitespace",
		config: &Config{
			Experimental: &Experimental{
				Hwmon: Hwmon{
					Enabled: ptr.To(true),
					Zones:   []string{" package ", "  core  ", "ppt"},
				},
			},
		},
		expectedZones: []string{"package", "core", "ppt"},
	}, {
		name: "empty zones",
		config: &Config{
			Experimental: &Experimental{
				Hwmon: Hwmon{
					Enabled: ptr.To(true),
					Zones:   []string{},
				},
			},
		},
		expectedZones: []string{},
	}, {
		name: "zones with tabs and newlines",
		config: &Config{
			Experimental: &Experimental{
				Hwmon: Hwmon{
					Enabled: ptr.To(true),
					Zones:   []string{"\tpackage\n", "core\t"},
				},
			},
		},
		expectedZones: []string{"package", "core"},
	}, {
		name: "nil zones slices",
		config: &Config{
			Experimental: &Experimental{
				Hwmon: Hwmon{
					Enabled: ptr.To(true),
					Zones:   nil,
				},
			},
		},
		expectedZones: nil,
	}, {
		name: "all experimental features disabled - should set experimental to nil",
		config: &Config{
			Experimental: &Experimental{
				Hwmon: Hwmon{
					Enabled: ptr.To(false),
					Zones:   []string{" zone1 "},
				},
				Platform: Platform{
					Redfish: Redfish{
						Enabled: ptr.To(false),
					},
				},
			},
		},
		experimentalNil: true,
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.config.sanitize()

			if tc.experimentalNil {
				assert.Nil(t, tc.config.Experimental)
				return
			}

			assert.NotNil(t, tc.config.Experimental)
			assert.Equal(t, tc.expectedZones, tc.config.Experimental.Hwmon.Zones)
		})
	}
}

func TestHwmonConfig_YAMLParsing(t *testing.T) {
	tests := []struct {
		name         string
		yamlContent  string
		expectError  bool
		validateFunc func(*testing.T, *Config)
	}{{
		name: "hwmon enabled in yaml",
		yamlContent: `
experimental:
  hwmon:
    enabled: true
    zones:
      - package
      - core
`,
		expectError: false,
		validateFunc: func(t *testing.T, cfg *Config) {
			assert.NotNil(t, cfg.Experimental)
			assert.Equal(t, ptr.To(true), cfg.Experimental.Hwmon.Enabled)
			assert.Equal(t, []string{"package", "core"}, cfg.Experimental.Hwmon.Zones)
		},
	}, {
		name: "hwmon disabled in yaml",
		yamlContent: `
experimental:
  hwmon:
    enabled: false
`,
		expectError: false,
		validateFunc: func(t *testing.T, cfg *Config) {
			// When all experimental features are disabled, Experimental is set to nil by sanitize()
			// This test verifies that hwmon can be disabled
			if cfg.Experimental != nil {
				assert.Equal(t, ptr.To(false), cfg.Experimental.Hwmon.Enabled)
			}
			// The fact that IsFeatureEnabled returns false is what matters
			assert.False(t, cfg.IsFeatureEnabled(ExperimentalHwmonFeature))
		},
	}, {
		name: "hwmon with empty zones",
		yamlContent: `
experimental:
  hwmon:
    enabled: true
    zones: []
`,
		expectError: false,
		validateFunc: func(t *testing.T, cfg *Config) {
			assert.NotNil(t, cfg.Experimental)
			assert.Equal(t, ptr.To(true), cfg.Experimental.Hwmon.Enabled)
			assert.Empty(t, cfg.Experimental.Hwmon.Zones)
		},
	}, {
		name: "hwmon and redfish both configured",
		yamlContent: `
experimental:
  hwmon:
    enabled: true
    zones:
      - package
  platform:
    redfish:
      enabled: false
`,
		expectError: false,
		validateFunc: func(t *testing.T, cfg *Config) {
			assert.NotNil(t, cfg.Experimental)
			assert.Equal(t, ptr.To(true), cfg.Experimental.Hwmon.Enabled)
			assert.Equal(t, []string{"package"}, cfg.Experimental.Hwmon.Zones)
			assert.Equal(t, ptr.To(false), cfg.Experimental.Platform.Redfish.Enabled)
		},
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			reader := strings.NewReader(tc.yamlContent)
			cfg, err := Load(reader)

			if tc.expectError {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			if tc.validateFunc != nil {
				tc.validateFunc(t, cfg)
			}
		})
	}
}
