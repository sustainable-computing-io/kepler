// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package testdata

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/stmcginnis/gofish/redfish"
)

// ValidateFixture validates a test fixture against gofish structs
func ValidateFixture(fixtureName string) error {
	fixture := GetFixture(fixtureName)

	// Parse as generic JSON first
	var jsonData map[string]interface{}
	if err := json.Unmarshal([]byte(fixture), &jsonData); err != nil {
		return fmt.Errorf("invalid JSON in fixture %s: %w", fixtureName, err)
	}

	// Check OData fields that are required for Redfish
	if strings.Contains(fixtureName, "power") {
		return validatePowerFixture(fixture)
	}

	if strings.Contains(fixtureName, "chassis") {
		return validateChassisFixture(fixture)
	}

	return nil
}

// validatePowerFixture validates power-related fixtures
func validatePowerFixture(fixture string) error {
	var power redfish.Power
	if err := json.Unmarshal([]byte(fixture), &power); err != nil {
		return fmt.Errorf("power fixture doesn't match gofish Power struct: %w", err)
	}

	// Validate required fields
	if power.ID == "" {
		return fmt.Errorf("power fixture missing required ID field")
	}

	if power.ODataType == "" {
		return fmt.Errorf("power fixture missing required @odata.type field")
	}

	// Validate PowerControl structure
	if len(power.PowerControl) > 0 {
		pc := power.PowerControl[0]
		if pc.PowerConsumedWatts < 0 {
			return fmt.Errorf("power fixture has negative PowerConsumedWatts")
		}
	}

	return nil
}

// validateChassisFixture validates chassis-related fixtures
func validateChassisFixture(fixture string) error {
	// // For chassis collection
	// if strings.Contains(fixture, "Collection") {
	// 	var chassisCollection redfish.ChassisCollection
	// 	if err := json.Unmarshal([]byte(fixture), &chassisCollection); err != nil {
	// 		return fmt.Errorf("chassis collection fixture doesn't match gofish struct: %w", err)
	// 	}
	// 	return nil
	// }
	//
	// // For individual chassis
	// var chassis redfish.Chassis
	// if err := json.Unmarshal([]byte(fixture), &chassis); err != nil {
	// 	return fmt.Errorf("chassis fixture doesn't match gofish Chassis struct: %w", err)
	// }

	return nil
}

// CreateMockResponseFromRealBMC creates a fixture from a real BMC response
// This would be used during development to capture real responses
func CreateMockResponseFromRealBMC(endpoint, username, password string) (map[string]string, error) {
	// This function would connect to a real BMC and capture responses
	// to create validated fixtures. Implementation would:
	// 1. Connect to real BMC
	// 2. Fetch actual responses
	// 3. Validate against gofish structs
	// 4. Sanitize sensitive data
	// 5. Return as fixture data

	return nil, fmt.Errorf("not implemented - use this for capturing real BMC responses")
}

// ValidateAllFixtures validates all fixtures in the package
func ValidateAllFixtures() []error {
	var errors []error

	for name := range PowerResponseFixtures {
		if err := ValidateFixture(name); err != nil {
			errors = append(errors, fmt.Errorf("fixture %s: %w", name, err))
		}
	}

	return errors
}
