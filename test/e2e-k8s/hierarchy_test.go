// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package e2e_k8s

import (
	"context"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

// TestPodContainerHierarchy verifies that container metrics have correct pod_id
// that actually links to a valid pod
func TestPodContainerHierarchy(t *testing.T) {
	feature := features.New("pod-container-hierarchy").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			deployStressWorkload(ctx, t, cfg)
			return ctx
		}).
		Assess("containers have valid pod_id linking to actual pods", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// Wait for workload to appear
			ok := waitForStressPodInMetrics(ctx, t, "running", waitForMetrics)
			require.True(t, ok, "Stress pod should appear in metrics")

			ok = waitForContainerInMetrics(ctx, t, stressContainerName, "running", waitForMetrics)
			require.True(t, ok, "Stress container should appear in metrics")

			snapshot := takeSnapshot(t)

			// Get all stress pod IDs from pod metrics
			podMetrics := snapshot.GetAllWithName("kepler_pod_cpu_watts")
			var stressPodIDs []string
			for _, m := range podMetrics {
				if isStressPodMetric(m, "running") {
					podID := m.Labels["pod_id"]
					if podID != "" {
						stressPodIDs = append(stressPodIDs, podID)
						t.Logf("Found stress pod: name=%s, pod_id=%s",
							m.Labels["pod_name"], truncateID(podID))
					}
				}
			}
			require.NotEmpty(t, stressPodIDs, "Should find at least one stress pod with pod_id")

			containerMetrics := snapshot.GetAllWithName("kepler_container_cpu_watts")
			var matchedContainers int
			for _, m := range containerMetrics {
				if m.Labels["container_name"] == stressContainerName && m.Labels["state"] == "running" {
					containerPodID := m.Labels["pod_id"]

					t.Logf("Found stress container: container_id=%s, pod_id=%s",
						truncateID(m.Labels["container_id"]), truncateID(containerPodID))

					// This is the key assertion: container's pod_id should match an actual pod
					assert.True(t, slices.Contains(stressPodIDs, containerPodID),
						"Container pod_id=%s should match a stress pod", truncateID(containerPodID))
					matchedContainers++
				}
			}

			assert.Greater(t, matchedContainers, 0,
				"Should find at least one container linked to a stress pod")
			t.Logf("Verified %d containers have valid pod_id linkage", matchedContainers)

			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			deleteStressWorkload(ctx, t, cfg)
			return ctx
		}).
		Feature()

	testenv.Test(t, feature)
}
