// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"syscall"

	"github.com/alecthomas/kingpin/v2"

	"github.com/sustainable-computing-io/kepler/config"
	"github.com/sustainable-computing-io/kepler/internal/engine"
	"github.com/sustainable-computing-io/kepler/internal/exporter/prometheus"
	"github.com/sustainable-computing-io/kepler/internal/exporter/stdout"
	"github.com/sustainable-computing-io/kepler/internal/logger"
	"github.com/sustainable-computing-io/kepler/internal/monitor"
	"github.com/sustainable-computing-io/kepler/internal/platform/redfish"
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

	// Configure logger - use stderr if stdout exporter is enabled to prevent output interleaving
	logOut := os.Stdout
	if *cfg.Exporter.Stdout.Enabled {
		logOut = os.Stderr
	}
	logger := logger.New(cfg.Log.Level, cfg.Log.Format, logOut)

	logVersionInfo(logger)
	printConfigInfo(logger, cfg)

	cfg.ApplyCpuMeterDeprecations(logger)
	cfg.LogUnknownFields(logger)

	services, err := createServices(logger, cfg)
	if err != nil {
		logger.Error("failed to create services", "error", err)
		os.Exit(1)
	}

	sh := service.NewSignalHandler(syscall.SIGINT, syscall.SIGTERM)
	services = append(services, sh)

	if err = service.Init(logger, services); err != nil {
		logger.Error("failed to initialize services", "error", err)
		os.Exit(1)
	}

	logger.Info("Starting Kepler")

	if err := service.Run(context.Background(), logger, services); err != nil {
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

func parseArgsAndConfig() (*config.Config, error) {
	const appName = "kepler"
	app := kingpin.New(appName, "Power consumption monitoring exporter for Prometheus.")

	configFile := app.Flag("config.file", "Path to YAML configuration file").String()
	updateConfig := config.RegisterFlags(app)
	kingpin.MustParse(app.Parse(os.Args[1:]))

	logger := logger.New("info", "text", os.Stdout)
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
	core, err := engine.New(cfg, logger)
	if err != nil {
		return nil, err
	}
	cfg = core.Config
	apiServer := server.NewAPIServer(
		server.WithLogger(logger),
		server.WithListenAddress(cfg.Web.ListenAddresses),
		server.WithWebConfig(cfg.Web.Config),
	)

	services := []service.Service{core, apiServer}
	pm, redfishService := core.Monitor, core.Platform

	// Add Prometheus exporter if enabled
	if cfg.IsFeatureEnabled(config.PrometheusFeature) {
		promExporter, err := createPrometheusExporter(logger, cfg, apiServer, pm, redfishService)
		if err != nil {
			return nil, errors.Join(fmt.Errorf("failed to create Prometheus exporter: %w", err), core.Shutdown())
		}
		services = append(services, promExporter)
	}

	// Add pprof if enabled
	if cfg.IsFeatureEnabled(config.PprofFeature) {
		pprof := server.NewPprof(apiServer)
		services = append(services, pprof)
	}

	// Add stdout exporter if enabled
	if cfg.IsFeatureEnabled(config.StdoutFeature) {
		stdoutExporter := stdout.NewExporter(pm, stdout.WithLogger(logger))
		services = append(services, stdoutExporter)
	}

	// created last so that it observes every other service
	services = append(services, server.NewProbe(logger, apiServer, services))

	return services, nil
}

func createPrometheusExporter(
	logger *slog.Logger, cfg *config.Config,
	apiServer *server.APIServer, pm *monitor.PowerMonitor,
	rs *redfish.Service,
) (*prometheus.Exporter, error) {
	logger.Debug("Creating Prometheus exporter")

	// Use metrics level from configuration (already parsed)
	metricsLevel := cfg.Exporter.Prometheus.MetricsLevel

	var collectorOpts []prometheus.OptionFn
	collectorOpts = append(collectorOpts,
		prometheus.WithLogger(logger),
		prometheus.WithProcFSPath(cfg.Host.ProcFS),
		prometheus.WithNodeName(cfg.Kube.Node),
		prometheus.WithMetricsLevel(metricsLevel),
	)

	// Add platform data provider if Redfish service is available
	if rs != nil {
		collectorOpts = append(collectorOpts, prometheus.WithPlatformDataProvider(rs))
	}

	collectors, err := prometheus.CreateCollectors(pm, collectorOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create Prometheus collectors: %w", err)
	}

	debugCollectors := cfg.Exporter.Prometheus.DebugCollectors

	promExporter := prometheus.NewExporter(
		pm,
		apiServer,
		prometheus.WithLogger(logger),
		prometheus.WithCollectors(collectors),
		prometheus.WithDebugCollectors(debugCollectors),
	)

	return promExporter, nil
}
