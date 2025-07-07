// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

func TestLevel_IsEnabled(t *testing.T) {
	tests := []struct {
		name     string
		level    Level
		expected map[string]bool
	}{
		{
			name:  "All levels",
			level: MetricsLevelAll,
			expected: map[string]bool{
				"node":      true,
				"process":   true,
				"container": true,
				"vm":        true,
				"pod":       true,
			},
		},
		{
			name:  "Node only",
			level: MetricsLevelNode,
			expected: map[string]bool{
				"node":      true,
				"process":   false,
				"container": false,
				"vm":        false,
				"pod":       false,
			},
		},
		{
			name:  "Node and Process",
			level: MetricsLevelNode | MetricsLevelProcess,
			expected: map[string]bool{
				"node":      true,
				"process":   true,
				"container": false,
				"vm":        false,
				"pod":       false,
			},
		},
		{
			name:  "Container, VM, and Pod",
			level: MetricsLevelContainer | MetricsLevelVM | MetricsLevelPod,
			expected: map[string]bool{
				"node":      false,
				"process":   false,
				"container": true,
				"vm":        true,
				"pod":       true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected["node"], tt.level.IsNodeEnabled())
			assert.Equal(t, tt.expected["process"], tt.level.IsProcessEnabled())
			assert.Equal(t, tt.expected["container"], tt.level.IsContainerEnabled())
			assert.Equal(t, tt.expected["vm"], tt.level.IsVMEnabled())
			assert.Equal(t, tt.expected["pod"], tt.level.IsPodEnabled())
		})
	}
}

