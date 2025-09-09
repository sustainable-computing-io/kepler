// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sustainable-computing-io/kepler/config"
	"github.com/sustainable-computing-io/kepler/internal/device"
	"github.com/sustainable-computing-io/kepler/internal/exporter/prometheus/collector"
	"github.com/sustainable-computing-io/kepler/internal/monitor"
	"github.com/sustainable-computing-io/kepler/internal/platform/redfish"
)

// MetricInfo holds information about a Prometheus metric
type MetricInfo struct {
	Name        string
	Type        string
	Description string
	Labels      []string
	ConstLabels map[string]string
}

// MockMonitor implements the minimal interface needed by collectors
type MockMonitor struct {
	dataChan chan struct{}
}

func (m *MockMonitor) DataChannel() <-chan struct{} {
	return m.dataChan
}

func (m *MockMonitor) Snapshot() (*monitor.Snapshot, error) {
	return &monitor.Snapshot{}, nil
}

// ZoneNames implements monitor.PowerDataProvider interface
func (m *MockMonitor) ZoneNames() []string {
	return []string{"package-0"}
}

// LastCollectionTime implements monitor.PowerDataProvider interface
func (m *MockMonitor) LastCollectionTime() time.Time {
	return time.Now()
}

// MockRedfishService implements collector.RedfishDataProvider interface
// Uses real test data from fixtures to generate realistic metrics documentation
type MockRedfishService struct {
	nodeName string
	bmcID    string
}

func (m *MockRedfishService) Power() (*redfish.PowerReading, error) {
	// Create a realistic power reading using test scenario data
	// This represents a typical multi-chassis server with different power controls
	return &redfish.PowerReading{
		Timestamp: time.Now(),
		Chassis: []redfish.Chassis{
			{
				ID: "System.Embedded.1",
				Readings: []redfish.Reading{
					{
						ControlID: "PC1",
						Name:      "System Power Control",
						Power:     245.0 * device.Watt, // Dell 245W scenario
					},
				},
			},
			{
				ID: "Enclosure.Internal.0-1",
				Readings: []redfish.Reading{
					{
						ControlID: "PC1",
						Name:      "Enclosure Power Control",
						Power:     189.5 * device.Watt, // HPE 189W scenario
					},
					{
						ControlID: "PC2",
						Name:      "CPU Sub-system Power",
						Power:     167.8 * device.Watt, // Lenovo 167W scenario
					},
				},
			},
		},
	}, nil
}

func (m *MockRedfishService) NodeName() string {
	return m.nodeName
}

func (m *MockRedfishService) BMCID() string {
	return m.bmcID
}

// DescCollector is a helper struct to collect metric descriptions
type DescCollector struct {
	descs []*prometheus.Desc
}

func (c *DescCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, desc := range c.descs {
		ch <- desc
	}
}

func (c *DescCollector) Collect(ch chan<- prometheus.Metric) {
	// Intentionally empty as we only care about descriptions
}

// extractMetricsInfo extracts metric information from a Prometheus collector
func extractMetricsInfo(collector prometheus.Collector) ([]MetricInfo, error) {
	ch := make(chan *prometheus.Desc, 100)
	collector.Describe(ch)
	close(ch)

	var metrics []MetricInfo
	fqNameRegex := regexp.MustCompile(`fqName: "([^"]+)"`)
	helpRegex := regexp.MustCompile(`help: "([^"]+)"`)
	variableLabelsRegex := regexp.MustCompile(`variableLabels: \{([^}]*)\}`)
	constLabelsRegex := regexp.MustCompile(`constLabels: \{([^}]*)\}`)

	for desc := range ch {
		descStr := desc.String()
		fqNameMatch := fqNameRegex.FindStringSubmatch(descStr)
		if len(fqNameMatch) < 2 {
			fmt.Printf("Warning: Could not parse fqName from: %s\n", descStr)
			continue
		}
		name := fqNameMatch[1]

		helpMatch := helpRegex.FindStringSubmatch(descStr)
		if len(helpMatch) < 2 {
			fmt.Printf("Warning: Could not parse help from: %s\n", descStr)
			continue
		}
		help := helpMatch[1]

		var labels []string
		variableLabelsMatch := variableLabelsRegex.FindStringSubmatch(descStr)
		if len(variableLabelsMatch) >= 2 && variableLabelsMatch[1] != "" {
			labelsStr := variableLabelsMatch[1]
			if labelsStr != "" {
				labels = strings.Split(labelsStr, ",")
				for i, label := range labels {
					labels[i] = strings.TrimSpace(label)
				}
			}
		}

		constLabels := make(map[string]string)
		constLabelsMatch := constLabelsRegex.FindStringSubmatch(descStr)
		if len(constLabelsMatch) >= 2 && constLabelsMatch[1] != "" {
			constLabelsStr := constLabelsMatch[1]
			// Parse const labels which are in format: labelName="labelValue"
			labelPairRegex := regexp.MustCompile(`(\w+)="([^"]*)"`)
			matches := labelPairRegex.FindAllStringSubmatch(constLabelsStr, -1)
			for _, match := range matches {
				if len(match) >= 3 {
					constLabels[match[1]] = match[2]
				}
			}
		}

		metricType := "GAUGE"
		if strings.HasSuffix(name, "_total") {
			metricType = "COUNTER"
		}

		metrics = append(metrics, MetricInfo{
			Name:        name,
			Type:        metricType,
			Description: help,
			Labels:      labels,
			ConstLabels: constLabels,
		})
	}

	return metrics, nil
}

