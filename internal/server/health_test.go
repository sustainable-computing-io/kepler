// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

// Mock implementations for testing
type mockLiveChecker struct {
	alive bool
	err   error
}

func (m *mockLiveChecker) IsLive(ctx context.Context) (bool, error) {
	return m.alive, m.err
}

type mockReadyChecker struct {
	ready bool
	err   error
}

func (m *mockReadyChecker) IsReady(ctx context.Context) (bool, error) {
	return m.ready, m.err
}

type mockAPIServer struct {
	handlers map[string]http.Handler
}

func (m *mockAPIServer) Name() string {
	return "mock-api-server"
}

func (m *mockAPIServer) Register(endpoint, summary, description string, handler http.Handler) error {
	if m.handlers == nil {
		m.handlers = make(map[string]http.Handler)
	}
	m.handlers[endpoint] = handler
	return nil
}

func TestHealthProbeService_Init(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	apiServer := &mockAPIServer{}
	liveChecker := &mockLiveChecker{alive: true}
	readyChecker := &mockReadyChecker{ready: true}

	healthService := NewHealthProbeService(apiServer, liveChecker, readyChecker, logger)

	err := healthService.Init()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Check that endpoints were registered
	if len(apiServer.handlers) != 2 {
		t.Fatalf("Expected 2 handlers registered, got %d", len(apiServer.handlers))
	}

	if _, exists := apiServer.handlers["/probe/livez"]; !exists {
		t.Error("Liveness probe handler not registered")
	}

	if _, exists := apiServer.handlers["/probe/readyz"]; !exists {
		t.Error("Readiness probe handler not registered")
	}
}

func TestLivenessHandler_Success(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	apiServer := &mockAPIServer{}
	liveChecker := &mockLiveChecker{alive: true}
	readyChecker := &mockReadyChecker{ready: true}

	healthService := NewHealthProbeService(apiServer, liveChecker, readyChecker, logger)
	err := healthService.Init()
	if err != nil {
		t.Fatalf("Failed to initialize health service: %v", err)
	}

	handler := apiServer.handlers["/probe/livez"]
	req := httptest.NewRequest("GET", "/probe/livez", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	if contentType := w.Header().Get("Content-Type"); contentType != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", contentType)
	}
}

func TestLivenessHandler_Failure(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	apiServer := &mockAPIServer{}
	liveChecker := &mockLiveChecker{alive: false, err: errors.New("service is down")}
	readyChecker := &mockReadyChecker{ready: true}

	healthService := NewHealthProbeService(apiServer, liveChecker, readyChecker, logger)
	err := healthService.Init()
	if err != nil {
		t.Fatalf("Failed to initialize health service: %v", err)
	}

	handler := apiServer.handlers["/probe/livez"]
	req := httptest.NewRequest("GET", "/probe/livez", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, w.Code)
	}
}

func TestReadinessHandler_Success(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	apiServer := &mockAPIServer{}
	liveChecker := &mockLiveChecker{alive: true}
	readyChecker := &mockReadyChecker{ready: true}

	healthService := NewHealthProbeService(apiServer, liveChecker, readyChecker, logger)
	err := healthService.Init()
	if err != nil {
		t.Fatalf("Failed to initialize health service: %v", err)
	}

	handler := apiServer.handlers["/probe/readyz"]
	req := httptest.NewRequest("GET", "/probe/readyz", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	if contentType := w.Header().Get("Content-Type"); contentType != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", contentType)
	}
}

func TestReadinessHandler_Failure(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	apiServer := &mockAPIServer{}
	liveChecker := &mockLiveChecker{alive: true}
	readyChecker := &mockReadyChecker{ready: false, err: errors.New("service not ready")}

	healthService := NewHealthProbeService(apiServer, liveChecker, readyChecker, logger)
	err := healthService.Init()
	if err != nil {
		t.Fatalf("Failed to initialize health service: %v", err)
	}

	handler := apiServer.handlers["/probe/readyz"]
	req := httptest.NewRequest("GET", "/probe/readyz", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, w.Code)
	}
}