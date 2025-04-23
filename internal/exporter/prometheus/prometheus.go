// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package prometheus

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	prom "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	collector "github.com/sustainable-computing-io/kepler/internal/exporter/prometheus/collector"
	"github.com/sustainable-computing-io/kepler/internal/monitor"
	"github.com/sustainable-computing-io/kepler/internal/service"
)

type (
	Service = service.Service
	Monitor = monitor.Service
)

type APIRegistry interface {
	Register(endpoint, summary, description string, handler http.Handler) error
}

type Opts struct {
	logger          *slog.Logger
	debugCollectors map[string]bool
	collectors      map[string]prom.Collector
	procfs          string
	sysfs           string
}

// DefaultOpts() returns a new Opts with defaults set
func DefaultOpts() Opts {
	return Opts{
		logger: slog.Default(),
		debugCollectors: map[string]bool{
			"go": true,
		},
		collectors: map[string]prom.Collector{},
	}
}

// OptionFn is a function sets one more more options in Opts struct
type OptionFn func(*Opts)

// WithLogger sets the logger for the PowerMonitor
func WithLogger(logger *slog.Logger) OptionFn {
	return func(o *Opts) {
		o.logger = logger
	}
}

// WithDebugCollectors sets the debug collectors
func WithDebugCollectors(c *[]string) OptionFn {
	return func(o *Opts) {
		for _, name := range *c {
			o.debugCollectors[name] = true
		}
	}
}

func WithProcFSPath(procfs string) OptionFn {
	return func(o *Opts) {
		o.procfs = procfs
	}
}

func WithSysFSPath(sysfs string) OptionFn {
	return func(o *Opts) {
		o.sysfs = sysfs
	}
}

func WithCollectors(c map[string]prom.Collector) OptionFn {
	return func(o *Opts) {
		o.collectors = c
	}
}

// Exporter exports power data to Prometheus
type Exporter struct {
	logger          *slog.Logger
	monitor         Monitor
	registry        *prom.Registry
	server          APIRegistry
	debugCollectors map[string]bool
	collectors      map[string]prom.Collector
}

var _ Service = (*Exporter)(nil)

// NewExporter creates a new PrometheusExporter instance
func NewExporter(pm Monitor, s APIRegistry, applyOpts ...OptionFn) *Exporter {
	opts := DefaultOpts()
	for _, apply := range applyOpts {
		apply(&opts)
	}

	exporter := &Exporter{
		monitor:         pm,
		server:          s,
		logger:          opts.logger.With("service", "prometheus"),
		debugCollectors: opts.debugCollectors,
		collectors:      opts.collectors,
		registry:        prom.NewRegistry(),
	}

	return exporter
}

func collectorForName(name string) (prom.Collector, error) {
	switch name {
	case "go":
		return collectors.NewGoCollector(), nil
	case "process":
		return collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}), nil
	default:
		return nil, fmt.Errorf("unknown collector: %s", name)
	}
}

func CreateCollectors(pm Monitor, applyOpts ...OptionFn) (map[string]prom.Collector, error) {
	opts := Opts{
		logger: slog.Default(),
		procfs: "/proc",
		sysfs:  "/sys",
	}
	for _, apply := range applyOpts {
		apply(&opts)
	}
	collectors := map[string]prom.Collector{
		"build_info": collector.NewKeplerBuildInfoCollector(),
		"power":      collector.NewPowerCollector(pm, opts.logger),
	}
	cpuInfoCollector, err := collector.NewCPUInfoCollector(opts.procfs)
	if err != nil {
		return nil, err
	}
	collectors["cpu_info"] = cpuInfoCollector
	energyZoneCollector, err := collector.NewEnergyZoneCollector(opts.sysfs)
	if err != nil {
		return nil, err
	}
	collectors["energy_zone"] = energyZoneCollector
	return collectors, nil
}

func (e *Exporter) Init(ctx context.Context) error {
	e.logger.Info("Initializing Prometheus exporter")
	for c := range e.debugCollectors {
		collector, err := collectorForName(c)
		if err != nil {
			e.logger.Error("Error creating collector", "collector", c, "error", err)
			return err
		}
		e.logger.Info("Enabling debug collector", "collector", c)
		e.registry.MustRegister(collector)
	}

	for name, collector := range e.collectors {
		e.logger.Info("Enabling collector", "collector", name)
		e.registry.MustRegister(collector)
	}

	err := e.server.Register("/metrics", "Metrics", "Prometheus metrics",
		promhttp.HandlerFor(
			e.registry,
			promhttp.HandlerOpts{
				EnableOpenMetrics: true,
				Registry:          e.registry,
			},
		))
	return err
}

// Name implements service.Name
func (e *Exporter) Name() string {
	return "prometheus"
}
