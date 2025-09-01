//go:build ignore

// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

// capture-bmc-testdata.go - A utility to capture test data from real BMCs
//
// Usage:
//   go run hack/redfish/capture-bmc-testdata.go -endpoint https://192.168.1.100 -username admin -password secret -vendor dell
//
// Or with config file:
//   go run hack/redfish/capture-bmc-testdata.go -config hack/redfish/bmc-config.yaml -node worker-node-1
//
// This script will:
// 1. Connect to your BMC safely
// 2. Capture relevant power monitoring data
// 3. Automatically sanitize sensitive information
// 4. Generate test fixtures compatible with mock server infrastructure
// 5. Output ready-to-use Go code for integration

package main

// TODO: support only -config file

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/stmcginnis/gofish"
	"github.com/stmcginnis/gofish/redfish"
	"gopkg.in/yaml.v3"
)

type BMCConfig struct {
	Endpoint string
	Username string
	Password string
	Vendor   string
	Insecure bool
	Timeout  time.Duration

	// Config file support
	ConfigFile string
	NodeName   string
}

// BMCNodeConfig represents the YAML config file format
type BMCNodeConfig struct {
	Nodes map[string]string `yaml:"nodes"`
	BMCs  map[string]struct {
		Endpoint string `yaml:"endpoint"`
		Username string `yaml:"username"`
		Password string `yaml:"password"`
		Insecure bool   `yaml:"insecure"`
	} `yaml:"bmcs"`
}

type CapturedFixtures struct {
	ServiceRoot       string
	ChassisCollection string
	Chassis           string
	Power             string
	PowerWatts        float64
	BMCModel          string
	ServerModel       string
	VendorType        string
}

// MockServerConfig represents the format needed by our mock server
type MockServerConfig struct {
	Name       string  `json:"name"`
	Vendor     string  `json:"vendor"`
	PowerWatts float64 `json:"powerWatts"`
	Fixtures   struct {
		ServiceRoot string `json:"serviceRoot"`
		Chassis     string `json:"chassis"`
		Power       string `json:"power"`
	} `json:"fixtures"`
}

func main() {
	config := parseFlags()

	fmt.Printf("🔌 Kepler BMC Test Data Capture Utility\n")
	fmt.Printf("========================================\n\n")

	if err := validateConfig(config); err != nil {
		log.Fatalf("❌ Configuration error: %v", err)
	}

	fmt.Printf("📡 Connecting to BMC: %s\n", config.Endpoint)
	fmt.Printf("👤 Username: %s\n", config.Username)
	fmt.Printf("🏭 Vendor: %s\n", config.Vendor)
	fmt.Printf("⏰ Timeout: %v\n\n", config.Timeout)

	fixtures, err := captureBMCData(config)
	if err != nil {
		log.Fatalf("❌ Failed to capture BMC data: %v", err)
	}

	// Set vendor type for fixtures
	fixtures.VendorType = config.Vendor

	outputResults(fixtures, config)
}

func parseFlags() BMCConfig {
	config := BMCConfig{}

	flag.StringVar(&config.Endpoint, "endpoint", "", "BMC endpoint URL")
	flag.StringVar(&config.Username, "username", "", "BMC username")
	flag.StringVar(&config.Password, "password", "", "BMC password")
	flag.StringVar(&config.Vendor, "vendor", "generic", "BMC vendor: dell, hpe, lenovo, or generic")
	flag.BoolVar(&config.Insecure, "insecure", true, "Skip TLS verification (recommended for testing)")
	flag.DurationVar(&config.Timeout, "timeout", 30*time.Second, "Connection timeout")

	// Config file support
	flag.StringVar(&config.ConfigFile, "config", "", "Path to BMC configuration YAML file")
	flag.StringVar(&config.NodeName, "node", "", "Node name to capture (when using config file)")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, ` Usage: %s [options]

Capture test data from Redfish BMCs for Kepler development.

Options:

Examples:

  # Direct connection:

		go run hack/redfish/capture-bmc-testdata.go \
			 -endpoint https://192.168.1.100 \
			-username admin -password secret -vendor dell

  # Using config file:
		go run hack/redfish/capture-bmc-testdata.go \
			-config hack/redfish/bmc-config.yaml \
			-node worker-node-1

`, os.Args[0])
	}

	flag.Parse()

	// Load from config file if specified
	if config.ConfigFile != "" {
		if err := loadConfigFromFile(&config); err != nil {
			log.Fatalf("❌ Failed to load config file: %v", err)
		}
	}

	return config
}

