// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package testdata

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestFixtureValidation ensures all our test fixtures are valid
func TestFixtureValidation(t *testing.T) {
	errors := ValidateAllFixtures()

	for _, err := range errors {
		t.Errorf("Fixture validation failed: %v", err)
	}

	assert.Empty(t, errors, "All fixtures should be valid")
}

// TestIndividualFixtures tests each fixture type
func TestIndividualFixtures(t *testing.T) {
	tests := []struct {
		name        string
		fixtureName string
	}{
		{"ServiceRoot", "service_root"},
		{"ChassisCollection", "chassis_collection"},
		{"Chassis", "chassis"},
		{"DellPower", "dell_power_245w"},
		{"HPEPower", "hpe_power_189w"},
		{"LenovoPower", "lenovo_power_167w"},
		{"GenericPower", "generic_power_200w"},
		{"ZeroPower", "zero_power"},
		{"EmptyPowerControl", "empty_power_control"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFixture(tt.fixtureName)
			assert.NoError(t, err, "Fixture %s should be valid", tt.fixtureName)
		})
	}
}

// TestErrorFixtures ensures error fixtures are proper JSON
func TestErrorFixtures(t *testing.T) {
	errorFixtures := []string{
		"error_not_found",
		"error_auth_failed",
	}

	for _, fixtureName := range errorFixtures {
		t.Run(fixtureName, func(t *testing.T) {
			// Error fixtures should be valid JSON but won't match gofish structs
			err := ValidateFixture(fixtureName)
			// We expect validation to pass for JSON structure
			assert.NoError(t, err, "Error fixture %s should have valid JSON", fixtureName)
		})
	}
}
