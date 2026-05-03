// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package device

import (
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sustainable-computing-io/kepler/config"
)

func TestCreateCPUMeter_FakeOnly(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Cpu.Meters = []string{"fake"}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	meter, err := CreateCPUMeter(logger, cfg)
	require.NoError(t, err)
	require.NotNil(t, meter)
	assert.Equal(t, "fake-cpu-meter", meter.Name())
}

func TestCreateCPUMeter_UnknownBackend(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Cpu.Meters = []string{"rappl"}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	meter, err := CreateCPUMeter(logger, cfg)
	require.Error(t, err)
	assert.Nil(t, meter)
	assert.Contains(t, err.Error(), "unknown cpu meter")
}

func TestCreateCPUMeter_FallthroughToFake(t *testing.T) {
	// "rappl" is unknown so it falls through to fake.
	cfg := config.DefaultConfig()
	cfg.Cpu.Meters = []string{"rappl", "fake"}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	meter, err := CreateCPUMeter(logger, cfg)
	require.NoError(t, err)
	require.NotNil(t, meter)
	assert.Equal(t, "fake-cpu-meter", meter.Name())
}

func TestCreateCPUMeter_EmptyMeters(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Cpu.Meters = nil

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	meter, err := CreateCPUMeter(logger, cfg)
	require.Error(t, err)
	assert.Nil(t, meter)
	assert.Contains(t, err.Error(), "cpu.meters is empty")
}

func TestBuildCPUMeter_Fake(t *testing.T) {
	cfg := config.DefaultConfig()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	meter, err := buildCPUMeter("fake", logger, cfg)
	require.NoError(t, err)
	require.NotNil(t, meter)
	assert.Equal(t, "fake-cpu-meter", meter.Name())
}

func TestBuildCPUMeter_Unknown(t *testing.T) {
	cfg := config.DefaultConfig()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	meter, err := buildCPUMeter("nope", logger, cfg)
	require.Error(t, err)
	assert.Nil(t, meter)
	assert.Contains(t, err.Error(), "unknown cpu meter")
}
