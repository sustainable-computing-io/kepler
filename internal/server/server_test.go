// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockResponseWriter is a mock implementation of http.ResponseWriter
type MockResponseWriter struct {
	mock.Mock
	StatusCode int
	Headers    http.Header
	Body       []byte
}

func (m *MockResponseWriter) Header() http.Header {
	return m.Headers
}

func (m *MockResponseWriter) Write(b []byte) (int, error) {
	args := m.Called(b)
	m.Body = append(m.Body, b...)
	return args.Int(0), args.Error(1)
}

func (m *MockResponseWriter) WriteHeader(statusCode int) {
	m.StatusCode = statusCode
	m.Called(statusCode)
}

func TestNewAPIServer(t *testing.T) {
	tt := []struct {
		name        string
		opts        []OptionFn
		serviceName string
	}{{
		name:        "default options",
		opts:        []OptionFn{},
		serviceName: "api-server",
	}, {
		name: "with custom logger",
		opts: []OptionFn{
			WithLogger(slog.Default().With("test", "custom")),
		},
		serviceName: "api-server",
	}, {
		name: "with custom listen address",
		opts: []OptionFn{
			WithListenAddress([]string{":8080", ":8081"}),
		},
		serviceName: "api-server",
	}, {
		name: "with multiple options",
		opts: []OptionFn{
			WithLogger(slog.Default().With("test", "custom")),
			WithListenAddress([]string{":9090"}),
		},
		serviceName: "api-server",
	}}

	for _, tt := range tt {
		t.Run(tt.name, func(t *testing.T) {
			server := NewAPIServer(tt.opts...)

			assert.NotNil(t, server)
			assert.Equal(t, tt.serviceName, server.Name())
			assert.NotNil(t, server.mux)
			assert.NotNil(t, server.logger)
		})
	}
	// check listen address
	{
		server := NewAPIServer(
			WithListenAddress([]string{":8080", ":8081"}),
		)

		assert.NotNil(t, server)
		assert.Equal(t, []string{":8080", ":8081"}, server.listenAddrs)
	}
}

func TestAPIServer_Init(t *testing.T) {
	server := NewAPIServer()

	err := server.Init()
	assert.NoError(t, err)
}

func TestAPIServer_Run(t *testing.T) {
	server := NewAPIServer()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	startTime := time.Now()
	err := server.Run(ctx)
	duration := time.Since(startTime)

	assert.NoError(t, err)
	assert.GreaterOrEqual(t, duration, 50*time.Millisecond,
		"Run should block until context is done")
}

func TestAPIServer_Shutdown(t *testing.T) {
	server := NewAPIServer()

	err := server.Shutdown()
	assert.NoError(t, err)
}

func TestAPIServer_Register(t *testing.T) {
	t.Run("registers endpoints correctly", func(t *testing.T) {
		server := NewAPIServer()

		// Create a handler and register it
		testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
		err := server.Register("/test", "Test Endpoint", "A test endpoint", testHandler)
		require.NoError(t, err)

		// Verify endpoint description
		assert.Contains(t, server.endpointDescription, "/test")
		assert.Contains(t, server.endpointDescription, "Test Endpoint")
		assert.Contains(t, server.endpointDescription, "A test endpoint")

		// Verify handler was registered with mux
		muxHandler, pattern := server.mux.Handler(&http.Request{URL: &url.URL{Path: "/test"}})
		assert.Equal(t, "/test", pattern)
		assert.NotNil(t, muxHandler)
	})

	t.Run("registers multiple endpoints", func(t *testing.T) {
		server := NewAPIServer()

		handler1 := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
		handler2 := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

		err1 := server.Register("/endpoint1", "Endpoint 1", "First test endpoint", handler1)
		err2 := server.Register("/endpoint2", "Endpoint 2", "Second test endpoint", handler2)

		require.NoError(t, err1)
		require.NoError(t, err2)

		assert.Contains(t, server.endpointDescription, "/endpoint1")
		assert.Contains(t, server.endpointDescription, "/endpoint2")

		// Verify both handlers were registered
		_, pattern1 := server.mux.Handler(&http.Request{URL: &url.URL{Path: "/endpoint1"}})
		_, pattern2 := server.mux.Handler(&http.Request{URL: &url.URL{Path: "/endpoint2"}})

		assert.Equal(t, "/endpoint1", pattern1)
		assert.Equal(t, "/endpoint2", pattern2)
	})
}

func TestAPIServer_InitWithNoListenAddr(t *testing.T) {
	server := NewAPIServer(WithListenAddress([]string{}))
	err := server.Init()
	assert.Error(t, err, "Init should fail with no listen address")
	assert.Contains(t, err.Error(), "no listening address provided")
}

