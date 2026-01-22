// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package collector

import (
	"errors"
	"sync"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/sustainable-computing-io/kepler/internal/monitor"
)

// sampleGPUStats returns sample GPU stats for testing.
func sampleGPUStats() []monitor.GPUDeviceStats {
	return []monitor.GPUDeviceStats{
		{
			DeviceIndex: 0,
			UUID:        "GPU-12345678-1234-1234-1234-123456789abc",
			Name:        "NVIDIA A100-SXM4-40GB",
			Vendor:      "nvidia",
			TotalPower:  150.5,
			IdlePower:   25.0,
			ActivePower: 125.5,
		},
		{
			DeviceIndex: 1,
			UUID:        "GPU-87654321-4321-4321-4321-cba987654321",
			Name:        "NVIDIA A100-SXM4-40GB",
			Vendor:      "nvidia",
			TotalPower:  180.0,
			IdlePower:   25.0,
			ActivePower: 155.0,
		},
	}
}

// TestNewGPUInfoCollector tests the creation of a new GPUInfoCollector.
func TestNewGPUInfoCollector(t *testing.T) {
	mockPM := NewMockPowerMonitor()
	collector := NewGPUInfoCollector(mockPM, "test-node")

	assert.NotNil(t, collector)
	assert.NotNil(t, collector.desc)
	assert.Equal(t, "test-node", collector.nodeName)
	assert.Contains(t, collector.desc.String(), "kepler_node_gpu_info")
	assert.Contains(t, collector.desc.String(), "variableLabels: {gpu,gpu_uuid,gpu_name,vendor}")
}

// TestGPUInfoCollector_Describe tests the Describe method.
func TestGPUInfoCollector_Describe(t *testing.T) {
	mockPM := NewMockPowerMonitor()
	collector := NewGPUInfoCollector(mockPM, "test-node")

	ch := make(chan *prometheus.Desc, 1)
	collector.Describe(ch)
	close(ch)

	desc := <-ch
	assert.Equal(t, collector.desc, desc)
}

// TestGPUInfoCollector_Collect_Success tests the Collect method with valid GPU stats.
func TestGPUInfoCollector_Collect_Success(t *testing.T) {
	mockPM := NewMockPowerMonitor()
	snapshot := monitor.NewSnapshot()
	snapshot.GPUStats = sampleGPUStats()
	mockPM.On("Snapshot").Return(snapshot, nil)

	collector := NewGPUInfoCollector(mockPM, "test-node")

	ch := make(chan prometheus.Metric, 10)
	collector.Collect(ch)
	close(ch)

	var metrics []prometheus.Metric
	for m := range ch {
		metrics = append(metrics, m)
	}

	assert.Len(t, metrics, 2, "expected two GPU info metrics")

	// Verify metrics have correct labels
	for i, m := range metrics {
		dtoMetric := &dto.Metric{}
		err := m.Write(dtoMetric)
		assert.NoError(t, err)
		assert.NotNil(t, dtoMetric.Gauge)
		assert.NotNil(t, dtoMetric.Gauge.Value)
		assert.Equal(t, 1.0, *dtoMetric.Gauge.Value)

		// Check labels are present
		labels := make(map[string]string)
		for _, l := range dtoMetric.Label {
			labels[*l.Name] = *l.Value
		}

		expectedStats := sampleGPUStats()[i]
		assert.Equal(t, expectedStats.UUID, labels["gpu_uuid"])
		assert.Equal(t, expectedStats.Name, labels["gpu_name"])
		assert.Equal(t, expectedStats.Vendor, labels["vendor"])
	}
}

// TestGPUInfoCollector_Collect_Error tests the Collect method when Snapshot fails.
func TestGPUInfoCollector_Collect_Error(t *testing.T) {
	mockPM := NewMockPowerMonitor()
	mockPM.On("Snapshot").Return((*monitor.Snapshot)(nil), errors.New("snapshot error"))

	collector := NewGPUInfoCollector(mockPM, "test-node")

	ch := make(chan prometheus.Metric, 10)
	collector.Collect(ch)
	close(ch)

	var metrics []prometheus.Metric
	for m := range ch {
		metrics = append(metrics, m)
	}

	assert.Len(t, metrics, 0, "expected no metrics on error")
}

// TestGPUInfoCollector_Collect_NoGPUs tests the Collect method when no GPUs are available.
func TestGPUInfoCollector_Collect_NoGPUs(t *testing.T) {
	mockPM := NewMockPowerMonitor()
	snapshot := monitor.NewSnapshot()
	snapshot.GPUStats = nil
	mockPM.On("Snapshot").Return(snapshot, nil)

	collector := NewGPUInfoCollector(mockPM, "test-node")

	ch := make(chan prometheus.Metric, 10)
	collector.Collect(ch)
	close(ch)

	var metrics []prometheus.Metric
	for m := range ch {
		metrics = append(metrics, m)
	}

	assert.Len(t, metrics, 0, "expected no metrics when no GPUs")
}

// TestGPUInfoCollector_Collect_Concurrency tests concurrent calls to Collect.
func TestGPUInfoCollector_Collect_Concurrency(t *testing.T) {
	mockPM := NewMockPowerMonitor()
	snapshot := monitor.NewSnapshot()
	snapshot.GPUStats = sampleGPUStats()
	mockPM.On("Snapshot").Return(snapshot, nil)

	collector := NewGPUInfoCollector(mockPM, "test-node")

	const numGoroutines = 10
	var wg sync.WaitGroup
	ch := make(chan prometheus.Metric, numGoroutines*len(sampleGPUStats()))

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			collector.Collect(ch)
		}()
	}

	wg.Wait()
	close(ch)

	var metrics []prometheus.Metric
	for m := range ch {
		metrics = append(metrics, m)
	}

	// Expect numGoroutines * number of GPUs metrics
	expectedMetrics := numGoroutines * len(sampleGPUStats())
	assert.Equal(t, expectedMetrics, len(metrics), "expected metrics from all goroutines")
}
