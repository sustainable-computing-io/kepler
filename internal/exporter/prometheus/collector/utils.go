// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package collector

import (
	"regexp"
	"strings"
)

var (
	invalidMetricChars = regexp.MustCompile(`[^a-zA-Z0-9_:]`)
	validMetricChars   = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)
)

// SanitizeMetricName replaces invalid metric name characters with underscores
// and ensures the result is a valid Prometheus metric name.
// This is inspired by node_exporter's implementation.
func SanitizeMetricName(name string) string {
	// Replace invalid chars with underscores
	name = invalidMetricChars.ReplaceAllString(name, "_")

	// Ensure the name starts with a letter or underscore
	if !validMetricChars.MatchString(name) {
		name = "_" + name
	}

	// Replace multiple consecutive underscores with a single one
	name = strings.ReplaceAll(name, "__", "_")

	return name
}
