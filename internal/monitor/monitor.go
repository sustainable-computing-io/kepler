// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package monitor

import (
	"context"
	"log/slog"

	"github.com/sustainable-computing-io/kepler/internal/device"
	"github.com/sustainable-computing-io/kepler/internal/service"
)

// Service defines the interface for the power monitoring service
type Service interface {
	service.Service
	// Snapshot returns the current power data
	Snapshot() (*Snapshot, error)

	// DataChannel returns a channel that signals when new data is available
	DataChannel() <-chan struct{}

	// ZoneNames returns the names of the available RAPL zones
	ZoneNames() []string
}

// PowerMonitor is the default implementation of the monitoring service
type PowerMonitor struct {
	// passed externally
	logger        *slog.Logger
	cpuPowerMeter device.CPUPowerMeter

	// signals when a snapshot has been updated
	dataCh   chan struct{}
	snapshot *Snapshot
}

var _ Service = (*PowerMonitor)(nil)

type Opts struct {
	logger        *slog.Logger
	cpuPowerMeter device.CPUPowerMeter
}

// NewConfig returns a new Config with defaults set
func DefaultOpts() Opts {
	return Opts{
		logger:        slog.Default(),
		cpuPowerMeter: device.NewCPUPowerMeter(),
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

// WithCPUPowerMeter sets the logger for the PowerMonitor
func WithCPUPowerMeter(m device.CPUPowerMeter) OptionFn {
	return func(o *Opts) {
		o.cpuPowerMeter = m
	}
}

// NewPowerMonitor creates a new PowerMonitor instance
func NewPowerMonitor(applyOpts ...OptionFn) *PowerMonitor {
	opts := DefaultOpts()
	for _, apply := range applyOpts {
		apply(&opts)
	}

	monitor := &PowerMonitor{
		logger:        opts.logger.With("service", "monitor"),
		cpuPowerMeter: opts.cpuPowerMeter,
		dataCh:        make(chan struct{}, 1),
		snapshot:      NewSnapshot(),
	}

	return monitor
}

func (pm *PowerMonitor) Name() string {
	return "monitor"
}

func (pm *PowerMonitor) Start(ctx context.Context) error {
	// TODO: Implement power monitoring logic

	pm.logger.Info("Monitor is running...")
	<-ctx.Done()
	pm.logger.Info("Monitor has terminated.")

	return nil
}

func (pm *PowerMonitor) Stop() error {
	// TODO: Implement stop logic
	err := pm.cpuPowerMeter.Stop()
	return err
}

func (pm *PowerMonitor) Snapshot() (*Snapshot, error) {
	// TODO: Implement snapshot logic
	return nil, nil
}

func (pm *PowerMonitor) DataChannel() <-chan struct{} {
	return pm.dataCh
}

func (pm *PowerMonitor) ZoneNames() []string {
	// TODO: Implement zone names logic
	return []string{}
}
