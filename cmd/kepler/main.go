// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"

	"github.com/alecthomas/kingpin/v2"
	"github.com/oklog/run"
	"github.com/sustainable-computing-io/kepler/internal/config"
	"github.com/sustainable-computing-io/kepler/internal/version"
)

func main() {
	// parse args and config and exit with error if there is an error
	cfg, err := parseArgsAndConfig()
	if err != nil {
		os.Exit(1)
	}
	logger := setupLogger(cfg.Log.Level, cfg.Log.Format)
	logVersionInfo(logger)
	printConfigInfo(logger, cfg)

	var g run.Group

	logger.Info("Starting Kepler...")
	ctx, cancel := context.WithCancel(context.Background())
	{
		g.Add(waitForInterrupt(ctx, os.Interrupt))
	}

	{
		// TODO: replace with monitor.Start()
		g.Add(
			func() error {
				logger.Info("Monitor is running. Press Ctrl+C to stop.")
				<-ctx.Done()
				logger.Info("Monitor is done running.")
				return nil
			},
			func(err error) {
				logger.Warn("Shutting down...:", "error", err)
				cancel()
			},
		)
	}

	{
		// TODO: replace with server.Start()
		g.Add(
			func() error {
				logger.Info("HTTP server is running. Press Ctrl+C to stop.")
				<-ctx.Done()
				return nil
			},
			func(err error) {
				logger.Info("HTTP Server: Shutting down...:", "error", err)
				cancel()
			},
		)
	}

	// run all groups
	if err := g.Run(); err != nil {
		logger.Warn("Kepler terminated with error: %v\n", "error", err)
		os.Exit(1)
	}
	logger.Info("Graceful shutdown completed")
}

func logVersionInfo(logger *slog.Logger) {
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

func waitForInterrupt(ctx context.Context, signals ...os.Signal) (func() error, func(error)) {
	ctx, cancel := context.WithCancel(ctx)
	return func() error {
			c := make(chan os.Signal, 1)
			signal.Notify(c, signals...)
			select {
			case <-c:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		}, func(error) {
			cancel()
		}
}

func parseArgsAndConfig() (*config.Config, error) {
	const appName = "kepler"
	app := kingpin.New(appName, "Power consumption monitoring exporter for Prometheus.")

	configFile := app.Flag("config.file", "Path to YAML configuration file").String()
	updateConfig := config.RegisterFlags(app)
	kingpin.MustParse(app.Parse(os.Args[1:]))

	logger := setupLogger("info", "text")
	cfg := config.DefaultConfig()
	if *configFile != "" {
		logger.Info("Loading configuration file", "path", *configFile)
		loadedCfg, err := config.FromFile(*configFile)
		if err != nil {
			logger.Error("Error loading config file", "error", err.Error())
			return nil, err
		}
		// Replace default config with loaded config
		cfg = loadedCfg
		logger.Info("Completed loading of configuration file", "path", *configFile)
	}

	// Apply command line flags (these override config file settings)
	if err := updateConfig(cfg); err != nil {
		logger.Error("Error applying command line flags", "error", err.Error())
		return nil, err
	}

	return cfg, nil
}

func printConfigInfo(logger *slog.Logger, cfg *config.Config) {
	if !logger.Enabled(context.Background(), slog.LevelInfo) || cfg.Log.Format == "json" {
		return
	}

	fmt.Printf(`
Configuration
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
%s
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
`, cfg)
}
