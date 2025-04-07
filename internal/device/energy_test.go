// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package device

import (
	"fmt"
	"math"
	"testing"
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

func TestEnergy_String(t *testing.T) {
	tests := []struct {
		name   string
		energy Energy
		want   string
	}{
		{"Zero", 0, "0.000000J"},
		{"Regular", 1_250_000, "1.250000J"},
		{"MaxUint64", Energy(math.MaxUint64), fmt.Sprintf("%fJ", float64(math.MaxUint64)/1_000_000)},
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
