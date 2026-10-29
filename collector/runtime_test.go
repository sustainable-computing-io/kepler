// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package collector_test

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
	"github.com/sustainable-computing-io/kepler/collector"
	"github.com/sustainable-computing-io/kepler/config"
)

func runtimeConfig(t *testing.T, node string) *config.Config {
	t.Helper()
	cfg := config.DefaultConfig()
	cfg.Host.SysFS = "/sys"
	cfg.Cpu.PreferredMeters = []string{"fake"}
	cfg.Dev.FakeCpuMeter.Zones = []string{"package"}
	cfg.Kube.Node = node
	cfg.Monitor.Interval = 10 * time.Millisecond
	cfg.Monitor.Staleness = 0
	cfg.Exporter.Prometheus.MetricsLevel = config.MetricsLevelNode
	cfg.Exporter.Prometheus.DebugCollectors = nil
	return cfg
}

func register(t *testing.T, rt *collector.Runtime) *prometheus.Registry {
	t.Helper()
	registry := prometheus.NewRegistry()
	for _, c := range rt.Collectors() {
		require.NoError(t, registry.Register(c))
	}
	return registry
}

func energy(t *testing.T, registry *prometheus.Registry, node string) float64 {
	t.Helper()
	families, err := registry.Gather()
	require.NoError(t, err)
	for _, family := range families {
		if family.GetName() != "kepler_node_cpu_joules_total" {
			continue
		}
		for _, metric := range family.Metric {
			for _, label := range metric.Label {
				if label.GetName() == "node_name" {
					require.Equal(t, node, label.GetValue())
				}
			}
			return metric.GetCounter().GetValue()
		}
	}
	return 0
}

func TestRuntimeIndependentInstances(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := runtimeConfig(t, "node-a")
	a, err := collector.NewRuntime(cfg, logger)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, a.Shutdown(context.Background())) })
	cfg.Kube.Node = "node-b"
	b, err := collector.NewRuntime(cfg, logger)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, b.Shutdown(context.Background())) })
	// Mutating the source config after construction cannot affect either runtime.
	cfg.Kube.Node = "mutated"
	cfg.Cpu.PreferredMeters[0] = "invalid"
	*cfg.Kube.Enabled = true
	ra, rb := register(t, a), register(t, b)
	require.Eventually(t, func() bool { return energy(t, ra, "node-a") > 0 && energy(t, rb, "node-b") > 0 }, 3*time.Second, 10*time.Millisecond)
	before := energy(t, rb, "node-b")
	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go func() { defer wg.Done(); require.NoError(t, a.Shutdown(context.Background())) }()
	}
	wg.Wait()
	require.Eventually(t, func() bool { return energy(t, rb, "node-b") > before }, 3*time.Second, 10*time.Millisecond)
	// Callers cannot overwrite the runtime's collector slice.
	set := b.Collectors()
	set[0] = nil
	require.NotNil(t, b.Collectors()[0])
}

func TestRuntimeConstructionErrors(t *testing.T) {
	_, err := collector.NewRuntime(nil, nil)
	require.ErrorContains(t, err, "nil Kepler configuration")
	cfg := runtimeConfig(t, "test")
	cfg.Host.ProcFS = t.TempDir() + "/missing"
	_, err = collector.NewRuntime(cfg, nil)
	require.ErrorContains(t, err, "invalid procfs path")
	cfg = runtimeConfig(t, "test")
	cfg.Cpu.PreferredMeters = []string{"not-a-meter"}
	_, err = collector.NewRuntime(cfg, nil)
	require.ErrorContains(t, err, "invalid cpu.preferredMeters")
	cfg = runtimeConfig(t, "test")
	cfg.Exporter.Prometheus.DebugCollectors = []string{"invalid"}
	_, err = collector.NewRuntime(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	require.ErrorContains(t, err, "unknown debug collector")
}

func TestRuntimeDoesNotRequireHTTPConfiguration(t *testing.T) {
	cfg := runtimeConfig(t, "test")
	cfg.Web.ListenAddresses = nil
	cfg.Web.Config = "/missing/web-config.yaml"
	require.Error(t, cfg.Validate())
	rt, err := collector.NewRuntime(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	require.NoError(t, err)
	require.NoError(t, rt.Shutdown(context.Background()))
}