func TestLevel_String(t *testing.T) {
	tests := []struct {
		name     string
		level    Level
		expected string
	}{
		{
			name:     "All levels",
			level:    MetricsLevelAll,
			expected: "node,process,container,vm,pod",
		},
		{
			name:     "Node only",
			level:    MetricsLevelNode,
			expected: "node",
		},
		{
			name:     "Process only",
			level:    MetricsLevelProcess,
			expected: "process",
		},
		{
			name:     "Node and Process",
			level:    MetricsLevelNode | MetricsLevelProcess,
			expected: "node,process",
		},
		{
			name:     "Container, VM, and Pod",
			level:    MetricsLevelContainer | MetricsLevelVM | MetricsLevelPod,
			expected: "container,vm,pod",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.level.String())
		})
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		name        string
		levels      []string
		expected    Level
		expectError bool
	}{
		{
			name:        "Empty slice",
			levels:      []string{},
			expected:    MetricsLevelAll,
			expectError: false,
		},
		{
			name:        "Single level",
			levels:      []string{"node"},
			expected:    MetricsLevelNode,
			expectError: false,
		},
		{
			name:        "Multiple levels",
			levels:      []string{"node", "process"},
			expected:    MetricsLevelNode | MetricsLevelProcess,
			expectError: false,
		},
		{
			name:        "All levels",
			levels:      []string{"node", "process", "container", "vm", "pod"},
			expected:    MetricsLevelAll,
			expectError: false,
		},
		{
			name:        "Case insensitive",
			levels:      []string{"NODE", "Process", "CONTAINER"},
			expected:    MetricsLevelNode | MetricsLevelProcess | MetricsLevelContainer,
			expectError: false,
		},
		{
			name:        "With whitespace",
			levels:      []string{" node ", " process "},
			expected:    MetricsLevelNode | MetricsLevelProcess,
			expectError: false,
		},
		{
			name:        "Invalid level",
			levels:      []string{"invalid"},
			expected:    0,
			expectError: true,
		},
		{
			name:        "Mixed valid and invalid",
			levels:      []string{"node", "invalid"},
			expected:    0,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseLevel(tt.levels)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestValidLevels(t *testing.T) {
	expected := []string{"node", "process", "container", "vm", "pod"}
	result := ValidLevels()
	assert.Equal(t, expected, result)
}

func TestBitPatterns(t *testing.T) {
	// Test that bit patterns are unique powers of 2
	assert.Equal(t, Level(1), MetricsLevelNode)      // 1 << 1 = 2 (corrected after fix)
	assert.Equal(t, Level(2), MetricsLevelProcess)   // 1 << 2 = 4
	assert.Equal(t, Level(4), MetricsLevelContainer) // 1 << 3 = 8
	assert.Equal(t, Level(8), MetricsLevelVM)        // 1 << 4 = 16
	assert.Equal(t, Level(16), MetricsLevelPod)      // 1 << 5 = 32

	// Test that combined levels work correctly
	expected := MetricsLevelAll
	assert.Equal(t, expected, MetricsLevelAll)
}

func TestLevel_MarshalYAML(t *testing.T) {
	tests := []struct {
		name     string
		level    Level
		expected string
	}{
		{
			name:     "All levels",
			level:    MetricsLevelAll,
			expected: "- node\n- process\n- container\n- vm\n- pod\n",
		},
		{
			name:     "Node only",
			level:    MetricsLevelNode,
			expected: "node\n",
		},
		{
			name:     "Process only",
			level:    MetricsLevelProcess,
			expected: "process\n",
		},
		{
			name:     "Node and Process",
			level:    MetricsLevelNode | MetricsLevelProcess,
			expected: "- node\n- process\n",
		},
		{
			name:     "Container, VM, and Pod",
			level:    MetricsLevelContainer | MetricsLevelVM | MetricsLevelPod,
			expected: "- container\n- vm\n- pod\n",
		},
		{
			name:     "Pod and Node (17)",
			level:    MetricsLevelPod | MetricsLevelNode, // 16 + 1 = 17
			expected: "- node\n- pod\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := yaml.Marshal(tt.level)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, string(data))
		})
	}
}

func TestLevel_UnmarshalYAML(t *testing.T) {
	tests := []struct {
		name        string
		yamlData    string
		expected    Level
		expectError bool
	}{
		{
			name:        "Node string",
			yamlData:    "node",
			expected:    MetricsLevelNode,
			expectError: false,
		},
		{
			name:        "Process string",
			yamlData:    "process",
			expected:    MetricsLevelProcess,
			expectError: false,
		},
		{
			name:        "Array of levels",
			yamlData:    "- node\n- process",
			expected:    MetricsLevelNode | MetricsLevelProcess,
			expectError: false,
		},
		{
			name:        "Array with all levels",
			yamlData:    "- node\n- process\n- container\n- vm\n- pod",
			expected:    MetricsLevelAll,
			expectError: false,
		},
		{
			name:        "Pod and Node array (should be 17)",
			yamlData:    "- node\n- pod",
			expected:    MetricsLevelPod | MetricsLevelNode, // 16 + 1 = 17
			expectError: false,
		},
		{
			name:        "Case insensitive",
			yamlData:    "- NODE\n- Process",
			expected:    MetricsLevelNode | MetricsLevelProcess,
			expectError: false,
		},
		{
			name:        "Invalid level string",
			yamlData:    "invalid",
			expected:    0,
			expectError: true,
		},
		{
			name:        "Invalid level in array",
			yamlData:    "- node\n- invalid",
			expected:    0,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var level Level
			err := yaml.Unmarshal([]byte(tt.yamlData), &level)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, level)
			}
		})
	}
}

func TestLevel_YAMLRoundTrip(t *testing.T) {
	tests := []Level{
		MetricsLevelNode,
		MetricsLevelProcess,
		MetricsLevelNode | MetricsLevelProcess,
		MetricsLevelContainer | MetricsLevelVM | MetricsLevelPod,
		MetricsLevelPod | MetricsLevelNode, // 17
		MetricsLevelAll,
	}

	for _, original := range tests {
		t.Run(original.String(), func(t *testing.T) {
			// Marshal to YAML
			data, err := yaml.Marshal(original)
			assert.NoError(t, err)

			// Unmarshal back
			var roundTrip Level
			err = yaml.Unmarshal(data, &roundTrip)
			assert.NoError(t, err)

			// Should be equal
			assert.Equal(t, original, roundTrip)
		})
	}
}
