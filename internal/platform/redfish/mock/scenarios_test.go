// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package mock

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateScenarioServer(t *testing.T) {
	scenario := TestScenario{
		Name: "BasicGeneric",
		Config: ServerConfig{
			Username:   "admin",
			Password:   "password",
			PowerWatts: 150.0,
			EnableAuth: true,
		},
	}

	server := CreateScenarioServer(scenario)
	defer server.Close()

	assert.NotNil(t, server)
	assert.NotEmpty(t, server.URL())
	assert.True(t, strings.HasPrefix(server.URL(), "http"))
}

func TestServerServiceRoot(t *testing.T) {
	config := ServerConfig{
		Username: "admin",
		Password: "password",
	}

	server := NewServer(config)
	defer server.Close()

	// Test service root endpoint
	resp, err := http.Get(server.URL() + "/redfish/v1/")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	var serviceRoot map[string]any
	err = json.NewDecoder(resp.Body).Decode(&serviceRoot)
	require.NoError(t, err)

	// Verify required fields
	assert.Equal(t, "/redfish/v1/", serviceRoot["@odata.id"])
	assert.Equal(t, "RootService", serviceRoot["Id"])
	assert.Equal(t, "1.6.1", serviceRoot["RedfishVersion"])
	assert.NotNil(t, serviceRoot["Chassis"])
}

func TestServerChassisCollection(t *testing.T) {
	config := ServerConfig{
		Username: "admin",
		Password: "password",
	}

	server := NewServer(config)
	defer server.Close()

	// Test chassis collection endpoint
	resp, err := http.Get(server.URL() + "/redfish/v1/Chassis")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var collection map[string]any
	err = json.NewDecoder(resp.Body).Decode(&collection)
	require.NoError(t, err)

	assert.Equal(t, "/redfish/v1/Chassis", collection["@odata.id"])
	assert.Equal(t, "Chassis Collection", collection["Name"])

	members, ok := collection["Members"].([]any)
	require.True(t, ok)
	assert.Len(t, members, 1)
}

func TestServerChassis(t *testing.T) {
	config := ServerConfig{
		Username: "admin",
		Password: "password",
	}

	server := NewServer(config)
	defer server.Close()

	// Test individual chassis endpoint
	resp, err := http.Get(server.URL() + "/redfish/v1/Chassis/1")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var chassis map[string]any
	err = json.NewDecoder(resp.Body).Decode(&chassis)
	require.NoError(t, err)

	assert.Equal(t, "/redfish/v1/Chassis/1", chassis["@odata.id"])
	assert.Equal(t, "1", chassis["Id"])
	assert.Equal(t, "Computer System Chassis", chassis["Name"])
	assert.Equal(t, "generic", chassis["Manufacturer"])
	assert.NotNil(t, chassis["Power"])
}

func TestServerPowerEndpoint(t *testing.T) {
	powerWatts := 175.5
	config := ServerConfig{
		Username:   "admin",
		Password:   "password",
		PowerWatts: powerWatts,
	}

	server := NewServer(config)
	defer server.Close()

	// Test power endpoint
	resp, err := http.Get(server.URL() + "/redfish/v1/Chassis/1/Power")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var power map[string]any
	err = json.NewDecoder(resp.Body).Decode(&power)
	require.NoError(t, err)

	assert.Equal(t, "/redfish/v1/Chassis/1/Power", power["@odata.id"])
	assert.Equal(t, "Power", power["Name"])

	// Check power control information
	powerControl, ok := power["PowerControl"].([]any)
	require.True(t, ok)
	require.Len(t, powerControl, 1)

	control := powerControl[0].(map[string]any)
	assert.InDelta(t, powerWatts, control["PowerConsumedWatts"].(float64), 0.001)
}

func TestServerPowerResponse(t *testing.T) {
	powerWatts := 200.0
	config := ServerConfig{
		Username:   "admin",
		Password:   "password",
		PowerWatts: powerWatts,
	}

	server := NewServer(config)
	defer server.Close()

	// Test power endpoint response
	resp, err := http.Get(server.URL() + "/redfish/v1/Chassis/1/Power")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var power map[string]any
	err = json.NewDecoder(resp.Body).Decode(&power)
	require.NoError(t, err)

	// Verify power structure
	powerControl, ok := power["PowerControl"].([]any)
	require.True(t, ok)
	require.Len(t, powerControl, 1)

	control := powerControl[0].(map[string]any)
	assert.InDelta(t, powerWatts, control["PowerConsumedWatts"].(float64), 0.001)
}

