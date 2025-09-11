// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/sustainable-computing-io/kepler/internal/monitor"
)

// mockPowerDataProvider implements monitor.PowerDataProvider for testing
type mockPowerDataProvider struct {
	mock.Mock
}

func (m *mockPowerDataProvider) Snapshot() (*monitor.Snapshot, error) {
	args := m.Called()
	snapshot := args.Get(0)
	if snapshot == nil {
		return nil, args.Error(1)
	}
	return snapshot.(*monitor.Snapshot), args.Error(1)
}


func (m *mockPowerDataProvider) DataChannel() <-chan struct{} {
	args := m.Called()
	return args.Get(0).(chan struct{})
}

func (m *mockPowerDataProvider) ZoneNames() []string {
	args := m.Called()
	return args.Get(0).([]string)
}

// mockAPIService implements APIService for testing
type mockAPIService struct {
	mock.Mock
	mux *http.ServeMux
}

func (m *mockAPIService) Name() string {
	return "mock-api"
}

func (m *mockAPIService) Register(endpoint, summary, description string, handler http.Handler) error {
	if m.mux == nil {
		m.mux = http.NewServeMux()
	}
	m.mux.Handle(endpoint, handler)
	return nil
}

func TestProbe_ReadyzHandler(t *testing.T) {
	tests := []struct {
		name             string
		snapshotReturn   *monitor.Snapshot
		snapshotError    error
		expectedStatus   int
		expectedResult   string
	}{
		{
			name:           "ready with valid snapshot",
			snapshotReturn: &monitor.Snapshot{Timestamp: time.Now()},
			snapshotError:  nil,
			expectedStatus: http.StatusOK,
			expectedResult: "ok",
		},
		{
			name:           "not ready - snapshot error",
			snapshotReturn: nil,
			snapshotError:  assert.AnError,
			expectedStatus: http.StatusServiceUnavailable,
			expectedResult: "not ready",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mocks
			mockAPI := &mockAPIService{}
			mockPowerProvider := &mockPowerDataProvider{}
			
			mockPowerProvider.On("Snapshot").Return(tt.snapshotReturn, tt.snapshotError)

			// Create probe service
			probe := NewProbe(mockAPI, mockPowerProvider)
			err := probe.Init()
			assert.NoError(t, err)

			// Create request
			req, err := http.NewRequest("GET", "/probe/readyz", nil)
			assert.NoError(t, err)

			// Create response recorder
			rr := httptest.NewRecorder()
			
			// Call handler
			mockAPI.mux.ServeHTTP(rr, req)

			// Check status code
			assert.Equal(t, tt.expectedStatus, rr.Code)

			// Parse response
			var response map[string]string
			err = json.Unmarshal(rr.Body.Bytes(), &response)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedResult, response["status"])

			mockPowerProvider.AssertExpectations(t)
		})
	}
}

func TestProbe_LivezHandler(t *testing.T) {
	tests := []struct {
		name             string
		snapshotReturn   *monitor.Snapshot
		snapshotError    error
		expectedStatus   int
		expectedResult   string
	}{
		{
			name:           "alive with valid snapshot",
			snapshotReturn: &monitor.Snapshot{Timestamp: time.Now()},
			snapshotError:  nil,
			expectedStatus: http.StatusOK,
			expectedResult: "alive",
		},
		{
			name:           "not alive - snapshot error",
			snapshotReturn: nil,
			snapshotError:  assert.AnError,
			expectedStatus: http.StatusServiceUnavailable,
			expectedResult: "not alive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mocks
			mockAPI := &mockAPIService{}
			mockPowerProvider := &mockPowerDataProvider{}
			
			mockPowerProvider.On("Snapshot").Return(tt.snapshotReturn, tt.snapshotError)

			// Create probe service
			probe := NewProbe(mockAPI, mockPowerProvider)
			err := probe.Init()
			assert.NoError(t, err)

			// Create request
			req, err := http.NewRequest("GET", "/probe/livez", nil)
			assert.NoError(t, err)

			// Create response recorder
			rr := httptest.NewRecorder()
			
			// Call handler
			mockAPI.mux.ServeHTTP(rr, req)

			// Check status code
			assert.Equal(t, tt.expectedStatus, rr.Code)

			// Parse response
			var response map[string]string
			err = json.Unmarshal(rr.Body.Bytes(), &response)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedResult, response["status"])

			mockPowerProvider.AssertExpectations(t)
		})
	}
}

func TestProbe_MethodNotAllowed(t *testing.T) {
	// Setup mocks
	mockAPI := &mockAPIService{}
	mockPowerProvider := &mockPowerDataProvider{}

	// Create probe service
	probe := NewProbe(mockAPI, mockPowerProvider)
	err := probe.Init()
	assert.NoError(t, err)

	endpoints := []string{"/probe/readyz", "/probe/livez"}
	
	for _, endpoint := range endpoints {
		t.Run("POST "+endpoint, func(t *testing.T) {
			req, err := http.NewRequest("POST", endpoint, nil)
			assert.NoError(t, err)

			rr := httptest.NewRecorder()
			mockAPI.mux.ServeHTTP(rr, req)

			assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
		})
	}
}