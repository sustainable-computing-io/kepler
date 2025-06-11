// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package collector

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/sustainable-computing-io/kepler/internal/device"
	"github.com/sustainable-computing-io/kepler/internal/monitor"
	"github.com/sustainable-computing-io/kepler/internal/resource"
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

	nodePkgAbs := 12300 * device.Joule
	nodePkgDelta := 123 * device.Joule
	nodePkgPower := 12 * device.Watt

	nodeDramAbs := 2340 * device.Joule
	nodeDramDelta := 234 * device.Joule
	nodeDramPower := 2 * device.Watt

	// Create test node Snapshot
	testNodeData := monitor.Node{
		Timestamp:  time.Now(),
		UsageRatio: 0.5,
		Zones: monitor.NodeZoneUsageMap{
			packageZone: monitor.NodeUsage{
				EnergyTotal:       nodePkgAbs,
				ActiveEnergyTotal: nodePkgDelta / 2, // 50% of delta is used
				IdleEnergyTotal:   nodePkgDelta / 2, // 50% of delta is idle
				Power:             nodePkgPower,
				ActivePower:       nodePkgPower / 2, // 50% of power is used
				IdlePower:         nodePkgPower / 2, // 50% of power is idle
			},
			dramZone: monitor.NodeUsage{
				EnergyTotal:       nodeDramAbs,
				ActiveEnergyTotal: nodeDramDelta / 2, // 50% of delta is used
				IdleEnergyTotal:   nodeDramDelta / 2, // 50% of delta is idle
				Power:             nodeDramPower,
				ActivePower:       nodeDramPower / 2, // 50% of power is used
				IdlePower:         nodeDramPower / 2, // 50% of power is idle
			},
		},
	}

	testProcesses := monitor.Processes{
		123: {
			PID:          123,
			Comm:         "test-process",
			Exe:          "/usr/bin/123",
			Type:         resource.RegularProcess,
			CPUTotalTime: 100,
			Zones: monitor.ZoneUsageMap{
				packageZone: {
					EnergyTotal: 100 * device.Joule,
					Power:       5 * device.Watt,
				},
			},
		},
	}

	testContainers := monitor.Containers{
		"abcd-efgh": {
			ID:      "abcd-efgh",
			Name:    "test-container",
			Runtime: resource.PodmanRuntime,
			Zones: monitor.ZoneUsageMap{
				packageZone: {
					EnergyTotal: 100 * device.Joule,
					Power:       5 * device.Watt,
				},
			},
		},
	}

	testVMs := monitor.VirtualMachines{
		"abcd-efgh": {
			ID:         "abcd-efgh",
			Name:       "test-vm",
			Hypervisor: resource.KVMHypervisor,
			Zones: monitor.ZoneUsageMap{
				packageZone: {
					EnergyTotal: 100 * device.Joule,
					Power:       5 * device.Watt,
				},
			},
		},
	}

	// Create test Snapshot
	testData := &monitor.Snapshot{
		Timestamp:       time.Now(),
		Node:            &testNodeData,
		Processes:       testProcesses,
		Containers:      testContainers,
		VirtualMachines: testVMs,
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
			"kepler_node_cpu_joules_total",
			"kepler_node_cpu_watts",
			"kepler_node_cpu_usage_ratio",
			"kepler_node_cpu_active_joules_total",
			"kepler_node_cpu_idle_joules_total",
			"kepler_node_cpu_active_watts",
			"kepler_node_cpu_idle_watts",

			"kepler_process_cpu_joules_total",
			"kepler_process_cpu_watts",
			"kepler_process_cpu_seconds_total",

			"kepler_container_cpu_joules_total",
			"kepler_container_cpu_watts",

			"kepler_vm_cpu_joules_total",
			"kepler_vm_cpu_watts",
		}

		assert.ElementsMatch(t, expectedMetricNames, metricNames(metrics))
	})

	t.Run("Node Metrics Values", func(t *testing.T) {
		// Get metrics from registry
		metrics, err := registry.Gather()
		assert.NoError(t, err)

		seenZoneNames := make(map[string]bool)
		seenZonePaths := make(map[string]bool)

		// Check main node joules metrics
		for _, metric := range metrics {
			if metric.GetName() == "kepler_node_cpu_joules_total" {
				for _, m := range metric.GetMetric() {
					path := valueOfLabel(m, "path")
					value := m.GetCounter().GetValue()
					zone := valueOfLabel(m, "zone")

					seenZoneNames[zone] = true
					seenZonePaths[path] = true

					// Check absolute values
					if path == packageZone.Path() {
						assert.Equal(t, nodePkgAbs.Joules(), value, "Unexpected package joules")
					} else if path == dramZone.Path() {
						assert.Equal(t, nodeDramAbs.Joules(), value, "Unexpected dram joules")
					}
				}
			}
		}

		for _, metric := range metrics {
			if metric.GetName() == "kepler_node_cpu_watts" {
				for _, m := range metric.GetMetric() {
					path := valueOfLabel(m, "path")
					value := m.GetGauge().GetValue()

					// Check total power values
					if path == packageZone.Path() {
						assert.Equal(t, nodePkgPower.Watts(), value, "Expected package watts")
					} else if path == dramZone.Path() {
						assert.Equal(t, nodeDramPower.Watts(), value, "Expected dram watts")
					}
				}
			}
		}

		// Check active/idle attribution metrics (separate metrics, no mode label)
		for _, metric := range metrics {
			if metric.GetName() == "kepler_node_cpu_active_watts" {
				for _, m := range metric.GetMetric() {
					path := valueOfLabel(m, "path")
					value := m.GetGauge().GetValue()

					if path == packageZone.Path() {
						expectedValue := (nodePkgPower / 2).Watts() // 50% active
						assert.Equal(t, expectedValue, value, "Expected package active watts")
					}
				}
			}
			if metric.GetName() == "kepler_node_cpu_idle_watts" {
				for _, m := range metric.GetMetric() {
					path := valueOfLabel(m, "path")
					value := m.GetGauge().GetValue()

					if path == packageZone.Path() {
						expectedValue := (nodePkgPower / 2).Watts() // 50% idle
						assert.Equal(t, expectedValue, value, "Expected package idle watts")
					}
				}
			}
		}

		// Convert maps to slices for assertion
		zoneNames := make([]string, 0, len(seenZoneNames))
		for name := range seenZoneNames {
			zoneNames = append(zoneNames, name)
		}
		zonePaths := make([]string, 0, len(seenZonePaths))
		for path := range seenZonePaths {
			zonePaths = append(zonePaths, path)
		}

		assert.ElementsMatch(t, zoneNames, []string{"package-0", "dram-0"})
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
