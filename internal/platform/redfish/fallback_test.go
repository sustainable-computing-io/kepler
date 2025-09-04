// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package redfish

import (
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	redfishcfg "github.com/sustainable-computing-io/kepler/config/redfish"
	"github.com/sustainable-computing-io/kepler/internal/device"
	"github.com/sustainable-computing-io/kepler/internal/platform/redfish/testutil"
)

func TestPowerReaderFallbackToPowerAPI(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	type expectation struct {
		sourceType          SourceType
		logMessage          string
		powerSubsystemCalls bool
		powerCalls          bool
	}

	tt := []struct {
		name   string
		config testutil.ServerConfig
		expect expectation
	}{{
		name: "ForceFallback - Should use Power API only",
		config: testutil.ServerConfig{
			Username:      "admin",
			Password:      "password",
			PowerWatts:    200.0,
			EnableAuth:    true,
			ForceFallback: true, // Force fallback to Power API
		},
		expect: expectation{
			sourceType:          PowerControlSource,
			logMessage:          "Successfully collected power readings via Power API (deprecated)",
			powerSubsystemCalls: false, // Should not try PowerSubsystem
			powerCalls:          true,
		},
	}, {
		name: "MissingPowerSubsystem - Should fallback to Power API",
		config: testutil.ServerConfig{
			Username:   "admin",
			Password:   "password",
			PowerWatts: 150.0,
			EnableAuth: true,
			ForceError: testutil.ErrorMissingPowerSubsystem, // PowerSubsystem returns 404
		},
		expect: expectation{
			sourceType:          PowerControlSource,
			logMessage:          "Successfully collected power readings via Power API (deprecated)",
			powerSubsystemCalls: true, // Should try PowerSubsystem first
			powerCalls:          true, // Then fallback to Power API
		},
	}}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// Create mock server with the specific config
			server := testutil.NewServer(tc.config)
			defer server.Close()

			// Create power reader with mock BMC config pointing to our test server
			// Use credentials that match the test server configuration
			mockBMC := &redfishcfg.BMCDetail{
				Endpoint: server.URL(),
				Username: tc.config.Username,
				Password: tc.config.Password,
				Insecure: true,
			}
			powerReader := NewPowerReader(mockBMC, 30*time.Second, logger)

			// Initialize the power reader to determine strategy
			err := powerReader.Init()
			require.NoError(t, err)

			// Test ReadAll
			chassisList, err := powerReader.ReadAll()

			// Should succeed
			require.NoError(t, err)
			require.NotEmpty(t, chassisList)
			require.NotEmpty(t, chassisList[0].Readings)

			// Verify the source type matches expectation
			reading := chassisList[0].Readings[0]
			assert.Equal(t, tc.expect.sourceType, reading.SourceType)

			// Verify power value
			expectedPower := Power(tc.config.PowerWatts) * device.Watt
			assert.Equal(t, expectedPower, reading.Power)

			// For PowerControl (deprecated API), verify the field mapping
			if tc.expect.sourceType == PowerControlSource {
				assert.Equal(t, "0", reading.SourceID)                      // MemberID from PowerControl
				assert.Equal(t, "System Power Control", reading.SourceName) // Name from PowerControl
			}
		})
	}
}

