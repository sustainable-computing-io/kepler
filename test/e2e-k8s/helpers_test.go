// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package e2e_k8s

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/sustainable-computing-io/kepler/test/common"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/klient/k8s/resources"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

const (
	// testNamespace is a dedicated namespace for test workloads
	testNamespace = "kepler-e2e-test"

	// stressDaemonSetName is the name of the stress test DaemonSet
	stressDaemonSetName = "kepler-e2e-stress"

	// stressContainerName is the name of the container in stress pods
	stressContainerName = "stress"

	// stressAppLabel is the label used to identify stress pods
	stressAppLabel = "kepler-e2e-test"

	// waitForDaemonSetReady is the timeout for DaemonSet to have ready pods
	waitForDaemonSetReady = 120 * time.Second

	// waitForDaemonSetDeleted is the timeout for DaemonSet deletion
	waitForDaemonSetDeleted = 60 * time.Second

	// waitForMetrics is the timeout for metrics to appear
	waitForMetrics = 60 * time.Second
)

// newStressDaemonSet returns a stress DaemonSet spec for testing.
func newStressDaemonSet() *appsv1.DaemonSet {
	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      stressDaemonSetName,
			Namespace: testNamespace,
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": stressAppLabel,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": stressAppLabel,
					},
				},
				Spec: corev1.PodSpec{
					Tolerations: []corev1.Toleration{
						{
							Key:      "node-role.kubernetes.io/control-plane",
							Operator: corev1.TolerationOpExists,
							Effect:   corev1.TaintEffectNoSchedule,
						},
						{
							Key:      "node-role.kubernetes.io/master",
							Operator: corev1.TolerationOpExists,
							Effect:   corev1.TaintEffectNoSchedule,
						},
					},
					Containers: []corev1.Container{
						{
							Name:    stressContainerName,
							Image:   "polinux/stress-ng:latest",
							Command: []string{"stress-ng", "--cpu", "1", "--cpu-load", "50", "--timeout", "0"},
						},
					},
				},
			},
		},
	}
}

// deployStressWorkload creates the stress DaemonSet and waits for pods to be running
func deployStressWorkload(ctx context.Context, t *testing.T, cfg *envconf.Config) {
	t.Helper()

	r, err := resources.New(cfg.Client().RESTConfig())
	if err != nil {
		t.Fatalf("Failed to create resources client: %v", err)
	}

	// Delete existing DaemonSet if present
	existingDS := newStressDaemonSet()
	_ = r.Delete(ctx, existingDS)
	_ = wait.For(conditions.New(r).ResourceDeleted(existingDS), wait.WithTimeout(10*time.Second))

	// Create the DaemonSet
	ds := newStressDaemonSet()
	if err := r.Create(ctx, ds); err != nil {
		t.Fatalf("Failed to create stress DaemonSet: %v", err)
	}

	// Wait for DaemonSet to have at least one ready pod
	err = wait.For(conditions.New(r).ResourceMatch(ds, func(object k8s.Object) bool {
		d, ok := object.(*appsv1.DaemonSet)
		if !ok {
			return false
		}
		return d.Status.NumberReady >= 1
	}), wait.WithTimeout(waitForDaemonSetReady))
	if err != nil {
		t.Fatalf("Stress DaemonSet not ready within timeout: %v", err)
	}

	// Get actual ready count for logging
	if err := r.Get(ctx, stressDaemonSetName, testNamespace, ds); err == nil {
		t.Logf("Stress DaemonSet deployed: %d/%d pods ready",
			ds.Status.NumberReady, ds.Status.DesiredNumberScheduled)
	}
}

// deleteStressWorkload removes the stress DaemonSet
func deleteStressWorkload(ctx context.Context, t *testing.T, cfg *envconf.Config) {
	t.Helper()

	r, err := resources.New(cfg.Client().RESTConfig())
	if err != nil {
		t.Logf("Warning: Failed to create resources client: %v", err)
		return
	}

	ds := newStressDaemonSet()
	if err := r.Delete(ctx, ds); err != nil {
		t.Logf("Warning: Failed to delete stress DaemonSet: %v", err)
		return
	}

	_ = wait.For(conditions.New(r).ResourceDeleted(ds), wait.WithTimeout(waitForDaemonSetDeleted))
	t.Logf("Stress DaemonSet deleted: %s/%s", testNamespace, stressDaemonSetName)
}

// waitForMetricCondition polls until the condition is met or timeout
func waitForMetricCondition(ctx context.Context, t *testing.T, check func(*common.MetricsSnapshot) bool, timeout time.Duration) bool {
	t.Helper()

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	err := common.WaitForCondition(ctx, 2*time.Second, func() bool {
		snapshot, err := scraper.TakeSnapshot()
		if err != nil {
			return false
		}
		return check(snapshot)
	})

	return err == nil
}

// takeSnapshot takes a metrics snapshot and fails the test on error.
func takeSnapshot(t *testing.T) *common.MetricsSnapshot {
	t.Helper()
	snapshot, err := scraper.TakeSnapshot()
	if err != nil {
		t.Fatalf("Failed to take metrics snapshot: %v", err)
	}
	return snapshot
}

// isStressPodMetric returns true if the metric belongs to a stress DaemonSet pod.
func isStressPodMetric(m common.Metric, state string) bool {
	return strings.HasPrefix(m.Labels["pod_name"], stressDaemonSetName) &&
		m.Labels["state"] == state
}

// waitForStressPodInMetrics waits for any stress DaemonSet pod to appear in metrics.
func waitForStressPodInMetrics(ctx context.Context, t *testing.T, state string, timeout time.Duration) bool {
	t.Helper()

	return waitForMetricCondition(ctx, t, func(s *common.MetricsSnapshot) bool {
		for _, m := range s.GetAllWithName("kepler_pod_cpu_watts") {
			if isStressPodMetric(m, state) {
				return true
			}
		}
		return false
	}, timeout)
}

// waitForContainerInMetrics waits for a container to appear in Kepler metrics with the specified state
func waitForContainerInMetrics(ctx context.Context, t *testing.T, containerName, state string, timeout time.Duration) bool {
	t.Helper()

	return waitForMetricCondition(ctx, t, func(s *common.MetricsSnapshot) bool {
		return s.HasMetricWithLabels("kepler_container_cpu_watts",
			map[string]string{"container_name": containerName, "state": state})
	}, timeout)
}

// getStressPodPower returns the total power of all stress DaemonSet pods
func getStressPodPower(s *common.MetricsSnapshot, state string) float64 {
	var total float64
	for _, m := range s.GetAllWithName("kepler_pod_cpu_watts") {
		if isStressPodMetric(m, state) {
			total += m.Value
		}
	}
	return total
}

// getStressPodEnergy returns the total energy of all stress DaemonSet pods
func getStressPodEnergy(s *common.MetricsSnapshot, state string) float64 {
	var total float64
	for _, m := range s.GetAllWithName("kepler_pod_cpu_joules_total") {
		if isStressPodMetric(m, state) {
			total += m.Value
		}
	}
	return total
}
