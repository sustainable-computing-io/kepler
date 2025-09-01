// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package testdata

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJSONFixtureLoading(t *testing.T) {
	t.Run("LoadGeneric590W_FromJSON", func(t *testing.T) {
		fixture := GetFixture("generic_power_590w")
		assert.NotEmpty(t, fixture, "fixture should not be empty")

		// Verify it's valid JSON
		var powerData map[string]interface{}
		err := json.Unmarshal([]byte(fixture), &powerData)
		assert.NoError(t, err, "fixture should be valid JSON")
		assert.NotEmpty(t, powerData, "parsed JSON should not be empty")

		// Verify structure
		assert.Equal(t, "Power", powerData["Id"])
		assert.Equal(t, "Power", powerData["Name"])

		// Check power value
		powerControl, ok := powerData["PowerControl"].([]interface{})
		require.True(t, ok, "PowerControl should be array")
		require.Len(t, powerControl, 1, "Should have one PowerControl entry")

		control, ok := powerControl[0].(map[string]interface{})
		require.True(t, ok, "PowerControl[0] should be object")

		powerConsumed, ok := control["PowerConsumedWatts"].(float64)
		require.True(t, ok, "PowerConsumedWatts should be float64")
		assert.Equal(t, 590.0, powerConsumed, "Power consumption should be 590W")
	})

	t.Run("GetFixtureFromJSON_Direct", func(t *testing.T) {
		fixture, err := GetFixtureFromJSON("generic_power_590w")
		assert.NoError(t, err, "should load JSON fixture successfully")
		assert.NotEmpty(t, fixture, "fixture should not be empty")

		// Verify it's valid JSON
		var powerData map[string]interface{}
		err = json.Unmarshal([]byte(fixture), &powerData)
		assert.NoError(t, err, "fixture should be valid JSON")
	})

	t.Run("ListJSONFixtures", func(t *testing.T) {
		fixtures, err := ListJSONFixtures()
		assert.NoError(t, err, "should list fixtures successfully")
		assert.NotEmpty(t, fixtures, "should have at least one fixture")
		assert.Contains(t, fixtures, "generic_power_590w", "should contain our fixture")
	})

	t.Run("GetFixture_Fallback_To_Embedded", func(t *testing.T) {
		// This should work for existing embedded fixtures
		fixture := GetFixture("service_root")
		assert.NotEmpty(t, fixture, "should fallback to embedded fixtures")

		var data map[string]interface{}
		err := json.Unmarshal([]byte(fixture), &data)
		assert.NoError(t, err, "embedded fixture should be valid JSON")
	})
}

func TestJSONFixtureErrors(t *testing.T) {
	t.Run("GetFixtureFromJSON_NotFound", func(t *testing.T) {
		_, err := GetFixtureFromJSON("nonexistent_fixture")
		assert.Error(t, err, "should return error for missing fixture")
	})

	t.Run("GetFixture_NotFound", func(t *testing.T) {
		assert.Panics(t, func() {
			GetFixture("totally_nonexistent_fixture")
		}, "should panic for missing fixture")
	})
}
