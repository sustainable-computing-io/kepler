// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"syscall"

	"github.com/alecthomas/kingpin/v2"
	"github.com/sustainable-computing-io/kepler/config"
	"github.com/sustainable-computing-io/kepler/internal/device"
	"github.com/sustainable-computing-io/kepler/internal/exporter/prometheus"
	"github.com/sustainable-computing-io/kepler/internal/exporter/stdout"
	"github.com/sustainable-computing-io/kepler/internal/k8s/pod"
	"github.com/sustainable-computing-io/kepler/internal/logger"
	"github.com/sustainable-computing-io/kepler/internal/monitor"
	"github.com/sustainable-computing-io/kepler/internal/resource"
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
	logger.Debug("Creating all services")
	cpuPowerMeter, err := createCPUMeter(logger, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create CPU power meter: %w", err)
	}
	var services []service.Service

	var podInformer pod.Informer
	if *cfg.Kube.Enabled {
		podInformer = pod.NewInformer(
			pod.WithLogger(logger),
			pod.WithKubeConfig(cfg.Kube.Config),
			pod.WithNodeName(cfg.Kube.Node),
		)
		services = append(services, podInformer)
	}
	resourceInformer, err := resource.NewInformer(
		resource.WithLogger(logger),
		resource.WithProcFSPath(cfg.Host.ProcFS),
		resource.WithPodInformer(podInformer),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource informer: %w", err)
	}

	pm := monitor.NewPowerMonitor(
		cpuPowerMeter,
		monitor.WithLogger(logger),
		monitor.WithResourceInformer(resourceInformer),
		monitor.WithInterval(cfg.Monitor.Interval),
		monitor.WithMaxStaleness(cfg.Monitor.Staleness),
		monitor.WithMaxTerminated(cfg.Monitor.MaxTerminated),
		monitor.WithMinTerminatedEnergyThreshold(monitor.Energy(cfg.Monitor.MinTerminatedEnergyThreshold)*monitor.Joule),
	)

	apiServer := server.NewAPIServer(
		server.WithLogger(logger),
		server.WithListenAddress(cfg.Web.ListenAddresses),
		server.WithWebConfig(cfg.Web.Config),
	)

	services = append(services,
		resourceInformer,
		cpuPowerMeter,
		apiServer,
		pm,
	)

	// Add Prometheus exporter if enabled
	if *cfg.Exporter.Prometheus.Enabled {
		promExporter, err := createPrometheusExporter(logger, cfg, apiServer, pm)
		if err != nil {
			return nil, fmt.Errorf("failed to create Prometheus exporter: %w", err)
		}
		services = append(services, promExporter)
	}

	// Add pprof if enabled
	if *cfg.Debug.Pprof.Enabled {
		pprof := server.NewPprof(apiServer)
		services = append(services, pprof)
	}

	// Add stdout exporter if enabled
	if *cfg.Exporter.Stdout.Enabled {
		stdoutExporter := stdout.NewExporter(pm, stdout.WithLogger(logger))
		services = append(services, stdoutExporter)
	}

	return services, nil
}

func createPrometheusExporter(logger *slog.Logger, cfg *config.Config, apiServer *server.APIServer, pm *monitor.PowerMonitor) (*prometheus.Exporter, error) {
	logger.Debug("Creating Prometheus exporter")

	// Use metrics level from configuration (already parsed)
	metricsLevel := cfg.Exporter.Prometheus.MetricsLevel

	collectors, err := prometheus.CreateCollectors(
		pm,
		prometheus.WithLogger(logger),
		prometheus.WithProcFSPath(cfg.Host.ProcFS),
		prometheus.WithNodeName(cfg.Kube.Node),
		prometheus.WithMetricsLevel(metricsLevel),
	)
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

func createCPUMeter(logger *slog.Logger, cfg *config.Config) (device.CPUPowerMeter, error) {
	if fake := cfg.Dev.FakeCpuMeter; *fake.Enabled {
		return device.NewFakeCPUMeter(fake.Zones, device.WithFakeLogger(logger))
	}

	if len(cfg.Rapl.Zones) > 0 {
		logger.Info("rapl zones are filtered", "zones-enabled", cfg.Rapl.Zones)
	}

	return device.NewCPUPowerMeter(
		cfg.Host.SysFS,
		device.WithRaplLogger(logger),
		device.WithZoneFilter(cfg.Rapl.Zones),
	)
}
