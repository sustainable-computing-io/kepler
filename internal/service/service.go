// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package service

import "context"

// Service is the interface that all services must implement
type Service interface {
	// Name returns the name of the service
	Name() string
}

// Initializer is the interface that all services must implement that are to be initialized
type Initializer interface {
	Service
	Init() error
}

// Runner is the interface that all services must implement that needs to run in background
type Runner interface {
	Service
	// Run runs the service and is expected to block and be thread safe
	Run(ctx context.Context) error
}

// Shutdowner is the interface that all services must implement that are to be shutdown / cleaned up
type Shutdowner interface {
	Service
	// Shutdown shuts down the service
	Shutdown() error
}

// LiveChecker is the interface for checking if a service is alive
type LiveChecker interface {
	// IsLive returns whether the service is alive and responsive
	IsLive(ctx context.Context) (bool, error)
}

// ReadyChecker is the interface for checking if a service is ready to serve traffic
type ReadyChecker interface {
	// IsReady returns whether the service is ready to handle requests
	IsReady(ctx context.Context) (bool, error)
}
