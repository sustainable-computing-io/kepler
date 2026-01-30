// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
)

const (
	// RAPLPath is the path to the RAPL sysfs interface
	RAPLPath = "/sys/class/powercap/intel-rapl"

	// StressNGBinary is the name of the stress-ng binary
	StressNGBinary = "stress-ng"
)

// skipIfNoRAPL skips the test if RAPL is not available
func skipIfNoRAPL(t *testing.T) {
	t.Helper()

	if _, err := os.Stat(RAPLPath); os.IsNotExist(err) {
		t.Skipf("Skipping: RAPL not available at %s", RAPLPath)
	}

	entries, err := os.ReadDir(RAPLPath)
	if err != nil {
		t.Skipf("Skipping: Cannot read RAPL directory: %v", err)
	}

	if len(entries) == 0 {
		t.Skipf("Skipping: No RAPL zones found at %s", RAPLPath)
	}
}

// skipIfNoStressNG skips the test if stress-ng is not available
func skipIfNoStressNG(t *testing.T) {
	t.Helper()

	if _, err := exec.LookPath(StressNGBinary); err != nil {
		t.Skipf("Skipping: stress-ng not found (install: apt-get install stress-ng)")
	}
}

// skipIfRAPLNotReadable skips the test if RAPL energy values are not readable
func skipIfRAPLNotReadable(t *testing.T) {
	t.Helper()

	energyPath := filepath.Join(RAPLPath, "intel-rapl:0", "energy_uj")
	if _, err := os.ReadFile(energyPath); err != nil {
		t.Skipf("Skipping: RAPL energy not readable at %s: %v", energyPath, err)
	}
}

// requireE2EPrerequisites checks all common prerequisites for e2e tests
func requireE2EPrerequisites(t *testing.T) {
	t.Helper()
	skipIfNoRAPL(t)
	skipIfRAPLNotReadable(t)
}

// requireWorkloadPrerequisites checks prerequisites for workload tests
func requireWorkloadPrerequisites(t *testing.T) {
	t.Helper()
	requireE2EPrerequisites(t)
	skipIfNoStressNG(t)
}

// KeplerInstance manages a Kepler process for testing
type KeplerInstance struct {
	cmd        *exec.Cmd
	configPath string
	binaryPath string
	port       int
	logOutput  io.Writer
}

// keplerOption configures KeplerInstance
type keplerOption func(*KeplerInstance)

// withLogOutput sets where to write logs
func withLogOutput(w io.Writer) keplerOption {
	return func(k *KeplerInstance) { k.logOutput = w }
}

// startKepler starts Kepler and registers cleanup
func startKepler(t *testing.T, opts ...keplerOption) *KeplerInstance {
	t.Helper()

	k := &KeplerInstance{
		port:       testConfig.metricsPort,
		binaryPath: testConfig.keplerBinary,
		configPath: testConfig.configFile,
		logOutput:  io.Discard,
	}

	for _, opt := range opts {
		opt(k)
	}

	if err := k.start(t); err != nil {
		t.Fatalf("Failed to start Kepler: %v", err)
	}

	t.Cleanup(func() {
		if err := k.stop(); err != nil {
			t.Logf("Warning: Failed to stop Kepler: %v", err)
		}
	})

	return k
}

func (k *KeplerInstance) start(t *testing.T) error {
	t.Helper()

	binaryPath, err := k.findBinary()
	if err != nil {
		return err
	}

	args := []string{
		fmt.Sprintf("--web.listen-address=:%d", k.port),
		"--log.level=debug",
	}
	if k.configPath != "" {
		args = append(args, fmt.Sprintf("--config.file=%s", k.configPath))
	}

	k.cmd = exec.Command(binaryPath, args...)

	stdout, _ := k.cmd.StdoutPipe()
	stderr, _ := k.cmd.StderrPipe()

	// Forward logs in background (errors are not actionable here)
	go func() { _, _ = io.Copy(k.logOutput, stdout) }()
	go func() { _, _ = io.Copy(k.logOutput, stderr) }()

	t.Logf("Starting Kepler: %s %v", binaryPath, args)

	if err := k.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start kepler: %w", err)
	}

	if err := k.waitForReady(30 * time.Second); err != nil {
		_ = k.stop() // Best-effort cleanup
		return fmt.Errorf("kepler failed to become ready: %w", err)
	}

	t.Logf("Kepler started on port %d (PID: %d)", k.port, k.cmd.Process.Pid)
	return nil
}

