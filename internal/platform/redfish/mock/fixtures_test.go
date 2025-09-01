// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package mock

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPowerResponseFixtures(t *testing.T) {
	t.Run("Dynamic_Power_Response_Generation", func(t *testing.T) {
		// Test that the mock server can generate dynamic responses
		response := PowerResponse(590.0)
		assert.NotEmpty(t, response, "should generate dynamic response")
		assert.Equal(t, "Power", response["Id"], "should have correct ID")
		assert.Equal(t, "Power", response["Name"], "should have correct Name")

		// Verify PowerControl structure
		powerControl, ok := response["PowerControl"].([]map[string]any)
		require.True(t, ok, "PowerControl should be array")
		require.Len(t, powerControl, 1, "Should have one PowerControl entry")

		// Verify dynamic power value
		powerConsumed, ok := powerControl[0]["PowerConsumedWatts"].(float64)
		require.True(t, ok, "PowerConsumedWatts should be float64")
		assert.Equal(t, 590.0, powerConsumed, "Power consumption should be 590W")
	})
}

func TestGeneric590WScenario(t *testing.T) {
	scenarios := SuccessScenarios()

	// Find our scenario
	var generic590WScenario *TestScenario
	for _, scenario := range scenarios {
		if scenario.Name == "Generic590W" {
			generic590WScenario = &scenario
			break
		}
	}

	require.NotNil(t, generic590WScenario, "Generic590W scenario should exist")
	assert.Equal(t, 590.0, generic590WScenario.Config.PowerWatts)
	assert.Equal(t, 590.0, generic590WScenario.PowerWatts)
}
