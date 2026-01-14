// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package redfish

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sustainable-computing-io/kepler/config"
	"github.com/sustainable-computing-io/kepler/internal/device"
	"github.com/sustainable-computing-io/kepler/internal/platform/redfish/testutil"
)

const testMonitorStaleness = 30 * time.Second // Test monitor staleness duration

// defaultRedfishConfig returns a default redfish config for testing
func defaultRedfishConfig(configFile string, nodeName string) config.Redfish {
	return config.Redfish{
		NodeName:    nodeName, // Pre-resolved NodeName
		ConfigFile:  configFile,
		HTTPTimeout: 5 * time.Second, // Use 5s HTTP timeout for testing
	}
}

func TestNewService(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Create a test server for valid endpoint URLs
	server := testutil.NewServer(testutil.ServerConfig{
		Username:   "admin",
		Password:   "password",
		PowerWatts: 150.0,
		EnableAuth: true,
	})
	defer server.Close()

	type expectation struct {
		error bool
	}

	tt := []struct {
		name          string
		configContent string
		nodeName      string
		kubeNodeName  string
		expect        expectation
	}{{
		name: "ValidConfiguration",
		configContent: `
nodes:
  test-node: test-bmc
bmcs:
  test-bmc:
    endpoint: "` + server.URL() + `"
    username: "admin"
    password: "password"
    insecure: true
`,
		nodeName:     "test-node",
		kubeNodeName: "",
		expect: expectation{
			error: false,
		},
	}, {
		name: "NodeNotFound",
		configContent: `
nodes:
  other-node: test-bmc
bmcs:
  test-bmc:
    endpoint: "` + server.URL() + `"
    username: "admin"
    password: "password"
    insecure: true
`,
		nodeName:     "missing-node",
		kubeNodeName: "",
		expect: expectation{
			error: true,
		},
	}, {
		name: "InvalidConfigFile",
		configContent: `
invalid: yaml: content
`,
		nodeName:     "test-node",
		kubeNodeName: "",
		expect: expectation{
			error: true,
		},
	}, {
		name: "HostnameFallback",
		configContent: func() string {
			hostname, _ := os.Hostname()
			return `
nodes:
  ` + hostname + `: test-bmc
bmcs:
  test-bmc:
    endpoint: "` + server.URL() + `"
    username: "admin"
    password: "password"
    insecure: true
`
		}(),
		nodeName:     "",
		kubeNodeName: "",
		expect: expectation{
			error: false, // Should succeed with hostname fallback
		},
	}}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// Create temporary config file
			tmpDir, err := os.MkdirTemp("", "service_test")
			require.NoError(t, err)
			defer func() { _ = os.RemoveAll(tmpDir) }()

			configFile := filepath.Join(tmpDir, "config.yaml")
			err = os.WriteFile(configFile, []byte(tc.configContent), 0644)
			require.NoError(t, err)

			// Create service with resolved NodeName
			// In the real implementation, NodeName would be resolved during config processing
			resolvedNodeName := tc.nodeName
			if resolvedNodeName == "" {
				// Simulate hostname fallback for test
				hostname, _ := os.Hostname()
				resolvedNodeName = hostname
			}
			redfishCfg := defaultRedfishConfig(configFile, resolvedNodeName)
			service, err := NewService(redfishCfg, logger, WithStaleness(testMonitorStaleness))

			if tc.expect.error {
				assert.Error(t, err)
				assert.Nil(t, service)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, service)

			// Verify service properties
			assert.Equal(t, "platform.redfish", service.Name())
			// Client is now managed internally by PowerReader
			assert.NotNil(t, service.powerReader)
			// Verify configuration
			assert.Equal(t, testMonitorStaleness, service.staleness)
			assert.Equal(t, 5*time.Second, service.httpTimeout)

			// Verify resolved node name
			if tc.nodeName != "" {
				// For explicit nodeName, should match exactly
				assert.Equal(t, tc.nodeName, service.nodeName)
			} else {
				// For empty nodeName, should fall back to hostname
				hostname, _ := os.Hostname()
				assert.Equal(t, hostname, service.nodeName)
			}
		})
	}
}

