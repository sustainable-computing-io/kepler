// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSignalHandlerRun(t *testing.T) {
	t.Run("returns when context is canceled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		sh := NewSignalHandler(syscall.SIGINT)

		errCh := make(chan error)
		go func() {
			errCh <- sh.Run(ctx)
		}()

		// Cancel the context
		cancel()

		var err error
		select {
		case err = <-errCh:
			// Got result
		case <-time.After(time.Second):
			t.Fatal("Run did not return after context cancellation")
		}

		assert.Equal(t, context.Canceled, err)
	})
}
