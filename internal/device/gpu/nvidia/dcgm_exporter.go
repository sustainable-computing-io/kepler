// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package nvidia

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	// metricsCacheTTL is how long cached metrics are valid before refetching.
	// This prevents HTTP request storms when querying multiple MIG instances.
	metricsCacheTTL = 2 * time.Second

	// maxConsecutiveFailures is how many HTTP failures before the circuit breaker trips.
	maxConsecutiveFailures = 3

	// reInitInterval is how long to wait before re-attempting initialization
	// after the circuit breaker has tripped.
	reInitInterval = 30 * time.Second
)

// DCGMExporterBackend provides MIG metrics by querying dcgm-exporter's Prometheus endpoint.
// This is an alternative to the go-dcgm library that doesn't require libdcgm.so.
//
// Why dcgm-exporter:
//   - dcgm-exporter is already deployed by NVIDIA GPU Operator as a DaemonSet
//   - Exposes per-MIG-instance metrics with GPU_I_ID and GPU_I_PROFILE labels
//   - No native library dependencies required
//   - Uses standard Prometheus text format
type DCGMExporterBackend struct {
	logger      *slog.Logger
	endpoint    string // e.g., "http://10.131.2.22:9400/metrics"
	client      *http.Client
	initialized bool
	mu          sync.Mutex

	// Cached metrics with TTL to avoid HTTP request storms
	cachedMetrics *dcgmMetrics

	// Circuit breaker: tracks consecutive failures and disables the backend
	// after maxConsecutiveFailures. Re-initialization is attempted after reInitInterval.
	consecutiveFailures int
	circuitOpenTime     time.Time

	// discoverEndpoint discovers the local dcgm-exporter endpoint.
	// Defaults to discoverLocalDCGMExporter. Overridable for testing.
	discoverEndpoint func() string

	// fallbackEndpoints is the list of static endpoints to try if discovery fails.
	fallbackEndpoints []string

	// discoveryLabels are label selectors tried in order to find dcgm-exporter pods.
	// Searched across all namespaces on the current node.
	discoveryLabels []string

	// kubeClient is the Kubernetes clientset for pod discovery.
	// If nil, created from in-cluster config at discovery time.
	// Injectable for testing.
	kubeClient kubernetes.Interface
}

// dcgmMetrics holds parsed metrics from dcgm-exporter
type dcgmMetrics struct {
	// MIG instances indexed by (gpuIndex, gpuInstanceID)
	instances map[migKey]*migInstanceMetrics
	timestamp time.Time
}

type migKey struct {
	gpuIndex      int
	gpuInstanceID uint
}

type migInstanceMetrics struct {
	gpuIndex      int
	gpuInstanceID uint
	profile       string  // e.g., "1g.5gb"
	activity      float64 // DCGM_FI_PROF_GR_ENGINE_ACTIVE
}

// NewDCGMExporterBackend creates a new dcgm-exporter HTTP backend
func NewDCGMExporterBackend(logger *slog.Logger) *DCGMExporterBackend {
	if logger == nil {
		logger = slog.Default()
	}
	d := &DCGMExporterBackend{
		logger: logger.With("component", "dcgm-exporter"),
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
		fallbackEndpoints: []string{
			"http://localhost:9400/metrics",
			"http://nvidia-dcgm-exporter.nvidia-gpu-operator.svc:9400/metrics",
		},
		discoveryLabels: []string{
			"app=nvidia-dcgm-exporter",             // GPU Operator default
			"app.kubernetes.io/name=dcgm-exporter", // Standalone Helm chart
		},
	}
	d.discoverEndpoint = d.discoverLocalDCGMExporter
	return d
}

// SetEndpoint sets the dcgm-exporter endpoint URL
func (d *DCGMExporterBackend) SetEndpoint(endpoint string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Ensure endpoint has /metrics suffix
	endpoint = strings.TrimSuffix(endpoint, "/")
	if !strings.HasSuffix(endpoint, "/metrics") {
		endpoint += "/metrics"
	}
	d.endpoint = endpoint
}

// Init initializes the backend by discovering the dcgm-exporter endpoint.
// If endpoint is not set, it discovers the local dcgm-exporter pod on the same node.
func (d *DCGMExporterBackend) Init(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.initialized {
		return nil
	}

	return d.initLocked(ctx)
}