func TestNewServiceNonExistentConfig(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	redfishCfg := defaultRedfishConfig("/non/existent/config.yaml", "test-node")
	service, err := NewService(redfishCfg, logger, WithStaleness(testMonitorStaleness))
	assert.Error(t, err)
	assert.Nil(t, service)
	assert.Contains(t, err.Error(), "failed to load BMC configuration")
}

func TestServiceInitSuccess(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	scenario := testutil.TestScenario{
		Config: testutil.ServerConfig{
			Username:   "admin",
			Password:   "password",
			PowerWatts: 150.0,
			EnableAuth: true,
		},
	}

	server := testutil.CreateScenarioServer(scenario)
	defer server.Close()

	// Create service with mock server
	service := createTestService(t, server, logger)

	// Test initialization
	err := service.Init()
	assert.NoError(t, err)
	// Client connection is now managed internally by PowerReader

	// Cleanup
	err = service.Shutdown()
	assert.NoError(t, err)
}

func TestServiceInitConnectionFailure_GracefulDegradation(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	scenario := testutil.TestScenario{
		Config: testutil.ServerConfig{
			Username:   "admin",
			Password:   "password",
			PowerWatts: 150.0,
			EnableAuth: true,
			ForceError: testutil.ErrorAuth,
		},
	}

	server := testutil.CreateScenarioServer(scenario)
	defer server.Close()

	// Create service with failing mock server
	service := createTestService(t, server, logger)

	// Test initialization - should NOT return error (graceful degradation)
	err := service.Init()
	assert.NoError(t, err, "Init should not return error for BMC connection failures (graceful degradation)")

	// Service should be marked as unavailable
	assert.False(t, service.IsAvailable(), "Service should be marked as unavailable after BMC connection failure")

	// Power() should return an error indicating service is unavailable
	_, err = service.Power()
	assert.Error(t, err, "Power() should return error when service is unavailable")
	assert.Contains(t, err.Error(), "redfish service unavailable")

	// Shutdown should still work
	err = service.Shutdown()
	assert.NoError(t, err)
}

func TestServicePowerDataCollection(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	initialPower := 150.0
	scenario := testutil.TestScenario{
		Config: testutil.ServerConfig{
			Username:   "admin",
			Password:   "password",
			PowerWatts: initialPower,
			EnableAuth: true,
		},
	}

	server := testutil.CreateScenarioServer(scenario)
	defer server.Close()

	// Create and initialize service with short staleness for testing
	service := createTestService(t, server, logger)
	service.staleness = 50 * time.Millisecond // Short staleness for testing
	err := service.Init()
	require.NoError(t, err)

	// Test ChassisPower() can collect data on-demand (even before Run())
	readings, err := service.Power()
	require.NoError(t, err)
	require.NotNil(t, readings)
	require.NotEmpty(t, readings.Chassis)
	require.NotEmpty(t, readings.Chassis[0].Readings)

	expectedPower := Power(initialPower) * device.Watt
	assert.Equal(t, expectedPower, readings.Chassis[0].Readings[0].Power)

	// Test ChassisPower() on-demand collection again (should return cached value)
	readings, err = service.Power()
	require.NoError(t, err)
	require.NotNil(t, readings)
	require.NotEmpty(t, readings.Chassis)
	require.NotEmpty(t, readings.Chassis[0].Readings)

	// Check first reading (should be same as the on-demand reading above)
	assert.Equal(t, expectedPower, readings.Chassis[0].Readings[0].Power)

	// Change power and wait for staleness to expire
	newPower := 250.0
	server.SetPowerWatts(newPower)

	// Wait for staleness to expire
	time.Sleep(100 * time.Millisecond)

	// Test on-demand collection again after power change
	readings, err = service.Power()
	require.NoError(t, err)
	require.NotNil(t, readings)
	require.NotEmpty(t, readings.Chassis)
	require.NotEmpty(t, readings.Chassis[0].Readings)

	// Check second reading (should get fresh data from BMC)
	expectedNewPower := Power(newPower) * device.Watt
	assert.Equal(t, expectedNewPower, readings.Chassis[0].Readings[0].Power)

	// Cleanup
	err = service.Shutdown()
	assert.NoError(t, err)
}

