// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package redfish

import (
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/stmcginnis/gofish"
	"github.com/stmcginnis/gofish/common"
	"github.com/stretchr/testify/require"

	"github.com/sustainable-computing-io/kepler/internal/platform/redfish/testdata"
)

// CreateMockResponse creates an HTTP response from a fixture
func CreateMockResponse(fixture string, statusCode int) *http.Response {
	body := io.NopCloser(strings.NewReader(testdata.GetFixture(fixture)))
	return &http.Response{
		StatusCode: statusCode,
		Body:       body,
		Header:     make(http.Header),
	}
}

// CreateSuccessResponse creates a successful HTTP response from a fixture
func CreateSuccessResponse(fixture string) *http.Response {
	return CreateMockResponse(fixture, http.StatusOK)
}

// CreateErrorResponse creates an error HTTP response from a fixture
func CreateErrorResponse(fixture string, statusCode int) *http.Response {
	return CreateMockResponse(fixture, statusCode)
}

// NewTestPowerReader creates a PowerReader with a mock gofish client
func NewTestPowerReader(t *testing.T, responses map[string]*http.Response) *PowerReader {
	testClient := &common.TestClient{}

	// Convert responses map to the slice format expected by gofish TestClient
	var getResponses []interface{}
	for _, response := range responses {
		getResponses = append(getResponses, response)
	}

	testClient.CustomReturnForActions = map[string][]interface{}{
		"GET": getResponses,
	}

	// Create a gofish API client with the test client
	apiClient := &gofish.APIClient{}

	// Create mock service to avoid connecting
	service := &gofish.Service{
		Entity: common.Entity{
			ODataID: "/redfish/v1/",
		},
	}
	apiClient.Service = service

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	powerReader := NewPowerReader(logger)
	powerReader.SetClient(apiClient)

	return powerReader
}

// PowerReadingScenario represents a test scenario for power readings
type PowerReadingScenario struct {
	Name          string
	Fixture       string
	ExpectedWatts float64
	ExpectError   bool
}

// GetPowerReadingScenarios returns predefined test scenarios
func GetPowerReadingScenarios() []PowerReadingScenario {
	return []PowerReadingScenario{
		{
			Name:          "DellPowerSuccess",
			Fixture:       "dell_power_245w",
			ExpectedWatts: 245.0,
			ExpectError:   false,
		},
		{
			Name:          "HPEPowerSuccess",
			Fixture:       "hpe_power_189w",
			ExpectedWatts: 189.5,
			ExpectError:   false,
		},
		{
			Name:          "LenovoPowerSuccess",
			Fixture:       "lenovo_power_167w",
			ExpectedWatts: 167.8,
			ExpectError:   false,
		},
		{
			Name:          "GenericPowerSuccess",
			Fixture:       "generic_power_200w",
			ExpectedWatts: 200.0,
			ExpectError:   false,
		},
		{
			Name:          "Generic590WFromJSON",
			Fixture:       "generic_power_590w",
			ExpectedWatts: 590.0,
			ExpectError:   false,
		},
		{
			Name:          "ZeroPowerReading",
			Fixture:       "zero_power",
			ExpectedWatts: 0.0,
			ExpectError:   false,
		},
	}
}

// GetErrorScenarios returns predefined error test scenarios
func GetErrorScenarios() []PowerReadingScenario {
	return []PowerReadingScenario{
		{
			Name:        "EmptyPowerControl",
			Fixture:     "empty_power_control",
			ExpectError: true,
		},
		{
			Name:        "ResourceNotFound",
			Fixture:     "error_not_found",
			ExpectError: true,
		},
		{
			Name:        "AuthenticationFailed",
			Fixture:     "error_auth_failed",
			ExpectError: true,
		},
	}
}

// AssertPowerReading validates a power reading with single chassis
func AssertPowerReading(t *testing.T, expected float64, actual *PowerReading) {
	require.NotNil(t, actual)
	require.False(t, actual.Timestamp.IsZero())
	require.NotEmpty(t, actual.Chassis, "PowerReading should contain at least one chassis")
	require.NotEmpty(t, actual.Chassis[0].Readings, "Chassis should contain at least one reading")

	// Check the first reading for backward compatibility with existing tests
	require.InDelta(t, expected, actual.Chassis[0].Readings[0].Power.Watts(), 0.001)
}
