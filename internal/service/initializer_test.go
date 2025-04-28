// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInit(t *testing.T) {
	t.Run("all services initialize successfully", func(t *testing.T) {
		svc1 := &mockInitializer{mockService: mockService{name: "svc1"}}
		svc2 := &mockInitializer{mockService: mockService{name: "svc2"}}
		svc3 := &mockService{name: "non-initializer"}

		services := []Service{svc1, svc2, svc3}

		err := Init(nil, services)

		// Verify results
		assert.NoError(t, err)
		assert.Equal(t, 1, svc1.initCount)
		assert.Equal(t, 1, svc2.initCount)
	})

	t.Run("initialization fails and shutdown is called", func(t *testing.T) {
		svc1 := &mockInitShutdownService{mockService: mockService{name: "svc1"}}

		initErr := errors.New("init error")
		svc2 := &mockInitShutdownService{
			mockService: mockService{name: "svc2"},
			initFn:      func() error { return initErr },
		}

		svc3 := &mockInitShutdownService{mockService: mockService{name: "svc3"}}

		services := []Service{svc1, svc2, svc3}

		err := Init(nil, services)

		// Verify results
		assert.Error(t, err)
		assert.ErrorIs(t, err, initErr)

		// svc1 should have been initialized and shut down
		assert.Equal(t, 1, svc1.initCount)
		assert.Equal(t, 1, svc1.shutdownCount)

		// svc2 initialization failed, so it shouldn't be shut down
		assert.Equal(t, 1, svc2.initCount)
		assert.Equal(t, 0, svc2.shutdownCount)

		// svc3 should not have been initialized or shut down
		assert.Equal(t, 0, svc3.initCount)
		assert.Equal(t, 0, svc3.shutdownCount)
	})

	t.Run("shutdown error is logged but doesn't affect return value", func(t *testing.T) {
		svc1 := &mockInitShutdownService{mockService: mockService{name: "svc1"}}

		initErr := errors.New("init error")
		shutdownErr := errors.New("shutdown error")

		svc2 := &mockInitShutdownService{
			mockService: mockService{name: "svc2"},
			initFn:      func() error { return initErr },
		}

		svc1.shutdownFn = func() error { return shutdownErr }

		services := []Service{svc1, svc2}

		err := Init(nil, services)

		// Verify Init should return the init error, not the shutdown error
		assert.Error(t, err)
		assert.ErrorIs(t, err, initErr)
		assert.NotErrorIs(t, err, shutdownErr)

		// svc1 should have been initialized and shut down
		assert.Equal(t, 1, svc1.initCount)
		assert.Equal(t, 1, svc1.shutdownCount)
	})

	t.Run("non-shutdowner service is skipped during cleanup", func(t *testing.T) {
		svc1 := &mockInitializer{mockService: mockService{name: "svc1"}}

		initErr := errors.New("init error")
		svc2 := &mockInitializer{
			mockService: mockService{name: "svc2"},
			initFn:      func() error { return initErr },
		}

		services := []Service{svc1, svc2}

		err := Init(nil, services)

		// Verify results
		assert.Error(t, err)
		assert.ErrorIs(t, err, initErr)

		// svc1 should have been initialized but not shut down (as it doesn't implement Shutdowner)
		assert.Equal(t, 1, svc1.initCount)
	})

	t.Run("empty service list completes successfully", func(t *testing.T) {
		err := Init(nil, []Service{})
		assert.NoError(t, err)
	})
}
