// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package redfish

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/sustainable-computing-io/kepler/internal/service"
)

// Service implements the Redfish power monitoring service
type Service struct {
	logger      *slog.Logger
	config      *BMCConfig
	client      *gofishClient
	powerReader *PowerReader
	nodeID      string

	// Data collection
	mu             sync.RWMutex
	lastReading    *PowerReading
	totalEnergyJ   float64 // Total energy consumed in joules
	lastUpdateTime time.Time

	// Service lifecycle
	running bool
	stopCh  chan struct{}
}

// NewService creates a new Redfish service
func NewService(configPath, nodeID string, logger *slog.Logger) (*Service, error) {
	// Load BMC configuration
	bmcConfig, err := LoadBMCConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load BMC configuration: %w", err)
	}

	logger.Info("Resolved node identifier", "node_id", nodeID)

	// Get BMC details for this node
	bmcDetail, err := bmcConfig.GetBMCForNode(nodeID)
	if err != nil {
		return nil, fmt.Errorf("failed to get BMC configuration for node %s: %w", nodeID, err)
	}

	// Create client and power reader
	client := NewClient(bmcDetail)
	powerReader := NewPowerReader(client, logger)

	return &Service{
		logger:      logger,
		config:      bmcConfig,
		client:      client,
		powerReader: powerReader,
		nodeID:      nodeID,
		stopCh:      make(chan struct{}),
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
		"bmc_endpoint", s.client.Endpoint())

	// Use context.Background() for client connection since gofish stores this context
	// and uses it for all subsequent HTTP requests. A timeout context would cause
	// "context canceled" errors on later requests when the timeout expires.
	if err := s.client.Connect(context.Background()); err != nil {
		// Don't log credentials in error messages
		return fmt.Errorf("failed to connect to BMC for node %s: %w", s.nodeID, err)
	}

	s.logger.Info("Successfully connected to BMC", "node_id", s.nodeID)
	return nil
}

// Run starts the power monitoring loop
func (s *Service) Run(ctx context.Context) error {
	s.mu.Lock()
	s.running = true
	s.mu.Unlock()

	s.logger.Info("Starting Redfish power monitoring loop", "node_id", s.nodeID)

	// Collection interval: every 10 seconds
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("Redfish power monitoring stopped due to context cancellation")
			return ctx.Err()
		case <-s.stopCh:
			s.logger.Info("Redfish power monitoring stopped")
			return nil
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
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil
	}

	s.logger.Info("Shutting down Redfish power monitoring service")

	close(s.stopCh)
	s.client.Disconnect()
	s.running = false

	s.logger.Info("Redfish power monitoring service shutdown complete")
	return nil
}

// GetLatestReading returns the most recent power reading
func (s *Service) GetLatestReading() (*PowerReading, float64, string) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.lastReading, s.totalEnergyJ, s.nodeID
}

// collectPowerData collects power data from the BMC with retry logic
func (s *Service) collectPowerData(ctx context.Context) error {
	// Use retry logic: 3 attempts with 2-second delay
	reading, err := s.powerReader.ReadPowerWithRetry(ctx, 3, 2*time.Second)
	if err != nil {
		return fmt.Errorf("failed to read power from BMC: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Calculate energy consumption if we have a previous reading
	if s.lastReading != nil {
		timeDelta := reading.Timestamp.Sub(s.lastUpdateTime).Seconds()
		if timeDelta > 0 {
			// Energy = Power × Time (in seconds)
			// Convert watts*seconds to joules (1 W*s = 1 J)
			avgPower := (reading.PowerWatts + s.lastReading.PowerWatts) / 2
			energyDelta := avgPower * timeDelta
			s.totalEnergyJ += energyDelta

			s.logger.Debug("Updated energy calculation",
				"node_id", s.nodeID,
				"power_watts", reading.PowerWatts,
				"time_delta_s", timeDelta,
				"energy_delta_j", energyDelta,
				"total_energy_j", s.totalEnergyJ)
		}
	}

	s.lastReading = reading
	s.lastUpdateTime = reading.Timestamp

	return nil
}

// IsRunning returns true if the service is currently running
func (s *Service) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// Ensure Service implements the required interfaces
var (
	_ service.Service     = (*Service)(nil)
	_ service.Initializer = (*Service)(nil)
	_ service.Runner      = (*Service)(nil)
	_ service.Shutdowner  = (*Service)(nil)
)
