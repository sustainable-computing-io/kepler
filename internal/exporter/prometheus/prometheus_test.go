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
	"github.com/sustainable-computing-io/kepler/internal/monitor"
)

// MockMonitor mocks the Monitor interface
type MockMonitor struct {
	mock.Mock
}

func (m *MockMonitor) Init() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockMonitor) Run(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockMonitor) Shutdown() error {
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
			WithDebugCollectors([]string{"go", "process"}),
		},
		expectService: "prometheus",
	}, {
		name: "with multiple options",
		opts: []OptionFn{
			WithLogger(slog.Default().With("test", "custom")),
			WithDebugCollectors([]string{"process"}),
		},
		expectService: "prometheus",
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockMonitor := &MockMonitor{}
			mockMonitor.On("DataChannel").Return(make(<-chan struct{}))

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
	mockMonitor.On("DataChannel").Return(make(<-chan struct{}))
	mockRegistry := &MockAPIRegistry{}

	exporter := NewExporter(mockMonitor, mockRegistry)

	assert.Equal(t, "prometheus", exporter.Name())
}

func TestExporter_Init(t *testing.T) {
	t.Run("starts successfully", func(t *testing.T) {
		mockMonitor := &MockMonitor{}
		mockMonitor.On("DataChannel").Return(make(<-chan struct{}))
		mockRegistry := &MockAPIRegistry{}

		// Setup the mock expectations
		mockRegistry.On("Register", "/metrics", "Metrics", "Prometheus metrics", mock.Anything).Return(nil)

		exporter := NewExporter(mockMonitor, mockRegistry)
		err := exporter.Init()
		assert.NoError(t, err)

		mockRegistry.AssertExpectations(t)
	})

	t.Run("registry returns error", func(t *testing.T) {
		mockMonitor := &MockMonitor{}
		mockMonitor.On("DataChannel").Return(make(<-chan struct{}))
		mockRegistry := &MockAPIRegistry{}

		// Setup the mock to return an error
		expectedErr := errors.New("register error")
		mockRegistry.On("Register", "/metrics", "Metrics", "Prometheus metrics", mock.Anything).Return(expectedErr)

		exporter := NewExporter(mockMonitor, mockRegistry)

		// Init should return the error immediately
		err := exporter.Init()

		assert.Error(t, err)
		assert.Equal(t, expectedErr, err)
		mockRegistry.AssertExpectations(t)
	})

	t.Run("with invalid collector", func(t *testing.T) {
		mockMonitor := &MockMonitor{}
		mockMonitor.On("DataChannel").Return(make(<-chan struct{}))
		mockRegistry := &MockAPIRegistry{}

		// Create an exporter with an unknown collector
		exporter := NewExporter(
			mockMonitor,
			mockRegistry,
			WithDebugCollectors([]string{"unknown_collector"}),
		)

		// Init should return an error
		err := exporter.Init()

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unknown collector: unknown_collector")
		mockRegistry.AssertNotCalled(t, "Register")
	})

	t.Run("with multiple valid collectors", func(t *testing.T) {
		mockMonitor := &MockMonitor{}
		mockMonitor.On("DataChannel").Return(make(<-chan struct{}))
		mockRegistry := &MockAPIRegistry{}

		mockRegistry.On("Register", "/metrics", "Metrics", "Prometheus metrics", mock.Anything).Return(nil)

		// Create an exporter with multiple valid collectors
		exporter := NewExporter(
			mockMonitor,
			mockRegistry,
			WithDebugCollectors([]string{"go", "process"}),
		)

		err := exporter.Init()
		assert.NoError(t, err)
		mockRegistry.AssertExpectations(t)
	})
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
		opts := DefaultOpts()
		assert.True(t, opts.debugCollectors["go"]) // From default

		collectors := []string{"process", "custom"}
		WithDebugCollectors(collectors)(&opts)

		assert.False(t, opts.debugCollectors["go"]) // should override default
		assert.True(t, opts.debugCollectors["process"])
		assert.True(t, opts.debugCollectors["custom"])
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

	dummyCollector := prom.CollectorFunc(func(ch chan<- prom.Metric) {})
	// Create exporter with dummyCollector
	exporter := NewExporter(
		mockMonitor,
		mockRegistry,
		WithDebugCollectors([]string{"go", "process"}),
		WithCollectors(map[string]prom.Collector{"dummy": dummyCollector}),
	)

	assert.NoError(t, exporter.Init(), "exporter init failed")

	// Verify all mocks
	mockRegistry.AssertExpectations(t)
	// TODO: verify mockMonitor calls once the exporter is implemented
	mockMonitor.AssertExpectations(t)
}

func TestExporter_CreateCollectors(t *testing.T) {
	mockMonitor := &MockMonitor{}
	mockMonitor.On("DataChannel").Return(make(<-chan struct{}))

	// create Collectors
	coll, err := CreateCollectors(
		mockMonitor,
		WithLogger(slog.Default()),
		WithProcFSPath("/proc"),
	)
	time.Sleep(50 * time.Millisecond)

	// Verify mocks
	mockMonitor.AssertExpectations(t)

	assert.NoError(t, err)
	assert.Len(t, coll, 3)
}