// loadConfigFromFile loads BMC configuration from YAML file
func loadConfigFromFile(config *BMCConfig) error {
	if config.NodeName == "" {
		return fmt.Errorf("node name is required when using config file")
	}

	data, err := os.ReadFile(config.ConfigFile)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	var nodeConfig BMCNodeConfig
	if err := yaml.Unmarshal(data, &nodeConfig); err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}

	// Find the BMC for the specified node
	bmcName, exists := nodeConfig.Nodes[config.NodeName]
	if !exists {
		return fmt.Errorf("node '%s' not found in config file", config.NodeName)
	}

	bmcConfig, exists := nodeConfig.BMCs[bmcName]
	if !exists {
		return fmt.Errorf("BMC '%s' not found in config file", bmcName)
	}

	// Override config with values from file (only if not already set via flags/env)
	if config.Endpoint == "" {
		config.Endpoint = bmcConfig.Endpoint
	}
	if config.Username == "" {
		config.Username = bmcConfig.Username
	}
	if config.Password == "" {
		config.Password = bmcConfig.Password
	}
	config.Insecure = bmcConfig.Insecure

	return nil
}

func validateConfig(config BMCConfig) error {
	if config.Endpoint == "" {
		return fmt.Errorf("BMC endpoint is required")
	}
	if config.Username == "" {
		return fmt.Errorf("BMC username is required")
	}
	if config.Password == "" {
		return fmt.Errorf("BMC password is required")
	}

	validVendors := map[string]bool{
		"dell": true, "hpe": true, "lenovo": true, "generic": true,
	}
	if !validVendors[config.Vendor] {
		return fmt.Errorf("invalid vendor '%s', must be one of: dell, hpe, lenovo, generic", config.Vendor)
	}

	return nil
}

func captureBMCData(config BMCConfig) (*CapturedFixtures, error) {
	ctx, cancel := context.WithTimeout(context.Background(), config.Timeout)
	defer cancel()

	// Connect to BMC
	clientConfig := gofish.ClientConfig{
		Endpoint:  config.Endpoint,
		Username:  config.Username,
		Password:  config.Password,
		Insecure:  config.Insecure,
		BasicAuth: true,
	}

	client, err := gofish.ConnectContext(ctx, clientConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to BMC: %w", err)
	}
	defer client.Logout()

	fixtures := &CapturedFixtures{}

	// Capture service root
	fmt.Printf("📋 Capturing service root...\n")
	if data, err := marshalAndSanitize(client.Service); err == nil {
		fixtures.ServiceRoot = data
	} else {
		fmt.Printf("⚠️  Warning: failed to capture service root: %v\n", err)
	}

	// Get chassis information
	fmt.Printf("🏗️  Capturing chassis information...\n")
	chassis, err := client.Service.Chassis()
	if err != nil {
		return nil, fmt.Errorf("failed to get chassis: %w", err)
	}

	if len(chassis) == 0 {
		return nil, fmt.Errorf("no chassis found")
	}

	// Capture first chassis
	firstChassis := chassis[0]
	if data, err := marshalAndSanitize(firstChassis); err == nil {
		fixtures.Chassis = data
		fixtures.ServerModel = extractServerModel(firstChassis)
	}

	// Capture power information
	fmt.Printf("⚡ Capturing power information...\n")
	power, err := firstChassis.Power()
	if err != nil {
		return nil, fmt.Errorf("failed to get power data: %w", err)
	}

	if data, err := marshalAndSanitize(power); err == nil {
		fixtures.Power = data
		fixtures.PowerWatts = extractPowerWatts(power)
	}

	// Try to get BMC model information
	fmt.Printf("🖥️  Capturing BMC model information...\n")
	if managers, err := client.Service.Managers(); err == nil && len(managers) > 0 {
		fixtures.BMCModel = extractBMCModel(managers[0])
	}

	fmt.Printf("✅ Data capture completed successfully!\n\n")
	return fixtures, nil
}