// initLocked performs endpoint discovery and initialization.
// Caller must hold d.mu.
func (d *DCGMExporterBackend) initLocked(ctx context.Context) error {
	// If endpoint is already set, use it
	if d.endpoint != "" {
		if err := d.testEndpoint(ctx, d.endpoint); err != nil {
			return fmt.Errorf("configured endpoint %s not reachable: %w", d.endpoint, err)
		}
		d.markInitialized()
		d.logger.Info("DCGM exporter backend initialized", "endpoint", d.endpoint)
		return nil
	}

	// Try to discover dcgm-exporter endpoint
	// Priority:
	// 1. Local dcgm-exporter pod on same node (discovered via K8s API)
	// 2. localhost:9400 (if dcgm-exporter uses hostNetwork)
	// 3. ClusterIP service (may route to wrong node — last resort, but with
	//    DaemonSet deployment the service routes to the local pod)

	if localEndpoint := d.discoverEndpoint(); localEndpoint != "" {
		if err := d.testEndpoint(ctx, localEndpoint); err == nil {
			d.endpoint = localEndpoint
			d.markInitialized()
			d.logger.Info("DCGM exporter backend initialized", "endpoint", localEndpoint, "discovery", "local-pod")
			return nil
		}
		d.logger.Warn("local dcgm-exporter not reachable", "endpoint", localEndpoint)
	}

	// Fallback to static endpoints. This covers cases where K8s API discovery
	// fails (e.g., RBAC issues, dcgm-exporter in a non-standard namespace)
	// or dcgm-exporter uses hostNetwork (localhost:9400).
	for _, ep := range d.fallbackEndpoints {
		if err := d.testEndpoint(ctx, ep); err == nil {
			d.endpoint = ep
			d.markInitialized()
			d.logger.Info("DCGM exporter backend initialized", "endpoint", ep, "discovery", "fallback")
			return nil
		}
		d.logger.Debug("dcgm-exporter endpoint not reachable", "endpoint", ep)
	}

	return fmt.Errorf("no reachable dcgm-exporter endpoint found")
}

// markInitialized resets circuit breaker state and marks the backend as ready.
// Caller must hold d.mu.
func (d *DCGMExporterBackend) markInitialized() {
	d.initialized = true
	d.consecutiveFailures = 0
	d.circuitOpenTime = time.Time{}
}

// discoverLocalDCGMExporter finds the dcgm-exporter pod IP on the same node
func (d *DCGMExporterBackend) discoverLocalDCGMExporter() string {
	// Get current node name from environment (set by downward API)
	nodeName := os.Getenv("NODE_NAME")
	if nodeName == "" {
		d.logger.Debug("NODE_NAME not set, cannot discover local dcgm-exporter")
		return ""
	}

	// Use injected client or create from in-cluster config
	clientset := d.kubeClient
	if clientset == nil {
		config, err := rest.InClusterConfig()
		if err != nil {
			d.logger.Debug("failed to get in-cluster config", "error", err)
			return ""
		}

		clientset, err = kubernetes.NewForConfig(config)
		if err != nil {
			d.logger.Debug("failed to create kubernetes client", "error", err)
			return ""
		}
	}

	// Search all namespaces for dcgm-exporter pods on the same node.
	// Try multiple label selectors to support different deployment methods.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	nodeSelector := fmt.Sprintf("spec.nodeName=%s", nodeName)

	for _, label := range d.discoveryLabels {
		pods, err := clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{
			LabelSelector: label,
			FieldSelector: nodeSelector,
		})
		if err != nil {
			d.logger.Debug("failed to list dcgm-exporter pods", "label", label, "error", err)
			continue
		}

		for i := range pods.Items {
			pod := &pods.Items[i]
			if pod.Status.PodIP == "" {
				continue
			}
			endpoint := fmt.Sprintf("http://%s:9400/metrics", pod.Status.PodIP)
			d.logger.Debug("discovered local dcgm-exporter",
				"pod", pod.Name, "namespace", pod.Namespace,
				"ip", pod.Status.PodIP, "node", nodeName)
			return endpoint
		}
	}

	d.logger.Debug("no dcgm-exporter pod found on node", "node", nodeName)
	return ""
}

// testEndpoint checks if an endpoint is reachable and returns valid metrics
func (d *DCGMExporterBackend) testEndpoint(ctx context.Context, endpoint string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, resp.Body)
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	// Drain body to allow HTTP keep-alive connection reuse
	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}

// Shutdown cleans up resources
func (d *DCGMExporterBackend) Shutdown() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.initialized = false
	d.cachedMetrics = nil
	d.consecutiveFailures = 0
	d.circuitOpenTime = time.Time{}
	d.logger.Info("DCGM exporter backend shutdown")
	return nil
}

// IsInitialized returns whether the backend is ready
func (d *DCGMExporterBackend) IsInitialized() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.initialized
}

// GetMIGInstanceActivity returns the GR_ENGINE_ACTIVE metric for a MIG instance.
func (d *DCGMExporterBackend) GetMIGInstanceActivity(ctx context.Context, gpuIndex int, gpuInstanceID uint) (float64, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.initialized {
		// If circuit breaker hasn't tripped (never initialized), return error
		if d.circuitOpenTime.IsZero() {
			return 0, fmt.Errorf("DCGM exporter backend not initialized")
		}
		// If not enough time has passed, don't retry
		if time.Since(d.circuitOpenTime) < reInitInterval {
			return 0, fmt.Errorf("DCGM circuit breaker open, retry in %v",
				reInitInterval-time.Since(d.circuitOpenTime))
		}
		// Attempt re-initialization
		d.logger.Info("attempting DCGM backend re-initialization", "endpoint", d.endpoint)
		if err := d.initLocked(ctx); err != nil {
			d.circuitOpenTime = time.Now()
			d.logger.Warn("DCGM re-initialization failed", "error", err)
			return 0, fmt.Errorf("DCGM re-init failed: %w", err)
		}
		d.logger.Info("DCGM backend re-initialized successfully", "endpoint", d.endpoint)
	}

	metrics, err := d.fetchMetrics(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch metrics: %w", err)
	}

	key := migKey{gpuIndex: gpuIndex, gpuInstanceID: gpuInstanceID}
	if m, ok := metrics.instances[key]; ok {
		return m.activity, nil
	}

	return 0, fmt.Errorf("no metrics found for GPU %d, MIG instance %d", gpuIndex, gpuInstanceID)
}

