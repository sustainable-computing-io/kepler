// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestCloneOwnsNestedConfiguration(t *testing.T) {
	original := DefaultConfig()
	original.Experimental = &Experimental{Hwmon: Hwmon{ChipRules: []ChipPairingRule{{Name: "test", Pairings: map[int]int{1: 2}, SkipVoltages: []int{3}, SkipCurrents: []int{4}}}}}
	copy := original.Clone()
	*copy.Kube.Enabled = true
	copy.Cpu.PreferredMeters[0] = "fake"
	copy.Web.ListenAddresses[0] = "changed"
	copy.Experimental.Hwmon.ChipRules[0].Pairings[1] = 9
	copy.Experimental.Hwmon.ChipRules[0].SkipVoltages[0] = 9
	copy.Experimental.Hwmon.ChipRules[0].SkipCurrents[0] = 9
	require.False(t, *original.Kube.Enabled)
	require.Equal(t, "rapl", original.Cpu.PreferredMeters[0])
	require.Equal(t, ":28282", original.Web.ListenAddresses[0])
	require.Equal(t, 2, original.Experimental.Hwmon.ChipRules[0].Pairings[1])
	require.Equal(t, []int{3}, original.Experimental.Hwmon.ChipRules[0].SkipVoltages)
	require.Equal(t, []int{4}, original.Experimental.Hwmon.ChipRules[0].SkipCurrents)
}