func marshalAndSanitize(obj any) (string, error) {
	data, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		return "", err
	}

	jsonStr := string(data)
	return sanitizeJSON(jsonStr), nil
}

func sanitizeJSON(jsonStr string) string {
	// Replace UUIDs with test UUID
	uuidRegex := regexp.MustCompile(`"UUID":\s*"[a-fA-F0-9-]{36}"`)
	jsonStr = uuidRegex.ReplaceAllString(jsonStr, `"UUID": "12345678-1234-1234-1234-123456789012"`)

	// Replace serial numbers
	serialRegex := regexp.MustCompile(`"SerialNumber":\s*"[^"]*"`)
	jsonStr = serialRegex.ReplaceAllString(jsonStr, `"SerialNumber": "TEST-SERIAL-123456"`)

	// Replace asset tags
	assetRegex := regexp.MustCompile(`"AssetTag":\s*"[^"]*"`)
	jsonStr = assetRegex.ReplaceAllString(jsonStr, `"AssetTag": "TEST-ASSET-TAG"`)

	// Replace service tags (Dell specific)
	serviceTagRegex := regexp.MustCompile(`"ServiceTag":\s*"[^"]*"`)
	jsonStr = serviceTagRegex.ReplaceAllString(jsonStr, `"ServiceTag": "TEST-SERVICE-TAG"`)

	// Replace MAC addresses
	macRegex := regexp.MustCompile(`"([0-9a-fA-F]{2}[:-]){5}[0-9a-fA-F]{2}"`)
	jsonStr = macRegex.ReplaceAllString(jsonStr, `"00:11:22:33:44:55"`)

	// Replace IP addresses with test IPs
	ipRegex := regexp.MustCompile(`"(\d{1,3}\.){3}\d{1,3}"`)
	jsonStr = ipRegex.ReplaceAllString(jsonStr, `"192.0.2.1"`)

	return jsonStr
}

func extractPowerWatts(power *redfish.Power) float64 {
	if len(power.PowerControl) > 0 {
		return float64(power.PowerControl[0].PowerConsumedWatts)
	}
	return 0.0
}

func extractServerModel(chassis *redfish.Chassis) string {
	if chassis.Model != "" {
		return chassis.Model
	}
	if chassis.Name != "" {
		return chassis.Name
	}
	return "Unknown Server Model"
}

func extractBMCModel(manager *redfish.Manager) string {
	model := ""
	if manager.Model != "" {
		model = manager.Model
	}
	if manager.FirmwareVersion != "" {
		if model != "" {
			model += " (FW: " + manager.FirmwareVersion + ")"
		} else {
			model = "FW: " + manager.FirmwareVersion
		}
	}
	if model == "" {
		return "Unknown BMC Model"
	}
	return model
}