func TestServiceCollectionErrors_GracefulDegradation(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	scenario := testutil.TestScenario{
		Config: testutil.ServerConfig{
			Username:   "admin",
			Password:   "password",
			PowerWatts: 150.0,
			EnableAuth: true,
			ForceError: testutil.ErrorMissingChassis,
		},
	}

	server := testutil.CreateScenarioServer(scenario)
	defer server.Close()

	// Create service
	service := createTestService(t, server, logger)

	// Initialize service - should NOT return error (graceful degradation)
	// even when chassis collection fails during PowerReader initialization
	err := service.Init()
	assert.NoError(t, err, "Init should not return error for BMC issues (graceful degradation)")

	// Service should be marked as unavailable
	assert.False(t, service.IsAvailable(), "Service should be marked as unavailable")

	// Power() should return error when service is unavailable
	_, err = service.Power()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "redfish service unavailable")

	// Shutdown should still work
	err = service.Shutdown()
	assert.NoError(t, err)
}

func TestServiceConcurrentAccessPowerSubsystem(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	scenario := testutil.TestScenario{
		Config: testutil.ServerConfig{
			Username:   "admin",
			Password:   "password",
			PowerWatts: 180.0,
			EnableAuth: true,
			// PowerSubsystem API is used by default (ForceFallback: false)
		},
	}

	server := testutil.CreateScenarioServer(scenario)
	defer server.Close()

	// Create and initialize service
	service := createTestService(t, server, logger)
	err := service.Init()
	require.NoError(t, err)

	// Test that we can collect data on-demand
	readings, err := service.Power()
	assert.NoError(t, err)
	assert.NotEmpty(t, readings)

	// Test concurrent reads - PowerSubsystem API returns multiple power supply readings
	const numReaders = 10
	var wg sync.WaitGroup

	for range numReaders {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 100 {
				readings, err := service.Power()
				if err == nil && readings != nil && len(readings.Chassis) > 0 && len(readings.Chassis[0].Readings) > 0 {
					// With standardized mock server: each power supply reports full chassis power (180W)
					expectedPower := 180 * device.Watt // Each power supply reports the full chassis power for consistency
					assert.Equal(t, expectedPower, readings.Chassis[0].Readings[0].Power)
				}
			}
		}()
	}

	// Concurrent data collection
	wg.Add(1)
	go func() {
		defer wg.Done()
		for range 10 {
			_, _ = service.Power()
			time.Sleep(10 * time.Millisecond)
		}
	}()

	wg.Wait()

	// Cleanup
	err = service.Shutdown()
	assert.NoError(t, err)
}