func TestServerAuthenticationEnabled(t *testing.T) {
	config := ServerConfig{
		Username:   "admin",
		Password:   "password",
		EnableAuth: true,
	}

	server := NewServer(config)
	defer server.Close()

	// Test session creation
	sessionData := map[string]string{
		"UserName": "admin",
		"Password": "password",
	}
	body, _ := json.Marshal(sessionData)

	resp, err := http.Post(server.URL()+"/redfish/v1/SessionService/Sessions",
		"application/json", strings.NewReader(string(body)))
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	assert.NotEmpty(t, resp.Header.Get("X-Auth-Token"))
	assert.NotEmpty(t, resp.Header.Get("Location"))

	var session map[string]any
	err = json.NewDecoder(resp.Body).Decode(&session)
	require.NoError(t, err)

	assert.Equal(t, "admin", session["UserName"])
	assert.NotEmpty(t, session["Id"])
}

func TestServerAuthenticationDisabled(t *testing.T) {
	config := ServerConfig{
		Username:   "admin",
		Password:   "password",
		EnableAuth: false,
	}

	server := NewServer(config)
	defer server.Close()

	// Test session creation without credentials
	resp, err := http.Post(server.URL()+"/redfish/v1/SessionService/Sessions",
		"application/json", strings.NewReader("{}"))
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	assert.NotEmpty(t, resp.Header.Get("X-Auth-Token"))
}

func TestServerErrorScenarios(t *testing.T) {
	errorTests := []struct {
		name         string
		errorType    ErrorType
		endpoint     string
		expectedCode int
	}{
		{
			name:         "MissingChassis",
			errorType:    ErrorMissingChassis,
			endpoint:     "/redfish/v1/Chassis",
			expectedCode: http.StatusNotFound,
		},
		{
			name:         "MissingPower",
			errorType:    ErrorMissingPower,
			endpoint:     "/redfish/v1/Chassis/1/Power",
			expectedCode: http.StatusNotFound,
		},
		{
			name:         "InternalServerError",
			errorType:    ErrorInternalServer,
			endpoint:     "/redfish/v1/",
			expectedCode: http.StatusInternalServerError,
		},
		{
			name:         "AuthError",
			errorType:    ErrorAuth,
			endpoint:     "/redfish/v1/SessionService/Sessions",
			expectedCode: http.StatusUnauthorized,
		},
	}

	for _, tt := range errorTests {
		t.Run(tt.name, func(t *testing.T) {
			config := ServerConfig{
				Username:   "admin",
				Password:   "password",
				EnableAuth: true,
				ForceError: tt.errorType,
			}

			server := NewServer(config)
			defer server.Close()

			var resp *http.Response
			var err error

			if tt.errorType == ErrorAuth {
				// Test with invalid credentials
				sessionData := map[string]string{
					"UserName": "wrong",
					"Password": "wrong",
				}
				body, _ := json.Marshal(sessionData)
				resp, err = http.Post(server.URL()+tt.endpoint,
					"application/json", strings.NewReader(string(body)))
			} else {
				resp, err = http.Get(server.URL() + tt.endpoint)
			}

			require.NoError(t, err)
			defer func() { _ = resp.Body.Close() }()

			assert.Equal(t, tt.expectedCode, resp.StatusCode)
		})
	}
}

func TestServerSlowResponse(t *testing.T) {
	responseDelay := 200 * time.Millisecond
	config := ServerConfig{
		Username:             "admin",
		Password:             "password",
		PowerWatts:           150.0,
		SimulateSlowResponse: true,
		ResponseDelay:        responseDelay,
	}

	server := NewServer(config)
	defer server.Close()

	start := time.Now()
	resp, err := http.Get(server.URL() + "/redfish/v1/")
	duration := time.Since(start)

	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.True(t, duration >= responseDelay,
		"Response should take at least %v, took %v", responseDelay, duration)
}

func TestServerTimeoutHandling(t *testing.T) {
	config := ServerConfig{
		Username:   "admin",
		Password:   "password",
		ForceError: ErrorTimeout,
	}

	server := NewServer(config)
	defer server.Close()

	// Create request with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", server.URL()+"/redfish/v1/", nil)
	require.NoError(t, err)

	client := &http.Client{}
	_, err = client.Do(req)

	// Should get context deadline exceeded or connection reset
	assert.Error(t, err)
}

