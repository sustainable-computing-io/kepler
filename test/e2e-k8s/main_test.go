// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package e2e_k8s

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/sustainable-computing-io/kepler/test/common"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
	"sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/klient/k8s/resources"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

// Timing constants for k8s e2e tests
const (
	// waitForMetricsAvailable: wait for metrics to be available after port-forward
	waitForMetricsAvailable = 10 * time.Second

	// waitBetweenSnapshots: interval between metric snapshots
	waitBetweenSnapshots = 5 * time.Second

	// portForwardRetries: number of retries for port-forward setup
	portForwardRetries = 3

	// portForwardRetryDelay: delay between port-forward retries
	portForwardRetryDelay = 5 * time.Second
)

// testConfig holds test configuration from flags
var testConfig = struct {
	keplerNamespace string
	keplerService   string
	metricsPort     int
	localPort       int
}{
	keplerNamespace: "kepler",
	keplerService:   "kepler",
	metricsPort:     28282, // Kepler's metrics port inside the container
	localPort:       28284, // Local port for port-forwarding (avoid conflict with any running Kepler)
}

// Global test environment and scraper
var (
	testenv env.Environment
	scraper *common.MetricsScraper

	// stopChan is used to stop port-forward
	stopChan  chan struct{}
	closeOnce sync.Once // ensures stopChan is closed only once
)

func init() {
	flag.StringVar(&testConfig.keplerNamespace, "kepler.namespace", testConfig.keplerNamespace,
		"Namespace where Kepler is deployed")
	flag.StringVar(&testConfig.keplerService, "kepler.service", testConfig.keplerService,
		"Name of the Kepler service")
	flag.IntVar(&testConfig.metricsPort, "kepler.metrics-port", testConfig.metricsPort,
		"Port where Kepler exposes metrics inside the pod (default: 28282)")
	flag.IntVar(&testConfig.localPort, "kepler.local-port", testConfig.localPort,
		"Local port for port-forwarding (default: 28284)")
}

func TestMain(m *testing.M) {
	// Parse flags before creating environment
	flag.Parse()

	// Create e2e-framework environment
	testenv = env.New()

	// Setup: verify prerequisites, create test namespace, and start port-forward
	testenv.Setup(
		verifyKeplerRunning(),
		ensureTestNamespace(),
		setupPortForward(),
		setupMetricsScraper(),
		waitForMetricsReady(),
	)

	// Teardown: cleanup port-forward and test namespace
	testenv.Finish(
		cleanupPortForward(),
		cleanupTestNamespace(),
	)

	os.Exit(testenv.Run(m))
}

// verifyKeplerRunning checks that the Kepler DaemonSet is running
func verifyKeplerRunning() env.Func {
	return func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
		r, err := resources.New(cfg.Client().RESTConfig())
		if err != nil {
			return ctx, fmt.Errorf("failed to create resources client: %w", err)
		}

		ds := &appsv1.DaemonSet{}
		ds.Name = "kepler"
		ds.Namespace = testConfig.keplerNamespace

		// Wait for DaemonSet to exist and have ready pods
		err = wait.For(conditions.New(r).ResourceMatch(ds, func(object k8s.Object) bool {
			d, ok := object.(*appsv1.DaemonSet)
			if !ok {
				return false
			}
			return d.Status.NumberReady >= 1
		}), wait.WithTimeout(2*time.Minute), wait.WithContext(ctx))
		if err != nil {
			// Try to get the DaemonSet to provide better error message
			if getErr := r.Get(ctx, "kepler", testConfig.keplerNamespace, ds); getErr != nil {
				return ctx, fmt.Errorf("kepler DaemonSet not found in namespace %s: %w", testConfig.keplerNamespace, getErr)
			}
			return ctx, fmt.Errorf("kepler DaemonSet has no ready pods: %w", err)
		}

		return ctx, nil
	}
}

