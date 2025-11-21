// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/sustainable-computing-io/kepler/internal/service"
)

// mockService implements service.Service, service.LiveChecker, and service.ReadyChecker
type mockService struct {
	name  string
	live  bool
	ready bool
}

func (m *mockService) Name() string {
	return m.name
}

func (m *mockService) IsLive() bool {
	return m.live
}

func (m *mockService) IsReady() bool {
	return m.ready
}

// mockAPIServer implements APIService for testing
type mockAPIServer struct {
	handlers map[string]http.Handler
}

func newMockAPIServer() *mockAPIServer {
	return &mockAPIServer{
		handlers: make(map[string]http.Handler),
	}
}

func (m *mockAPIServer) Name() string {
	return "mock-api-server"
}

func (m *mockAPIServer) Register(endpoint, summary, description string, handler http.Handler) error {
	m.handlers[endpoint] = handler
	return nil
}

func (m *mockAPIServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if handler, ok := m.handlers[r.URL.Path]; ok {
		handler.ServeHTTP(w, r)
		return
	}
	http.NotFound(w, r)
}

func TestHealthProbe_Init(t *testing.T) {
	logger := slog.Default()
	apiServer := newMockAPIServer()
	services := []service.Service{
		&mockService{name: "test-service", live: true, ready: true},
	}

	hp := NewHealthProbe(apiServer, services, logger)
	err := hp.Init()

	require.NoError(t, err)
	assert.Contains(t, apiServer.handlers, "/probe/livez")
	assert.Contains(t, apiServer.handlers, "/probe/readyz")
}

func TestHealthProbe_Liveness_AllLive(t *testing.T) {
	logger := slog.Default()
	apiServer := newMockAPIServer()
	services := []service.Service{
		&mockService{name: "service1", live: true, ready: true},
		&mockService{name: "service2", live: true, ready: false},
	}

	hp := NewHealthProbe(apiServer, services, logger)
	require.NoError(t, hp.Init())

	req := httptest.NewRequest(http.MethodGet, "/probe/livez", nil)
	w := httptest.NewRecorder()
	apiServer.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var status HealthStatus
	err := json.NewDecoder(w.Body).Decode(&status)
	require.NoError(t, err)
	assert.Equal(t, "ok", status.Status)
	assert.Len(t, status.Services, 2)
	assert.True(t, status.Services[0].Live)
	assert.True(t, status.Services[1].Live)
}

func TestHealthProbe_Liveness_SomeUnhealthy(t *testing.T) {
	logger := slog.Default()
	apiServer := newMockAPIServer()
	services := []service.Service{
		&mockService{name: "service1", live: true, ready: true},
		&mockService{name: "service2", live: false, ready: false},
	}

	hp := NewHealthProbe(apiServer, services, logger)
	require.NoError(t, hp.Init())

	req := httptest.NewRequest(http.MethodGet, "/probe/livez", nil)
	w := httptest.NewRecorder()
	apiServer.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	var status HealthStatus
	err := json.NewDecoder(w.Body).Decode(&status)
	require.NoError(t, err)
	assert.Equal(t, "unhealthy", status.Status)
	assert.Len(t, status.Services, 2)
	assert.True(t, status.Services[0].Live)
	assert.False(t, status.Services[1].Live)
}

func TestHealthProbe_Readiness_AllReady(t *testing.T) {
	logger := slog.Default()
	apiServer := newMockAPIServer()
	services := []service.Service{
		&mockService{name: "service1", live: true, ready: true},
		&mockService{name: "service2", live: true, ready: true},
	}

	hp := NewHealthProbe(apiServer, services, logger)
	require.NoError(t, hp.Init())

	req := httptest.NewRequest(http.MethodGet, "/probe/readyz", nil)
	w := httptest.NewRecorder()
	apiServer.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var status HealthStatus
	err := json.NewDecoder(w.Body).Decode(&status)
	require.NoError(t, err)
	assert.Equal(t, "ok", status.Status)
	assert.Len(t, status.Services, 2)
	assert.True(t, status.Services[0].Ready)
	assert.True(t, status.Services[1].Ready)
}

func TestHealthProbe_Readiness_SomeNotReady(t *testing.T) {
	logger := slog.Default()
	apiServer := newMockAPIServer()
	services := []service.Service{
		&mockService{name: "service1", live: true, ready: true},
		&mockService{name: "service2", live: true, ready: false},
	}

	hp := NewHealthProbe(apiServer, services, logger)
	require.NoError(t, hp.Init())

	req := httptest.NewRequest(http.MethodGet, "/probe/readyz", nil)
	w := httptest.NewRecorder()
	apiServer.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	var status HealthStatus
	err := json.NewDecoder(w.Body).Decode(&status)
	require.NoError(t, err)
	assert.Equal(t, "unhealthy", status.Status)
	assert.Len(t, status.Services, 2)
	assert.True(t, status.Services[0].Ready)
	assert.False(t, status.Services[1].Ready)
}

// simpleService is a test service that doesn't implement any health checker interfaces
type simpleService struct {
	name string
}

func (s *simpleService) Name() string {
	return s.name
}

func TestHealthProbe_NoHealthCheckers(t *testing.T) {
	logger := slog.Default()
	apiServer := newMockAPIServer()

	// Service that doesn't implement any health checker interfaces
	svc := &simpleService{name: "simple"}
	services := []service.Service{svc}

	hp := NewHealthProbe(apiServer, services, logger)
	require.NoError(t, hp.Init())

	// Test liveness - should return ok with no services
	req := httptest.NewRequest(http.MethodGet, "/probe/livez", nil)
	w := httptest.NewRecorder()
	apiServer.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var status HealthStatus
	err := json.NewDecoder(w.Body).Decode(&status)
	require.NoError(t, err)
	assert.Equal(t, "ok", status.Status)
	assert.Len(t, status.Services, 0)
}

func TestHealthProbe_Run(t *testing.T) {
	logger := slog.Default()
	apiServer := newMockAPIServer()
	services := []service.Service{}

	hp := NewHealthProbe(apiServer, services, logger)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := hp.Run(ctx)
	assert.NoError(t, err)
}
