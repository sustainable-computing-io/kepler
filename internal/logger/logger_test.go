// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package logger

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSetupLogger(t *testing.T) {
	tests := []struct {
		name     string
		format   string
		logLevel string

		shouldLogInfo bool // indicate if info should be logged or not
		expectPanic   bool
	}{{
		name:          "json format debug level",
		format:        "json",
		logLevel:      "debug",
		shouldLogInfo: true,
	}, {
		name:          "json format info level",
		format:        "json",
		logLevel:      "info",
		shouldLogInfo: true,
	}, {
		name:          "json format warn level",
		format:        "json",
		logLevel:      "warn",
		shouldLogInfo: false,
	}, {
		name:          "text format info level",
		format:        "text",
		logLevel:      "info",
		shouldLogInfo: true,
	}, {
		name:          "text format error level",
		format:        "text",
		logLevel:      "warn",
		shouldLogInfo: false,
	}, {
		name:          "text format error level",
		format:        "text",
		logLevel:      "error",
		shouldLogInfo: false,
	}, {
		name:        "invalid format panics",
		format:      "invalid",
		logLevel:    "info",
		expectPanic: true,
	}}

	// This test setup up logger in various formats and log levels
	// and checks if the log message at INFO is logged or not

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.expectPanic {
				assert.Panics(t, func() {
					_ = New(tt.logLevel, tt.format, os.Stderr)
				}, "Expected setupLogger to panic with invalid format")
				//
				return
			}

			// Logger writes to stderr, so, redirect stderr to a buffer and
			// restore it at the end
			stderrOrig := os.Stderr
			r, w, _ := os.Pipe()
			os.Stderr = w

			logger := New(tt.logLevel, tt.format, os.Stderr)
			logger.Info("test message", "key", "value")

			// Restore stdout
			assert.NoError(t, w.Close())
			os.Stderr = stderrOrig

			// read stdout to string
			var outBuffer bytes.Buffer
			_, err := outBuffer.ReadFrom(r)
			assert.NoError(t, err, "Failed to read output")

			output := outBuffer.String()

			if tt.shouldLogInfo {
				assert.Contains(t, output, "test message", "Expected log message not found in output")
			} else {
				assert.NotContains(t, output, "test message", "Unexpected log message found in output")
			}

			// text format -> verify source path is shortened
			messageLogged := strings.Contains(output, "test message")
			if tt.format == "text" && messageLogged {
				// ensure source path was transformed
				assert.NotContains(t, output, "/home/user/",
					"Source path was not shortened as expected: %s", output)
			}

			// JSON format -> verify the structure
			if tt.format == "json" && messageLogged {
				logParts := map[string]any{}
				err := json.Unmarshal(outBuffer.Bytes(), &logParts)
				assert.NoError(t, err, "Failed to parse JSON log")

				assert.Contains(t, logParts, "time", "JSON log: missing 'time' field")
				assert.Contains(t, logParts, "msg", "JSON log missing 'msg' field")
				assert.Equal(t, "test message", logParts["msg"], "JSON log: incorrect 'msg' value")
				assert.Contains(t, logParts, "key", "JSON log: missing 'key' field")
				assert.Equal(t, "value", logParts["key"], "JSON log: incorrect 'key' value")
			}
		})
	}
}

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		name     string
		level    string
		expected slog.Level
	}{
		{
			name:     "debug level",
			level:    "debug",
			expected: slog.LevelDebug,
		},
		{
			name:     "info level",
			level:    "info",
			expected: slog.LevelInfo,
		},
		{
			name:     "warn level",
			level:    "warn",
			expected: slog.LevelWarn,
		},
		{
			name:     "error level",
			level:    "error",
			expected: slog.LevelError,
		},
		{
			name:     "invalid level defaults to info",
			level:    "invalid",
			expected: slog.LevelInfo,
		},
		{
			name:     "empty level defaults to info",
			level:    "",
			expected: slog.LevelInfo,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseLogLevel(tt.level)
			assert.Equal(t, tt.expected, result, "parseLogLevel(%q) = %v, want %v", tt.level, result, tt.expected)
		})
	}
}
