// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package network

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/sustainable-computing-io/kepler/internal/device"
)

const (
	// defaultOutputDir is the directory where the eBPF NIC energy program
	// writes powercap-style energy files.
	defaultOutputDir = "/var/lib/powercap/ebpf-nic"

	energyFile   = "energy_uj"
	maxEnergyFile = "max_energy_range_uj"
	nameFile     = "name"
)

// nicPowerMeter implements NICPowerMeter by reading energy_uj from a
// powercap-style directory populated by an eBPF program.
type nicPowerMeter struct {
	dir string // path to the powercap directory
}

// NewNICPowerMeter creates a NICPowerMeter that reads from dir.
// If dir is empty, defaultOutputDir is used.
func NewNICPowerMeter(dir string) NICPowerMeter {
	if dir == "" {
		dir = defaultOutputDir
	}
	return &nicPowerMeter{dir: dir}
}

func (m *nicPowerMeter) Name() string {
	return "ebpf-nic"
}

func (m *nicPowerMeter) Init() error {
	p := filepath.Join(m.dir, energyFile)
	_, err := readUint64(p)
	if err != nil {
		return fmt.Errorf("nic power meter init: %w", err)
	}
	return nil
}

func (m *nicPowerMeter) Zone() (device.EnergyZone, error) {
	z := &nicEnergyZone{dir: m.dir}

	// Validate that we can read the energy file.
	if _, err := z.Energy(); err != nil {
		return nil, fmt.Errorf("nic energy zone: %w", err)
	}
	return z, nil
}

// nicEnergyZone implements device.EnergyZone for NIC energy.
type nicEnergyZone struct {
	dir string
}

func (z *nicEnergyZone) Name() string {
	p := filepath.Join(z.dir, nameFile)
	data, err := os.ReadFile(p)
	if err != nil {
		return "nic"
	}
	return strings.TrimSpace(string(data))
}

func (z *nicEnergyZone) Index() int {
	return 0
}

func (z *nicEnergyZone) Path() string {
	return filepath.Join(z.dir, energyFile)
}

func (z *nicEnergyZone) Energy() (device.Energy, error) {
	val, err := readUint64(filepath.Join(z.dir, energyFile))
	if err != nil {
		return 0, fmt.Errorf("read nic energy_uj: %w", err)
	}
	return device.Energy(val), nil
}

func (z *nicEnergyZone) MaxEnergy() device.Energy {
	val, err := readUint64(filepath.Join(z.dir, maxEnergyFile))
	if err != nil {
		// Default to max uint64 if the file is absent.
		return device.Energy(^uint64(0))
	}
	return device.Energy(val)
}

func (z *nicEnergyZone) Power() (device.Power, error) {
	return 0, fmt.Errorf("nic zone provides cumulative energy, not instantaneous power")
}

// readUint64 reads a single uint64 value from a sysfs-style file.
func readUint64(path string) (uint64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	return strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
}
