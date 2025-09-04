// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package redfish

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadAndValidate(t *testing.T) {
	tt := []struct {
		name          string
		configContent string
		expectError   bool
		errorContains string
	}{{
		name: "Valid configuration",
		configContent: `
nodes:
  node1: bmc1
  node2: bmc2
bmcs:
  bmc1:
    endpoint: "https://bmc1.example.com"
    username: "admin"
    password: "secret"
  bmc2:
    endpoint: "https://bmc2.example.com"
    insecure: true
`,
		expectError: false,
	}, {
		name: "No credentials",
		configContent: `
nodes:
  node1: bmc1
bmcs:
  bmc1:
    endpoint: "https://bmc1.example.com"
`,
		expectError: false,
	}, {
		name: "Username without password",
		configContent: `
nodes:
  node1: bmc1
bmcs:
  bmc1:
    endpoint: "https://bmc1.example.com"
    username: "admin"
`,
		expectError:   true,
		errorContains: "password is required when username is provided",
	}, {
		name: "Password without username",
		configContent: `
nodes:
  node1: bmc1
bmcs:
  bmc1:
    endpoint: "https://bmc1.example.com"
    password: "secret"
`,
		expectError:   true,
		errorContains: "username is required when password is provided",
	}, {
		name: "Missing endpoint",
		configContent: `
nodes:
  node1: bmc1
bmcs:
  bmc1:
    username: "admin"
    password: "secret"
`,
		expectError:   true,
		errorContains: "endpoint is required",
	}, {
		name: "Node references non-existent BMC",
		configContent: `
nodes:
  node1: bmc1
  node2: nonexistent
bmcs:
  bmc1:
    endpoint: "https://bmc1.example.com"
`,
		expectError:   true,
		errorContains: "node node2 references non-existent BMC nonexistent",
	}, {
		name: "No nodes configured",
		configContent: `
bmcs:
  bmc1:
    endpoint: "https://bmc1.example.com"
`,
		expectError:   true,
		errorContains: "no nodes configured",
	}, {
		name: "No BMCs configured",
		configContent: `
nodes:
  node1: bmc1
`,
		expectError:   true,
		errorContains: "no BMCs configured",
	}}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir, err := os.MkdirTemp("", "config_test")
			require.NoError(t, err)
			defer func() { _ = os.RemoveAll(tmpDir) }()

			configFile := filepath.Join(tmpDir, "config.yaml")
			err = os.WriteFile(configFile, []byte(tc.configContent), 0644)
			require.NoError(t, err)

			config, err := Load(configFile)

			if tc.expectError {
				assert.Error(t, err)
				if tc.errorContains != "" {
					assert.Contains(t, err.Error(), tc.errorContains)
				}
				assert.Nil(t, config)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, config)
			}
		})
	}
}

func TestBMCIDForNodeSuccess(t *testing.T) {
	// Create temporary config file
	tmpDir, err := os.MkdirTemp("", "config_test")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	configContent := `
nodes:
  node1: bmc1
  node2: bmc2
bmcs:
  bmc1:
    endpoint: "https://bmc1.example.com"
  bmc2:
    endpoint: "https://bmc2.example.com"
`

	configFile := filepath.Join(tmpDir, "config.yaml")
	err = os.WriteFile(configFile, []byte(configContent), 0644)
	require.NoError(t, err)

	config, err := Load(configFile)
	require.NoError(t, err)

	tt := []struct {
		name     string
		nodeName string
		expected string
		wantErr  bool
	}{{
		name:     "Valid node1",
		nodeName: "node1",
		expected: "bmc1",
		wantErr:  false,
	}, {
		name:     "Valid node2",
		nodeName: "node2",
		expected: "bmc2",
		wantErr:  false,
	}, {
		name:     "Non-existent node",
		nodeName: "node3",
		wantErr:  true,
	}, {
		name:     "Empty node ID",
		nodeName: "",
		wantErr:  true,
	}}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			result, err := config.BMCIDForNode(tc.nodeName)

			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, result)
			}
		})
	}
}

func TestBMCForNodeEdgeCases(t *testing.T) {
	// Create temporary config with edge cases
	tmpDir, err := os.MkdirTemp("", "config_edge_test")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	configContent := `
nodes:
  node1: bmc1
  node2: nonexistent-bmc  # BMC that doesn't exist in bmcs section
bmcs:
  bmc1:
    endpoint: "https://bmc1.example.com"
    username: "admin"
    password: "secret"
    insecure: true
`

	configFile := filepath.Join(tmpDir, "config.yaml")
	err = os.WriteFile(configFile, []byte(configContent), 0644)
	require.NoError(t, err)

	_, err = Load(configFile)
	require.Error(t, err) // Should fail validation due to nonexistent-bmc

	// Test manually created config for edge cases
	config := &BMCConfig{
		Nodes: map[string]string{
			"node1": "bmc1",
		},
		BMCs: map[string]BMCDetail{
			"bmc1": {
				Endpoint: "https://bmc1.example.com",
				Username: "admin",
				Password: "secret",
				Insecure: true,
			},
		},
	}

	tt := []struct {
		name     string
		nodeName string
		wantErr  bool
	}{{
		name:     "Valid node",
		nodeName: "node1",
		wantErr:  false,
	}}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			_, err := config.BMCForNode(tc.nodeName)

			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
