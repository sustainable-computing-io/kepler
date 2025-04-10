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

	Config struct {
		Log Log `yaml:"log"`
	}
)

const (
	// Flags
	LogLevelFlag  = "log.level"
	LogFormatFlag = "log.format"
)

// DefaultConfig returns a Config with default values
func DefaultConfig() *Config {
	cfg := &Config{
		Log: Log{
			Level:  "info",
			Format: "text",
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

	return func(cfg *Config) error {
		// Logging settings
		if flagsSet[LogLevelFlag] {
			cfg.Log.Level = *logLevel
		}

		if flagsSet[LogFormatFlag] {
			cfg.Log.Format = *logFormat
		}

		cfg.sanitize()
		return cfg.Validate()
	}
}

func (c *Config) sanitize() {
	c.Log.Level = strings.TrimSpace(c.Log.Level)
	c.Log.Format = strings.TrimSpace(c.Log.Format)
}

// Validate checks for configuration errors
func (c *Config) Validate() error {
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

	if len(errs) > 0 {
		return fmt.Errorf("invalid configuration: %s", strings.Join(errs, ", "))
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
