// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/sustainable-computing-io/kepler/internal/service"
)

// HealthProbe provides health check endpoints for Kubernetes probes
type HealthProbe struct {
	logger    *slog.Logger
	apiServer APIService
	services  []service.Service
}

// ServiceHealth represents the health status of a single service
type ServiceHealth struct {
	Name  string `json:"name"`
	Live  bool   `json:"live,omitempty"`
	Ready bool   `json:"ready,omitempty"`
}

// HealthStatus represents the overall health status
type HealthStatus struct {
	Status   string          `json:"status"` // "ok" or "unhealthy"
	Services []ServiceHealth `json:"services,omitempty"`
}

// NewHealthProbe creates a new HealthProbe service
func NewHealthProbe(apiServer APIService, services []service.Service, logger *slog.Logger) *HealthProbe {
	return &HealthProbe{
		logger:    logger.With("service", "health-probe"),
		apiServer: apiServer,
		services:  services,
	}
}

func (h *HealthProbe) Name() string {
	return "health-probe"
}

func (h *HealthProbe) Init() error {
	h.logger.Info("Initializing health probe endpoints")

	// Register liveness probe endpoint
	if err := h.apiServer.Register(
		"/probe/livez",
		"Liveness Probe",
		"Returns 200 if all services are alive",
		http.HandlerFunc(h.handleLiveness),
	); err != nil {
		return err
	}

	// Register readiness probe endpoint
	if err := h.apiServer.Register(
		"/probe/readyz",
		"Readiness Probe",
		"Returns 200 if all services are ready",
		http.HandlerFunc(h.handleReadiness),
	); err != nil {
		return err
	}

	h.logger.Info("Health probe endpoints registered")
	return nil
}

func (h *HealthProbe) Run(ctx context.Context) error {
	// Health probe doesn't need to run in the background
	// It only responds to HTTP requests via the API server
	<-ctx.Done()
	return nil
}

// handleLiveness checks if all services implementing LiveChecker are alive
func (h *HealthProbe) handleLiveness(w http.ResponseWriter, r *http.Request) {
	status := HealthStatus{
		Status:   "ok",
		Services: make([]ServiceHealth, 0),
	}

	allLive := true
	for _, svc := range h.services {
		if liveChecker, ok := svc.(service.LiveChecker); ok {
			isLive := liveChecker.IsLive()
			status.Services = append(status.Services, ServiceHealth{
				Name: svc.Name(),
				Live: isLive,
			})
			if !isLive {
				allLive = false
			}
		}
	}

	if !allLive {
		status.Status = "unhealthy"
		h.writeJSONResponse(w, http.StatusServiceUnavailable, status)
		return
	}

	h.writeJSONResponse(w, http.StatusOK, status)
}

// handleReadiness checks if all services implementing ReadyChecker are ready
func (h *HealthProbe) handleReadiness(w http.ResponseWriter, r *http.Request) {
	status := HealthStatus{
		Status:   "ok",
		Services: make([]ServiceHealth, 0),
	}

	allReady := true
	for _, svc := range h.services {
		if readyChecker, ok := svc.(service.ReadyChecker); ok {
			isReady := readyChecker.IsReady()
			status.Services = append(status.Services, ServiceHealth{
				Name:  svc.Name(),
				Ready: isReady,
			})
			if !isReady {
				allReady = false
			}
		}
	}

	if !allReady {
		status.Status = "unhealthy"
		h.writeJSONResponse(w, http.StatusServiceUnavailable, status)
		return
	}

	h.writeJSONResponse(w, http.StatusOK, status)
}

// writeJSONResponse writes a JSON response with the given status code
func (h *HealthProbe) writeJSONResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.Error("failed to encode JSON response", "error", err)
	}
}
