// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package redfish

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewPowerReader(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	powerReader := NewPowerReader(logger)

	assert.NotNil(t, powerReader)
	assert.Equal(t, logger, powerReader.logger)
	assert.Nil(t, powerReader.client) // Should be nil initially
}

func TestPowerReaderSetClient(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	powerReader := NewPowerReader(logger)

	// Test with nil client
	powerReader.SetClient(nil)
	assert.Nil(t, powerReader.client)
	assert.Empty(t, powerReader.endpoint)
}

func TestPowerReaderReadPowerNotConnected(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	powerReader := NewPowerReader(logger)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	reading, err := powerReader.ReadPower(ctx)
	assert.Error(t, err)
	assert.Nil(t, reading)
	assert.Contains(t, err.Error(), "not connected")
}

func TestPowerReaderReadPowerWithRetryNotConnected(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	powerReader := NewPowerReader(logger)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	reading, err := powerReader.ReadPowerWithRetry(ctx, 2, 10*time.Millisecond)
	assert.Error(t, err)
	assert.Nil(t, reading)
	assert.Contains(t, err.Error(), "failed to read power after 2 attempts")
}
