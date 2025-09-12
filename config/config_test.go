// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
	"k8s.io/utils/ptr"
)

func TestDefaultConfig(t *testing.T) {
	// Test default configuration values
	cfg := DefaultConfig()

	// Assert default values are set correctly
	assert.Equal(t, "info", cfg.Log.Level)
	assert.Equal(t, "text", cfg.Log.Format)
	assert.Equal(t, "", cfg.Web.Config)
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

	assert.Equal(t, defaultCfg.Monitor.Interval, cfg.Monitor.Interval)
	assert.Equal(t, defaultCfg.Monitor.Staleness, cfg.Monitor.Staleness)
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
exporter:
  stdout:
    enabled: false
  prometheus:
    enabled: false
    debugCollectors:
      - go
debug:
  pprof:
    enabled: false
`
	// Load config from YAML
	reader := strings.NewReader(yamlData)
	cfg, err := Load(reader)
	assert.NoError(t, err)

	// Create a kingpin app and register flags
	app := kingpin.New("test", "Test application")
	updateConfig := RegisterFlags(app)

	// Parse command line arguments that override some settings
	_, err = app.Parse([]string{
		"--exporter.stdout",
		"--debug.pprof",
	})
	assert.NoError(t, err)

	// Update config with parsed flags
	err = updateConfig(cfg)
	assert.NoError(t, err)

	// Verify that command line arguments take precedence
	assert.True(t, *cfg.Exporter.Stdout.Enabled, "stdout exporter should be enabled from flag")
	assert.False(t, *cfg.Exporter.Prometheus.Enabled, "prometheus exporter should remain disabled from yaml")
	assert.ElementsMatch(t, []string{"go"}, cfg.Exporter.Prometheus.DebugCollectors,
		"debug collectors should be overridden by flag")
	assert.True(t, *cfg.Debug.Pprof.Enabled, "pprof should be enabled from flag")
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

exporter:
  prometheus:
    debugCollectors: ["  go  ", "  process  "]
`
	reader := strings.NewReader(yamlData)
	cfg, err := Load(reader)
	assert.NoError(t, err)

	cfg.sanitize()

	// Verify whitespace is trimmed
	assert.Equal(t, "debug", cfg.Log.Level)
	assert.Equal(t, "json", cfg.Log.Format)
	assert.ElementsMatch(t, []string{"go", "process"}, cfg.Exporter.Prometheus.DebugCollectors,
		"debug collectors should be sanitized")
}

func TestFromRealFile(t *testing.T) {
	// Create a temporary config file
	yamlData := `
log:
  level: debug
`
	tmpfile, err := os.CreateTemp("", "config-*.yaml")
	assert.NoError(t, err)
	defer func() {
		_ = os.Remove(tmpfile.Name())
	}()

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
	}, {
		name: "unreadable web config",
		config: &Config{
			Web: Web{
				Config: "/from/unreadable/path/web.yaml",
			},
		},
		error: "invalid web config file",
	}, {
		name: "unreadable kubeconfig",
		config: &Config{
			Kube: Kube{
				Config:  "/non/existent/file",
				Enabled: ptr.To(true),
			},
		},
		error: "unreadable kubeconfig",
	}, {
		name: "kube enabled, nodeName not supplied",
		config: &Config{
			Kube: Kube{
				Enabled: ptr.To(true),
			},
		},
		error: "kube.node-name not supplied but kube.enable set to true",
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
	}, {
		name: "custom web.config",
		config: &Config{
			Web: Web{
				Config: "/fake/web.config.yml",
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

func TestEnablePprof(t *testing.T) {
	tt := []struct {
		name    string
		args    []string
		enabled bool
	}{{
		name:    "enable pprof with flag",
		args:    []string{"--debug.pprof"},
		enabled: true,
	}, {
		name:    "disable pprof no flag",
		args:    []string{"--log.level=debug"},
		enabled: false,
	}, {
		name:    "disable pprof with flag",
		args:    []string{"--no-debug.pprof"},
		enabled: false,
	}}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			app := kingpin.New("test", "Test application")
			updateConfig := RegisterFlags(app)
			_, parseErr := app.Parse(tc.args)
			assert.NoError(t, parseErr, "unexpected flag parsing error")
			cfg := DefaultConfig()
			err := updateConfig(cfg)
			assert.NoError(t, err, "unexpected config update error")
			assert.Equal(t, *cfg.Debug.Pprof.Enabled, tc.enabled, "unexpected flag value")
		})
	}
}

func TestWebConfig(t *testing.T) {
	t.Run("no web config", func(t *testing.T) {
		app := kingpin.New("test", "Test application")
		updateConfig := RegisterFlags(app)
		_, parseErr := app.Parse([]string{"--log.level=debug"})
		assert.NoError(t, parseErr, "unexpected flag parsing error")
		cfg := DefaultConfig()
		err := updateConfig(cfg)
		assert.NoError(t, err, "unexpected config update error")
		assert.Equal(t, cfg.Web.Config, "", "unexpected web.config-file configured")
	})
	t.Run("invalid web config", func(t *testing.T) {
		app := kingpin.New("test", "Test application")
		updateConfig := RegisterFlags(app)
		_, parseErr := app.Parse([]string{"--web.config-file=/fake/web.yml"})
		assert.NoError(t, parseErr, "unexpected flag parsing error")
		cfg := DefaultConfig()
		err := updateConfig(cfg)
		assert.Error(t, err, "expected config update error")
	})
	t.Run("valid web config", func(t *testing.T) {
		tempWebConfig, err := os.CreateTemp("", "temp_*web.yml")
		assert.NoError(t, err, "cannot create temp file")
		webConfig := `
tls_server_config:
  cert_file: cert.pem
  key_file: key.pem
`
		_, err = tempWebConfig.Write([]byte(webConfig))
		assert.NoError(t, err, "cannot write to temp web config")

		app := kingpin.New("test", "Test application")
		updateConfig := RegisterFlags(app)
		flagStr := fmt.Sprintf("--web.config-file=%s", tempWebConfig.Name())
		_, parseErr := app.Parse([]string{flagStr})
		assert.NoError(t, parseErr, "unexpected flag parsing error")
		cfg := DefaultConfig()
		err = updateConfig(cfg)
		assert.NoError(t, err, "expected config update error")
		assert.Equal(t, cfg.Web.Config, tempWebConfig.Name(), "unexpected config update")
		_ = os.Remove(tempWebConfig.Name())
	})
}

func TestStdoutExporter(t *testing.T) {
	tt := []struct {
		name    string
		args    []string
		enabled bool
	}{{
		name:    "no exporter.stdout flag present",
		args:    []string{"--log.level=debug"},
		enabled: false,
	}, {
		name:    "disable stdout exporter with flag",
		args:    []string{"--no-exporter.stdout"},
		enabled: false,
	}, {
		name:    "disable stdout exporter with flag",
		args:    []string{"--exporter.stdout"},
		enabled: true,
	}}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			app := kingpin.New("test", "Test application")
			updateConfig := RegisterFlags(app)
			_, parseErr := app.Parse(tc.args)
			assert.NoError(t, parseErr, "unexpected flag parsing error")
			cfg := DefaultConfig()
			err := updateConfig(cfg)
			assert.NoError(t, err, "unexpected config update error")
			assert.Equal(t, *cfg.Exporter.Stdout.Enabled, tc.enabled, "unexpected flag value")
		})
	}
}

