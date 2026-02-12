// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package e2e_k8s

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

// TestPodMetricsPresent verifies that pod power metrics exist
func TestPodMetricsPresent(t *testing.T) {
	feature := features.New("pod-metrics-present").
		Assess("kepler_pod_cpu_joules_total exists", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			snapshot := takeSnapshot(t)
			assert.True(t, snapshot.HasMetric("kepler_pod_cpu_joules_total"),
				"kepler_pod_cpu_joules_total metric should exist")
			return ctx
		}).
		Assess("kepler_pod_cpu_watts exists", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			snapshot := takeSnapshot(t)
			assert.True(t, snapshot.HasMetric("kepler_pod_cpu_watts"),
				"kepler_pod_cpu_watts metric should exist")
			return ctx
		}).
		Feature()

	testenv.Test(t, feature)
}

// TestPodMetricsHaveRequiredLabels verifies pod metrics have all required labels
func TestPodMetricsHaveRequiredLabels(t *testing.T) {
	// Required labels for pod metrics based on power_collector.go
	requiredLabels := []string{
		"pod_id",
		"pod_name",
		"pod_namespace",
		"state",
		"zone",
	}

	feature := features.New("pod-metrics-labels").
		Assess("kepler_pod_cpu_joules_total has required labels", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			snapshot := takeSnapshot(t)
			metrics := snapshot.GetAllWithName("kepler_pod_cpu_joules_total")
			// Pod metrics must exist - Kepler itself and system pods always exist in a K8s cluster
			require.NotEmpty(t, metrics,
				"kepler_pod_cpu_joules_total metric should exist")

			// Check first metric has all required labels
			m := metrics[0]
			for _, label := range requiredLabels {
				assert.Contains(t, m.Labels, label,
					"kepler_pod_cpu_joules_total should have %s label", label)
			}

			t.Logf("Pod metrics have %d label keys: %v", len(m.Labels), labelKeys(m.Labels))
			return ctx
		}).
		Assess("kepler_pod_cpu_watts has required labels", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			snapshot := takeSnapshot(t)
			metrics := snapshot.GetAllWithName("kepler_pod_cpu_watts")
			require.NotEmpty(t, metrics,
				"kepler_pod_cpu_watts metric should exist")

			m := metrics[0]
			for _, label := range requiredLabels {
				assert.Contains(t, m.Labels, label,
					"kepler_pod_cpu_watts should have %s label", label)
			}
			return ctx
		}).
		Feature()

	testenv.Test(t, feature)
}

// TestPodMetricsNonNegative verifies all pod metric values are >= 0
func TestPodMetricsNonNegative(t *testing.T) {
	feature := features.New("pod-metrics-non-negative").
		Assess("kepler_pod_cpu_joules_total values are non-negative", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			snapshot := takeSnapshot(t)
			metrics := snapshot.GetAllWithName("kepler_pod_cpu_joules_total")
			for _, m := range metrics {
				assert.GreaterOrEqual(t, m.Value, float64(0),
					"Pod joules should be >= 0 (pod_name=%s)", m.Labels["pod_name"])
			}

			t.Logf("Verified %d pod joules metrics are non-negative", len(metrics))
			return ctx
		}).
		Assess("kepler_pod_cpu_watts values are non-negative", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			snapshot := takeSnapshot(t)
			metrics := snapshot.GetAllWithName("kepler_pod_cpu_watts")
			for _, m := range metrics {
				assert.GreaterOrEqual(t, m.Value, float64(0),
					"Pod watts should be >= 0 (pod_name=%s)", m.Labels["pod_name"])
			}

			t.Logf("Verified %d pod watts metrics are non-negative", len(metrics))
			return ctx
		}).
		Feature()

	testenv.Test(t, feature)
}
