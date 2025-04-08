// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package service

import "context"

// Service is the interface that all services must implement
type Service interface {
	// Name returns the name of the service
	Name() string
	// Start starts the service
	Start(ctx context.Context) error
	// Stop stops the service
	Stop() error
}