func TestServerDynamicPowerChanges(t *testing.T) {
	config := ServerConfig{
		Username:   "admin",
		Password:   "password",
		PowerWatts: 100.0,
	}

	server := NewServer(config)
	defer server.Close()

	// Test initial power reading
	resp, err := http.Get(server.URL() + "/redfish/v1/Chassis/1/Power")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	var power1 map[string]any
	err = json.NewDecoder(resp.Body).Decode(&power1)
	require.NoError(t, err)

	powerControl1 := power1["PowerControl"].([]any)[0].(map[string]any)
	assert.InDelta(t, 100.0, powerControl1["PowerConsumedWatts"].(float64), 0.001)

	// Change power dynamically
	server.SetPowerWatts(250.0)

	// Test updated power reading
	resp2, err := http.Get(server.URL() + "/redfish/v1/Chassis/1/Power")
	require.NoError(t, err)
	defer func() { _ = resp2.Body.Close() }()

	var power2 map[string]any
	err = json.NewDecoder(resp2.Body).Decode(&power2)
	require.NoError(t, err)

	powerControl2 := power2["PowerControl"].([]any)[0].(map[string]any)
	assert.InDelta(t, 250.0, powerControl2["PowerConsumedWatts"].(float64), 0.001)
}

func TestServerConcurrentRequests(t *testing.T) {
	config := ServerConfig{
		Username:   "admin",
		Password:   "password",
		PowerWatts: 150.0,
	}

	server := NewServer(config)
	defer server.Close()

	const numRequests = 10
	results := make(chan error, numRequests)

	// Make concurrent requests
	for i := 0; i < numRequests; i++ {
		go func() {
			resp, err := http.Get(server.URL() + "/redfish/v1/")
			if err != nil {
				results <- err
				return
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				results <- fmt.Errorf("unexpected status code: %d", resp.StatusCode)
				return
			}

			results <- nil
		}()
	}

	// Check all results
	for i := 0; i < numRequests; i++ {
		err := <-results
		assert.NoError(t, err, "Concurrent request %d failed", i)
	}
}

func TestServerMethodNotAllowed(t *testing.T) {
	config := ServerConfig{
		Username: "admin",
		Password: "password",
	}

	server := NewServer(config)
	defer server.Close()

	endpoints := []string{
		"/redfish/v1/",
		"/redfish/v1/Chassis",
		"/redfish/v1/Chassis/1",
		"/redfish/v1/Chassis/1/Power",
	}

	for _, endpoint := range endpoints {
		// Test POST on GET-only endpoints
		resp, err := http.Post(server.URL()+endpoint, "application/json", strings.NewReader("{}"))
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		if endpoint == "/redfish/v1/SessionService/Sessions" {
			// This endpoint accepts POST
			continue
		}

		assert.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode,
			"Endpoint %s should not allow POST", endpoint)
	}
}

func TestServerNotFoundEndpoints(t *testing.T) {
	config := ServerConfig{
		Username: "admin",
		Password: "password",
	}

	server := NewServer(config)
	defer server.Close()

	notFoundEndpoints := []string{
		"/redfish/v1/NonExistent",
		"/redfish/v1/Chassis/999",
		"/redfish/v1/Chassis/1/NonExistent",
		"/completely/wrong/path",
	}

	for _, endpoint := range notFoundEndpoints {
		resp, err := http.Get(server.URL() + endpoint)
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusNotFound, resp.StatusCode,
			"Endpoint %s should return 404", endpoint)
	}
}

func TestServerSessionManagement(t *testing.T) {
	config := ServerConfig{
		Username:   "admin",
		Password:   "password",
		EnableAuth: true,
	}

	server := NewServer(config)
	defer server.Close()

	// Create session
	sessionData := map[string]string{
		"UserName": "admin",
		"Password": "password",
	}
	body, _ := json.Marshal(sessionData)

	resp, err := http.Post(server.URL()+"/redfish/v1/SessionService/Sessions",
		"application/json", strings.NewReader(string(body)))
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	sessionLocation := resp.Header.Get("Location")
	assert.NotEmpty(t, sessionLocation)

	// Get session
	resp2, err := http.Get(server.URL() + sessionLocation)
	require.NoError(t, err)
	defer func() { _ = resp2.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp2.StatusCode)

	// Delete session
	req, _ := http.NewRequest("DELETE", server.URL()+sessionLocation, nil)
	resp3, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp3.Body.Close() }()

	assert.Equal(t, http.StatusNoContent, resp3.StatusCode)

	// Verify session is gone
	resp4, err := http.Get(server.URL() + sessionLocation)
	require.NoError(t, err)
	defer func() { _ = resp4.Body.Close() }()

	assert.Equal(t, http.StatusNotFound, resp4.StatusCode)
}

