// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/prometheus/exporter-toolkit/web"
	"github.com/sustainable-computing-io/kepler/internal/service"
)

// APIService defines the interface for the HTTP server providing API endpoints
type APIService interface {
	service.Service
	Register(endpoint, summary, description string, handler http.Handler) error
}

// APIServer implements APIServer
type APIServer struct {
	// input
	logger      *slog.Logger
	listenAddrs []string

	// http
	server              *http.Server
	mux                 *http.ServeMux
	endpointDescription string
	webCfgPath          string
}

var _ APIService = (*APIServer)(nil)

type Opts struct {
	logger      *slog.Logger
	listenAddrs []string
	webCfgPath  string
}

// OptionFn is a function sets one more more options in Opts struct
type OptionFn func(*Opts)

// WithLogger sets the logger for the APIServer
func WithLogger(logger *slog.Logger) OptionFn {
	return func(o *Opts) {
		o.logger = logger
	}
}

// WithListenAddress sets the listening addresses for the APIServer
func WithListenAddress(addr []string) OptionFn {
	return func(o *Opts) {
		o.listenAddrs = addr
	}
}

func WithWebConfig(path string) OptionFn {
	return func(o *Opts) {
		o.webCfgPath = path
	}
}

// DefaultOpts returns the default options
func DefaultOpts() Opts {
	return Opts{
		logger:      slog.Default(),
		listenAddrs: []string{":28282"}, // Default HTTP Port
		webCfgPath:  "",                 // Not present by default
	}
}

// NewAPIServer creates a new HTTPAPIServer instance
func NewAPIServer(applyOpts ...OptionFn) *APIServer {
	opts := DefaultOpts()
	for _, apply := range applyOpts {
		apply(&opts)
	}

	mux := http.NewServeMux()
	server := &http.Server{
		Handler: mux,
	}
	apiServer := &APIServer{
		logger:      opts.logger.With("service", "api-server"),
		listenAddrs: opts.listenAddrs,
		mux:         mux,
		server:      server,
		webCfgPath:  opts.webCfgPath,
	}

	return apiServer
}

func (s *APIServer) Name() string {
	return "api-server"
}

func (s *APIServer) Init() error {
	s.logger.Info("Initializing HTTP server", "listening-on", s.listenAddrs)
	if len(s.listenAddrs) == 0 {
		return fmt.Errorf("no listening address provided")
	}

	// create landing page that shows all available endpoints
	s.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Only respond to the root path
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, err := w.Write(fmt.Appendf([]byte{}, `<html>
<head><title>Kepler</title></head>
<body>
<h1>Kepler Service</h1>
<p>Available endpoints:</p>
<ul>
	%s
</ul>
</body>
</html>`,
			s.endpointDescription))
		if err != nil {
			s.logger.Error("failed to write landing page", "error", err)
		}
	})

	return nil
}

func (s *APIServer) Run(ctx context.Context) error {
	s.logger.Info("Running HTTP server", "listening-on", s.listenAddrs)
	errCh := make(chan error)
	go func() {
		webCfg := &web.FlagConfig{
			WebListenAddresses: &s.listenAddrs,
			WebConfigFile:      &s.webCfgPath,
		}
		errCh <- web.ListenAndServe(s.server, webCfg, s.logger)
	}()

	select {
	case <-ctx.Done():
		s.logger.Info("shutting down HTTP server on context done")
		return nil

	case err := <-errCh:
		s.logger.Error("HTTP server returned an error", "error", err)
		return err
	}
}

func (s *APIServer) Shutdown() error {
	s.logger.Info("shutting down API server on request")

	// NOTE: ensure http server shuts down within 5 seconds
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.server.Shutdown(ctx)
}

func (s *APIServer) Register(endpoint, summary, description string, handler http.Handler) error {
	s.logger.Debug("Endpoint Registered", "endpoint", endpoint)
	s.mux.Handle(endpoint, handler)
	s.endpointDescription += fmt.Sprintf("<li> <a href=\"%s\"> %s </a> %s </li>\n", endpoint, summary, description)
	return nil
}
