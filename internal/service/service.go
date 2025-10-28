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

// LiveChecker is the interface that services can implement to provide liveness status
type LiveChecker interface {
	Service
	// IsLive returns true if the service is alive (initialized and running without fatal errors)
	IsLive() bool
}

// ReadyChecker is the interface that services can implement to provide readiness status
type ReadyChecker interface {
	Service
	// IsReady returns true if the service is ready to serve requests (has data and is not stale)
	IsReady() bool
}
