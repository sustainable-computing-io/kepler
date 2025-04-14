// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package collectors

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
)

func TestDescribe(t *testing.T) {
	collector := NewBuildInfoCollector()
	ch := make(chan *prometheus.Desc, 1)
	collector.Describe(ch)
	assert.Len(t, ch, 1, "expected one metric description")
}

func TestCollect(t *testing.T) {
	// Create collector
	collector := NewBuildInfoCollector()

	// Create a channel for metrics
	ch := make(chan prometheus.Metric, 1)

	// Collect metrics
	collector.Collect(ch)

	// Verify we got one metric
	assert.Len(t, ch, 1, "should have received exactly one metric")

	// Get the metric
	metric := <-ch

	// Verify metric description
	desc := metric.Desc().String()
	assert.Contains(t, desc, "kepler_build_info")
	assert.Contains(t, desc, "arch")
	assert.Contains(t, desc, "branch")
	assert.Contains(t, desc, "revision")
	assert.Contains(t, desc, "version")
	assert.Contains(t, desc, "goversion")
}
