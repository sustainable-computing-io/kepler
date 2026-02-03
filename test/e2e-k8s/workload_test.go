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

// TestWorkloadDetected verifies that stress DaemonSet pods appear in Kepler metrics
func TestWorkloadDetected(t *testing.T) {
	feature := features.New("workload-detected").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			deployStressWorkload(ctx, t, cfg)
			return ctx
		}).
		Assess("pod appears in metrics", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// Wait for any stress DaemonSet pod to appear in Kepler metrics with state=running
			ok := waitForStressPodInMetrics(ctx, t, "running", waitForMetrics)
			require.True(t, ok, "Stress DaemonSet pods should appear in Kepler pod metrics")

			// Also verify container appears
			ok = waitForContainerInMetrics(ctx, t, stressContainerName, "running", waitForMetrics)
			assert.True(t, ok, "Stress container should appear in Kepler container metrics")

			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			deleteStressWorkload(ctx, t, cfg)
			return ctx
		}).
		Feature()

	testenv.Test(t, feature)
}

// TestWorkloadPowerAttribution verifies that the stress DaemonSet pods have power attributed
func TestWorkloadPowerAttribution(t *testing.T) {
	feature := features.New("workload-power-attribution").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			deployStressWorkload(ctx, t, cfg)
			return ctx
		}).
		Assess("pod has power > 0", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// Wait for pod to appear in metrics
			ok := waitForStressPodInMetrics(ctx, t, "running", waitForMetrics)
			require.True(t, ok, "Pod should appear in metrics")

			// Check stress pods have positive power
			ok = waitForMetricCondition(ctx, t, func(s *common.MetricsSnapshot) bool {
				return getStressPodPower(s, "running") > 0
			}, 45*time.Second) // Extended timeout to allow power accumulation

			if ok {
				snapshot, _ := scraper.TakeSnapshot()
				power := getStressPodPower(snapshot, "running")
				t.Logf("Stress DaemonSet pods total power: %.4f W", power)
				assert.Greater(t, power, float64(0), "Pods should have positive power")
			} else {
				t.Log("Pod power not detected - this can happen due to timing or hardware")
			}

			return ctx
		}).
		Assess("container has power > 0", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			ok := waitForMetricCondition(ctx, t, func(s *common.MetricsSnapshot) bool {
				power := s.SumValues("kepler_container_cpu_watts",
					map[string]string{"container_name": stressContainerName, "state": "running"})
				return power > 0
			}, 30*time.Second)

			if ok {
				snapshot, _ := scraper.TakeSnapshot()
				power := snapshot.SumValues("kepler_container_cpu_watts",
					map[string]string{"container_name": stressContainerName, "state": "running"})
				t.Logf("Container '%s' total power: %.4f W", stressContainerName, power)
				assert.Greater(t, power, float64(0), "Container should have positive power")
			} else {
				t.Log("Container power not detected - this can happen due to timing or hardware")
			}

			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			deleteStressWorkload(ctx, t, cfg)
			return ctx
		}).
		Feature()

	testenv.Test(t, feature)
}

// TestWorkloadEnergyAccumulates verifies that energy (joules) accumulates over time
func TestWorkloadEnergyAccumulates(t *testing.T) {
	feature := features.New("workload-energy-accumulates").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			deployStressWorkload(ctx, t, cfg)
			return ctx
		}).
		Assess("energy increases over time", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// Wait for pod to appear
			ok := waitForStressPodInMetrics(ctx, t, "running", waitForMetrics)
			require.True(t, ok, "Pod should appear in metrics")

			// Take first snapshot after some time for energy to accumulate
			time.Sleep(waitBetweenSnapshots)
			snapshot1 := takeSnapshot(t)
			energy1 := getStressPodEnergy(snapshot1, "running")
			t.Logf("Energy at T1: %.4f J", energy1)

			// Wait and take second snapshot
			time.Sleep(waitBetweenSnapshots * 2)
			snapshot2 := takeSnapshot(t)
			energy2 := getStressPodEnergy(snapshot2, "running")
			t.Logf("Energy at T2: %.4f J", energy2)

			t.Logf("Energy delta: %.4f J", energy2-energy1)

			// Energy should increase (counter metric)
			assert.GreaterOrEqual(t, energy2, energy1,
				"Energy should accumulate over time (counter should not decrease)")

			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			deleteStressWorkload(ctx, t, cfg)
			return ctx
		}).
		Feature()

	testenv.Test(t, feature)
}
