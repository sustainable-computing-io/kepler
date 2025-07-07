// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package collector

import (
	"context"
	"log/slog"
	"os"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/sustainable-computing-io/kepler/config"
	"github.com/sustainable-computing-io/kepler/internal/device"
	"github.com/sustainable-computing-io/kepler/internal/monitor"
)

func musT[T any](t T, err error) T {
	if err != nil {
		panic(err)
	}
	return t
}

// TestPowerCollectorConcurrency tests the thread safety of PowerCollector
// when multiple goroutines call its methods concurrently.
func TestPowerCollectorConcurrency(t *testing.T) {
	tr := monitor.CreateTestResources()
	ri := &monitor.MockResourceInformer{}
	ri.SetExpectations(t, tr)
	ri.On("Refresh").Return(nil)
	fakeMonitor := monitor.NewPowerMonitor(
		musT(device.NewFakeCPUMeter(nil)),
		monitor.WithResourceInformer(ri),
	)
	collector := NewPowerCollector(fakeMonitor, "test-node", newLogger(), config.MetricsLevelAll)

	assert.NoError(t, fakeMonitor.Init())

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		err := fakeMonitor.Run(ctx)
		assert.NoError(t, err)
	}()

	t.Run("Concurrent Describe", func(t *testing.T) {
		numGoroutines := runtime.NumCPU() * 3
		var wg sync.WaitGroup
		wg.Add(numGoroutines)
		for range numGoroutines {
			go callDescribe(collector, &wg)
		}
		wg.Wait()
	})

	t.Run("Concurrent Collect", func(t *testing.T) {
		numGoroutines := runtime.NumCPU() * 3
		var wg sync.WaitGroup
		wg.Add(numGoroutines)
		for range numGoroutines {
			go callCollect(collector, &wg)
		}
	})

	t.Run("Concurrent Describe and Collect", func(t *testing.T) {
		const numGoroutines = 5
		var wgOuter sync.WaitGroup
		wgOuter.Add(2)

		go func() {
			defer wgOuter.Done()

			var wg sync.WaitGroup
			wg.Add(numGoroutines)
			for range numGoroutines {
				go callDescribe(collector, &wg)
			}
			wg.Wait()
		}()

		go func() {
			defer wgOuter.Done()
			var wg sync.WaitGroup
			wg.Add(numGoroutines)
			for range numGoroutines {
				go callCollect(collector, &wg)
			}
			wg.Wait()
		}()

		wgOuter.Wait()
	})
}

// TestPowerCollectorWithRegistry tests that the PowerCollector can be registered
// with a Prometheus registry and that metrics can be gathered concurrently
func TestPowerCollectorWithRegistry(t *testing.T) {
	mockMonitor := NewMockPowerMonitor()

	package0Zone := device.NewMockRaplZone("package", 0, "/sys/class/powercap/intel-rapl/intel-rapl:0", 1000)
	package1Zone := device.NewMockRaplZone("package", 1, "/sys/class/powercap/intel-rapl/intel-rapl:1", 1000)
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
			package0Zone: monitor.NodeUsage{
				EnergyTotal:       nodePkgAbs,
				ActiveEnergyTotal: nodePkgDelta / 2,
				IdleEnergyTotal:   nodePkgDelta / 2,
				Power:             nodePkgPower,
				ActivePower:       nodePkgPower / 2,
				IdlePower:         nodePkgPower / 2,
			},
			dramZone: monitor.NodeUsage{
				EnergyTotal:       nodeDramAbs,
				ActiveEnergyTotal: nodeDramDelta / 2,
				IdleEnergyTotal:   nodeDramDelta / 2,
				Power:             nodeDramPower,
				ActivePower:       nodeDramPower / 2,
				IdlePower:         nodeDramPower / 2,
			},
			package1Zone: monitor.NodeUsage{
				EnergyTotal:       nodePkgAbs,
				ActiveEnergyTotal: nodePkgDelta / 2,
				IdleEnergyTotal:   nodePkgDelta / 2,
				Power:             nodePkgPower,
				ActivePower:       nodePkgPower / 2,
				IdlePower:         nodePkgPower / 2,
			},
		},
	}

	snapshot := &monitor.Snapshot{
		Timestamp: time.Now(),
		Node:      &testNodeData,
	}
	mockMonitor.On("Snapshot").Return(snapshot, nil)

	collector := NewPowerCollector(mockMonitor, "test-node", newLogger(), config.MetricsLevelAll)
	mockMonitor.TriggerUpdate()
	time.Sleep(10 * time.Millisecond)

	// Create a registry and register the collector
	registry := prometheus.NewRegistry()
	err := registry.Register(collector)
	assert.NoError(t, err, "Failed to register collector")

	// Test concurrent gathering of metrics
	t.Run("Concurrent Gather", func(t *testing.T) {
		numGoroutines := runtime.NumCPU()
		var wg sync.WaitGroup
		wg.Add(numGoroutines)

		for range numGoroutines {
			go func() {
				defer wg.Done()
				metrics, err := registry.Gather()
				assert.NoError(t, err, "Gather should not return an error")
				assert.Len(t, metrics, 7, "Expected 7 node metric families") // Updated from 5 to 7 (added separate active/idle metrics)

				for _, mf := range metrics {
					switch mf.GetName() {
					case "kepler_node_cpu_joules_total":
						// Main joules metric - no mode label
						assertMainMetricValue(t, mf, "package", nodePkgAbs.Joules())
						assertMainMetricValue(t, mf, "dram", nodeDramAbs.Joules())

					case "kepler_node_cpu_watts":
						// Main watts metric - no mode label
						assertMainMetricValue(t, mf, "package", nodePkgPower.Watts())
						assertMainMetricValue(t, mf, "dram", nodeDramPower.Watts())

					case "kepler_node_cpu_active_watts":
						// Active watts metric - no mode label
						assertMainMetricValue(t, mf, "package", (nodePkgPower / 2).Watts())
						assertMainMetricValue(t, mf, "dram", (nodeDramPower / 2).Watts())

					case "kepler_node_cpu_idle_watts":
						// Idle watts metric - no mode label
						assertMainMetricValue(t, mf, "package", (nodePkgPower / 2).Watts())
						assertMainMetricValue(t, mf, "dram", (nodeDramPower / 2).Watts())

					case "kepler_node_cpu_active_joules_total":
						// Active joules metric - no mode label
						assertMainMetricValue(t, mf, "package", (nodePkgDelta / 2).Joules())
						assertMainMetricValue(t, mf, "dram", (nodeDramDelta / 2).Joules())

					case "kepler_node_cpu_idle_joules_total":
						// Idle joules metric - no mode label
						assertMainMetricValue(t, mf, "package", (nodePkgDelta / 2).Joules())
						assertMainMetricValue(t, mf, "dram", (nodeDramDelta / 2).Joules())

					case "kepler_node_cpu_usage_ratio":
						// Usage ratio metric
						assert.Len(t, mf.GetMetric(), 1, "Expected single usage ratio metric")
						assert.Equal(t, 0.5, mf.GetMetric()[0].GetGauge().GetValue())
					}
				}
			}()
		}

		wg.Wait()
	})

	// Verify mock expectations
	mockMonitor.AssertExpectations(t)
}

