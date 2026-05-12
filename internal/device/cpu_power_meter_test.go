// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package device

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sustainable-computing-io/kepler/config"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

func TestCreateCPUMeter_FakeOnly(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Cpu.Meters = []string{"fake"}

	meter, err := CreateCPUMeter(discardLogger(), cfg)
	require.NoError(t, err)
	require.NotNil(t, meter)
	assert.Equal(t, "fake-cpu-meter", meter.Name())
}

func TestCreateCPUMeter_UnknownBackend(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Cpu.Meters = []string{"rappl"}

	meter, err := CreateCPUMeter(discardLogger(), cfg)
	require.Error(t, err)
	assert.Nil(t, meter)
	assert.Contains(t, err.Error(), "unknown cpu meter")
}

func TestCreateCPUMeter_FallthroughToFake(t *testing.T) {
	// Unknown name, then fake — exercises the factory-error path
	// and the success-after-continue branch.
	cfg := config.DefaultConfig()
	cfg.Cpu.Meters = []string{"rappl", "fake"}

	meter, err := CreateCPUMeter(discardLogger(), cfg)
	require.NoError(t, err)
	require.NotNil(t, meter)
	assert.Equal(t, "fake-cpu-meter", meter.Name())
}

func TestCreateCPUMeter_EmptyMeters(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Cpu.Meters = nil

	meter, err := CreateCPUMeter(discardLogger(), cfg)
	require.Error(t, err)
	assert.Nil(t, meter)
	assert.Contains(t, err.Error(), "cpu.meters is empty")
}

func TestCreateCPUMeter_RaplFactoryError_FallsThroughToFake(t *testing.T) {
	// A bogus sysfs path makes NewCPUPowerMeter (rapl) fail at construction
	// because sysfs.NewFS validates the path. Falls through to fake.
	cfg := config.DefaultConfig()
	cfg.Host.SysFS = "/nonexistent/sysfs/path"
	cfg.Cpu.Meters = []string{"rapl", "fake"}
	cfg.Rapl.Zones = []string{"package"} // exercise the "rapl zones are filtered" log line

	meter, err := CreateCPUMeter(discardLogger(), cfg)
	require.NoError(t, err)
	require.NotNil(t, meter)
	assert.Equal(t, "fake-cpu-meter", meter.Name())
}

func TestCreateCPUMeter_HwmonInitError_FallsThroughToFake(t *testing.T) {
	// hwmon's NewHwmonPowerMeter constructs successfully on any path; Init
	// fails when the path has no hwmon zones. Exercises the Init-error path
	// in CreateCPUMeter and the experimental-config wiring in buildCPUMeter
	// (zones + chip rules).
	cfg := config.DefaultConfig()
	cfg.Host.SysFS = "/nonexistent/sysfs/path"
	cfg.Cpu.Meters = []string{"hwmon", "fake"}
	cfg.Experimental = &config.Experimental{}
	cfg.Experimental.Hwmon.Zones = []string{"power1"}
	cfg.Experimental.Hwmon.ChipRules = []config.ChipPairingRule{
		{Name: "ina3221", UseSameIndex: true},
	}

	meter, err := CreateCPUMeter(discardLogger(), cfg)
	require.NoError(t, err)
	require.NotNil(t, meter)
	assert.Equal(t, "fake-cpu-meter", meter.Name())
}

func TestCreateCPUMeter_AllFail_AggregatedError(t *testing.T) {
	// Both rapl and hwmon fail (bogus sysfs); no fallback. Aggregated error.
	cfg := config.DefaultConfig()
	cfg.Host.SysFS = "/nonexistent/sysfs/path"
	cfg.Cpu.Meters = []string{"rapl", "hwmon"}

	meter, err := CreateCPUMeter(discardLogger(), cfg)
	require.Error(t, err)
	assert.Nil(t, meter)
	assert.Contains(t, err.Error(), "rapl")
	assert.Contains(t, err.Error(), "hwmon")
}

func TestBuildCPUMeter_Fake(t *testing.T) {
	cfg := config.DefaultConfig()

	meter, err := buildCPUMeter("fake", discardLogger(), cfg)
	require.NoError(t, err)
	require.NotNil(t, meter)
	assert.Equal(t, "fake-cpu-meter", meter.Name())
}

func TestBuildCPUMeter_Rapl_FactoryFails(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Host.SysFS = "/nonexistent"

	meter, err := buildCPUMeter("rapl", discardLogger(), cfg)
	require.Error(t, err)
	assert.Nil(t, meter)
}

func TestBuildCPUMeter_Hwmon_ConstructsWithExperimentalConfig(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Experimental = &config.Experimental{}
	cfg.Experimental.Hwmon.Zones = []string{"power1", "power2"}
	cfg.Experimental.Hwmon.ChipRules = []config.ChipPairingRule{
		{Name: "ltc2945", Pairings: map[int]int{1: 1}},
	}

	meter, err := buildCPUMeter("hwmon", discardLogger(), cfg)
	require.NoError(t, err)
	require.NotNil(t, meter)
	assert.Equal(t, "hwmon", meter.Name())
}

func TestBuildCPUMeter_Hwmon_NilExperimental(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Experimental = nil

	meter, err := buildCPUMeter("hwmon", discardLogger(), cfg)
	require.NoError(t, err)
	require.NotNil(t, meter)
}

func TestBuildCPUMeter_Unknown(t *testing.T) {
	cfg := config.DefaultConfig()

	meter, err := buildCPUMeter("nope", discardLogger(), cfg)
	require.Error(t, err)
	assert.Nil(t, meter)
	assert.Contains(t, err.Error(), "unknown cpu meter")
}
