// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package redfish

import (
	"context"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sustainable-computing-io/kepler/internal/platform/redfish/mock"
)

func TestNewPowerReader(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Create a dummy client
	config := &BMCDetail{
		Endpoint: "https://192.168.1.100",
		Username: "admin",
		Password: "password",
		Insecure: true,
	}
	client := NewClient(config)

	powerReader := NewPowerReader(client, logger)

	assert.NotNil(t, powerReader)
	assert.Equal(t, client, powerReader.client)
	assert.Equal(t, logger, powerReader.logger)
}

func TestPowerReaderReadPowerSuccess(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	successScenarios := mock.GetSuccessScenarios()

	for _, scenario := range successScenarios {
		t.Run(scenario.Name, func(t *testing.T) {
			server := mock.CreateScenarioServer(scenario)
			defer server.Close()

			// Create client and connect
			config := &BMCDetail{
				Endpoint: server.URL(),
				Username: scenario.Config.Username,
				Password: scenario.Config.Password,
				Insecure: true,
			}
			client := NewClient(config)

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			err := client.Connect(ctx)
			require.NoError(t, err)
			defer client.Disconnect()

			// Create power reader and test
			powerReader := NewPowerReader(client, logger)

			reading, err := powerReader.ReadPower(ctx)
			assert.NoError(t, err)
			require.NotNil(t, reading)

			assert.InDelta(t, scenario.PowerWatts, reading.PowerWatts, 0.001)
			assert.True(t, time.Since(reading.Timestamp) < 1*time.Second)
		})
	}
}