func TestServiceConcurrentAccessPowerAPI(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	scenario := testutil.TestScenario{
		Config: testutil.ServerConfig{
			Username:      "admin",
			Password:      "password",
			PowerWatts:    180.0,
			EnableAuth:    true,
			ForceFallback: true, // Force use of Power API instead of PowerSubsystem
		},
	}

	server := testutil.CreateScenarioServer(scenario)
	defer server.Close()

	// Create and initialize service
	service := createTestService(t, server, logger)
	err := service.Init()
	require.NoError(t, err)

	// Test that we can collect data on-demand
	readings, err := service.Power()
	assert.NoError(t, err)
	assert.NotEmpty(t, readings)

	// Test concurrent reads - Power API returns single total power reading
	const numReaders = 10
	var wg sync.WaitGroup

	for range numReaders {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 100 {
				readings, err := service.Power()
				if err == nil && readings != nil && len(readings.Chassis) > 0 && len(readings.Chassis[0].Readings) > 0 {
					// Power API: returns total chassis power consumption
					expectedPower := 180 * device.Watt // Full power from single PowerControl reading
					assert.Equal(t, expectedPower, readings.Chassis[0].Readings[0].Power)
				}
			}
		}()
	}

	// Concurrent data collection
	wg.Add(1)
	go func() {
		defer wg.Done()
		for range 10 {
			_, _ = service.Power()
			time.Sleep(10 * time.Millisecond)
		}
	}()

	wg.Wait()

	// Cleanup
	err = service.Shutdown()
	assert.NoError(t, err)
}

func TestServiceStalenessCache(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Create a mock server with initial power reading
	config := testutil.ServerConfig{
		Username:   "admin",
		Password:   "secret",
		PowerWatts: 200.0,
	}
	server := testutil.NewServer(config)
	defer server.Close()

	// Create service using helper with short staleness for testing
	service := createTestService(t, server, logger)

	// Override staleness for testing
	service.staleness = 100 * time.Millisecond // Very short staleness for testing

	err := service.Init()
	require.NoError(t, err)
	defer func() {
		err := service.Shutdown()
		require.NoError(t, err)
	}()

	// First call should hit the BMC
	readings1, err := service.Power()
	require.NoError(t, err)
	require.NotEmpty(t, readings1)
	// With standardized mock server, each power supply reports full chassis power (200W)
	assert.Equal(t, 200.0*device.Watt, readings1.Chassis[0].Readings[0].Power)

	// Change power on server
	server.SetPowerWatts(300.0)

	// Immediate second call should return cached data (same power)
	readings2, err := service.Power()
	require.NoError(t, err)
	require.NotEmpty(t, readings2)
	assert.Equal(t, 200.0*device.Watt, readings2.Chassis[0].Readings[0].Power) // Still cached value

	// Wait for staleness to expire
	time.Sleep(150 * time.Millisecond)

	// Third call should hit BMC again and get new power
	readings3, err := service.Power()
	require.NoError(t, err)
	require.NotEmpty(t, readings3)
	// With standardized mock server, each power supply reports full chassis power (300W)
	assert.Equal(t, 300.0*device.Watt, readings3.Chassis[0].Readings[0].Power) // New value from BMC

	// Fourth immediate call should return new cached data
	readings4, err := service.Power()
	require.NoError(t, err)
	require.NotEmpty(t, readings4)
	assert.Equal(t, 300.0*device.Watt, readings4.Chassis[0].Readings[0].Power) // Cached new value
}

func TestServiceShutdownIdempotent(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	scenario := testutil.TestScenario{
		Config: testutil.ServerConfig{
			Username:   "admin",
			Password:   "password",
			PowerWatts: 150.0,
			EnableAuth: true,
		},
	}

	server := testutil.CreateScenarioServer(scenario)
	defer server.Close()

	// Create and initialize service
	service := createTestService(t, server, logger)
	err := service.Init()
	require.NoError(t, err)

	// First shutdown
	err = service.Shutdown()
	assert.NoError(t, err)

	// Second shutdown should be safe
	err = service.Shutdown()
	assert.NoError(t, err)

	// Third shutdown should also be safe
	err = service.Shutdown()
	assert.NoError(t, err)
}

