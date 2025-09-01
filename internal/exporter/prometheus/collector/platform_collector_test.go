// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package collector

import (
	"errors"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/sustainable-computing-io/kepler/internal/device"
	"github.com/sustainable-computing-io/kepler/internal/platform/redfish"
)

// mockRedfishDataProvider implements RedfishDataProvider for testing
type mockRedfishDataProvider struct {
	nodeName     string
	bmcID        string
	powerReading *redfish.PowerReading
	err          error
	callCount    int
	mu           sync.Mutex
}

func (m *mockRedfishDataProvider) Power() (*redfish.PowerReading, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callCount++

	if m.err != nil {
		return nil, m.err
	}
	return m.powerReading, nil
}

func (m *mockRedfishDataProvider) NodeName() string {
	return m.nodeName
}

func (m *mockRedfishDataProvider) BMCID() string {
	return m.bmcID
}

func (m *mockRedfishDataProvider) getCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.callCount
}

// Helper function to find metric value by labels
func findMetricValue(t *testing.T, metricFamily *dto.MetricFamily, expectedLabels map[string]string) float64 {
	for _, metric := range metricFamily.GetMetric() {
		allLabelsMatch := true
		for expectedName, expectedValue := range expectedLabels {
			found := false
			for _, label := range metric.GetLabel() {
				if label.GetName() == expectedName && label.GetValue() == expectedValue {
					found = true
					break
				}
			}
			if !found {
				allLabelsMatch = false
				break
			}
		}

		if allLabelsMatch {
			if metric.GetGauge() != nil {
				return metric.GetGauge().GetValue()
			} else if metric.GetCounter() != nil {
				return metric.GetCounter().GetValue()
			}
		}
	}
	t.Errorf("Metric with labels %v not found", expectedLabels)
	return 0
}

func TestNewRedfishCollector(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	mockProvider := &mockRedfishDataProvider{
		nodeName: "test-node",
		bmcID:    "test-bmc",
	}

	collector := NewRedfishCollector(mockProvider, logger)

	require.NotNil(t, collector)
	assert.Equal(t, "test-node", collector.nodeName)
	assert.Equal(t, "test-bmc", collector.bmcID)
	assert.NotNil(t, collector.wattsDesc)
	assert.Equal(t, logger, collector.logger)
	assert.Equal(t, mockProvider, collector.redfish)
}

func TestNewRedfishCollector_ValidationPanics(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	t.Run("Nil data provider panics", func(t *testing.T) {
		assert.Panics(t, func() {
			NewRedfishCollector(nil, logger)
		}, "Should panic when RedfishDataProvider is nil")
	})

	t.Run("Nil logger uses default", func(t *testing.T) {
		mockProvider := &mockRedfishDataProvider{
			nodeName: "test-node",
			bmcID:    "test-bmc",
		}

		collector := NewRedfishCollector(mockProvider, nil)
		require.NotNil(t, collector)
		assert.NotNil(t, collector.logger, "Should use default logger when nil is passed")
	})
}

func TestPlatformCollector_Describe(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	mockProvider := &mockRedfishDataProvider{
		nodeName: "test-node",
		bmcID:    "test-bmc",
	}

	collector := NewRedfishCollector(mockProvider, logger)

	// Create a channel to collect descriptors
	ch := make(chan *prometheus.Desc, 10)

	// Test Describe method
	collector.Describe(ch)
	close(ch)

	// Verify we got exactly one descriptor
	descriptors := make([]*prometheus.Desc, 0)
	for desc := range ch {
		descriptors = append(descriptors, desc)
	}

	require.Len(t, descriptors, 1)
	assert.Equal(t, collector.wattsDesc, descriptors[0])

	// Verify descriptor properties
	desc := descriptors[0]
	assert.Contains(t, desc.String(), "kepler_platform_watts")
	assert.Contains(t, desc.String(), "source")
	assert.Contains(t, desc.String(), "node_name")
	assert.Contains(t, desc.String(), "bmc")
	assert.Contains(t, desc.String(), "chassis_id")
}

