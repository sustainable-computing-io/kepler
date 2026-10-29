// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package collector

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/sustainable-computing-io/kepler/config"
	"github.com/sustainable-computing-io/kepler/internal/engine"
)

// Runtime owns the services and Prometheus collectors for one Kepler instance.
// It never opens a listener or registers with the default Prometheus registry.
type Runtime struct {
	engine     *engine.Engine
	collectors []prometheus.Collector
}

// NewRuntime validates an owned configuration copy, initializes hardware and
// metadata providers, and starts background sampling. Use config.DefaultConfig
// as the starting point. HTTP, stdout and pprof settings are CLI-only; metrics
// levels and debug collectors apply to this runtime too.
func NewRuntime(cfg *config.Config, logger *slog.Logger) (*Runtime, error) {
	if logger == nil {
		logger = slog.Default()
	}
	core, err := engine.New(cfg, logger)
	if err != nil {
		return nil, err
	}
	set, err := createCollectors(core, logger)
	if err != nil {
		return nil, errors.Join(err, core.Close(context.Background()))
	}
	return &Runtime{engine: core, collectors: set}, nil
}

// Collectors returns a copy of this instance's collector set. Register it in a
// caller-owned registry and stop gathering before calling Shutdown.
func (r *Runtime) Collectors() []prometheus.Collector { return slices.Clone(r.collectors) }

// Shutdown cancels background services, waits for runners, and releases their
// dependencies. Repeated calls are safe, including after a deadline expires.
func (r *Runtime) Shutdown(ctx context.Context) error { return r.engine.Close(ctx) }

// createCollectors constructs the same collectors for CLI and library callers.
// The engine must have been initialized before metrics are gathered.
// This helper is internal to Kepler's composition; embedders use NewRuntime.
func createCollectors(core *engine.Engine, logger *slog.Logger) ([]prometheus.Collector, error) {
	cfg := core.Config
	cpuInfo, err := NewCPUInfoCollector(cfg.Host.ProcFS, cfg.Kube.Node)
	if err != nil {
		return nil, fmt.Errorf("create CPU info collector: %w", err)
	}
	set := []prometheus.Collector{
		NewKeplerBuildInfoCollector(),
		NewPowerCollector(core.Monitor, cfg.Kube.Node, logger, cfg.Exporter.Prometheus.MetricsLevel),
		cpuInfo,
		NewGPUInfoCollector(core.Monitor, cfg.Kube.Node),
	}
	if core.Platform != nil {
		platform, err := NewRedfishCollector(core.Platform, logger)
		if err != nil {
			return nil, err
		}
		set = append(set, platform)
	}
	seen := make(map[string]bool)
	for _, name := range cfg.Exporter.Prometheus.DebugCollectors {
		if seen[name] {
			continue
		}
		seen[name] = true
		switch name {
		case "go":
			set = append(set, collectors.NewGoCollector())
		case "process":
			set = append(set, collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
		default:
			return nil, fmt.Errorf("unknown debug collector: %s", name)
		}
	}
	return set, nil
}