// TestUpdateDuringCollection tests the behavior when the monitor updates zone data
// during collection
func TestUpdateDuringCollection(t *testing.T) {
	mockMonitor := NewMockPowerMonitor()
	collectingCh := make(chan struct{})
	allowCollectCh := make(chan struct{})

	packageZone := device.NewMockRaplZone("package", 0, "/sys/class/powercap/intel-rapl/intel-rapl:0", 1000)

	mockMonitor.On("Snapshot").Run(func(args mock.Arguments) {
		// NOTE: this waits for allow collect to close
		// before returning the snapshot
		t.Log("Snapshot called -> waiting")
		<-allowCollectCh
		t.Log("Snapshot called -> done waiting")
	}).Return(
		&monitor.Snapshot{
			Timestamp: time.Now(),
			Node: &monitor.Node{
				Timestamp:  time.Now(),
				UsageRatio: 0.5,
				Zones: monitor.NodeZoneUsageMap{
					packageZone: monitor.NodeUsage{
						EnergyTotal:       100 * device.Joule,
						ActiveEnergyTotal: 5 * device.Joule,
						IdleEnergyTotal:   5 * device.Joule,

						Power:       5 * device.Watt,
						ActivePower: 2.5 * device.Watt,
						IdlePower:   2.5 * device.Watt,
					},
				},
			},
		}, nil)

	collector := NewPowerCollector(mockMonitor, "test-node", newLogger(), config.MetricsLevelAll)
	mockMonitor.TriggerUpdate() // collector should now start building descriptors
	time.Sleep(10 * time.Millisecond)

	registry := prometheus.NewRegistry()
	err := registry.Register(collector)
	assert.NoError(t, err)

	var wg sync.WaitGroup
	wg.Add(2)

	// routine 1: Start collecting
	go func() {
		defer wg.Done()

		// Start gather operation
		gatherDone := make(chan struct{})
		go func() {
			// Wait for collect to start
			<-collectingCh
			metrics, err := registry.Gather()
			assert.NoError(t, err)
			assert.NotEmpty(t, metrics)
			close(gatherDone)
		}()

		// Wait for gather to complete or timeout
		select {
		case <-gatherDone:
			t.Log("Gather completed normally")

		case <-time.After(3 * time.Second): // test timeout
			// Unblock the collection anyway to prevent test hanging
			t.Error("Gather timed out, unblocking collect")
			close(allowCollectCh)
		}
	}()

	// routine 2: Trigger updates during collection
	go func() {
		defer wg.Done()

		// Wait for collection to start
		<-collectingCh
		mockMonitor.TriggerUpdate()

		// Unblock collection
		close(allowCollectCh)
	}()

	// Wait for test to complete
	close(collectingCh)
	wg.Wait()

	mockMonitor.AssertExpectations(t)
}

