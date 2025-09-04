// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package testutil

import "time"

// TestScenario represents a test scenario configuration
type TestScenario struct {
	Name       string
	Config     ServerConfig
	PowerWatts float64 // For backward compatibility
}

// SuccessScenarios returns predefined success test scenarios
func SuccessScenarios() []TestScenario {
	baseConfig := ServerConfig{
		Username:   "admin",
		Password:   "password",
		EnableAuth: true,
		PowerWatts: 150.0,
	}

	return []TestScenario{{
		Name: "BasicAuth",
		Config: ServerConfig{
			Username:   baseConfig.Username,
			Password:   baseConfig.Password,
			PowerWatts: baseConfig.PowerWatts,
			EnableAuth: baseConfig.EnableAuth,
		},
		PowerWatts: baseConfig.PowerWatts,
	}, {
		Name: "NoAuthentication",
		Config: ServerConfig{
			Username:   "",
			Password:   "",
			PowerWatts: 100.0,
			EnableAuth: false,
		},
		PowerWatts: 100.0,
	}, {
		Name: "TLSEnabled",
		Config: ServerConfig{
			Username:   baseConfig.Username,
			Password:   baseConfig.Password,
			PowerWatts: baseConfig.PowerWatts,
			EnableAuth: baseConfig.EnableAuth,
			EnableTLS:  true,
		},
		PowerWatts: baseConfig.PowerWatts,
	}, {
		Name: "Generic590W",
		Config: ServerConfig{
			Username:   baseConfig.Username,
			Password:   baseConfig.Password,
			PowerWatts: 590.0,
			EnableAuth: baseConfig.EnableAuth,
		},
		PowerWatts: 590.0,
	}, {
		Name: "ForceFallback",
		Config: ServerConfig{
			Username:      baseConfig.Username,
			Password:      baseConfig.Password,
			PowerWatts:    200.0,
			EnableAuth:    baseConfig.EnableAuth,
			ForceFallback: true, // Force fallback to Power API
		},
		PowerWatts: 200.0,
	}}
}

// ErrorScenarios returns predefined error test scenarios
func ErrorScenarios() []TestScenario {
	baseConfig := ServerConfig{
		Username:   "admin",
		Password:   "password",
		PowerWatts: 150.0,
		EnableAuth: true,
	}

	return []TestScenario{{
		Name: "ConnectionError",
		Config: ServerConfig{
			Username:   baseConfig.Username,
			Password:   baseConfig.Password,
			PowerWatts: baseConfig.PowerWatts,
			EnableAuth: baseConfig.EnableAuth,
			ForceError: ErrorConnection,
		},
	}, {
		Name: "AuthenticationError",
		Config: ServerConfig{
			Username:   "wrong",
			Password:   "wrong",
			PowerWatts: baseConfig.PowerWatts,
			EnableAuth: baseConfig.EnableAuth,
			ForceError: ErrorAuth,
		},
	}, {
		Name: "TimeoutError",
		Config: ServerConfig{
			Username:   baseConfig.Username,
			Password:   baseConfig.Password,
			PowerWatts: baseConfig.PowerWatts,
			EnableAuth: baseConfig.EnableAuth,
			ForceError: ErrorTimeout,
		},
	}, {
		Name: "MissingChassis",
		Config: ServerConfig{
			Username:   baseConfig.Username,
			Password:   baseConfig.Password,
			PowerWatts: baseConfig.PowerWatts,
			EnableAuth: baseConfig.EnableAuth,
			ForceError: ErrorMissingChassis,
		},
	}, {
		Name: "MissingPowerInfo",
		Config: ServerConfig{
			Username:   baseConfig.Username,
			Password:   baseConfig.Password,
			PowerWatts: baseConfig.PowerWatts,
			EnableAuth: baseConfig.EnableAuth,
			ForceError: ErrorMissingPower,
		},
	}, {
		Name: "InternalServerError",
		Config: ServerConfig{
			Username:   baseConfig.Username,
			Password:   baseConfig.Password,
			PowerWatts: baseConfig.PowerWatts,
			EnableAuth: baseConfig.EnableAuth,
			ForceError: ErrorInternalServer,
		},
	}, {
		Name: "BadJSONResponse",
		Config: ServerConfig{
			Username:   baseConfig.Username,
			Password:   baseConfig.Password,
			PowerWatts: baseConfig.PowerWatts,
			EnableAuth: baseConfig.EnableAuth,
			ForceError: ErrorBadJSON,
		},
	}, {
		Name: "SlowResponse",
		Config: ServerConfig{
			Username:             baseConfig.Username,
			Password:             baseConfig.Password,
			PowerWatts:           baseConfig.PowerWatts,
			EnableAuth:           baseConfig.EnableAuth,
			SimulateSlowResponse: true,
			ResponseDelay:        1 * time.Second,
		},
	}, {
		Name: "MissingPowerSubsystem",
		Config: ServerConfig{
			Username:   baseConfig.Username,
			Password:   baseConfig.Password,
			PowerWatts: baseConfig.PowerWatts,
			EnableAuth: baseConfig.EnableAuth,
			ForceError: ErrorMissingPowerSubsystem,
		},
	}}
}

// CreateScenarioServer creates a mock server for a given test scenario
func CreateScenarioServer(scenario TestScenario) *Server {
	// Use PowerWatts from scenario config, fallback to scenario PowerWatts for backward compatibility
	if scenario.Config.PowerWatts == 0 && scenario.PowerWatts != 0 {
		scenario.Config.PowerWatts = scenario.PowerWatts
	}
	return NewServer(scenario.Config)
}