// setupPortForward starts port-forward
func setupPortForward() env.Func {
	return func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
		r, err := resources.New(cfg.Client().RESTConfig())
		if err != nil {
			return ctx, fmt.Errorf("failed to create resources client: %w", err)
		}

		// Find a Kepler pod to port-forward to
		var pods corev1.PodList
		if err := r.List(ctx, &pods,
			resources.WithLabelSelector("app.kubernetes.io/name=kepler"),
			resources.WithFieldSelector("status.phase=Running")); err != nil {
			return ctx, fmt.Errorf("failed to list kepler pods: %w", err)
		}

		if len(pods.Items) == 0 {
			return ctx, fmt.Errorf("no running kepler pods found")
		}

		keplerPod := &pods.Items[0]
		var lastErr error

		for attempt := 1; attempt <= portForwardRetries; attempt++ {
			// Setup port-forward using client-go
			restConfig := cfg.Client().RESTConfig()
			transport, upgrader, err := spdy.RoundTripperFor(restConfig)
			if err != nil {
				lastErr = fmt.Errorf("failed to create round tripper: %w", err)
				continue
			}

			path := fmt.Sprintf("/api/v1/namespaces/%s/pods/%s/portforward",
				keplerPod.Namespace, keplerPod.Name)
			hostURL, err := url.Parse(restConfig.Host)
			if err != nil {
				lastErr = fmt.Errorf("failed to parse host URL: %w", err)
				continue
			}
			hostURL.Path = path

			dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, http.MethodPost, hostURL)

			// Use local channel - only save to global on success
			localStopChan := make(chan struct{}, 1)
			readyChan := make(chan struct{})

			ports := []string{fmt.Sprintf("%d:%d", testConfig.localPort, testConfig.metricsPort)}
			pf, err := portforward.New(dialer, ports, localStopChan, readyChan, nil, nil)
			if err != nil {
				lastErr = fmt.Errorf("failed to create port-forwarder: %w", err)
				continue
			}

			// Start port-forward in background
			errChan := make(chan error, 1)
			go func() {
				errChan <- pf.ForwardPorts()
			}()

			// Wait for port-forward to be ready or error
			select {
			case <-readyChan:
				if waitForPortForwardReady(testConfig.localPort) {
					stopChan = localStopChan // Save successful channel for cleanup
					return ctx, nil
				}
				close(localStopChan) // Clean up failed port-forward
				lastErr = fmt.Errorf("port-forward not responding")
			case err := <-errChan:
				close(localStopChan)
				lastErr = fmt.Errorf("port-forward error: %w", err)
			case <-time.After(30 * time.Second):
				close(localStopChan)
				lastErr = fmt.Errorf("port-forward timed out")
			}

			if attempt < portForwardRetries {
				time.Sleep(portForwardRetryDelay * time.Duration(attempt))
			}
		}

		return ctx, fmt.Errorf("port-forward failed after %d attempts: %w", portForwardRetries, lastErr)
	}
}

// waitForPortForwardReady waits for the port-forward to be accessible
func waitForPortForwardReady(port int) bool {
	url := fmt.Sprintf("http://localhost:%d/metrics", port)
	testScraper := common.NewMetricsScraper(url)

	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := testScraper.Scrape(); err == nil {
			return true
		}
		time.Sleep(500 * time.Millisecond)
	}
	return false
}

// setupMetricsScraper initializes the metrics scraper
func setupMetricsScraper() env.Func {
	return func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
		url := fmt.Sprintf("http://localhost:%d/metrics", testConfig.localPort)
		scraper = common.NewMetricsScraper(url)
		return ctx, nil
	}
}

// waitForMetricsReady waits for Kepler metrics to be available
func waitForMetricsReady() env.Func {
	return func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
		// Wait for initial metrics collection
		time.Sleep(waitForMetricsAvailable)

		// Verify we can scrape metrics
		_, err := scraper.TakeSnapshot()
		if err != nil {
			return ctx, fmt.Errorf("failed to scrape metrics after setup: %w", err)
		}

		return ctx, nil
	}
}

// cleanupPortForward stops the port-forward safely (handles double-close)
func cleanupPortForward() env.Func {
	return func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
		closeOnce.Do(func() {
			if stopChan != nil {
				close(stopChan)
			}
		})
		return ctx, nil
	}
}

// ensureTestNamespace creates the test namespace if it doesn't exist
func ensureTestNamespace() env.Func {
	return func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
		r, err := resources.New(cfg.Client().RESTConfig())
		if err != nil {
			return ctx, fmt.Errorf("failed to create resources client: %w", err)
		}

		ns := &corev1.Namespace{}
		ns.Name = testNamespace

		// Check if namespace already exists
		if err := r.Get(ctx, testNamespace, "", ns); err == nil {
			return ctx, nil // Namespace exists
		}

		// Create namespace
		ns = &corev1.Namespace{}
		ns.Name = testNamespace
		if err := r.Create(ctx, ns); err != nil {
			return ctx, fmt.Errorf("failed to create test namespace %s: %w", testNamespace, err)
		}

		return ctx, nil
	}
}

// cleanupTestNamespace deletes the test namespace and all resources in it
func cleanupTestNamespace() env.Func {
	return func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
		r, err := resources.New(cfg.Client().RESTConfig())
		if err != nil {
			return ctx, nil // Best effort cleanup
		}

		ns := &corev1.Namespace{}
		ns.Name = testNamespace
		_ = r.Delete(ctx, ns) // Ignore errors - namespace may not exist

		return ctx, nil
	}
}
