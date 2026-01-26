// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package device

import "strings"

// chipPairingRule defines how voltage and current sensors should be paired for a specific chip.
// The hwmon ABI defines sensor types independently - index numbers are per-type.
// Whether in{N} pairs with curr{N} depends on how the chip driver registers its channels.
type chipPairingRule struct {
	// voltageIndex maps to currentIndex for explicit pairings
	// Key: voltage sensor index (in{N}), Value: current sensor index (curr{N})
	pairings map[int]int
	// skipVoltages lists voltage indices to skip (shunt voltages, aux inputs, etc.)
	skipVoltages []int
	// skipCurrents lists current indices to skip (current-only channels, per-phase, etc.)
	skipCurrents []int
	// useSameIndex indicates this chip uses same-index pairing (in{N} ↔ curr{N})
	// If true, pairings map is ignored and same-index matching is used
	useSameIndex bool
}

// knownChipPairings maps chip driver names (from hwmon "name" file) to their pairing rules.
// This table covers the most commonly deployed power monitoring hardware.
// Reference: Linux kernel hwmon driver documentation
var knownChipPairings = map[string]chipPairingRule{
	// INA Family (Texas Instruments)
	// INA3221: 3-channel monitor
	// in1-in3 = bus voltages, in4-in6 = shunt voltages, in7 = sum of shunts
	// curr1-curr3 = channel currents
	"ina3221": {
		useSameIndex: true,
		skipVoltages: []int{4, 5, 6, 7}, // Shunt voltages and sum - no current pair
	},
	// INA226/INA219/INA209/INA238: Single-channel monitors
	// in0 = shunt voltage, in1 = bus voltage, curr1 = current
	"ina226": {
		pairings:     map[int]int{1: 1}, // in1 ↔ curr1
		skipVoltages: []int{0},          // in0 is shunt voltage
	},
	"ina219": {
		pairings:     map[int]int{1: 1},
		skipVoltages: []int{0},
	},
	"ina209": {
		pairings:     map[int]int{1: 1},
		skipVoltages: []int{0},
	},
	"ina238": {
		pairings:     map[int]int{1: 1},
		skipVoltages: []int{0},
	},
	// INA260: Integrated shunt resistor, no shunt voltage output
	"ina260": {
		pairings: map[int]int{1: 1},
	},
	// INA233: PMBus-based
	"ina233": {
		pairings:     map[int]int{1: 1},
		skipVoltages: []int{2}, // in2 is shunt voltage
	},

	// LTC Family (Analog Devices / Linear Technology)
	// LTC2945: in1 = VIN, in2 = ADIN auxiliary
	"ltc2945": {
		pairings:     map[int]int{1: 1},
		skipVoltages: []int{2}, // Auxiliary ADC
	},
	// LTC2947: in0 = voltage, curr1 = current
	"ltc2947": {
		pairings: map[int]int{0: 1},
	},
	// LTC4260/LTC4261: Hot-swap controllers
	"ltc4260": {
		pairings:     map[int]int{1: 1},
		skipVoltages: []int{2}, // Auxiliary ADC
	},
	"ltc4261": {
		pairings:     map[int]int{1: 1},
		skipVoltages: []int{2}, // Auxiliary ADC
	},
	// LTC2992: 2 power channels
	// in0/in1 = ch1 (bus/delta-sense), in2/in3 = ch2, in4/in5 = GPIO
	"ltc2992": {
		pairings:     map[int]int{0: 1, 2: 2}, // in0→curr1, in2→curr2
		skipVoltages: []int{1, 3, 4, 5},       // Delta-sense and GPIO
	},
	// LTC4282: Hot-swap with many aux voltages
	"ltc4282": {
		pairings:     map[int]int{1: 1},
		skipVoltages: []int{2, 3, 4}, // VDD, Vsrc-Vdrain, GPIO
	},

	// ADM Family (Analog Devices - PMBus hot-swap controllers)
	// All use in1 = VIN, in2 = VOUT, curr1 = current
	"adm1275": {
		pairings: map[int]int{1: 1},
	},
	"adm1276": {
		pairings: map[int]int{1: 1},
	},
	"adm1278": {
		pairings: map[int]int{1: 1},
	},
	"adm1293": {
		pairings: map[int]int{1: 1},
	},

	// MAX Family (Maxim / Analog Devices - PMBus)
	// MAX20730/MAX20751: EXCEPTION - curr1 is output current, pairs with VOUT (in2)
	"max20730": {
		pairings: map[int]int{2: 1}, // in2 (VOUT) ↔ curr1 (IOUT)
	},
	"max20751": {
		pairings: map[int]int{2: 1}, // in2 (VOUT) ↔ curr1 (IOUT)
	},
	// MAX34440: Multi-channel with extra current-only channels
	"max34440": {
		useSameIndex: true,
		skipCurrents: []int{7, 8}, // Current-only monitoring channels
	},
	// MAX34451: Highly configurable, same-index
	"max34451": {
		useSameIndex: true,
	},

	// TPS Family (Texas Instruments - PMBus regulators)
	"tps40422": {
		useSameIndex: true,
	},
	"tps53679": {
		useSameIndex: true,
	},
	"tps546d24": {
		useSameIndex: true,
	},

	// Other PMBus-Based Drivers
	"ir35221": {
		useSameIndex: true,
	},
	"xdpe12284": {
		useSameIndex: true,
	},
	// MP2975: in1 ↔ curr1, curr2+ are per-phase output currents
	"mp2975": {
		pairings:     map[int]int{1: 1},
		skipCurrents: []int{2, 3, 4, 5, 6, 7, 8}, // Per-phase currents
	},
	// Generic PMBus: same-index per page
	"pmbus": {
		useSameIndex: true,
	},
	// UCD9000 series: primarily voltage sequencer, varies by variant
	"ucd9000": {
		useSameIndex: true,
	},
	"ucd90320": {
		useSameIndex: true,
	},
}

// shouldSkipVoltage returns true if the voltage index should be skipped for this chip
func (r chipPairingRule) shouldSkipVoltage(idx int) bool {
	for _, skip := range r.skipVoltages {
		if skip == idx {
			return true
		}
	}
	return false
}

// shouldSkipCurrent returns true if the current index should be skipped for this chip
func (r chipPairingRule) shouldSkipCurrent(idx int) bool {
	for _, skip := range r.skipCurrents {
		if skip == idx {
			return true
		}
	}
	return false
}

// getChipPairingRule returns the pairing rule for a chip, or nil if unknown
func getChipPairingRule(chipName string) *chipPairingRule {
	// Normalize chip name for lookup
	normalized := strings.ToLower(strings.TrimSpace(chipName))
	if rule, ok := knownChipPairings[normalized]; ok {
		return &rule
	}
	return nil
}