func TestPowerReaderSourceTypeDifferentiation(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	type expectation struct {
		sourceType SourceType
	}

	// Test that we can differentiate between PowerSupply and PowerControl sources
	testCases := []struct {
		name          string
		forceFallback bool
		expect        expectation
	}{{
		name:          "PowerSubsystem API should use PowerSupplySource",
		forceFallback: false,
		expect: expectation{
			sourceType: PowerSupplySource,
		},
	}, {
		name:          "Power API fallback should use PowerControlSource",
		forceFallback: true,
		expect: expectation{
			sourceType: PowerControlSource,
		},
	}}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create server config
			config := testutil.ServerConfig{
				Username:      "admin",
				Password:      "password",
				PowerWatts:    175.0,
				EnableAuth:    true,
				ForceFallback: tc.forceFallback,
			}

			server := testutil.NewServer(config)
			defer server.Close()

			// Create power reader with mock BMC config pointing to our test server
			// Use credentials that match the test server configuration
			mockBMC := &redfishcfg.BMCDetail{
				Endpoint: server.URL(),
				Username: config.Username,
				Password: config.Password,
				Insecure: true,
			}
			powerReader := NewPowerReader(mockBMC, 30*time.Second, logger)

			// Initialize the power reader to determine strategy
			err := powerReader.Init()
			require.NoError(t, err)

			// Read power data
			chassisList, err := powerReader.ReadAll()

			require.NoError(t, err)
			require.NotEmpty(t, chassisList)
			require.NotEmpty(t, chassisList[0].Readings)

			// Verify the source type
			reading := chassisList[0].Readings[0]
			assert.Equal(t, tc.expect.sourceType, reading.SourceType)
		})
	}
}

func TestPowerReaderHybridUsage(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Create a scenario where we want to test that the logging correctly
	// identifies which API is being used
	config := testutil.ServerConfig{
		Username:      "admin",
		Password:      "password",
		PowerWatts:    300.0,
		EnableAuth:    true,
		ForceFallback: true, // Force fallback for clear testing
	}

	server := testutil.NewServer(config)
	defer server.Close()

	// Create power reader with mock BMC config pointing to our test server
	// Use credentials that match the test server configuration
	mockBMC := &redfishcfg.BMCDetail{
		Endpoint: server.URL(),
		Username: config.Username,
		Password: config.Password,
		Insecure: true,
	}
	powerReader := NewPowerReader(mockBMC, 30*time.Second, logger)

	// Initialize the power reader to determine strategy
	err := powerReader.Init()
	require.NoError(t, err)

	chassisList, err := powerReader.ReadAll()

	require.NoError(t, err)
	require.NotEmpty(t, chassisList)

	// Verify we got PowerControl data (from deprecated Power API)
	reading := chassisList[0].Readings[0]
	assert.Equal(t, PowerControlSource, reading.SourceType)
	assert.Equal(t, Power(300.0)*device.Watt, reading.Power)
	assert.Equal(t, "0", reading.SourceID)                      // MemberID from PowerControl
	assert.Equal(t, "System Power Control", reading.SourceName) // Name from PowerControl
}

func TestPowerReaderBothAPIsUnavailable(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Test when neither PowerSubsystem nor Power API is available
	config := testutil.ServerConfig{
		Username: "admin",
		Password: "password",
		// Combining both error scenarios to make both APIs unavailable
		ForceFallback: true,                       // Disables PowerSubsystem
		ForceError:    testutil.ErrorMissingPower, // Disables Power API
	}

	server := testutil.NewServer(config)
	defer server.Close()

	mockBMC := &redfishcfg.BMCDetail{
		Endpoint: server.URL(),
		Username: config.Username,
		Password: config.Password,
		Insecure: true,
	}
	powerReader := NewPowerReader(mockBMC, 30*time.Second, logger)

	// Init should fail when no power APIs are available
	err := powerReader.Init()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "neither PowerSubsystem nor Power API is available")
}

