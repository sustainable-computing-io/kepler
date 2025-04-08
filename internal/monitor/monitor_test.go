// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package monitor

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewPowerMonitor(t *testing.T) {
	tests := []struct {
		name string
		opts []OptionFn
		want string
	}{
		{
			name: "default options",
			opts: []OptionFn{},
			want: "monitor",
		},
		{
			name: "with logger",
			opts: []OptionFn{
				WithLogger(slog.Default().With("test", "custom")),
			},
			want: "monitor",
		},
		{
			name: "with custom power meter",
			opts: []OptionFn{
				func() OptionFn {
					mockPowerMeter := new(MockCPUPowerMeter)
					mockPowerMeter.On("Name").Return("mock-cpu")
					return WithCPUPowerMeter(mockPowerMeter)
				}(),
			},
			want: "monitor",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			monitor := NewPowerMonitor(tt.opts...)

			// Check if monitor is correctly initialized
			assert.NotNil(t, monitor)
			assert.Equal(t, tt.want, monitor.Name())
			assert.NotNil(t, monitor.dataCh)
			assert.NotNil(t, monitor.snapshot)
			assert.NotNil(t, monitor.logger)
			assert.NotNil(t, monitor.cpuPowerMeter)
		})
	}
}

func TestPowerMonitor_Start(t *testing.T) {
	mockPowerMeter := &MockCPUPowerMeter{}

	// TODO: Since the current monitor doesn't actually call these methods,
	// we don't set any expectations on mock
	// FIX: Implement a mock implementation that actually calls the methods
	// call: assert.Expectations(t)

	monitor := NewPowerMonitor(
		WithCPUPowerMeter(mockPowerMeter),
		WithLogger(slog.Default()),
	)

	// Create a context with a short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Start the monitor and verify it blocks until context cancellation
	startTime := time.Now()
	err := monitor.Start(ctx)
	duration := time.Since(startTime)

	// Verify the method returns when context is done and there's no error
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, duration, 50*time.Millisecond, "Start should block until context is done")
}

func TestPowerMonitor_Stop(t *testing.T) {
	mockPowerMeter := &MockCPUPowerMeter{}
	mockPowerMeter.On("Stop").Return(nil)

	monitor := NewPowerMonitor(
		WithCPUPowerMeter(mockPowerMeter),
	)

	err := monitor.Stop()
	assert.NoError(t, err)
	mockPowerMeter.AssertExpectations(t)
}

func TestPowerMonitor_DataChannel(t *testing.T) {
	monitor := NewPowerMonitor()

	dataCh := monitor.DataChannel()
	assert.NotNil(t, dataCh)
}

func TestPowerMonitor_ZoneNames(t *testing.T) {
	mockPowerMeter := &MockCPUPowerMeter{}
	monitor := NewPowerMonitor(
		WithCPUPowerMeter(mockPowerMeter),
	)
	// TODO: Implement zone names validation

	names := monitor.ZoneNames()
	assert.Empty(t, names)
}

func TestPowerMonitor_Snapshot(t *testing.T) {
	mockPowerMeter := new(MockCPUPowerMeter)

	monitor := NewPowerMonitor(
		WithCPUPowerMeter(mockPowerMeter),
	)

	// TODO: Implement snapshot validation
	snapshot, err := monitor.Snapshot()
	assert.Nil(t, snapshot)
	assert.Nil(t, err)
}