func (k *KeplerInstance) findBinary() (string, error) {
	if _, err := os.Stat(k.binaryPath); err == nil {
		return filepath.Abs(k.binaryPath)
	}
	if path, err := exec.LookPath("kepler"); err == nil {
		return path, nil
	}
	return "", fmt.Errorf("kepler binary not found at %s or in PATH", k.binaryPath)
}

func (k *KeplerInstance) waitForReady(timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	url := fmt.Sprintf("http://localhost:%d/metrics", k.port)

	return waitForCondition(ctx, 500*time.Millisecond, func() bool {
		resp, err := http.Get(url)
		if err != nil {
			return false
		}
		status := resp.StatusCode
		_ = resp.Body.Close()
		return status == http.StatusOK
	})
}

func (k *KeplerInstance) stop() error {
	if k.cmd == nil || k.cmd.Process == nil {
		return nil
	}

	// Send SIGTERM for graceful shutdown
	_ = k.cmd.Process.Signal(syscall.SIGTERM)

	done := make(chan error, 1)
	go func() {
		_, err := k.cmd.Process.Wait()
		done <- err
	}()

	select {
	case <-time.After(10 * time.Second):
		_ = k.cmd.Process.Kill() // Force kill on timeout
		return fmt.Errorf("kepler did not stop gracefully")
	case <-done:
		return nil
	}
}

// MetricsURL returns the metrics endpoint URL
func (k *KeplerInstance) MetricsURL() string {
	return fmt.Sprintf("http://localhost:%d/metrics", k.port)
}

// BaseURL returns the base URL for the Kepler server
func (k *KeplerInstance) BaseURL() string {
	return fmt.Sprintf("http://localhost:%d", k.port)
}

// PID returns the Kepler process PID
func (k *KeplerInstance) PID() int {
	if k.cmd == nil || k.cmd.Process == nil {
		return 0
	}
	return k.cmd.Process.Pid
}

// IsRunning returns true if Kepler is running
func (k *KeplerInstance) IsRunning() bool {
	if k.cmd == nil || k.cmd.Process == nil {
		return false
	}
	return k.cmd.Process.Signal(syscall.Signal(0)) == nil
}

// Stop stops Kepler
func (k *KeplerInstance) Stop() error {
	return k.stop()
}

// Metric represents a parsed Prometheus metric
type Metric struct {
	Name   string
	Labels map[string]string
	Value  float64
}

// MetricFamily represents a group of metrics with the same name
type MetricFamily struct {
	Name    string
	Metrics []Metric
}

// MetricsScraper scrapes Prometheus metrics
type MetricsScraper struct {
	url    string
	client *http.Client
}