func TestPowerReaderReadPowerNotConnected(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	config := &BMCDetail{
		Endpoint: "https://192.168.1.100",
		Username: "admin",
		Password: "password",
		Insecure: true,
	}
	client := NewClient(config)

	powerReader := NewPowerReader(client, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	reading, err := powerReader.ReadPower(ctx)
	assert.Error(t, err)
	assert.Nil(t, reading)
	assert.Contains(t, err.Error(), "not connected")
}

func TestPowerReaderReadPowerErrors(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	errorScenarios := mock.GetErrorScenarios()

	for _, scenario := range errorScenarios {
		// Skip scenarios that prevent connection
		if scenario.Config.ForceError == mock.ErrorConnection ||
			scenario.Config.ForceError == mock.ErrorAuth ||
			scenario.Config.ForceError == mock.ErrorTimeout {
			continue
		}

		t.Run(scenario.Name, func(t *testing.T) {
			server := mock.CreateScenarioServer(scenario)
			defer server.Close()

			// Create client and connect (should succeed for these error types)
			config := &BMCDetail{
				Endpoint: server.URL(),
				Username: scenario.Config.Username,
				Password: scenario.Config.Password,
				Insecure: true,
			}
			client := NewClient(config)

			// For scenarios that don't prevent connection, we need to connect first
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			err := client.Connect(ctx)
			if err != nil {
				// Some scenarios might prevent connection
				t.Logf("Connection failed as expected for %s: %v", scenario.Name, err)
				return
			}
			defer client.Disconnect()

			// Create power reader and test
			powerReader := NewPowerReader(client, logger)

			reading, err := powerReader.ReadPower(ctx)
			assert.Error(t, err)
			assert.Nil(t, reading)

			// Verify error contains expected information
			switch scenario.Config.ForceError {
			case mock.ErrorMissingChassis:
				assert.Contains(t, err.Error(), "chassis")
			case mock.ErrorMissingPower:
				assert.Contains(t, err.Error(), "power")
			case mock.ErrorInternalServer:
				// This might be caught at HTTP level
				assert.True(t, err != nil)
			}
		})
	}
}

func TestPowerReaderReadPowerVendorVariations(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	vendors := []mock.VendorType{
		mock.VendorDell,
		mock.VendorHPE,
		mock.VendorLenovo,
		mock.VendorGeneric,
	}

	powerVariations := mock.GetPowerReadingVariations()
	testValues := []float64{
		powerVariations.Zero,
		powerVariations.Idle,
		powerVariations.Light,
		powerVariations.Medium,
		powerVariations.Heavy,
		powerVariations.Peak,
	}

	for _, vendor := range vendors {
		for _, powerWatts := range testValues {
			t.Run(string(vendor)+"_"+formatPowerValue(powerWatts), func(t *testing.T) {
				scenario := mock.TestScenario{
					Config: mock.ServerConfig{
						Vendor:     vendor,
						Username:   "admin",
						Password:   "password",
						PowerWatts: powerWatts,
						EnableAuth: true,
					},
				}

				server := mock.CreateScenarioServer(scenario)
				defer server.Close()

				// Create client and connect
				config := &BMCDetail{
					Endpoint: server.URL(),
					Username: scenario.Config.Username,
					Password: scenario.Config.Password,
					Insecure: true,
				}
				client := NewClient(config)

				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()

				err := client.Connect(ctx)
				require.NoError(t, err)
				defer client.Disconnect()

				// Create power reader and test
				powerReader := NewPowerReader(client, logger)

				reading, err := powerReader.ReadPower(ctx)
				assert.NoError(t, err)
				require.NotNil(t, reading)

				// Use InDelta for floating-point comparison due to float32/float64 precision conversion in gofish
				assert.InDelta(t, powerWatts, reading.PowerWatts, 0.001)
				assert.True(t, time.Since(reading.Timestamp) < 1*time.Second)
			})
		}
	}
}

func TestPowerReaderReadPowerWithRetrySuccess(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	scenario := mock.TestScenario{
		Config: mock.ServerConfig{
			Vendor:     mock.VendorGeneric,
			Username:   "admin",
			Password:   "password",
			PowerWatts: 175.5,
			EnableAuth: true,
		},
	}

	server := mock.CreateScenarioServer(scenario)
	defer server.Close()

	// Create client and connect
	config := &BMCDetail{
		Endpoint: server.URL(),
		Username: scenario.Config.Username,
		Password: scenario.Config.Password,
		Insecure: true,
	}
	client := NewClient(config)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := client.Connect(ctx)
	require.NoError(t, err)
	defer client.Disconnect()

	// Create power reader and test retry
	powerReader := NewPowerReader(client, logger)

	reading, err := powerReader.ReadPowerWithRetry(ctx, 3, 100*time.Millisecond)
	assert.NoError(t, err)
	require.NotNil(t, reading)

	assert.InDelta(t, scenario.Config.PowerWatts, reading.PowerWatts, 0.001)
	assert.True(t, time.Since(reading.Timestamp) < 1*time.Second)
}

func TestPowerReaderReadPowerWithRetryFailures(t *testing.T) {
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

	// Create client and connect
	config := &BMCDetail{
		Endpoint: server.URL(),
		Username: scenario.Config.Username,
		Password: scenario.Config.Password,
		Insecure: true,
	}
	client := NewClient(config)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := client.Connect(ctx)
	require.NoError(t, err)
	defer client.Disconnect()

	// Create power reader and test retry
	powerReader := NewPowerReader(client, logger)

	maxAttempts := 3
	start := time.Now()
	reading, err := powerReader.ReadPowerWithRetry(ctx, maxAttempts, 50*time.Millisecond)
	duration := time.Since(start)

	assert.Error(t, err)
	assert.Nil(t, reading)
	assert.Contains(t, err.Error(), "failed to read power after")
	assert.Contains(t, err.Error(), "3 attempts")

	// Should have taken at least 2 retry delays (100ms total)
	assert.True(t, duration >= 100*time.Millisecond)
}

func TestPowerReaderReadPowerWithRetryContextCancellation(t *testing.T) {
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

	// Create client and connect
	config := &BMCDetail{
		Endpoint: server.URL(),
		Username: scenario.Config.Username,
		Password: scenario.Config.Password,
		Insecure: true,
	}
	client := NewClient(config)

	connectCtx, connectCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer connectCancel()

	err := client.Connect(connectCtx)
	require.NoError(t, err)
	defer client.Disconnect()

	// Create power reader with a short timeout context
	powerReader := NewPowerReader(client, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	reading, err := powerReader.ReadPowerWithRetry(ctx, 10, 200*time.Millisecond)
	assert.Error(t, err)
	assert.Nil(t, reading)
	assert.Equal(t, context.DeadlineExceeded, err)
}

func TestPowerReaderReadPowerWithSlowResponse(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	scenario := mock.TestScenario{
		Config: mock.ServerConfig{
			Vendor:               mock.VendorGeneric,
			Username:             "admin",
			Password:             "password",
			PowerWatts:           150.0,
			EnableAuth:           true,
			SimulateSlowResponse: true,
			ResponseDelay:        500 * time.Millisecond,
		},
	}

	server := mock.CreateScenarioServer(scenario)
	defer server.Close()

	// Create client and connect
	config := &BMCDetail{
		Endpoint: server.URL(),
		Username: scenario.Config.Username,
		Password: scenario.Config.Password,
		Insecure: true,
	}
	client := NewClient(config)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := client.Connect(ctx)
	require.NoError(t, err)
	defer client.Disconnect()

	// Create power reader and test
	powerReader := NewPowerReader(client, logger)

	start := time.Now()
	reading, err := powerReader.ReadPower(ctx)
	duration := time.Since(start)

	assert.NoError(t, err)
	require.NotNil(t, reading)
	assert.InDelta(t, scenario.Config.PowerWatts, reading.PowerWatts, 0.001)

	// Should have taken at least the response delay
	assert.True(t, duration >= scenario.Config.ResponseDelay)
}

func TestPowerReaderConcurrentReads(t *testing.T) {
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

	// Create client and connect
	config := &BMCDetail{
		Endpoint: server.URL(),
		Username: scenario.Config.Username,
		Password: scenario.Config.Password,
		Insecure: true,
	}
	client := NewClient(config)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := client.Connect(ctx)
	require.NoError(t, err)
	defer client.Disconnect()

	// Create power reader
	powerReader := NewPowerReader(client, logger)

	// Test concurrent reads
	const numReads = 10
	var wg sync.WaitGroup
	results := make(chan *PowerReading, numReads)
	errors := make(chan error, numReads)

	for i := 0; i < numReads; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			reading, err := powerReader.ReadPower(ctx)
			if err != nil {
				errors <- err
			} else {
				results <- reading
			}
		}()
	}

	wg.Wait()
	close(results)
	close(errors)

	// Check results
	var readings []*PowerReading
	for reading := range results {
		readings = append(readings, reading)
	}

	var readErrors []error
	for err := range errors {
		readErrors = append(readErrors, err)
	}

	// All reads should succeed
	assert.Empty(t, readErrors, "All concurrent reads should succeed")
	assert.Len(t, readings, numReads, "Should get reading from all goroutines")

	// All readings should have the same power value
	for _, reading := range readings {
		assert.InDelta(t, scenario.Config.PowerWatts, reading.PowerWatts, 0.001)
	}
}

