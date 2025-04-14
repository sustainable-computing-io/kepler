// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package collectors

import (
	"errors"
	"sync"
	"testing"

	prom "github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
)

// mockCmd simulates command output
type mockCmd struct {
	output string
	err    error
}

// mockCommandRunner implements CommandRunner for testing
type mockCommandRunner struct {
	outputs   map[string]*mockCmd
	callCount int
}

func (m *mockCommandRunner) Run(name string, args ...string) ([]byte, error) {
	m.callCount++
	if name == "lscpu" {
		if len(args) == 2 && args[0] == "--sysroot" {
			if mock, ok := m.outputs[args[1]]; ok {
				return []byte(mock.output), mock.err
			}
			return nil, errors.New("unknown sysroot")
		}
		if len(args) == 0 {
			return []byte(m.outputs["nil"].output), m.outputs["nil"].err
		}
	}
	return nil, errors.New("invalid command")
}

// mockSysrootOutputs maps sysroot paths to lscpu outputs
var mockSysrootOutputs = map[string]*mockCmd{
	"/host": {
		output: `
Architecture:             x86_64
  CPU op-mode(s):         32-bit, 64-bit
  Address sizes:          39 bits physical, 48 bits virtual
  Byte Order:             Little Endian
CPU(s):                   16
  On-line CPU(s) list:    0-15
Vendor ID:                GenuineIntel
  Model name:             11th Gen Intel(R) Core(TM) i7-11850H @ 2.50GHz
    CPU family:           6
    Model:                141
    Thread(s) per core:   2
    Core(s) per socket:   8
    Socket(s):            1
    Stepping:             1
    CPU(s) scaling MHz:   33%
    CPU max MHz:          4800.0000
    CPU min MHz:          800.0000
    BogoMIPS:             4992.00
`,
		err: nil,
	},
	"nil": {
		output: `
Architecture:             x86_64
  CPU op-mode(s):         32-bit, 64-bit
  Address sizes:          39 bits physical, 48 bits virtual
  Byte Order:             Little Endian
CPU(s):                   4
  On-line CPU(s) list:    0-3
Vendor ID:                GenuineIntel
  Model name:             Intel(R) Core(TM) i5-6600T CPU @ 2.70GHz
    CPU family:           6
    Model:                94
    Thread(s) per core:   1
    Core(s) per socket:   4
    Socket(s):            1
    Stepping:             3
    CPU(s) scaling MHz:   78%
    CPU max MHz:          3500.0000
    CPU min MHz:          800.0000
    BogoMIPS:             5399.81
`,
		err: nil,
	},
	"/invalid": {
		output: `lscpu: failed to determine number of CPUs: /sys/devices/system/cpu/possible: No such file or directory`,
		err:    errors.New("invalid sysroot"),
	},
}

func newMockRunner() cmdRunner {
	return &mockCommandRunner{
		outputs: mockSysrootOutputs,
	}
}

func TestNewCpuInfoCollector(t *testing.T) {
	ci := NewCpuInfoCollector("/host")

	assert.Equal(t, "/host", ci.sysroot, "sysroot should be set correctly")
	assert.NotNil(t, ci.desc, "descriptor should be initialized")
	assert.NotNil(t, ci.cache, "cache should be initialized")
}

func TestDescribe(t *testing.T) {
	ci := NewCpuInfoCollector("/host")
	ch := make(chan *prom.Desc, 1)

	ci.Describe(ch)
	close(ch)

	desc := <-ch
	assert.NotNil(t, desc, "should yield a descriptor")
	assert.Equal(t, ci.desc, desc, "should yield the collector's descriptor")
}

func TestCollectSuccess(t *testing.T) {

	ci := NewCpuInfoCollector("/host")
	ci.commandRunner = newMockRunner()
	ch := make(chan prom.Metric, 1)

	ci.Collect(ch)
	close(ch)

	metric := <-ch
	assert.NotNil(t, metric, "should yield a metric")

	var m dto.Metric
	assert.NoError(t, metric.Write(&m), "should write metric successfully")

	assert.Equal(t, 1.0, m.Gauge.GetValue(), "gauge value should be 1.0")

	expectedLabels := map[string]string{
		architecture:     "x86_64",
		model_name:       "11th Gen Intel(R) Core(TM) i7-11850H @ 2.50GHz",
		cpus:             "16",
		cores_per_socket: "8",
		sockets:          "1",
		vendor_id:        "GenuineIntel",
	}

	actualLabels := make(map[string]string)
	for _, lp := range m.Label {
		actualLabels[lp.GetName()] = lp.GetValue()
	}

	for name, expected := range expectedLabels {
		value, ok := actualLabels[name]
		assert.True(t, ok, "label %q should exist", name)
		assert.Equal(t, expected, value, "label %q value mismatch", name)
	}
	for name := range actualLabels {
		_, ok := expectedLabels[name]
		assert.True(t, ok, "unexpected label %q", name)
	}
}