func TestServiceIntegrationBasic(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	scenario := testutil.TestScenario{
		Config: testutil.ServerConfig{
			Username:   "admin",
			Password:   "password",
			PowerWatts: 165.5,
			EnableAuth: true,
		},
	}

	server := testutil.CreateScenarioServer(scenario)
	defer server.Close()

	// Create and test service
	service := createTestService(t, server, logger)

	// Init
	err := service.Init()
	require.NoError(t, err)

	// Test on-demand collection
	readings, err := service.Power()
	assert.NoError(t, err)
	require.NotNil(t, readings)
	require.NotEmpty(t, readings.Chassis)
	require.NotEmpty(t, readings.Chassis[0].Readings)

	// Create expected power value using the same pattern as in PowerReader
	expectedPower := 165.5 * device.Watt
	assert.Equal(t, expectedPower, readings.Chassis[0].Readings[0].Power)

	// Cleanup
	err = service.Shutdown()
	assert.NoError(t, err)
}

func TestServiceInterfaceCompliance(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	scenario := testutil.TestScenario{
		Config: testutil.ServerConfig{
			Username:   "admin",
			Password:   "password",
			PowerWatts: 150.0,
			EnableAuth: true,
		},
	}

	server := testutil.CreateScenarioServer(scenario)
	defer server.Close()

	service := createTestService(t, server, logger)

	// Test Service interface
	assert.Equal(t, "platform.redfish", service.Name())

	// Test Initializer interface
	err := service.Init()
	assert.NoError(t, err)

	// Test Shutdowner interface
	err = service.Shutdown()
	assert.NoError(t, err)
}

func TestServiceInitCredentialValidation(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	testCases := []struct {
		name     string
		username string
		password string
		wantErr  bool
	}{
		{
			name:     "Both username and password provided",
			username: "admin",
			password: "secret",
			wantErr:  false,
		},
		{
			name:     "Both username and password empty",
			username: "",
			password: "",
			wantErr:  false,
		},
		{
			name:     "Username without password",
			username: "admin",
			password: "",
			wantErr:  true,
		},
		{
			name:     "Password without username",
			username: "",
			password: "secret",
			wantErr:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create test server
			server := testutil.NewServer(testutil.ServerConfig{
				Username:   "admin",
				Password:   "secret",
				PowerWatts: 150.0,
				EnableAuth: true,
			})
			defer server.Close()

			// Create temporary config file with specific credentials
			tmpDir, err := os.MkdirTemp("", "credential_test")
			require.NoError(t, err)
			t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

			configContent := fmt.Sprintf(`
nodes:
  test-node: test-bmc
bmcs:
  test-bmc:
    endpoint: "%s"
    username: "%s"
    password: "%s"
    insecure: true
`, server.URL(), tc.username, tc.password)

			configFile := filepath.Join(tmpDir, "config.yaml")
			err = os.WriteFile(configFile, []byte(configContent), 0644)
			require.NoError(t, err)

			// Create service - this should now fail for invalid credentials
			redfishCfg := defaultRedfishConfig(configFile, "test-node")
			service, err := NewService(redfishCfg, logger, WithStaleness(testMonitorStaleness))

			if tc.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "invalid BMC configuration")
				assert.Nil(t, service)
			} else {
				require.NoError(t, err)
				require.NotNil(t, service)

				// Test initialization - may fail due to auth issues if credentials don't match
				err = service.Init()
				if err != nil {
					// For credential mismatch cases, we expect auth errors
					// but not "both username and password must be provided" errors
					assert.NotContains(t, err.Error(), "both username and password must be provided")
				}
			}
		})
	}
}

// Helper function to create a test service with mock server
func createTestService(t *testing.T, server *testutil.Server, logger *slog.Logger) *Service {
	// Create temporary config file
	tmpDir, err := os.MkdirTemp("", "service_test")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	configContent := `
nodes:
  test-node: test-bmc
bmcs:
  test-bmc:
    endpoint: "` + server.URL() + `"
    username: "admin"
    password: "password"
    insecure: true
`

	configFile := filepath.Join(tmpDir, "config.yaml")
	err = os.WriteFile(configFile, []byte(configContent), 0644)
	require.NoError(t, err)

	// Create service
	redfishCfg := defaultRedfishConfig(configFile, "test-node")
	service, err := NewService(redfishCfg, logger, WithStaleness(testMonitorStaleness))
	require.NoError(t, err)

	return service
}

