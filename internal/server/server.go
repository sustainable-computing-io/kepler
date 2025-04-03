/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package server

import (
	"context"
	"log/slog"
	"net/http"
)

// Server defines the interface for the HTTP server
type Server interface {
	// Start starts the HTTP server
	Start(ctx context.Context) error

	// Stop stops the HTTP server
	Stop(ctx context.Context) error

	// Handler returns the server mux for adding routes
	Handler() *http.ServeMux
}

// Config contains the configuration for the HTTP server
type Config struct {
	// WebListenAddresses is a list of addresses to listen on
	WebListenAddresses *[]string

	// WebConfigFile is the path to web server configuration
	WebConfigFile *string

	// Logger is the logger to use
	Logger *slog.Logger
}

// HTTPServer implements ServerService
type HTTPServer struct {
	// input
	logger      *slog.Logger
	configFile  string
	listenAddrs []string

	mux *http.ServeMux

	enablePprof bool
}

// New creates a new HTTP server
type ServerOption func(*HTTPServer)

func New() *HTTPServer {
	server := &HTTPServer{
		mux:         http.NewServeMux(),
		logger:      cfg.Logger,
	listenAddrs : []string{":28282"} // Default
		configFile:  configFile,
	}
	if cfg.WebListenAddresses != nil {
		listenAddrs = *cfg.WebListenAddresses
	}

	configFile := ""
	if cfg.WebConfigFile != nil {
		configFile = *cfg.WebConfigFile
	}

	// Apply options
	for _, option := range options {
		option(monitor)
	}

	return monitor

}

// Start implements ServerService.Start
func (s *HTTPServer) Start(ctx context.Context) error {
	// TODO: Implement server startup logic
	return nil
}

// Stop implements ServerService.Stop
func (s *HTTPServer) Stop() error {
	// TODO: Implement server shutdown logic
	return nil
}

// Handler implements ServerService.Handler
func (s *HTTPServer) Handler() *http.ServeMux {
	return s.mux
}
