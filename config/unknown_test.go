// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnknownFields(t *testing.T) {
	tt := []struct {
		name     string
		yaml     string
		expected []string
	}{{
		name: "known fields only",
		yaml: `
log:
  level: debug
monitor:
  interval: 5s
experimental:
  gpu:
    enabled: true
    idlePower: 30
`,
		expected: nil,
	}, {
		name: "unknown nested field",
		yaml: `
experimental:
  gpu:
    enabled: true
    dcgmEndpoints: http://127.0.0.1:9400/metrics
`,
		expected: []string{"dcgmEndpoints"},
	}, {
		name: "unknown top level field",
		yaml: `
log:
  level: debug
notASetting: 42
`,
		expected: []string{"notASetting"},
	}, {
		name: "the same unknown key in two sections is reported once",
		yaml: `
log:
  notAKey: debug
monitor:
  notAKey: 5s
`,
		expected: []string{"notAKey"},
	}, {
		name: "several unknown fields are reported once each",
		yaml: `
log:
  level: debug
  notAKey: debug
monitor:
  interval: 5s
  alsoNotAKey: 5s
`,
		expected: []string{"alsoNotAKey", "notAKey"},
	}}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := Load(strings.NewReader(tc.yaml))
			require.NoError(t, err, "unknown fields must not fail the load")
			assert.Equal(t, tc.expected, cfg.unknownFields)
			assert.Equal(t, tc.expected, unknownFields([]byte(tc.yaml)))
		})
	}
}

func TestLogUnknownFields(t *testing.T) {
	logs := &bytes.Buffer{}
	logger := slog.New(slog.NewTextHandler(logs, nil))

	cfg, err := Load(strings.NewReader("experimental:\n  gpu:\n    dcgmEndpoints: http://127.0.0.1:9400\n"))
	require.NoError(t, err)

	cfg.LogUnknownFields(logger)
	assert.Contains(t, logs.String(), "dcgmEndpoints")

	logs.Reset()
	DefaultConfig().LogUnknownFields(logger)
	assert.Empty(t, logs.String(), "a config with no unknown field must stay quiet")
}
