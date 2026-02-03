// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package e2e_k8s

import (
	"context"
	"fmt"
	"math"
	"testing"

	"github.com/stretchr/testify/require"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

// powerTolerance is the acceptable tolerance for power comparisons (10%)
// Using higher tolerance for k8s tests due to timing differences between metric collection
const powerTolerance = 0.10

// absolutePowerTolerance is the minimum absolute tolerance in watts
const absolutePowerTolerance = 0.001

// TestPodPowerEqualsContainerSum verifies: Pod Watts = Σ(Container Watts in Pod)
func TestPodPowerEqualsContainerSum(t *testing.T) {
	feature := features.New("pod-equals-container-sum").
		Assess("pod power equals sum of its containers", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			snapshot := takeSnapshot(t)

			podWatts := snapshot.GetAllWithName("kepler_pod_cpu_watts")
			require.NotEmpty(t, podWatts, "Should have pod power metrics")

			containerWatts := snapshot.GetAllWithName("kepler_container_cpu_watts")

			// Collect all zones from pod metrics
			zones := make(map[string]bool)
			for _, m := range podWatts {
				zones[m.Labels["zone"]] = true
			}

			verified := 0
			for zone := range zones {
				// Build pod power map for this zone (only pods with power > 0)
				podPowerByID := make(map[string]float64)
				for _, m := range podWatts {
					if m.Labels["state"] == "running" && m.Labels["zone"] == zone && m.Value > 0 {
						podPowerByID[m.Labels["pod_id"]] = m.Value
					}
				}

				// Build container sum map for this zone
				containerSumByPodID := make(map[string]float64)
				for _, m := range containerWatts {
					if m.Labels["state"] == "running" && m.Labels["zone"] == zone {
						if podID := m.Labels["pod_id"]; podID != "" {
							containerSumByPodID[podID] += m.Value
						}
					}
				}

				// Verify invariant for each pod in this zone
				for podID, podPower := range podPowerByID {
					containerSum, ok := containerSumByPodID[podID]
					if !ok {
						continue
					}

					t.Logf("Zone %s, Pod %s: pod_power=%.4f W, container_sum=%.4f W",
						zone, truncateID(podID), podPower, containerSum)
					assertWithinTolerance(t, podPower, containerSum, powerTolerance, absolutePowerTolerance,
						"Zone %s, Pod %s: power should equal Σ(container powers)", zone, truncateID(podID))
					verified++
				}
			}

			require.Greater(t, verified, 0, "Should verify at least one pod with power > 0")
			t.Logf("Verified %d pod-zone combinations: power = Σ(containers)", verified)

			return ctx
		}).
		Feature()

	testenv.Test(t, feature)
}

// TestContainerPowerEqualsProcessSum verifies: Container Watts = Σ(Process Watts in Container)
func TestContainerPowerEqualsProcessSum(t *testing.T) {
	feature := features.New("container-equals-process-sum").
		Assess("container power equals sum of its processes", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			snapshot := takeSnapshot(t)

			containerWatts := snapshot.GetAllWithName("kepler_container_cpu_watts")
			require.NotEmpty(t, containerWatts, "Should have container power metrics")

			processWatts := snapshot.GetAllWithName("kepler_process_cpu_watts")
			require.NotEmpty(t, processWatts, "Should have process power metrics")

			// Collect all zones from container metrics
			zones := make(map[string]bool)
			for _, m := range containerWatts {
				zones[m.Labels["zone"]] = true
			}

			verified := 0
			for zone := range zones {
				// Build container power map for this zone (only containers with power > 0)
				containerPowerByID := make(map[string]float64)
				for _, m := range containerWatts {
					if m.Labels["state"] == "running" && m.Labels["zone"] == zone && m.Value > 0 {
						containerPowerByID[m.Labels["container_id"]] = m.Value
					}
				}

				// Build process sum map for this zone
				processSumByContainerID := make(map[string]float64)
				for _, m := range processWatts {
					if m.Labels["state"] == "running" && m.Labels["zone"] == zone {
						if containerID := m.Labels["container_id"]; containerID != "" {
							processSumByContainerID[containerID] += m.Value
						}
					}
				}

				// Verify invariant for each container in this zone
				for containerID, containerPower := range containerPowerByID {
					processSum, ok := processSumByContainerID[containerID]
					if !ok {
						continue
					}

					t.Logf("Zone %s, Container %s: container_power=%.4f W, process_sum=%.4f W",
						zone, truncateID(containerID), containerPower, processSum)
					assertWithinTolerance(t, containerPower, processSum, powerTolerance, absolutePowerTolerance,
						"Zone %s, Container %s: power should equal Σ(process powers)", zone, truncateID(containerID))
					verified++
				}
			}

			require.Greater(t, verified, 0, "Should verify at least one container with power > 0")
			t.Logf("Verified %d container-zone combinations: power = Σ(processes)", verified)

			return ctx
		}).
		Feature()

	testenv.Test(t, feature)
}

// truncateID safely truncates an ID for display (avoids panic on short IDs)
func truncateID(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}

// assertWithinTolerance checks if actual is within tolerance of expected
func assertWithinTolerance(t *testing.T, expected, actual, percentTolerance, absTolerance float64, msgAndArgs ...any) {
	t.Helper()

	diff := math.Abs(expected - actual)
	tolerance := math.Max(math.Abs(expected)*percentTolerance, absTolerance)

	if diff > tolerance {
		prefix := ""
		if len(msgAndArgs) > 0 {
			if format, ok := msgAndArgs[0].(string); ok && len(msgAndArgs) > 1 {
				prefix = fmt.Sprintf(format, msgAndArgs[1:]...) + ": "
			} else if s, ok := msgAndArgs[0].(string); ok {
				prefix = s + ": "
			}
		}
		t.Errorf("%sexpected %.6f, got %.6f (diff: %.6f, tolerance: %.6f)",
			prefix, expected, actual, diff, tolerance)
	}
}