// NewMetricsScraper creates a new scraper
func NewMetricsScraper(url string) *MetricsScraper {
	return &MetricsScraper{
		url:    url,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// Scrape fetches all metrics
func (s *MetricsScraper) Scrape() (map[string]*MetricFamily, error) {
	resp, err := s.client.Get(s.url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch metrics: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	return parseMetrics(resp.Body, resp.Header)
}

// ScrapeMetric fetches a specific metric
func (s *MetricsScraper) ScrapeMetric(name string) ([]Metric, error) {
	families, err := s.Scrape()
	if err != nil {
		return nil, err
	}

	family, ok := families[name]
	if !ok {
		return nil, fmt.Errorf("metric %s not found", name)
	}
	return family.Metrics, nil
}

// ScrapeMetricWithLabels fetches metrics matching labels
func (s *MetricsScraper) ScrapeMetricWithLabels(name string, labels map[string]string) ([]Metric, error) {
	metrics, err := s.ScrapeMetric(name)
	if err != nil {
		return nil, err
	}

	var matched []Metric
	for _, m := range metrics {
		if matchLabels(m.Labels, labels) {
			matched = append(matched, m)
		}
	}
	return matched, nil
}

// GetMetricValue returns a single metric value
func (s *MetricsScraper) GetMetricValue(name string, labels map[string]string) (float64, error) {
	metrics, err := s.ScrapeMetricWithLabels(name, labels)
	if err != nil {
		return 0, err
	}
	if len(metrics) == 0 {
		return 0, fmt.Errorf("no metrics found for %s", name)
	}
	if len(metrics) > 1 {
		return 0, fmt.Errorf("multiple metrics found for %s", name)
	}
	return metrics[0].Value, nil
}

// SumMetricValues returns sum of matching metrics
func (s *MetricsScraper) SumMetricValues(name string, labels map[string]string) (float64, error) {
	metrics, err := s.ScrapeMetricWithLabels(name, labels)
	if err != nil {
		return 0, err
	}

	var sum float64
	for _, m := range metrics {
		sum += m.Value
	}
	return sum, nil
}

func matchLabels(metricLabels, expected map[string]string) bool {
	for k, v := range expected {
		if metricLabels[k] != v {
			return false
		}
	}
	return true
}

// parseMetrics parses Prometheus metrics
func parseMetrics(r io.Reader, header http.Header) (map[string]*MetricFamily, error) {
	families := make(map[string]*MetricFamily)

	// Determine format from response header
	format := expfmt.ResponseFormat(header)

	decoder := expfmt.NewDecoder(r, format)
	for {
		var mf dto.MetricFamily
		if err := decoder.Decode(&mf); err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("failed to decode metrics: %w", err)
		}

		name := mf.GetName()
		family := &MetricFamily{Name: name}

		for _, m := range mf.GetMetric() {
			labels := make(map[string]string)
			for _, lp := range m.GetLabel() {
				labels[lp.GetName()] = lp.GetValue()
			}

			value := extractMetricValue(m)
			family.Metrics = append(family.Metrics, Metric{
				Name:   name,
				Labels: labels,
				Value:  value,
			})
		}

		families[name] = family
	}

	return families, nil
}

// extractMetricValue extracts the numeric value from a metric based on its type
func extractMetricValue(m *dto.Metric) float64 {
	if g := m.GetGauge(); g != nil {
		return g.GetValue()
	}
	if c := m.GetCounter(); c != nil {
		return c.GetValue()
	}
	if u := m.GetUntyped(); u != nil {
		return u.GetValue()
	}
	// For histograms and summaries, return the sample count as a reasonable default
	if h := m.GetHistogram(); h != nil {
		return float64(h.GetSampleCount())
	}
	if s := m.GetSummary(); s != nil {
		return float64(s.GetSampleCount())
	}
	return 0
}

// MetricsSnapshot is a point-in-time capture
type MetricsSnapshot struct {
	Timestamp time.Time
	Families  map[string]*MetricFamily
}

// TakeSnapshot captures current metrics
func (s *MetricsScraper) TakeSnapshot() (*MetricsSnapshot, error) {
	families, err := s.Scrape()
	if err != nil {
		return nil, err
	}
	return &MetricsSnapshot{Timestamp: time.Now(), Families: families}, nil
}

// GetValue returns a metric value from snapshot
func (ms *MetricsSnapshot) GetValue(name string, labels map[string]string) (float64, bool) {
	family, ok := ms.Families[name]
	if !ok {
		return 0, false
	}
	for _, m := range family.Metrics {
		if matchLabels(m.Labels, labels) {
			return m.Value, true
		}
	}
	return 0, false
}

// SumValues returns sum of matching metrics
func (ms *MetricsSnapshot) SumValues(name string, labels map[string]string) float64 {
	family, ok := ms.Families[name]
	if !ok {
		return 0
	}
	var sum float64
	for _, m := range family.Metrics {
		if matchLabels(m.Labels, labels) {
			sum += m.Value
		}
	}
	return sum
}

// HasMetric returns true if metric exists
func (ms *MetricsSnapshot) HasMetric(name string) bool {
	_, ok := ms.Families[name]
	return ok
}

// GetAllWithName returns all metrics with the name
func (ms *MetricsSnapshot) GetAllWithName(name string) []Metric {
	family, ok := ms.Families[name]
	if !ok {
		return nil
	}
	return family.Metrics
}

// Workload represents a stress-ng workload
type Workload struct {
	cmd        *exec.Cmd
	workerPIDs []int
	name       string
}

// workloadOption configures a workload
type workloadOption func(*workloadConfig)

type workloadConfig struct {
	name       string
	cpuWorkers int
	cpuLoad    int
}

// WithWorkloadName sets workload name
func WithWorkloadName(name string) workloadOption {
	return func(c *workloadConfig) { c.name = name }
}

// WithCPUWorkers sets CPU worker count
func WithCPUWorkers(n int) workloadOption {
	return func(c *workloadConfig) { c.cpuWorkers = n }
}

// WithCPULoad sets CPU load percentage
func WithCPULoad(percent int) workloadOption {
	return func(c *workloadConfig) { c.cpuLoad = percent }
}

// StartWorkload starts a stress workload
func StartWorkload(t *testing.T, opts ...workloadOption) *Workload {
	t.Helper()

	cfg := workloadConfig{name: "stress", cpuWorkers: 1, cpuLoad: 50}
	for _, opt := range opts {
		opt(&cfg)
	}

	args := []string{"--cpu", strconv.Itoa(cfg.cpuWorkers)}
	if cfg.cpuLoad > 0 && cfg.cpuLoad <= 100 {
		args = append(args, "--cpu-load", strconv.Itoa(cfg.cpuLoad))
	}
	args = append(args, "--metrics-brief")

	t.Logf("Starting workload %q: stress-ng %s", cfg.name, strings.Join(args, " "))

	cmd := exec.Command(StressNGBinary, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start stress-ng: %v", err)
	}

	w := &Workload{cmd: cmd, name: cfg.name}

	time.Sleep(500 * time.Millisecond)
	w.workerPIDs = findChildPIDs(cmd.Process.Pid)

	t.Logf("Workload %q started: PID=%d, workers=%v", cfg.name, cmd.Process.Pid, w.workerPIDs)

	t.Cleanup(func() {
		_ = w.Stop() // Best-effort cleanup
	})

	return w
}

func findChildPIDs(parentPID int) []int {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}

	var children []int
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}

		data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
		if err != nil {
			continue
		}

		fields := strings.Fields(string(data))
		if len(fields) >= 4 {
			if ppid, _ := strconv.Atoi(fields[3]); ppid == parentPID {
				children = append(children, pid)
			}
		}
	}
	return children
}

