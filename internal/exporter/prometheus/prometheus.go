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
	collector "github.com/sustainable-computing-io/kepler/internal/exporter/prometheus/collectors"
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
}

// DefaultOpts() returns a new Opts with defaults set
func DefaultOpts() Opts {
	return Opts{
		logger: slog.Default(),
		debugCollectors: map[string]bool{
			"go": true,
		},
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

// Exporter exports power data to Prometheus
type Exporter struct {
	logger          *slog.Logger
	monitor         Monitor
	registry        *prom.Registry
	server          APIRegistry
	debugCollectors map[string]bool
}

var _ Service = (*Exporter)(nil)

// NewExporter creates a new PrometheusExporter instance
func NewExporter(pm Monitor, s APIRegistry, applyOpts ...OptionFn) *Exporter {
	opts := DefaultOpts()
	for _, apply := range applyOpts {
		apply(&opts)
	}

	monitor := &Exporter{
		monitor:         pm,
		server:          s,
		logger:          opts.logger.With("service", "prometheus"),
		debugCollectors: opts.debugCollectors,
		registry:        prom.NewRegistry(),
	}

	return monitor
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

// Start implements Exporter.Start
func (e *Exporter) Start(ctx context.Context) error {
	e.logger.Info("Starting Prometheus exporter")

	for c := range e.debugCollectors {
		collector, err := collectorForName(c)
		if err != nil {
			e.logger.Error("Error creating collector", "collector", c, "error", err)
			return err
		}
		e.logger.Info("Enabling debug collector", "collector", c)
		e.registry.MustRegister(collector)
	}

	// Register build info collector
	buildInfoCollector := collector.NewBuildInfoCollector()
	e.registry.MustRegister(buildInfoCollector)

	err := e.server.Register("/metrics", "Metrics", "Prometheus metrics",
		promhttp.HandlerFor(
			e.registry,
			promhttp.HandlerOpts{
				EnableOpenMetrics: true,
				Registry:          e.registry,
			},
		))
	if err != nil {
		return err
	}

	e.logger.Info("Prometheus exporter started running; waiting for context to be cancelled")
	<-ctx.Done()
	e.logger.Info("Prometheus exporter stopped running")
	return nil
}

// Stop implements Exporter.Stop
func (e *Exporter) Stop() error {
	// NOTE: This is a no-op since prometheus exporter makes uses of http server
	// for exporting metrics
	e.logger.Info("Stopping Prometheus exporter")
	return nil
}

// Name implements service.Name
func (e *Exporter) Name() string {
	return "prometheus"
}
