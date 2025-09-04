// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package testutil

import (
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"time"
)

// ServerConfig holds configuration for the mock server
type ServerConfig struct {
	Username             string
	Password             string
	PowerWatts           float64
	EnableAuth           bool
	EnableTLS            bool
	SimulateSlowResponse bool
	ResponseDelay        time.Duration
	ForceError           ErrorType
	SessionTimeout       time.Duration
	ForceFallback        bool // Force fallback to Power API (disable PowerSubsystem)
}

// ErrorType represents different error scenarios
type ErrorType string

const (
	ErrorNone                  ErrorType = ""
	ErrorConnection            ErrorType = "connection"
	ErrorAuth                  ErrorType = "auth"
	ErrorTimeout               ErrorType = "timeout"
	ErrorMissingChassis        ErrorType = "missing_chassis"
	ErrorMissingPower          ErrorType = "missing_power"
	ErrorMissingPowerSubsystem ErrorType = "missing_power_subsystem"
	ErrorMissingBothAPIs       ErrorType = "missing_both_apis"
	ErrorInternalServer        ErrorType = "internal_server"
	ErrorBadJSON               ErrorType = "bad_json"
)

// Server represents a mock Redfish BMC server
type Server struct {
	server *httptest.Server
	config ServerConfig

	mutex         sync.RWMutex
	sessions      map[string]time.Time // Track active sessions
	calledAPIs    []string             // Track which API endpoints were called
	apiCallCounts map[string]int       // Count calls to each API endpoint
}

// NewServer creates a new mock Redfish server
func NewServer(config ServerConfig) *Server {
	// Set defaults
	if config.Username == "" {
		config.Username = "admin"
	}
	if config.Password == "" {
		config.Password = "password"
	}
	// Don't set default PowerWatts - tests should explicitly set the value they want
	// This allows testing zero power consumption scenarios
	if config.SessionTimeout == 0 {
		config.SessionTimeout = 30 * time.Minute
	}

	s := &Server{
		config:        config,
		sessions:      make(map[string]time.Time),
		calledAPIs:    make([]string, 0),
		apiCallCounts: make(map[string]int),
	}

	// Create HTTP server with custom handler
	if config.EnableTLS {
		s.server = httptest.NewTLSServer(http.HandlerFunc(s.handler))
	} else {
		s.server = httptest.NewServer(http.HandlerFunc(s.handler))
	}

	return s
}

// URL returns the server's URL
func (s *Server) URL() string {
	return s.server.URL
}

// Close shuts down the mock server
func (s *Server) Close() {
	s.server.Close()
}

// SetPowerWatts dynamically sets the power reading for testing
func (s *Server) SetPowerWatts(watts float64) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.config.PowerWatts = watts
}

// SetError forces a specific error scenario
func (s *Server) SetError(errorType ErrorType) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.config.ForceError = errorType
}

// CalledAPIs returns a list of API endpoints that were called
func (s *Server) CalledAPIs() []string {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	// Return a copy to avoid race conditions
	called := make([]string, len(s.calledAPIs))
	copy(called, s.calledAPIs)
	return called
}

// APICallCount returns the number of times a specific API endpoint was called
func (s *Server) APICallCount(endpoint string) int {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.apiCallCounts[endpoint]
}

// ResetAPITracking clears the API call tracking
func (s *Server) ResetAPITracking() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.calledAPIs = make([]string, 0)
	s.apiCallCounts = make(map[string]int)
}

// trackAPICall records that an API endpoint was called
func (s *Server) trackAPICall(endpoint string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.calledAPIs = append(s.calledAPIs, endpoint)
	s.apiCallCounts[endpoint]++
}

// TLSCertificate returns the server's TLS certificate (for testing TLS scenarios)
func (s *Server) TLSCertificate() *tls.Certificate {
	if s.server.TLS != nil && len(s.server.TLS.Certificates) > 0 {
		return &s.server.TLS.Certificates[0]
	}
	return nil
}