func TestPlatformCollector_Collect_Success(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	// Create mock power reading with multiple chassis and multiple PowerControl entries
	powerReading := &redfish.PowerReading{
		Timestamp: time.Now(),
		Chassis: []redfish.Chassis{
			{
				ID: "System.Embedded.1",
				Readings: []redfish.Reading{
					{
						ControlID: "PC1",
						Name:      "Server Power Control",
						Power:     450.5 * device.Watt,
					},
					{
						ControlID: "PC2",
						Name:      "CPU Sub-system Power",
						Power:     85.2 * device.Watt,
					},
				},
			},
			{
				ID: "Enclosure.Internal.0-1",
				Readings: []redfish.Reading{
					{
						ControlID: "PC1",
						Name:      "Enclosure Power Control",
						Power:     125.3 * device.Watt,
					},
				},
			},
		},
	}

	mockProvider := &mockRedfishDataProvider{
		nodeName:     "worker-1",
		bmcID:        "bmc-1",
		powerReading: powerReading,
	}

	collector := NewRedfishCollector(mockProvider, logger)

	// Create registry and register collector
	registry := prometheus.NewRegistry()
	registry.MustRegister(collector)

	// Gather metrics
	metrics, err := registry.Gather()
	require.NoError(t, err)

	// Verify we have the platform metric
	require.Len(t, metrics, 1)
	platformMetric := metrics[0]
	assert.Equal(t, "kepler_platform_watts", platformMetric.GetName())
	assert.Equal(t, dto.MetricType_GAUGE, platformMetric.GetType())

	// Verify we have metrics for all PowerControl entries (3 total: 2 from first chassis, 1 from second)
	require.Len(t, platformMetric.GetMetric(), 3)

	// Verify first chassis, first PowerControl metric
	chassis1PC1Value := findMetricValue(t, platformMetric, map[string]string{
		"source":             "redfish",
		"node_name":          "worker-1",
		"bmc_id":             "bmc-1",
		"chassis_id":         "System.Embedded.1",
		"power_control_id":   "PC1",
		"power_control_name": "Server Power Control",
	})
	assert.Equal(t, 450.5, chassis1PC1Value)

	// Verify first chassis, second PowerControl metric
	chassis1PC2Value := findMetricValue(t, platformMetric, map[string]string{
		"source":             "redfish",
		"node_name":          "worker-1",
		"bmc_id":             "bmc-1",
		"chassis_id":         "System.Embedded.1",
		"power_control_id":   "PC2",
		"power_control_name": "CPU Sub-system Power",
	})
	assert.Equal(t, 85.2, chassis1PC2Value)

	// Verify second chassis metric
	chassis2Value := findMetricValue(t, platformMetric, map[string]string{
		"source":             "redfish",
		"node_name":          "worker-1",
		"bmc_id":             "bmc-1",
		"chassis_id":         "Enclosure.Internal.0-1",
		"power_control_id":   "PC1",
		"power_control_name": "Enclosure Power Control",
	})
	assert.Equal(t, 125.3, chassis2Value)
}

func TestPlatformCollector_Collect_Error(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	mockProvider := &mockRedfishDataProvider{
		nodeName: "test-node",
		bmcID:    "test-bmc",
		err:      errors.New("BMC connection failed"),
	}

	collector := NewRedfishCollector(mockProvider, logger)

	// Create registry and register collector
	registry := prometheus.NewRegistry()
	registry.MustRegister(collector)

	// Gather metrics
	metrics, err := registry.Gather()
	require.NoError(t, err)

	// Verify no metrics were emitted on error
	assert.Len(t, metrics, 0)
}

func TestPlatformCollector_Collect_NilReading(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	mockProvider := &mockRedfishDataProvider{
		nodeName:     "test-node",
		bmcID:        "test-bmc",
		powerReading: nil, // nil reading
	}

	collector := NewRedfishCollector(mockProvider, logger)

	// Create registry and register collector
	registry := prometheus.NewRegistry()
	registry.MustRegister(collector)

	// Gather metrics
	metrics, err := registry.Gather()
	require.NoError(t, err)

	// Verify no metrics were emitted with nil reading
	assert.Len(t, metrics, 0)
}

func TestPlatformCollector_Collect_EmptyReadings(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	// Create empty power reading
	powerReading := &redfish.PowerReading{
		Timestamp: time.Now(),
		Chassis:   []redfish.Chassis{}, // empty chassis
	}

	mockProvider := &mockRedfishDataProvider{
		nodeName:     "test-node",
		bmcID:        "test-bmc",
		powerReading: powerReading,
	}

	collector := NewRedfishCollector(mockProvider, logger)

	// Create registry and register collector
	registry := prometheus.NewRegistry()
	registry.MustRegister(collector)

	// Gather metrics
	metrics, err := registry.Gather()
	require.NoError(t, err)

	// Verify no metrics were emitted with empty readings
	assert.Len(t, metrics, 0)
}

