// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package device

import (
	"fmt"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEnergy_Joules(t *testing.T) {
	tests := []struct {
		name   string
		energy Energy
		want   float64
	}{
		{"Zero", 0, 0.0},
		{"One", 1_000_000, 1.0},
		{"1.5 Joule", 1_500_000, 1.5},
		{"Maximum Value", math.MaxUint64, float64(math.MaxUint64) / 1_000_000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.energy.Joules()
			if got != tt.want {
				t.Errorf("Joules() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEnergy_MicroJoules(t *testing.T) {
	tests := []struct {
		name   string
		energy Energy
		want   uint64
	}{
		{"Zero", 0, 0.0},
		{"2 Million MicroJoules", 2_000_000, 2_000_000},
		{"Maximum value", math.MaxUint64, math.MaxUint64},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.energy.MicroJoules()
			if got != tt.want {
				t.Errorf("MicroJoules() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEnergy_MilliJoules(t *testing.T) {
	tests := []struct {
		name   string
		energy Energy
		want   float64
	}{
		{"Zero", 0, 0.0},
		{"2 Thousand MilliJoules", 2_000_000, 2_000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.energy.MilliJoules()
			assert.Equal(t, tt.want, got, "MilliJoules = %v, want %v", got, tt.want)
		})
	}

	// NOTE: cannot compare Max Milli joules directly as above due to float64 precision
	// E.g. {"Max MilliJoules", math.MaxFloat64 / 1000 * MilliWatt, math.MaxFloat64}, won't work
	// compute max milli Joules
	maxMilliJoules := Energy(math.MaxUint64 * MicroJoule).MilliJoules()
	assert.InDelta(t, math.MaxUint64/1_000, maxMilliJoules, 0.01)
}

func TestEnergy_String(t *testing.T) {
	tests := []struct {
		name   string
		energy Energy
		want   string
	}{
		{"Zero", 0, "0.00J"},
		{"Regular", 1_250_000, "1.25J"},
		{"MaxUint64", Energy(math.MaxUint64), fmt.Sprintf("%.2fJ", float64(math.MaxUint64)/1_000_000)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.energy.String()
			if got != tt.want {
				t.Errorf("String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPower_MicroWatts(t *testing.T) {
	tests := []struct {
		name  string
		power Power
		want  float64
	}{
		{"Zero", 0, 0.0},
		{"2 Million", 2_000_000, 2_000_000},
		{"Maximum value", math.MaxFloat64, math.MaxFloat64},

		{"Zero MicoWatt	", 0 * MicroWatt, 0.0},
		{"1 MicroWatt", 1 * MicroWatt, 1.0},
		{"Five MicroWatts", 5 * MicroWatt, 5.0},
		{"1.5 Watts", 1.5 * MicroWatt, 1.5},
		{"Maximum MicroWatts", math.MaxFloat64 * MicroWatt, math.MaxFloat64},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.power.MicroWatts()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestPower_MilliWatts(t *testing.T) {
	tests := []struct {
		name  string
		power Power
		want  float64
	}{
		{"Zero", 0, 0.0},
		{"Regular One", 1, 0.001},
		{"Regular 5", 5, 0.005},
		{"Regular 1000", 1000, 1.0},
		{"MaxFloat64", math.MaxFloat64, math.MaxFloat64 / 1000},

		{"MilliWatt", MilliWatt, 1.0},
		{"Zero MilliWatt", 0 * MilliWatt, 0.0},
		{"Five MilliWatt", 5 * MilliWatt, 5.0},
		{"1.5 MilliWatt", 1.5 * MilliWatt, 1.5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.power.MilliWatts()
			assert.Equal(t, tt.want, got)
		})
	}

	// NOTE: cannot compare Max Milli watt directly as above due to float64 precision
	// {"Max MilliWatts", math.MaxFloat64 / 1000 * MilliWatt, math.MaxFloat64},
	// compute max milli watts
	maxMilliWatts := Power(math.MaxFloat64 * MicroWatt).MilliWatts()
	assert.InDelta(t, math.MaxFloat64/1_000, maxMilliWatts, 0.0001)
}

func TestPower_Watts(t *testing.T) {
	tests := []struct {
		name  string
		power Power
		want  float64
	}{
		{"Zero", 0, 0.0},
		{"Regular One", 1, 0.000_001},
		{"Regular 5", 5, 0.000_005},
		{"Regular 1000", 1000, 0.001},
		{"MaxFloat64", math.MaxFloat64, math.MaxFloat64 / 1000_000},

		{"Zero Watt", 0, 0.0},
		{"One Watt", Watt, 1.0},
		{"Five Watt", 5 * Watt, 5.0},
		{"1.5 Watts", 1.5 * Watt, 1.5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.power.Watts()
			assert.Equal(t, tt.want, got)
		})
	}

	// compute max watts
	maxWatts := Power(math.MaxFloat64 * MicroWatt).Watts()
	assert.InDelta(t, math.MaxFloat64/1_000_000, maxWatts, 0.0001)
}

func TestPower_String(t *testing.T) {
	tests := []struct {
		name  string
		power Power
		want  string
	}{
		{"Zero", 0, "0.00W"},
		{"Regular", 1_250_000, "1.25W"},
		{"Watt", 1.25 * Watt, "1.25W"},
		{"MaxFloat64", Power(math.MaxFloat64), fmt.Sprintf("%.2fW", float64(math.MaxFloat64)/1_000_000)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.power.String()
			assert.Equal(t, tt.want, got)
		})
	}
}