func TestPrometheusExporter(t *testing.T) {
	tt := []struct {
		name    string
		args    []string
		enabled bool
	}{{
		name:    "no exporter.prometheus flag present",
		args:    []string{"--log.level=debug"},
		enabled: true,
	}, {
		name:    "disable prometheus exporter with flag",
		args:    []string{"--no-exporter.prometheus"},
		enabled: false,
	}, {
		name:    "enable prometheus exporter with flag",
		args:    []string{"--exporter.prometheus"},
		enabled: true,
	}}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			app := kingpin.New("test", "Test application")
			updateConfig := RegisterFlags(app)
			_, parseErr := app.Parse(tc.args)
			assert.NoError(t, parseErr, "unexpected flag parsing error")
			cfg := DefaultConfig()
			err := updateConfig(cfg)
			assert.NoError(t, err, "unexpected config update error")
			assert.Equal(t, *cfg.Exporter.Prometheus.Enabled, tc.enabled, "unexpected flag value")
		})
	}
}

func TestValidateWithSkip(t *testing.T) {
	// Create a config with invalid host paths
	cfg := DefaultConfig()
	cfg.Host.SysFS = "/path/invalid"
	cfg.Host.ProcFS = "/path/invalid"

	// Validate with skipping host validation
	err := cfg.Validate(SkipHostValidation)
	assert.NoError(t, err, "Should pass when SkipHostValidation is provided")

	// Create a config with invalid experimental config
	cfg = DefaultConfig()
	cfg.Experimental = &Experimental{
		Platform: Platform{
			Redfish: Redfish{
				Enabled:    ptr.To(true),
				ConfigFile: "/path/invalid",
			},
		},
	}

	// Validate with skipping experimental validation
	err = cfg.Validate(SkipExperimentalValidation)
	assert.NoError(t, err, "Should pass when SkipExperimentalValidation is provided")
}

func TestMonitorConfig(t *testing.T) {
	cfg := DefaultConfig()

	// Monitor should be enabled by default
	assert.True(t, cfg.Monitor.Interval > 0, "Monitor should be enabled by default")
	assert.True(t, cfg.Monitor.Staleness > 0, "staleness should be set to a positive value")

	t.Run("interval", func(t *testing.T) {
		cfg := DefaultConfig()
		assert.NoError(t, cfg.Validate())

		cfg.Monitor.Interval = -10
		assert.ErrorContains(t, cfg.Validate(), "invalid configuration: invalid monitor interval")

		cfg.Monitor.Interval = 0
		assert.NoError(t, cfg.Validate())

		cfg.Monitor.Interval = 100
		assert.NoError(t, cfg.Validate())
	})

	t.Run("staleness", func(t *testing.T) {
		cfg := DefaultConfig()
		assert.NoError(t, cfg.Validate())

		cfg.Monitor.Staleness = -10
		assert.ErrorContains(t, cfg.Validate(), "invalid configuration: invalid monitor staleness")

		cfg.Monitor.Staleness = 0
		assert.NoError(t, cfg.Validate())

		cfg.Monitor.Staleness = 100
		assert.NoError(t, cfg.Validate())
	})

	t.Run("maxTerminated", func(t *testing.T) {
		cfg := DefaultConfig()
		assert.Equal(t, 500, cfg.Monitor.MaxTerminated, "default maxTerminated should be 500")
		assert.NoError(t, cfg.Validate())

		cfg.Monitor.MaxTerminated = -10
		assert.NoError(t, cfg.Validate(), "invalid configuration: invalid monitor max terminated")

		cfg.Monitor.MaxTerminated = 0
		assert.NoError(t, cfg.Validate(), "maxTerminated=0 should be valid (unlimited)")

		cfg.Monitor.MaxTerminated = 1000
		assert.NoError(t, cfg.Validate())
	})

	t.Run("minTerminatedEnergyThreshold", func(t *testing.T) {
		cfg := DefaultConfig()
		assert.Equal(t, int64(10), cfg.Monitor.MinTerminatedEnergyThreshold, "default minTerminatedEnergyThreshold should be 10")
		assert.NoError(t, cfg.Validate())

		cfg.Monitor.MinTerminatedEnergyThreshold = -10
		assert.ErrorContains(t, cfg.Validate(), "invalid configuration: invalid monitor min terminated energy threshold")

		cfg.Monitor.MinTerminatedEnergyThreshold = 0
		assert.NoError(t, cfg.Validate(), "minTerminatedEnergyThreshold=0 should be valid (no filtering)")

		cfg.Monitor.MinTerminatedEnergyThreshold = 1000
		assert.NoError(t, cfg.Validate())
	})
}

func TestMonitorConfigFlags(t *testing.T) {
	type expect struct {
		interval      time.Duration
		staleness     time.Duration
		maxTerminated int
		parseError    error
		cfgErr        error
	}
	tt := []struct {
		name     string
		args     []string
		expected expect
	}{{
		name:     "default",
		args:     []string{},
		expected: expect{interval: 5 * time.Second, staleness: 500 * time.Millisecond, maxTerminated: 500, parseError: nil},
	}, {
		name:     "invalid-interval flag",
		args:     []string{"--monitor.interval=-10Fs"},
		expected: expect{parseError: fmt.Errorf("time: unknown unit")},
	}, {
		name:     "invalid-interval",
		args:     []string{"--monitor.interval=-10s"},
		expected: expect{cfgErr: fmt.Errorf("invalid configuration: invalid monitor interval")},
	}, {
		name:     "valid-max-terminated",
		args:     []string{"--monitor.max-terminated=1000"},
		expected: expect{interval: 5 * time.Second, staleness: 500 * time.Millisecond, maxTerminated: 1000, parseError: nil},
	}, {
		name:     "max-terminated-zero",
		args:     []string{"--monitor.max-terminated=0"},
		expected: expect{interval: 5 * time.Second, staleness: 500 * time.Millisecond, maxTerminated: 0, parseError: nil},
	}, {
		name:     "negative-max-terminated",
		args:     []string{"--monitor.max-terminated=-10"},
		expected: expect{interval: 5 * time.Second, staleness: 500 * time.Millisecond, maxTerminated: -10, parseError: nil},
	}}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			app := kingpin.New("test", "Test application")
			updateConfig := RegisterFlags(app)

			_, parseErr := app.Parse(tc.args)
			if tc.expected.parseError != nil {
				assert.ErrorContains(t, parseErr, tc.expected.parseError.Error(), "args: %v", tc.args)
				return
			}
			assert.NoError(t, parseErr, "unexpected config update error")

			cfg := DefaultConfig()
			err := updateConfig(cfg)
			if tc.expected.cfgErr != nil {
				assert.ErrorContains(t, err, tc.expected.cfgErr.Error())
				return
			}

			assert.NoError(t, err, "unexpected config update error")
			assert.Equal(t, cfg.Monitor.Interval, tc.expected.interval)
			assert.Equal(t, cfg.Monitor.Staleness, tc.expected.staleness)
			assert.Equal(t, cfg.Monitor.MaxTerminated, tc.expected.maxTerminated)
		})
	}
}