func TestPlatformCollector_Collect_SingleChassis(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	// Create power reading with single chassis
	powerReading := &redfish.PowerReading{
		Timestamp: time.Now(),
		Chassis: []redfish.Chassis{
			{
				ID: "System.Embedded.1",
				Readings: []redfish.Reading{
					{
						ControlID: "PC1",
						Name:      "Server Power Control",
						Power:     300.0 * device.Watt,
					},
				},
			},
		},
	}

	mockProvider := &mockRedfishDataProvider{
		nodeName:     "single-node",
		bmcID:        "single-bmc",
		powerReading: powerReading,
	}

	collector := NewRedfishCollector(mockProvider, logger)

	// Create registry and register collector
	registry := prometheus.NewRegistry()
	registry.MustRegister(collector)

	// Gather metrics
	metrics, err := registry.Gather()
	require.NoError(t, err)

	// Verify we got exactly one metric family with one metric
	require.Len(t, metrics, 1)
	platformMetric := metrics[0]
	require.Len(t, platformMetric.GetMetric(), 1)

	// Verify the metric value
	chassisValue := findMetricValue(t, platformMetric, map[string]string{
		"source":     "redfish",
		"node_name":  "single-node",
		"bmc_id":     "single-bmc",
		"chassis_id": "System.Embedded.1",
	})
	assert.Equal(t, 300.0, chassisValue)
}

func TestPlatformCollector_Collect_ParallelCollection(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	// Create power reading
	powerReading := &redfish.PowerReading{
		Timestamp: time.Now(),
		Chassis: []redfish.Chassis{
			{
				ID: "System.Embedded.1",
				Readings: []redfish.Reading{
					{
						ControlID: "PC1",
						Name:      "Server Power Control",
						Power:     200.0 * device.Watt,
					},
				},
			},
		},
	}

	mockProvider := &mockRedfishDataProvider{
		nodeName:     "parallel-node",
		bmcID:        "parallel-bmc",
		powerReading: powerReading,
	}

	collector := NewRedfishCollector(mockProvider, logger)

	// Create registry and register collector
	registry := prometheus.NewRegistry()
	registry.MustRegister(collector)

	// Test parallel collection
	const numGoroutines = 10
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			// Gather metrics
			metrics, err := registry.Gather()
			assert.NoError(t, err)

			if len(metrics) > 0 {
				// Verify metric structure is consistent
				platformMetric := metrics[0]
				assert.Equal(t, "kepler_platform_watts", platformMetric.GetName())
				assert.Len(t, platformMetric.GetMetric(), 1)
			}
		}()
	}

	wg.Wait()

	// Verify the mock provider was called multiple times
	assert.Greater(t, mockProvider.getCallCount(), 1)
}

func TestPlatformCollector_Collect_MetricLabelsValidation(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	testCases := []struct {
		name      string
		nodeName  string
		bmcID     string
		chassisID string
		power     device.Power
	}{
		{
			name:      "Standard Dell System",
			nodeName:  "dell-worker-01",
			bmcID:     "dell-bmc-01",
			chassisID: "System.Embedded.1",
			power:     428.5 * device.Watt,
		},
		{
			name:      "HPE System with Special Characters",
			nodeName:  "hpe-node_with-dashes",
			bmcID:     "hpe-bmc.domain.local",
			chassisID: "Chassis.Internal-0",
			power:     523.1 * device.Watt,
		},
		{
			name:      "Generic System",
			nodeName:  "generic-node",
			bmcID:     "generic-bmc",
			chassisID: "chassis-0",
			power:     350.0 * device.Watt,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			powerReading := &redfish.PowerReading{
				Timestamp: time.Now(),
				Chassis: []redfish.Chassis{
					{
						ID: tc.chassisID,
						Readings: []redfish.Reading{
							{
								ControlID: "PC1",
								Name:      "Server Power Control",
								Power:     tc.power,
							},
						},
					},
				},
			}

			mockProvider := &mockRedfishDataProvider{
				nodeName:     tc.nodeName,
				bmcID:        tc.bmcID,
				powerReading: powerReading,
			}

			collector := NewRedfishCollector(mockProvider, logger)

			// Create registry and register collector
			registry := prometheus.NewRegistry()
			registry.MustRegister(collector)

			// Gather metrics
			metrics, err := registry.Gather()
			require.NoError(t, err)
			require.Len(t, metrics, 1)

			platformMetric := metrics[0]
			require.Len(t, platformMetric.GetMetric(), 1)

			// Verify all labels are present and correct
			metric := platformMetric.GetMetric()[0]
			labels := make(map[string]string)
			for _, label := range metric.GetLabel() {
				labels[label.GetName()] = label.GetValue()
			}

			assert.Equal(t, "redfish", labels["source"])
			assert.Equal(t, tc.nodeName, labels["node_name"])
			assert.Equal(t, tc.bmcID, labels["bmc_id"])
			assert.Equal(t, tc.chassisID, labels["chassis_id"])

			// Verify power value
			assert.Equal(t, tc.power.Watts(), metric.GetGauge().GetValue())
		})
	}
}