// Stop stops the workload
func (w *Workload) Stop() error {
	if w.cmd == nil || w.cmd.Process == nil {
		return nil
	}

	pgid, err := syscall.Getpgid(w.cmd.Process.Pid)
	if err == nil {
		_ = syscall.Kill(-pgid, syscall.SIGTERM) // Signal entire process group
	}

	done := make(chan error, 1)
	go func() {
		_, err := w.cmd.Process.Wait()
		done <- err
	}()

	select {
	case <-time.After(5 * time.Second):
		// Force kill on timeout
		if pgid != 0 {
			_ = syscall.Kill(-pgid, syscall.SIGKILL)
		}
		_ = w.cmd.Process.Kill()
		return fmt.Errorf("workload killed")
	case <-done:
		return nil
	}
}

// PIDs returns worker PIDs
func (w *Workload) PIDs() []int { return w.workerPIDs }

// ParentPID returns parent PID
func (w *Workload) ParentPID() int {
	if w.cmd == nil || w.cmd.Process == nil {
		return 0
	}
	return w.cmd.Process.Pid
}

// waitForCondition is a generic helper that waits for a condition to become true.
// It returns nil if the condition is met, or an error if the context is cancelled.
func waitForCondition(ctx context.Context, interval time.Duration, check func() bool) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if check() {
				return nil
			}
		}
	}
}

