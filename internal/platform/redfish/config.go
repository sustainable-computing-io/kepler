// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package redfish

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// BMCConfig represents the configuration structure for BMC connections
type BMCConfig struct {
	Nodes map[string]string    `yaml:"nodes"` // Node name -> BMC ID mapping
	BMCs  map[string]BMCDetail `yaml:"bmcs"`  // BMC ID -> BMC connection details
}

// BMCDetail contains the connection details for a specific BMC
type BMCDetail struct {
	Endpoint string `yaml:"endpoint"` // BMC endpoint URL
	Username string `yaml:"username"` // BMC username
	Password string `yaml:"password"` // BMC password
	Insecure bool   `yaml:"insecure"` // Skip TLS verification
}

// LoadBMCConfig loads and parses the BMC configuration file
func LoadBMCConfig(configPath string) (*BMCConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read BMC config file %s: %w", configPath, err)
	}

	var config BMCConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse BMC config file %s: %w", configPath, err)
	}

	return &config, nil
}

// GetBMCForNode returns the BMC details for a given node name
func (c *BMCConfig) GetBMCForNode(nodeName string) (*BMCDetail, error) {
	bmcID, exists := c.Nodes[nodeName]
	if !exists {
		return nil, fmt.Errorf("node %s not found in BMC configuration", nodeName)
	}

	bmcDetail, exists := c.BMCs[bmcID]
	if !exists {
		return nil, fmt.Errorf("BMC %s not found in BMC configuration", bmcID)
	}

	return &bmcDetail, nil
}

// ResolveNodeID resolves the node identifier using the following precedence:
// 1. CLI flag (nodeID parameter)
// 2. Hostname fallback
func ResolveNodeID(nodeID string) (string, error) {
	// Priority 1: CLI flag
	if strings.TrimSpace(nodeID) != "" {
		return strings.TrimSpace(nodeID), nil
	}

	// TODO: move this to a config.go
	// Priority 2: Hostname fallback
	hostname, err := os.Hostname()
	if err != nil {
		return "", fmt.Errorf("failed to determine node identifier: %w", err)
	}

	return hostname, nil
}
