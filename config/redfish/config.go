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

// Load loads and parses the BMC configuration file
func Load(configPath string) (*BMCConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read BMC config file %s: %w", configPath, err)
	}

	var config BMCConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse BMC config file %s: %w", configPath, err)
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid BMC configuration: %w", err)
	}

	return &config, nil
}

// Validate validates the BMC configuration
func (c *BMCConfig) Validate() error {
	if len(c.Nodes) == 0 {
		return fmt.Errorf("no nodes configured")
	}

	if len(c.BMCs) == 0 {
		return fmt.Errorf("no BMCs configured")
	}

	// Validate that all node mappings point to valid BMCs
	for node, bmcID := range c.Nodes {
		if _, exists := c.BMCs[bmcID]; !exists {
			return fmt.Errorf("node %s references non-existent BMC %s", node, bmcID)
		}
	}

	// Validate BMC configurations
	for bmcID, bmc := range c.BMCs {
		if err := bmc.Validate(); err != nil {
			return fmt.Errorf("BMC %s configuration invalid: %w", bmcID, err)
		}
	}

	return nil
}

// Validate validates a BMC detail configuration
func (b *BMCDetail) Validate() error {
	if strings.TrimSpace(b.Endpoint) == "" {
		return fmt.Errorf("endpoint is required")
	}

	// Validate credentials - if one is provided, both must be provided
	hasUsername := strings.TrimSpace(b.Username) != ""
	hasPassword := strings.TrimSpace(b.Password) != ""

	if hasUsername && !hasPassword {
		return fmt.Errorf("password is required when username is provided")
	}

	if !hasUsername && hasPassword {
		return fmt.Errorf("username is required when password is provided")
	}

	return nil
}

// BMCForNode returns the BMC details for a given node name
func (c *BMCConfig) BMCForNode(nodeName string) (*BMCDetail, error) {
	bmcID, exists := c.Nodes[nodeName]
	if !exists {
		return nil, fmt.Errorf("node %s not found in BMC configuration", nodeName)
	}

	bmc, exists := c.BMCs[bmcID]
	if !exists {
		return nil, fmt.Errorf("BMC %s not found in BMC configuration", bmcID)
	}

	return &bmc, nil
}

// BMCIDForNode returns the BMC ID for a given node name
func (c *BMCConfig) BMCIDForNode(nodeName string) (string, error) {
	bmcID, exists := c.Nodes[nodeName]
	if !exists {
		return "", fmt.Errorf("node %s not found in BMC configuration", nodeName)
	}

	_, exists = c.BMCs[bmcID]
	if !exists {
		return "", fmt.Errorf("BMC %s not found in BMC configuration", bmcID)
	}

	return bmcID, nil
}