// handler is the main HTTP handler for the mock server
func (s *Server) handler(w http.ResponseWriter, r *http.Request) {
	// Debug logging (remove in production)
	fmt.Printf("[MockServer] %s %s - Auth: %s\n", r.Method, r.URL.Path, r.Header.Get("Authorization"))

	// Simulate slow response if configured
	if s.config.SimulateSlowResponse {
		select {
		case <-r.Context().Done():
			return // Client cancelled, exit immediately
		case <-time.After(s.config.ResponseDelay):
			// Continue with normal processing
		}
	}

	// Handle forced errors
	s.mutex.RLock()
	forceError := s.config.ForceError
	s.mutex.RUnlock()

	switch forceError {
	case ErrorConnection:
		// Simulate connection error by closing connection
		return
	case ErrorTimeout:
		// Force timeout by sleeping longer than client timeout
		select {
		case <-r.Context().Done():
			return // Client cancelled, exit immediately
		case <-time.After(2 * time.Second):
			return // Force timeout
		}
	case ErrorInternalServer:
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Set common headers
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("OData-Version", "4.0")

	// Route requests to appropriate handlers
	switch r.URL.Path {
	case "/redfish/v1/", "/redfish/v1":
		s.handleServiceRoot(w, r)
	case "/redfish/v1/SessionService/Sessions":
		s.handleSessionService(w, r)
	case "/redfish/v1/Chassis":
		s.handleChassisCollection(w, r)
	case "/redfish/v1/Chassis/1":
		s.handleChassis(w, r)
	case "/redfish/v1/Chassis/1/Power":
		s.handlePower(w, r)
	case "/redfish/v1/Chassis/1/PowerSubsystem":
		s.handlePowerSubsystem(w, r)
	case "/redfish/v1/Chassis/1/PowerSubsystem/PowerSupplies":
		s.handlePowerSupplies(w, r)
	case "/redfish/v1/Chassis/1/PowerSubsystem/PowerSupplies/PS1":
		s.handleIndividualPowerSupply(w, r, "PS1")
	case "/redfish/v1/Chassis/1/PowerSubsystem/PowerSupplies/PS2":
		s.handleIndividualPowerSupply(w, r, "PS2")
	default:
		if strings.HasPrefix(r.URL.Path, "/redfish/v1/SessionService/Sessions/") {
			// Handle individual session endpoints
			s.handleSession(w, r)
		} else {
			http.NotFound(w, r)
		}
	}
}

// handleServiceRoot handles the Redfish service root endpoint
func (s *Server) handleServiceRoot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.config.ForceError == ErrorBadJSON {
		_, _ = w.Write([]byte("{invalid json"))
		return
	}

	response := map[string]any{
		"@odata.context": "/redfish/v1/$metadata#ServiceRoot.ServiceRoot",
		"@odata.type":    "#ServiceRoot.v1_5_0.ServiceRoot",
		"@odata.id":      "/redfish/v1/",
		"Id":             "RootService",
		"Name":           "Root Service",
		"RedfishVersion": "1.6.1",
		"UUID":           "12345678-1234-1234-1234-123456789012",
		"Chassis": map[string]any{
			"@odata.id": "/redfish/v1/Chassis",
		},
		"SessionService": map[string]any{
			"@odata.id": "/redfish/v1/SessionService",
		},
		"Links": map[string]any{
			"Sessions": map[string]any{
				"@odata.id": "/redfish/v1/SessionService/Sessions",
			},
		},
	}

	_ = json.NewEncoder(w).Encode(response)
}

// handleChassisCollection handles the chassis collection endpoint
func (s *Server) handleChassisCollection(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.config.ForceError == ErrorMissingChassis {
		http.NotFound(w, r)
		return
	}

	response := map[string]any{
		"@odata.context":      "/redfish/v1/$metadata#ChassisCollection.ChassisCollection",
		"@odata.type":         "#ChassisCollection.ChassisCollection",
		"@odata.id":           "/redfish/v1/Chassis",
		"Name":                "Chassis Collection",
		"Members@odata.count": 1,
		"Members": []map[string]any{
			{
				"@odata.id": "/redfish/v1/Chassis/1",
			},
		},
	}

	_ = json.NewEncoder(w).Encode(response)
}

