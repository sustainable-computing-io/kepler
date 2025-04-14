// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package device

// TODO: Move this mock to a separate testutil package

import (
	"slices"
	"testing"

	"github.com/prometheus/procfs/sysfs"
	"github.com/stretchr/testify/require"
)

const (
	validSysFSPath = "testdata/sys"
	badSysFSPath   = "testdata/bad_sysfs"
)

type (
	MockRaplZone struct {
		energy    Energy
		energyErr error

		name           string
		index          int
		path           string
		maxMicroJoules Energy
	}
)

func NewMockRaplZone(name string, index int, path string, maxMicroJoules Energy) *MockRaplZone {
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

func (m MockRaplZone) Energy() (Energy, error) {
	return m.energy, m.energyErr
}

func (m MockRaplZone) MaxEnergy() Energy {
	return m.maxMicroJoules
}

func (m *MockRaplZone) OnEnergy(j Energy, err error) {
	m.energy = j
	m.energyErr = err
}

func (m *MockRaplZone) Inc(delta Energy) {
	m.energy = (m.energy + delta) % m.maxMicroJoules
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

func sortedZoneNames(zones []EnergyZone) []string {
	names := make([]string, len(zones))
	for i, zone := range zones {
		names[i] = zone.Name()
	}
	slices.Sort(names)

	return names
}
