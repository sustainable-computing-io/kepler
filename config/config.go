// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/alecthomas/kingpin/v2"
	"gopkg.in/yaml.v3"
)

// Config represents the complete application configuration
type (
	Log struct {
		Level  string `yaml:"level"`
		Format string `yaml:"format"`
	}
	Host struct {
		SysFS  string `yaml:"sysfs"`
		ProcFS string `yaml:"procfs"`
	}

	// Rapl configuration
	Rapl struct {
		Zones []string `yaml:"zones"`
	}

	// Development mode settings; disabled by default
	Dev struct {
		FakeCpuMeter struct {
			Enabled bool     `yaml:"enabled"`
			Zones   []string `yaml:"zones"`
		} `yaml:"fake-cpu-meter"`
	}

	Config struct {
		Log         Log  `yaml:"log"`
		Host        Host `yaml:"host"`
		Dev         Dev  `yaml:"dev"` // WARN: do not expose dev settings as flags
		EnablePprof bool `yaml:"enable-pprof"`
		Rapl        Rapl `yaml:"rapl"`
	}
)

type SkipValidation int

const (
	SkipHostValidation SkipValidation = 1
)

const (
	// Flags
	LogLevelFlag  = "log.level"
	LogFormatFlag = "log.format"

	HostSysFSFlag  = "host.sysfs"
	HostProcFSFlag = "host.procfs"

	EnablePprofFlag = "enable.pprof"

// WARN:  dev settings shouldn't be exposed as flags as flags are intended for end users
)

// DefaultConfig returns a Config with default values
func DefaultConfig() *Config {
	cfg := &Config{
		Log: Log{
			Level:  "info",
			Format: "text",
		},
		Host: Host{
			SysFS:  "/sys",
			ProcFS: "/proc",
		},
		Rapl: Rapl{
			Zones: []string{},
		},
	}

	return cfg
}

// Load loads configuration from an io.Reader
func Load(r io.Reader) (*Config, error) {
	cfg := DefaultConfig()

	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}
	cfg.sanitize()

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// FromFile loads configuration from a file
func FromFile(filePath string) (*Config, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file: %w", err)
	}
	defer file.Close()

	return Load(file)
}

type ConfigUpdaterFn func(*Config) error

// RegisterFlags registers command-line flags with kingpin app
// and returns ConfigUpdaterFn that updates the config from parsed flags
// as command line arguments override config file settings
func RegisterFlags(app *kingpin.Application) ConfigUpdaterFn {
	// track flags that were explicitly set
	flagsSet := map[string]bool{}

	app.PreAction(func(ctx *kingpin.ParseContext) error {
		// Clear the map in case this function is called multiple times
		flagsSet = map[string]bool{}

		for _, element := range ctx.Elements {
			if flag, ok := element.Clause.(*kingpin.FlagClause); ok && element.Value != nil {
				flagsSet[flag.Model().Name] = true
			}
		}
		return nil
	})

	// Logging
	logLevel := app.Flag(LogLevelFlag, "Logging level: debug, info, warn, error").Default("info").Enum("debug", "info", "warn", "error")
	logFormat := app.Flag(LogFormatFlag, "Logging format: text or json").Default("text").Enum("text", "json")
	hostSysFS := app.Flag(HostSysFSFlag, "Host sysfs path").Default("/sys").ExistingDir()
	hostProcFS := app.Flag(HostProcFSFlag, "Host procfs path").Default("/proc").ExistingDir()
	enablePprof := app.Flag(EnablePprofFlag, "Enable pprof").Default("false").Bool()

	return func(cfg *Config) error {
		// Logging settings
		if flagsSet[LogLevelFlag] {
			cfg.Log.Level = *logLevel
		}

		if flagsSet[LogFormatFlag] {
			cfg.Log.Format = *logFormat
		}

		if flagsSet[HostSysFSFlag] {
			cfg.Host.SysFS = *hostSysFS
		}

		if flagsSet[HostProcFSFlag] {
			cfg.Host.ProcFS = *hostProcFS
		}

		if flagsSet[EnablePprofFlag] {
			cfg.EnablePprof = *enablePprof
		}

		cfg.sanitize()
		return cfg.Validate()
	}
}

func (c *Config) sanitize() {
	c.Log.Level = strings.TrimSpace(c.Log.Level)
	c.Log.Format = strings.TrimSpace(c.Log.Format)
	c.Host.SysFS = strings.TrimSpace(c.Host.SysFS)
	c.Host.ProcFS = strings.TrimSpace(c.Host.ProcFS)

	for i := range c.Rapl.Zones {
		c.Rapl.Zones[i] = strings.TrimSpace(c.Rapl.Zones[i])
	}
}

// Validate checks for configuration errors
func (c *Config) Validate(skips ...SkipValidation) error {
	validationSkipped := make(map[SkipValidation]bool, len(skips))
	for _, v := range skips {
		validationSkipped[v] = true
	}
	var errs []string
	{ // log level

		validLogLevels := map[string]bool{
			"debug": true,
			"info":  true,
			"warn":  true,
			"error": true,
		}

		// Validate logging settings
		if _, valid := validLogLevels[c.Log.Level]; !valid {
			errs = append(errs, fmt.Sprintf("invalid log level: %s", c.Log.Level))
		}
	}
	{ // log format
		validFormats := map[string]bool{
			"text": true,
			"json": true,
		}
		if _, valid := validFormats[c.Log.Format]; !valid {
			errs = append(errs, fmt.Sprintf("invalid log format: %s", c.Log.Format))
		}
	}

	{ // Validate host settings
		if _, skip := validationSkipped[SkipHostValidation]; !skip {
			if err := canReadDir(c.Host.SysFS); err != nil {
				errs = append(errs, fmt.Sprintf("invalid sysfs path: %s: %s ", c.Host.SysFS, err.Error()))
			}
			if err := canReadDir(c.Host.ProcFS); err != nil {
				errs = append(errs, fmt.Sprintf("invalid procfs path: %s: %s ", c.Host.ProcFS, err.Error()))
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("invalid configuration: %s", strings.Join(errs, ", "))
	}

	return nil
}

func canReadDir(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}

	defer func() {
		// ignored on purpose
		_ = f.Close()
	}()

	_, err = f.ReadDir(1)
	if err != nil {
		return err
	}

	return nil
}

func (c *Config) String() string {
	bytes, err := yaml.Marshal(c)
	if err == nil {
		return string(bytes)
	}
	// NOTE:  this code path should not happen but if it does (i.e if yaml marshal) fails
	// for some reason, manually build the string
	return c.manualString()
}

func (c *Config) manualString() string {
	cfgs := []struct {
		Name  string
		Value string
	}{
		{LogLevelFlag, c.Log.Level},
		{LogFormatFlag, c.Log.Format},
		{HostSysFSFlag, c.Host.SysFS},
		{HostProcFSFlag, c.Host.ProcFS},
		{"rapl.zones", strings.Join(c.Rapl.Zones, ", ")},
	}
	sb := strings.Builder{}

	for _, cfg := range cfgs {
		sb.WriteString(cfg.Name)
		sb.WriteString(": ")
		sb.WriteString(cfg.Value)
		sb.WriteString("\n")
	}

	return sb.String()
}