func TestMonitorMaxTerminatedYAML(t *testing.T) {
	t.Run("yaml-config-maxTerminated", func(t *testing.T) {
		yamlData := `
monitor:
  maxTerminated: 1000
`
		reader := strings.NewReader(yamlData)
		cfg, err := Load(reader)
		assert.NoError(t, err)
		assert.Equal(t, 1000, cfg.Monitor.MaxTerminated)
	})

	t.Run("yaml-config-maxTerminated-zero", func(t *testing.T) {
		yamlData := `
monitor:
  maxTerminated: 0
`
		reader := strings.NewReader(yamlData)
		cfg, err := Load(reader)
		assert.NoError(t, err)
		assert.Equal(t, 0, cfg.Monitor.MaxTerminated)
	})

	t.Run("yaml-config-maxTerminated-negative", func(t *testing.T) {
		yamlData := `
monitor:
  maxTerminated: -100
`
		reader := strings.NewReader(yamlData)
		cfg, err := Load(reader)
		assert.NoError(t, err)
		assert.Equal(t, -100, cfg.Monitor.MaxTerminated)
	})
}

func TestMonitorMinTerminatedEnergyThresholdYAML(t *testing.T) {
	t.Run("yaml-config-minTerminatedEnergyThreshold", func(t *testing.T) {
		yamlData := `
monitor:
  minTerminatedEnergyThreshold: 50
`
		reader := strings.NewReader(yamlData)
		cfg, err := Load(reader)
		assert.NoError(t, err)
		assert.Equal(t, int64(50), cfg.Monitor.MinTerminatedEnergyThreshold)
	})

	t.Run("yaml-config-minTerminatedEnergyThreshold-zero", func(t *testing.T) {
		yamlData := `
monitor:
  minTerminatedEnergyThreshold: 0
`
		reader := strings.NewReader(yamlData)
		cfg, err := Load(reader)
		assert.NoError(t, err)
		assert.Equal(t, int64(0), cfg.Monitor.MinTerminatedEnergyThreshold)
	})

	t.Run("yaml-config-minTerminatedEnergyThreshold-invalid", func(t *testing.T) {
		yamlData := `
monitor:
  minTerminatedEnergyThreshold: -100
`
		reader := strings.NewReader(yamlData)
		_, err := Load(reader)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid monitor min terminated energy threshold")
	})
}

func TestConfigDefault(t *testing.T) {
	cfg := DefaultConfig()

	// Check default exporter config
	assert.False(t, *cfg.Exporter.Stdout.Enabled, "stdout exporter should be disabled by default")
	assert.True(t, *cfg.Exporter.Prometheus.Enabled, "prometheus exporter should be enabled by default")
	assert.Equal(t, []string{"go"}, cfg.Exporter.Prometheus.DebugCollectors, "default debug collectors should be set")

	// Check default debug config
	assert.False(t, *cfg.Debug.Pprof.Enabled, "pprof should be disabled by default")
}

func TestConifgLoadFromYaml(t *testing.T) {
	yamlData := `
log:
  level: debug
  format: json
exporter:
  stdout:
    enabled: true
  prometheus:
    enabled: true
    debugCollectors:
      - go
      - process
debug:
  pprof:
    enabled: true
`
	reader := strings.NewReader(yamlData)
	cfg, err := Load(reader)
	assert.NoError(t, err)

	// Verify exporter config
	assert.True(t, *cfg.Exporter.Stdout.Enabled, "stdout exporter should be enabled")
	assert.True(t, *cfg.Exporter.Prometheus.Enabled, "prometheus exporter should be enabled")
	assert.ElementsMatch(t, []string{"go", "process"}, cfg.Exporter.Prometheus.DebugCollectors,
		"debug collectors should match")

	// Verify debug config
	assert.True(t, *cfg.Debug.Pprof.Enabled, "pprof should be enabled")
}

func TestBuilder(t *testing.T) {
	t.Run("Build", func(t *testing.T) {
		// Test Build should return default config
		b := &Builder{}
		got, err := b.Build()
		assert.NoError(t, err)

		exp := DefaultConfig()
		assert.Equal(t, exp.String(), got.String())
	})

	t.Run("Use", func(t *testing.T) {
		b := &Builder{}
		exp := DefaultConfig()
		exp.Log.Level = "warn"

		got, err := b.Use(exp).Build()
		assert.NoError(t, err)
		assert.Equal(t, exp.String(), got.String())
	})

	t.Run("MergeWithInvalidYAML", func(t *testing.T) {
		b := &Builder{}
		cfg, err := b.Merge().
			Merge(`invalid yaml: [invalid`).
			Build()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse YAML")
		assert.Nil(t, cfg)
	})

	t.Run("MultipleMerges", func(t *testing.T) {
		b := &Builder{}
		cfg, err := b.
			Merge(`
log:
  level: debug
`,
				`
monitor:
  interval: 3h
`,
				`
log:
  level: info
`).
			Build()
		assert.NoError(t, err)
		exp := DefaultConfig()
		exp.Log.Level = "info"
		exp.Monitor.Interval = 3 * time.Hour
		assert.Equal(t, exp.String(), cfg.String())
	})

	t.Run("MergeNested", func(t *testing.T) {
		b := &Builder{}
		cfg, err := b.
			Merge(`
exporter:
  prometheus:
    enabled: false
`).
			Build()
		assert.NoError(t, err)
		exp := DefaultConfig()
		exp.Exporter.Prometheus.Enabled = ptr.To(false)
		assert.Equal(t, exp.String(), cfg.String())
	})

	t.Run("MergeArrays", func(t *testing.T) {
		b := &Builder{}
		cfg, err := b.
			Merge(`
exporter:
  prometheus:
    debugCollectors: ["go", "process"]
`).
			Build()
		assert.NoError(t, err)
		exp := DefaultConfig()
		exp.Exporter.Prometheus.DebugCollectors = []string{"go", "process"}
		assert.Equal(t, exp.String(), cfg.String())
	})
}