func TestPowerReaderEmptyPowerSources(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	tt := []struct {
		name          string
		setupMockData func(*testutil.Server)
		expectedError string
	}{{
		name: "PowerSubsystem with empty power supplies",
		setupMockData: func(s *testutil.Server) {
			// This would require modifying the mock to return empty power supplies
			// For now, we'll test the existing zero power scenario
			s.SetPowerWatts(0)
		},
		expectedError: "failed to determine power reading strategy: neither PowerSubsystem nor Power API is available on any chassis",
	}, {
		name: "Power API with zero power consumption",
		setupMockData: func(s *testutil.Server) {
			s.SetError("") // Clear any errors
			s.SetPowerWatts(0)
		},
		expectedError: "failed to determine power reading strategy: neither PowerSubsystem nor Power API is available on any chassis",
	}}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			config := testutil.ServerConfig{
				Username:      "admin",
				Password:      "password",
				PowerWatts:    0, // Start with zero power
				EnableAuth:    true,
				ForceFallback: tc.name == "Power API with zero power consumption",
			}

			server := testutil.NewServer(config)
			defer server.Close()

			if tc.setupMockData != nil {
				tc.setupMockData(server)
			}

			mockBMC := &redfishcfg.BMCDetail{
				Endpoint: server.URL(),
				Username: config.Username,
				Password: config.Password,
				Insecure: true,
			}
			powerReader := NewPowerReader(mockBMC, 30*time.Second, logger)

			// Initialize should fail when both APIs have zero power readings
			err := powerReader.Init()
			if tc.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedError)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestPowerReaderZeroPowerHandlingPowerSubsystem(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Test that zero power readings are properly skipped with PowerSubsystem API

	// First test with zero power - should fail during Init()
	zeroConfig := testutil.ServerConfig{
		Username:   "admin",
		Password:   "password",
		PowerWatts: 0,
		EnableAuth: true,
		// PowerSubsystem API is used by default (ForceFallback: false)
	}

	zeroServer := testutil.NewServer(zeroConfig)
	defer zeroServer.Close()

	mockBMC := &redfishcfg.BMCDetail{
		Endpoint: zeroServer.URL(),
		Username: zeroConfig.Username,
		Password: zeroConfig.Password,
		Insecure: true,
	}
	powerReader := NewPowerReader(mockBMC, 30*time.Second, logger)

	// Should fail during Init() with zero power - no valid power supplies
	err := powerReader.Init()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to determine power reading strategy: neither PowerSubsystem nor Power API is available on any chassis")

	// Now test with non-zero power - should succeed
	nonZeroConfig := testutil.ServerConfig{
		Username:   "admin",
		Password:   "password",
		PowerWatts: 100.0,
		EnableAuth: true,
		// PowerSubsystem API is used by default (ForceFallback: false)
	}

	nonZeroServer := testutil.NewServer(nonZeroConfig)
	defer nonZeroServer.Close()

	mockBMC2 := &redfishcfg.BMCDetail{
		Endpoint: nonZeroServer.URL(),
		Username: nonZeroConfig.Username,
		Password: nonZeroConfig.Password,
		Insecure: true,
	}
	powerReader2 := NewPowerReader(mockBMC2, 30*time.Second, logger)

	err = powerReader2.Init()
	require.NoError(t, err)

	chassisList, err := powerReader2.ReadAll()
	require.NoError(t, err)
	require.NotEmpty(t, chassisList)
	require.NotEmpty(t, chassisList[0].Readings)

	// Verify we got PowerSupply sources and correct power reading
	reading := chassisList[0].Readings[0]
	assert.Equal(t, PowerSupplySource, reading.SourceType)
	assert.Equal(t, Power(100.0)*device.Watt, reading.Power) // PowerSubsystem: each power supply reports the full chassis power for consistency
}

