// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package redfish

import (
	"context"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sustainable-computing-io/kepler/internal/platform/redfish/mock"
)

func TestNewClient(t *testing.T) {
	config := &BMCDetail{
		Endpoint: "https://192.168.1.100",
		Username: "admin",
		Password: "password",
		Insecure: true,
	}

	client := NewClient(config)

	assert.NotNil(t, client)
	assert.Equal(t, config, client.config)
	assert.Nil(t, client.client) // Should not be connected yet
	assert.False(t, client.IsConnected())
}

func TestClientConnectSuccess(t *testing.T) {
	scenarios := mock.GetSuccessScenarios()

	for _, scenario := range scenarios {
		t.Run(scenario.Name, func(t *testing.T) {
			server := mock.CreateScenarioServer(scenario)
			defer server.Close()

			config := &BMCDetail{
				Endpoint: server.URL(),
				Username: scenario.Config.Username,
				Password: scenario.Config.Password,
				Insecure: true, // Use insecure for testing
			}

			client := NewClient(config)
			assert.False(t, client.IsConnected())

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			err := client.Connect(ctx)
			assert.NoError(t, err)
			assert.True(t, client.IsConnected())
			assert.NotNil(t, client.GetAPIClient())

			// Test endpoint
			assert.Equal(t, server.URL(), client.Endpoint())

			// Cleanup
			client.Disconnect()
			assert.False(t, client.IsConnected())
			assert.Nil(t, client.client)
		})
	}
}

func TestClientConnectWithTLS(t *testing.T) {
	scenario := mock.TestScenario{
		Name: "TLSConnection",
		Config: mock.ServerConfig{
			Vendor:     mock.VendorGeneric,
			Username:   "admin",
			Password:   "password",
			PowerWatts: 150.0,
			EnableAuth: true,
			EnableTLS:  true,
		},
	}

	server := mock.CreateScenarioServer(scenario)
	defer server.Close()

	config := &BMCDetail{
		Endpoint: server.URL(),
		Username: scenario.Config.Username,
		Password: scenario.Config.Password,
		Insecure: true, // Skip TLS verification for test
	}

	client := NewClient(config)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := client.Connect(ctx)
	assert.NoError(t, err)
	assert.True(t, client.IsConnected())

	// Verify it's using HTTPS
	assert.True(t, strings.HasPrefix(client.Endpoint(), "https://"))

	client.Disconnect()
}

func TestClientConnectWithAuth(t *testing.T) {
	tests := []struct {
		name          string
		username      string
		password      string
		expectSuccess bool
	}{
		{
			name:          "ValidCredentials",
			username:      "admin",
			password:      "password",
			expectSuccess: true,
		},
		{
			name:          "InvalidCredentials",
			username:      "wrong",
			password:      "wrong",
			expectSuccess: false,
		},
		{
			name:          "EmptyCredentials",
			username:      "",
			password:      "",
			expectSuccess: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			serverConfig := mock.ServerConfig{
				Vendor:     mock.VendorGeneric,
				Username:   "admin",
				Password:   "password",
				PowerWatts: 150.0,
				EnableAuth: true,
			}

			// Force authentication error for invalid credentials
			if !tt.expectSuccess {
				serverConfig.ForceError = mock.ErrorAuth
			}

			// For empty credentials test, disable auth enforcement
			if tt.name == "EmptyCredentials" {
				serverConfig.EnableAuth = false
			}

			server := mock.NewServer(serverConfig)
			defer server.Close()

			config := &BMCDetail{
				Endpoint: server.URL(),
				Username: tt.username,
				Password: tt.password,
				Insecure: true,
			}

			client := NewClient(config)

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			err := client.Connect(ctx)

			if tt.expectSuccess {
				assert.NoError(t, err)
				assert.True(t, client.IsConnected())
				client.Disconnect()
			} else {
				assert.Error(t, err)
				assert.False(t, client.IsConnected())
			}
		})
	}
}

func TestClientConnectErrors(t *testing.T) {
	errorScenarios := mock.GetErrorScenarios()

	for _, scenario := range errorScenarios {
		// Skip scenarios that don't test connection errors or are handled during power reading
		if scenario.Config.ForceError == mock.ErrorMissingChassis ||
			scenario.Config.ForceError == mock.ErrorMissingPower ||
			scenario.Config.ForceError == mock.ErrorBadJSON ||
			scenario.Config.SimulateSlowResponse {
			continue
		}

		t.Run(scenario.Name, func(t *testing.T) {
			server := mock.CreateScenarioServer(scenario)
			defer server.Close()

			config := &BMCDetail{
				Endpoint: server.URL(),
				Username: scenario.Config.Username,
				Password: scenario.Config.Password,
				Insecure: true,
			}

			client := NewClient(config)

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			err := client.Connect(ctx)
			assert.Error(t, err)
			assert.False(t, client.IsConnected())
			assert.Nil(t, client.GetAPIClient())
		})
	}
}