// generateMarkdown generates Markdown documentation from metric information
func generateMarkdown(metrics []MetricInfo) string {
	var md strings.Builder
	sort.Slice(metrics, func(i, j int) bool {
		return metrics[i].Name < metrics[j].Name
	})

	md.WriteString("# Kepler Metrics\n\n")
	md.WriteString("This document describes the metrics exported by Kepler for monitoring energy consumption at various levels (node, container, process, VM).\n\n")
	md.WriteString("## Overview\n\n")
	md.WriteString("Kepler exports metrics in Prometheus format that can be scraped by Prometheus or other compatible monitoring systems.\n\n")
	md.WriteString("### Metric Types\n\n")
	md.WriteString("- **COUNTER**: A cumulative metric that only increases over time\n")
	md.WriteString("- **GAUGE**: A metric that can increase and decrease\n\n")
	md.WriteString("## Metrics Reference\n\n")

	nodeMetrics := []MetricInfo{}
	containerMetrics := []MetricInfo{}
	processMetrics := []MetricInfo{}
	vmMetrics := []MetricInfo{}
	podMetrics := []MetricInfo{}
	experimentalMetrics := []MetricInfo{}
	otherMetrics := []MetricInfo{}

	for _, metric := range metrics {
		switch {
		case strings.HasPrefix(metric.Name, "kepler_platform_"):
			// Platform metrics are experimental
			experimentalMetrics = append(experimentalMetrics, metric)
		case strings.HasPrefix(metric.Name, "kepler_node_"):
			nodeMetrics = append(nodeMetrics, metric)
		case strings.HasPrefix(metric.Name, "kepler_container_"):
			containerMetrics = append(containerMetrics, metric)
		case strings.HasPrefix(metric.Name, "kepler_process_"):
			processMetrics = append(processMetrics, metric)
		case strings.HasPrefix(metric.Name, "kepler_vm_"):
			vmMetrics = append(vmMetrics, metric)
		case strings.HasPrefix(metric.Name, "kepler_pod_"):
			podMetrics = append(podMetrics, metric)
		default:
			otherMetrics = append(otherMetrics, metric)
		}
	}

	if len(nodeMetrics) > 0 {
		md.WriteString("### Node Metrics\n\n")
		md.WriteString("These metrics provide energy and power information at the node level.\n\n")
		writeMetricsSection(&md, nodeMetrics)
	}
	if len(containerMetrics) > 0 {
		md.WriteString("### Container Metrics\n\n")
		md.WriteString("These metrics provide energy and power information for containers.\n\n")
		writeMetricsSection(&md, containerMetrics)
	}
	if len(processMetrics) > 0 {
		md.WriteString("### Process Metrics\n\n")
		md.WriteString("These metrics provide energy and power information for individual processes.\n\n")
		writeMetricsSection(&md, processMetrics)
	}
	if len(vmMetrics) > 0 {
		md.WriteString("### Virtual Machine Metrics\n\n")
		md.WriteString("These metrics provide energy and power information for virtual machines.\n\n")
		writeMetricsSection(&md, vmMetrics)
	}
	if len(podMetrics) > 0 {
		md.WriteString("### Pod Metrics\n\n")
		md.WriteString("These metrics provide energy and power information for pods.\n\n")
		writeMetricsSection(&md, podMetrics)
	}
	if len(otherMetrics) > 0 {
		md.WriteString("### Other Metrics\n\n")
		md.WriteString("Additional metrics provided by Kepler.\n\n")
		writeMetricsSection(&md, otherMetrics)
	}

	// Add experimental section
	if len(experimentalMetrics) > 0 {
		md.WriteString("## Experimental Metrics\n\n")
		md.WriteString("⚠️ **Warning**: The following metrics are experimental and may change or be removed in future versions. ")
		md.WriteString("They are provided for early testing and feedback purposes.\n\n")
		md.WriteString("### Platform Power Metrics\n\n")
		md.WriteString("These experimental metrics provide platform-level power information from BMC sources (e.g., Redfish). ")
		md.WriteString("Enable the experimental Redfish feature to collect these metrics.\n\n")
		writeMetricsSection(&md, experimentalMetrics)
	}

	md.WriteString("---\n\n")
	md.WriteString("This documentation was automatically generated by the gen-metric-docs tool.")
	md.WriteString("\n")
	return md.String()
}