func TestMetricsLevelValue_Set(t *testing.T) {
	tests := []struct {
		name          string
		initialLevel  Level
		setValue      string
		expectedLevel Level
		expectError   bool
		errorMessage  string
	}{
		{
			name:          "Set node from default all",
			initialLevel:  MetricsLevelAll,
			setValue:      "node",
			expectedLevel: MetricsLevelNode,
			expectError:   false,
		},
		{
			name:          "Set process from default all",
			initialLevel:  MetricsLevelAll,
			setValue:      "process",
			expectedLevel: MetricsLevelProcess,
			expectError:   false,
		},
		{
			name:          "Set container from default all",
			initialLevel:  MetricsLevelAll,
			setValue:      "container",
			expectedLevel: MetricsLevelContainer,
			expectError:   false,
		},
		{
			name:          "Set vm from default all",
			initialLevel:  MetricsLevelAll,
			setValue:      "vm",
			expectedLevel: MetricsLevelVM,
			expectError:   false,
		},
		{
			name:          "Set pod from default all",
			initialLevel:  MetricsLevelAll,
			setValue:      "pod",
			expectedLevel: MetricsLevelPod,
			expectError:   false,
		},
		{
			name:          "Accumulate node to existing process",
			initialLevel:  MetricsLevelProcess,
			setValue:      "node",
			expectedLevel: MetricsLevelProcess | MetricsLevelNode,
			expectError:   false,
		},
		{
			name:          "Accumulate container to existing node+process",
			initialLevel:  MetricsLevelNode | MetricsLevelProcess,
			setValue:      "container",
			expectedLevel: MetricsLevelNode | MetricsLevelProcess | MetricsLevelContainer,
			expectError:   false,
		},
		{
			name:          "Invalid level returns error",
			initialLevel:  MetricsLevelAll,
			setValue:      "invalid",
			expectedLevel: MetricsLevelAll, // Should remain unchanged
			expectError:   true,
			errorMessage:  "unknown metrics level: invalid",
		},
		{
			name:          "Empty string returns error",
			initialLevel:  MetricsLevelAll,
			setValue:      "",
			expectedLevel: MetricsLevelAll, // Should remain unchanged
			expectError:   true,
			errorMessage:  "unknown metrics level: ",
		},
		{
			name:          "Case insensitive - NODE",
			initialLevel:  MetricsLevelAll,
			setValue:      "NODE",
			expectedLevel: MetricsLevelNode,
			expectError:   false,
		},
		{
			name:          "Case insensitive - Process",
			initialLevel:  MetricsLevelAll,
			setValue:      "Process",
			expectedLevel: MetricsLevelProcess,
			expectError:   false,
		},
		{
			name:          "Whitespace handling - node with spaces",
			initialLevel:  MetricsLevelAll,
			setValue:      "  node  ",
			expectedLevel: MetricsLevelNode,
			expectError:   false,
		},
		{
			name:          "Set same level twice (idempotent)",
			initialLevel:  MetricsLevelNode,
			setValue:      "node",
			expectedLevel: MetricsLevelNode,
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			level := tt.initialLevel
			mlv := NewMetricsLevelValue(&level)

			err := mlv.Set(tt.setValue)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMessage != "" {
					assert.Contains(t, err.Error(), tt.errorMessage)
				}
				// Level should remain unchanged on error
				assert.Equal(t, tt.initialLevel, level)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedLevel, level)
			}
		})
	}
}

func TestMetricsLevelValue_AccumulativeBehavior(t *testing.T) {
	// Test the cumulative behavior when multiple Set calls are made
	tests := []struct {
		name          string
		initialLevel  Level
		setValues     []string
		expectedLevel Level
		expectError   bool
	}{
		{
			name:          "Accumulate multiple levels from all",
			initialLevel:  MetricsLevelAll,
			setValues:     []string{"node", "process"},
			expectedLevel: MetricsLevelNode | MetricsLevelProcess,
			expectError:   false,
		},
		{
			name:          "Accumulate multiple levels from none",
			initialLevel:  Level(0),
			setValues:     []string{"node", "process", "container"},
			expectedLevel: MetricsLevelNode | MetricsLevelProcess | MetricsLevelContainer,
			expectError:   false,
		},
		{
			name:          "Error in middle stops processing",
			initialLevel:  Level(0),
			setValues:     []string{"node", "invalid", "process"},
			expectedLevel: MetricsLevelNode, // Should have node from first call
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			level := tt.initialLevel
			mlv := NewMetricsLevelValue(&level)

			var lastErr error
			for _, setValue := range tt.setValues {
				err := mlv.Set(setValue)
				if err != nil {
					lastErr = err
					break // Stop on first error
				}
			}

			if tt.expectError {
				assert.Error(t, lastErr)
			} else {
				assert.NoError(t, lastErr)
			}

			assert.Equal(t, tt.expectedLevel, level)
		})
	}
}

func TestMetricsLevelValue_String(t *testing.T) {
	tests := []struct {
		name     string
		level    Level
		expected string
	}{
		{
			name:     "No levels (empty)",
			level:    Level(0),
			expected: "",
		},
		{
			name:     "All individual levels",
			level:    MetricsLevelAll,
			expected: "node,process,container,vm,pod",
		},
		{
			name:     "Single level - node",
			level:    MetricsLevelNode,
			expected: "node",
		},
		{
			name:     "Multiple levels - node and process",
			level:    MetricsLevelNode | MetricsLevelProcess,
			expected: "node,process",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mlv := NewMetricsLevelValue(&tt.level)
			result := mlv.String()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMetricsLevelValue_IsCumulative(t *testing.T) {
	level := MetricsLevelAll
	mlv := NewMetricsLevelValue(&level)
	assert.True(t, mlv.IsCumulative(), "MetricsLevelValue should be cumulative")
}

func TestNewMetricsLevelValue(t *testing.T) {
	t.Run("Creates valid MetricsLevelValue", func(t *testing.T) {
		level := MetricsLevelAll
		mlv := NewMetricsLevelValue(&level)

		assert.NotNil(t, mlv)
		assert.Equal(t, level, *mlv.level)
	})

	t.Run("Modifying target level affects MetricsLevelValue", func(t *testing.T) {
		level := MetricsLevelAll
		mlv := NewMetricsLevelValue(&level)

		// Modify the original level
		level = MetricsLevelNode

		// MetricsLevelValue should reflect the change
		assert.Equal(t, MetricsLevelNode, *mlv.level)
	})
}

func TestMetricsLevelValue_CommandLineIntegration(t *testing.T) {
	// Test integration with kingpin command line parsing
	tests := []struct {
		name          string
		args          []string
		expectedLevel Level
		expectError   bool
	}{
		{
			name:          "Single flag value - node",
			args:          []string{"--metrics", "node"},
			expectedLevel: MetricsLevelNode,
			expectError:   false,
		},
		{
			name:          "Multiple flag values accumulate",
			args:          []string{"--metrics", "node", "--metrics", "process"},
			expectedLevel: MetricsLevelNode | MetricsLevelProcess,
			expectError:   false,
		},
		{
			name:          "All flag values",
			args:          []string{"--metrics", "node", "--metrics", "process", "--metrics", "container", "--metrics", "vm", "--metrics", "pod"},
			expectedLevel: MetricsLevelAll,
			expectError:   false,
		},
		{
			name:          "Invalid flag value",
			args:          []string{"--metrics", "invalid"},
			expectedLevel: MetricsLevelAll, // Should remain at default
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a kingpin application for testing
			app := kingpin.New("test", "test application")
			metricsLevel := MetricsLevelAll
			app.Flag("metrics", "Metrics levels to export").SetValue(NewMetricsLevelValue(&metricsLevel))

			// Parse the arguments
			_, err := app.Parse(tt.args)

			if tt.expectError {
				assert.Error(t, err)
				// On error, the level should remain unchanged (default)
				assert.Equal(t, MetricsLevelAll, metricsLevel)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedLevel, metricsLevel)
			}
		})
	}
}

func TestMetricsLevelValue_EdgeCases(t *testing.T) {
	tests := []struct {
		name          string
		initialLevel  Level
		setValue      string
		expectedLevel Level
		expectError   bool
	}{
		{
			name:          "Special characters in value",
			initialLevel:  MetricsLevelAll,
			setValue:      "node!@#",
			expectedLevel: MetricsLevelAll,
			expectError:   true,
		},
		{
			name:          "Numeric value",
			initialLevel:  MetricsLevelAll,
			setValue:      "123",
			expectedLevel: MetricsLevelAll,
			expectError:   true,
		},
		{
			name:          "Tab and newline whitespace",
			initialLevel:  MetricsLevelAll,
			setValue:      "\t\nnode\t\n",
			expectedLevel: MetricsLevelNode,
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			level := tt.initialLevel
			mlv := NewMetricsLevelValue(&level)

			err := mlv.Set(tt.setValue)

			if tt.expectError {
				assert.Error(t, err)
				assert.Equal(t, tt.initialLevel, level)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedLevel, level)
			}
		})
	}
}

