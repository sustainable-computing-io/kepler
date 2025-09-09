// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/sustainable-computing-io/kepler/internal/monitor"
	"github.com/sustainable-computing-io/kepler/internal/service"
)

type probe struct {
	api          APIService
	powerMonitor monitor.PowerDataProvider
	maxStaleTime time.Duration
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
		maxStaleTime: 30 * time.Second, // Consider stale if no sample in 30s
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

	// Readiness probe endpoint
	mux.HandleFunc("/probe/readyz", p.readyzHandler)
	
	// Liveness probe endpoint
	mux.HandleFunc("/probe/livez", p.livezHandler)

	return mux
}

// readyzHandler handles readiness probe requests
// Returns 200 after critical init (sensors) and first successful sample/export; else 503
func (p *probe) readyzHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check if we have at least one successful sample (ultra-lightweight)
	lastCollection := p.powerMonitor.LastCollectionTime()
	if lastCollection.IsZero() {
		p.respondWithError(w, "not ready", "no successful sample yet")
		return
	}

	p.respondWithSuccess(w, "ok")
}

// livezHandler handles liveness probe requests  
// Returns 200 if the sampling loop ticked recently, 503 if stalled
func (p *probe) livezHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get the last collection time (ultra-lightweight)
	lastCollection := p.powerMonitor.LastCollectionTime()
	if lastCollection.IsZero() {
		p.respondWithError(w, "not alive", "no samples available")
		return
	}

	// Check if the sampling loop has ticked recently
	timeSinceLastSample := time.Since(lastCollection)
	if timeSinceLastSample > p.maxStaleTime {
		p.respondWithError(w, "not alive", "sampling loop stalled")
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