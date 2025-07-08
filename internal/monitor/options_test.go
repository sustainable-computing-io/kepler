// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package monitor

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestWithMaxTerminated tests the WithMaxTerminated option function
func TestWithMaxTerminated(t *testing.T) {
	tests := []struct {
		name     string
		maxValue int
	}{
		{"zero", 0},
		{"positive", 100},
		{"unlimited", -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := DefaultOpts()
			option := WithMaxTerminated(tt.maxValue)
			option(&opts)
			assert.Equal(t, tt.maxValue, opts.maxTerminated)
		})
	}
}
