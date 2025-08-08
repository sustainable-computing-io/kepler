// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package redfish

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sustainable-computing-io/kepler/internal/platform/redfish/mock"
)

func TestNewService(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	tests := []struct {
		name          string
		configContent string
		nodeID        string
		kubeNodeName  string
		expectError   bool
	}{
		{
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
			nodeID:       "test-node",
			kubeNodeName: "",
			expectError:  false,
		},
		{
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
			nodeID:       "missing-node",
			kubeNodeName: "",
			expectError:  true,
		},
		{
			name: "InvalidConfigFile",
			configContent: `
invalid: yaml: content
`,
			nodeID:       "test-node",
			kubeNodeName: "",
			expectError:  true,
		},
		{
			name: "HostnameFallback",
			configContent: `
nodes:
  test-hostname: test-bmc
bmcs:
  test-bmc:
    endpoint: "https://192.168.1.100"
    username: "admin"
    password: "password"
    insecure: true
`,
			nodeID:       "",
			kubeNodeName: "",
			expectError:  true, // Should fail because we don't implement hostname fallback in the constructor
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary config file
			tmpDir, err := os.MkdirTemp("", "service_test")
			require.NoError(t, err)
			defer func() { _ = os.RemoveAll(tmpDir) }()

			configFile := filepath.Join(tmpDir, "config.yaml")
			err = os.WriteFile(configFile, []byte(tt.configContent), 0644)
			require.NoError(t, err)

			// Create service
			service, err := NewService(configFile, tt.nodeID, logger)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, service)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, service)

			// Verify service properties
			assert.Equal(t, "platform.redfish", service.Name())
			assert.False(t, service.IsRunning())
			assert.NotNil(t, service.client)
			assert.NotNil(t, service.powerReader)
			assert.NotNil(t, service.stopCh)

			// Verify resolved node ID
			assert.Equal(t, tt.nodeID, service.nodeID)
		})
	}
}

func TestNewServiceNonExistentConfig(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	service, err := NewService("/non/existent/config.yaml", "test-node", logger)
	assert.Error(t, err)
	assert.Nil(t, service)
	assert.Contains(t, err.Error(), "failed to load BMC configuration")
}

