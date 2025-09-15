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

func TestPowerReaderWithMockServerIntegration(t *testing.T) {
	type expectation struct {
		power       float64
		error       bool
		expectedAPI string
	}

	// Integration test showing realistic usage of mock server with PowerReader
	testCases := []struct {
		name         string
		serverConfig testutil.ServerConfig
		expect       expectation
	}{
		{
			name: "PowerSubsystem API success",
			serverConfig: testutil.ServerConfig{
				Username:      "admin",
				Password:      "password",
				PowerWatts:    245.0,
				EnableAuth:    false,
				ForceFallback: false, // Use PowerSubsystem API
			},
			expect: expectation{
				power:       245.0,
				error:       false,
				expectedAPI: "PowerSubsystem",
			},
		},
		{
			name: "Power API fallback success",
			serverConfig: testutil.ServerConfig{
				Username:      "admin",
				Password:      "password",
				PowerWatts:    189.5,
				EnableAuth:    false,
				ForceFallback: true, // Force fallback to Power API
			},
			expect: expectation{
				power:       189.5,
				error:       false,
				expectedAPI: "Power",
			},
		},
		{
			name: "PowerSubsystem missing - fallback to Power API",
			serverConfig: testutil.ServerConfig{
				Username:   "admin",
				Password:   "password",
				PowerWatts: 150.0,
				EnableAuth: false,
				ForceError: testutil.ErrorMissingPowerSubsystem,
			},
			expect: expectation{
				power:       150.0,
				error:       false,
				expectedAPI: "Power",
			},
		},
		{
			name: "Authentication failure",
			serverConfig: testutil.ServerConfig{
				Username:   "admin",
				Password:   "password",
				PowerWatts: 100.0,
				EnableAuth: true,
				ForceError: testutil.ErrorAuth,
			},
			expect: expectation{
				power:       0,
				error:       true,
				expectedAPI: "", // No API should be called on auth failure
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server := testutil.NewServer(tc.serverConfig)
			defer server.Close()

			powerReader := createPowerReaderForServer(t, server)

			// Reset API tracking before test
			server.ResetAPITracking()

			// For authentication failure tests, Init() may fail
			err := powerReader.Init()
			if tc.expect.error && tc.name == "Authentication failure" {
				// For auth errors, Init() should fail
				assert.Error(t, err)
				// Verify no API calls were made after auth failure
				assert.Empty(t, server.CalledAPIs(), "No API should be called after auth failure")
				return
			}
			require.NoError(t, err, "PowerReader initialization should succeed for non-auth tests")

			// Test power reading
			reading, err := powerReader.ReadAll()

			if tc.expect.error {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotEmpty(t, reading)
			require.NotEmpty(t, reading[0].Readings)

			// Verify power matches expectation (now both APIs return the same values)
			actualPower := float64(reading[0].Readings[0].Power / device.Watt)
			assert.Equal(t, tc.expect.power, actualPower, "Power reading should match expected value")

			// Verify the correct API endpoint was called
			if tc.expect.expectedAPI != "" {
				assert.Greater(t, server.APICallCount(tc.expect.expectedAPI), 0,
					"Expected API %s should have been called at least once", tc.expect.expectedAPI)

				// Verify that the other API was not called (except for fallback scenarios)
				if tc.expect.expectedAPI == "Power" {
					// For Power API, PowerSubsystem might have been called first (in fallback scenarios)
					// but we should verify at least one Power call was made
					assert.Greater(t, server.APICallCount("Power"), 0, "Power API should have been called")
				} else {
					// For PowerSubsystem API, no Power calls should be made
					assert.Equal(t, 0, server.APICallCount("Power"), "Power API should not be called when PowerSubsystem succeeds")
					assert.Greater(t, server.APICallCount("PowerSubsystem"), 0, "PowerSubsystem API should have been called")
				}
			}
		})
	}
}

func TestDynamicPowerChangesIntegration(t *testing.T) {
	// Test dynamic power changes using the mock server
	server := testutil.NewServer(testutil.ServerConfig{
		Username:   "admin",
		Password:   "password",
		PowerWatts: 100.0,
		EnableAuth: false,
	})
	defer server.Close()

	powerReader := createPowerReaderForServer(t, server)
	err := powerReader.Init()
	require.NoError(t, err)

	// Initial reading
	reading1, err := powerReader.ReadAll()
	require.NoError(t, err)
	// With standardized mock server, expect the full power value
	assert.InDelta(t, 100.0, reading1[0].Readings[0].Power.Watts(), 0.001)

	// Change power and read again
	server.SetPowerWatts(250.0)
	reading2, err := powerReader.ReadAll()
	require.NoError(t, err)
	assert.InDelta(t, 250.0, reading2[0].Readings[0].Power.Watts(), 0.001)

	// Change to very low power (since zero power is filtered out as invalid)
	server.SetPowerWatts(1.0)
	reading3, err := powerReader.ReadAll()
	require.NoError(t, err)
	assert.InDelta(t, 1.0, reading3[0].Readings[0].Power.Watts(), 0.001)
}

// Helper function to create a PowerReader configured for a test server
func createPowerReaderForServer(t *testing.T, server *testutil.Server) *PowerReader {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	bmcDetail := &redfishcfg.BMCDetail{
		Endpoint: server.URL(),
		Username: "admin",
		Password: "password",
		Insecure: true,
	}

	powerReader := NewPowerReader(bmcDetail, 30*time.Second, logger)
	return powerReader
}