func TestWebListenAddressesValidation(t *testing.T) {
	tests := []struct {
		name          string
		addresses     []string
		expectError   bool
		errorContains string
	}{{
		name:        "valid port-only address",
		addresses:   []string{":8080"},
		expectError: false,
	}, {
		name:        "valid host:port address",
		addresses:   []string{"localhost:8080"},
		expectError: false,
	}, {
		name:        "valid IPv4 address",
		addresses:   []string{"192.168.1.1:8080"},
		expectError: false,
	}, {
		name:        "valid IPv6 address",
		addresses:   []string{"[::1]:8080"},
		expectError: false,
	}, {
		name:        "multiple valid addresses",
		addresses:   []string{":8080", "localhost:8081", "192.168.1.1:8082"},
		expectError: false,
	}, {
		name:          "empty addresses list",
		addresses:     []string{},
		expectError:   true,
		errorContains: "at least one web listen address must be specified",
	}, {
		name:          "empty address string",
		addresses:     []string{""},
		expectError:   true,
		errorContains: "web listen address cannot be empty",
	}, {
		name:          "empty address in list",
		addresses:     []string{":8080", "", "localhost:8081"},
		expectError:   true,
		errorContains: "web listen address cannot be empty",
	}, {
		name:          "invalid port-only format (missing port)",
		addresses:     []string{":"},
		expectError:   true,
		errorContains: "port must be numeric",
	}, {
		name:          "invalid port number (too high)",
		addresses:     []string{":99999"},
		expectError:   true,
		errorContains: "port must be between 1 and 65535",
	}, {
		name:          "invalid port number (zero)",
		addresses:     []string{":0"},
		expectError:   true,
		errorContains: "port must be between 1 and 65535",
	}, {
		name:          "invalid port (non-numeric)",
		addresses:     []string{":abc"},
		expectError:   true,
		errorContains: "port must be numeric",
	}, {
		name:          "missing port in host:port format",
		addresses:     []string{"localhost"},
		expectError:   true,
		errorContains: "invalid address format",
	}, {
		name:        "empty host in host:port format",
		addresses:   []string{":8080"},
		expectError: false, // This is valid (port-only format)
	}, {
		name:          "invalid host:port format (empty host with colon)",
		addresses:     []string{"localhost:"},
		expectError:   true,
		errorContains: "port must be numeric",
	}, {
		name:          "valid address mixed with invalid",
		addresses:     []string{":8080", "invalid"},
		expectError:   true,
		errorContains: "invalid address format",
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.Web.ListenAddresses = tt.addresses

			err := cfg.Validate()

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestWebListenAddressesFlags(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		expected      []string
		expectError   bool
		errorContains string
	}{{
		name:     "default listen address",
		args:     []string{},
		expected: []string{":28282"},
	}, {
		name:     "single custom address",
		args:     []string{"--web.listen-address=:9090"},
		expected: []string{":9090"},
	}, {
		name:     "multiple addresses",
		args:     []string{"--web.listen-address=:9090", "--web.listen-address=localhost:9091"},
		expected: []string{":9090", "localhost:9091"},
	}, {
		name:          "invalid address via flag",
		args:          []string{"--web.listen-address=invalid"},
		expectError:   true,
		errorContains: "invalid address format",
	}, {
		name:          "invalid port via flag",
		args:          []string{"--web.listen-address=:99999"},
		expectError:   true,
		errorContains: "port must be between 1 and 65535",
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := kingpin.New("test", "Test application")
			updateConfig := RegisterFlags(app)

			_, parseErr := app.Parse(tt.args)
			assert.NoError(t, parseErr, "flag parsing should not fail")

			cfg := DefaultConfig()
			err := updateConfig(cfg)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, cfg.Web.ListenAddresses)
			}
		})
	}
}

func TestWebListenAddressesYAML(t *testing.T) {
	tests := []struct {
		name          string
		yamlData      string
		expected      []string
		expectError   bool
		errorContains string
	}{{
		name: "valid single address in YAML",
		yamlData: `
web:
  listenAddresses:
    - ":9090"
`,
		expected: []string{":9090"},
	}, {
		name: "valid multiple addresses in YAML",
		yamlData: `
web:
  listenAddresses:
    - ":9090"
    - "localhost:9091"
    - "192.168.1.1:9092"
`,
		expected: []string{":9090", "localhost:9091", "192.168.1.1:9092"},
	}, {
		name: "empty addresses list in YAML",
		yamlData: `
web:
  listenAddresses: []
`,
		expectError:   true,
		errorContains: "at least one web listen address must be specified",
	}, {
		name: "invalid address in YAML",
		yamlData: `
web:
  listenAddresses:
    - ":9090"
    - "invalid"
`,
		expectError:   true,
		errorContains: "invalid address format",
	}, {
		name: "addresses with whitespace in YAML",
		yamlData: `
web:
  listenAddresses:
    - "  :9090  "
    - "  localhost:9091  "
`,
		expected: []string{":9090", "localhost:9091"}, // Should be trimmed by sanitize()
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.yamlData)
			cfg, err := Load(reader)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, cfg.Web.ListenAddresses)
			}
		})
	}
}

