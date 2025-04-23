// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package collector

import (
	"context"
	"log/slog"
	"os"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
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
	fakeMonitor := monitor.NewPowerMonitor(musT(device.NewFakeCPUMeter(nil)))
	collector := NewPowerCollector(fakeMonitor, newLogger())

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	assert.NoError(t, fakeMonitor.Init(ctx))

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

	packageZone := device.NewMockRaplZone("package", 0, "/sys/class/powercap/intel-rapl/intel-rapl:0", 1000)
	dramZone := device.NewMockRaplZone("dram", 0, "/sys/class/powercap/intel-rapl/intel-rapl:0:1", 1000)

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

	snapshot := &monitor.Snapshot{
		Timestamp: time.Now(),
		Node:      &testNodeData,
	}
	mockMonitor.On("Snapshot").Return(snapshot, nil)

	collector := NewPowerCollector(mockMonitor, newLogger())
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
				assert.NotEmpty(t, metrics, "Metrics should not be empty")

				// Verify the metrics
				foundPackageJoules := false
				foundDramJoules := false
				foundPackageWatts := false
				foundDramWatts := false

				for _, mf := range metrics {
					switch mf.GetName() {
					case "kepler_node_package_joules_total":
						foundPackageJoules = true
						assertMetricValue(t, mf, "package", packageZone.Path(), nodePkgAbs.Joules())
					case "kepler_node_dram_joules_total":
						foundDramJoules = true
						assertMetricValue(t, mf, "dram", dramZone.Path(), nodeDramAbs.Joules())
					case "kepler_node_package_watts":
						foundPackageWatts = true
						assertMetricValue(t, mf, "package", packageZone.Path(), nodePkgPower.Watts())
					case "kepler_node_dram_watts":
						foundDramWatts = true
						assertMetricValue(t, mf, "dram", dramZone.Path(), nodeDramPower.Watts())
					}
				}

				// Ensure all metrics were found
				assert.True(t, foundPackageJoules, "package_joules_total metric not found")
				assert.True(t, foundDramJoules, "dram_joules_total metric not found")
				assert.True(t, foundPackageWatts, "package_watts metric not found")
				assert.True(t, foundDramWatts, "dram_watts metric not found")
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
	mockMonitor.On("ZoneNames").Return([]string{"package"})

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
				Zones: monitor.ZoneUsageMap{
					packageZone: {
						Absolute: 100 * device.Joule,
						Delta:    10 * device.Joule,
						Power:    5 * device.Watt,
					},
				},
			},
		}, nil)

	collector := NewPowerCollector(mockMonitor, newLogger())
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

	fakeMonitor := monitor.NewPowerMonitor(musT(device.NewFakeCPUMeter(nil)))
	collector := NewPowerCollector(fakeMonitor, newLogger())

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	assert.NoError(t, fakeMonitor.Init(ctx))

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
	fakeMonitor := monitor.NewPowerMonitor(musT(device.NewFakeCPUMeter(nil)))
	collector := NewPowerCollector(fakeMonitor, newLogger())

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	assert.NoError(t, fakeMonitor.Init(ctx))

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

// Helper function to assert metric values
func assertMetricValue(t *testing.T, mf *dto.MetricFamily, zoneName, zonePath string, expected float64) {
	for _, m := range mf.Metric {
		for _, label := range m.Label {
			if label.GetName() == "path" && label.GetValue() == zonePath {
				var value float64
				if mf.GetName() == "kepler_node_"+zoneName+"_joules_total" {
					value = m.Counter.GetValue()
				} else if mf.GetName() == "kepler_node_"+zoneName+"_watts" {
					value = m.Gauge.GetValue()
				}
				assert.Equal(t, expected, value, "Unexpected value for %s, path %s", mf.GetName(), zonePath)
				return
			}
		}
	}
	t.Errorf("Metric for zone %s with path %s not found", zoneName, zonePath)
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