func TestPowerReaderDynamicPowerChanges(t *testing.T) {
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

	// Create client and connect
	config := &BMCDetail{
		Endpoint: server.URL(),
		Username: scenario.Config.Username,
		Password: scenario.Config.Password,
		Insecure: true,
	}
	client := NewClient(config)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := client.Connect(ctx)
	require.NoError(t, err)
	defer client.Disconnect()

	// Create power reader
	powerReader := NewPowerReader(client, logger)

	// Test initial reading
	reading1, err := powerReader.ReadPower(ctx)
	assert.NoError(t, err)
	require.NotNil(t, reading1)
	assert.InDelta(t, 100.0, reading1.PowerWatts, 0.001)

	// Change power value on server
	server.SetPowerWatts(250.0)

	// Test second reading
	reading2, err := powerReader.ReadPower(ctx)
	assert.NoError(t, err)
	require.NotNil(t, reading2)
	assert.InDelta(t, 250.0, reading2.PowerWatts, 0.001)

	// Verify timestamps are different
	assert.True(t, reading2.Timestamp.After(reading1.Timestamp))
}

func TestPowerReaderZeroPowerHandling(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	scenario := mock.TestScenario{
		Config: mock.ServerConfig{
			Vendor:     mock.VendorGeneric,
			Username:   "admin",
			Password:   "password",
			PowerWatts: 0.0, // Zero power
			EnableAuth: true,
		},
	}

	server := mock.CreateScenarioServer(scenario)
	defer server.Close()

	// Create client and connect
	config := &BMCDetail{
		Endpoint: server.URL(),
		Username: scenario.Config.Username,
		Password: scenario.Config.Password,
		Insecure: true,
	}
	client := NewClient(config)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := client.Connect(ctx)
	require.NoError(t, err)
	defer client.Disconnect()

	// Create power reader and test
	powerReader := NewPowerReader(client, logger)

	reading, err := powerReader.ReadPower(ctx)
	assert.NoError(t, err)
	require.NotNil(t, reading)

	// Zero power should be valid (idle system)
	assert.InDelta(t, 0.0, reading.PowerWatts, 0.001)
	assert.True(t, time.Since(reading.Timestamp) < 1*time.Second)
}

// Helper function to format power values for test names
func formatPowerValue(watts float64) string {
	if watts == 0 {
		return "Zero"
	}
	return string(rune(int(watts)))
}
