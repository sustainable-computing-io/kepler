// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockAPIService is an implementation of the APIService interface for testing.
type MockAPIService struct {
	mock.Mock
}

func (m *MockAPIService) Register(path, name, description string, handler http.Handler) error {
	args := m.Called(path, name, description, handler)
	return args.Error(0)
}

func (m *MockAPIService) Name() string {
	return "mockApiService"
}

// TestNewPprof tests the NewPprof constructor.
func TestNewPprof(t *testing.T) {
	api := &MockAPIService{}
	p := NewPprof(api)

	assert.NotNil(t, p, "NewPprof should return a non-nil pointer")
	assert.Equal(t, api, p.api, "NewPprof should set the api field correctly")
}

// TestPprofName tests the Name method.
func TestPprofName(t *testing.T) {
	api := &MockAPIService{}
	p := NewPprof(api)

	name := p.Name()
	assert.Equal(t, "pprof", name, "Name should return 'pprof'")
}

// TestPprofInit_Success tests the Init method when the API registration succeeds.
func TestPprofInit_Success(t *testing.T) {
	api := &MockAPIService{}
	p := NewPprof(api)

	// Set up mock expectation
	api.On("Register", "/debug/pprof/", "pprof", "Profiling Data", mock.AnythingOfType("*http.ServeMux")).Return(nil)

	err := p.Init()
	assert.NoError(t, err, "Init should not return an error when registration succeeds")
	api.AssertExpectations(t)
}

// TestPprofInit_Failure tests the Init method when the API registration fails.
func TestPprofInit_Failure(t *testing.T) {
	api := &MockAPIService{}
	p := NewPprof(api)

	// Set up mock expectation
	expectedErr := assert.AnError
	api.On("Register", "/debug/pprof/", "pprof", "Profiling Data", mock.AnythingOfType("*http.ServeMux")).Return(expectedErr)

	err := p.Init()
	assert.Error(t, err, "Init should return an error when registration fails")
	assert.Equal(t, expectedErr, err, "Init should return the expected error")
	api.AssertExpectations(t)
}

// TestPprofHandlers tests the handlers function to ensure it registers the correct pprof endpoints.
func TestPprofHandlers(t *testing.T) {
	handler := handlers()
	mux, ok := handler.(*http.ServeMux)
	assert.True(t, ok, "handlers should return an http.ServeMux")

	// Test cases for each pprof endpoint
	tests := []struct {
		path string
	}{
		{"/debug/pprof/"},
		{"/debug/pprof/cmdline"},
		{"/debug/pprof/profile?seconds=1"},
		{"/debug/pprof/symbol"},
		{"/debug/pprof/trace"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			rr := httptest.NewRecorder()
			mux.ServeHTTP(rr, req)
			assert.NotEqual(t, http.StatusNotFound, rr.Code, "Handler for %s should be registered", tt.path)
		})
	}
}
