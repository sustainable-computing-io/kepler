// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package redfish

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/stmcginnis/gofish"
	"github.com/sustainable-computing-io/kepler/config"
	"github.com/sustainable-computing-io/kepler/config/redfish"
	"github.com/sustainable-computing-io/kepler/internal/service"
)

// Service implements the Redfish power monitoring service
type Service struct {
	logger *slog.Logger
	bmc    *redfish.BMCDetail // Store BMC configuration
	client *gofish.APIClient  // Direct gofish client

	powerReader *PowerReader
	nodeName    string
	bmcID       string // Store BMC ID for metrics

	staleness   time.Duration // Max age before forcing new collection
	httpTimeout time.Duration // HTTP client timeout for BMC requests

	// Simplified caching for staleness support
	mu            sync.RWMutex  // Protects cached readings
	cachedReading *PowerReading // Last reading from all chassis
}

// Ensure Service implements the required interfaces
var (
	_ service.Initializer = (*Service)(nil)
	_ service.Shutdowner  = (*Service)(nil) // To logout
)

// OptionFn is a functional option for configuring the Redfish service
type OptionFn func(*Service)

// WithStaleness sets the staleness duration for cached power readings
func WithStaleness(staleness time.Duration) OptionFn {
	return func(s *Service) {
		s.staleness = staleness
	}
}

// NewService creates a new Redfish service
func NewService(cfg config.Redfish, logger *slog.Logger, opts ...OptionFn) (*Service, error) {
	// Log experimental feature warning
	logger = logger.With(slog.String("service", "experimental.redfish"))
	logger.Warn("Using EXPERIMENTAL Redfish power monitoring feature", "feature", "redfish")

	// NodeName is already resolved in config processing
	nodeName := cfg.NodeName
	if nodeName == "" {
		return nil, fmt.Errorf("NodeName is empty - ensure Redfish is enabled and configured properly")
	}

	logger.Info("Using resolved node name", "node_name", nodeName)

	// Load BMC configuration using redfishCfg.ConfigFile
	bmcCfg, err := redfish.Load(cfg.ConfigFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load BMC configuration: %w", err)
	}

	// Get BMC details and ID for this node
	bmcDetail, err := bmcCfg.BMCForNode(nodeName)
	if err != nil {
		return nil, fmt.Errorf("failed to get BMC configuration for node %s: %w", nodeName, err)
	}

	bmcID, err := bmcCfg.BMCIDForNode(nodeName)
	if err != nil {
		return nil, fmt.Errorf("failed to get BMC ID for node %s: %w", nodeName, err)
	}

	logger.Info("BMC configuration loaded", "node_name", nodeName, "bmc_id", bmcID, "endpoint", bmcDetail.Endpoint)

	// Create power reader (will be initialized in Init())
	reader := NewPowerReader(logger)

	service := &Service{
		logger:      logger,
		bmc:         bmcDetail,
		powerReader: reader,
		nodeName:    nodeName,
		bmcID:       bmcID,
		staleness:   500 * time.Millisecond, // Default staleness
		httpTimeout: cfg.HTTPTimeout,
		// Initialize cache fields
		cachedReading: nil,
	}

	// Apply functional options
	for _, opt := range opts {
		opt(service)
	}

	return service, nil
}

// Name returns the service name
func (s *Service) Name() string {
	return "platform.redfish"
}

// Init initializes the service by connecting to the BMC
func (s *Service) Init() error {
	s.logger.Info("Initializing Redfish power monitoring service",
		"node_name", s.nodeName,
		"bmc_endpoint", s.bmc.Endpoint)

	// Configure HTTP client with timeout and TLS configuration
	httpClient := &http.Client{
		Timeout: s.httpTimeout,
	}

	if s.bmc.Insecure {
		httpClient.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
	}

	//  Gofish client
	gofishConfig := gofish.ClientConfig{
		Endpoint:   s.bmc.Endpoint,
		Username:   s.bmc.Username,
		Password:   s.bmc.Password,
		HTTPClient: httpClient,
	}

	// NOTE: Use Background() for client connection since gofish stores this context
	// and uses it for all subsequent HTTP requests. A timeout context causes
	// "context canceled" errors on later requests when the timeout expires.
	client, err := gofish.ConnectContext(context.Background(), gofishConfig)
	if err != nil {
		// Don't log credentials in error messages
		return fmt.Errorf("failed to connect to BMC at %s for node %s: %w", s.bmc.Endpoint, s.nodeName, err)
	}

	s.client = client

	// Initialize power reader with the connected client
	s.powerReader.SetClient(client)

	// NOTE: Do not validate power reading capability during Init()
	// to allow the service to start even if power data is temporarily unavailable.
	// Power reading errors will be handled during actual data collection.

	s.logger.Info("Successfully connected to BMC", "node_name", s.nodeName)
	return nil
}

// Shutdown cleanly shuts down the service
func (s *Service) Shutdown() error {
	s.logger.Info("Shutting down Redfish power monitoring service")
	defer s.logger.Info("Redfish power monitoring service shutdown complete")

	// Disconnect gofish client if connected
	if s.client == nil {
		return nil
	}
	s.client.Logout()
	s.client = nil

	return nil
}

// NodeName returns the node name
func (s *Service) NodeName() string {
	return s.nodeName
}

// BMCID returns the BMC identifier
func (s *Service) BMCID() string {
	return s.bmcID
}

// isFresh checks if the cached reading is still within the staleness threshold
func (s *Service) isFresh() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.cachedReading == nil || s.cachedReading.Timestamp.IsZero() {
		return false
	}

	age := time.Since(s.cachedReading.Timestamp)
	return age <= s.staleness
}

// Power returns power readings from all chassis with power data
func (s *Service) Power() (*PowerReading, error) {
	if s.powerReader == nil {
		return nil, fmt.Errorf("power reader is not initialized")
	}

	// Check if we have fresh cached data
	if s.isFresh() {
		s.mu.RLock()
		cached := s.cachedReading.Clone()
		cacheAge := time.Since(s.cachedReading.Timestamp)
		s.mu.RUnlock()

		s.logger.Debug("Returning cached chassis power readings",
			"chassis.count", len(cached.Chassis),
			"cache.age", cacheAge,
			"staleness", s.staleness)
		return cached, nil
	}

	// Need fresh data - collect from BMC
	readings, err := s.powerReader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to collect power data from BMC: %w", err)
	}

	// Assemble PowerReading with timestamp
	newReading := &PowerReading{
		Timestamp: time.Now(),
		Chassis:   readings,
	}

	// Update the cache with the new reading
	s.mu.Lock()
	s.cachedReading = newReading.Clone() // Clone for safe storage
	s.mu.Unlock()

	s.logger.Debug("Collected and cached fresh chassis power readings",
		"chassis.count", len(newReading.Chassis),
		"staleness", s.staleness)

	return newReading, nil
}
