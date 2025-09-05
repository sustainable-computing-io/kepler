// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package redfish

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	redfishcfg "github.com/sustainable-computing-io/kepler/config/redfish"
)

// NewTestPowerReader creates a PowerReader with a mock gofish client
func NewTestPowerReader(t *testing.T, responses map[string]*http.Response) *PowerReader {
	// Create an HTTP test server that serves the mock responses
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Look for a matching response in our map
		if response, exists := responses[r.URL.Path]; exists {
			// Copy status code
			w.WriteHeader(response.StatusCode)

			// Copy headers
			for key, values := range response.Header {
				for _, value := range values {
					w.Header().Add(key, value)
				}
			}

			// Copy body
			if response.Body != nil {
				body, err := io.ReadAll(response.Body)
				if err == nil {
					_, err := w.Write(body)
					require.NoError(t, err)
				}
				// Reset the body for potential reuse
				response.Body = io.NopCloser(bytes.NewBuffer(body))
			}
		} else {
			// Return 404 for unmocked endpoints
			w.WriteHeader(http.StatusNotFound)
			_, err := w.Write([]byte("Not Found"))
			require.NoError(t, err)
		}
	}))

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	// Create mock BMC configuration pointing to our test server
	mockBMC := &redfishcfg.BMCDetail{
		Endpoint: server.URL,
		Username: "test-user",
		Password: "test-pass",
		Insecure: true,
	}

	powerReader := NewPowerReader(mockBMC, 30*time.Second, logger)

	// Add cleanup function to close the server when test completes
	t.Cleanup(func() {
		server.Close()
	})

	return powerReader
}

// AssertPowerReading validates a power reading with single chassis
func AssertPowerReading(t *testing.T, expected float64, actual *PowerReading) {
	t.Helper()
	require.NotNil(t, actual)
	require.False(t, actual.Timestamp.IsZero())
	require.NotEmpty(t, actual.Chassis, "PowerReading should contain at least one chassis")
	require.NotEmpty(t, actual.Chassis[0].Readings, "Chassis should contain at least one reading")

	// Check the first reading for backward compatibility with existing tests
	require.InDelta(t, expected, actual.Chassis[0].Readings[0].Power.Watts(), 0.001)
}
