// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package redfish

import (
	"log/slog"
	"os"
	"testing"

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

func TestPowerReaderReadAllNotConnected(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	powerReader := NewPowerReader(logger)

	readings, err := powerReader.ReadAll()
	assert.Error(t, err)
	assert.Nil(t, readings)
	assert.Contains(t, err.Error(), "not connected")
}