func TestClientConnectTimeout(t *testing.T) {
	scenario := mock.TestScenario{
		Config: mock.ServerConfig{
			Vendor:     mock.VendorGeneric,
			Username:   "admin",
			Password:   "password",
			PowerWatts: 150.0,
			EnableAuth: true,
			ForceError: mock.ErrorTimeout,
		},
	}

	server := mock.CreateScenarioServer(scenario)
	defer server.Close()

	config := &BMCDetail{
		Endpoint: server.URL(),
		Username: scenario.Config.Username,
		Password: scenario.Config.Password,
		Insecure: true,
	}

	client := NewClient(config)

	// Use a very short timeout to force timeout error
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := client.Connect(ctx)
	assert.Error(t, err)
	assert.False(t, client.IsConnected())
}

func TestClientConnectInvalidEndpoint(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
	}{
		{
			name:     "InvalidURL",
			endpoint: "not-a-valid-url",
		},
		{
			name:     "NonExistentHost",
			endpoint: "https://192.168.255.255:8443",
		},
		{
			name:     "InvalidPort",
			endpoint: "https://localhost:99999",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &BMCDetail{
				Endpoint: tt.endpoint,
				Username: "admin",
				Password: "password",
				Insecure: true,
			}

			client := NewClient(config)

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			err := client.Connect(ctx)
			assert.Error(t, err)
			assert.False(t, client.IsConnected())
		})
	}
}

func TestClientTLSConfiguration(t *testing.T) {
	tests := []struct {
		name     string
		insecure bool
	}{
		{
			name:     "InsecureTLS",
			insecure: true,
		},
		{
			name:     "SecureTLS",
			insecure: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scenario := mock.TestScenario{
				Config: mock.ServerConfig{
					Vendor:     mock.VendorGeneric,
					Username:   "admin",
					Password:   "password",
					PowerWatts: 150.0,
					EnableAuth: true,
					EnableTLS:  true,
				},
			}

			server := mock.CreateScenarioServer(scenario)
			defer server.Close()

			config := &BMCDetail{
				Endpoint: server.URL(),
				Username: scenario.Config.Username,
				Password: scenario.Config.Password,
				Insecure: tt.insecure,
			}

			client := NewClient(config)

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			err := client.Connect(ctx)

			if tt.insecure {
				// Should succeed with insecure flag
				assert.NoError(t, err)
				assert.True(t, client.IsConnected())
				client.Disconnect()
			} else {
				// May fail with secure TLS due to self-signed certificate
				// This is expected behavior in test environment
				if err != nil {
					assert.Contains(t, err.Error(), "certificate")
				}
			}
		})
	}
}

func TestClientDisconnect(t *testing.T) {
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

	config := &BMCDetail{
		Endpoint: server.URL(),
		Username: scenario.Config.Username,
		Password: scenario.Config.Password,
		Insecure: true,
	}

	client := NewClient(config)

	// Test disconnect when not connected
	client.Disconnect()
	assert.False(t, client.IsConnected())

	// Connect and then disconnect
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := client.Connect(ctx)
	require.NoError(t, err)
	assert.True(t, client.IsConnected())

	client.Disconnect()
	assert.False(t, client.IsConnected())
	assert.Nil(t, client.client)

	// Test multiple disconnects
	client.Disconnect()
	assert.False(t, client.IsConnected())
}

func TestClientGetAPIClientWhenNotConnected(t *testing.T) {
	config := &BMCDetail{
		Endpoint: "https://192.168.1.100",
		Username: "admin",
		Password: "password",
		Insecure: true,
	}

	client := NewClient(config)
	apiClient := client.GetAPIClient()
	assert.Nil(t, apiClient)
}

func TestClientEndpoint(t *testing.T) {
	endpoint := "https://192.168.1.100:8443"
	config := &BMCDetail{
		Endpoint: endpoint,
		Username: "admin",
		Password: "password",
		Insecure: true,
	}

	client := NewClient(config)
	assert.Equal(t, endpoint, client.Endpoint())

	// Endpoint should return the same value whether connected or not
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

	config.Endpoint = server.URL()
	client = NewClient(config)

	// Before connection
	assert.Equal(t, server.URL(), client.Endpoint())

	// After connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := client.Connect(ctx)
	require.NoError(t, err)
	assert.Equal(t, server.URL(), client.Endpoint())

	client.Disconnect()
}

