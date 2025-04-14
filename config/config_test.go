// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"os"
	"strings"
	"testing"

	"github.com/alecthomas/kingpin/v2"
	"github.com/stretchr/testify/assert"
)

func TestDefaultConfig(t *testing.T) {
	// Test default configuration values
	cfg := DefaultConfig()

	// Assert default values are set correctly
	assert.Equal(t, "info", cfg.Log.Level)
	assert.Equal(t, "text", cfg.Log.Format)
}

func TestLoadFromYAML(t *testing.T) {
	// Test loading configuration from YAML
	yamlData := `
log:
  level: debug
  format: json
`
	// Load config from YAML
	reader := strings.NewReader(yamlData)
	cfg, err := Load(reader)
	assert.NoError(t, err)

	// Verify configuration values
	assert.Equal(t, "debug", cfg.Log.Level)
	assert.Equal(t, "json", cfg.Log.Format)
}

func TestLoadEmptyFromYAML(t *testing.T) {
	// Test loading an empty configuration
	yamlData := ``
	reader := strings.NewReader(yamlData)
	cfg, err := Load(reader)
	assert.NoError(t, err)

	// Verify all values are defaults
	defaultCfg := DefaultConfig()
	assert.Equal(t, defaultCfg.Log.Level, cfg.Log.Level)
	assert.Equal(t, defaultCfg.Log.Format, cfg.Log.Format)
}

func TestLoadInvalidConfigFromYAML(t *testing.T) {
	// Test loading an empty configuration
	yamlData := `
log:
  level: FATAL
  format: json
`
	reader := strings.NewReader(yamlData)
	cfg, err := Load(reader)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid configuration")
	assert.Nil(t, cfg)
}

func TestCommandLinePrecedence(t *testing.T) {
	// Create config from YAML
	yamlData := `
log:
  level: info
`
	// Load config from YAML
	reader := strings.NewReader(yamlData)
	cfg, err := Load(reader)
	assert.Equal(t, "info", cfg.Log.Level, "Must read YAML file")
	assert.NoError(t, err)

	// Create a kingpin app and register flags
	app := kingpin.New("test", "Test application")
	updateConfig := RegisterFlags(app)
	assert.Equal(t, "info", cfg.Log.Level, "Must not change YAML values until updateConfig is called")

	// Parse command line arguments that override some settings
	_, err = app.Parse([]string{"--log.level=debug"})
	assert.NoError(t, err)

	// Update config with parsed flags
	err = updateConfig(cfg)
	assert.NoError(t, err)

	// Verify that command line arguments take precedence
	assert.Equal(t, "debug", cfg.Log.Level, "Command line should override YAML value")
	assert.Equal(t, "text", cfg.Log.Format, "Default value should not be overridden")
}

func TestPartialConfig(t *testing.T) {
	// Test loading partial configuration
	yamlData := `
log:
  level: warn
`
	// Load config from YAML
	reader := strings.NewReader(yamlData)
	cfg, err := Load(reader)
	assert.NoError(t, err)

	// Values specified in YAML should be loaded
	assert.Equal(t, "warn", cfg.Log.Level)

	// Values not specified should use defaults
	assert.Equal(t, "text", cfg.Log.Format)
}

func TestWhitespaceHandling(t *testing.T) {
	// Test whitespace handling in configuration
	yamlData := `
log:
  level: "  debug  "
  format: "  json  "
`
	// Load config from YAML
	reader := strings.NewReader(yamlData)
	cfg, err := Load(reader)
	assert.NoError(t, err)

	// Trim whitespace
	cfg.sanitize()

	// Verify whitespace is trimmed
	assert.Equal(t, "debug", cfg.Log.Level)
	assert.Equal(t, "json", cfg.Log.Format)
}

func TestFromRealFile(t *testing.T) {
	// Create a temporary config file
	yamlData := `
log:
  level: debug
`
	tmpfile, err := os.CreateTemp("", "config-*.yaml")
	assert.NoError(t, err)
	defer os.Remove(tmpfile.Name())

	_, err = tmpfile.Write([]byte(yamlData))
	assert.NoError(t, err)
	assert.NoError(t, tmpfile.Close())

	// Load config from file
	cfg, err := FromFile(tmpfile.Name())
	assert.NoError(t, err)

	// Verify config is loaded correctly
	assert.Equal(t, "debug", cfg.Log.Level)
	assert.Equal(t, "text", cfg.Log.Format)
}

func TestInvalidYAML(t *testing.T) {
	// Test loading invalid YAML
	yamlData := `
log:
  level: FATAL
invalid yaml
`
	// Load config from YAML
	reader := strings.NewReader(yamlData)
	_, err := Load(reader)
	assert.Error(t, err, "Loading invalid YAML should return an error")
}

func TestInvalidFile(t *testing.T) {
	_, err := FromFile("non_existent_file.yaml")
	assert.Error(t, err, "Loading from non-existent file should return an error")
}

