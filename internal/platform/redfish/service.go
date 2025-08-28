// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package redfish

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/stmcginnis/gofish"
	"github.com/sustainable-computing-io/kepler/config"
	"github.com/sustainable-computing-io/kepler/config/redfish"
	"github.com/sustainable-computing-io/kepler/internal/service"
	"golang.org/x/sync/singleflight"
)

// Service implements the Redfish power monitoring service
type Service struct {
	logger *slog.Logger
	bmc    *redfish.BMCDetail // Store BMC configuration
	client *gofish.APIClient  // Direct gofish client

	powerReader *PowerReader
	nodeID      string
	bmcID       string // Store BMC ID for metrics

	// Collection configuration
	collection config.RedfishCollection

	// Data collection (following monitor pattern)
	lastReading atomic.Pointer[PowerReading] // Thread-safe reading storage

	// Singleflight for synchronized collection
	computeGroup singleflight.Group
}

// Ensure Service implements the required interfaces
var (
	_ service.Service     = (*Service)(nil)
	_ service.Initializer = (*Service)(nil)
	_ service.Runner      = (*Service)(nil)
	_ service.Shutdowner  = (*Service)(nil)
)

// NewService creates a new Redfish service
func NewService(configPath, nodeID string, redfishCfg config.Redfish, logger *slog.Logger) (*Service, error) {
	// Log experimental feature warning
	logger = logger.With(slog.String("service", "experimental.redfish"))
	logger.Warn("Using EXPERIMENTAL Redfish power monitoring feature", "feature", "redfish", "node_id", nodeID)

	// Load BMC configuration
	cfg, err := redfish.Load(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load BMC configuration: %w", err)
	}

	logger.Info("Resolved node identifier", "node_id", nodeID)

	// Get BMC details and ID for this node
	bmcDetail, err := cfg.BMCForNode(nodeID)
	if err != nil {
		return nil, fmt.Errorf("failed to get BMC configuration for node %s: %w", nodeID, err)
	}

	bmcID, err := cfg.BMCIDForNode(nodeID)
	if err != nil {
		return nil, fmt.Errorf("failed to get BMC ID for node %s: %w", nodeID, err)
	}

	logger.Info("BMC configuration loaded", "node_id", nodeID, "bmc_id", bmcID, "endpoint", bmcDetail.Endpoint)

	// Create power reader (will be initialized in Init())
	reader := NewPowerReader(logger)

	return &Service{
		logger:      logger,
		bmc:         bmcDetail,
		powerReader: reader,
		nodeID:      nodeID,
		bmcID:       bmcID,
		collection:  redfishCfg.Collection,
	}, nil
}

// Name returns the service name
func (s *Service) Name() string {
	return "platform.redfish"
}

// Init initializes the service by connecting to the BMC
func (s *Service) Init() error {
	s.logger.Info("Initializing Redfish power monitoring service",
		"node_id", s.nodeID,
		"bmc_endpoint", s.bmc.Endpoint)

	// Validate credentials - if one is provided, both must be provided
	if (s.bmc.Username == "" && s.bmc.Password != "") ||
		(s.bmc.Username != "" && s.bmc.Password == "") {
		return fmt.Errorf("both username and password must be provided for authentication or none")
	}

	// Create HTTP client with timeout and TLS configuration
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Configure TLS settings if insecure flag is set
	if s.bmc.Insecure {
		httpClient.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
	}

	// Configure Gofish client
	gofishConfig := gofish.ClientConfig{
		Endpoint:   s.bmc.Endpoint,
		Username:   s.bmc.Username,
		Password:   s.bmc.Password,
		HTTPClient: httpClient,
	}

	// Use context.Background() for client connection since gofish stores this context
	// and uses it for all subsequent HTTP requests. A timeout context would cause
	// "context canceled" errors on later requests when the timeout expires.
	client, err := gofish.ConnectContext(context.Background(), gofishConfig)
	if err != nil {
		// Don't log credentials in error messages
		return fmt.Errorf("failed to connect to BMC at %s for node %s: %w", s.bmc.Endpoint, s.nodeID, err)
	}

	s.client = client

	// Initialize power reader with the connected client
	s.powerReader.SetClient(client)

	// Note: We don't validate power reading capability during Init()
	// to allow the service to start even if power data is temporarily unavailable.
	// Power reading errors will be handled during actual data collection.

	s.logger.Info("Successfully connected to BMC", "node_id", s.nodeID)
	return nil
}

// Run starts the power monitoring loop with hybrid collection mode
func (s *Service) Run(ctx context.Context) error {
	if s.collection.Interval == 0 {
		// On-demand only mode: no periodic collection
		s.logger.Info("Starting Redfish power monitoring in on-demand only mode", "node_id", s.nodeID)

		// Wait for shutdown signal
		<-ctx.Done()
		s.logger.Info("Redfish power monitoring stopped due to context cancellation")
		return ctx.Err()
	}

	// Periodic collection mode
	s.logger.Info("Starting Redfish power monitoring with periodic collection",
		"node_id", s.nodeID,
		"interval", s.collection.Interval,
		"staleness", s.collection.Staleness)

	ticker := time.NewTicker(s.collection.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("Redfish power monitoring stopped due to context cancellation")
			return ctx.Err()
		case <-ticker.C:
			if err := s.collectPowerData(ctx); err != nil {
				s.logger.Error("Failed to collect power data", "error", err)
				// Continue monitoring despite errors
			}
		}
	}
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

// LatestReading returns the most recent power reading with demand-based collection
func (s *Service) LatestReading() (*PowerReading, string) {
	if err := s.ensureFreshData(); err != nil {
		s.logger.Error("Failed to ensure fresh data", "error", err)
		// Return cached data even if refresh failed
	}

	reading := s.lastReading.Load()
	if reading == nil {
		return nil, s.nodeID
	}
	return reading, s.nodeID
}

// BMCID returns the BMC ID for metrics labeling
func (s *Service) BMCID() string {
	return s.bmcID
}

// ensureFreshData ensures power data is fresh, collecting new data if stale
func (s *Service) ensureFreshData() error {
	if s.isFresh() {
		return nil
	}

	// Use singleflight to prevent concurrent power collection calls
	_, err, _ := s.computeGroup.Do("power", func() (interface{}, error) {
		return nil, s.synchronizedPowerRefresh()
	})

	return err
}

// isFresh checks if the current power data is within the staleness threshold
func (s *Service) isFresh() bool {
	reading := s.lastReading.Load()
	if reading == nil {
		return false
	}
	return time.Since(reading.Timestamp) < s.collection.Staleness
}

// synchronizedPowerRefresh performs the actual power data collection
func (s *Service) synchronizedPowerRefresh() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Use retry logic: 3 attempts with 2-second delay
	reading, err := s.powerReader.ReadPowerWithRetry(ctx, 3, 2*time.Second)
	if err != nil {
		s.logger.Error("Failed to collect power data on-demand", "error", err)
		return fmt.Errorf("failed to read power from BMC: %w", err)
	}

	// Update atomic pointer
	s.lastReading.Store(reading)

	return nil
}

// collectPowerData collects power data from the BMC with retry logic (for periodic collection)
func (s *Service) collectPowerData(ctx context.Context) error {
	return s.synchronizedPowerRefresh()
}
