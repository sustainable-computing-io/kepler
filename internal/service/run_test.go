// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRun(t *testing.T) {
	t.Run("all services run successfully", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Create mock services that complete immediately
		svc1 := &mockRunner{
			mockService: mockService{name: "svc1"},
			runFn: func(ctx context.Context) error {
				return nil
			},
		}

		svc2 := &mockRunner{
			mockService: mockService{name: "svc2"},
			runFn: func(ctx context.Context) error {
				return nil
			},
		}

		svc3 := &mockService{name: "non-runner"}

		services := []Service{svc1, svc2, svc3}

		// Set timeout to prevent test from hanging
		ctxTimeout, cancelTimeout := context.WithTimeout(ctx, 100*time.Millisecond)
		defer cancelTimeout()

		// We need to run this in a goroutine since Run blocks until all services complete
		errCh := make(chan error)
		go func() {
			errCh <- Run(ctxTimeout, nil, services)
		}()

		// Give services time to start
		time.Sleep(50 * time.Millisecond)
		// calling cancel should trigger shutdown
		cancel()
		err := <-errCh

		assert.NoError(t, err)
	})

	t.Run("service fails and triggers shutdown", func(t *testing.T) {
		runErr := errors.New("run error")

		// NOTE: This service will return an error
		svc1 := &mockRunShutdownService{
			mockService: mockService{name: "svc1"},
			runFn: func(ctx context.Context) error {
				return runErr
			},
		}

		svc2 := &mockRunShutdownService{
			mockService: mockService{name: "svc2"},
			runFn: func(ctx context.Context) error {
				// Block until context canceled
				<-ctx.Done()
				return ctx.Err()
			},
		}

		// We need to run this in a goroutine since Run blocks
		errCh := make(chan error)
		go func() {
			services := []Service{svc1, svc2}
			errCh <- Run(context.Background(), nil, services)
		}()

		// Give services time to run
		time.Sleep(50 * time.Millisecond)

		// Wait for Run to return
		err := <-errCh

		// Verify results
		assert.Error(t, err)
		assert.ErrorIs(t, err, runErr)

		// svc1's Shutdown should be called
		assert.Equal(t, 1, svc1.shutdownCount)

		// svc2's Shutdown might or might not be called depending on timing
		// We can't reliably assert on this
	})

	t.Run("service shutdown error is logged", func(t *testing.T) {
		ctx := context.Background()

		runErr := errors.New("run error")
		shutdownErr := errors.New("shutdown error")

		svc := &mockRunShutdownService{
			mockService: mockService{name: "svc"},
			runFn: func(ctx context.Context) error {
				return runErr
			},
			shutdownFn: func() error {
				return shutdownErr
			},
		}

		services := []Service{svc}

		// Set timeout to prevent test from hanging
		ctxTimeout, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
		defer cancel()

		// Run the services
		err := Run(ctxTimeout, nil, services)

		// Verify results
		assert.Error(t, err)
		assert.ErrorIs(t, err, runErr)
		assert.Equal(t, 1, svc.runCount)
		assert.Equal(t, 1, svc.shutdownCount)
	})

	t.Run("context cancellation stops all services", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Set up channels to track service state
		svc1Started := make(chan struct{})
		svc2Started := make(chan struct{})

		svc1 := &mockRunShutdownService{
			mockService: mockService{name: "svc1"},
			runFn: func(ctx context.Context) error {
				close(svc1Started)
				<-ctx.Done()
				return ctx.Err()
			},
		}

		svc2 := &mockRunShutdownService{
			mockService: mockService{name: "svc2"},
			runFn: func(ctx context.Context) error {
				close(svc2Started)
				<-ctx.Done()
				return ctx.Err()
			},
		}

		services := []Service{svc1, svc2}

		// Run the services in a goroutine
		errCh := make(chan error)
		go func() {
			errCh <- Run(ctx, nil, services)
		}()

		// Wait for services to start
		<-svc1Started
		<-svc2Started

		// Cancel context
		cancel()

		// Wait for Run to return
		err := <-errCh

		// Verify results
		assert.Error(t, err)
		assert.Equal(t, context.Canceled, err)
		assert.Equal(t, 1, svc1.runCount)
		assert.Equal(t, 1, svc2.runCount)
	})

	t.Run("non-shutdowner service is skipped during cleanup", func(t *testing.T) {
		ctx := context.Background()

		runErr := errors.New("run error")

		svc1 := &mockRunner{
			mockService: mockService{name: "svc1"},
			runFn: func(ctx context.Context) error {
				return runErr
			},
		}

		svc2 := &mockRunner{
			mockService: mockService{name: "svc2"},
			runFn: func(ctx context.Context) error {
				<-ctx.Done()
				return ctx.Err()
			},
		}

		services := []Service{svc1, svc2}

		// Set timeout to prevent test from hanging
		ctxTimeout, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
		defer cancel()

		// Run the services
		err := Run(ctxTimeout, nil, services)

		// Verify results
		assert.Error(t, err)
		assert.ErrorIs(t, err, runErr)
	})

	t.Run("empty service list completes successfully", func(t *testing.T) {
		err := Run(context.Background(), nil, []Service{})
		assert.NoError(t, err)
	})
}
