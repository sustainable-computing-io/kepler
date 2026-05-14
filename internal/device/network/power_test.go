// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package network

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestDir(t *testing.T, energyUJ string, extras map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, energyFile), []byte(energyUJ), 0o644))
	for name, content := range extras {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644))
	}
	return dir
}

func TestNICPowerMeter_Init(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		dir := setupTestDir(t, "123456\n", nil)
		m := NewNICPowerMeter(dir)
		assert.NoError(t, m.Init())
	})

	t.Run("missing energy file", func(t *testing.T) {
		m := NewNICPowerMeter(t.TempDir())
		assert.Error(t, m.Init())
	})
}

func TestNICPowerMeter_Name(t *testing.T) {
	m := NewNICPowerMeter("/nonexistent")
	assert.Equal(t, "ebpf-nic", m.Name())
}

func TestNICPowerMeter_Zone(t *testing.T) {
	t.Run("reads energy", func(t *testing.T) {
		dir := setupTestDir(t, "999000\n", nil)
		m := NewNICPowerMeter(dir)

		zone, err := m.Zone()
		require.NoError(t, err)

		energy, err := zone.Energy()
		require.NoError(t, err)
		assert.Equal(t, uint64(999000), energy.MicroJoules())
	})

	t.Run("missing energy file returns error", func(t *testing.T) {
		m := NewNICPowerMeter(t.TempDir())
		_, err := m.Zone()
		assert.Error(t, err)
	})
}

func TestNICEnergyZone_Name(t *testing.T) {
	t.Run("reads name file", func(t *testing.T) {
		dir := setupTestDir(t, "0\n", map[string]string{nameFile: "eth0-energy\n"})
		m := NewNICPowerMeter(dir)

		zone, err := m.Zone()
		require.NoError(t, err)
		assert.Equal(t, "eth0-energy", zone.Name())
	})

	t.Run("defaults to nic when name file missing", func(t *testing.T) {
		dir := setupTestDir(t, "0\n", nil)
		m := NewNICPowerMeter(dir)

		zone, err := m.Zone()
		require.NoError(t, err)
		assert.Equal(t, "nic", zone.Name())
	})
}

func TestNICEnergyZone_MaxEnergy(t *testing.T) {
	t.Run("reads max energy file", func(t *testing.T) {
		dir := setupTestDir(t, "100\n", map[string]string{maxEnergyFile: "262143328850\n"})
		m := NewNICPowerMeter(dir)

		zone, err := m.Zone()
		require.NoError(t, err)
		assert.Equal(t, uint64(262143328850), zone.MaxEnergy().MicroJoules())
	})

	t.Run("defaults to max uint64 when file missing", func(t *testing.T) {
		dir := setupTestDir(t, "100\n", nil)
		m := NewNICPowerMeter(dir)

		zone, err := m.Zone()
		require.NoError(t, err)
		assert.Equal(t, uint64(^uint64(0)), zone.MaxEnergy().MicroJoules())
	})
}

func TestNICEnergyZone_Path(t *testing.T) {
	dir := setupTestDir(t, "0\n", nil)
	m := NewNICPowerMeter(dir)

	zone, err := m.Zone()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, energyFile), zone.Path())
}

func TestNICEnergyZone_Power(t *testing.T) {
	dir := setupTestDir(t, "0\n", nil)
	m := NewNICPowerMeter(dir)

	zone, err := m.Zone()
	require.NoError(t, err)

	_, err = zone.Power()
	assert.Error(t, err, "Power() should return an error for cumulative-energy-only zones")
}

func TestNICEnergyZone_Index(t *testing.T) {
	dir := setupTestDir(t, "0\n", nil)
	m := NewNICPowerMeter(dir)

	zone, err := m.Zone()
	require.NoError(t, err)
	assert.Equal(t, 0, zone.Index())
}

func TestNewNICPowerMeter_DefaultDir(t *testing.T) {
	m := NewNICPowerMeter("")
	pm := m.(*nicPowerMeter)
	assert.Equal(t, defaultOutputDir, pm.dir)
}
