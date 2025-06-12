// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
)

// MockCollector implements prometheus.Collector for testing
type MockCollector struct {
	descs []*prometheus.Desc
}

func (c *MockCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, desc := range c.descs {
		ch <- desc
	}
}

func (c *MockCollector) Collect(ch chan<- prometheus.Metric) {
	// Empty implementation for testing
}

// TestExtractMetricsInfo tests the extractMetricsInfo function
func TestExtractMetricsInfo(t *testing.T) {
	tests := []struct {
		name               string
		descs              []*prometheus.Desc
		expectedMetricsLen int
		expectedMetrics    []MetricInfo
	}{
		{
			name: "ValidMetrics",
			descs: []*prometheus.Desc{
				prometheus.NewDesc("test_counter_total", "Test counter metric", []string{"label1", "label2"}, nil),
				prometheus.NewDesc("test_gauge", "Test gauge metric", []string{"label3"}, nil),
				prometheus.NewDesc("test_no_labels", "Test metric without labels", nil, nil),
			},
			expectedMetricsLen: 3,
			expectedMetrics: []MetricInfo{
				{
					Name:        "test_counter_total",
					Type:        "COUNTER",
					Description: "Test counter metric",
					Labels:      []string{"label1", "label2"},
					ConstLabels: map[string]string{},
				},
				{
					Name:        "test_gauge",
					Type:        "GAUGE",
					Description: "Test gauge metric",
					Labels:      []string{"label3"},
					ConstLabels: map[string]string{},
				},
				{
					Name:        "test_no_labels",
					Type:        "GAUGE",
					Description: "Test metric without labels",
					Labels:      nil,
					ConstLabels: map[string]string{},
				},
			},
		},
		{
			name: "MetricsWithConstLabels",
			descs: []*prometheus.Desc{
				prometheus.NewDesc("test_const_labels", "Test metric with constant labels", []string{"var_label"}, prometheus.Labels{"node_name": "test-node", "region": "us-west-1"}),
			},
			expectedMetricsLen: 1,
			expectedMetrics: []MetricInfo{
				{
					Name:        "test_const_labels",
					Type:        "GAUGE",
					Description: "Test metric with constant labels",
					Labels:      []string{"var_label"},
					ConstLabels: map[string]string{"node_name": "test-node", "region": "us-west-1"},
				},
			},
		},
		{
			name:               "EmptyCollector",
			descs:              []*prometheus.Desc{},
			expectedMetricsLen: 0,
			expectedMetrics:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCollector := &MockCollector{
				descs: tt.descs,
			}

			gotMetrics, err := extractMetricsInfo(mockCollector)
			assert.NoError(t, err)
			assert.Len(t, gotMetrics, tt.expectedMetricsLen)
			assert.Equal(t, tt.expectedMetrics, gotMetrics)
		})
	}
}

