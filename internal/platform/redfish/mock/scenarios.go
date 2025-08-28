// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package mock

import "time"

// TestScenario represents a test scenario configuration
type TestScenario struct {
	Name       string
	Config     ServerConfig
	PowerWatts float64 // For backward compatibility
}

// PowerReadingVariations contains different power consumption levels for testing
type PowerReadingVariations struct {
	Zero   float64
	Idle   float64
	Light  float64
	Medium float64
	Heavy  float64
	Peak   float64
}

// GetSuccessScenarios returns predefined success test scenarios
func GetSuccessScenarios() []TestScenario {
	baseConfig := ServerConfig{
		Username:   "admin",
		Password:   "password",
		EnableAuth: true,
		PowerWatts: 150.0,
	}

	return []TestScenario{
		{
			Name: "GenericVendor",
			Config: ServerConfig{
				Vendor:     VendorGeneric,
				Username:   baseConfig.Username,
				Password:   baseConfig.Password,
				PowerWatts: baseConfig.PowerWatts,
				EnableAuth: baseConfig.EnableAuth,
			},
			PowerWatts: baseConfig.PowerWatts,
		},
		{
			Name: "DellVendor",
			Config: ServerConfig{
				Vendor:     VendorDell,
				Username:   baseConfig.Username,
				Password:   baseConfig.Password,
				PowerWatts: 200.0,
				EnableAuth: baseConfig.EnableAuth,
			},
			PowerWatts: 200.0,
		},
		{
			Name: "HPEVendor",
			Config: ServerConfig{
				Vendor:     VendorHPE,
				Username:   baseConfig.Username,
				Password:   baseConfig.Password,
				PowerWatts: 180.0,
				EnableAuth: baseConfig.EnableAuth,
			},
			PowerWatts: 180.0,
		},
		{
			Name: "LenovoVendor",
			Config: ServerConfig{
				Vendor:     VendorLenovo,
				Username:   baseConfig.Username,
				Password:   baseConfig.Password,
				PowerWatts: 165.0,
				EnableAuth: baseConfig.EnableAuth,
			},
			PowerWatts: 165.0,
		},
		{
			Name: "NoAuthentication",
			Config: ServerConfig{
				Vendor:     VendorGeneric,
				Username:   "",
				Password:   "",
				PowerWatts: 100.0,
				EnableAuth: false,
			},
			PowerWatts: 100.0,
		},
		{
			Name: "TLSEnabled",
			Config: ServerConfig{
				Vendor:     VendorGeneric,
				Username:   baseConfig.Username,
				Password:   baseConfig.Password,
				PowerWatts: baseConfig.PowerWatts,
				EnableAuth: baseConfig.EnableAuth,
				EnableTLS:  true,
			},
			PowerWatts: baseConfig.PowerWatts,
		},
	}
}

// GetErrorScenarios returns predefined error test scenarios
func GetErrorScenarios() []TestScenario {
	baseConfig := ServerConfig{
		Vendor:     VendorGeneric,
		Username:   "admin",
		Password:   "password",
		PowerWatts: 150.0,
		EnableAuth: true,
	}

	return []TestScenario{
		{
			Name: "ConnectionError",
			Config: ServerConfig{
				Vendor:     baseConfig.Vendor,
				Username:   baseConfig.Username,
				Password:   baseConfig.Password,
				PowerWatts: baseConfig.PowerWatts,
				EnableAuth: baseConfig.EnableAuth,
				ForceError: ErrorConnection,
			},
		},
		{
			Name: "AuthenticationError",
			Config: ServerConfig{
				Vendor:     baseConfig.Vendor,
				Username:   "wrong",
				Password:   "wrong",
				PowerWatts: baseConfig.PowerWatts,
				EnableAuth: baseConfig.EnableAuth,
				ForceError: ErrorAuth,
			},
		},
		{
			Name: "TimeoutError",
			Config: ServerConfig{
				Vendor:     baseConfig.Vendor,
				Username:   baseConfig.Username,
				Password:   baseConfig.Password,
				PowerWatts: baseConfig.PowerWatts,
				EnableAuth: baseConfig.EnableAuth,
				ForceError: ErrorTimeout,
			},
		},
		{
			Name: "MissingChassis",
			Config: ServerConfig{
				Vendor:     baseConfig.Vendor,
				Username:   baseConfig.Username,
				Password:   baseConfig.Password,
				PowerWatts: baseConfig.PowerWatts,
				EnableAuth: baseConfig.EnableAuth,
				ForceError: ErrorMissingChassis,
			},
		},
		{
			Name: "MissingPowerInfo",
			Config: ServerConfig{
				Vendor:     baseConfig.Vendor,
				Username:   baseConfig.Username,
				Password:   baseConfig.Password,
				PowerWatts: baseConfig.PowerWatts,
				EnableAuth: baseConfig.EnableAuth,
				ForceError: ErrorMissingPower,
			},
		},
		{
			Name: "InternalServerError",
			Config: ServerConfig{
				Vendor:     baseConfig.Vendor,
				Username:   baseConfig.Username,
				Password:   baseConfig.Password,
				PowerWatts: baseConfig.PowerWatts,
				EnableAuth: baseConfig.EnableAuth,
				ForceError: ErrorInternalServer,
			},
		},
		{
			Name: "BadJSONResponse",
			Config: ServerConfig{
				Vendor:     baseConfig.Vendor,
				Username:   baseConfig.Username,
				Password:   baseConfig.Password,
				PowerWatts: baseConfig.PowerWatts,
				EnableAuth: baseConfig.EnableAuth,
				ForceError: ErrorBadJSON,
			},
		},
		{
			Name: "SlowResponse",
			Config: ServerConfig{
				Vendor:               baseConfig.Vendor,
				Username:             baseConfig.Username,
				Password:             baseConfig.Password,
				PowerWatts:           baseConfig.PowerWatts,
				EnableAuth:           baseConfig.EnableAuth,
				SimulateSlowResponse: true,
				ResponseDelay:        1 * time.Second,
			},
		},
	}
}

// CreateScenarioServer creates a mock server for a given test scenario
func CreateScenarioServer(scenario TestScenario) *Server {
	// Use PowerWatts from scenario config, fallback to scenario PowerWatts for backward compatibility
	if scenario.Config.PowerWatts == 0 && scenario.PowerWatts != 0 {
		scenario.Config.PowerWatts = scenario.PowerWatts
	}
	return NewServer(scenario.Config)
}

// GetPowerReadingVariations returns different power consumption levels for testing
func GetPowerReadingVariations() PowerReadingVariations {
	return PowerReadingVariations{
		Zero:   0.0,
		Idle:   45.0,
		Light:  120.0,
		Medium: 200.0,
		Heavy:  350.0,
		Peak:   500.0,
	}
}
