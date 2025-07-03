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
	"github.com/sustainable-computing-io/kepler/internal/exporter/prometheus/metrics"
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
		assert.ErrorContains(t, cfg.Validate(), "invalid configuration: invalid monitor max terminated")

		cfg.Monitor.MaxTerminated = 0
		assert.NoError(t, cfg.Validate(), "maxTerminated=0 should be valid (unlimited)")

		cfg.Monitor.MaxTerminated = 1000
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
		name:     "invalid-max-terminated",
		args:     []string{"--monitor.max-terminated=-10"},
		expected: expect{cfgErr: fmt.Errorf("invalid configuration: invalid monitor max terminated")},
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

	t.Run("yaml-config-maxTerminated-invalid", func(t *testing.T) {
		yamlData := `
monitor:
  maxTerminated: -100
`
		reader := strings.NewReader(yamlData)
		_, err := Load(reader)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid monitor max terminated")
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
		initialLevel  metrics.Level
		setValue      string
		expectedLevel metrics.Level
		expectError   bool
		errorMessage  string
	}{
		{
			name:          "Set node from default all",
			initialLevel:  metrics.MetricsLevelNode | metrics.MetricsLevelProcess | metrics.MetricsLevelContainer | metrics.MetricsLevelVM | metrics.MetricsLevelPod,
			setValue:      "node",
			expectedLevel: metrics.MetricsLevelNode,
			expectError:   false,
		},
		{
			name:          "Set process from default all",
			initialLevel:  metrics.MetricsLevelNode | metrics.MetricsLevelProcess | metrics.MetricsLevelContainer | metrics.MetricsLevelVM | metrics.MetricsLevelPod,
			setValue:      "process",
			expectedLevel: metrics.MetricsLevelProcess,
			expectError:   false,
		},
		{
			name:          "Set container from default all",
			initialLevel:  metrics.MetricsLevelNode | metrics.MetricsLevelProcess | metrics.MetricsLevelContainer | metrics.MetricsLevelVM | metrics.MetricsLevelPod,
			setValue:      "container",
			expectedLevel: metrics.MetricsLevelContainer,
			expectError:   false,
		},
		{
			name:          "Set vm from default all",
			initialLevel:  metrics.MetricsLevelNode | metrics.MetricsLevelProcess | metrics.MetricsLevelContainer | metrics.MetricsLevelVM | metrics.MetricsLevelPod,
			setValue:      "vm",
			expectedLevel: metrics.MetricsLevelVM,
			expectError:   false,
		},
		{
			name:          "Set pod from default all",
			initialLevel:  metrics.MetricsLevelNode | metrics.MetricsLevelProcess | metrics.MetricsLevelContainer | metrics.MetricsLevelVM | metrics.MetricsLevelPod,
			setValue:      "pod",
			expectedLevel: metrics.MetricsLevelPod,
			expectError:   false,
		},
		{
			name:          "Accumulate node to existing process",
			initialLevel:  metrics.MetricsLevelProcess,
			setValue:      "node",
			expectedLevel: metrics.MetricsLevelProcess | metrics.MetricsLevelNode,
			expectError:   false,
		},
		{
			name:          "Accumulate container to existing node+process",
			initialLevel:  metrics.MetricsLevelNode | metrics.MetricsLevelProcess,
			setValue:      "container",
			expectedLevel: metrics.MetricsLevelNode | metrics.MetricsLevelProcess | metrics.MetricsLevelContainer,
			expectError:   false,
		},
		{
			name:          "Invalid level returns error",
			initialLevel:  metrics.MetricsLevelNode | metrics.MetricsLevelProcess | metrics.MetricsLevelContainer | metrics.MetricsLevelVM | metrics.MetricsLevelPod,
			setValue:      "invalid",
			expectedLevel: metrics.MetricsLevelNode | metrics.MetricsLevelProcess | metrics.MetricsLevelContainer | metrics.MetricsLevelVM | metrics.MetricsLevelPod, // Should remain unchanged
			expectError:   true,
			errorMessage:  "unknown metrics level: invalid",
		},
		{
			name:          "Empty string returns error",
			initialLevel:  metrics.MetricsLevelNode | metrics.MetricsLevelProcess | metrics.MetricsLevelContainer | metrics.MetricsLevelVM | metrics.MetricsLevelPod,
			setValue:      "",
			expectedLevel: metrics.MetricsLevelNode | metrics.MetricsLevelProcess | metrics.MetricsLevelContainer | metrics.MetricsLevelVM | metrics.MetricsLevelPod, // Should remain unchanged
			expectError:   true,
			errorMessage:  "unknown metrics level: ",
		},
		{
			name:          "Case insensitive - NODE",
			initialLevel:  metrics.MetricsLevelNode | metrics.MetricsLevelProcess | metrics.MetricsLevelContainer | metrics.MetricsLevelVM | metrics.MetricsLevelPod,
			setValue:      "NODE",
			expectedLevel: metrics.MetricsLevelNode,
			expectError:   false,
		},
		{
			name:          "Case insensitive - Process",
			initialLevel:  metrics.MetricsLevelNode | metrics.MetricsLevelProcess | metrics.MetricsLevelContainer | metrics.MetricsLevelVM | metrics.MetricsLevelPod,
			setValue:      "Process",
			expectedLevel: metrics.MetricsLevelProcess,
			expectError:   false,
		},
		{
			name:          "Whitespace handling - node with spaces",
			initialLevel:  metrics.MetricsLevelNode | metrics.MetricsLevelProcess | metrics.MetricsLevelContainer | metrics.MetricsLevelVM | metrics.MetricsLevelPod,
			setValue:      "  node  ",
			expectedLevel: metrics.MetricsLevelNode,
			expectError:   false,
		},
		{
			name:          "Set same level twice (idempotent)",
			initialLevel:  metrics.MetricsLevelNode,
			setValue:      "node",
			expectedLevel: metrics.MetricsLevelNode,
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
		initialLevel  metrics.Level
		setValues     []string
		expectedLevel metrics.Level
		expectError   bool
	}{
		{
			name:          "Accumulate multiple levels from all",
			initialLevel:  metrics.MetricsLevelNode | metrics.MetricsLevelProcess | metrics.MetricsLevelContainer | metrics.MetricsLevelVM | metrics.MetricsLevelPod,
			setValues:     []string{"node", "process"},
			expectedLevel: metrics.MetricsLevelNode | metrics.MetricsLevelProcess,
			expectError:   false,
		},
		{
			name:          "Accumulate multiple levels from none",
			initialLevel:  metrics.Level(0),
			setValues:     []string{"node", "process", "container"},
			expectedLevel: metrics.MetricsLevelNode | metrics.MetricsLevelProcess | metrics.MetricsLevelContainer,
			expectError:   false,
		},
		{
			name:          "Error in middle stops processing",
			initialLevel:  metrics.Level(0),
			setValues:     []string{"node", "invalid", "process"},
			expectedLevel: metrics.MetricsLevelNode, // Should have node from first call
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
		level    metrics.Level
		expected string
	}{
		{
			name:     "No levels (empty)",
			level:    metrics.Level(0),
			expected: "",
		},
		{
			name:     "All individual levels",
			level:    metrics.MetricsLevelNode | metrics.MetricsLevelProcess | metrics.MetricsLevelContainer | metrics.MetricsLevelVM | metrics.MetricsLevelPod,
			expected: "node,process,container,vm,pod",
		},
		{
			name:     "Single level - node",
			level:    metrics.MetricsLevelNode,
			expected: "node",
		},
		{
			name:     "Multiple levels - node and process",
			level:    metrics.MetricsLevelNode | metrics.MetricsLevelProcess,
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
	level := metrics.MetricsLevelNode | metrics.MetricsLevelProcess | metrics.MetricsLevelContainer | metrics.MetricsLevelVM | metrics.MetricsLevelPod
	mlv := NewMetricsLevelValue(&level)
	assert.True(t, mlv.IsCumulative(), "MetricsLevelValue should be cumulative")
}

func TestNewMetricsLevelValue(t *testing.T) {
	t.Run("Creates valid MetricsLevelValue", func(t *testing.T) {
		level := metrics.MetricsLevelNode | metrics.MetricsLevelProcess | metrics.MetricsLevelContainer | metrics.MetricsLevelVM | metrics.MetricsLevelPod
		mlv := NewMetricsLevelValue(&level)

		assert.NotNil(t, mlv)
		assert.Equal(t, level, *mlv.level)
	})

	t.Run("Modifying target level affects MetricsLevelValue", func(t *testing.T) {
		level := metrics.MetricsLevelNode | metrics.MetricsLevelProcess | metrics.MetricsLevelContainer | metrics.MetricsLevelVM | metrics.MetricsLevelPod
		mlv := NewMetricsLevelValue(&level)

		// Modify the original level
		level = metrics.MetricsLevelNode

		// MetricsLevelValue should reflect the change
		assert.Equal(t, metrics.MetricsLevelNode, *mlv.level)
	})
}

func TestMetricsLevelValue_CommandLineIntegration(t *testing.T) {
	// Test integration with kingpin command line parsing
	tests := []struct {
		name          string
		args          []string
		expectedLevel metrics.Level
		expectError   bool
	}{
		{
			name:          "Single flag value - node",
			args:          []string{"--metrics", "node"},
			expectedLevel: metrics.MetricsLevelNode,
			expectError:   false,
		},
		{
			name:          "Multiple flag values accumulate",
			args:          []string{"--metrics", "node", "--metrics", "process"},
			expectedLevel: metrics.MetricsLevelNode | metrics.MetricsLevelProcess,
			expectError:   false,
		},
		{
			name:          "All flag values",
			args:          []string{"--metrics", "node", "--metrics", "process", "--metrics", "container", "--metrics", "vm", "--metrics", "pod"},
			expectedLevel: metrics.MetricsLevelNode | metrics.MetricsLevelProcess | metrics.MetricsLevelContainer | metrics.MetricsLevelVM | metrics.MetricsLevelPod,
			expectError:   false,
		},
		{
			name:          "Invalid flag value",
			args:          []string{"--metrics", "invalid"},
			expectedLevel: metrics.MetricsLevelNode | metrics.MetricsLevelProcess | metrics.MetricsLevelContainer | metrics.MetricsLevelVM | metrics.MetricsLevelPod, // Should remain at default
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a kingpin application for testing
			app := kingpin.New("test", "test application")
			var metricsLevel = metrics.MetricsLevelNode | metrics.MetricsLevelProcess | metrics.MetricsLevelContainer | metrics.MetricsLevelVM | metrics.MetricsLevelPod
			app.Flag("metrics", "Metrics levels to export").SetValue(NewMetricsLevelValue(&metricsLevel))

			// Parse the arguments
			_, err := app.Parse(tt.args)

			if tt.expectError {
				assert.Error(t, err)
				// On error, the level should remain unchanged (default)
				assert.Equal(t, metrics.MetricsLevelNode|metrics.MetricsLevelProcess|metrics.MetricsLevelContainer|metrics.MetricsLevelVM|metrics.MetricsLevelPod, metricsLevel)
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
		initialLevel  metrics.Level
		setValue      string
		expectedLevel metrics.Level
		expectError   bool
	}{
		{
			name:          "Special characters in value",
			initialLevel:  metrics.MetricsLevelNode | metrics.MetricsLevelProcess | metrics.MetricsLevelContainer | metrics.MetricsLevelVM | metrics.MetricsLevelPod,
			setValue:      "node!@#",
			expectedLevel: metrics.MetricsLevelNode | metrics.MetricsLevelProcess | metrics.MetricsLevelContainer | metrics.MetricsLevelVM | metrics.MetricsLevelPod,
			expectError:   true,
		},
		{
			name:          "Numeric value",
			initialLevel:  metrics.MetricsLevelNode | metrics.MetricsLevelProcess | metrics.MetricsLevelContainer | metrics.MetricsLevelVM | metrics.MetricsLevelPod,
			setValue:      "123",
			expectedLevel: metrics.MetricsLevelNode | metrics.MetricsLevelProcess | metrics.MetricsLevelContainer | metrics.MetricsLevelVM | metrics.MetricsLevelPod,
			expectError:   true,
		},
		{
			name:          "Tab and newline whitespace",
			initialLevel:  metrics.MetricsLevelNode | metrics.MetricsLevelProcess | metrics.MetricsLevelContainer | metrics.MetricsLevelVM | metrics.MetricsLevelPod,
			setValue:      "\t\nnode\t\n",
			expectedLevel: metrics.MetricsLevelNode,
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

func TestMetricsLevelYAMLMarshalling(t *testing.T) {
	tests := []struct {
		name         string
		metricsLevel metrics.Level
		expectedYAML string
	}{
		{
			name:         "All individual levels",
			metricsLevel: metrics.MetricsLevelNode | metrics.MetricsLevelProcess | metrics.MetricsLevelContainer | metrics.MetricsLevelVM | metrics.MetricsLevelPod,
			expectedYAML: "metricsLevel:\n    - node\n    - process\n    - container\n    - vm\n    - pod",
		},
		{
			name:         "Node only",
			metricsLevel: metrics.MetricsLevelNode,
			expectedYAML: "node",
		},
		{
			name:         "Pod and Node",
			metricsLevel: metrics.MetricsLevelPod | metrics.MetricsLevelNode,
			expectedYAML: "metricsLevel:\n    - node\n    - pod",
		},
		{
			name:         "Node and Process",
			metricsLevel: metrics.MetricsLevelNode | metrics.MetricsLevelProcess,
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
