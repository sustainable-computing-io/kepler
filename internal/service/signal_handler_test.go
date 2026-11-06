// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSignalHandlerRun(t *testing.T) {
	t.Run("returns when context is canceled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		sh := NewSignalHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), syscall.SIGINT)

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

func TestSignalHandlerLogsHintAsJSON(t *testing.T) {
	var out bytes.Buffer
	sh := NewSignalHandler(slog.New(slog.NewJSONHandler(&out, nil)), syscall.SIGINT)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- sh.Run(ctx)
	}()
	cancel()

	select {
	case err := <-errCh:
		assert.Equal(t, context.Canceled, err)
	case <-time.After(time.Second):
		t.Fatal("Run did not return after context cancellation")
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	require.NotEmpty(t, lines)

	foundHint := false
	for _, line := range lines {
		var record map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &record), "stderr line is not valid JSON: %q", line)
		if record["msg"] == "Press Ctrl+C to shutdown" {
			foundHint = true
		}
	}
	assert.True(t, foundHint, "shutdown hint was not logged")
}
