// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/sustainable-computing-io/kepler/internal/service"
)

// HealthProbeService provides Kubernetes health probe endpoints
type HealthProbeService struct {
	logger      *slog.Logger
	apiServer   APIService
	liveChecker service.LiveChecker
	readyChecker service.ReadyChecker
}

// NewHealthProbeService creates a new health probe service
func NewHealthProbeService(apiServer APIService, liveChecker service.LiveChecker, readyChecker service.ReadyChecker, logger *slog.Logger) *HealthProbeService {
	return &HealthProbeService{
		logger:       logger.With("service", "health-probe"),
		apiServer:    apiServer,
		liveChecker:  liveChecker,
		readyChecker: readyChecker,
	}
}

func (h *HealthProbeService) Name() string {
	return "health-probe"
}

func (h *HealthProbeService) Init() error {
	h.logger.Info("Initializing health probe endpoints")
	
	// Register liveness probe endpoint
	if err := h.apiServer.Register("/probe/livez", "Liveness Probe", "Kubernetes liveness probe endpoint", h.livenessHandler()); err != nil {
		return fmt.Errorf("failed to register liveness probe: %w", err)
	}
	
	// Register readiness probe endpoint
	if err := h.apiServer.Register("/probe/readyz", "Readiness Probe", "Kubernetes readiness probe endpoint", h.readinessHandler()); err != nil {
		return fmt.Errorf("failed to register readiness probe: %w", err)
	}
	
	h.logger.Info("Health probe endpoints registered successfully")
	return nil
}

// livenessHandler handles the liveness probe endpoint
func (h *HealthProbeService) livenessHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ctx := r.Context()
		
		// Set appropriate headers
		w.Header().Set("Content-Type", "application/json")
		
		alive, err := h.liveChecker.IsLive(ctx)
		duration := time.Since(start)
		
		response := map[string]any{
			"status":      "ok",
			"timestamp":   start.UTC().Format(time.RFC3339),
			"duration":    duration.String(),
		}
		
		if err != nil || !alive {
			w.WriteHeader(http.StatusServiceUnavailable)
			response["status"] = "error"
			if err != nil {
				response["error"] = err.Error()
			}
			h.logger.Error("Liveness check failed", "error", err, "duration", duration)
		} else {
			w.WriteHeader(http.StatusOK)
			h.logger.Debug("Liveness check passed", "duration", duration)
		}
		
		if err := json.NewEncoder(w).Encode(response); err != nil {
			h.logger.Error("Failed to encode liveness response", "error", err)
		}
	})
}

// readinessHandler handles the readiness probe endpoint
func (h *HealthProbeService) readinessHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ctx := r.Context()
		
		// Set appropriate headers
		w.Header().Set("Content-Type", "application/json")
		
		ready, err := h.readyChecker.IsReady(ctx)
		duration := time.Since(start)
		
		response := map[string]any{
			"status":      "ok",
			"timestamp":   start.UTC().Format(time.RFC3339),
			"duration":    duration.String(),
		}
		
		if err != nil || !ready {
			w.WriteHeader(http.StatusServiceUnavailable)
			response["status"] = "error"
			if err != nil {
				response["error"] = err.Error()
			}
			h.logger.Error("Readiness check failed", "error", err, "duration", duration)
		} else {
			w.WriteHeader(http.StatusOK)
			h.logger.Debug("Readiness check passed", "duration", duration)
		}
		
		if err := json.NewEncoder(w).Encode(response); err != nil {
			h.logger.Error("Failed to encode readiness response", "error", err)
		}
	})
}