// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/sustainable-computing-io/kepler/internal/monitor"
	"github.com/sustainable-computing-io/kepler/internal/service"
)

type (
	Initializer       = service.Initializer
	Runner            = service.Runner
	PowerDataProvider = monitor.PowerDataProvider
	APIRegistry       = interface {
		Register(endpoint, summary, description string, handler http.Handler) error
	}
)

// Server implements MCP server functionality for Kepler
type Server struct {
	logger      *slog.Logger
	monitor     PowerDataProvider
	server      *mcp.Server
	apiRegistry APIRegistry

	// Configuration
	useHTTP    bool
	httpPath   string
	transport  string // "stdio", "sse", "streamable"
}

var (
	_ Initializer = (*Server)(nil)
	_ Runner      = (*Server)(nil)
)

// Option defines functional options for MCP server configuration
type Option func(*Server)

// WithHTTPTransport enables HTTP transport (SSE by default)
func WithHTTPTransport(apiRegistry APIRegistry, path string) Option {
	return func(s *Server) {
		s.useHTTP = true
		s.apiRegistry = apiRegistry
		s.httpPath = path
		s.transport = "sse" // default to SSE
	}
}

// WithStreamableHTTP enables streamable HTTP transport
func WithStreamableHTTP(apiRegistry APIRegistry, path string) Option {
	return func(s *Server) {
		s.useHTTP = true
		s.apiRegistry = apiRegistry
		s.httpPath = path
		s.transport = "streamable"
	}
}

// WithSSETransport enables Server-Sent Events transport (default HTTP transport)
func WithSSETransport(apiRegistry APIRegistry, path string) Option {
	return func(s *Server) {
		s.useHTTP = true
		s.apiRegistry = apiRegistry
		s.httpPath = path
		s.transport = "sse"
	}
}

// NewServer creates a new MCP server instance
func NewServer(monitor PowerDataProvider, logger *slog.Logger, options ...Option) *Server {
	mcpServer := mcp.NewServer(&mcp.Implementation{
		Name:    "kepler",
		Version: "v1.0.0",
	}, nil)

	server := &Server{
		logger:    logger.With("service", "mcp"),
		monitor:   monitor,
		server:    mcpServer,
		useHTTP:   false,
		httpPath:  "/mcp",
		transport: "stdio",
	}

	// Apply options
	for _, option := range options {
		option(server)
	}

	server.registerTools()

	return server
}

// registerTools registers all MCP tools with the server
func (s *Server) registerTools() {
	s.logger.Debug("Registering MCP tools")

	// Register list_top_consumers tool
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "list_top_consumers",
		Description: "List the top power consumers by resource type",
	}, s.handleListTopConsumers)

	// Register get_resource_power tool
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "get_resource_power",
		Description: "Get detailed power data for a specific resource",
	}, s.handleGetResourcePower)

	// Register search_resources tool
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "search_resources",
		Description: "Search for resources matching specific criteria",
	}, s.handleSearchResources)
}

// Init implements the Initializer interface
func (s *Server) Init() error {
	s.logger.Info("Initializing MCP server",
		"transport", s.transport,
		"http_enabled", s.useHTTP,
		"http_path", s.httpPath)

	// Register HTTP handler if using HTTP transport
	if s.useHTTP && s.apiRegistry != nil {
		var handler http.Handler

		switch s.transport {
		case "sse":
			// Create SSE handler that returns our server for any request
			handler = mcp.NewSSEHandler(func(req *http.Request) *mcp.Server {
				return s.server
			})
		case "streamable":
			// Create streamable HTTP handler
			handler = mcp.NewStreamableHTTPHandler(func(req *http.Request) *mcp.Server {
				return s.server
			}, nil)
		default:
			s.logger.Warn("Unknown HTTP transport type, defaulting to SSE", "transport", s.transport)
			handler = mcp.NewSSEHandler(func(req *http.Request) *mcp.Server {
				return s.server
			})
		}

		// Register the MCP handler with the API server
		err := s.apiRegistry.Register(
			s.httpPath,
			"MCP Server",
			"Model Context Protocol server for querying power consumption data",
			handler,
		)
		if err != nil {
			return err
		}

		s.logger.Info("Registered MCP HTTP handler", "path", s.httpPath, "transport", s.transport)
	}

	return nil
}

// Name implements the Service interface
func (s *Server) Name() string {
	return "mcp"
}

// Run starts the MCP server
func (s *Server) Run(ctx context.Context) error {
	s.logger.Info("Starting MCP server", "transport", s.transport, "http_enabled", s.useHTTP)

	// For HTTP transport, the server runs via the registered HTTP handler
	// so we just need to keep this service alive
	if s.useHTTP {
		s.logger.Info("MCP server running via HTTP transport", "path", s.httpPath)
		// Wait for context cancellation
		<-ctx.Done()
		return ctx.Err()
	}

	// Use stdio transport for non-HTTP mode
	s.logger.Info("MCP server starting with stdio transport")
	transport := mcp.NewStdioTransport()

	return s.server.Run(ctx, transport)
}
