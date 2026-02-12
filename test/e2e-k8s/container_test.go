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

// TestContainerMetricsPresent verifies that container power metrics exist
func TestContainerMetricsPresent(t *testing.T) {
	feature := features.New("container-metrics-present").
		Assess("kepler_container_cpu_joules_total exists", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			snapshot := takeSnapshot(t)
			assert.True(t, snapshot.HasMetric("kepler_container_cpu_joules_total"),
				"kepler_container_cpu_joules_total metric should exist")
			return ctx
		}).
		Assess("kepler_container_cpu_watts exists", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			snapshot := takeSnapshot(t)
			assert.True(t, snapshot.HasMetric("kepler_container_cpu_watts"),
				"kepler_container_cpu_watts metric should exist")
			return ctx
		}).
		Feature()

	testenv.Test(t, feature)
}

// TestContainerMetricsHaveRequiredLabels verifies container metrics have all required labels
func TestContainerMetricsHaveRequiredLabels(t *testing.T) {
	// Required labels for container metrics based on power_collector.go
	requiredLabels := []string{
		"container_id",
		"container_name",
		"runtime",
		"state",
		"zone",
		"pod_id",
	}

	feature := features.New("container-metrics-labels").
		Assess("kepler_container_cpu_joules_total has required labels", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			snapshot := takeSnapshot(t)
			metrics := snapshot.GetAllWithName("kepler_container_cpu_joules_total")
			// Container metrics must exist - Kepler itself and system pods always run as containers
			require.NotEmpty(t, metrics,
				"kepler_container_cpu_joules_total metric should exist")

			m := metrics[0]
			for _, label := range requiredLabels {
				assert.Contains(t, m.Labels, label,
					"kepler_container_cpu_joules_total should have %s label", label)
			}

			t.Logf("Container metrics have %d label keys: %v", len(m.Labels), labelKeys(m.Labels))
			return ctx
		}).
		Assess("kepler_container_cpu_watts has required labels", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			snapshot := takeSnapshot(t)
			metrics := snapshot.GetAllWithName("kepler_container_cpu_watts")
			require.NotEmpty(t, metrics,
				"kepler_container_cpu_watts metric should exist")

			m := metrics[0]
			for _, label := range requiredLabels {
				assert.Contains(t, m.Labels, label,
					"kepler_container_cpu_watts should have %s label", label)
			}
			return ctx
		}).
		Feature()

	testenv.Test(t, feature)
}

// TestContainerMetricsNonNegative verifies all container metric values are >= 0
func TestContainerMetricsNonNegative(t *testing.T) {
	feature := features.New("container-metrics-non-negative").
		Assess("kepler_container_cpu_joules_total values are non-negative", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			snapshot := takeSnapshot(t)
			metrics := snapshot.GetAllWithName("kepler_container_cpu_joules_total")
			for _, m := range metrics {
				assert.GreaterOrEqual(t, m.Value, float64(0),
					"Container joules should be >= 0 (container_id=%s)", m.Labels["container_id"])
			}

			t.Logf("Verified %d container joules metrics are non-negative", len(metrics))
			return ctx
		}).
		Assess("kepler_container_cpu_watts values are non-negative", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			snapshot := takeSnapshot(t)
			metrics := snapshot.GetAllWithName("kepler_container_cpu_watts")
			for _, m := range metrics {
				assert.GreaterOrEqual(t, m.Value, float64(0),
					"Container watts should be >= 0 (container_id=%s)", m.Labels["container_id"])
			}

			t.Logf("Verified %d container watts metrics are non-negative", len(metrics))
			return ctx
		}).
		Feature()

	testenv.Test(t, feature)
}

// labelKeys returns the keys from a map
func labelKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