// WaitForValidCPUMetrics waits for Kepler to have valid CPU metrics.
// Returns true when both kepler_node_cpu_usage_ratio AND kepler_node_cpu_watts
// are present with valid (non-zero) values.
func WaitForValidCPUMetrics(t *testing.T, scraper *MetricsScraper, timeout time.Duration) bool {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var usageRatio float64
	var powerValue float64

	err := waitForCondition(ctx, 2*time.Second, func() bool {
		// Check for usage ratio
		usageMetrics, err := scraper.ScrapeMetric("kepler_node_cpu_usage_ratio")
		if err != nil || len(usageMetrics) == 0 || usageMetrics[0].Value <= 0 {
			return false
		}
		usageRatio = usageMetrics[0].Value

		// Check for power watts
		powerMetrics, err := scraper.ScrapeMetric("kepler_node_cpu_watts")
		if err != nil || len(powerMetrics) == 0 {
			return false
		}
		for _, m := range powerMetrics {
			if m.Value > 0 {
				powerValue = m.Value
				return true
			}
		}
		return false
	})
	if err != nil {
		t.Log("Timeout waiting for valid CPU metrics")
		return false
	}

	t.Logf("CPU metrics ready: usage_ratio=%.4f, power=%.2f W", usageRatio, powerValue)
	return true
}

// WaitForProcessInMetrics waits for a process to appear in metrics
func WaitForProcessInMetrics(t *testing.T, scraper *MetricsScraper, pid int, timeout time.Duration) bool {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	pidStr := strconv.Itoa(pid)

	err := waitForCondition(ctx, 1*time.Second, func() bool {
		metrics, err := scraper.ScrapeMetric("kepler_process_cpu_watts")
		if err != nil {
			return false
		}
		for _, m := range metrics {
			if m.Labels["pid"] == pidStr {
				return true
			}
		}
		return false
	})
	if err != nil {
		return false
	}

	t.Logf("Found PID=%d in metrics", pid)
	return true
}

// WaitForNonZeroProcessPower waits for a process to show non-zero power
func WaitForNonZeroProcessPower(t *testing.T, scraper *MetricsScraper, pid int, timeout time.Duration) (float64, bool) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	pidStr := strconv.Itoa(pid)
	var foundPower float64

	err := waitForCondition(ctx, 1*time.Second, func() bool {
		metrics, err := scraper.ScrapeMetric("kepler_process_cpu_watts")
		if err != nil {
			return false
		}
		for _, m := range metrics {
			if m.Labels["pid"] == pidStr && m.Value > 0 {
				foundPower = m.Value
				return true
			}
		}
		return false
	})
	if err != nil {
		return 0, false
	}

	return foundPower, true
}

// FindWorkloadPower searches for a workload's power in a metrics snapshot.
// It checks the parent PID first, then aggregates power from all worker PIDs.
// Returns the total power and whether the workload was found.
func FindWorkloadPower(snapshot *MetricsSnapshot, w *Workload) (power float64, found bool) {
	processMetrics := snapshot.GetAllWithName("kepler_process_cpu_watts")

	// Check parent PID first (only return if it has actual power and is running)
	parentPIDStr := strconv.Itoa(w.ParentPID())
	for _, m := range processMetrics {
		if m.Labels["pid"] == parentPIDStr && m.Labels["state"] == "running" && m.Value > 0 {
			return m.Value, true
		}
	}

	// Check worker PIDs and aggregate their power (only running processes)
	for _, pid := range w.PIDs() {
		pidStr := strconv.Itoa(pid)
		for _, m := range processMetrics {
			if m.Labels["pid"] == pidStr && m.Labels["state"] == "running" {
				power += m.Value
				found = true
			}
		}
	}
	return power, found
}

// WaitForTerminatedProcess waits for a process to appear in metrics with state=terminated
func WaitForTerminatedProcess(t *testing.T, scraper *MetricsScraper, pid int, timeout time.Duration) (energy float64, found bool) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	pidStr := strconv.Itoa(pid)

	err := waitForCondition(ctx, 2*time.Second, func() bool {
		metrics, err := scraper.ScrapeMetric("kepler_process_cpu_joules_total")
		if err != nil {
			return false
		}
		for _, m := range metrics {
			if m.Labels["pid"] == pidStr && m.Labels["state"] == "terminated" {
				energy = m.Value
				found = true
				return true
			}
		}
		return false
	})
	if err != nil {
		return 0, false
	}

	t.Logf("Found terminated PID=%d with energy=%.4f J", pid, energy)
	return energy, true
}