func TestServiceInitSuccess(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	scenario := mock.TestScenario{
		Config: mock.ServerConfig{
			Vendor:     mock.VendorGeneric,
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
	assert.True(t, service.client.IsConnected())

	// Cleanup
	err = service.Shutdown()
	assert.NoError(t, err)
}

func TestServiceInitConnectionFailure(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	scenario := mock.TestScenario{
		Config: mock.ServerConfig{
			Vendor:     mock.VendorGeneric,
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
	assert.False(t, service.client.IsConnected())
}

func TestServiceRunAndShutdown(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	scenario := mock.TestScenario{
		Config: mock.ServerConfig{
			Vendor:     mock.VendorGeneric,
			Username:   "admin",
			Password:   "password",
			PowerWatts: 200.0,
			EnableAuth: true,
		},
	}

	server := mock.CreateScenarioServer(scenario)
	defer server.Close()

	// Create and initialize service
	service := createTestService(t, server, logger)
	err := service.Init()
	require.NoError(t, err)

	// Start service in background
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var runErr error
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		runErr = service.Run(ctx)
	}()

	// Wait a bit for service to start
	time.Sleep(100 * time.Millisecond)
	assert.True(t, service.IsRunning())

	// Shutdown service
	err = service.Shutdown()
	assert.NoError(t, err)

	// Wait for Run to complete
	wg.Wait()
	assert.NoError(t, runErr)
	assert.False(t, service.IsRunning())
	assert.False(t, service.client.IsConnected())
}

func TestServiceRunWithContextCancellation(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	scenario := mock.TestScenario{
		Config: mock.ServerConfig{
			Vendor:     mock.VendorGeneric,
			Username:   "admin",
			Password:   "password",
			PowerWatts: 175.0,
			EnableAuth: true,
		},
	}

	server := mock.CreateScenarioServer(scenario)
	defer server.Close()

	// Create and initialize service
	service := createTestService(t, server, logger)
	err := service.Init()
	require.NoError(t, err)

	// Start service with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	err = service.Run(ctx)
	assert.Equal(t, context.DeadlineExceeded, err)

	// Cleanup
	err = service.Shutdown()
	assert.NoError(t, err)
}

func TestServicePowerDataCollection(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	initialPower := 150.0
	scenario := mock.TestScenario{
		Config: mock.ServerConfig{
			Vendor:     mock.VendorGeneric,
			Username:   "admin",
			Password:   "password",
			PowerWatts: initialPower,
			EnableAuth: true,
		},
	}

	server := mock.CreateScenarioServer(scenario)
	defer server.Close()

	// Create and initialize service
	service := createTestService(t, server, logger)
	err := service.Init()
	require.NoError(t, err)

	// Test initial state (no readings yet)
	reading, energy, nodeID := service.GetLatestReading()
	assert.Nil(t, reading)
	assert.Equal(t, 0.0, energy)
	assert.Equal(t, "test-node", nodeID)

	// Collect some power data manually
	ctx := context.Background()
	err = service.collectPowerData(ctx)
	assert.NoError(t, err)

	// Check first reading
	reading, energy, nodeID = service.GetLatestReading()
	require.NotNil(t, reading)
	assert.InDelta(t, initialPower, reading.PowerWatts, 0.001)
	assert.Equal(t, 0.0, energy) // No energy yet (need two readings)
	assert.Equal(t, "test-node", nodeID)

	// Change power and collect again
	newPower := 250.0
	server.SetPowerWatts(newPower)

	// Wait a bit to ensure time difference
	time.Sleep(10 * time.Millisecond)

	err = service.collectPowerData(ctx)
	assert.NoError(t, err)

	// Check second reading
	reading, energy, nodeID = service.GetLatestReading()
	require.NotNil(t, reading)
	assert.InDelta(t, newPower, reading.PowerWatts, 0.001)
	assert.True(t, energy > 0) // Should have calculated some energy
	assert.Equal(t, "test-node", nodeID)

	// Cleanup
	err = service.Shutdown()
	assert.NoError(t, err)
}

func TestServiceEnergyCalculation(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	scenario := mock.TestScenario{
		Config: mock.ServerConfig{
			Vendor:     mock.VendorGeneric,
			Username:   "admin",
			Password:   "password",
			PowerWatts: 100.0,
			EnableAuth: true,
		},
	}

	server := mock.CreateScenarioServer(scenario)
	defer server.Close()

	// Create and initialize service
	service := createTestService(t, server, logger)
	err := service.Init()
	require.NoError(t, err)

	ctx := context.Background()

	// First reading
	err = service.collectPowerData(ctx)
	assert.NoError(t, err)

	reading1, energy1, _ := service.GetLatestReading()
	require.NotNil(t, reading1)
	assert.Equal(t, 0.0, energy1) // No energy for first reading

	// Wait specific time and collect again
	time.Sleep(100 * time.Millisecond)

	// Change to different power value
	server.SetPowerWatts(200.0)
	err = service.collectPowerData(ctx)
	assert.NoError(t, err)

	reading2, energy2, _ := service.GetLatestReading()
	require.NotNil(t, reading2)
	assert.True(t, energy2 > 0) // Should have energy now

	// Energy calculation: avgPower * timeDelta
	// avgPower = (100 + 200) / 2 = 150W
	// timeDelta ≈ 0.1s (100ms)
	// expectedEnergy ≈ 150 * 0.1 = 15J
	assert.True(t, energy2 > 10.0 && energy2 < 25.0, "Energy should be roughly 15J, got %f", energy2)

	// Third reading with same power
	time.Sleep(100 * time.Millisecond)
	err = service.collectPowerData(ctx)
	assert.NoError(t, err)

	reading3, energy3, _ := service.GetLatestReading()
	require.NotNil(t, reading3)
	assert.True(t, energy3 > energy2) // Energy should have increased

	// Cleanup
	err = service.Shutdown()
	assert.NoError(t, err)
}

func TestServiceCollectionErrors(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	scenario := mock.TestScenario{
		Config: mock.ServerConfig{
			Vendor:     mock.VendorGeneric,
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
	ctx := context.Background()
	err = service.collectPowerData(ctx)
	assert.Error(t, err)

	// Verify no data was stored
	reading, energy, _ := service.GetLatestReading()
	assert.Nil(t, reading)
	assert.Equal(t, 0.0, energy)

	// Cleanup
	err = service.Shutdown()
	assert.NoError(t, err)
}

func TestServiceConcurrentAccess(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	scenario := mock.TestScenario{
		Config: mock.ServerConfig{
			Vendor:     mock.VendorGeneric,
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

	// Collect initial data
	ctx := context.Background()
	err = service.collectPowerData(ctx)
	assert.NoError(t, err)

	// Test concurrent reads
	const numReaders = 10
	var wg sync.WaitGroup

	for i := 0; i < numReaders; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				reading, energy, nodeID := service.GetLatestReading()
				if reading != nil {
					assert.InDelta(t, 180.0, reading.PowerWatts, 0.001)
					assert.True(t, energy >= 0.0)
					assert.Equal(t, "test-node", nodeID)
				}
			}
		}()
	}

	// Concurrent data collection
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			_ = service.collectPowerData(ctx)
			time.Sleep(10 * time.Millisecond)
		}
	}()

	wg.Wait()

	// Cleanup
	err = service.Shutdown()
	assert.NoError(t, err)
}

func TestServiceShutdownIdempotent(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	scenario := mock.TestScenario{
		Config: mock.ServerConfig{
			Vendor:     mock.VendorGeneric,
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
	assert.False(t, service.IsRunning())

	// Second shutdown should be safe
	err = service.Shutdown()
	assert.NoError(t, err)
	assert.False(t, service.IsRunning())

	// Third shutdown should also be safe
	err = service.Shutdown()
	assert.NoError(t, err)
	assert.False(t, service.IsRunning())
}

func TestServiceIntegrationWithDifferentVendors(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	vendors := []mock.VendorType{
		mock.VendorDell,
		mock.VendorHPE,
		mock.VendorLenovo,
		mock.VendorGeneric,
	}

	for _, vendor := range vendors {
		t.Run(string(vendor), func(t *testing.T) {
			scenario := mock.TestScenario{
				Config: mock.ServerConfig{
					Vendor:     vendor,
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

			// Collect data
			ctx := context.Background()
			err = service.collectPowerData(ctx)
			assert.NoError(t, err)

			// Verify reading
			reading, _, _ := service.GetLatestReading()
			require.NotNil(t, reading)
			assert.InDelta(t, 165.5, reading.PowerWatts, 0.001)

			// Cleanup
			err = service.Shutdown()
			assert.NoError(t, err)
		})
	}
}

func TestServiceInterfaceCompliance(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	scenario := mock.TestScenario{
		Config: mock.ServerConfig{
			Vendor:     mock.VendorGeneric,
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

	// Test Runner interface (with quick timeout)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err = service.Run(ctx)
	assert.Equal(t, context.DeadlineExceeded, err)

	// Test Shutdowner interface
	err = service.Shutdown()
	assert.NoError(t, err)
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
	service, err := NewService(configFile, "test-node", logger)
	require.NoError(t, err)

	return service
}
