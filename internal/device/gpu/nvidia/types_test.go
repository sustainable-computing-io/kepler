// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package nvidia

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestComputeMode_String(t *testing.T) {
	tests := []struct {
		name     string
		mode     ComputeMode
		expected string
	}{
		{
			name:     "default mode",
			mode:     ComputeModeDefault,
			expected: "default",
		},
		{
			name:     "exclusive thread mode",
			mode:     ComputeModeExclusiveThread,
			expected: "exclusive-thread",
		},
		{
			name:     "exclusive process mode",
			mode:     ComputeModeExclusiveProcess,
			expected: "exclusive-process",
		},
		{
			name:     "prohibited mode",
			mode:     ComputeModeProhibited,
			expected: "prohibited",
		},
		{
			name:     "unknown mode",
			mode:     ComputeMode(99),
			expected: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.mode.String()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestComputeModeConstants(t *testing.T) {
	// Verify constant values match NVML definitions
	assert.Equal(t, ComputeMode(0), ComputeModeDefault)
	assert.Equal(t, ComputeMode(1), ComputeModeExclusiveThread)
	assert.Equal(t, ComputeMode(2), ComputeModeExclusiveProcess)
	assert.Equal(t, ComputeMode(3), ComputeModeProhibited)
}
