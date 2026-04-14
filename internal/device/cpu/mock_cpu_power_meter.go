// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package cpu

// TODO: Move this mock to a separate testutil package

import (
	"slices"
	"testing"

	"fmt"

	"github.com/prometheus/procfs/sysfs"
	"github.com/stretchr/testify/require"
        "github.com/sustainable-computing-io/kepler/internal/device"
)

const (
	validSysFSPath = "testdata/sys"
	badSysFSPath   = "testdata/bad_sysfs"
)

type (
	MockRaplZone struct {
		energy    device.Energy
		energyErr error

		name           string
		index          int
		path           string
		maxMicroJoules device.Energy
	}

	MockPowerZone struct {
		power    device.Power
		powerErr error

		name  string
		index int
		path  string
	}
)

func NewMockRaplZone(name string, index int, path string, maxMicroJoules device.Energy) *MockRaplZone {
	return &MockRaplZone{
		name:           name,
		index:          index,
		path:           path,
		maxMicroJoules: maxMicroJoules,
	}
}

func (m MockRaplZone) Index() int {
	return m.index
}

func (m MockRaplZone) Path() string {
	return m.path
}

func (m MockRaplZone) Name() string {
	return m.name
}

func (m MockRaplZone) Energy() (device.Energy, error) {
	return m.energy, m.energyErr
}

func (m MockRaplZone) MaxEnergy() device.Energy {
	return m.maxMicroJoules
}

func (m MockRaplZone) Power() (device.Power, error) {
	// Mock RAPL zones don't provide power
	return 0, fmt.Errorf("mock rapl zones do not provide power readings")
}

func (m *MockRaplZone) OnEnergy(j device.Energy, err error) {
	m.energy = j
	m.energyErr = err
}

func (m *MockRaplZone) Inc(delta device.Energy) {
	m.energy = (m.energy + delta) % m.maxMicroJoules
}

func NewMockPowerZone(name string, index int, path string) *MockPowerZone {
	return &MockPowerZone{
		name:  name,
		index: index,
		path:  path,
	}
}

func (m MockPowerZone) Index() int {
	return m.index
}

func (m MockPowerZone) Path() string {
	return m.path
}

func (m MockPowerZone) Name() string {
	return m.name
}

func (m MockPowerZone) Energy() (device.Energy, error) {
	// Power zones don't provide energy readings
	return 0, nil
}

func (m MockPowerZone) MaxEnergy() device.Energy {
	// Power zones don't have max energy
	return 0
}

func (m MockPowerZone) Power() (device.Power, error) {
	return m.power, m.powerErr
}

func (m *MockPowerZone) OnPower(power device.Power, err error) {
	m.power = power
	m.powerErr = err
}

func (m *MockPowerZone) SetPower(power device.Power) {
	m.power = power
}

func validSysFSFixtures(t *testing.T) sysfs.FS {
	t.Helper()
	fs, err := sysfs.NewFS(validSysFSPath)
	require.NoError(t, err, "Failed to create sysfs test FS")
	return fs
}

func invalidSysFSFixtures(t *testing.T) sysfs.FS {
	t.Helper()
	fs, err := sysfs.NewFS(badSysFSPath)
	require.NoError(t, err, "Failed to create sysfs test FS")
	return fs
}

func sortedZoneNames(zones []device.EnergyZone) []string {
	names := make([]string, len(zones))
	for i, zone := range zones {
		names[i] = zone.Name()
	}
	slices.Sort(names)

	return names
}