// TestGenerateMarkdown tests the generateMarkdown function
func TestGenerateMarkdown(t *testing.T) {
	tests := []struct {
		name             string
		metrics          []MetricInfo
		expectedMarkdown []string
	}{
		{
			name: "NodeMetrics",
			metrics: []MetricInfo{
				{
					Name:        "kepler_node_cpu_watts",
					Type:        "GAUGE",
					Description: "Power consumption of cpu at node level in watts",
					Labels:      []string{"zone", "path"},
				},
				{
					Name:        "kepler_node_cpu_joules_total",
					Type:        "COUNTER",
					Description: "Energy consumption of cpu at node level in joules",
					Labels:      []string{"zone", "path"},
				},
			},
			expectedMarkdown: []string{
				"# Kepler Metrics",
				"### Node Metrics",
				"#### kepler_node_cpu_watts",
				"#### kepler_node_cpu_joules_total",
				"- `zone`",
				"- `path`",
			},
		},
		{
			name: "MetricsWithConstLabels",
			metrics: []MetricInfo{
				{
					Name:        "kepler_node_cpu_watts",
					Type:        "GAUGE",
					Description: "Power consumption of cpu at node level in watts",
					Labels:      []string{"zone", "path"},
					ConstLabels: map[string]string{"node_name": "test-node"},
				},
			},
			expectedMarkdown: []string{
				"### Node Metrics",
				"#### kepler_node_cpu_watts",
				"- **Type**: GAUGE",
				"- **Description**: Power consumption of cpu at node level in watts",
				"- **Labels**:",
				"- `zone`",
				"- `path`",
				"- **Constant Labels**:",
				"- `node_name`",
			},
		},
		{
			name: "ContainerMetrics",
			metrics: []MetricInfo{
				{
					Name:        "kepler_container_cpu_joules_total",
					Type:        "COUNTER",
					Description: "Energy consumption of cpu at container level in joules",
					Labels:      []string{"container_id", "container_name", "runtime", "zone"},
				},
			},
			expectedMarkdown: []string{
				"### Container Metrics",
				"#### kepler_container_cpu_joules_total",
				"- `container_id`",
				"- `container_name`",
				"- `runtime`",
				"- `zone`",
			},
		},
		{
			name: "ProcessMetrics",
			metrics: []MetricInfo{
				{
					Name:        "kepler_process_cpu_watts",
					Type:        "GAUGE",
					Description: "Power consumption of cpu at process level in watts",
					Labels:      []string{"pid", "comm", "exe", "type", "container_id", "vm_id", "zone"},
				},
			},
			expectedMarkdown: []string{
				"### Process Metrics",
				"#### kepler_process_cpu_watts",
				"- `pid`",
				"- `comm`",
				"- `exe`",
				"- `type`",
				"- `container_id`",
				"- `vm_id`",
				"- `zone`",
			},
		},
		{
			name: "VirtualMachineMetrics",
			metrics: []MetricInfo{
				{
					Name:        "kepler_vm_cpu_watts",
					Type:        "GAUGE",
					Description: "Power consumption of cpu at vm level in watts",
					Labels:      []string{"vm_id", "vm_name", "hypervisor", "zone"},
				},
			},
			expectedMarkdown: []string{
				"### Virtual Machine Metrics",
				"#### kepler_vm_cpu_watts",
				"- `vm_id`",
				"- `vm_name`",
				"- `hypervisor`",
				"- `zone`",
			},
		},
		{
			name: "OtherMetrics",
			metrics: []MetricInfo{
				{
					Name:        "kepler_build_info",
					Type:        "GAUGE",
					Description: "Build information",
					Labels:      []string{"arch", "branch", "revision", "version", "goversion"},
				},
			},
			expectedMarkdown: []string{
				"### Other Metrics",
				"#### kepler_build_info",
				"- `arch`",
				"- `branch`",
				"- `revision`",
				"- `version`",
				"- `goversion`",
			},
		},
		{
			name:    "EmptyMetrics",
			metrics: []MetricInfo{},
			expectedMarkdown: []string{
				"# Kepler Metrics",
				"## Overview",
				"Kepler exports metrics in Prometheus format that can be scraped by Prometheus or other compatible monitoring systems.",
				"### Metric Types",
				"- **COUNTER**: A cumulative metric that only increases over time",
				"- **GAUGE**: A metric that can increase and decrease",
				"## Metrics Reference",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := generateMarkdown(tt.metrics)
			for _, expected := range tt.expectedMarkdown {
				if !strings.Contains(got, expected) {
					t.Errorf("generateMarkdown() got = %v, want %v", got, expected)
				}
			}
		})
	}
}

