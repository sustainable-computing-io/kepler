// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package collector

import (
	"sync"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
)

func TestBuildInfo_Describe(t *testing.T) {
	collector := NewKeplerBuildInfoCollector()
	ch := make(chan *prometheus.Desc, 1)
	collector.Describe(ch)
	assert.Len(t, ch, 1, "expected one metric description")
}

func TestBuildInfo_Collect(t *testing.T) {
	// Create collector
	collector := NewKeplerBuildInfoCollector()

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

func TestBuildInfo_ParallelCollect(t *testing.T) {
	// Create collector
	collector := NewKeplerBuildInfoCollector()
	parallelCalls := 10

	// Create a shared channel for metrics
	ch := make(chan prometheus.Metric, parallelCalls)

	// WaitGroup to sync goroutines
	var wg sync.WaitGroup
	wg.Add(parallelCalls)

	// Run multiple collect calls in parallel to check for race conditions
	for i := 0; i < parallelCalls; i++ {
		go func() {
			defer wg.Done()
			// Collect metrics
			collector.Collect(ch)
		}()
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(ch)

	var metrics []prometheus.Metric
	for metric := range ch {
		assert.NotNil(t, metric, "metric should not be nil")
		metrics = append(metrics, metric)
	}

	assert.Len(t, metrics, parallelCalls, "should have received the correct number of metrics")
}
