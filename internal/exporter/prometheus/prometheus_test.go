// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package prometheus

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"testing"
	"time"

	prom "github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/sustainable-computing-io/kepler/internal/monitor"
)

// MockMonitor mocks the Monitor interface
type MockMonitor struct {
	mock.Mock
}

func (m *MockMonitor) Start(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockMonitor) Stop() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockMonitor) Name() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockMonitor) Snapshot() (*monitor.Snapshot, error) {
	args := m.Called()
	if s := args.Get(0); s != nil {
		return s.(*monitor.Snapshot), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockMonitor) DataChannel() <-chan struct{} {
	args := m.Called()
	return args.Get(0).(<-chan struct{})
}

func (m *MockMonitor) ZoneNames() []string {
	args := m.Called()
	return args.Get(0).([]string)
}

// MockAPIRegistry mocks the APIRegistry interface
type MockAPIRegistry struct {
	mock.Mock
}

func (m *MockAPIRegistry) Register(endpoint, summary, description string, handler http.Handler) error {
	args := m.Called(endpoint, summary, description, handler)
	return args.Error(0)
}

func TestNewExporter(t *testing.T) {
	tests := []struct {
		name          string
		opts          []OptionFn
		expectService string
	}{{
		name:          "default options",
		opts:          []OptionFn{},
		expectService: "prometheus",
	}, {
		name: "with custom logger",
		opts: []OptionFn{
			WithLogger(slog.Default().With("test", "custom")),
		},
		expectService: "prometheus",
	}, {
		name: "with debug collectors",
		opts: []OptionFn{
			WithDebugCollectors(&[]string{"go", "process"}),
		},
		expectService: "prometheus",
	}, {
		name: "with multiple options",
		opts: []OptionFn{
			WithLogger(slog.Default().With("test", "custom")),
			WithDebugCollectors(&[]string{"process"}),
		},
		expectService: "prometheus",
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockMonitor := new(MockMonitor)
			mockRegistry := new(MockAPIRegistry)

			exporter := NewExporter(mockMonitor, mockRegistry, tt.opts...)

			assert.NotNil(t, exporter)
			assert.Equal(t, tt.expectService, exporter.Name())
			assert.NotNil(t, exporter.logger)
			assert.NotNil(t, exporter.registry)
			assert.Same(t, mockMonitor, exporter.monitor)
			assert.Same(t, mockRegistry, exporter.server)
		})
	}
}

func TestExporter_Name(t *testing.T) {
	mockMonitor := &MockMonitor{}
	mockRegistry := &MockAPIRegistry{}

	exporter := NewExporter(mockMonitor, mockRegistry)

	assert.Equal(t, "prometheus", exporter.Name())
}

func TestExporter_Start(t *testing.T) {
	t.Run("starts successfully", func(t *testing.T) {
		mockMonitor := &MockMonitor{}
		mockRegistry := &MockAPIRegistry{}

		// Setup the mock expectations
		mockRegistry.On("Register", "/metrics", "Metrics", "Prometheus metrics", mock.Anything).Return(nil)

		exporter := NewExporter(mockMonitor, mockRegistry)

		// Create context with timeout to ensure the test doesn't hang
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		// Start in a goroutine because it will block until context is done
		errCh := make(chan error)
		go func() {
			errCh <- exporter.Start(ctx)
		}()

		// Wait for timeout or error
		select {
		case err := <-errCh:
			assert.NoError(t, err)
		case <-time.After(200 * time.Millisecond):
			t.Fatal("Start didn't return after context was cancelled")
		}

		mockRegistry.AssertExpectations(t)
	})

	t.Run("registry returns error", func(t *testing.T) {
		mockMonitor := &MockMonitor{}
		mockRegistry := &MockAPIRegistry{}

		// Setup the mock to return an error
		expectedErr := errors.New("register error")
		mockRegistry.On("Register", "/metrics", "Metrics", "Prometheus metrics", mock.Anything).Return(expectedErr)

		exporter := NewExporter(mockMonitor, mockRegistry)

		// Start with a context - should return the error immediately
		ctx := context.Background()
		err := exporter.Start(ctx)

		assert.Error(t, err)
		assert.Equal(t, expectedErr, err)
		mockRegistry.AssertExpectations(t)
	})

	t.Run("with invalid collector", func(t *testing.T) {
		mockMonitor := &MockMonitor{}
		mockRegistry := &MockAPIRegistry{}

		// Create an exporter with an unknown collector
		exporter := NewExporter(
			mockMonitor,
			mockRegistry,
			WithDebugCollectors(&[]string{"unknown_collector"}),
		)

		// Start should return an error
		ctx := context.Background()
		err := exporter.Start(ctx)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unknown collector: unknown_collector")
		mockRegistry.AssertNotCalled(t, "Register")
	})

	t.Run("with multiple valid collectors", func(t *testing.T) {
		mockMonitor := &MockMonitor{}
		mockRegistry := &MockAPIRegistry{}

		mockRegistry.On("Register", "/metrics", "Metrics", "Prometheus metrics", mock.Anything).Return(nil)

		// Create an exporter with multiple valid collectors
		exporter := NewExporter(
			mockMonitor,
			mockRegistry,
			WithDebugCollectors(&[]string{"go", "process"}),
		)

		// Create context with immediate cancellation
		ctx, cancel := context.WithCancel(context.Background())

		// Start in goroutine and cancel immediately
		go func() {
			time.Sleep(10 * time.Millisecond)
			cancel()
		}()

		err := exporter.Start(ctx)

		assert.NoError(t, err)
		mockRegistry.AssertExpectations(t)
	})
}

