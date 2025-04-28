// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"fmt"
	"log/slog"
	"os"
)

// Init initializes all services that implement the Initializer interface.
// If any service fails to initialize, it will shut down all previously initialized services
// that implement the Shutdowner interface.
func Init(logger *slog.Logger, services []Service) error {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(os.Stderr, nil))
	}

	var retErr error
	initialized := make([]Service, 0, len(services))

	for _, s := range services {
		srv, ok := s.(Initializer)
		if !ok {
			logger.Debug("skipping service initialization", "service", s.Name(),
				"reason", "service does not implement Initializer")
			continue
		}

		logger.Info("Initializing service", "service", s.Name())
		if err := srv.Init(); err != nil {
			retErr = fmt.Errorf("failed to initialize service %s: %w", s.Name(), err)
			break
		}
		initialized = append(initialized, s)
	}

	if retErr == nil {
		return nil
	}

	logger.Info("Shutting down initialized services")
	for _, s := range initialized {
		srv, ok := s.(Shutdowner)
		if !ok {
			logger.Debug("skipping service shutdown", "service", s.Name(),
				"reason", "service does not implement Shutdowner")
			continue
		}
		if err := srv.Shutdown(); err != nil {
			logger.Error("failed to shutdown service", "service", s.Name(), "error", err)
		} else {
			logger.Debug("service shutdown successfully", "service", s.Name())
		}
	}
	return retErr
}
