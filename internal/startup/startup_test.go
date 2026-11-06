// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package startup

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/utils/ptr"

	"github.com/sustainable-computing-io/kepler/config"
	"github.com/sustainable-computing-io/kepler/internal/logger"
)

func TestStartupDiagnosticsUseJSONAndStayOffStdout(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Log.Format = "json"
	cfg.Exporter.Stdout.Enabled = ptr.To(true)

	var logOut, stdout bytes.Buffer
	log := logger.New(cfg.Log.Level, cfg.Log.Format, &logOut)

	LogConfigLoaded(log, "/etc/kepler/config.yaml")
	LogVersionInfo(log)
	PrintConfigInfo(log, cfg, &logOut)

	assert.Empty(t, stdout.String(), "startup diagnostics must not write to stdout")

	lines := strings.Split(strings.TrimSpace(logOut.String()), "\n")
	require.Len(t, lines, 2)

	messages := make([]string, 0, len(lines))
	for _, line := range lines {
		var record map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &record), "log line is not valid JSON: %q", line)
		msg, ok := record["msg"].(string)
		require.True(t, ok, "log line has no msg field: %q", line)
		messages = append(messages, msg)
	}

	assert.Contains(t, messages, "Loaded configuration file")
	assert.Contains(t, messages, "Kepler version information")
}

func TestPrintConfigInfoBannerStaysOffStdout(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Log.Format = "text"
	cfg.Exporter.Stdout.Enabled = ptr.To(true)

	var logOut, stdout bytes.Buffer
	log := logger.New(cfg.Log.Level, cfg.Log.Format, &logOut)

	PrintConfigInfo(log, cfg, &logOut)

	assert.Empty(t, stdout.String(), "startup diagnostics must not write to stdout")
	assert.Contains(t, logOut.String(), "Configuration")
}