// handleChassis handles individual chassis endpoint
func (s *Server) handleChassis(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	response := map[string]any{
		"@odata.context": "/redfish/v1/$metadata#Chassis.Chassis",
		"@odata.type":    "#Chassis.v1_10_0.Chassis",
		"@odata.id":      "/redfish/v1/Chassis/1",
		"Id":             "1",
		"Name":           "Computer System Chassis",
		"ChassisType":    "RackMount",
		"Manufacturer":   "generic",
		"PowerState":     "On",
		"Status": map[string]any{
			"State":  "Enabled",
			"Health": "OK",
		},
		"Power": map[string]any{
			"@odata.id": "/redfish/v1/Chassis/1/Power",
		},
		"PowerSubsystem": map[string]any{
			"@odata.id": "/redfish/v1/Chassis/1/PowerSubsystem",
		},
	}

	_ = json.NewEncoder(w).Encode(response)
}

// handlePower handles power endpoint for chassis
func (s *Server) handlePower(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Track that Power API was called
	s.trackAPICall("Power")

	if s.config.ForceError == ErrorMissingPower || s.config.ForceError == ErrorMissingBothAPIs {
		http.NotFound(w, r)
		return
	}

	if s.config.ForceError == ErrorBadJSON {
		_, _ = w.Write([]byte("{invalid json"))
		return
	}

	s.mutex.RLock()
	powerWatts := s.config.PowerWatts
	s.mutex.RUnlock()

	// Power API returns the full chassis power consumption
	response := PowerResponse(powerWatts)
	_ = json.NewEncoder(w).Encode(response)
}

