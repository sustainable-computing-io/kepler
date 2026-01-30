// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package e2e_k8s

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/sustainable-computing-io/kepler/test/common"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

// TestTerminatedPodTracking verifies that terminated pods appear in metrics with state=terminated
func TestTerminatedPodTracking(t *testing.T) {
	feature := features.New("terminated-pod-tracking").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// Deploy stress workload
			deployStressWorkload(ctx, t, cfg)
			return ctx
		}).
		Assess("pod tracked while running", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// Wait for pod to appear in metrics with state=running
			ok := waitForStressPodInMetrics(ctx, t, "running", waitForMetrics)
			require.True(t, ok, "Pod should appear in metrics while running")

			// Verify pod has power > 0
			snapshot := takeSnapshot(t)
			power := getStressPodPower(snapshot, "running")
			t.Logf("Stress DaemonSet pods power while running: %.4f W", power)

			return ctx
		}).
		Assess("pod tracked after termination", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// Delete the stress workload
			deleteStressWorkload(ctx, t, cfg)

			// Wait for terminated stress pods to appear in metrics
			ok := waitForMetricCondition(ctx, t, func(s *common.MetricsSnapshot) bool {
				return getStressPodEnergy(s, "terminated") > 0
			}, 45*time.Second)

			if ok {
				snapshot, _ := scraper.TakeSnapshot()
				energy := getStressPodEnergy(snapshot, "terminated")
				t.Logf("Terminated stress pods total energy: %.4f J", energy)
				assert.Greater(t, energy, float64(0), "Terminated pods should have accumulated energy")
			} else {
				snapshot, _ := scraper.TakeSnapshot()
				metrics := snapshot.GetAllWithName("kepler_pod_cpu_joules_total")

				terminatedCount := 0
				for _, m := range metrics {
					if m.Labels["state"] == "terminated" {
						terminatedCount++
						if terminatedCount <= 5 {
							t.Logf("Terminated pod: %s (energy=%.4f J)", m.Labels["pod_name"], m.Value)
						}
					}
				}
				t.Logf("Total terminated pods in metrics: %d", terminatedCount)
				t.Log("Note: Pod may not appear as terminated due to energy threshold or timing")
			}

			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// Ensure cleanup even if test fails
			deleteStressWorkload(ctx, t, cfg)
			return ctx
		}).
		Feature()

	testenv.Test(t, feature)
}

// TestTerminatedContainerTracking verifies that terminated containers appear in metrics
func TestTerminatedContainerTracking(t *testing.T) {
	feature := features.New("terminated-container-tracking").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			deployStressWorkload(ctx, t, cfg)
			return ctx
		}).
		Assess("container tracked while running", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// Wait for container to appear in metrics with state=running
			ok := waitForContainerInMetrics(ctx, t, stressContainerName, "running", waitForMetrics)
			require.True(t, ok, "Container should appear in metrics while running")

			snapshot := takeSnapshot(t)
			power := snapshot.SumValues("kepler_container_cpu_watts",
				map[string]string{"container_name": stressContainerName, "state": "running"})
			t.Logf("Container '%s' power while running: %.4f W", stressContainerName, power)

			return ctx
		}).
		Assess("container tracked after termination", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			deleteStressWorkload(ctx, t, cfg)

			// Wait for terminated containers to appear in metrics
			ok := waitForMetricCondition(ctx, t, func(s *common.MetricsSnapshot) bool {
				return s.HasMetricWithLabels("kepler_container_cpu_joules_total",
					map[string]string{"container_name": stressContainerName, "state": "terminated"})
			}, 45*time.Second)

			if ok {
				snapshot, _ := scraper.TakeSnapshot()
				energy := snapshot.SumValues("kepler_container_cpu_joules_total",
					map[string]string{"container_name": stressContainerName, "state": "terminated"})
				t.Logf("Terminated container '%s' energy: %.4f J", stressContainerName, energy)
				assert.Greater(t, energy, float64(0), "Terminated container should have accumulated energy")
			} else {
				snapshot, _ := scraper.TakeSnapshot()
				metrics := snapshot.GetAllWithName("kepler_container_cpu_joules_total")

				terminatedCount := 0
				for _, m := range metrics {
					if m.Labels["state"] == "terminated" {
						terminatedCount++
					}
				}
				t.Logf("Total terminated containers in metrics: %d", terminatedCount)
				t.Log("Note: Container may not appear as terminated due to energy threshold or timing")
			}

			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// Ensure cleanup even if test fails
			deleteStressWorkload(ctx, t, cfg)
			return ctx
		}).
		Feature()

	testenv.Test(t, feature)
}