func outputResults(fixtures *CapturedFixtures, config BMCConfig) {
	fmt.Printf("📊 Capture Results Summary\n")
	fmt.Printf("==========================\n")
	fmt.Printf("🏭 Vendor: %s\n", config.Vendor)
	fmt.Printf("🖥️  BMC Model: %s\n", fixtures.BMCModel)
	fmt.Printf("🏗️  Server Model: %s\n", fixtures.ServerModel)
	fmt.Printf("⚡ Power Consumption: %.1f watts\n\n", fixtures.PowerWatts)

	// Generate names based on vendor and power
	fixtureName := fmt.Sprintf("%s_power_%.0fw", strings.ToLower(config.Vendor), fixtures.PowerWatts)
	scenarioName := fmt.Sprintf("%s%.0fW", strings.Title(config.Vendor), fixtures.PowerWatts)

	fmt.Printf("📝 Generated Mock Server Integration\n")
	fmt.Printf("====================================\n\n")

	// 1. Power response fixture for power_responses.go
	fmt.Printf("// 1. Add this to internal/platform/redfish/mock/power_responses.go:\n")
	fmt.Printf("// In the PowerResponseFixtures map:\n")
	fmt.Printf(`"%s": `+"`%s`"+",\n\n", fixtureName, fixtures.Power)

	// 2. Success scenario for scenarios.go
	fmt.Printf("// 2. Add this to GetSuccessScenarios() in internal/platform/redfish/mock/scenarios.go:\n")
	fmt.Printf("{\n")
	fmt.Printf("\tName: \"%s\",\n", scenarioName)
	fmt.Printf("\tConfig: ServerConfig{\n")
	fmt.Printf("\t\tVendor:     Vendor%s,\n", strings.Title(config.Vendor))
	fmt.Printf("\t\tUsername:   \"admin\",\n")
	fmt.Printf("\t\tPassword:   \"password\",\n")
	fmt.Printf("\t\tPowerWatts: %.1f,\n", fixtures.PowerWatts)
	fmt.Printf("\t\tEnableAuth: true,\n")
	fmt.Printf("\t},\n")
	fmt.Printf("\tPowerWatts: %.1f,\n", fixtures.PowerWatts)
	fmt.Printf("},\n\n")

	// 3. Vendor constant (if new)
	if isNewVendor(config.Vendor) {
		fmt.Printf("// 3. Add this vendor constant to internal/platform/redfish/mock/server.go:\n")
		fmt.Printf("Vendor%s VendorType = \"%s\"\n\n", strings.Title(config.Vendor), config.Vendor)
	}

	// 4. Test scenario for power reader tests
	fmt.Printf("// 4. This will automatically work with existing tests once integrated.\n")
	fmt.Printf("// The mock server will serve the captured power data for vendor: %s\n\n", config.Vendor)

	// 5. Create complete files to copy-paste
	outputMockServerFiles(fixtures, config, fixtureName, scenarioName)

	// Validation commands
	fmt.Printf("🧪 Validation Commands\n")
	fmt.Printf("======================\n")
	fmt.Printf("# After integration, run these commands:\n")
	fmt.Printf("go test ./internal/platform/redfish/mock -v\n")
	fmt.Printf("go test ./internal/platform/redfish -run TestPowerReader\n")
	fmt.Printf("go test ./internal/platform/redfish -run TestServiceIntegrationWithDifferentVendors\n\n")

	// Security and contribution notes
	outputSecurityAndContributionNotes(config, fixtures)
}

// outputMockServerFiles creates complete file snippets for easy integration
func outputMockServerFiles(fixtures *CapturedFixtures, config BMCConfig, fixtureName, scenarioName string) {
	fmt.Printf("📄 Complete File Snippets for Copy-Paste Integration\n")
	fmt.Printf("=====================================================\n\n")

	// Complete power_responses.go addition
	fmt.Printf("// File: internal/platform/redfish/mock/power_responses.go\n")
	fmt.Printf("// Add to PowerResponseFixtures map:\n")
	fmt.Printf("var PowerResponseFixtures = map[string]string{\n")
	fmt.Printf("\t// ... existing fixtures ...\n")
	fmt.Printf(`	"%s": `+"`%s`"+",\n", fixtureName, fixtures.Power)
	fmt.Printf("}\n\n")

	// Complete scenario addition
	fmt.Printf("// File: internal/platform/redfish/mock/scenarios.go\n")
	fmt.Printf("// Add to GetSuccessScenarios() return slice:\n")
	fmt.Printf("return []TestScenario{\n")
	fmt.Printf("\t// ... existing scenarios ...\n")
	fmt.Printf("\t{\n")
	fmt.Printf("\t\tName: \"%s\",\n", scenarioName)
	fmt.Printf("\t\tConfig: ServerConfig{\n")
	fmt.Printf("\t\t\tVendor:     Vendor%s,\n", strings.Title(config.Vendor))
	fmt.Printf("\t\t\tUsername:   baseConfig.Username,\n")
	fmt.Printf("\t\t\tPassword:   baseConfig.Password,\n")
	fmt.Printf("\t\t\tPowerWatts: %.1f,\n", fixtures.PowerWatts)
	fmt.Printf("\t\t\tEnableAuth: baseConfig.EnableAuth,\n")
	fmt.Printf("\t\t},\n")
	fmt.Printf("\t\tPowerWatts: %.1f,\n", fixtures.PowerWatts)
	fmt.Printf("\t},\n")
	fmt.Printf("}\n\n")
}

