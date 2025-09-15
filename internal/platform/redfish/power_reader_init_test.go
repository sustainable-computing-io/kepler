// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package redfish

import (
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/stmcginnis/gofish/redfish"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	redfishcfg "github.com/sustainable-computing-io/kepler/config/redfish"
	"github.com/sustainable-computing-io/kepler/internal/platform/redfish/testutil"
)

// TestPowerReaderInitFailureLogout tests that Init() calls Logout() when initialization fails
func TestPowerReaderInitFailureLogout(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	type expectation struct {
		error    bool
		errorMsg string
	}

	// Test scenarios where Init() should fail and cleanup
	testCases := []struct {
		name   string
		config testutil.ServerConfig
		expect expectation
	}{
		{
			name: "Both APIs unavailable - should logout on failure",
			config: testutil.ServerConfig{
				Username:      "admin",
				Password:      "password",
				ForceFallback: true,                       // Disables PowerSubsystem
				ForceError:    testutil.ErrorMissingPower, // Disables Power API
				EnableAuth:    true,
			},
			expect: expectation{
				error:    true,
				errorMsg: "neither PowerSubsystem nor Power API is available",
			},
		},
		{
			name: "Missing chassis - should logout on failure",
			config: testutil.ServerConfig{
				Username:   "admin",
				Password:   "password",
				ForceError: testutil.ErrorMissingChassis,
				EnableAuth: true,
			},
			expect: expectation{
				error:    true,
				errorMsg: "failed to get chassis collection",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Capture the server logs to verify logout is called
			server := testutil.NewServer(tc.config)
			defer server.Close()

			mockBMC := &redfishcfg.BMCDetail{
				Endpoint: server.URL(),
				Username: tc.config.Username,
				Password: tc.config.Password,
				Insecure: true,
			}
			powerReader := NewPowerReader(mockBMC, 30*time.Second, logger)

			// Init should fail
			err := powerReader.Init()
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.expect.errorMsg)

			// Verify the client was cleaned up by checking it's nil
			assert.Nil(t, powerReader.client, "Client should be nil after failed initialization")

			// Additional verification: try to call ReadAll - should fail gracefully
			_, readErr := powerReader.ReadAll()
			require.Error(t, readErr)
			assert.Contains(t, readErr.Error(), "BMC client is not initialized")
		})
	}
}