// ErrorReader is a mock io.Reader that always returns an error
type ErrorReader struct{}

func (r *ErrorReader) Read(p []byte) (n int, err error) {
	return 0, os.ErrInvalid
}

func TestReadError(t *testing.T) {
	// Test error when reading fails
	reader := &ErrorReader{}
	_, err := Load(reader)
	assert.Error(t, err, "Read error should propagate")
}

func TestInvalidConfigurationValues(t *testing.T) {
	// Test validation of configuration values (command line and YAML)
	// Create a kingpin app and register flags
	tt := []struct {
		name   string
		config *Config
		error  string
	}{{
		name:   "default config",
		config: DefaultConfig(), // no errors
	}, {
		name: "custom config",
		config: &Config{
			Log: Log{
				Level:  "debg",  // invalid log level
				Format: "jAson", // invalid log format
			},
			Host: Host{
				SysFS:  "/sys",
				ProcFS: "/proc",
			},
		},
		error: "invalid log level",
	}, {
		name: "custom host sysfs",
		config: &Config{
			Log: Log{
				Level:  "info",
				Format: "text",
			},
			Host: Host{
				SysFS: "/invalid/path",
			},
		},
		error: "invalid sysfs path",
	}, {
		name: "custom host procfs",
		config: &Config{
			Log: Log{
				Level:  "info",
				Format: "text",
			},
			Host: Host{
				ProcFS: "/invalid/path",
			},
		},
		error: "invalid procfs path",
	}, {
		name: "unreadable host procfs",
		config: &Config{
			Log: Log{
				Level:  "info",
				Format: "text",
			},
			Host: Host{
				ProcFS: "/root",
			},
		},
		error: "invalid procfs path",
	}, {
		name: "unreadable host sysfs",
		config: &Config{
			Log: Log{
				Level:  "info",
				Format: "text",
			},
			Host: Host{
				SysFS: "/root",
			},
		},
		error: "invalid sysfs path",
	}}

	// test yaml marshall
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// Get string representation
			err := tc.config.Validate()
			if tc.error == "" {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.error)
			}
		})
	}

	// test manual string builder approach
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// Get string representation
			str := tc.config.manualString()

			// Verify it's valid YAML and contains the expected values
			assert.Contains(t, str, "log.level: "+tc.config.Log.Level)
			assert.Contains(t, str, "log.format: "+tc.config.Log.Format)
			assert.Contains(t, str, "host.sysfs: "+tc.config.Host.SysFS)
			assert.Contains(t, str, "host.procfs: "+tc.config.Host.ProcFS)
		})
	}
}

func TestConfigValidation(t *testing.T) {
	tt := []struct {
		name          string
		args          []string
		expectedError string
	}{
		{"invalid log.level", []string{"--log.level=FATAL"}, "invalid log level"},
		{"invalid log.format", []string{"--log.format=JASON"}, "invalid log format"},
		{"invalid host.sysfs", []string{"--host.sysfs=/non-existent-dir"}, "invalid sysfs"},
		{"invalid host.procfs", []string{"--host.procfs=/non-existent-dir"}, "invalid procfs"},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			app := kingpin.New("test", "Test application")
			updateConfig := RegisterFlags(app)
			_, parseErr := app.Parse(tc.args)
			assert.Error(t, parseErr, "expected test args to produce a parse error")
			cfg := DefaultConfig()
			err := updateConfig(cfg)
			assert.Error(t, err, "invalid input should be rejected by validation")
			assert.Contains(t, err.Error(), tc.expectedError)
		})
	}
}

func TestConfigString(t *testing.T) {
	tt := []struct {
		name   string
		config *Config
	}{{
		name: "default config",
		config: &Config{
			Log: Log{
				Level:  "info",
				Format: "text",
			},
		},
	}, {
		name: "custom config",
		config: &Config{
			Log: Log{
				Level:  "debug",
				Format: "json",
			},
		},
	}, {
		name: "custom host sysfs",
		config: &Config{
			Host: Host{
				SysFS: "/sys/fake",
			},
		},
	}, {
		name: "custom host procfs",
		config: &Config{
			Host: Host{
				ProcFS: "/proc/fake",
			},
		},
	}}

	// test yaml marshall
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// Get string representation
			str := tc.config.String()

			// Verify it's valid YAML and contains the expected values
			assert.Contains(t, str, "log:")
			assert.Contains(t, str, "level: "+tc.config.Log.Level)
			assert.Contains(t, str, "format: "+tc.config.Log.Format)
		})
	}

	// test manual string builder approach
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// Get string representation
			str := tc.config.manualString()

			// Verify it's valid YAML and contains the expected values
			assert.Contains(t, str, "log.level: "+tc.config.Log.Level)
			assert.Contains(t, str, "log.format: "+tc.config.Log.Format)
			assert.Contains(t, str, "host.sysfs: "+tc.config.Host.SysFS)
			assert.Contains(t, str, "host.procfs: "+tc.config.Host.ProcFS)
		})
	}
}