func TestMetricsLevelYAMLMarshalling(t *testing.T) {
	tests := []struct {
		name         string
		metricsLevel Level
		expectedYAML string
	}{
		{
			name:         "All individual levels",
			metricsLevel: MetricsLevelAll,
			expectedYAML: "metricsLevel:\n    - node\n    - process\n    - container\n    - vm\n    - pod",
		},
		{
			name:         "Node only",
			metricsLevel: MetricsLevelNode,
			expectedYAML: "node",
		},
		{
			name:         "Pod and Node",
			metricsLevel: MetricsLevelPod | MetricsLevelNode,
			expectedYAML: "metricsLevel:\n    - node\n    - pod",
		},
		{
			name:         "Node and Process",
			metricsLevel: MetricsLevelNode | MetricsLevelProcess,
			expectedYAML: "metricsLevel:\n    - node\n    - process",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.Exporter.Prometheus.MetricsLevel = tt.metricsLevel

			// Marshal the prometheus exporter section
			data, err := yaml.Marshal(cfg.Exporter.Prometheus)
			assert.NoError(t, err)

			yamlStr := string(data)

			// Check that the YAML contains the expected metrics level representation
			assert.Contains(t, yamlStr, tt.expectedYAML, "YAML should contain expected metrics level representation")

			// Importantly, it should NOT contain the integer representation
			integerStr := fmt.Sprintf("metricsLevel: %d", tt.metricsLevel)
			assert.NotContains(t, yamlStr, integerStr, "YAML should not contain integer representation")

			// Test round-trip: unmarshal back and verify it's the same
			var unmarshaled PrometheusExporter
			err = yaml.Unmarshal(data, &unmarshaled)
			assert.NoError(t, err)
			assert.Equal(t, tt.metricsLevel, unmarshaled.MetricsLevel)
		})
	}
}

// TestValidateListenAddress tests the validateListenAddress function directly
func TestValidateListenAddress(t *testing.T) {
	tests := []struct {
		name          string
		addr          string
		expectError   bool
		errorContains string
	}{
		// Valid cases
		{
			name:        "valid port-only address",
			addr:        ":8080",
			expectError: false,
		},
		{
			name:        "valid host:port address",
			addr:        "localhost:8080",
			expectError: false,
		},
		{
			name:        "valid IPv4 address",
			addr:        "192.168.1.1:8080",
			expectError: false,
		},
		{
			name:        "valid IPv6 address",
			addr:        "[::1]:8080",
			expectError: false,
		},
		{
			name:        "valid IPv6 address with full notation",
			addr:        "[2001:db8::1]:8080",
			expectError: false,
		},
		{
			name:        "valid minimum port",
			addr:        ":1",
			expectError: false,
		},
		{
			name:        "valid maximum port",
			addr:        ":65535",
			expectError: false,
		},
		{
			name:        "valid 0.0.0.0 address",
			addr:        "0.0.0.0:8080",
			expectError: false,
		},
		{
			name:        "valid hostname with domain",
			addr:        "example.com:8080",
			expectError: false,
		},
		// Invalid cases - port-only format
		{
			name:          "port-only format with empty port",
			addr:          ":",
			expectError:   true,
			errorContains: "port must be numeric",
		},
		{
			name:          "port-only format with zero port",
			addr:          ":0",
			expectError:   true,
			errorContains: "port must be between 1 and 65535",
		},
		{
			name:          "port-only format with port too high",
			addr:          ":99999",
			expectError:   true,
			errorContains: "port must be between 1 and 65535",
		},
		{
			name:          "port-only format with non-numeric port",
			addr:          ":abc",
			expectError:   true,
			errorContains: "port must be numeric",
		},
		{
			name:          "port-only format with mixed alphanumeric",
			addr:          ":8080a",
			expectError:   true,
			errorContains: "port must be numeric",
		},
		// Invalid cases - host:port format
		{
			name:          "host:port format with missing port",
			addr:          "localhost",
			expectError:   true,
			errorContains: "invalid address format",
		},
		{
			name:          "host:port format with empty port",
			addr:          "localhost:",
			expectError:   true,
			errorContains: "port must be numeric",
		},
		{
			name:          "host:port format with zero port",
			addr:          "localhost:0",
			expectError:   true,
			errorContains: "port must be between 1 and 65535",
		},
		{
			name:          "host:port format with port too high",
			addr:          "localhost:99999",
			expectError:   true,
			errorContains: "port must be between 1 and 65535",
		},
		{
			name:          "host:port format with non-numeric port",
			addr:          "localhost:abc",
			expectError:   true,
			errorContains: "port must be numeric",
		},
		{
			name:          "host:port format with mixed alphanumeric port",
			addr:          "localhost:8080a",
			expectError:   true,
			errorContains: "port must be numeric",
		},
		// Edge cases
		{
			name:          "empty address",
			addr:          "",
			expectError:   true,
			errorContains: "address cannot be empty",
		},
		// Add some additional valid cases
		{
			name:        "IPv6 with proper brackets",
			addr:        "[fe80::1]:8080",
			expectError: false,
		},
		{
			name:        "IPv6 localhost with brackets",
			addr:        "[::1]:9090",
			expectError: false,
		},
		{
			name:          "IPv6 address without brackets (invalid)",
			addr:          "::1:8080",
			expectError:   true, // net.SplitHostPort requires brackets for IPv6
			errorContains: "invalid address format",
		},
		{
			name:          "IPv6 address with brackets but empty port",
			addr:          "[::1]:",
			expectError:   true,
			errorContains: "port must be numeric",
		},
		{
			name:          "IPv6 address with brackets but invalid port",
			addr:          "[::1]:abc",
			expectError:   true,
			errorContains: "port must be numeric",
		},
		{
			name:          "IPv6 address without brackets (invalid)",
			addr:          "fe80::1:8080",
			expectError:   true, // net.SplitHostPort requires brackets for IPv6
			errorContains: "invalid address format",
		},
		{
			name:          "colon without port number",
			addr:          "localhost:",
			expectError:   true,
			errorContains: "port must be numeric",
		},
		{
			name:          "only colon character",
			addr:          ":",
			expectError:   true,
			errorContains: "port must be numeric",
		},
		{
			name:        "port with leading zeros",
			addr:        ":08080",
			expectError: false, // Leading zeros are valid in our implementation
		},
		{
			name:          "very long port number",
			addr:          ":123456789",
			expectError:   true,
			errorContains: "port must be between 1 and 65535",
		},
		{
			name:          "port with special characters",
			addr:          ":80-80",
			expectError:   true,
			errorContains: "port must be numeric",
		},
		{
			name:          "port with spaces",
			addr:          ":80 80",
			expectError:   true,
			errorContains: "port must be numeric",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateListenAddress(tt.addr)

			if tt.expectError {
				assert.Error(t, err, "Expected error for address: %s", tt.addr)
				if err != nil && tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains, "Error message should contain expected text")
				}
			} else {
				assert.NoError(t, err, "Expected no error for address: %s", tt.addr)
			}
		})
	}
}