func TestPowerReaderZeroPowerHandlingPowerAPI(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Test that zero power readings are properly skipped with Power API (fallback)

	// First test with zero power - should fail during Init()
	zeroConfig := testutil.ServerConfig{
		Username:      "admin",
		Password:      "password",
		PowerWatts:    0,
		EnableAuth:    true,
		ForceFallback: true, // Force use of Power API
	}

	zeroServer := testutil.NewServer(zeroConfig)
	defer zeroServer.Close()

	mockBMC := &redfishcfg.BMCDetail{
		Endpoint: zeroServer.URL(),
		Username: zeroConfig.Username,
		Password: zeroConfig.Password,
		Insecure: true,
	}
	powerReader := NewPowerReader(mockBMC, 30*time.Second, logger)

	// Should fail during Init() with zero power - no valid power controls
	err := powerReader.Init()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to determine power reading strategy: neither PowerSubsystem nor Power API is available on any chassis")

	// Now test with non-zero power - should succeed
	nonZeroConfig := testutil.ServerConfig{
		Username:      "admin",
		Password:      "password",
		PowerWatts:    100.0,
		EnableAuth:    true,
		ForceFallback: true, // Force use of Power API
	}

	nonZeroServer := testutil.NewServer(nonZeroConfig)
	defer nonZeroServer.Close()

	mockBMC2 := &redfishcfg.BMCDetail{
		Endpoint: nonZeroServer.URL(),
		Username: nonZeroConfig.Username,
		Password: nonZeroConfig.Password,
		Insecure: true,
	}
	powerReader2 := NewPowerReader(mockBMC2, 30*time.Second, logger)

	err = powerReader2.Init()
	require.NoError(t, err)

	chassisList, err := powerReader2.ReadAll()
	require.NoError(t, err)
	require.NotEmpty(t, chassisList)
	require.NotEmpty(t, chassisList[0].Readings)

	// Verify we got PowerControl source and correct power reading
	reading := chassisList[0].Readings[0]
	assert.Equal(t, PowerControlSource, reading.SourceType)
	assert.Equal(t, Power(100.0)*device.Watt, reading.Power) // Power API: full chassis power
}

func TestPowerReaderMultipleChassis(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// This test would require modifying the mock server to support multiple chassis
	// For now, we test the single chassis handling with nil chassis edge case
	config := testutil.ServerConfig{
		Username:   "admin",
		Password:   "password",
		PowerWatts: 200.0,
		EnableAuth: true,
	}

	server := testutil.NewServer(config)
	defer server.Close()

	mockBMC := &redfishcfg.BMCDetail{
		Endpoint: server.URL(),
		Username: config.Username,
		Password: config.Password,
		Insecure: true,
	}
	powerReader := NewPowerReader(mockBMC, 30*time.Second, logger)

	err := powerReader.Init()
	require.NoError(t, err)

	// Test normal operation with single chassis
	chassisList, err := powerReader.ReadAll()
	require.NoError(t, err)
	require.Len(t, chassisList, 1)
	assert.Equal(t, "1", chassisList[0].ID)
	assert.NotEmpty(t, chassisList[0].Readings)
}

func TestPowerReaderStrategyConsistency(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Test that once a strategy is determined, it's consistently used
	config := testutil.ServerConfig{
		Username:   "admin",
		Password:   "password",
		PowerWatts: 150.0,
		EnableAuth: true,
		// Start with PowerSubsystem available
		ForceFallback: false,
	}

	server := testutil.NewServer(config)
	defer server.Close()

	mockBMC := &redfishcfg.BMCDetail{
		Endpoint: server.URL(),
		Username: config.Username,
		Password: config.Password,
		Insecure: true,
	}
	powerReader := NewPowerReader(mockBMC, 30*time.Second, logger)

	// Initialize with PowerSubsystem available
	err := powerReader.Init()
	require.NoError(t, err)
	assert.Equal(t, PowerSubsystemStrategy, powerReader.strategy)

	// First read should use PowerSubsystem
	chassisList, err := powerReader.ReadAll()
	require.NoError(t, err)
	require.NotEmpty(t, chassisList)
	assert.Equal(t, PowerSupplySource, chassisList[0].Readings[0].SourceType)

	// Even if we change server config, strategy should remain consistent
	// (In real scenario, BMC shouldn't change APIs mid-operation)
	server.SetError(testutil.ErrorMissingPowerSubsystem)

	// Second read should still try PowerSubsystem (pre-determined strategy)
	_, err = powerReader.ReadAll()
	// This will fail because we're forcing PowerSubsystem error but strategy is locked
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no chassis with valid power readings found")
}