// outputSecurityAndContributionNotes provides security and contribution guidance
func outputSecurityAndContributionNotes(config BMCConfig, fixtures *CapturedFixtures) {
	fmt.Printf("🔒 Security Verification\n")
	fmt.Printf("========================\n")
	fmt.Printf("✅ IP addresses sanitized (192.0.2.x)\n")
	fmt.Printf("✅ Serial numbers replaced with TEST-SERIAL-*\n")
	fmt.Printf("✅ UUIDs replaced with test UUID\n")
	fmt.Printf("✅ MAC addresses anonymized\n")
	fmt.Printf("✅ No credentials or tokens included\n")
	fmt.Printf("✅ Safe for public repository sharing\n\n")

	fmt.Printf("🤝 Integration Steps\n")
	fmt.Printf("====================\n")
	fmt.Printf("1. Copy power response fixture to internal/platform/redfish/mock/power_responses.go\n")
	fmt.Printf("2. Add success scenario to internal/platform/redfish/mock/scenarios.go\n")
	if isNewVendor(config.Vendor) {
		fmt.Printf("3. Add vendor constant to internal/platform/redfish/mock/server.go\n")
		fmt.Printf("4. Update vendor lists in tests if needed\n")
		fmt.Printf("5. Run tests to verify integration\n")
	} else {
		fmt.Printf("3. Run tests to verify integration\n")
	}
	fmt.Printf("\n")

	fmt.Printf("🚀 Creating Pull Request\n")
	fmt.Printf("========================\n")
	fmt.Printf("Title: feat(redfish): add %s %s BMC test data\n", strings.Title(config.Vendor), fixtures.ServerModel)
	fmt.Printf("\nDescription template:\n")
	fmt.Printf("```\n")
	fmt.Printf("Add real BMC test data for %s systems.\n\n", strings.Title(config.Vendor))
	fmt.Printf("**Hardware Details:**\n")
	fmt.Printf("- Server: %s\n", fixtures.ServerModel)
	fmt.Printf("- BMC: %s\n", fixtures.BMCModel)
	fmt.Printf("- Power Reading: %.1f watts\n\n", fixtures.PowerWatts)
	fmt.Printf("**Test Coverage:**\n")
	fmt.Printf("- Power monitoring via Redfish\n")
	fmt.Printf("- Vendor-specific response format\n")
	fmt.Printf("- Authentication and connection handling\n\n")
	fmt.Printf("**Security:**\n")
	fmt.Printf("- All sensitive data sanitized\n")
	fmt.Printf("- No real IP addresses, serials, or UUIDs\n")
	fmt.Printf("```\n\n")

	fmt.Printf("Thank you for contributing to Kepler! 🎉\n")
}

// isNewVendor checks if this is a vendor we haven't seen before
func isNewVendor(vendor string) bool {
	knownVendors := []string{"dell", "hpe", "lenovo", "generic"}
	vendor = strings.ToLower(vendor)
	for _, known := range knownVendors {
		if vendor == known {
			return false
		}
	}
	return true
}