func TestAPIServer_InitWithContextCancellation(t *testing.T) {
	server := NewAPIServer()

	// NOTE: create a context and cancel it immediately
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Init should **NOT** return an error if context is cancelled
	err := server.Run(ctx)
	assert.NoError(t, err)
}

// TestAPIServer_EndToEnd tests the server with HTTP mock
func TestAPIServer_EndToEnd(t *testing.T) {
	// Create a test server with a mock handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte("test response"))
		require.NoError(t, err)
	})

	mockWriter := &MockResponseWriter{
		Headers: make(http.Header),
	}
	mockWriter.On("WriteHeader", http.StatusOK).Return()
	mockWriter.On("Write", []byte("test response")).Return(12, nil)

	req, err := http.NewRequest(http.MethodGet, "/test-endpoint", nil)
	require.NoError(t, err)

	server := NewAPIServer()
	err = server.Register("/test-endpoint", "Test", "Test endpoint", testHandler)
	require.NoError(t, err)

	// manually call the handler
	server.mux.ServeHTTP(mockWriter, req)

	assert.Equal(t, http.StatusOK, mockWriter.StatusCode)
	assert.Equal(t, []byte("test response"), mockWriter.Body)
	mockWriter.AssertExpectations(t)
}

// TestAPIServer_PortConflict tests that the API server correctly fails to start
// when another server is already listening on the same port
func TestAPIServer_PortConflict(t *testing.T) {
	port := findFreePort()
	addr := fmt.Sprintf(":%d", port)

	// Init a HTTP server that listens on the same port as the API server
	blockingServer := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	}

	listener, err := net.Listen("tcp", addr)
	require.NoError(t, err, "Failed to create listener for blocking server")

	go func() {
		_ = blockingServer.Serve(listener)
	}()

	// cleanup on test completion
	t.Cleanup(func() {
		// Use a short timeout for shutdown
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		_ = blockingServer.Shutdown(ctx)
		_ = listener.Close()
	})

	// Create our API server with the same port
	apiServer := NewAPIServer(WithListenAddress([]string{addr}))

	// Initing the API server on the same port should fail due to port conflict
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	err = apiServer.Run(ctx)
	assert.Error(t, err, "API server should fail to start due to port conflict")

	// The error should contain a specific message indicating port is in use: from log output
	// nc -l -p 2828i2; ./bin/kepler produces
	// ... level=ERROR source=internal/server/server.go:113
	// ... msg="HTTP server returned an error"
	// ... service=api-server
	// ... error="listen tcp :28282: bind: address already in use"
	assert.Contains(t, err.Error(), "in use", "Error should indicate port is already in use")
}

func findFreePort() int {
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		panic(err)
	}
	defer func() {
		// ignore error
		_ = l.Close()
	}()
	return l.Addr().(*net.TCPAddr).Port
}

func TestAPIServer_RootEndpoint(t *testing.T) {
	port := findFreePort()

	addr := fmt.Sprintf("127.0.0.1:%d", port)

	server := NewAPIServer(WithListenAddress([]string{addr}))
	assert.NoError(t, server.Init())

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	err := server.Register("/api/test", "Test API", "Test API endpoint", testHandler)
	require.NoError(t, err)

	// 3. Init the server with a timeout
	// ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)

	errCh := make(chan error, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	go func() {
		errCh <- server.Run(ctx)
	}()

	time.Sleep(300 * time.Millisecond)

	// 4. Make a request to "/"
	client := &http.Client{Timeout: 500 * time.Millisecond}
	resp, err := client.Get(fmt.Sprintf("http://%s/", addr))
	require.NoError(t, err, "HTTP request to root endpoint failed")
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "text/html; charset=utf-8", resp.Header.Get("Content-Type"))

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	// Verify the HTML contains our registered endpoint
	htmlContent := string(body)
	assert.Contains(t, htmlContent, "/api/test")
	assert.Contains(t, htmlContent, "Test API")
	assert.Contains(t, htmlContent, "Test API endpoint")

	// Verify basic HTML structure
	assert.Contains(t, htmlContent, "<html>")
	assert.Contains(t, htmlContent, "<h1>Kepler Service</h1>")
	assert.Contains(t, htmlContent, "<ul>")
	assert.Contains(t, htmlContent, "</ul>")
	assert.Contains(t, htmlContent, "</html>")

	// Wait for server to stop after context timeout
	select {
	case err := <-errCh:
		assert.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("Server didn't shut down within expected timeframe")
	}
}