func TestCollectDifferentSysroot(t *testing.T) {

	ci := NewCpuInfoCollector("")
	ci.commandRunner = newMockRunner()
	ch := make(chan prom.Metric, 1)

	ci.Collect(ch)
	close(ch)

	metric := <-ch
	assert.NotNil(t, metric, "should yield a metric")

	var m dto.Metric
	assert.NoError(t, metric.Write(&m), "should write metric successfully")

	assert.Equal(t, 1.0, m.Gauge.GetValue(), "gauge value should be 1.0")

	expectedLabels := map[string]string{
		architecture:     "x86_64",
		model_name:       "Intel(R) Core(TM) i5-6600T CPU @ 2.70GHz",
		cpus:             "4",
		cores_per_socket: "4",
		sockets:          "1",
		vendor_id:        "GenuineIntel",
	}

	actualLabels := make(map[string]string)
	for _, lp := range m.Label {
		actualLabels[lp.GetName()] = lp.GetValue()
	}

	for name, expected := range expectedLabels {
		value, ok := actualLabels[name]
		assert.True(t, ok, "label %q should exist", name)
		assert.Equal(t, expected, value, "label %q value mismatch", name)
	}
	for name := range actualLabels {
		_, ok := expectedLabels[name]
		assert.True(t, ok, "unexpected label %q", name)
	}
}

func TestCollectInvalidSysroot(t *testing.T) {

	ci := NewCpuInfoCollector("/invalid")
	ci.commandRunner = newMockRunner()
	ch := make(chan prom.Metric, 1)

	ci.Collect(ch)
	close(ch)

	assert.Empty(t, ch, "should yield no metrics for invalid sysroot")
}

func TestCollectCaching(t *testing.T) {

	ci := NewCpuInfoCollector("/host")
	ci.commandRunner = newMockRunner()

	ch1 := make(chan prom.Metric, 1)
	ci.Collect(ch1)
	close(ch1)

	ch2 := make(chan prom.Metric, 1)
	ci.Collect(ch2)
	close(ch2)

	mockRunner, ok := ci.commandRunner.(*mockCommandRunner)
	assert.True(t, ok, "commandRunner should be mockCommandRunner")
	assert.Equal(t, 1, mockRunner.callCount, "lscpu should be called once")

	metric1 := <-ch1
	metric2 := <-ch2
	assert.NotNil(t, metric1, "first collect should yield a metric")
	assert.NotNil(t, metric2, "second collect should yield a metric")

	var m1, m2 dto.Metric
	assert.NoError(t, metric1.Write(&m1), "should write first metric successfully")
	assert.NoError(t, metric2.Write(&m2), "should write second metric successfully")

	assert.Equal(t, len(m1.Label), len(m2.Label), "metrics should have same number of labels")

	for _, l1 := range m1.Label {
		found := false
		for _, l2 := range m2.Label {
			if l1.GetName() == l2.GetName() && l1.GetValue() == l2.GetValue() {
				found = true
				break
			}
		}
		assert.True(t, found, "label %q=%q from first collect should be in second", l1.GetName(), l1.GetValue())
	}
}

func TestCollectConcurrent(t *testing.T) {
	ci := NewCpuInfoCollector("/host")
	ci.commandRunner = newMockRunner()
	var wg sync.WaitGroup
	channels := make([]chan prom.Metric, 10)

	for i := 0; i < 10; i++ {
		channels[i] = make(chan prom.Metric, 1)
		wg.Add(1)
		go func(ch chan prom.Metric) {
			defer wg.Done()
			ci.Collect(ch)
			close(ch)
		}(channels[i])
	}

	wg.Wait()

	mockRunner, ok := ci.commandRunner.(*mockCommandRunner)
	assert.True(t, ok, "commandRunner should be mockCommandRunner")
	assert.Equal(t, 1, mockRunner.callCount, "lscpu should be called once")

	var firstMetric *dto.Metric
	for i, ch := range channels {
		metric := <-ch
		assert.NotNil(t, metric, "channel %d should yield a metric", i)

		var m dto.Metric
		assert.NoError(t, metric.Write(&m), "should write metric from channel %d", i)

		if i == 0 {
			firstMetric = &m
		} else {
			assert.Equal(t, len(firstMetric.Label), len(m.Label), "metric %d should have same number of labels", i)
			for _, l := range m.Label {
				found := false
				for _, lFirst := range firstMetric.Label {
					if l.GetName() == lFirst.GetName() && l.GetValue() == lFirst.GetValue() {
						found = true
						break
					}
				}
				assert.True(t, found, "metric %d has unexpected label %q=%q", i, l.GetName(), l.GetValue())
			}
		}
	}
}

