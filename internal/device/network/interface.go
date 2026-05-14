// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package network

import "github.com/sustainable-computing-io/kepler/internal/device"

// Stats holds a snapshot of per-interface traffic counters sourced from
// /sys/class/net/<iface>/statistics.
type Stats struct {
	RxBytes   uint64
	TxBytes   uint64
	RxPackets uint64
	TxPackets uint64
	RxErrors  uint64
	TxErrors  uint64
	RxDropped uint64
	TxDropped uint64
}

// NIC represents a single physical network interface on the node.
type NIC interface {
	Name() string
	Stats() (Stats, error)
}

// Meter enumerates physical network interfaces on the node.
type Meter interface {
	Name() string
	PhysicalNICs() ([]NIC, error)
}

// NICPowerMeter reads cumulative NIC energy from a powercap-style directory
// (e.g. /var/lib/powercap/ebpf-nic) where an eBPF program writes energy_uj.
type NICPowerMeter interface {
	// Name identifies this power meter.
	Name() string

	// Init validates that the energy source is readable.
	Init() error

	// Zone returns the energy zone for the NIC.
	Zone() (device.EnergyZone, error)
}