// TestConcurrentRegistration tests collector registration under concurrent conditions
func TestConcurrentRegistration(t *testing.T) {
	const numRegistries = 5

	tr := monitor.CreateTestResources()
	ri := &monitor.MockResourceInformer{}
	ri.SetExpectations(t, tr)
	ri.On("Refresh").Return(nil)

	fakeMonitor := monitor.NewPowerMonitor(
		musT(device.NewFakeCPUMeter(nil)),
		monitor.WithResourceInformer(ri),
	)

	collector := NewPowerCollector(fakeMonitor, "test-node", newLogger(), config.MetricsLevelAll)
	assert.NoError(t, fakeMonitor.Init())

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		err := fakeMonitor.Run(ctx)
		assert.NoError(t, err)
	}()

	// Create registries
	registries := make([]*prometheus.Registry, numRegistries)
	for i := range numRegistries {
		registries[i] = prometheus.NewRegistry()
	}

	// Register collector concurrently
	var wg sync.WaitGroup
	wg.Add(numRegistries)
	for i := range numRegistries {
		go func(idx int) {
			defer wg.Done()
			err := registries[idx].Register(collector)
			assert.NoError(t, err, "Registration should not fail")
		}(i)
	}

	// Wait for registrations
	wg.Wait()

	// Gather metrics from all registries concurrently
	wg.Add(numRegistries)
	for i := range numRegistries {
		go func(idx int) {
			defer wg.Done()
			metrics, err := registries[idx].Gather()
			assert.NoError(t, err, "Gather %d should not fail", idx)
			assert.NotEmpty(t, metrics, "Metrics %d should not be empty", idx)
		}(i)
	}

	// Wait for gatherings
	wg.Wait()
}

// TestFastCollectAndDescribe tests extremely rapid consecutive calls
func TestFastCollectAndDescribe(t *testing.T) {
	tr := monitor.CreateTestResources()
	ri := &monitor.MockResourceInformer{}
	ri.SetExpectations(t, tr)
	ri.On("Refresh").Return(nil)

	fakeMonitor := monitor.NewPowerMonitor(
		musT(device.NewFakeCPUMeter(nil)),
		monitor.WithResourceInformer(ri),
	)
	collector := NewPowerCollector(fakeMonitor, "test-node", newLogger(), config.MetricsLevelAll)

	assert.NoError(t, fakeMonitor.Init())

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		err := fakeMonitor.Run(ctx)
		assert.NoError(t, err)
	}()

	// Test rapid Collect calls
	const iterations = 100
	t.Run("Collect", func(t *testing.T) {
		for range iterations {
			ch := make(chan prometheus.Metric, 100)
			collector.Collect(ch)
			close(ch)
			for range ch {
				// drain channel
			}
		}
	})

	// Test rapid Describe calls
	t.Run("Describe", func(t *testing.T) {
		for range iterations {
			ch := make(chan *prometheus.Desc, 100)
			collector.Describe(ch)
			close(ch)
			for range ch {
				// drain channel
			}
		}
	})

	// Test alternating calls
	t.Run("Alternating Calls", func(t *testing.T) {
		for range iterations {
			// Describe
			descCh := make(chan *prometheus.Desc, 100)
			collector.Describe(descCh)
			close(descCh)
			for range descCh {
				// drain channel
			}

			// Collect
			collectCh := make(chan prometheus.Metric, 100)
			collector.Collect(collectCh)
			close(collectCh)
			for range collectCh {
				// drain channel
			}
		}
	})
}

// Helper function to assert main metric values (without mode label)
func assertMainMetricValue(t *testing.T, mf *dto.MetricFamily, zoneName string, expected float64) {
	t.Helper()

	metricName := mf.GetName()
	for _, m := range mf.Metric {
		zoneMatch := false

		// Check for zone label only (no mode label expected)
		for _, label := range m.Label {
			if label.GetName() == "zone" && label.GetValue() == zoneName {
				zoneMatch = true
				break
			}
		}

		if !zoneMatch {
			continue
		}

		var value float64
		if strings.HasSuffix(metricName, "_joules_total") {
			value = m.Counter.GetValue()
		} else if strings.HasSuffix(metricName, "_watts") {
			value = m.Gauge.GetValue()
		}
		assert.Equal(t, expected, value, "Unexpected value for %s zone: %s", metricName, zoneName)
		return
	}

	t.Errorf("Main metric for zone %s not found", zoneName)
}

func callDescribe(c prometheus.Collector, wg *sync.WaitGroup) {
	defer wg.Done()
	ch := make(chan *prometheus.Desc, 100)
	c.Describe(ch)
	close(ch)
	for range ch {
		// drain the channel
	}
}

func callCollect(c prometheus.Collector, wg *sync.WaitGroup) {
	defer wg.Done()
	ch := make(chan prometheus.Metric, 100)
	c.Collect(ch)
	close(ch)
	for range ch {
		// drain the channel
	}
}

func newLogger() *slog.Logger {
	return slog.New(
		slog.NewTextHandler(
			os.Stderr,
			&slog.HandlerOptions{Level: slog.LevelError},
		),
	)
}
