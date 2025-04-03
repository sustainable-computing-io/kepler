package monitor

import (
	"context"
	"log/slog"
)

// Service defines the interface for the power monitoring service
type Service interface {
	// Start begins power monitoring using the provided context
	Start(ctx context.Context) error

	// Stop ends the power monitoring
	Stop() error

	// Snapshot returns the current power data
	Snapshot() (*Snapshot, error)

	// DataChannel returns a channel that signals when new data is available
	DataChannel() <-chan struct{}

	// ZoneNames returns the names of the available RAPL zones
	ZoneNames() []string
}

// PowerMonitor is the default implementation of the power monitoring service (monitor.Service)
type PowerMonitor struct {
	// inputs
	logger slog.Logger

	dataCh   chan struct{}
	snapshot *Snapshot
}

var _ Service = (*PowerMonitor)(nil)

// OptionFn is a function that configures a PowerMonitor
type OptionFn func(*PowerMonitor)

// NewPowerMonitor creates a new PowerMonitor instance
func NewPowerMonitor(logger *slog.Logger, options ...OptionFn) *PowerMonitor {
	monitor := &PowerMonitor{
		logger:   *logger,
		dataCh:   make(chan struct{}, 1),
		snapshot: NewSnapshot(),
	}

	// Apply options
	for _, option := range options {
		option(monitor)
	}

	return monitor
}

func (pm *PowerMonitor) Start(ctx context.Context) error {
	// TODO: Implement power monitoring logic
	pm.logger.Info("Monitor is running. Press Ctrl+C to stop.")
	<-ctx.Done()
	pm.logger.Info("Monitor is done running.")
	return nil
}

func (pm *PowerMonitor) Stop() error {
	// TODO: Implement stop logic
	return nil
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
