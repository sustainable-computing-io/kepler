// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"encoding/json"
	"net/http"

	"github.com/sustainable-computing-io/kepler/internal/monitor"
	"github.com/sustainable-computing-io/kepler/internal/service"
)

type probe struct {
	api          APIService
	powerMonitor monitor.PowerDataProvider
}

var (
	_ service.Service     = (*probe)(nil)
	_ service.Initializer = (*probe)(nil)
)

// NewProbe creates a new probe service that provides health check endpoints
func NewProbe(api APIService, powerMonitor monitor.PowerDataProvider) *probe {
	return &probe{
		api:          api,
		powerMonitor: powerMonitor,
	}
}

func (p *probe) Name() string {
	return "probe"
}

func (p *probe) Init() error {
	return p.api.Register("/probe/", "probe", "Health check endpoints", p.handlers())
}

// handlers returns HTTP handlers for health check endpoints
func (p *probe) handlers() http.Handler {
	mux := http.NewServeMux()

	// Register both health check endpoints
	p.registerHealthEndpoints(mux)

	return mux
}

// registerHealthEndpoints consolidates the registration of health check endpoints
func (p *probe) registerHealthEndpoints(mux *http.ServeMux) {
	// Readiness probe endpoint
	mux.HandleFunc("/probe/readyz", p.readyzHandler)
	
	// Liveness probe endpoint
	mux.HandleFunc("/probe/livez", p.livezHandler)
}

// readyzHandler handles readiness probe requests
// Returns 200 when all services are running, regardless of data collection status
func (p *probe) readyzHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// For readiness, we check if the monitor service is operational
	// This works even when collection interval is 0
	_, err := p.powerMonitor.Snapshot()
	if err != nil {
		p.respondWithError(w, "not ready", "monitor service not operational")
		return
	}

	p.respondWithSuccess(w, "ok")
}

// livezHandler handles liveness probe requests  
// Returns 200 if all services are running, regardless of sampling frequency
func (p *probe) livezHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// For liveness, we check if the monitor service is operational
	// This approach works even with interval=0
	_, err := p.powerMonitor.Snapshot()
	if err != nil {
		p.respondWithError(w, "not alive", "monitor service not operational")
		return
	}

	p.respondWithSuccess(w, "alive")
}

func (p *probe) respondWithSuccess(w http.ResponseWriter, status string) {
	response := map[string]string{
		"status": status,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func (p *probe) respondWithError(w http.ResponseWriter, status, reason string) {
	response := map[string]string{
		"status": status,
		"reason": reason,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusServiceUnavailable)
	
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}