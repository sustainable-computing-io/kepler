// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"log/slog"
	"os"

	"github.com/oklog/run"
)

// Run runs all services that implement the Runner interface.
// It returns an error if any service fails.
func Run(outer context.Context, logger *slog.Logger, services []Service) error {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(os.Stderr, nil))
	}

	logger.Info("Running all services")
	ctx, cancel := context.WithCancel(outer)
	defer cancel()
	// Create run group
	var g run.Group

	// Add services to run group
	for _, s := range services {
		runner, ok := s.(Runner)
		if !ok {
			logger.Warn("skipping service", "service", s.Name())
			continue
		}

		// Create local copies of the variables for the closure
		svc := s
		r := runner
		g.Add(
			func() error {
				logger.Info("Running service", "service", svc.Name())
				return r.Run(ctx)
			},
			func(err error) {
				cancel()
				if err != nil {
					logger.Warn("service terminated", "service", svc.Name(), "reason", err)
				}

				shutdowner, ok := svc.(Shutdowner)
				if !ok {
					logger.Debug("skipping service shutting down", "service", svc.Name(),
						"reason", "service does not implement Shutdowner interface")
					return
				}

				logger.Info("shutting down", "service", svc.Name())
				if shutdownErr := shutdowner.Shutdown(); shutdownErr != nil {
					logger.Warn("service shutdown failed with error", "service", svc.Name(), "error", shutdownErr)
				}
			},
		)
	}

	return g.Run()
}
