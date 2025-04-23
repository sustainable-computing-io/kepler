package collector

import (
	"fmt"
	"sync"
	"testing"

	prom "github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/procfs/sysfs"
	"github.com/stretchr/testify/assert"
)

// mockSysFS is a mock implementation of the sysFS interface for testing.
type mockSysFS struct {
	zonesFunc func() ([]sysfs.RaplZone, error)
}

func (m *mockSysFS) Zones() ([]sysfs.RaplZone, error) {
	return m.zonesFunc()
}

// sampleEnergyZoneInfo returns a sample RaplZone slice for testing.
func sampleEnergyZoneInfo() []sysfs.RaplZone {
	return []sysfs.RaplZone{{
		Name:           "package-0",
		Index:          0,
		Path:           "/sys/class/powercap/intel-rapl:0",
		MaxMicrojoules: 262143328850.0,
	}, {
		Name:           "dram",
		Index:          0,
		Path:           "/sys/class/powercap/intel-rapl:0:0",
		MaxMicrojoules: 262143328850.0,
	}}
}

func expectedEnergyZoneLabels() map[string]string {
	return map[string]string{
		"name":  "",
		"index": "",
		"path":  "",
	}
}

// TestNewEnergyZoneCollector tests creation of EnergyZone Collector
func TestNewEnergyZoneCollector(t *testing.T) {
	collector, err := NewEnergyZoneCollector("/sys")
	assert.NoError(t, err)
	assert.NotNil(t, collector)
	assert.NotNil(t, collector.sysfs)
	assert.NotNil(t, collector.desc)
}

// TestNewEnergyZoneCollectorWithFS tests creator creation with injected sysfs
func TestNewEnergyZoneCollectorWithFS(t *testing.T) {
	mockFS := &mockSysFS{
		zonesFunc: func() ([]sysfs.RaplZone, error) {
			return sampleEnergyZoneInfo(), nil
		},
	}
	collector := newEnergyCollectorWithFS(mockFS)
	assert.NotNil(t, collector)
	assert.Equal(t, mockFS, collector.sysfs)
	assert.NotNil(t, collector.desc)
	assert.Contains(t, collector.desc.String(), "kepler_node_rapl_zone")
	assert.Contains(t, collector.desc.String(), "variableLabels: {name,index,path}")
}

// TestEnergyZoneCollector_Describe tests the Describe method.
func TestEnergyZoneCollector_Describe(t *testing.T) {
	mockFS := &mockSysFS{
		zonesFunc: func() ([]sysfs.RaplZone, error) {
			return sampleEnergyZoneInfo(), nil
		},
	}
	collector := newEnergyCollectorWithFS(mockFS)

	ch := make(chan *prom.Desc, 1)
	collector.Describe(ch)
	close(ch)

	desc := <-ch
	assert.Equal(t, collector.desc, desc)
}

func TestEnergyZoneCollector_Collect_Success(t *testing.T) {
	mockFS := &mockSysFS{
		zonesFunc: func() ([]sysfs.RaplZone, error) {
			return sampleEnergyZoneInfo(), nil
		},
	}
	collector := newEnergyCollectorWithFS(mockFS)

	ch := make(chan prom.Metric, 10)
	collector.Collect(ch)
	close(ch)

	var metrics []prom.Metric
	for m := range ch {
		metrics = append(metrics, m)
	}

	assert.Len(t, metrics, 2, "expected two zone info")

	el := expectedEnergyZoneLabels()

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

func TestEnergyZoneCollector_Collect_Error(t *testing.T) {
	mockFS := &mockSysFS{
		zonesFunc: func() ([]sysfs.RaplZone, error) {
			return nil, fmt.Errorf("cannot read zones")
		},
	}
	collector := newEnergyCollectorWithFS(mockFS)

	ch := make(chan prom.Metric, 10)
	collector.Collect(ch)
	close(ch)

	var metrics []prom.Metric
	for m := range ch {
		metrics = append(metrics, m)
	}

	assert.Len(t, metrics, 0, "expected no metrics on error")
}

func TestEnergyZoneCollector_Collect_Concurrency(t *testing.T) {
	mockFS := &mockSysFS{
		zonesFunc: func() ([]sysfs.RaplZone, error) {
			return sampleEnergyZoneInfo(), nil
		},
	}
	collector := newEnergyCollectorWithFS(mockFS)

	const numGoroutines = 10
	var wg sync.WaitGroup
	ch := make(chan prom.Metric, numGoroutines*len(sampleCPUInfo()))

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			collector.Collect(ch)
		}()
	}

	wg.Wait()
	close(ch)

	var metrics []prom.Metric
	for m := range ch {
		metrics = append(metrics, m)
	}

	// Expect numGoroutines * number of CPUs metrics
	expectedMetrics := numGoroutines * len(sampleCPUInfo())
	assert.Equal(t, expectedMetrics, len(metrics), "expected metrics from all goroutines")
}
