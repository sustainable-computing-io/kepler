// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package monitor

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAttributeZone(t *testing.T) {
	active := NodeUsage{Power: 50 * Watt, ActivePower: 20 * Watt, activeEnergy: 8 * Joule}

	for _, tt := range []struct {
		name             string
		nodeZoneUsage    NodeUsage
		prevEnergy       Energy
		cpuTimeDelta     float64
		nodeCPUTimeDelta float64
		want             Usage
	}{
		{
			name:             "adds the CPU time share of active energy",
			nodeZoneUsage:    active,
			prevEnergy:       25 * Joule,
			cpuTimeDelta:     25,
			nodeCPUTimeDelta: 100,
			want:             Usage{Power: 5 * Watt, EnergyTotal: 27 * Joule},
		},
		{
			name:             "workload without CPU time keeps its total",
			nodeZoneUsage:    active,
			prevEnergy:       25 * Joule,
			cpuTimeDelta:     0,
			nodeCPUTimeDelta: 100,
			want:             Usage{EnergyTotal: 25 * Joule},
		},
		{
			name:             "drops the part of a share below one microjoule",
			nodeZoneUsage:    NodeUsage{Power: 50 * Watt, ActivePower: 20 * Watt, activeEnergy: 8*Joule + 3},
			prevEnergy:       25 * Joule,
			cpuTimeDelta:     25,
			nodeCPUTimeDelta: 100,
			want:             Usage{Power: 5 * Watt, EnergyTotal: 27 * Joule},
		},
		{
			// Power is set so this row fails if the guard reads Power instead of ActivePower.
			name:             "no active power keeps the total",
			nodeZoneUsage:    NodeUsage{Power: 50 * Watt, activeEnergy: 8 * Joule},
			prevEnergy:       25 * Joule,
			cpuTimeDelta:     25,
			nodeCPUTimeDelta: 100,
			want:             Usage{EnergyTotal: 25 * Joule},
		},
		{
			name:             "no active energy keeps the total",
			nodeZoneUsage:    NodeUsage{Power: 50 * Watt, ActivePower: 20 * Watt},
			prevEnergy:       25 * Joule,
			cpuTimeDelta:     25,
			nodeCPUTimeDelta: 100,
			want:             Usage{EnergyTotal: 25 * Joule},
		},
		{
			name:             "no process CPU time on the node keeps the total",
			nodeZoneUsage:    active,
			prevEnergy:       25 * Joule,
			cpuTimeDelta:     25,
			nodeCPUTimeDelta: 0,
			want:             Usage{EnergyTotal: 25 * Joule},
		},
		{
			name:             "ignores the zone's total power",
			nodeZoneUsage:    NodeUsage{ActivePower: 20 * Watt, activeEnergy: 8 * Joule},
			prevEnergy:       25 * Joule,
			cpuTimeDelta:     25,
			nodeCPUTimeDelta: 100,
			want:             Usage{Power: 5 * Watt, EnergyTotal: 27 * Joule},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := attributeZone(tt.nodeZoneUsage, tt.prevEnergy, tt.cpuTimeDelta, tt.nodeCPUTimeDelta)
			assert.Equal(t, tt.want, got)
		})
	}
}