func TestExporter_Stop(t *testing.T) {
	mockMonitor := &MockMonitor{}
	mockRegistry := &MockAPIRegistry{}

	exporter := NewExporter(mockMonitor, mockRegistry)

	// Stop should return nil since it's a no-op in the implementation
	err := exporter.Stop()
	assert.NoError(t, err)
}

func TestCollectorForName(t *testing.T) {
	tests := []struct {
		name          string
		collectorName string
		expectError   bool
	}{{
		name:          "go collector",
		collectorName: "go",
		expectError:   false,
	}, {
		name:          "process collector",
		collectorName: "process",
		expectError:   false,
	}, {
		name:          "unknown collector",
		collectorName: "unknown",
		expectError:   true,
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			collector, err := collectorForName(tt.collectorName)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, collector)
				assert.Contains(t, err.Error(), "unknown collector: "+tt.collectorName)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, collector)

				// Further verify the collector type
				switch tt.collectorName {
				case "go":
					// Check that it's registered correctly with a registry
					registry := prom.NewRegistry()
					err := registry.Register(collector)
					assert.NoError(t, err)
				case "process":
					// Check that it's registered correctly with a registry
					registry := prom.NewRegistry()
					err := registry.Register(collector)
					assert.NoError(t, err)
				}
			}
		})
	}
}

func TestWithOptions(t *testing.T) {
	t.Run("WithLogger", func(t *testing.T) {
		customLogger := slog.Default().With("custom", "logger")
		opts := DefaultOpts()

		WithLogger(customLogger)(&opts)

		assert.Equal(t, customLogger, opts.logger)
	})

	t.Run("WithDebugCollectors", func(t *testing.T) {
		collectors := []string{"process", "custom"}
		opts := DefaultOpts()

		WithDebugCollectors(&collectors)(&opts)

		assert.True(t, opts.debugCollectors["go"])      // From default
		assert.True(t, opts.debugCollectors["process"]) // Added
		assert.True(t, opts.debugCollectors["custom"])  // Added
	})
}

func TestDefaultOpts(t *testing.T) {
	opts := DefaultOpts()

	// Check defaults
	assert.NotNil(t, opts.logger)
	assert.NotNil(t, opts.debugCollectors)
	assert.True(t, opts.debugCollectors["go"])
}

func TestExporter_Integration(t *testing.T) {
	mockMonitor := &MockMonitor{}
	mockRegistry := &MockAPIRegistry{}

	mockRegistry.On("Register", "/metrics", "Metrics", "Prometheus metrics", mock.Anything).Return(nil)

	// Create exporter with both collectors
	exporter := NewExporter(
		mockMonitor,
		mockRegistry,
		WithDebugCollectors(&[]string{"go", "process"}),
	)

	// Set up a cancellable context
	ctx, cancel := context.WithCancel(context.Background())

	// Start exporter in goroutine and cancel after brief period
	errCh := make(chan error)
	go func() {
		errCh <- exporter.Start(ctx)
	}()

	// Allow some time for registration then cancel
	time.Sleep(50 * time.Millisecond)
	cancel()

	// Wait for exporter to stop
	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Exporter did not stop after context cancellation")
	}

	// Verify all mocks
	mockRegistry.AssertExpectations(t)
	// TODO: verify mockMonitor calls once the exporter is implemented
	// mockMonitor.AssertExpectations(t)

	// Test stop method
	err := exporter.Stop()
	assert.NoError(t, err)
}
