// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package collector

import (
	"errors"
	"sync"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/procfs"
	"github.com/stretchr/testify/assert"
)

// mockProcFS is a mock implementation of the procFS interface for testing.
type mockProcFS struct {
	cpuInfoFunc func() ([]procfs.CPUInfo, error)
}

func (m *mockProcFS) CPUInfo() ([]procfs.CPUInfo, error) {
	return m.cpuInfoFunc()
}

// sampleCPUInfo returns a sample CPUInfo slice for testing.
func sampleCPUInfo() []procfs.CPUInfo {
	return []procfs.CPUInfo{
		{
			Processor:  0,
			VendorID:   "GenuineIntel",
			ModelName:  "Intel(R) Core(TM) i7-9750H CPU @ 2.60GHz",
			PhysicalID: "0",
			CoreID:     "0",
		},
		{
			Processor:  1,
			VendorID:   "GenuineIntel",
			ModelName:  "Intel(R) Core(TM) i7-9750H CPU @ 2.60GHz",
			PhysicalID: "0",
			CoreID:     "1",
		},
	}
}

func expectedLabels() map[string]string {
	return map[string]string{
		"processor":   "",
		"vendor_id":   "",
		"model_name":  "",
		"physical_id": "",
		"core_id":     "",
	}
}

// TestNewCPUInfoCollector tests the creation of a new CPUInfoCollector.
func TestNewCPUInfoCollector(t *testing.T) {
	// Test successful creation with a mock procfs
	collector, err := NewCPUInfoCollector("/proc")
	assert.NoError(t, err)
	assert.NotNil(t, collector)
	assert.NotNil(t, collector.fs)
	assert.NotNil(t, collector.desc)
}

// TestNewCPUInfoCollectorWithFS tests the creation with an injected procFS.
func TestNewCPUInfoCollectorWithFS(t *testing.T) {
	mockFS := &mockProcFS{
		cpuInfoFunc: func() ([]procfs.CPUInfo, error) {
			return sampleCPUInfo(), nil
		},
	}
	collector := newCPUInfoCollectorWithFS(mockFS)
	assert.NotNil(t, collector)
	assert.Equal(t, mockFS, collector.fs)
	assert.NotNil(t, collector.desc)
	assert.Contains(t, collector.desc.String(), "kepler_node_cpu_info")
	assert.Contains(t, collector.desc.String(), "variableLabels: {processor,vendor_id,model_name,physical_id,core_id}")
}

// TestCPUInfoCollector_Describe tests the Describe method.
func TestCPUInfoCollector_Describe(t *testing.T) {
	mockFS := &mockProcFS{
		cpuInfoFunc: func() ([]procfs.CPUInfo, error) {
			return sampleCPUInfo(), nil
		},
	}
	collector := newCPUInfoCollectorWithFS(mockFS)

	ch := make(chan *prometheus.Desc, 1)
	collector.Describe(ch)
	close(ch)

	desc := <-ch
	assert.Equal(t, collector.desc, desc)
}

// TestCPUInfoCollector_Collect_Success tests the Collect method with valid CPU info.
func TestCPUInfoCollector_Collect_Success(t *testing.T) {
	mockFS := &mockProcFS{
		cpuInfoFunc: func() ([]procfs.CPUInfo, error) {
			return sampleCPUInfo(), nil
		},
	}
	collector := newCPUInfoCollectorWithFS(mockFS)

	ch := make(chan prometheus.Metric, 10)
	collector.Collect(ch)
	close(ch)

	var metrics []prometheus.Metric
	for m := range ch {
		metrics = append(metrics, m)
	}

	assert.Len(t, metrics, 2, "expected two CPU info metrics")

	el := expectedLabels()

	for _, m := range metrics {
		dtoMetric := &dto.Metric{}
		err := m.Write(dtoMetric)
		assert.NoError(t, err)
		assert.NotNil(t, dtoMetric.Gauge)
		assert.NotNil(t, dtoMetric.Gauge.Value)
		assert.Equal(t, 1.0, *dtoMetric.Gauge.Value)
		assert.NotNil(t, dtoMetric.Label)
		for _, l := range dtoMetric.Label {
			assert.NotNil(t, l.Name)
			delete(el, *l.Name)
		}
	}
	assert.Empty(t, el, "all expected labels not received")
}

// TestCPUInfoCollector_Collect_Error tests the Collect method when CPUInfo fails.
func TestCPUInfoCollector_Collect_Error(t *testing.T) {
	mockFS := &mockProcFS{
		cpuInfoFunc: func() ([]procfs.CPUInfo, error) {
			return nil, errors.New("failed to read CPU info")
		},
	}
	collector := newCPUInfoCollectorWithFS(mockFS)

	ch := make(chan prometheus.Metric, 10)
	collector.Collect(ch)
	close(ch)

	var metrics []prometheus.Metric
	for m := range ch {
		metrics = append(metrics, m)
	}

	assert.Len(t, metrics, 0, "expected no metrics on error")
}

// TestCPUInfoCollector_Collect_Concurrency tests concurrent calls to Collect.
func TestCPUInfoCollector_Collect_Concurrency(t *testing.T) {
	mockFS := &mockProcFS{
		cpuInfoFunc: func() ([]procfs.CPUInfo, error) {
			return sampleCPUInfo(), nil
		},
	}
	collector := newCPUInfoCollectorWithFS(mockFS)

	const numGoroutines = 10
	var wg sync.WaitGroup
	ch := make(chan prometheus.Metric, numGoroutines*len(sampleCPUInfo()))

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

	// Expect numGoroutines * number of CPUs metrics
	expectedMetrics := numGoroutines * len(sampleCPUInfo())
	assert.Equal(t, expectedMetrics, len(metrics), "expected metrics from all goroutines")
}
