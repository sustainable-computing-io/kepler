// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package collector

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/sustainable-computing-io/kepler/internal/device"
	"github.com/sustainable-computing-io/kepler/internal/monitor"
)

// MockPowerMonitor mocks the PowerMonitor for testing
type MockPowerMonitor struct {
	mock.Mock
	dataCh chan struct{}
}

func NewMockPowerMonitor() *MockPowerMonitor {
	return &MockPowerMonitor{
		dataCh: make(chan struct{}, 1),
	}
}

var _ PowerDataProvider = (*MockPowerMonitor)(nil)

func (m *MockPowerMonitor) Start(ctx context.Context) error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockPowerMonitor) Name() string {
	m.Called()
	return "mock-power-monitor"
}

func (m *MockPowerMonitor) Snapshot() (*monitor.Snapshot, error) {
	args := m.Called()
	return args.Get(0).(*monitor.Snapshot), args.Error(1)
}

func (m *MockPowerMonitor) DataChannel() <-chan struct{} {
	return m.dataCh
}

func (m *MockPowerMonitor) ZoneNames() []string {
	args := m.Called()
	return args.Get(0).([]string)
}

func (m *MockPowerMonitor) TriggerUpdate() {
	select {
	case m.dataCh <- struct{}{}:
	default:
	}
}

func TestPowerCollector(t *testing.T) {
	// Create a logger that writes to stderr for testing
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	// Create mock PowerMonitor
	mockMonitor := NewMockPowerMonitor()

	// Setup test zones
	packageZone := device.NewMockRaplZone("package", 0, "/sys/class/powercap/intel-rapl/intel-rapl:0", 1000)
	dramZone := device.NewMockRaplZone("dram", 0, "/sys/class/powercap/intel-rapl/intel-rapl:0:1", 1000)

	// Mock zone names
	mockMonitor.On("ZoneNames").Return([]string{"package", "dram"})

	nodePkgAbs := 12300 * device.Joule
	nodePkgDelta := 123 * device.Joule
	nodePkgPower := 12 * device.Watt

	nodeDramAbs := 2340 * device.Joule
	nodeDramDelta := 234 * device.Joule
	nodeDramPower := 2 * device.Watt

	// Create test node Snapshot
	testNodeData := monitor.Node{
		Zones: monitor.ZoneUsageMap{
			packageZone: {
				Absolute: nodePkgAbs,
				Delta:    nodePkgDelta,
				Power:    nodePkgPower,
			},
			dramZone: {
				Absolute: nodeDramAbs,
				Delta:    nodeDramDelta,
				Power:    nodeDramPower,
			},
		},
	}

	// Create test Snapshot
	testData := &monitor.Snapshot{
		Timestamp: time.Now(),
		Node:      &testNodeData,
	}

	// Mock Snapshot method
	mockMonitor.On("Snapshot").Return(testData, nil)

	// Create collector
	collector := NewPowerCollector(mockMonitor, logger)

	// Trigger update to ensure descriptors are created
	mockMonitor.TriggerUpdate()
	// Small sleep to ensure goroutine processes the update
	time.Sleep(10 * time.Millisecond)

	// Create a registry and register the collector
	registry := prometheus.NewRegistry()
	registry.MustRegister(collector)

	// Test cases
	t.Run("Metrics Creation", func(t *testing.T) {
		// Get metrics from registry
		metrics, err := registry.Gather()
		assert.NoError(t, err)

		// Check that metrics exist in registry
		expectedMetricNames := []string{
			"kepler_node_package_joules_total",
			"kepler_node_package_watts",
			"kepler_node_dram_joules_total",
			"kepler_node_dram_watts",
			"kepler_node_energy_zone",
		}

		assert.ElementsMatch(t, expectedMetricNames, metricNames(metrics))
	})

	t.Run("Node Metrics Values", func(t *testing.T) {
		// Get metrics from registry
		metrics, err := registry.Gather()
		assert.NoError(t, err)

		zoneNames := []string{}
		zonePaths := []string{}

		// Check node joules metrics
		for _, metric := range metrics {
			if strings.HasPrefix(metric.GetName(), "kepler_node_") && strings.HasSuffix(metric.GetName(), "_joules_total") {
				for _, m := range metric.GetMetric() {
					path := valueOfLabel(m, "path")
					value := m.GetCounter().GetValue()

					if path == packageZone.Path() {
						assert.Equal(t, nodePkgAbs.Joules(), value, "Unexpected package joules")
					} else if path == dramZone.Path() {
						assert.Equal(t, nodeDramAbs.Joules(), value, "Unexpected dram joules")
					}
				}
			}
		}

		// Check node watts metrics
		for _, metric := range metrics {
			if strings.HasPrefix(metric.GetName(), "kepler_node_") && strings.HasSuffix(metric.GetName(), "_watts") {
				for _, m := range metric.GetMetric() {
					path := valueOfLabel(m, "path")
					value := m.GetGauge().GetValue()

					if path == packageZone.Path() {
						assert.Equal(t, nodePkgPower.Watts(), value, "Expected zone1 watts to be 50.0")
					} else if path == dramZone.Path() {
						assert.Equal(t, nodeDramPower.Watts(), value, "Expected zone2 watts to be 10.0")
					}
				}
			}
		}

		// check node energy zone metrics
		for _, metric := range metrics {
			if strings.HasPrefix(metric.GetName(), "kepler_node_") && strings.HasSuffix(metric.GetName(), "energy_zone") {
				for _, m := range metric.GetMetric() {
					value := m.GetGauge().GetValue()
					assert.Equal(t, 1.0, value, "Expected 2 energy zones")
					zoneNames = append(zoneNames, valueOfLabel(m, "name"))
					zonePaths = append(zonePaths, valueOfLabel(m, "path"))
				}
			}
		}
		assert.ElementsMatch(t, zoneNames, []string{"package", "dram"})
		assert.ElementsMatch(t, zonePaths, []string{
			"/sys/class/powercap/intel-rapl/intel-rapl:0",
			"/sys/class/powercap/intel-rapl/intel-rapl:0:1",
		})
	})

	// Verify mock expectations
	mockMonitor.AssertExpectations(t)
}

// valueOfLabel returns the value of the label with the given name
func valueOfLabel(metric *dto.Metric, name string) string {
	for _, label := range metric.GetLabel() {
		if label.GetName() == name {
			return label.GetValue()
		}
	}
	return ""
}

func metricNames(metrics []*dto.MetricFamily) []string {
	if len(metrics) == 0 {
		return []string{}
	}
	names := []string{}
	for _, metric := range metrics {
		if metric != nil {
			names = append(names, metric.GetName())
		}
	}
	return names
}