// TestValidatePort tests the validatePort function directly
func TestValidatePort(t *testing.T) {
	tests := []struct {
		name          string
		port          string
		expectError   bool
		errorContains string
	}{
		{
			name:        "valid port 8080",
			port:        "8080",
			expectError: false,
		},
		{
			name:        "valid port 1",
			port:        "1",
			expectError: false,
		},
		{
			name:        "valid port 65535",
			port:        "65535",
			expectError: false,
		},
		{
			name:        "valid port with leading zeros",
			port:        "08080",
			expectError: false,
		},
		{
			name:          "invalid port 0",
			port:          "0",
			expectError:   true,
			errorContains: "port must be between 1 and 65535",
		},
		{
			name:          "invalid port 65536",
			port:          "65536",
			expectError:   true,
			errorContains: "port must be between 1 and 65535",
		},
		{
			name:          "invalid port 99999",
			port:          "99999",
			expectError:   true,
			errorContains: "port must be between 1 and 65535",
		},
		{
			name:          "invalid port with letters",
			port:          "abc",
			expectError:   true,
			errorContains: "port must be numeric",
		},
		{
			name:          "invalid port with mixed alphanumeric",
			port:          "8080a",
			expectError:   true,
			errorContains: "port must be numeric",
		},
		{
			name:          "invalid port with special characters",
			port:          "80-80",
			expectError:   true,
			errorContains: "port must be numeric",
		},
		{
			name:          "invalid port with spaces",
			port:          "80 80",
			expectError:   true,
			errorContains: "port must be numeric",
		},
		{
			name:          "empty port",
			port:          "",
			expectError:   true,
			errorContains: "port must be numeric",
		},
		{
			name:          "very long port number",
			port:          "123456789",
			expectError:   true,
			errorContains: "port must be between 1 and 65535",
		},
		{
			name:          "negative sign in port",
			port:          "-8080",
			expectError:   true,
			errorContains: "port must be between 1 and 65535",
		},
		{
			name:          "port with decimal point",
			port:          "80.80",
			expectError:   true,
			errorContains: "port must be numeric",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePort(tt.port)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// Redfish-related tests for improved coverage

func TestDefaultRedfishConfig(t *testing.T) {
	redfish := defaultRedfishConfig()
	assert.Equal(t, ptr.To(false), redfish.Enabled)
	assert.Equal(t, 5*time.Second, redfish.HTTPTimeout)
}

func TestApplyRedfishFlags(t *testing.T) {
	tests := []struct {
		name     string
		redfish  *Redfish
		flagsSet map[string]bool
		enabled  *bool
		nodeName *string
		cfgFile  *string
		expected *Redfish
	}{{
		name:     "no flags set",
		redfish:  &Redfish{},
		flagsSet: map[string]bool{},
		enabled:  ptr.To(true),
		nodeName: ptr.To("test-node"),
		cfgFile:  ptr.To("/test/config.yaml"),
		expected: &Redfish{},
	}, {
		name:    "enabled flag set",
		redfish: &Redfish{},
		flagsSet: map[string]bool{
			ExperimentalPlatformRedfishEnabledFlag: true,
		},
		enabled:  ptr.To(true),
		nodeName: ptr.To("test-node"),
		cfgFile:  ptr.To("/test/config.yaml"),
		expected: &Redfish{
			Enabled: ptr.To(true),
		},
	}, {
		name:    "nodename flag set",
		redfish: &Redfish{},
		flagsSet: map[string]bool{
			ExperimentalPlatformRedfishNodeNameFlag: true,
		},
		enabled:  ptr.To(true),
		nodeName: ptr.To("test-node"),
		cfgFile:  ptr.To("/test/config.yaml"),
		expected: &Redfish{
			NodeName: "test-node",
		},
	}, {
		name:    "config flag set",
		redfish: &Redfish{},
		flagsSet: map[string]bool{
			ExperimentalPlatformRedfishConfigFlag: true,
		},
		enabled:  ptr.To(true),
		nodeName: ptr.To("test-node"),
		cfgFile:  ptr.To("/test/config.yaml"),
		expected: &Redfish{
			ConfigFile: "/test/config.yaml",
		},
	}, {
		name:    "all flags set",
		redfish: &Redfish{},
		flagsSet: map[string]bool{
			ExperimentalPlatformRedfishEnabledFlag:  true,
			ExperimentalPlatformRedfishNodeNameFlag: true,
			ExperimentalPlatformRedfishConfigFlag:   true,
		},
		enabled:  ptr.To(true),
		nodeName: ptr.To("test-node"),
		cfgFile:  ptr.To("/test/config.yaml"),
		expected: &Redfish{
			Enabled:    ptr.To(true),
			NodeName:   "test-node",
			ConfigFile: "/test/config.yaml",
		},
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			applyRedfishFlags(tc.redfish, tc.flagsSet, tc.enabled, tc.nodeName, tc.cfgFile)
			assert.Equal(t, tc.expected, tc.redfish)
		})
	}
}

func TestHasRedfishFlags(t *testing.T) {
	tests := []struct {
		name     string
		flagsSet map[string]bool
		expected bool
	}{{
		name:     "no redfish flags",
		flagsSet: map[string]bool{},
		expected: false,
	}, {
		name: "has enabled flag",
		flagsSet: map[string]bool{
			ExperimentalPlatformRedfishEnabledFlag: true,
		},
		expected: true,
	}, {
		name: "has nodename flag",
		flagsSet: map[string]bool{
			ExperimentalPlatformRedfishNodeNameFlag: true,
		},
		expected: true,
	}, {
		name: "has config flag",
		flagsSet: map[string]bool{
			ExperimentalPlatformRedfishConfigFlag: true,
		},
		expected: true,
	}, {
		name: "has multiple redfish flags",
		flagsSet: map[string]bool{
			ExperimentalPlatformRedfishEnabledFlag:  true,
			ExperimentalPlatformRedfishNodeNameFlag: true,
		},
		expected: true,
	}, {
		name: "has non-redfish flags only",
		flagsSet: map[string]bool{
			"some.other.flag": true,
		},
		expected: false,
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := hasRedfishFlags(tc.flagsSet)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestResolveNodeName(t *testing.T) {
	tests := []struct {
		name            string
		redfishNodeName string
		kubeNodeName    string
		expectError     bool
		errorContains   string
	}{{
		name:            "redfish node name provided",
		redfishNodeName: "redfish-node",
		kubeNodeName:    "kube-node",
		expectError:     false,
	}, {
		name:            "redfish node name with whitespace",
		redfishNodeName: "  redfish-node  ",
		kubeNodeName:    "kube-node",
		expectError:     false,
	}, {
		name:            "kube node name fallback",
		redfishNodeName: "",
		kubeNodeName:    "kube-node",
		expectError:     false,
	}, {
		name:            "kube node name with whitespace",
		redfishNodeName: "",
		kubeNodeName:    "  kube-node  ",
		expectError:     false,
	}, {
		name:            "hostname fallback",
		redfishNodeName: "",
		kubeNodeName:    "",
		expectError:     false,
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := resolveNodeName(tc.redfishNodeName, tc.kubeNodeName)

			if tc.expectError {
				assert.Error(t, err)
				if tc.errorContains != "" {
					assert.Contains(t, err.Error(), tc.errorContains)
				}
				return
			}

			assert.NoError(t, err)
			if tc.redfishNodeName != "" {
				assert.Equal(t, strings.TrimSpace(tc.redfishNodeName), result)
			} else if tc.kubeNodeName != "" {
				assert.Equal(t, strings.TrimSpace(tc.kubeNodeName), result)
			} else {
				// Should be hostname
				assert.NotEmpty(t, result)
			}
		})
	}
}

func TestResolveRedfishNodeName(t *testing.T) {
	tests := []struct {
		name         string
		redfish      *Redfish
		kubeNodeName string
		expectError  bool
	}{{
		name: "successful resolution",
		redfish: &Redfish{
			NodeName: "test-node",
		},
		kubeNodeName: "kube-node",
		expectError:  false,
	}, {
		name: "fallback to kube node name",
		redfish: &Redfish{
			NodeName: "",
		},
		kubeNodeName: "kube-node",
		expectError:  false,
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := resolveRedfishNodeName(tc.redfish, tc.kubeNodeName)

			if tc.expectError {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.NotEmpty(t, tc.redfish.NodeName)
		})
	}
}

func TestIsFeatureEnabled(t *testing.T) {
	tests := []struct {
		name     string
		config   *Config
		feature  Feature
		expected bool
	}{{
		name: "redfish feature enabled",
		config: &Config{
			Experimental: &Experimental{
				Platform: Platform{
					Redfish: Redfish{
						Enabled: ptr.To(true),
					},
				},
			},
		},
		feature:  ExperimentalRedfishFeature,
		expected: true,
	}, {
		name: "redfish feature disabled",
		config: &Config{
			Experimental: &Experimental{
				Platform: Platform{
					Redfish: Redfish{
						Enabled: ptr.To(false),
					},
				},
			},
		},
		feature:  ExperimentalRedfishFeature,
		expected: false,
	}, {
		name:     "redfish feature nil experimental",
		config:   &Config{},
		feature:  ExperimentalRedfishFeature,
		expected: false,
	}, {
		name: "prometheus feature enabled",
		config: &Config{
			Exporter: Exporter{
				Prometheus: PrometheusExporter{
					Enabled: ptr.To(true),
				},
			},
		},
		feature:  PrometheusFeature,
		expected: true,
	}, {
		name: "stdout feature enabled",
		config: &Config{
			Exporter: Exporter{
				Stdout: StdoutExporter{
					Enabled: ptr.To(true),
				},
			},
		},
		feature:  StdoutFeature,
		expected: true,
	}, {
		name: "pprof feature enabled",
		config: &Config{
			Debug: Debug{
				Pprof: PprofDebug{
					Enabled: ptr.To(true),
				},
			},
		},
		feature:  PprofFeature,
		expected: true,
	}, {
		name:     "unknown feature",
		config:   &Config{},
		feature:  Feature("unknown"),
		expected: false,
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.config.IsFeatureEnabled(tc.feature)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestApplyRedfishConfig(t *testing.T) {
	// Create a temporary config file for testing
	tmpFile, err := os.CreateTemp("", "redfish-config-*.yaml")
	assert.NoError(t, err)
	defer func() {
		_ = os.Remove(tmpFile.Name())
	}()

	// Write some dummy config content to make it a valid file
	_, err = tmpFile.WriteString("# dummy redfish config\nendpoint: https://redfish.example.com\n")
	assert.NoError(t, err)
	assert.NoError(t, tmpFile.Close())

	tests := []struct {
		name        string
		cfg         *Config
		flagsSet    map[string]bool
		enabled     *bool
		nodeName    *string
		cfgFile     *string
		expectError bool
	}{{
		name:     "no redfish flags and no experimental config",
		cfg:      &Config{},
		flagsSet: map[string]bool{},
		enabled:  ptr.To(false),
		nodeName: ptr.To("test-node"),
		cfgFile:  ptr.To(tmpFile.Name()),
	}, {
		name: "has redfish flags",
		cfg:  &Config{},
		flagsSet: map[string]bool{
			ExperimentalPlatformRedfishEnabledFlag: true,
		},
		enabled:  ptr.To(true),
		nodeName: ptr.To("test-node"),
		cfgFile:  ptr.To(tmpFile.Name()),
	}, {
		name: "experimental config exists",
		cfg: &Config{
			Experimental: &Experimental{
				Platform: Platform{
					Redfish: Redfish{
						Enabled: ptr.To(false),
					},
				},
			},
		},
		flagsSet: map[string]bool{},
		enabled:  ptr.To(false),
		nodeName: ptr.To("test-node"),
		cfgFile:  ptr.To(tmpFile.Name()),
	}, {
		name: "redfish enabled with valid config",
		cfg:  &Config{},
		flagsSet: map[string]bool{
			ExperimentalPlatformRedfishEnabledFlag: true,
			ExperimentalPlatformRedfishConfigFlag:  true,
		},
		enabled:  ptr.To(true),
		nodeName: ptr.To("test-node"),
		cfgFile:  ptr.To(tmpFile.Name()),
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := applyRedfishConfig(tc.cfg, tc.flagsSet, tc.enabled, tc.nodeName, tc.cfgFile)

			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateExperimentalConfig(t *testing.T) {
	// Create a temporary config file for testing
	tmpFile, err := os.CreateTemp("", "redfish-config-*.yaml")
	assert.NoError(t, err)
	defer func() {
		_ = os.Remove(tmpFile.Name())
	}()

	// Write some dummy config content to make it a valid file
	_, err = tmpFile.WriteString("# dummy redfish config\nendpoint: https://redfish.example.com\n")
	assert.NoError(t, err)
	assert.NoError(t, tmpFile.Close())

	tests := []struct {
		name           string
		config         *Config
		expectedErrors []string
	}{{
		name:           "no experimental config",
		config:         &Config{},
		expectedErrors: nil,
	}, {
		name: "redfish disabled",
		config: &Config{
			Experimental: &Experimental{
				Platform: Platform{
					Redfish: Redfish{
						Enabled: ptr.To(false),
					},
				},
			},
		},
		expectedErrors: nil,
	}, {
		name: "redfish enabled without config file",
		config: &Config{
			Experimental: &Experimental{
				Platform: Platform{
					Redfish: Redfish{
						Enabled:    ptr.To(true),
						ConfigFile: "",
					},
				},
			},
		},
		expectedErrors: []string{ExperimentalPlatformRedfishConfigFlag + " not supplied"},
	}, {
		name: "redfish enabled with valid config file",
		config: &Config{
			Experimental: &Experimental{
				Platform: Platform{
					Redfish: Redfish{
						Enabled:    ptr.To(true),
						ConfigFile: tmpFile.Name(),
					},
				},
			},
		},
		expectedErrors: nil,
	}, {
		name: "redfish enabled with invalid config file",
		config: &Config{
			Experimental: &Experimental{
				Platform: Platform{
					Redfish: Redfish{
						Enabled:    ptr.To(true),
						ConfigFile: "/non/existent/file.yaml",
					},
				},
			},
		},
		expectedErrors: []string{"unreadable Redfish config file"},
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			errors := tc.config.validateExperimentalConfig(nil)

			if tc.expectedErrors == nil {
				assert.Empty(t, errors)
				return
			}
			assert.NotEmpty(t, errors)
			for _, expectedErr := range tc.expectedErrors {
				found := false
				for _, actualErr := range errors {
					if strings.Contains(actualErr, expectedErr) {
						found = true
						break
					}
				}
				assert.True(t, found, "Expected error containing '%s' not found in: %v", expectedErr, errors)
			}
		})
	}
}
