// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package collector

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/sustainable-computing-io/kepler/config"
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

// assertMetricLabelValues finds a specific metric by matching all provided labels and checks its value
func assertMetricLabelValues(t *testing.T, registry *prometheus.Registry, metricName string, matchLabels map[string]string, expectedValue float64) {
	t.Helper()
	metrics, err := registry.Gather()
	assert.NoError(t, err)

	var availableMetrics []string
	var labelMismatches []string

	for _, mf := range metrics {
		if mf.GetName() != metricName {
			continue
		}

		for _, metric := range mf.GetMetric() {
			// Check if all provided labels match before proceeding
			allLabelsMatch := true
			var mismatches []string

			for labelName, expectedLabelValue := range matchLabels {
				actualLabelValue := valueOfLabel(metric, labelName)
				if actualLabelValue != expectedLabelValue {
					allLabelsMatch = false
					if actualLabelValue == "" {
						mismatches = append(mismatches, fmt.Sprintf("%s: missing", labelName))
					} else {
						mismatches = append(mismatches, fmt.Sprintf("%s: expected '%s', got '%s'", labelName, expectedLabelValue, actualLabelValue))
					}
				}
			}

			// Track this metric for error reporting
			metricLabels := make(map[string]string)
			for _, label := range metric.GetLabel() {
				metricLabels[label.GetName()] = label.GetValue()
			}
			availableMetrics = append(availableMetrics, fmt.Sprintf("  %v", metricLabels))

			// Skip this metric if not all labels match
			if !allLabelsMatch {
				labelMismatches = append(labelMismatches, fmt.Sprintf("  %v -> mismatches: [%s]", metricLabels, strings.Join(mismatches, ", ")))
				continue
			}

			// All labels match, now check the value
			var actualValue float64
			if metric.GetCounter() != nil {
				actualValue = metric.GetCounter().GetValue()
			} else if metric.GetGauge() != nil {
				actualValue = metric.GetGauge().GetValue()
			} else {
				t.Errorf("Metric %s has neither Counter nor Gauge value", metricName)
				return
			}

			assert.Equal(t, expectedValue, actualValue,
				"Metric %s with labels %v should have value %f but got %f",
				metricName, matchLabels, expectedValue, actualValue)

			// Found the matching metric and validated value
			return
		}
	}

	// Build detailed error message
	errorMsg := fmt.Sprintf("Metric %s with labels %v not found", metricName, matchLabels)
	if len(availableMetrics) > 0 {
		errorMsg += fmt.Sprintf("\nAvailable metrics for %s:\n%s", metricName, strings.Join(availableMetrics, "\n"))
	}
	if len(labelMismatches) > 0 {
		errorMsg += fmt.Sprintf("\nLabel mismatches found:\n%s", strings.Join(labelMismatches, "\n"))
	}

	t.Error(errorMsg)
}

