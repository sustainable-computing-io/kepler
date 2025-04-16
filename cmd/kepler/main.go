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
	"github.com/sustainable-computing-io/kepler/config"
	"github.com/sustainable-computing-io/kepler/internal/device"
	"github.com/sustainable-computing-io/kepler/internal/exporter/prometheus"
	"github.com/sustainable-computing-io/kepler/internal/logger"
	"github.com/sustainable-computing-io/kepler/internal/monitor"
	"github.com/sustainable-computing-io/kepler/internal/server"
	"github.com/sustainable-computing-io/kepler/internal/service"
	"github.com/sustainable-computing-io/kepler/internal/version"
)

func main() {
	// parse args and config and exit with error if there is an error
	cfg, err := parseArgsAndConfig()
	if err != nil {
		os.Exit(1)
	}
	logger := logger.New(cfg.Log.Level, cfg.Log.Format)
	logVersionInfo(logger)
	printConfigInfo(logger, cfg)

	// create & register all services with run group
	services, err := createServices(logger, cfg)
	if err != nil {
		logger.Error("failed to create services", "error", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	var g run.Group
	for _, s := range services {
		g.Add(
			func() error {
				return s.Start(ctx)
			},
			func(err error) {
				if err != nil {
					logger.Warn("service terminated with error", "service", s.Name(), "error", err)
				}

				if cleanupErr := s.Stop(); cleanupErr != nil {
					logger.Warn("service cleanup failed with error", "service", s.Name(), "error", cleanupErr)
				}
				cancel()
			},
		)
	}
	g.Add(waitForInterrupt(ctx, logger, os.Interrupt))

	// run all groups
	logger.Info("Starting Kepler")
	if err := g.Run(); err != nil {
		logger.Error("Kepler terminated with an error", "error", err)
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

func waitForInterrupt(ctx context.Context, logger *slog.Logger, signals ...os.Signal) (func() error, func(error)) {
	ctxInternal, cancel := context.WithCancel(ctx)
	return func() error {
			c := make(chan os.Signal, 1)
			signal.Notify(c, signals...)
			logger.Info("Press Ctrl+C to shutdown")
			select {
			case <-c:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			case <-ctxInternal.Done():
				return ctxInternal.Err()
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

	logger := logger.New("info", "text")
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

func createServices(logger *slog.Logger, cfg *config.Config) ([]service.Service, error) {
	logger.Debug("Creating all services")
	pm, err := createPowerMonitor(logger, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create power monitor: %w", err)
	}

	apiServer := server.NewAPIServer(
		server.WithLogger(logger),
	)

	collectors, err := prometheus.CreateCollectors(
		pm,
		prometheus.WithLogger(logger),
		prometheus.WithProcFSPath(cfg.Host.ProcFS),
	)
	// TODO: enable exporters based on config / flags
	promExporter := prometheus.NewExporter(
		pm,
		apiServer,
		prometheus.WithLogger(logger),
		prometheus.WithCollectors(collectors),
	)

	return []service.Service{
		promExporter,
		apiServer,
		pm,
	}, nil
}

func createPowerMonitor(logger *slog.Logger, cfg *config.Config) (*monitor.PowerMonitor, error) {
	logger.Debug("Creating PowerMonitor")

	cpuPowerMeter, err := createCPUMeter(logger, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create CPU power meter: %w", err)
	}

	pm := monitor.NewPowerMonitor(
		cpuPowerMeter,
		monitor.WithLogger(logger),
	)

	return pm, nil
}

func createCPUMeter(logger *slog.Logger, cfg *config.Config) (device.CPUPowerMeter, error) {
	if fake := cfg.Dev.FakeCpuMeter; fake.Enabled {
		return device.NewFakeCPUMeter(fake.Zones, device.WithFakeLogger(logger))
	}
	return device.NewCPUPowerMeter(cfg.Host.SysFS)
}
