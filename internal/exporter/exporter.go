/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package exporter

import (
	"context"
	"log/slog"
)

// Exporter interface defines the methods required for power data exporters
type Exporter interface {
	// Start initializes and starts the exporter
	Start(ctx context.Context) error

	// Stop gracefully shuts down the exporter
	Stop() error

	// Name returns the name of the exporter
	Name() string
}

// PrometheusConfig contains configuration options for the Prometheus exporter
type PrometheusConfig struct {
	// PowerMonitor is the source of power data
	PowerMonitor PowerMonitor

	// Logger is the logger to use for exporter messages
	Logger *slog.Logger

	// DebugCollectors is a list of debugging information collectors to enable
	DebugCollectors []string
}

// PowerMonitor is an interface for accessing power monitoring data
// This is a simplified version of monitor.PowerMonitorService for exporter use
type PowerMonitor interface {
	// Snapshot returns the current power data
	Snapshot() interface{}

	// DataChannel returns a channel that signals when new data is available
	DataChannel() <-chan struct{}

	// ZoneNames returns the names of the RAPL zones
	ZoneNames() []string
}

// PrometheusExporter exports power data to Prometheus
type PrometheusExporter struct {
	config       PrometheusConfig
	powerMonitor PowerMonitor
	logger       *slog.Logger
}

// NewPrometheusExporter creates a new Prometheus exporter
func NewPrometheusExporter(config PrometheusConfig) *PrometheusExporter {
	return &PrometheusExporter{
		config:       config,
		powerMonitor: config.PowerMonitor,
		logger:       config.Logger,
	}
}

// Start implements Exporter.Start
func (e *PrometheusExporter) Start(ctx context.Context) error {
	// TODO: Implement Prometheus exporter startup logic
	return nil
}

// Stop implements Exporter.Stop
func (e *PrometheusExporter) Stop() error {
	// TODO: Implement Prometheus exporter shutdown logic
	return nil
}

// Name implements Exporter.Name
func (e *PrometheusExporter) Name() string {
	return "prometheus"
}

// StdoutConfig contains configuration options for the stdout exporter
type StdoutConfig struct {
	// PowerMonitor is the source of power data
	PowerMonitor PowerMonitor

	// MonitorInterval is the interval at which data is collected
	MonitorInterval *int64

	// ProcessLimit limits the number of processes to display (0 = all)
	ProcessLimit *int

	// Logger is the logger to use for exporter messages
	Logger *slog.Logger
}

// StdoutExporter exports power data to stdout
type StdoutExporter struct {
	config       StdoutConfig
	powerMonitor PowerMonitor
	logger       *slog.Logger
	processLimit int
}

// NewStdoutExporter creates a new stdout exporter
func NewStdoutExporter(config StdoutConfig) *StdoutExporter {
	processLimit := 0
	if config.ProcessLimit != nil {
		processLimit = *config.ProcessLimit
	}

	return &StdoutExporter{
		config:       config,
		powerMonitor: config.PowerMonitor,
		logger:       config.Logger,
		processLimit: processLimit,
	}
}

// Start implements Exporter.Start
func (e *StdoutExporter) Start(ctx context.Context) error {
	// TODO: Implement stdout exporter startup logic
	return nil
}

// Stop implements Exporter.Stop
func (e *StdoutExporter) Stop() error {
	// TODO: Implement stdout exporter shutdown logic
	return nil
}

// Name implements Exporter.Name
func (e *StdoutExporter) Name() string {
	return "stdout"
}