func TestRunLscpuSuccess(t *testing.T) {
	ci := NewCpuInfoCollector("/host")
	ci.commandRunner = newMockRunner()
	kv, err := ci.runLscpu()
	assert.NoError(t, err, "should run lscpu successfully")

	expected := map[string]string{
		"Architecture":       "x86_64",
		"Model name":         "11th Gen Intel(R) Core(TM) i7-11850H @ 2.50GHz",
		"CPU(s)":             "16",
		"Core(s) per socket": "8",
		"Socket(s)":          "1",
		"Vendor ID":          "GenuineIntel",
	}
	for k, v := range expected {
		value, ok := kv[k]
		assert.True(t, ok, "key %q should exist", k)
		assert.Equal(t, v, value, "key %q value mismatch", k)
	}
}

func TestRunLscpuFailure(t *testing.T) {
	ci := NewCpuInfoCollector("/invalid")
	ci.commandRunner = newMockRunner()
	_, err := ci.runLscpu()
	assert.Error(t, err, "should fail for invalid sysroot")
}

func TestRunLscpuMalformedOutput(t *testing.T) {
	mockSysrootOutputs["/host"] = &mockCmd{
		output: `Architecture: x86_64
Invalid line
Model name: Intel CPU
`,
		err: nil,
	}

	ci := NewCpuInfoCollector("/host")
	ci.commandRunner = newMockRunner()
	kv, err := ci.runLscpu()
	assert.NoError(t, err, "should handle malformed output")

	expected := map[string]string{
		"Architecture": "x86_64",
		"Model name":   "Intel CPU",
	}
	for k, v := range expected {
		value, ok := kv[k]
		assert.True(t, ok, "key %q should exist", k)
		assert.Equal(t, v, value, "key %q value mismatch", k)
	}
}

func TestProcessCPUInfo(t *testing.T) {
	ci := NewCpuInfoCollector("/host")
	kv := map[string]string{
		"Architecture":       "x86_64",
		"Model name":         "Intel CPU",
		"CPU(s)":             "4",
		"Core(s) per socket": "2",
	}

	info := ci.processCPUInfo(kv)

	expected := map[string]string{
		architecture:     "x86_64",
		model_name:       "Intel CPU",
		cpus:             "4",
		cores_per_socket: "2",
		sockets:          "unknown",
		vendor_id:        "unknown",
	}

	for k, v := range expected {
		value, ok := info[k]
		assert.True(t, ok, "key %q should exist", k)
		assert.Equal(t, v, value, "key %q value mismatch", k)
	}
}

func TestGetCPUArch(t *testing.T) {
	ci := NewCpuInfoCollector("/host")
	tests := []struct {
		name   string
		kv     map[string]string
		expect string
	}{
		{"Present", map[string]string{"Architecture": "x86_64"}, "x86_64"},
		{"Missing", map[string]string{}, "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ci.getCPUArch(tt.kv)
			assert.Equal(t, tt.expect, got, "architecture value mismatch")
		})
	}
}

func TestGetCPUModel(t *testing.T) {
	ci := NewCpuInfoCollector("/host")
	tests := []struct {
		name   string
		kv     map[string]string
		expect string
	}{
		{"Present", map[string]string{"Model name": "Intel CPU"}, "Intel CPU"},
		{"Missing", map[string]string{}, "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ci.getCPUModelName(tt.kv)
			assert.Equal(t, tt.expect, got, "model name value mismatch")
		})
	}
}

func TestGetCPUs(t *testing.T) {
	ci := NewCpuInfoCollector("/host")
	tests := []struct {
		name   string
		kv     map[string]string
		expect string
	}{
		{"Present", map[string]string{"CPU(s)": "4"}, "4"},
		{"Missing", map[string]string{}, "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ci.getCPUs(tt.kv)
			assert.Equal(t, tt.expect, got, "cpus value mismatch")
		})
	}
}

func TestGetCoresPerSocket(t *testing.T) {
	ci := NewCpuInfoCollector("/host")
	tests := []struct {
		name   string
		kv     map[string]string
		expect string
	}{
		{"Present", map[string]string{"Core(s) per socket": "2"}, "2"},
		{"Missing", map[string]string{}, "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ci.getCoresPerSocket(tt.kv)
			assert.Equal(t, tt.expect, got, "cores per socket value mismatch")
		})
	}
}

func TestGetSockets(t *testing.T) {
	ci := NewCpuInfoCollector("/host")
	tests := []struct {
		name   string
		kv     map[string]string
		expect string
	}{
		{"Present", map[string]string{"Socket(s)": "1"}, "1"},
		{"Missing", map[string]string{}, "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ci.getSockets(tt.kv)
			assert.Equal(t, tt.expect, got, "sockets value mismatch")
		})
	}
}

func TestGetVendorID(t *testing.T) {
	ci := NewCpuInfoCollector("/host")
	tests := []struct {
		name   string
		kv     map[string]string
		expect string
	}{
		{"Present", map[string]string{"Vendor ID": "GenuineIntel"}, "GenuineIntel"},
		{"Missing", map[string]string{}, "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ci.getVendorID(tt.kv)
			assert.Equal(t, tt.expect, got, "vendor ID value mismatch")
		})
	}
}