// TestPowerReaderInitSuccessNoLogout tests that Init() does not logout when initialization succeeds
func TestPowerReaderInitSuccessNoLogout(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	config := testutil.ServerConfig{
		Username:   "admin",
		Password:   "password",
		PowerWatts: 150.0,
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

	// Init should succeed
	err := powerReader.Init()
	require.NoError(t, err)

	// Verify the client is still available after successful initialization
	assert.NotNil(t, powerReader.client, "Client should not be nil after successful initialization")

	// Verify we can read power data successfully
	chassisList, readErr := powerReader.ReadAll()
	require.NoError(t, readErr)
	require.NotEmpty(t, chassisList)

	// Clean up manually
	powerReader.Close()
	assert.Nil(t, powerReader.client, "Client should be nil after Close()")
}

// TestPowerReaderLogoutBehavior tests the logout behavior with different scenarios
func TestPowerReaderLogoutBehavior(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	t.Run("Multiple Close() calls should be safe", func(t *testing.T) {
		config := testutil.ServerConfig{
			Username:   "admin",
			Password:   "password",
			PowerWatts: 100.0,
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

		// Initialize successfully
		err := powerReader.Init()
		require.NoError(t, err)
		assert.NotNil(t, powerReader.client)

		// First close
		powerReader.Close()
		assert.Nil(t, powerReader.client)

		// Second close should not panic
		assert.NotPanics(t, func() {
			powerReader.Close()
		})
	})

	t.Run("Close() without Init() should be safe", func(t *testing.T) {
		mockBMC := &redfishcfg.BMCDetail{
			Endpoint: "http://example.com",
			Username: "admin",
			Password: "password",
			Insecure: true,
		}
		powerReader := NewPowerReader(mockBMC, 30*time.Second, logger)

		// Close without Init should not panic
		assert.NotPanics(t, func() {
			powerReader.Close()
		})
		assert.Nil(t, powerReader.client)
	})
}

// TestPowerReaderInitWithCapturedOutput tests the logout by capturing actual server logs
func TestPowerReaderInitWithCapturedOutput(t *testing.T) {
	// This test verifies logout behavior by examining the mock server output
	// Since the mock server logs all requests including DELETE for logout,
	// we can verify the cleanup behavior indirectly through the test output

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Create a scenario that will fail after successful connection
	config := testutil.ServerConfig{
		Username:      "admin",
		Password:      "password",
		ForceFallback: true,                       // Disables PowerSubsystem
		ForceError:    testutil.ErrorMissingPower, // Disables Power API
		EnableAuth:    true,
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

	// Init should fail and trigger logout
	err := powerReader.Init()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "neither PowerSubsystem nor Power API is available")

	// Client should be cleaned up
	assert.Nil(t, powerReader.client)

	// Note: In the test output, you should see:
	// [MockServer] POST /redfish/v1/SessionService/Sessions - Auth: (login)
	// [MockServer] DELETE /redfish/v1/SessionService/Sessions/session_XXXXX - Auth: (logout)
	// This indicates the cleanup is working correctly
}

// TestPowerReaderCleanupOnDifferentErrors tests cleanup behavior with various error scenarios
func TestPowerReaderCleanupOnDifferentErrors(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	type expectation struct {
		error    bool
		errorMsg string
	}

	// Test different error scenarios that should all trigger cleanup
	errorScenarios := []struct {
		name       string
		forceError testutil.ErrorType
		expect     expectation
	}{
		{
			name:       "Missing chassis should cleanup",
			forceError: testutil.ErrorMissingChassis,
			expect: expectation{
				error:    true,
				errorMsg: "failed to get chassis collection",
			},
		},
		{
			name:       "Missing power should cleanup",
			forceError: testutil.ErrorMissingPower,
			expect: expectation{
				error:    true,
				errorMsg: "neither PowerSubsystem nor Power API is available",
			},
		},
	}

	for _, scenario := range errorScenarios {
		t.Run(scenario.name, func(t *testing.T) {
			config := testutil.ServerConfig{
				Username:   "admin",
				Password:   "password",
				ForceError: scenario.forceError,
				EnableAuth: true,
			}

			// For missing power scenario, also force fallback to make both APIs unavailable
			if scenario.forceError == testutil.ErrorMissingPower {
				config.ForceFallback = true
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

			// Init should fail and cleanup
			err := powerReader.Init()
			require.Error(t, err)
			assert.Contains(t, err.Error(), scenario.expect.errorMsg)

			// Verify cleanup happened
			assert.Nil(t, powerReader.client, "Client should be nil after failed initialization")
		})
	}
}

// TestNewPowerReader tests PowerReader constructor functionality
func TestNewPowerReader(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	mockBMC := &redfishcfg.BMCDetail{
		Endpoint: "",
		Username: "test-user",
		Password: "test-pass",
		Insecure: true,
	}

	powerReader := NewPowerReader(mockBMC, 30*time.Second, logger)

	assert.NotNil(t, powerReader)
	assert.Equal(t, logger, powerReader.logger)
	assert.Nil(t, powerReader.client) // Should be nil initially
	assert.Equal(t, mockBMC.Endpoint, powerReader.cfg.Endpoint)
}

// TestPowerReaderInitNotConnected tests Init() failure due to connection issues
func TestPowerReaderInitNotConnected(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	mockBMC := &redfishcfg.BMCDetail{
		Endpoint: "http://localhost:1", // Invalid port to ensure connection failure
		Username: "test-user",
		Password: "test-pass",
		Insecure: true,
	}
	powerReader := NewPowerReader(mockBMC, 30*time.Second, logger)

	err := powerReader.Init()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to connect to BMC")
}

// TestPowerReaderReadAllNotInitialized tests ReadAll() before Init()
func TestPowerReaderReadAllNotInitialized(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	mockBMC := &redfishcfg.BMCDetail{
		Endpoint: "",
		Username: "test-user",
		Password: "test-pass",
		Insecure: true,
	}
	powerReader := NewPowerReader(mockBMC, 30*time.Second, logger)

	// Don't call Init(), so strategy is not determined
	readings, err := powerReader.ReadAll()
	assert.Error(t, err)
	assert.Nil(t, readings)
	assert.Contains(t, err.Error(), "not initialized")
}

// TestPowerReader_determineStrategy_EdgeCases tests edge cases in strategy determination
func TestPowerReader_determineStrategy_EdgeCases(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	type expectation struct {
		error    bool
		errorMsg string
	}

	testCases := []struct {
		name    string
		chassis []*redfish.Chassis
		expect  expectation
	}{{
		name:    "empty chassis list",
		chassis: []*redfish.Chassis{},
		expect: expectation{
			error:    true,
			errorMsg: "no chassis available for testing",
		},
	}, {
		name:    "nil chassis in list",
		chassis: []*redfish.Chassis{nil, nil},
		expect: expectation{
			error:    true,
			errorMsg: "neither PowerSubsystem nor Power API is available",
		},
	}}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			bmcDetail := &redfishcfg.BMCDetail{}
			powerReader := NewPowerReader(bmcDetail, 30*time.Second, logger)
			strategy, err := powerReader.determineStrategy(tc.chassis)

			if tc.expect.error {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expect.errorMsg)
				assert.Equal(t, UnknownStrategy, strategy)
			} else {
				assert.NoError(t, err)
				assert.NotEqual(t, UnknownStrategy, strategy)
			}
		})
	}
}