// fetchMetrics returns cached metrics if still valid, otherwise fetches fresh data.
// Caller must hold d.mu. On cache miss (every ~2s per metricsCacheTTL), performs
// an HTTP GET to dcgm-exporter and writes d.cachedMetrics with parsed results.
// Tracks consecutive failures and trips the circuit breaker after maxConsecutiveFailures.
func (d *DCGMExporterBackend) fetchMetrics(ctx context.Context) (*dcgmMetrics, error) {
	// Return cached metrics if still valid
	if d.cachedMetrics != nil && time.Since(d.cachedMetrics.timestamp) < metricsCacheTTL {
		return d.cachedMetrics, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, d.endpoint, nil)
	if err != nil {
		d.recordFailure()
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := d.client.Do(req)
	if err != nil {
		d.recordFailure()
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, resp.Body)
		d.recordFailure()
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	metrics, err := d.parseMetrics(resp.Body)
	if err != nil {
		d.recordFailure()
		return nil, err
	}

	d.consecutiveFailures = 0
	d.cachedMetrics = metrics
	return metrics, nil
}

// recordFailure increments the failure counter and trips the circuit breaker
// if the threshold is reached. Caller must hold d.mu.
func (d *DCGMExporterBackend) recordFailure() {
	d.consecutiveFailures++
	if d.consecutiveFailures >= maxConsecutiveFailures {
		d.initialized = false
		d.circuitOpenTime = time.Now()
		d.cachedMetrics = nil
		d.logger.Warn("DCGM circuit breaker tripped, disabling backend",
			"consecutive_failures", d.consecutiveFailures,
			"endpoint", d.endpoint,
			"retry_after", reInitInterval)
	}
}

// metricRegex parses Prometheus metric lines.
// Format: metric_name{label1="value1",label2="value2",...} value
var metricRegex = regexp.MustCompile(`^(DCGM_FI_\w+)\{([^}]+)\}\s+([0-9.eE+-]+)`)

// labelRegex parses individual labels from a Prometheus label string.
var labelRegex = regexp.MustCompile(`(\w+)="([^"]*)"`)

// parseMetrics parses Prometheus text format metrics from dcgm-exporter
func (d *DCGMExporterBackend) parseMetrics(reader io.Reader) (*dcgmMetrics, error) {
	metrics := &dcgmMetrics{
		instances: make(map[migKey]*migInstanceMetrics),
		timestamp: time.Now(),
	}

	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()

		// Skip comments and empty lines
		if len(line) == 0 || line[0] == '#' {
			continue
		}

		matches := metricRegex.FindStringSubmatch(line)
		if matches == nil {
			continue
		}

		metricName := matches[1]
		labelsStr := matches[2]
		valueStr := matches[3]

		// Parse labels
		labels := parseLabels(labelsStr)

		// Only process metrics with GPU_I_ID (MIG instance metrics)
		gpuIIDStr, hasMIG := labels["GPU_I_ID"]
		if !hasMIG {
			continue
		}

		gpuStr := labels["gpu"]
		gpuIndex, _ := strconv.Atoi(gpuStr)
		gpuInstanceID, _ := strconv.ParseUint(gpuIIDStr, 10, 32)

		key := migKey{gpuIndex: gpuIndex, gpuInstanceID: uint(gpuInstanceID)}

		// Get or create instance metrics
		instance, ok := metrics.instances[key]
		if !ok {
			instance = &migInstanceMetrics{
				gpuIndex:      gpuIndex,
				gpuInstanceID: uint(gpuInstanceID),
				profile:       labels["GPU_I_PROFILE"],
			}
			metrics.instances[key] = instance
		}

		value, err := strconv.ParseFloat(valueStr, 64)
		if err != nil {
			continue
		}

		// Only DCGM_FI_PROF_GR_ENGINE_ACTIVE is used for activity-based power attribution
		if metricName == "DCGM_FI_PROF_GR_ENGINE_ACTIVE" {
			instance.activity = value
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading metrics: %w", err)
	}

	return metrics, nil
}

// parseLabels parses a Prometheus label string like `key1="val1",key2="val2"`
func parseLabels(labelsStr string) map[string]string {
	labels := make(map[string]string)
	matches := labelRegex.FindAllStringSubmatch(labelsStr, -1)
	for _, match := range matches {
		if len(match) == 3 {
			labels[match[1]] = match[2]
		}
	}
	return labels
}

// Verify interface implementation
var _ DCGMBackend = (*DCGMExporterBackend)(nil)
