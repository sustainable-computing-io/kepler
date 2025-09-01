// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package testdata

import (
	"embed"
	"fmt"
	"path/filepath"
	"strings"
)

//go:embed fixtures/*.json
var fixturesFS embed.FS

// PowerResponseFixtures contains JSON fixtures for different power response scenarios
// NOTE: Most fixtures are now loaded from JSON files in the fixtures/ directory.
// This map serves as a fallback for backward compatibility.
var PowerResponseFixtures = map[string]string{
	// All fixtures have been migrated to JSON files in fixtures/ directory
	// This map is kept for backward compatibility and will load from JSON files first
}

// GetFixture returns a fixture by name, loading from JSON files first, then fallback to embedded strings
func GetFixture(name string) string {
	// First try to load from JSON file
	jsonFilename := name + ".json"
	if data, err := fixturesFS.ReadFile(filepath.Join("fixtures", jsonFilename)); err == nil {
		return string(data)
	}

	// Fallback to embedded string fixtures
	fixture, exists := PowerResponseFixtures[name]
	if !exists {
		panic(fmt.Sprintf("fixture not found: %s (tried JSON file %s and embedded fixtures)", name, jsonFilename))
	}
	return fixture
}

// GetFixtureFromJSON loads a fixture directly from a JSON file
func GetFixtureFromJSON(filename string) (string, error) {
	if !strings.HasSuffix(filename, ".json") {
		filename += ".json"
	}

	data, err := fixturesFS.ReadFile(filepath.Join("fixtures", filename))
	if err != nil {
		return "", fmt.Errorf("failed to load JSON fixture %s: %w", filename, err)
	}

	return string(data), nil
}

// ListJSONFixtures returns a list of available JSON fixture files
func ListJSONFixtures() ([]string, error) {
	entries, err := fixturesFS.ReadDir("fixtures")
	if err != nil {
		return nil, fmt.Errorf("failed to read fixtures directory: %w", err)
	}

	var fixtures []string
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".json") {
			// Return name without .json extension to match GetFixture convention
			name := strings.TrimSuffix(entry.Name(), ".json")
			fixtures = append(fixtures, name)
		}
	}

	return fixtures, nil
}
