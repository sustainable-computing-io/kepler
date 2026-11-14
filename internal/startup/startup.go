// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

// Package startup contains logging helpers used while Kepler starts up.
package startup

import (
	"context"
	"fmt"
	"io"
	"log/slog"

	"github.com/sustainable-computing-io/kepler/config"
	"github.com/sustainable-computing-io/kepler/internal/version"
)

// LogConfigLoaded reports the configuration file that was successfully loaded.
// It must only be called after the effective log configuration, including CLI
// overrides, has been applied so the message uses the configured format.
func LogConfigLoaded(logger *slog.Logger, path string) {
	if path == "" {
		return
	}
	logger.Info("Loaded configuration file", "path", path)
}

// LogVersionInfo logs build information for the running Kepler binary.
func LogVersionInfo(logger *slog.Logger) {
	v := version.Info()
	logger.Info("Kepler version information",
		"version", v.Version,
		"buildTime", v.BuildTime,
		"gitBranch", v.GitBranch,
		"gitCommit", v.GitCommit,
		"goVersion", v.GoVersion,
		"goOS", v.GoOS,
		"goArch", v.GoArch,
	)
}

// PrintConfigInfo writes the human-readable configuration banner.
// JSON logs already carry structured configuration data, so the banner is
// suppressed in that format.
func PrintConfigInfo(logger *slog.Logger, cfg *config.Config, out io.Writer) {
	if !logger.Enabled(context.Background(), slog.LevelInfo) || cfg.Log.Format == "json" {
		return
	}

	fmt.Fprintf(out, `
Configuration
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
%s
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
`, cfg)
}
