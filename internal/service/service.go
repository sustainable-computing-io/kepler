// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package service

import "context"

// Service is the interface that all services must implement
type Service interface {
	// Name returns the name of the service
	Name() string
	// Init initializes the service and is called before the service is run. Init
	// is not required to be thread safe.
	Init(ctx context.Context) error

	// Run runs the service and is expected to block and be thread safe
	Run(ctx context.Context) error

	// Shutdown() shuts down the service and is called after the service is run
	Shutdown() error
}