// writeMetricsSection writes a section of metrics to the markdown builder
func writeMetricsSection(md *strings.Builder, metrics []MetricInfo) {
	for _, metric := range metrics {
		fmt.Fprintf(md, "#### %s\n\n", metric.Name)
		fmt.Fprintf(md, "- **Type**: %s\n", metric.Type)
		fmt.Fprintf(md, "- **Description**: %s\n", metric.Description)
		if len(metric.Labels) > 0 {
			md.WriteString("- **Labels**:\n")
			for _, label := range metric.Labels {
				fmt.Fprintf(md, "  - `%s`\n", label)
			}
		}
		if len(metric.ConstLabels) > 0 {
			md.WriteString("- **Constant Labels**:\n")
			// Sort constant labels for consistent output
			var keys []string
			for key := range metric.ConstLabels {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				fmt.Fprintf(md, "  - `%s`\n", key)
			}
		}
		md.WriteString("\n")
	}
}

func main() {
	outputPath := flag.String("output", "metrics.md", "Path to output Markdown file")
	flag.Parse()

	fmt.Println("Starting Kepler metrics extractor...")

	// Get current working directory
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Printf("Failed to get current working directory: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Current directory: %s\n", cwd)

	// Create a mock monitor for the collectors
	mockMonitor := &MockMonitor{
		dataChan: make(chan struct{}),
	}
	close(mockMonitor.dataChan)

	// Create the collectors
	fmt.Println("Creating collectors...")
	// Create a logger for the collectors
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	powerCollector := collector.NewPowerCollector(mockMonitor, "test-node", logger, config.MetricsLevelAll)
	fmt.Println("Created power collector")
	buildInfoCollector := collector.NewKeplerBuildInfoCollector()
	fmt.Println("Created build info collector")
	cpuInfoCollector, err := collector.NewCPUInfoCollector("/proc")
	if err != nil {
		fmt.Printf("Warning: Could not create CPU info collector: %v\n", err)
	} else {
		fmt.Println("Created CPU info collector")
	}

	// Extract metrics information from collectors
	var allMetrics []MetricInfo

	fmt.Println("Extracting metrics from power collector...")
	powerMetrics, err := extractMetricsInfo(powerCollector)
	if err != nil {
		fmt.Printf("Failed to extract power metrics: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Extracted %d power metrics\n", len(powerMetrics))
	allMetrics = append(allMetrics, powerMetrics...)

	fmt.Println("Extracting metrics from build info collector...")
	buildInfoMetrics, err := extractMetricsInfo(buildInfoCollector)
	if err != nil {
		fmt.Printf("Failed to extract build info metrics: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Extracted %d build info metrics\n", len(buildInfoMetrics))
	allMetrics = append(allMetrics, buildInfoMetrics...)

	if cpuInfoCollector != nil {
		fmt.Println("Extracting metrics from CPU info collector...")
		cpuInfoMetrics, err := extractMetricsInfo(cpuInfoCollector)
		if err != nil {
			fmt.Printf("Failed to extract CPU info metrics: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Extracted %d CPU info metrics\n", len(cpuInfoMetrics))
		allMetrics = append(allMetrics, cpuInfoMetrics...)
	}

	// Create mock redfish service for platform collector
	mockRedfish := &MockRedfishService{
		nodeName: "test-node",
		bmcID:    "test-bmc",
	}
	fmt.Println("Creating platform collector...")
	platformCollector := collector.NewRedfishCollector(mockRedfish, logger)
	fmt.Println("Created platform collector")

	fmt.Println("Extracting metrics from platform collector...")
	platformMetrics, err := extractMetricsInfo(platformCollector)
	if err != nil {
		fmt.Printf("Failed to extract platform metrics: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Extracted %d platform metrics\n", len(platformMetrics))
	allMetrics = append(allMetrics, platformMetrics...)

	fmt.Printf("Total metrics extracted: %d\n", len(allMetrics))

	// Generate Markdown
	markdown := generateMarkdown(allMetrics)

	// Set the output path - using the current directory
	defaultOutputPath := *outputPath
	fmt.Printf("Writing metrics documentation to: %s\n", defaultOutputPath)

	// Create output directory if it doesn't exist
	outputDir := filepath.Dir(defaultOutputPath)
	if outputDir != "" && outputDir != "." {
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			fmt.Printf("Failed to create output directory: %v\n", err)
			os.Exit(1)
		}
	}

	// Write to file
	err = os.WriteFile(defaultOutputPath, []byte(markdown), 0644)
	if err != nil {
		fmt.Printf("Failed to write markdown file: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Metrics documentation generated successfully!")
}