func TestServiceNodeNameAndBMCID(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Create a test server
	server := testutil.NewServer(testutil.ServerConfig{
		Username:   "admin",
		Password:   "password",
		PowerWatts: 150.0,
		EnableAuth: true,
	})
	defer server.Close()

	// Create temporary config file
	tmpDir, err := os.MkdirTemp("", "service_test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	configContent := `
nodes:
  test-worker-1: test-bmc-1
bmcs:
  test-bmc-1:
    endpoint: "` + server.URL() + `"
    username: "admin"
    password: "password"
    insecure: true
`

	configFile := filepath.Join(tmpDir, "config.yaml")
	err = os.WriteFile(configFile, []byte(configContent), 0644)
	require.NoError(t, err)

	// Create service
	redfishCfg := defaultRedfishConfig(configFile, "test-worker-1")
	service, err := NewService(redfishCfg, logger, WithStaleness(testMonitorStaleness))
	require.NoError(t, err)
	require.NotNil(t, service)

	// Test NodeName method
	nodeName := service.NodeName()
	assert.Equal(t, "test-worker-1", nodeName)

	// Test BMCID method
	bmcID := service.BMCID()
	assert.Equal(t, "test-bmc-1", bmcID)
}

func TestServiceIsFresh(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Create a test server
	server := testutil.NewServer(testutil.ServerConfig{
		Username:   "admin",
		Password:   "password",
		PowerWatts: 150.0,
		EnableAuth: true,
	})
	defer server.Close()

	// Create temporary config file
	tmpDir, err := os.MkdirTemp("", "service_test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	configContent := `
nodes:
  test-node: test-bmc
bmcs:
  test-bmc:
    endpoint: "` + server.URL() + `"
    username: "admin"
    password: "password"
    insecure: true
`

	configFile := filepath.Join(tmpDir, "config.yaml")
	err = os.WriteFile(configFile, []byte(configContent), 0644)
	require.NoError(t, err)

	// Create service with short staleness for testing
	redfishCfg := defaultRedfishConfig(configFile, "test-node")
	service, err := NewService(redfishCfg, logger, WithStaleness(100*time.Millisecond)) // Short staleness for testing
	require.NoError(t, err)
	require.NotNil(t, service)

	// Test 1: No cached data - should not be fresh
	assert.False(t, service.isFresh())

	// Test 2: Add cached data with current timestamp - should be fresh
	service.cachedReading = &PowerReading{
		Timestamp: time.Now(),
		Chassis: []Chassis{
			{
				ID: "test",
				Readings: []Reading{
					{SourceID: "PS1", SourceName: "Test Power Supply", SourceType: PowerSupplySource, Power: 100 * device.Watt},
				},
			},
		},
	}
	assert.True(t, service.isFresh())

	// Test 3: Wait for staleness to expire - should not be fresh
	time.Sleep(150 * time.Millisecond) // Wait longer than staleness threshold
	assert.False(t, service.isFresh())

	// Test 4: Cached data with zero timestamp - should not be fresh
	service.cachedReading = &PowerReading{
		Timestamp: time.Time{}, // Zero timestamp
		Chassis: []Chassis{
			{
				ID: "test",
				Readings: []Reading{
					{SourceID: "PS1", SourceName: "Test Power Supply", SourceType: PowerSupplySource, Power: 100 * device.Watt},
				},
			},
		},
	}
	assert.False(t, service.isFresh())

	// Test 5: Nil cached data - should not be fresh
	service.cachedReading = nil
	assert.False(t, service.isFresh())
}

func TestServiceIsAvailable(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	t.Run("Available after successful init", func(t *testing.T) {
		server := testutil.NewServer(testutil.ServerConfig{
			Username:   "admin",
			Password:   "password",
			PowerWatts: 150.0,
			EnableAuth: true,
		})
		defer server.Close()

		service := createTestService(t, server, logger)
		err := service.Init()
		require.NoError(t, err)

		assert.True(t, service.IsAvailable(), "Service should be available after successful init")

		err = service.Shutdown()
		assert.NoError(t, err)
	})

	t.Run("Unavailable after BMC connection failure", func(t *testing.T) {
		server := testutil.NewServer(testutil.ServerConfig{
			Username:   "admin",
			Password:   "password",
			PowerWatts: 150.0,
			EnableAuth: true,
			ForceError: testutil.ErrorAuth,
		})
		defer server.Close()

		service := createTestService(t, server, logger)
		err := service.Init()
		require.NoError(t, err, "Init should succeed with graceful degradation")

		assert.False(t, service.IsAvailable(), "Service should be unavailable after BMC failure")
	})
}

func TestServiceGracefulDegradation(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	t.Run("Kepler continues when BMC is unreachable", func(t *testing.T) {
		// Simulate a BMC that rejects authentication (common scenario)
		server := testutil.NewServer(testutil.ServerConfig{
			Username:   "admin",
			Password:   "password",
			PowerWatts: 150.0,
			EnableAuth: true,
			ForceError: testutil.ErrorAuth,
		})
		defer server.Close()

		service := createTestService(t, server, logger)

		// Init should NOT fail - graceful degradation
		err := service.Init()
		assert.NoError(t, err, "Init should not fail for BMC connection issues")

		// Service should be marked unavailable
		assert.False(t, service.IsAvailable())

		// Power() should return informative error
		_, err = service.Power()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "BMC was unreachable during initialization")

		// Shutdown should work normally
		err = service.Shutdown()
		assert.NoError(t, err)
	})

	t.Run("Service works normally when BMC is reachable", func(t *testing.T) {
		server := testutil.NewServer(testutil.ServerConfig{
			Username:   "admin",
			Password:   "password",
			PowerWatts: 200.0,
			EnableAuth: true,
		})
		defer server.Close()

		service := createTestService(t, server, logger)

		// Init should succeed
		err := service.Init()
		require.NoError(t, err)

		// Service should be available
		assert.True(t, service.IsAvailable())

		// Power() should return data
		readings, err := service.Power()
		require.NoError(t, err)
		require.NotNil(t, readings)
		require.NotEmpty(t, readings.Chassis)

		err = service.Shutdown()
		assert.NoError(t, err)
	})
}

func TestServiceRun(t *testing.T) {
	// Test the Service.Run method which is currently a no-op waiting for context cancellation
	server := testutil.NewServer(testutil.ServerConfig{
		Username:   "admin",
		Password:   "password",
		PowerWatts: 100.0,
		EnableAuth: false,
	})
	defer server.Close()

	// Create a temporary config file for the test
	tmpDir, err := os.MkdirTemp("", "service_run_test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	configContent := `
nodes:
  test-node: test-bmc
bmcs:
  test-bmc:
    endpoint: "` + server.URL() + `"
    username: "admin"
    password: "password"
    insecure: true
`
	configFile := filepath.Join(tmpDir, "config.yaml")
	err = os.WriteFile(configFile, []byte(configContent), 0600)
	require.NoError(t, err)

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	redfishCfg := defaultRedfishConfig(configFile, "test-node")
	service, err := NewService(redfishCfg, logger, WithStaleness(testMonitorStaleness))
	require.NoError(t, err)

	// Initialize the service
	err = service.Init()
	require.NoError(t, err)

	// Test Run method with context cancellation
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	err = service.Run(ctx)
	duration := time.Since(start)

	// Run should complete when context is cancelled
	assert.NoError(t, err)
	assert.True(t, duration >= 100*time.Millisecond, "Run should wait for context cancellation")
	assert.True(t, duration < 200*time.Millisecond, "Run should return promptly after context cancellation")
}
