// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package redfish

import (
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
	"github.com/sustainable-computing-io/kepler/internal/platform/redfish/mock"
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

	tt := []struct {
		name          string
		configContent string
		nodeName      string
		kubeNodeName  string
		expectError   bool
	}{{
		name: "ValidConfiguration",
		configContent: `
nodes:
  test-node: test-bmc
bmcs:
  test-bmc:
    endpoint: "https://192.168.1.100"
    username: "admin"
    password: "password"
    insecure: true
`,
		nodeName:     "test-node",
		kubeNodeName: "",
		expectError:  false,
	}, {
		name: "NodeNotFound",
		configContent: `
nodes:
  other-node: test-bmc
bmcs:
  test-bmc:
    endpoint: "https://192.168.1.100"
    username: "admin"
    password: "password"
    insecure: true
`,
		nodeName:     "missing-node",
		kubeNodeName: "",
		expectError:  true,
	}, {
		name: "InvalidConfigFile",
		configContent: `
invalid: yaml: content
`,
		nodeName:     "test-node",
		kubeNodeName: "",
		expectError:  true,
	}, {
		name: "HostnameFallback",
		configContent: func() string {
			hostname, _ := os.Hostname()
			return `
nodes:
  ` + hostname + `: test-bmc
bmcs:
  test-bmc:
    endpoint: "https://192.168.1.100"
    username: "admin"
    password: "password"
    insecure: true
`
		}(),
		nodeName:     "",
		kubeNodeName: "",
		expectError:  false, // Should succeed with hostname fallback
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

			if tc.expectError {
				assert.Error(t, err)
				assert.Nil(t, service)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, service)

			// Verify service properties
			assert.Equal(t, "platform.redfish", service.Name())
			assert.Nil(t, service.client) // Client is created during Init()
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
	scenario := mock.TestScenario{
		Config: mock.ServerConfig{
			Username:   "admin",
			Password:   "password",
			PowerWatts: 150.0,
			EnableAuth: true,
		},
	}

	server := mock.CreateScenarioServer(scenario)
	defer server.Close()

	// Create service with mock server
	service := createTestService(t, server, logger)

	// Test initialization
	err := service.Init()
	assert.NoError(t, err)
	assert.NotNil(t, service.client)

	// Cleanup
	err = service.Shutdown()
	assert.NoError(t, err)
}

func TestServiceInitConnectionFailure(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	scenario := mock.TestScenario{
		Config: mock.ServerConfig{
			Username:   "admin",
			Password:   "password",
			PowerWatts: 150.0,
			EnableAuth: true,
			ForceError: mock.ErrorAuth,
		},
	}

	server := mock.CreateScenarioServer(scenario)
	defer server.Close()

	// Create service with failing mock server
	service := createTestService(t, server, logger)

	// Test initialization failure
	err := service.Init()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to connect to BMC")
	assert.Nil(t, service.client)
}

func TestServicePowerDataCollection(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	initialPower := 150.0
	scenario := mock.TestScenario{
		Config: mock.ServerConfig{
			Username:   "admin",
			Password:   "password",
			PowerWatts: initialPower,
			EnableAuth: true,
		},
	}

	server := mock.CreateScenarioServer(scenario)
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

func TestServiceCollectionErrors(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	scenario := mock.TestScenario{
		Config: mock.ServerConfig{
			Username:   "admin",
			Password:   "password",
			PowerWatts: 150.0,
			EnableAuth: true,
			ForceError: mock.ErrorMissingChassis,
		},
	}

	server := mock.CreateScenarioServer(scenario)
	defer server.Close()

	// Create and initialize service (should succeed)
	service := createTestService(t, server, logger)
	err := service.Init()
	require.NoError(t, err)

	// Try to collect power data (should fail)
	readings, err := service.Power()
	assert.Error(t, err)
	assert.Nil(t, readings)

	// Verify subsequent calls also fail
	readings, err = service.Power()
	assert.Error(t, err)
	assert.Nil(t, readings)

	// Cleanup
	err = service.Shutdown()
	assert.NoError(t, err)
}

func TestServiceConcurrentAccess(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	scenario := mock.TestScenario{
		Config: mock.ServerConfig{
			Username:   "admin",
			Password:   "password",
			PowerWatts: 180.0,
			EnableAuth: true,
		},
	}

	server := mock.CreateScenarioServer(scenario)
	defer server.Close()

	// Create and initialize service
	service := createTestService(t, server, logger)
	err := service.Init()
	require.NoError(t, err)

	// Test that we can collect data on-demand
	readings, err := service.Power()
	assert.NoError(t, err)
	assert.NotEmpty(t, readings)

	// Test concurrent reads using ChassisPower
	const numReaders = 10
	var wg sync.WaitGroup

	for range numReaders {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 100 {
				readings, err := service.Power()
				if err == nil && readings != nil && len(readings.Chassis) > 0 && len(readings.Chassis[0].Readings) > 0 {
					expectedPower := 180 * device.Watt
					assert.Equal(t, expectedPower, readings.Chassis[0].Readings[0].Power)
				}
			}
		}()
	}

	// Concurrent data collection using ChassisPower()
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
	config := mock.ServerConfig{
		Username:   "admin",
		Password:   "secret",
		PowerWatts: 200.0,
	}
	server := mock.NewServer(config)
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
	assert.Equal(t, 300.0*device.Watt, readings3.Chassis[0].Readings[0].Power) // New value from BMC

	// Fourth immediate call should return new cached data
	readings4, err := service.Power()
	require.NoError(t, err)
	require.NotEmpty(t, readings4)
	assert.Equal(t, 300.0*device.Watt, readings4.Chassis[0].Readings[0].Power) // Cached new value
}

func TestServiceShutdownIdempotent(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	scenario := mock.TestScenario{
		Config: mock.ServerConfig{
			Username:   "admin",
			Password:   "password",
			PowerWatts: 150.0,
			EnableAuth: true,
		},
	}

	server := mock.CreateScenarioServer(scenario)
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
	scenario := mock.TestScenario{
		Config: mock.ServerConfig{
			Username:   "admin",
			Password:   "password",
			PowerWatts: 165.5,
			EnableAuth: true,
		},
	}

	server := mock.CreateScenarioServer(scenario)
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
	scenario := mock.TestScenario{
		Config: mock.ServerConfig{
			Username:   "admin",
			Password:   "password",
			PowerWatts: 150.0,
			EnableAuth: true,
		},
	}

	server := mock.CreateScenarioServer(scenario)
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
			// Create temporary config file with specific credentials
			tmpDir, err := os.MkdirTemp("", "credential_test")
			require.NoError(t, err)
			t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

			configContent := fmt.Sprintf(`
nodes:
  test-node: test-bmc
bmcs:
  test-bmc:
    endpoint: "https://192.168.1.100"
    username: "%s"
    password: "%s"
    insecure: true
`, tc.username, tc.password)

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

				// Test initialization - may fail due to connection issues, but not credential validation
				err = service.Init()
				if err != nil {
					assert.NotContains(t, err.Error(), "both username and password must be provided")
				}
			}
		})
	}
}

// Helper function to create a test service with mock server
func createTestService(t *testing.T, server *mock.Server, logger *slog.Logger) *Service {
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

	// Create temporary config file
	tmpDir, err := os.MkdirTemp("", "service_test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	configContent := `
nodes:
  test-worker-1: test-bmc-1
bmcs:
  test-bmc-1:
    endpoint: "https://192.168.1.100"
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

	// Create temporary config file
	tmpDir, err := os.MkdirTemp("", "service_test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	configContent := `
nodes:
  test-node: test-bmc
bmcs:
  test-bmc:
    endpoint: "https://192.168.1.100"
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
					{ControlID: "PC1", Name: "Test Power Control", Power: 100 * device.Watt},
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
					{ControlID: "PC1", Name: "Test Power Control", Power: 100 * device.Watt},
				},
			},
		},
	}
	assert.False(t, service.isFresh())

	// Test 5: Nil cached data - should not be fresh
	service.cachedReading = nil
	assert.False(t, service.isFresh())
}