func TestClientConcurrentAccess(t *testing.T) {
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

	config := &BMCDetail{
		Endpoint: server.URL(),
		Username: scenario.Config.Username,
		Password: scenario.Config.Password,
		Insecure: true,
	}

	client := NewClient(config)

	// Connect
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := client.Connect(ctx)
	require.NoError(t, err)

	// Test concurrent access to read-only methods
	done := make(chan bool, 3)

	go func() {
		for i := 0; i < 10; i++ {
			assert.True(t, client.IsConnected())
			assert.NotNil(t, client.GetAPIClient())
			assert.Equal(t, server.URL(), client.Endpoint())
		}
		done <- true
	}()

	go func() {
		for i := 0; i < 10; i++ {
			assert.True(t, client.IsConnected())
			assert.NotNil(t, client.GetAPIClient())
			assert.Equal(t, server.URL(), client.Endpoint())
		}
		done <- true
	}()

	go func() {
		for i := 0; i < 10; i++ {
			assert.True(t, client.IsConnected())
			assert.NotNil(t, client.GetAPIClient())
			assert.Equal(t, server.URL(), client.Endpoint())
		}
		done <- true
	}()

	// Wait for all goroutines to complete
	for i := 0; i < 3; i++ {
		<-done
	}

	client.Disconnect()
}

func TestClientHTTPClientConfiguration(t *testing.T) {
	// This test verifies that the HTTP client is properly configured
	// We can't directly access the HTTP client, but we can test behavior

	tests := []struct {
		name      string
		insecure  bool
		enableTLS bool
	}{
		{
			name:      "HTTPWithInsecure",
			insecure:  true,
			enableTLS: false,
		},
		{
			name:      "HTTPSWithInsecure",
			insecure:  true,
			enableTLS: true,
		},
		{
			name:      "HTTPSSecure",
			insecure:  false,
			enableTLS: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scenario := mock.TestScenario{
				Config: mock.ServerConfig{
					Vendor:     mock.VendorGeneric,
					Username:   "admin",
					Password:   "password",
					PowerWatts: 150.0,
					EnableAuth: true,
					EnableTLS:  tt.enableTLS,
				},
			}

			server := mock.CreateScenarioServer(scenario)
			defer server.Close()

			config := &BMCDetail{
				Endpoint: server.URL(),
				Username: scenario.Config.Username,
				Password: scenario.Config.Password,
				Insecure: tt.insecure,
			}

			client := NewClient(config)

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			err := client.Connect(ctx)

			if tt.enableTLS && !tt.insecure {
				// May fail due to self-signed certificate
				if err != nil {
					t.Logf("Expected potential TLS error for secure connection: %v", err)
				}
			} else {
				assert.NoError(t, err)
				assert.True(t, client.IsConnected())
				client.Disconnect()
			}
		})
	}
}

func TestClientURLParsing(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		valid    bool
	}{
		{
			name:     "ValidHTTPS",
			endpoint: "https://192.168.1.100",
			valid:    true,
		},
		{
			name:     "ValidHTTP",
			endpoint: "http://192.168.1.100",
			valid:    true,
		},
		{
			name:     "WithPort",
			endpoint: "https://192.168.1.100:8443",
			valid:    true,
		},
		{
			name:     "WithPath",
			endpoint: "https://192.168.1.100/redfish/v1",
			valid:    true,
		},
		{
			name:     "FQDN",
			endpoint: "https://bmc.example.com",
			valid:    true,
		},
		{
			name:     "InvalidScheme",
			endpoint: "ftp://192.168.1.100",
			valid:    false,
		},
		{
			name:     "NoScheme",
			endpoint: "192.168.1.100",
			valid:    false,
		},
		{
			name:     "EmptyURL",
			endpoint: "",
			valid:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test URL parsing
			if tt.valid {
				parsedURL, err := url.Parse(tt.endpoint)
				assert.NoError(t, err)
				assert.NotEmpty(t, parsedURL.Host)
			} else {
				parsedURL, err := url.Parse(tt.endpoint)
				if err == nil {
					// URL parsing might succeed, but it should be invalid for our use
					// Check if it's missing host/scheme OR has invalid scheme for Redfish
					isInvalid := parsedURL.Host == "" || parsedURL.Scheme == "" ||
						(parsedURL.Scheme != "http" && parsedURL.Scheme != "https")
					assert.True(t, isInvalid)
				}
			}

			// Test with actual client (this will fail for invalid endpoints)
			config := &BMCDetail{
				Endpoint: tt.endpoint,
				Username: "admin",
				Password: "password",
				Insecure: true,
			}

			client := NewClient(config)
			assert.Equal(t, tt.endpoint, client.Endpoint())

			// Don't test actual connection for invalid URLs
			if !tt.valid {
				return
			}

			// For valid URLs, test with mock server
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

			// Update config to use mock server URL
			config.Endpoint = server.URL()
			client = NewClient(config)

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			err := client.Connect(ctx)
			assert.NoError(t, err)
			assert.True(t, client.IsConnected())

			client.Disconnect()
		})
	}
}