// handleSessionService handles session management
func (s *Server) handleSessionService(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		s.createSession(w, r)
	case http.MethodGet:
		s.listSessions(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// createSession creates a new authentication session
func (s *Server) createSession(w http.ResponseWriter, r *http.Request) {
	if !s.config.EnableAuth {
		// Skip authentication if disabled
		sessionID := fmt.Sprintf("session_%d", time.Now().Unix())
		response := map[string]any{
			"@odata.context": "/redfish/v1/$metadata#Session.Session",
			"@odata.type":    "#Session.v1_1_0.Session",
			"@odata.id":      fmt.Sprintf("/redfish/v1/SessionService/Sessions/%s", sessionID),
			"Id":             sessionID,
			"Name":           "Session",
			"UserName":       "admin",
		}
		w.Header().Set("X-Auth-Token", "dummy-token-12345")
		w.Header().Set("Location", fmt.Sprintf("/redfish/v1/SessionService/Sessions/%s", sessionID))
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(response)
		return
	}

	if s.config.ForceError == ErrorAuth {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Parse request body for credentials
	var creds struct {
		UserName string `json:"UserName"`
		Password string `json:"Password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&creds); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	// Validate credentials
	if creds.UserName == "" || creds.Password == "" ||
		creds.UserName != s.config.Username || creds.Password != s.config.Password {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Create session
	sessionID := fmt.Sprintf("session_%d", time.Now().Unix())
	s.mutex.Lock()
	s.sessions[sessionID] = time.Now().Add(s.config.SessionTimeout)
	s.mutex.Unlock()

	response := map[string]any{
		"@odata.context": "/redfish/v1/$metadata#Session.Session",
		"@odata.type":    "#Session.v1_1_0.Session",
		"@odata.id":      fmt.Sprintf("/redfish/v1/SessionService/Sessions/%s", sessionID),
		"Id":             sessionID,
		"Name":           "Session",
		"UserName":       creds.UserName,
	}

	w.Header().Set("X-Auth-Token", base64.StdEncoding.EncodeToString([]byte(sessionID)))
	w.Header().Set("Location", fmt.Sprintf("/redfish/v1/SessionService/Sessions/%s", sessionID))
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(response)
}

// listSessions lists active sessions
func (s *Server) listSessions(w http.ResponseWriter, r *http.Request) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	var members []map[string]any
	for sessionID := range s.sessions {
		members = append(members, map[string]any{
			"@odata.id": fmt.Sprintf("/redfish/v1/SessionService/Sessions/%s", sessionID),
		})
	}

	response := map[string]any{
		"@odata.context":      "/redfish/v1/$metadata#SessionCollection.SessionCollection",
		"@odata.type":         "#SessionCollection.SessionCollection",
		"@odata.id":           "/redfish/v1/SessionService/Sessions",
		"Name":                "Session Collection",
		"Members@odata.count": len(members),
		"Members":             members,
	}

	_ = json.NewEncoder(w).Encode(response)
}

// handleSession handles individual session operations
func (s *Server) handleSession(w http.ResponseWriter, r *http.Request) {
	sessionID := strings.TrimPrefix(r.URL.Path, "/redfish/v1/SessionService/Sessions/")

	switch r.Method {
	case http.MethodGet:
		s.getSession(w, r, sessionID)
	case http.MethodDelete:
		s.deleteSession(w, r, sessionID)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// getSession retrieves session information
func (s *Server) getSession(w http.ResponseWriter, r *http.Request, sessionID string) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if _, exists := s.sessions[sessionID]; !exists {
		http.NotFound(w, r)
		return
	}

	response := map[string]any{
		"@odata.context": "/redfish/v1/$metadata#Session.Session",
		"@odata.type":    "#Session.v1_1_0.Session",
		"@odata.id":      fmt.Sprintf("/redfish/v1/SessionService/Sessions/%s", sessionID),
		"Id":             sessionID,
		"Name":           "Session",
		"UserName":       s.config.Username,
	}

	_ = json.NewEncoder(w).Encode(response)
}

// deleteSession removes a session
func (s *Server) deleteSession(w http.ResponseWriter, r *http.Request, sessionID string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if _, exists := s.sessions[sessionID]; !exists {
		http.NotFound(w, r)
		return
	}

	delete(s.sessions, sessionID)
	w.WriteHeader(http.StatusNoContent)
}

// handlePowerSubsystem handles PowerSubsystem endpoint
func (s *Server) handlePowerSubsystem(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check if PowerSubsystem should be disabled for fallback testing
	if s.config.ForceFallback || s.config.ForceError == ErrorMissingPowerSubsystem || s.config.ForceError == ErrorMissingBothAPIs {
		http.NotFound(w, r)
		return
	}

	// PowerSubsystem endpoint - return PowerSubsystem object with link to PowerSupplies
	response := map[string]any{
		"@odata.context": "/redfish/v1/$metadata#PowerSubsystem.PowerSubsystem",
		"@odata.type":    "#PowerSubsystem.v1_1_0.PowerSubsystem",
		"@odata.id":      "/redfish/v1/Chassis/1/PowerSubsystem",
		"Id":             "PowerSubsystem",
		"Name":           "Power Subsystem for Chassis",
		"CapacityWatts":  1500.0,
		"Status": map[string]any{
			"State":  "Enabled",
			"Health": "OK",
		},
		"PowerSupplies": map[string]any{
			"@odata.id": "/redfish/v1/Chassis/1/PowerSubsystem/PowerSupplies",
		},
		"PowerSupplyRedundancy": []map[string]any{
			{
				"RedundancyType":      "N+1",
				"MaxSupportedInGroup": 2,
				"MinNeededInGroup":    1,
				"RedundancyGroup": []map[string]any{
					{"@odata.id": "/redfish/v1/Chassis/1/PowerSubsystem/PowerSupplies/PS1"},
					{"@odata.id": "/redfish/v1/Chassis/1/PowerSubsystem/PowerSupplies/PS2"},
				},
				"Status": map[string]any{"State": "Enabled", "Health": "OK"},
			},
		},
	}

	_ = json.NewEncoder(w).Encode(response)
}

// handlePowerSupplies handles PowerSupplies collection endpoint
func (s *Server) handlePowerSupplies(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Track that PowerSubsystem API was called
	s.trackAPICall("PowerSubsystem")

	// Check if PowerSubsystem should be disabled for fallback testing
	if s.config.ForceFallback || s.config.ForceError == ErrorMissingPowerSubsystem || s.config.ForceError == ErrorMissingBothAPIs {
		http.NotFound(w, r)
		return
	}

	if s.config.ForceError == ErrorBadJSON {
		_, _ = w.Write([]byte("{invalid json"))
		return
	}

	s.mutex.RLock()
	powerWatts := s.config.PowerWatts
	s.mutex.RUnlock()

	// PowerSupplies collection - return collection of PowerSupply objects
	response := map[string]any{
		"@odata.context":      "/redfish/v1/$metadata#PowerSupplyCollection.PowerSupplyCollection",
		"@odata.type":         "#PowerSupplyCollection.PowerSupplyCollection",
		"@odata.id":           "/redfish/v1/Chassis/1/PowerSubsystem/PowerSupplies",
		"Name":                "Power Supply Collection",
		"Members@odata.count": 2,
		"Members": []map[string]any{
			{
				"@odata.id":   "/redfish/v1/Chassis/1/PowerSubsystem/PowerSupplies/PS1",
				"@odata.type": "#PowerSupply.v1_6_0.PowerSupply",
				"Id":          "PS1",
				"Name":        "Power Supply 1",
				"MemberId":    "PS1",
				"Status": map[string]any{
					"State":  "Enabled",
					"Health": "OK",
				},
				"PowerSupplyType":    "AC",
				"PowerCapacityWatts": 750.0,
				"PowerOutputWatts":   powerWatts,       // Each PSU reports the total chassis power for consistency
				"PowerInputWatts":    powerWatts / 0.9, // Account for 90% efficiency
				"EfficiencyPercent":  90.0,
				"Manufacturer":       "Generic",
				"Model":              "PSU-750W",
				"SerialNumber":       "SN123456",
				"PartNumber":         "PN123-750",
			},
			{
				"@odata.id":   "/redfish/v1/Chassis/1/PowerSubsystem/PowerSupplies/PS2",
				"@odata.type": "#PowerSupply.v1_6_0.PowerSupply",
				"Id":          "PS2",
				"Name":        "Power Supply 2",
				"MemberId":    "PS2",
				"Status": map[string]any{
					"State":  "Enabled",
					"Health": "OK",
				},
				"PowerSupplyType":    "AC",
				"PowerCapacityWatts": 750.0,
				"PowerOutputWatts":   powerWatts,       // Each PSU reports the total chassis power for consistency
				"PowerInputWatts":    powerWatts / 0.9, // Account for 90% efficiency
				"EfficiencyPercent":  90.0,
				"Manufacturer":       "Generic",
				"Model":              "PSU-750W",
				"SerialNumber":       "SN654321",
				"PartNumber":         "PN456-750",
			},
		},
	}
	_ = json.NewEncoder(w).Encode(response)
}

// handleIndividualPowerSupply handles individual PowerSupply endpoint
func (s *Server) handleIndividualPowerSupply(w http.ResponseWriter, r *http.Request, powerSupplyID string) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Track that PowerSubsystem API was called
	s.trackAPICall("PowerSubsystem")

	// Check if PowerSubsystem should be disabled for fallback testing
	if s.config.ForceFallback || s.config.ForceError == ErrorMissingPowerSubsystem || s.config.ForceError == ErrorMissingBothAPIs {
		http.NotFound(w, r)
		return
	}

	if s.config.ForceError == ErrorBadJSON {
		_, _ = w.Write([]byte("{invalid json"))
		return
	}

	s.mutex.RLock()
	powerWatts := s.config.PowerWatts
	s.mutex.RUnlock()

	// Inline individual PowerSupply response (avoiding unused function)
	response := map[string]any{
		"@odata.context":       "/redfish/v1/$metadata#PowerSupply.PowerSupply",
		"@odata.type":          "#PowerSupply.v1_6_0.PowerSupply",
		"@odata.id":            fmt.Sprintf("/redfish/v1/Chassis/1/PowerSubsystem/PowerSupplies/%s", powerSupplyID),
		"Id":                   powerSupplyID,
		"Name":                 fmt.Sprintf("Power Supply %s", powerSupplyID),
		"MemberId":             powerSupplyID,
		"PowerInputWatts":      powerWatts / 0.9, // Account for 90% efficiency
		"PowerOutputWatts":     powerWatts,       // Each PSU reports the total chassis power for consistency
		"PowerCapacityWatts":   750.0,
		"EfficiencyPercent":    90.0,
		"HotPluggable":         true,
		"PowerSupplyType":      "AC",
		"LineInputVoltageType": "ACHighLine",
		"LineInputVoltage":     240.0,
		"Status": map[string]any{
			"State":  "Enabled",
			"Health": "OK",
		},
		"Manufacturer": "Generic",
		"Model":        fmt.Sprintf("PSU-%s-750W", powerSupplyID),
		"SerialNumber": fmt.Sprintf("SN%s123456", powerSupplyID),
		"PartNumber":   fmt.Sprintf("PN%s-750", powerSupplyID),
	}
	_ = json.NewEncoder(w).Encode(response)
}