// assertMetricExists verifies that a metric with the given labels exists
func assertMetricExists(t *testing.T, registry *prometheus.Registry, metricName string, matchLabels map[string]string) {
	metrics, err := registry.Gather()
	assert.NoError(t, err)

	for _, metricFamily := range metrics {
		if metricFamily.GetName() == metricName {
			for _, metric := range metricFamily.GetMetric() {
				// Check if all provided labels match
				allLabelsMatch := true
				for labelName, expectedLabelValue := range matchLabels {
					actualLabelValue := valueOfLabel(metric, labelName)
					if actualLabelValue != expectedLabelValue {
						allLabelsMatch = false
						break
					}
				}

				if allLabelsMatch {
					return // Found the metric
				}
			}
		}
	}

	t.Errorf("Metric %s with labels %v not found", metricName, matchLabels)
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
		"123": {
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

	testPods := monitor.Pods{
		"test-pod": {
			Name:      "test-pod",
			Namespace: "default",
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
		Pods:            testPods,
	}

	// Mock Snapshot method
	mockMonitor.On("Snapshot").Return(testData, nil)

	// Create collector
	allLevels := config.MetricsLevelAll
	collector := NewPowerCollector(mockMonitor, "test-node", logger, allLevels)

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

			"kepler_pod_cpu_joules_total",
			"kepler_pod_cpu_watts",
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
					nodeName := valueOfLabel(m, "node_name")

					seenZoneNames[zone] = true
					seenZonePaths[path] = true

					// Check that node_name constant label is present
					assert.Equal(t, "test-node", nodeName, "Expected node_name constant label")

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
					nodeName := valueOfLabel(m, "node_name")

					// Check that node_name constant label is present
					assert.Equal(t, "test-node", nodeName, "Expected node_name constant label")

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
					nodeName := valueOfLabel(m, "node_name")

					// Check that node_name constant label is present
					assert.Equal(t, "test-node", nodeName, "Expected node_name constant label")

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
					nodeName := valueOfLabel(m, "node_name")

					// Check that node_name constant label is present
					assert.Equal(t, "test-node", nodeName, "Expected node_name constant label")

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

		assert.ElementsMatch(t, zoneNames, []string{"package", "dram"})
		assert.ElementsMatch(t, zonePaths, []string{
			"/sys/class/powercap/intel-rapl/intel-rapl:0",
			"/sys/class/powercap/intel-rapl/intel-rapl:0:1",
		})
	})

	t.Run("Process Metrics Labels", func(t *testing.T) {
		expectedLabels := map[string]string{
			"node_name": "test-node",
			"pid":       "123",
			"comm":      "test-process",
			"exe":       "/usr/bin/123",
			"type":      "regular",
			"zone":      "package",
		}
		assertMetricLabelValues(t, registry, "kepler_process_cpu_joules_total", expectedLabels, 100.0)
		assertMetricLabelValues(t, registry, "kepler_process_cpu_watts", expectedLabels, 5.0)
	})

	t.Run("Container Metrics Labels", func(t *testing.T) {
		expectedLabels := map[string]string{
			"node_name":      "test-node",
			"container_id":   "abcd-efgh",
			"container_name": "test-container",
			"runtime":        "podman",
			"zone":           "package",
		}
		assertMetricLabelValues(t, registry, "kepler_container_cpu_joules_total", expectedLabels, 100.0)
		assertMetricLabelValues(t, registry, "kepler_container_cpu_watts", expectedLabels, 5.0)
	})

	t.Run("VM Metrics Labels", func(t *testing.T) {
		expectedLabels := map[string]string{
			"node_name":  "test-node",
			"vm_id":      "abcd-efgh",
			"vm_name":    "test-vm",
			"hypervisor": "kvm",
			"zone":       "package",
		}
		assertMetricLabelValues(t, registry, "kepler_vm_cpu_joules_total", expectedLabels, 100.0)
		assertMetricLabelValues(t, registry, "kepler_vm_cpu_watts", expectedLabels, 5.0)
	})

	t.Run("Pod Metrics Labels", func(t *testing.T) {
		expectedLabels := map[string]string{
			"node_name":     "test-node",
			"pod_id":        "test-pod",
			"pod_name":      "test-pod",
			"pod_namespace": "default",
			"zone":          "package",
		}
		assertMetricLabelValues(t, registry, "kepler_pod_cpu_joules_total", expectedLabels, 100.0)
		assertMetricLabelValues(t, registry, "kepler_pod_cpu_watts", expectedLabels, 5.0)
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

func TestTerminatedProcessExport(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	mockMonitor := NewMockPowerMonitor()

	packageZone := device.NewMockRaplZone("package", 0, "/sys/class/powercap/intel-rapl/intel-rapl:0", 1000)

	testSnapshot := &monitor.Snapshot{
		Timestamp: time.Now(),
		Node: &monitor.Node{
			Zones: monitor.NodeZoneUsageMap{
				packageZone: monitor.NodeUsage{
					EnergyTotal: 1000 * device.Joule,
					Power:       10 * device.Watt,
				},
			},
		},
		Processes: monitor.Processes{
			"123": &monitor.Process{
				PID:          123,
				Comm:         "running-proc",
				Exe:          "/usr/bin/running-proc",
				Type:         resource.RegularProcess,
				CPUTotalTime: 5.0,
				Zones: monitor.ZoneUsageMap{
					packageZone: monitor.Usage{
						EnergyTotal: 100 * device.Joule,
						Power:       5 * device.Watt,
					},
				},
				ContainerID:      "",
				VirtualMachineID: "",
			},
		},
		TerminatedProcesses: monitor.Processes{
			"456": &monitor.Process{
				PID:          456,
				Comm:         "terminated-proc",
				Exe:          "/usr/bin/terminated-proc",
				Type:         resource.RegularProcess,
				CPUTotalTime: 10.0,
				Zones: monitor.ZoneUsageMap{
					packageZone: monitor.Usage{
						EnergyTotal: 200 * device.Joule,
						Power:       20 * device.Watt,
					},
				},
				ContainerID:      "",
				VirtualMachineID: "",
			},
		},
		Containers:      monitor.Containers{},
		VirtualMachines: monitor.VirtualMachines{},
		Pods:            monitor.Pods{},
	}

	mockMonitor.On("Snapshot").Return(testSnapshot, nil)

	collector := NewPowerCollector(mockMonitor, "test-node", logger, config.MetricsLevelAll)

	registry := prometheus.NewRegistry()
	registry.MustRegister(collector)

	mockMonitor.TriggerUpdate()
	time.Sleep(10 * time.Millisecond)

	t.Run("Terminated Process Metrics Export", func(t *testing.T) {
		// Test running process metrics
		assertMetricLabelValues(t, registry, "kepler_process_cpu_joules_total",
			map[string]string{"pid": "123", "state": "running"}, 100.0)
		assertMetricLabelValues(t, registry, "kepler_process_cpu_watts",
			map[string]string{"pid": "123", "state": "running"}, 5.0)

		// Test terminated process metrics
		assertMetricLabelValues(t, registry, "kepler_process_cpu_joules_total",
			map[string]string{"pid": "456", "state": "terminated"}, 200.0)
		assertMetricLabelValues(t, registry, "kepler_process_cpu_watts",
			map[string]string{"pid": "456", "state": "terminated"}, 20.0)

		// Test additional labels for running process
		assertMetricLabelValues(t, registry, "kepler_process_cpu_joules_total",
			map[string]string{"pid": "123", "comm": "running-proc", "exe": "/usr/bin/running-proc", "type": "regular"}, 100.0)

		// Test additional labels for terminated process
		assertMetricLabelValues(t, registry, "kepler_process_cpu_joules_total",
			map[string]string{"pid": "456", "comm": "terminated-proc", "exe": "/usr/bin/terminated-proc", "type": "regular"}, 200.0)
	})

	t.Run("Process State Labels", func(t *testing.T) {
		// Verify that the state label exists and has correct values
		assertMetricExists(t, registry, "kepler_process_cpu_joules_total",
			map[string]string{"state": "running"})
		assertMetricExists(t, registry, "kepler_process_cpu_joules_total",
			map[string]string{"state": "terminated"})

		// Also verify for watts metrics
		assertMetricExists(t, registry, "kepler_process_cpu_watts",
			map[string]string{"state": "running"})
		assertMetricExists(t, registry, "kepler_process_cpu_watts",
			map[string]string{"state": "terminated"})
	})

	mockMonitor.AssertExpectations(t)
}

func TestEnhancedErrorReporting(t *testing.T) {
	t.Skip("This test demonstrates enhanced error reporting - skipped by default")

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	mockMonitor := NewMockPowerMonitor()

	packageZone := device.NewMockRaplZone("package", 0, "/sys/class/powercap/intel-rapl/intel-rapl:0", 1000)

	testSnapshot := &monitor.Snapshot{
		Timestamp: time.Now(),
		Node: &monitor.Node{
			Zones: monitor.NodeZoneUsageMap{
				packageZone: monitor.NodeUsage{
					EnergyTotal: 1000 * device.Joule,
					Power:       10 * device.Watt,
				},
			},
		},
		Processes: monitor.Processes{
			"123": &monitor.Process{
				PID:          123,
				Comm:         "actual-proc",
				Exe:          "/usr/bin/actual-proc",
				Type:         resource.RegularProcess,
				CPUTotalTime: 5.0,
				Zones: monitor.ZoneUsageMap{
					packageZone: monitor.Usage{
						EnergyTotal: 100 * device.Joule,
						Power:       5 * device.Watt,
					},
				},
				ContainerID:      "actual-container",
				VirtualMachineID: "",
			},
		},
		TerminatedProcesses: monitor.Processes{
			"456": &monitor.Process{
				PID:          456,
				Comm:         "terminated-proc",
				Exe:          "/usr/bin/terminated-proc",
				Type:         resource.RegularProcess,
				CPUTotalTime: 10.0,
				Zones: monitor.ZoneUsageMap{
					packageZone: monitor.Usage{
						EnergyTotal: 200 * device.Joule,
						Power:       20 * device.Watt,
					},
				},
				ContainerID:      "",
				VirtualMachineID: "",
			},
		},
		Containers:      monitor.Containers{},
		VirtualMachines: monitor.VirtualMachines{},
		Pods:            monitor.Pods{},
	}

	mockMonitor.On("Snapshot").Return(testSnapshot, nil)
	collector := NewPowerCollector(mockMonitor, "test-node", logger, config.MetricsLevelAll)
	registry := prometheus.NewRegistry()
	registry.MustRegister(collector)
	mockMonitor.TriggerUpdate()
	time.Sleep(10 * time.Millisecond)

	// This should fail and show detailed error reporting
	assertMetricLabelValues(t, registry, "kepler_process_cpu_joules_total",
		map[string]string{"pid": "999", "state": "nonexistent", "comm": "missing-proc"}, 100.0)

	mockMonitor.AssertExpectations(t)
}

func TestPowerCollector_MetricsLevelFiltering(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	tests := []struct {
		name            string
		metricsLevel    config.Level
		expectedMetrics map[string]bool
	}{
		{
			name:         "Only Node metrics",
			metricsLevel: config.MetricsLevelNode,
			expectedMetrics: map[string]bool{
				"kepler_node_cpu_joules_total":        true,
				"kepler_node_cpu_watts":               true,
				"kepler_node_cpu_usage_ratio":         true,
				"kepler_node_cpu_active_joules_total": true,
				"kepler_node_cpu_active_watts":        true,
				"kepler_node_cpu_idle_joules_total":   true,
				"kepler_node_cpu_idle_watts":          true,
				"kepler_process_cpu_joules_total":     false,
				"kepler_container_cpu_joules_total":   false,
				"kepler_vm_cpu_joules_total":          false,
				"kepler_pod_cpu_joules_total":         false,
			},
		},
		{
			name:         "Only Process metrics",
			metricsLevel: config.MetricsLevelProcess,
			expectedMetrics: map[string]bool{
				"kepler_node_cpu_joules_total":      false,
				"kepler_process_cpu_joules_total":   true,
				"kepler_process_cpu_watts":          true,
				"kepler_process_cpu_seconds_total":  true,
				"kepler_container_cpu_joules_total": false,
				"kepler_vm_cpu_joules_total":        false,
				"kepler_pod_cpu_joules_total":       false,
			},
		},
		{
			name:         "Node and Container metrics",
			metricsLevel: config.MetricsLevelNode | config.MetricsLevelContainer,
			expectedMetrics: map[string]bool{
				"kepler_node_cpu_joules_total":      true,
				"kepler_node_cpu_watts":             true,
				"kepler_process_cpu_joules_total":   false,
				"kepler_container_cpu_joules_total": true,
				"kepler_container_cpu_watts":        true,
				"kepler_vm_cpu_joules_total":        false,
				"kepler_pod_cpu_joules_total":       false,
			},
		},
		{
			name:         "No metrics",
			metricsLevel: config.Level(0),
			expectedMetrics: map[string]bool{
				"kepler_node_cpu_joules_total":      false,
				"kepler_process_cpu_joules_total":   false,
				"kepler_container_cpu_joules_total": false,
				"kepler_vm_cpu_joules_total":        false,
				"kepler_pod_cpu_joules_total":       false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockMonitor := NewMockPowerMonitor()

			// Create test data with all types of metrics
			packageZone := device.NewMockRaplZone("package", 0, "/sys/class/powercap/intel-rapl/intel-rapl:0", 1000)
			testData := &monitor.Snapshot{
				Timestamp: time.Now(),
				Node: &monitor.Node{
					Zones: monitor.NodeZoneUsageMap{
						packageZone: monitor.NodeUsage{
							EnergyTotal:       1000 * device.Joule,
							Power:             10 * device.Watt,
							ActiveEnergyTotal: 500 * device.Joule,
							IdleEnergyTotal:   500 * device.Joule,
							ActivePower:       5 * device.Watt,
							IdlePower:         5 * device.Watt,
						},
					},
					UsageRatio: 0.5,
				},
				Processes: monitor.Processes{
					"123": &monitor.Process{
						PID:          123,
						Comm:         "test-process",
						Exe:          "/usr/bin/test-process",
						Type:         resource.RegularProcess,
						CPUTotalTime: 5.0,
						Zones: monitor.ZoneUsageMap{
							packageZone: monitor.Usage{
								EnergyTotal: 100 * device.Joule,
								Power:       5 * device.Watt,
							},
						},
						ContainerID:      "test-container",
						VirtualMachineID: "test-vm",
					},
				},
				Containers: monitor.Containers{
					"test-container": &monitor.Container{
						ID:      "test-container",
						Name:    "test-container",
						Runtime: resource.PodmanRuntime,
						PodID:   "test-pod",
						Zones: monitor.ZoneUsageMap{
							packageZone: monitor.Usage{
								EnergyTotal: 100 * device.Joule,
								Power:       5 * device.Watt,
							},
						},
					},
				},
				VirtualMachines: monitor.VirtualMachines{
					"test-vm": &monitor.VirtualMachine{
						ID:         "test-vm",
						Name:       "test-vm",
						Hypervisor: resource.KVMHypervisor,
						Zones: monitor.ZoneUsageMap{
							packageZone: monitor.Usage{
								EnergyTotal: 100 * device.Joule,
								Power:       5 * device.Watt,
							},
						},
					},
				},
				Pods: monitor.Pods{
					"test-pod": &monitor.Pod{
						ID:        "test-pod",
						Name:      "test-pod",
						Namespace: "default",
						Zones: monitor.ZoneUsageMap{
							packageZone: monitor.Usage{
								EnergyTotal: 100 * device.Joule,
								Power:       5 * device.Watt,
							},
						},
					},
				},
			}

			mockMonitor.On("Snapshot").Return(testData, nil)

			collector := NewPowerCollector(mockMonitor, "test-node", logger, tt.metricsLevel)
			registry := prometheus.NewRegistry()
			registry.MustRegister(collector)

			mockMonitor.TriggerUpdate()
			time.Sleep(10 * time.Millisecond)

			// Gather metrics
			metricFamilies, err := registry.Gather()
			assert.NoError(t, err)

			// Create a map of existing metrics
			existingMetrics := make(map[string]bool)
			for _, mf := range metricFamilies {
				existingMetrics[mf.GetName()] = true
			}

			// Check expected metrics
			for metricName, shouldExist := range tt.expectedMetrics {
				if shouldExist {
					assert.True(t, existingMetrics[metricName], "Expected metric %s to exist", metricName)
				} else {
					assert.False(t, existingMetrics[metricName], "Expected metric %s to not exist", metricName)
				}
			}

			mockMonitor.AssertExpectations(t)
		})
	}
}

func TestTerminatedContainerExport(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	mockMonitor := NewMockPowerMonitor()

	packageZone := device.NewMockRaplZone("package", 0, "/sys/class/powercap/intel-rapl/intel-rapl:0", 1000)

	testSnapshot := &monitor.Snapshot{
		Timestamp: time.Now(),
		Node: &monitor.Node{
			Zones: monitor.NodeZoneUsageMap{
				packageZone: monitor.NodeUsage{
					EnergyTotal: 1000 * device.Joule,
					Power:       10 * device.Watt,
				},
			},
		},
		Processes: monitor.Processes{},
		Containers: monitor.Containers{
			"running-container": &monitor.Container{
				ID:      "running-container",
				Name:    "running-cont",
				Runtime: resource.DockerRuntime,
				Zones: monitor.ZoneUsageMap{
					packageZone: monitor.Usage{
						EnergyTotal: 150 * device.Joule,
						Power:       15 * device.Watt,
					},
				},
			},
		},
		TerminatedContainers: monitor.Containers{
			"terminated-container": &monitor.Container{
				ID:      "terminated-container",
				Name:    "terminated-cont",
				Runtime: resource.PodmanRuntime,
				Zones: monitor.ZoneUsageMap{
					packageZone: monitor.Usage{
						EnergyTotal: 300 * device.Joule,
						Power:       30 * device.Watt,
					},
				},
			},
		},
		VirtualMachines: monitor.VirtualMachines{},
		Pods:            monitor.Pods{},
	}

	mockMonitor.On("Snapshot").Return(testSnapshot, nil)

	allLevels := config.MetricsLevelAll
	collector := NewPowerCollector(mockMonitor, "test-node", logger, allLevels)

	registry := prometheus.NewRegistry()
	registry.MustRegister(collector)

	mockMonitor.TriggerUpdate()
	time.Sleep(10 * time.Millisecond)

	t.Run("Terminated Container Metrics Export", func(t *testing.T) {
		// Test running container metrics
		assertMetricLabelValues(t, registry, "kepler_container_cpu_joules_total",
			map[string]string{"container_id": "running-container", "state": "running"}, 150.0)
		assertMetricLabelValues(t, registry, "kepler_container_cpu_watts",
			map[string]string{"container_id": "running-container", "state": "running"}, 15.0)

		// Test terminated container metrics
		assertMetricLabelValues(t, registry, "kepler_container_cpu_joules_total",
			map[string]string{"container_id": "terminated-container", "state": "terminated"}, 300.0)
		assertMetricLabelValues(t, registry, "kepler_container_cpu_watts",
			map[string]string{"container_id": "terminated-container", "state": "terminated"}, 30.0)

		// Test additional labels for running container
		assertMetricLabelValues(t, registry, "kepler_container_cpu_joules_total",
			map[string]string{"container_id": "running-container", "container_name": "running-cont", "runtime": "docker"}, 150.0)

		// Test additional labels for terminated container
		assertMetricLabelValues(t, registry, "kepler_container_cpu_joules_total",
			map[string]string{"container_id": "terminated-container", "container_name": "terminated-cont", "runtime": "podman"}, 300.0)
	})

	t.Run("Container State Labels", func(t *testing.T) {
		// Verify that the state label exists and has correct values
		assertMetricExists(t, registry, "kepler_container_cpu_joules_total",
			map[string]string{"state": "running"})
		assertMetricExists(t, registry, "kepler_container_cpu_joules_total",
			map[string]string{"state": "terminated"})

		// Also verify for watts metrics
		assertMetricExists(t, registry, "kepler_container_cpu_watts",
			map[string]string{"state": "running"})
		assertMetricExists(t, registry, "kepler_container_cpu_watts",
			map[string]string{"state": "terminated"})
	})

	mockMonitor.AssertExpectations(t)
}

func TestTerminatedVMExport(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	mockMonitor := NewMockPowerMonitor()

	packageZone := device.NewMockRaplZone("package", 0, "/sys/class/powercap/intel-rapl/intel-rapl:0", 1000)

	testSnapshot := &monitor.Snapshot{
		Timestamp: time.Now(),
		Node: &monitor.Node{
			Zones: monitor.NodeZoneUsageMap{
				packageZone: monitor.NodeUsage{
					EnergyTotal: 1000 * device.Joule,
					Power:       10 * device.Watt,
				},
			},
		},
		Processes:  monitor.Processes{},
		Containers: monitor.Containers{},
		VirtualMachines: monitor.VirtualMachines{
			"running-vm": &monitor.VirtualMachine{
				ID:         "running-vm",
				Name:       "running-virtual-machine",
				Hypervisor: resource.KVMHypervisor,
				Zones: monitor.ZoneUsageMap{
					packageZone: monitor.Usage{
						EnergyTotal: 250 * device.Joule,
						Power:       25 * device.Watt,
					},
				},
			},
		},
		TerminatedVirtualMachines: monitor.VirtualMachines{
			"terminated-vm": &monitor.VirtualMachine{
				ID:         "terminated-vm",
				Name:       "terminated-virtual-machine",
				Hypervisor: resource.KVMHypervisor,
				Zones: monitor.ZoneUsageMap{
					packageZone: monitor.Usage{
						EnergyTotal: 400 * device.Joule,
						Power:       40 * device.Watt,
					},
				},
			},
		},
		Pods: monitor.Pods{},
	}

	mockMonitor.On("Snapshot").Return(testSnapshot, nil)

	allLevels := config.MetricsLevelAll
	collector := NewPowerCollector(mockMonitor, "test-node", logger, allLevels)

	registry := prometheus.NewRegistry()
	registry.MustRegister(collector)

	mockMonitor.TriggerUpdate()
	time.Sleep(10 * time.Millisecond)

	t.Run("Terminated VM Metrics Export", func(t *testing.T) {
		// Test running VM metrics
		assertMetricLabelValues(t, registry, "kepler_vm_cpu_joules_total",
			map[string]string{"vm_id": "running-vm", "state": "running"}, 250.0)
		assertMetricLabelValues(t, registry, "kepler_vm_cpu_watts",
			map[string]string{"vm_id": "running-vm", "state": "running"}, 25.0)

		// Test terminated VM metrics
		assertMetricLabelValues(t, registry, "kepler_vm_cpu_joules_total",
			map[string]string{"vm_id": "terminated-vm", "state": "terminated"}, 400.0)
		assertMetricLabelValues(t, registry, "kepler_vm_cpu_watts",
			map[string]string{"vm_id": "terminated-vm", "state": "terminated"}, 40.0)

		// Test additional labels for running VM
		assertMetricLabelValues(t, registry, "kepler_vm_cpu_joules_total",
			map[string]string{"vm_id": "running-vm", "vm_name": "running-virtual-machine", "hypervisor": "kvm"}, 250.0)

		// Test additional labels for terminated VM
		assertMetricLabelValues(t, registry, "kepler_vm_cpu_joules_total",
			map[string]string{"vm_id": "terminated-vm", "vm_name": "terminated-virtual-machine", "hypervisor": "kvm"}, 400.0)
	})

	t.Run("VM State Labels", func(t *testing.T) {
		// Verify that the state label exists and has correct values
		assertMetricExists(t, registry, "kepler_vm_cpu_joules_total",
			map[string]string{"state": "running"})
		assertMetricExists(t, registry, "kepler_vm_cpu_joules_total",
			map[string]string{"state": "terminated"})

		// Also verify for watts metrics
		assertMetricExists(t, registry, "kepler_vm_cpu_watts",
			map[string]string{"state": "running"})
		assertMetricExists(t, registry, "kepler_vm_cpu_watts",
			map[string]string{"state": "terminated"})
	})

	mockMonitor.AssertExpectations(t)
}