// TestWriteMetricsSection tests the writeMetricsSection function
func TestWriteMetricsSection(t *testing.T) {
	tests := []struct {
		name                string
		metrics             []MetricInfo
		expectedMarkdown    []string
		notExpectedMarkdown []string
	}{
		{
			name: "ValidMetrics",
			metrics: []MetricInfo{
				{
					Name:        "kepler_node_cpu_watts",
					Type:        "GAUGE",
					Description: "Power consumption of cpu at node level in watts",
					Labels:      []string{"zone", "path"},
				},
			},
			expectedMarkdown: []string{
				"#### kepler_node_cpu_watts",
				"- **Type**: GAUGE",
				"- **Description**: Power consumption of cpu at node level in watts",
				"- `zone`",
				"- `path`",
			},
		},
		{
			name: "MetricWithConstLabels",
			metrics: []MetricInfo{
				{
					Name:        "kepler_node_cpu_watts",
					Type:        "GAUGE",
					Description: "Power consumption of cpu at node level in watts",
					Labels:      []string{"zone", "path"},
					ConstLabels: map[string]string{"node_name": "test-node", "region": "us-west-1"},
				},
			},
			expectedMarkdown: []string{
				"#### kepler_node_cpu_watts",
				"- **Type**: GAUGE",
				"- **Description**: Power consumption of cpu at node level in watts",
				"- **Labels**:",
				"- `zone`",
				"- `path`",
				"- **Constant Labels**:",
				"- `node_name`",
				"- `region`",
			},
			notExpectedMarkdown: []string{
				"- `node_name=test-node`",
				"- `region=us-west-1`",
			},
		},
		{
			name: "MetricWithoutLabels",
			metrics: []MetricInfo{
				{
					Name:        "kepler_node_cpu_joules_total",
					Type:        "COUNTER",
					Description: "Energy consumption of cpu at node level in joules",
					Labels:      []string{},
				},
			},
			expectedMarkdown: []string{
				"#### kepler_node_cpu_joules_total",
				"- **Type**: COUNTER",
				"- **Description**: Energy consumption of cpu at node level in joules",
			},
			notExpectedMarkdown: []string{"- **Labels**:"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var md strings.Builder
			writeMetricsSection(&md, tt.metrics)
			output := md.String()
			for _, expected := range tt.expectedMarkdown {
				if !strings.Contains(output, expected) {
					t.Errorf("writeMetricsSection() got = %v, want %v", output, expected)
				}
			}
			for _, notExpected := range tt.notExpectedMarkdown {
				if strings.Contains(output, notExpected) {
					t.Errorf("writeMetricsSection() got = %v, want %v", output, notExpected)
				}
			}
		})
	}
}

// TestMainFunction tests the main function behavior
func TestMainFunction(t *testing.T) {
	origStdout := os.Stdout
	defer func() { os.Stdout = origStdout }()

	origWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	//nolint:errcheck
	defer func() { os.Chdir(origWd) }()

	tempDir := t.TempDir()
	err = os.Chdir(tempDir)
	if err != nil {
		t.Fatal(err)
	}

	r, w, _ := os.Pipe()
	os.Stdout = w

	done := make(chan bool)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				t.Logf("main() panicked: %v", r)
			}
			done <- true
		}()
		main()
	}()

	<-done
	//nolint:errcheck
	w.Close()

	var buf bytes.Buffer
	_, err = io.Copy(&buf, r)
	if err != nil {
		t.Errorf("failed to capture stdout: %v", err)
	}
	//nolint:errcheck
	r.Close()
	output := buf.String()

	outputFile := filepath.Join(tempDir, "metrics.md")
	if _, err := os.Stat(outputFile); os.IsNotExist(err) {
		t.Errorf("output file %q was not created", outputFile)
	} else {
		content, err := os.ReadFile(outputFile)
		if err != nil {
			t.Errorf("failed to read output file: %v", err)
		}
		if !strings.Contains(string(content), "# Kepler Metrics") {
			t.Errorf("output file %q missing expected content", outputFile)
		}
	}

	wantLogMsgs := []string{
		"Starting Kepler metrics extractor",
		"Creating collectors",
	}
	for _, msg := range wantLogMsgs {
		if !strings.Contains(output, msg) {
			t.Errorf("missing log message %q in output:\n%s", msg, output)
		}
	}
}