func TestServerSetError(t *testing.T) {
	server := NewServer(ServerConfig{
		Username:   "admin",
		Password:   "password",
		PowerWatts: 100.0,
		EnableAuth: false,
	})
	defer server.Close()

	// Test setting different error types
	testCases := []struct {
		name      string
		errorType ErrorType
	}{
		{"Connection Error", ErrorConnection},
		{"Auth Error", ErrorAuth},
		{"Timeout Error", ErrorTimeout},
		{"Missing Chassis", ErrorMissingChassis},
		{"Missing Power", ErrorMissingPower},
		{"Internal Server Error", ErrorInternalServer},
		{"Bad JSON", ErrorBadJSON},
		{"No Error", ErrorNone},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Set the error type
			server.SetError(tc.errorType)

			// Make a request to verify the error behavior
			resp, err := http.Get(server.URL() + "/redfish/v1/")
			assert.NoError(t, err)
			defer func() { assert.NoError(t, resp.Body.Close()) }()

			// Verify response based on error type
			switch tc.errorType {
			case ErrorNone:
				assert.Equal(t, http.StatusOK, resp.StatusCode)
			case ErrorInternalServer:
				assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
			case ErrorBadJSON:
				assert.Equal(t, http.StatusOK, resp.StatusCode)
				// Should have malformed JSON - verify by trying to parse
				body, readErr := io.ReadAll(resp.Body)
				assert.NoError(t, readErr)
				var jsonData any
				parseErr := json.Unmarshal(body, &jsonData)
				assert.Error(t, parseErr) // Should fail to parse
			default:
				// Most error types still return 200 but with error content
				assert.True(t, resp.StatusCode >= 200)
			}
		})
	}
}

func TestServerGetTLSCertificate(t *testing.T) {
	// Test with TLS enabled
	server := NewServer(ServerConfig{
		EnableTLS: true,
	})
	defer server.Close()

	cert := server.GetTLSCertificate()
	assert.NotNil(t, cert)
	assert.NotEmpty(t, cert.Certificate)
}

func TestServerGetTLSCertificateWithoutTLS(t *testing.T) {
	// Test with TLS disabled
	server := NewServer(ServerConfig{
		EnableTLS: false,
	})
	defer server.Close()

	cert := server.GetTLSCertificate()
	assert.Nil(t, cert)
}

func TestServerListSessions(t *testing.T) {
	server := NewServer(ServerConfig{
		Username:   "admin",
		Password:   "password",
		EnableAuth: true,
	})
	defer server.Close()

	// Create a session first
	sessionBody := `{"UserName":"admin","Password":"password"}`
	resp, err := http.Post(server.URL()+"/redfish/v1/SessionService/Sessions",
		"application/json", strings.NewReader(sessionBody))
	assert.NoError(t, err)
	assert.NoError(t, resp.Body.Close())
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	// List sessions
	resp, err = http.Get(server.URL() + "/redfish/v1/SessionService/Sessions")
	assert.NoError(t, err)
	defer func() { assert.NoError(t, resp.Body.Close()) }()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)

	var sessions map[string]any
	err = json.Unmarshal(body, &sessions)
	assert.NoError(t, err)

	// Should contain session information
	assert.Contains(t, sessions, "Members")
}

func TestSuccessScenarios(t *testing.T) {
	scenarios := SuccessScenarios()

	assert.NotEmpty(t, scenarios)

	// Verify all scenarios have valid configurations
	for _, scenario := range scenarios {
		assert.NotEmpty(t, scenario.Name)
		// Note: Username/Password can be empty for no-auth scenarios
		if scenario.Config.EnableAuth {
			assert.NotEmpty(t, scenario.Config.Username)
			assert.NotEmpty(t, scenario.Config.Password)
		}
		assert.Equal(t, ErrorNone, scenario.Config.ForceError)
	}
}

func TestErrorScenarios(t *testing.T) {
	scenarios := ErrorScenarios()

	assert.NotEmpty(t, scenarios)

	// Verify all scenarios have error conditions or special configurations
	for _, scenario := range scenarios {
		assert.NotEmpty(t, scenario.Name)
		// Error scenarios either have ForceError set OR have special conditions like slow response
		hasError := scenario.Config.ForceError != ErrorNone
		hasSpecialCondition := scenario.Config.SimulateSlowResponse
		assert.True(t, hasError || hasSpecialCondition,
			"Scenario %s should have either an error condition or special behavior", scenario.Name)
	}
}
