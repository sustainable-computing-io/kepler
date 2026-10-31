// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"os"
	"os/exec"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/sustainable-computing-io/kepler/test/common"
)

const (
	// fakeGPUName is the device name reported by the fake GPU meter.
	fakeGPUName = "Fake NVIDIA GPU"

	// gpuSettleTimeout bounds waits on real GPU readings. Kepler only
	// refreshes them every few seconds, and a GPU takes a moment to change
	// power state, so this covers several refreshes.
	gpuSettleTimeout = 90 * time.Second

	// minLoadActiveWatts is the least active power a GPU under full load
	// should show. Real GPUs draw far more than this under dcgmproftester.
	minLoadActiveWatts = 20.0

	// loadGPU is the GPU the load runs on, as Docker's device index and
	// Kepler's gpu label. Other GPUs can show a little active power that no
	// process gets, so the load is only compared against this GPU.
	loadGPU = "0"

	// smiPercentTolerance and smiAbsTolerance bound the gap between Kepler
	// and nvidia-smi. Both read the same NVML sensor, but at different
	// moments, and an idle GPU's reading moves by a watt or so.
	smiPercentTolerance = 0.05
	smiAbsTolerance     = 3.0

	// commandTimeout bounds one-off nvidia-smi and docker calls made outside
	// the polling loops, so a stuck command fails the test instead of hanging it.
	commandTimeout = 30 * time.Second

	// gpuLoadStartTimeout bounds docker run, which pulls the DCGM image, a few
	// GB, the first time it runs on a host.
	gpuLoadStartTimeout = 10 * time.Minute

	// fakeGPUIdleWatts is the idle power the fake GPU meter reports until
	// tagged stress-ng workers appear.
	fakeGPUIdleWatts = 40.0

	keplerFakeGPUWorkloadDirEnv = "KEPLER_FAKE_GPU_WORKLOAD_DIR"
)

// setupKeplerForGPUTest starts Kepler for a GPU test with the GPU config. By
// default the GPU tests read a real NVIDIA GPU and skip on hosts without one.
// They do not need RAPL then, since e2e-gpu-config.yaml uses the fake CPU
// meter, so they also run on GPU hosts without RAPL, such as cloud VMs. With
// -gpu.fake they use the fake GPU meter and the default config, which needs RAPL.
func setupKeplerForGPUTest(t *testing.T, extra ...keplerOption) (*KeplerInstance, *common.MetricsScraper) {
	t.Helper()

	if testConfig.fakeGPU {
		requireE2EPrerequisites(t)
	} else {
		skipIfNoNvidiaGPU(t)
	}

	opts := []keplerOption{withLogOutput(os.Stderr), withConfig(testConfig.gpuConfigFile)}
	opts = append(opts, extra...)
	k := startKepler(t, opts...)
	return k, common.NewMetricsScraper(k.MetricsURL())
}

// skipIfNoNvidiaGPU skips the test on hosts without a usable NVIDIA GPU, found
// through nvidia-smi. Having nvidia-smi is not enough: a CPU-only VM built from
// a GPU image ships it but has no device, and a host whose driver is not loaded
// has a device nvidia-smi cannot reach. NVML finds no GPU on either.
func skipIfNoNvidiaGPU(t *testing.T) {
	t.Helper()

	if _, err := exec.LookPath("nvidia-smi"); err != nil {
		t.Skip("Skipping: no NVIDIA GPU (nvidia-smi not found), run with -gpu.fake to use the fake GPU")
	}

	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "nvidia-smi", "-L").CombinedOutput()
	if err != nil || !strings.Contains(string(out), "GPU ") {
		t.Skipf("Skipping: no usable NVIDIA GPU (nvidia-smi -L: %s), run with -gpu.fake to use the fake GPU",
			strings.TrimSpace(string(out)))
	}
}

// requireRealGPU skips the test unless Kepler reads a real NVIDIA GPU through
// NVML, which is the default when -gpu.fake is not set.
func requireRealGPU(t *testing.T) {
	t.Helper()

	if testConfig.fakeGPU {
		t.Skip("Skipping: needs a real GPU, runs without -gpu.fake")
	}
	skipIfNoNvidiaGPU(t)
}

// requireFakeGPU skips the test unless the suite is running against the fake
// GPU meter (-gpu.fake).
func requireFakeGPU(t *testing.T) {
	t.Helper()

	if !testConfig.fakeGPU {
		t.Skip("Skipping: needs the fake GPU meter, run with -gpu.fake")
	}
}

// waitForGPUMetrics waits until Kepler exposes GPU node metrics.
// This indicates the GPU backend has initialized and the monitor
// has completed at least one refresh cycle with GPU data.
func waitForGPUMetrics(t *testing.T, scraper *common.MetricsScraper, timeout time.Duration) bool {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	err := common.WaitForCondition(ctx, 2*time.Second, func() bool {
		metrics, err := scraper.ScrapeMetric("kepler_node_gpu_watts")
		return err == nil && len(metrics) > 0
	})

	return err == nil
}

// TestGPUNodeMetricsPresent verifies that GPU node-level metrics exist when
// GPU metrics are enabled, with the fake GPU meter or a real GPU.
func TestGPUNodeMetricsPresent(t *testing.T) {
	_, scraper := setupKeplerForGPUTest(t)

	require.True(t, waitForGPUMetrics(t, scraper, 30*time.Second),
		"Kepler should have GPU metrics when GPU metrics are enabled")

	snapshot, err := scraper.TakeSnapshot()
	require.NoError(t, err, "Failed to take metrics snapshot")

	gpuMetrics := []string{
		"kepler_node_gpu_watts",
		"kepler_node_gpu_idle_watts",
		"kepler_node_gpu_active_watts",
	}

	for _, metric := range gpuMetrics {
		assert.True(t, snapshot.HasMetric(metric),
			"metric %s should exist when GPU metrics are enabled", metric)
	}

	// GPU info metric should also be present
	assert.True(t, snapshot.HasMetric("kepler_node_gpu_info"),
		"kepler_node_gpu_info should exist when GPU metrics are enabled")
}

// TestGPUNodeMetricsLabels verifies GPU node metrics have required labels.
func TestGPUNodeMetricsLabels(t *testing.T) {
	_, scraper := setupKeplerForGPUTest(t)

	require.True(t, waitForGPUMetrics(t, scraper, 30*time.Second),
		"Kepler should have GPU metrics when GPU metrics are enabled")

	snapshot, err := scraper.TakeSnapshot()
	require.NoError(t, err)

	requiredLabels := []string{"gpu", "gpu_uuid", "gpu_name", "vendor"}

	metrics := snapshot.GetAllWithName("kepler_node_gpu_watts")
	require.NotEmpty(t, metrics)

	for _, m := range metrics {
		for _, label := range requiredLabels {
			assert.Contains(t, m.Labels, label,
				"kepler_node_gpu_watts should have %s label", label)
		}
	}
}

// TestGPUNodeMetricsNonNegative verifies all GPU power metric values are >= 0.
func TestGPUNodeMetricsNonNegative(t *testing.T) {
	_, scraper := setupKeplerForGPUTest(t)

	require.True(t, waitForGPUMetrics(t, scraper, 30*time.Second),
		"Kepler should have GPU metrics when GPU metrics are enabled")

	snapshot, err := scraper.TakeSnapshot()
	require.NoError(t, err)

	gpuMetrics := []string{
		"kepler_node_gpu_watts",
		"kepler_node_gpu_idle_watts",
		"kepler_node_gpu_active_watts",
	}

	for _, metricName := range gpuMetrics {
		metrics := snapshot.GetAllWithName(metricName)
		for _, m := range metrics {
			assert.GreaterOrEqual(t, m.Value, float64(0),
				"%s should be >= 0 (gpu=%s)", metricName, m.Labels["gpu"])
		}
	}
}

// TestGPUNodePowerConservation verifies: Total GPU Watts = Active + Idle.
func TestGPUNodePowerConservation(t *testing.T) {
	_, scraper := setupKeplerForGPUTest(t)

	require.True(t, waitForGPUMetrics(t, scraper, 30*time.Second),
		"Kepler should have GPU metrics when GPU metrics are enabled")

	snapshot, err := scraper.TakeSnapshot()
	require.NoError(t, err)

	totalMetrics := snapshot.GetAllWithName("kepler_node_gpu_watts")
	require.NotEmpty(t, totalMetrics)

	activeMetrics := snapshot.GetAllWithName("kepler_node_gpu_active_watts")
	idleMetrics := snapshot.GetAllWithName("kepler_node_gpu_idle_watts")

	// Build maps by gpu label for matching
	activeByGPU := make(map[string]float64)
	for _, m := range activeMetrics {
		activeByGPU[m.Labels["gpu"]] = m.Value
	}
	idleByGPU := make(map[string]float64)
	for _, m := range idleMetrics {
		idleByGPU[m.Labels["gpu"]] = m.Value
	}

	verified := 0
	for _, m := range totalMetrics {
		gpuLabel := m.Labels["gpu"]
		active := activeByGPU[gpuLabel]
		idle := idleByGPU[gpuLabel]
		computed := active + idle

		t.Logf("GPU %s: total=%.4f W, active=%.4f W, idle=%.4f W, computed=%.4f W",
			gpuLabel, m.Value, active, idle, computed)

		assertWithinTolerance(t, m.Value, computed, powerTolerance, absolutePowerTolerance,
			"GPU %s: total (%.4f) should equal active + idle (%.4f)", gpuLabel, m.Value, computed)

		verified++
	}

	assert.Greater(t, verified, 0, "Should verify at least one GPU power conservation")
}

// TestGPURealHardware verifies that Kepler reads a real GPU: an NVIDIA
// device with its own UUID that draws more than zero watts.
func TestGPURealHardware(t *testing.T) {
	requireRealGPU(t)
	_, scraper := setupKeplerForGPUTest(t)

	require.True(t, waitForGPUMetrics(t, scraper, 30*time.Second),
		"Kepler should have GPU metrics for a real GPU")

	snapshot, err := scraper.TakeSnapshot()
	require.NoError(t, err)

	metrics := snapshot.GetAllWithName("kepler_node_gpu_watts")
	require.NotEmpty(t, metrics)

	for _, m := range metrics {
		t.Logf("GPU %s: %s (%s), %.2f W", m.Labels["gpu"], m.Labels["gpu_name"], m.Labels["gpu_uuid"], m.Value)

		assert.Equal(t, "nvidia", m.Labels["vendor"], "GPU %s vendor", m.Labels["gpu"])
		assert.NotEqual(t, fakeGPUName, m.Labels["gpu_name"], "GPU %s should not be the fake GPU", m.Labels["gpu"])
		assert.True(t, strings.HasPrefix(m.Labels["gpu_uuid"], "GPU-"),
			"GPU %s should have an NVML UUID, got %q", m.Labels["gpu"], m.Labels["gpu_uuid"])
		assert.Greater(t, m.Value, float64(0), "GPU %s should draw power", m.Labels["gpu"])
	}
}

// TestGPUPowerMatchesNvidiaSMI compares Kepler's GPU power with nvidia-smi.
// Both read the same NVML sensor, so they should agree.
func TestGPUPowerMatchesNvidiaSMI(t *testing.T) {
	requireRealGPU(t)
	_, scraper := setupKeplerForGPUTest(t)

	require.True(t, waitForGPUMetrics(t, scraper, 30*time.Second),
		"Kepler should have GPU metrics for a real GPU")

	keplerWatts, smiWatts := waitForKeplerToMatchNvidiaSMI(t, scraper)

	for uuid, watts := range keplerWatts {
		t.Logf("GPU %s: kepler=%.2f W, nvidia-smi=%.2f W", uuid, watts, smiWatts[uuid])
	}
}

// TestGPUWorkloadAttribution loads the GPU with dcgmproftester and verifies
// that Kepler gives the active GPU power to that process and its container.
//
// The load runs in NVIDIA's DCGM image (-gpu.load-image) through Docker, so
// the host needs Docker set up for GPUs (nvidia-ctk runtime configure).
func TestGPUWorkloadAttribution(t *testing.T) {
	requireRealGPU(t)
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("Skipping: docker not found, it runs the GPU load")
	}
	skipIfGPUBusy(t)
	_, scraper := setupKeplerForGPUTest(t)

	require.True(t, waitForGPUMetrics(t, scraper, 30*time.Second),
		"Kepler should have GPU metrics for a real GPU")

	// Let Kepler see the GPU at idle first, so the active power it reports
	// under load is measured from the GPU's real idle power.
	_, idleSMI := waitForKeplerToMatchNvidiaSMI(t, scraper)
	idleSnapshot, err := scraper.TakeSnapshot()
	require.NoError(t, err)
	t.Logf("idle: nvidia-smi=%.2f W, node total=%.2f W, idle=%.2f W, active=%.2f W",
		sumWatts(idleSMI), idleSnapshot.SumValues("kepler_node_gpu_watts", nil),
		idleSnapshot.SumValues("kepler_node_gpu_idle_watts", nil),
		idleSnapshot.SumValues("kepler_node_gpu_active_watts", nil))

	load := startGPULoad(t)
	pids := waitForGPULoadPIDs(t, load)
	t.Logf("GPU load running: container=%s pid=%s", load.containerID[:12], strings.Join(pids, ","))

	// Wait for a refresh taken while the load holds the GPU at full power:
	// the load has GPU power, and Kepler's total matches nvidia-smi, so the
	// refresh did not land while the GPU was still ramping up.
	ctx, cancel := context.WithTimeout(context.Background(), gpuSettleTimeout)
	defer cancel()

	var snapshot *common.MetricsSnapshot
	var loadSMI map[string]float64
	var otherPIDs []string
	err = common.WaitForCondition(ctx, 2*time.Second, func() bool {
		// dcgmproftester can start more than one worker, for example one per
		// MIG instance, so the load is every load process on the GPU.
		loadPIDs, others, err := gpuLoadPIDs(ctx, load)
		if err != nil || len(loadPIDs) == 0 {
			return false
		}
		if len(others) > 0 {
			otherPIDs = others
			return true
		}
		s, err := scraper.TakeSnapshot()
		if err != nil {
			return false
		}
		active := s.SumValues("kepler_node_gpu_active_watts", map[string]string{"gpu": loadGPU})
		loadWatts, found := processGPUWatts(s, loadPIDs)
		if !found || loadWatts <= 0 || active < minLoadActiveWatts {
			return false
		}
		smi, err := nvidiaSMIWatts(ctx)
		if err != nil || !gpuWattsAgree(gpuWattsByUUID(s.GetAllWithName("kepler_node_gpu_watts")), smi) {
			return false
		}
		snapshot, loadSMI, pids = s, smi, loadPIDs
		return true
	})
	if len(otherPIDs) > 0 {
		t.Skipf("Skipping: another process started using a GPU during the test (pid %s)",
			strings.Join(otherPIDs, ", "))
	}
	require.NoError(t, err, "Kepler never attributed GPU power to the load (pid %s)", strings.Join(pids, ", "))

	total := snapshot.SumValues("kepler_node_gpu_watts", nil)
	idle := snapshot.SumValues("kepler_node_gpu_idle_watts", nil)
	active := snapshot.SumValues("kepler_node_gpu_active_watts", map[string]string{"gpu": loadGPU})
	running := snapshot.SumValues("kepler_process_gpu_watts", map[string]string{"state": "running"})
	loadWatts, _ := processGPUWatts(snapshot, pids)
	containerWatts, containerFound := snapshot.GetValue("kepler_container_gpu_watts",
		map[string]string{"container_id": load.containerID})

	t.Logf("under load: nvidia-smi=%.2f W, node total=%.2f W, idle=%.2f W, active=%.2f W, "+
		"running processes=%.2f W, load pid=%.2f W, load container=%.2f W",
		sumWatts(loadSMI), total, idle, active, running, loadWatts, containerWatts)

	// The load is the only GPU process on the host, so running processes
	// share all of the load GPU's active power. Kepler's process GPU power has
	// no GPU label, which is why other GPU processes make the test skip.
	assertWithinTolerance(t, active, running, powerTolerance, absolutePowerTolerance,
		"sum of process GPU power should equal the load GPU's active power")

	// The load is the only process using the GPU, so it gets nearly all of it.
	assert.GreaterOrEqual(t, loadWatts, 0.9*active,
		"the load should get most of its GPU's active power")

	for _, m := range snapshot.GetAllWithName("kepler_process_gpu_watts") {
		if slices.Contains(pids, m.Labels["pid"]) {
			assert.Equal(t, load.containerID, m.Labels["container_id"],
				"the load process %s should be attributed to its container", m.Labels["pid"])
		}
	}

	require.True(t, containerFound, "kepler_container_gpu_watts should exist for the load container")
	assertWithinTolerance(t, loadWatts, containerWatts, powerTolerance, absolutePowerTolerance,
		"the load container should get its processes' GPU power")
}

// TestGPUFakeWorkloadAttribution verifies the fake GPU meter reports idle
// power, then attributes load power to tagged stress-ng workers.
func TestGPUFakeWorkloadAttribution(t *testing.T) {
	requireFakeGPU(t)
	requireStressNG(t)

	dir := t.TempDir()
	_, scraper := setupKeplerForGPUTest(t,
		withEnv(keplerFakeGPUWorkloadDirEnv+"="+dir),
	)

	require.True(t, waitForGPUMetrics(t, scraper, 30*time.Second),
		"Kepler should have GPU metrics when the fake GPU meter is enabled")
	waitForFakeGPUIdle(t, scraper)

	workload := StartWorkload(t,
		WithWorkloadName("fake-gpu"),
		WithCPUWorkers(2),
		WithWorkDir(dir),
	)
	pids := waitForFakeGPUWorkerPIDs(t, workload)
	t.Logf("fake GPU workload running: parent=%d workers=%s",
		workload.ParentPID(), strings.Join(pids, ","))

	snapshot := waitForFakeGPULoad(t, scraper, pids)

	gpu := map[string]string{"gpu": loadGPU}
	total := snapshot.SumValues("kepler_node_gpu_watts", gpu)
	idle := snapshot.SumValues("kepler_node_gpu_idle_watts", gpu)
	active := snapshot.SumValues("kepler_node_gpu_active_watts", gpu)
	loadWatts, found := processGPUWatts(snapshot, pids)
	require.True(t, found, "kepler_process_gpu_watts should exist for workers %s", strings.Join(pids, ", "))

	t.Logf("under fake load: total=%.4f W, idle=%.4f W, active=%.4f W, workers=%.4f W",
		total, idle, active, loadWatts)

	assertWithinTolerance(t, total, active+idle, powerTolerance, absolutePowerTolerance,
		"GPU %s: total (%.4f) should equal active + idle (%.4f)", loadGPU, total, active+idle)
	assertWithinTolerance(t, active, loadWatts, powerTolerance, absolutePowerTolerance,
		"sum of workload process GPU power should equal the fake GPU's active power")

	w0, _ := snapshot.GetValue("kepler_process_gpu_watts", map[string]string{"pid": pids[0]})
	w1, _ := snapshot.GetValue("kepler_process_gpu_watts", map[string]string{"pid": pids[1]})
	assert.NotEqual(t, w0, w1, "time-slicing 80/20 should give unequal process GPU power (pid %s=%.4f, pid %s=%.4f)",
		pids[0], w0, pids[1], w1)
}

func waitForFakeGPUIdle(t *testing.T, scraper *common.MetricsScraper) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), gpuSettleTimeout)
	defer cancel()

	err := common.WaitForCondition(ctx, 2*time.Second, func() bool {
		s, err := scraper.TakeSnapshot()
		if err != nil {
			return false
		}
		gpu := map[string]string{"gpu": loadGPU}
		total := s.SumValues("kepler_node_gpu_watts", gpu)
		active := s.SumValues("kepler_node_gpu_active_watts", gpu)
		return math.Abs(active) <= absolutePowerTolerance &&
			math.Abs(total-fakeGPUIdleWatts) <= math.Max(fakeGPUIdleWatts*powerTolerance, absolutePowerTolerance)
	})
	require.NoError(t, err, "Kepler never reported the fake GPU as idle (~0 W active, ~%.0f W total)", fakeGPUIdleWatts)
}

func waitForFakeGPUWorkerPIDs(t *testing.T, w *Workload) []string {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), gpuSettleTimeout)
	defer cancel()

	err := common.WaitForCondition(ctx, 200*time.Millisecond, func() bool {
		var workers []int
		for _, pid := range findChildPIDs(w.ParentPID()) {
			comm, err := os.ReadFile(fmt.Sprintf("/proc/%d/comm", pid))
			if err != nil {
				continue
			}
			if strings.HasPrefix(strings.TrimSpace(string(comm)), "stress-ng-cpu") {
				workers = append(workers, pid)
			}
		}
		w.workerPIDs = workers
		return len(workers) == 2
	})
	require.NoError(t, err, "stress-ng never started exactly 2 CPU workers (got %v)", w.PIDs())

	pids := make([]string, len(w.PIDs()))
	for i, pid := range w.PIDs() {
		pids[i] = strconv.Itoa(pid)
	}
	return pids
}

func waitForFakeGPULoad(t *testing.T, scraper *common.MetricsScraper, pids []string) *common.MetricsSnapshot {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), gpuSettleTimeout)
	defer cancel()

	var snapshot *common.MetricsSnapshot
	err := common.WaitForCondition(ctx, 2*time.Second, func() bool {
		s, err := scraper.TakeSnapshot()
		if err != nil {
			return false
		}
		active := s.SumValues("kepler_node_gpu_active_watts", map[string]string{"gpu": loadGPU})
		loadWatts, found := processGPUWatts(s, pids)
		if !found || loadWatts <= 0 || active <= 0 {
			return false
		}
		snapshot = s
		return true
	})
	require.NoError(t, err, "Kepler never attributed fake GPU power to workers %s", strings.Join(pids, ", "))
	return snapshot
}

// keplerGPUWatts returns Kepler's GPU power keyed by GPU UUID.
func keplerGPUWatts(scraper *common.MetricsScraper) map[string]float64 {
	metrics, err := scraper.ScrapeMetric("kepler_node_gpu_watts")
	if err != nil {
		return nil
	}
	return gpuWattsByUUID(metrics)
}

// gpuWattsByUUID keys kepler_node_gpu_watts values by GPU UUID.
func gpuWattsByUUID(metrics []common.Metric) map[string]float64 {
	watts := make(map[string]float64, len(metrics))
	for _, m := range metrics {
		watts[m.Labels["gpu_uuid"]] = m.Value
	}
	return watts
}

// nvidiaSMIWatts returns nvidia-smi's GPU power keyed by GPU UUID. GPUs
// without a power reading are left out. ctx bounds the nvidia-smi call.
func nvidiaSMIWatts(ctx context.Context) (map[string]float64, error) {
	out, err := exec.CommandContext(ctx, "nvidia-smi", "--query-gpu=uuid,power.draw",
		"--format=csv,noheader,nounits").Output()
	if err != nil {
		return nil, fmt.Errorf("nvidia-smi: %w", err)
	}

	watts := make(map[string]float64)
	for line := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
		uuid, power, ok := strings.Cut(line, ",")
		if !ok {
			continue
		}
		value, err := strconv.ParseFloat(strings.TrimSpace(power), 64)
		if err != nil {
			continue
		}
		watts[strings.TrimSpace(uuid)] = value
	}
	return watts, nil
}

// waitForKeplerToMatchNvidiaSMI waits until Kepler's GPU power agrees with
// nvidia-smi on several polls in a row, and returns both readings. Kepler
// reports the reading from its last refresh, so a single poll is not enough.
func waitForKeplerToMatchNvidiaSMI(t *testing.T, scraper *common.MetricsScraper) (kepler, smi map[string]float64) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), gpuSettleTimeout)
	defer cancel()

	matches := 0
	var smiErr error
	err := common.WaitForCondition(ctx, 2*time.Second, func() bool {
		kepler = keplerGPUWatts(scraper)
		smi, smiErr = nvidiaSMIWatts(ctx)
		if smiErr == nil && gpuWattsAgree(kepler, smi) {
			matches++
		} else {
			matches = 0
		}
		return matches >= 3
	})
	require.NoError(t, err, "Kepler GPU power never matched nvidia-smi: kepler=%v nvidia-smi=%v (last nvidia-smi error: %v)",
		kepler, smi, smiErr)
	return kepler, smi
}

// sumWatts adds up per-GPU readings.
func sumWatts(watts map[string]float64) float64 {
	var sum float64
	for _, w := range watts {
		sum += w
	}
	return sum
}

// gpuWattsAgree reports whether every GPU Kepler reports is within
// tolerance of nvidia-smi's reading for the same GPU.
func gpuWattsAgree(kepler, smi map[string]float64) bool {
	if len(kepler) == 0 {
		return false
	}
	for uuid, watts := range kepler {
		smiWatts, ok := smi[uuid]
		if !ok {
			return false
		}
		if math.Abs(watts-smiWatts) > math.Max(smiWatts*smiPercentTolerance, smiAbsTolerance) {
			return false
		}
	}
	return true
}

// gpuLoad is a container running dcgmproftester to keep the GPU busy.
type gpuLoad struct {
	name        string
	containerID string
}

// gpuLoadBinary returns the dcgmproftester to run in the load image.
// dcgmproftester only runs when its CUDA major version matches the driver's,
// so unless -gpu.load-binary is set it follows the CUDA version nvidia-smi
// reports, e.g. dcgmproftester13 for CUDA 13. The exception is a CUDA 13.0
// driver with a GPU below compute capability 7.5 (Maxwell, Pascal, Volta),
// which CUDA 13.0 dropped: there DCGM 4.6.1 requires dcgmproftester12.
func gpuLoadBinary(t *testing.T) string {
	t.Helper()

	if testConfig.gpuLoadBinary != "" {
		return testConfig.gpuLoadBinary
	}
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "nvidia-smi").Output()
	require.NoError(t, err, "nvidia-smi failed")
	match := regexp.MustCompile(`CUDA Version: (\d+)\.(\d+)`).FindSubmatch(out)
	require.NotNil(t, match, "nvidia-smi did not report a CUDA version")
	major, minor := string(match[1]), string(match[2])

	if major == "13" && minor == "0" {
		capOut, err := exec.CommandContext(ctx, "nvidia-smi", "-i", loadGPU,
			"--query-gpu=compute_cap", "--format=csv,noheader").Output()
		require.NoError(t, err, "nvidia-smi could not report the compute capability of GPU %s", loadGPU)
		computeCap, err := strconv.ParseFloat(strings.TrimSpace(string(capOut)), 64)
		require.NoError(t, err, "unexpected compute capability %q", capOut)
		if computeCap < 7.5 {
			return "/usr/bin/dcgmproftester12"
		}
	}
	return "/usr/bin/dcgmproftester" + major
}

// startGPULoad starts dcgmproftester on the tensor cores (field 1004) of GPU 0
// in the background and removes the container when the test ends, even if it
// fails to start. The container is kept until then so its logs can explain a
// load that failed to start.
func startGPULoad(t *testing.T) *gpuLoad {
	t.Helper()

	binary := gpuLoadBinary(t)
	t.Logf("Starting GPU load: %s from %s", binary, testConfig.gpuLoadImage)

	name := fmt.Sprintf("kepler-e2e-gpu-load-%d", time.Now().UnixNano())

	// Register the cleanup before docker run: docker can create the container
	// and then fail to start it, and the container must not outlive the test.
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
		defer cancel()
		_ = exec.CommandContext(ctx, "docker", "rm", "--force", name).Run() // best-effort cleanup
	})

	ctx, cancel := context.WithTimeout(context.Background(), gpuLoadStartTimeout)
	defer cancel()
	// Only loadGPU: dcgmproftester starts a worker for each GPU it can see, and
	// the attribution checks expect the load to hold all of that GPU's active
	// power. Any other GPUs stay idle.
	cmd := exec.CommandContext(ctx, "docker", "run", "--detach",
		"--gpus", "device="+loadGPU, "--cap-add", "SYS_ADMIN", "--name", name,
		"--entrypoint", binary, testConfig.gpuLoadImage,
		"--no-dcgm-validation", "-t", "1004", "-d", "120")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	out, err := cmd.Output()
	require.NoError(t, err, "Failed to start GPU load: %s", stderr.String())

	containerID := strings.TrimSpace(string(out))
	require.Len(t, containerID, 64, "docker run should print the full container ID")
	return &gpuLoad{name: name, containerID: containerID}
}

// skipIfGPUBusy skips the test when a process is already using a GPU. Kepler's
// process GPU power has no GPU label, so the attribution checks need the load
// to be the only GPU process on the host.
func skipIfGPUBusy(t *testing.T) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()
	pids, err := gpuComputePIDs(ctx)
	require.NoError(t, err, "nvidia-smi could not list the GPU processes")
	if len(pids) > 0 {
		t.Skipf("Skipping: other processes are using a GPU (pid %s), the load must be the only GPU process",
			strings.Join(pids, ", "))
	}
}

// gpuComputePIDs returns the host PIDs nvidia-smi lists as using a GPU.
func gpuComputePIDs(ctx context.Context) ([]string, error) {
	out, err := exec.CommandContext(ctx, "nvidia-smi", "--query-compute-apps=pid",
		"--format=csv,noheader").Output()
	if err != nil {
		return nil, err
	}
	return strings.Fields(string(out)), nil
}

// gpuLoadPIDs returns the host PIDs using a GPU, split into the load
// container's processes and all others.
func gpuLoadPIDs(ctx context.Context, load *gpuLoad) (loadPIDs, otherPIDs []string, err error) {
	top, err := exec.CommandContext(ctx, "docker", "top", load.name, "-eo", "pid").Output()
	if err != nil {
		return nil, nil, err
	}
	containerPIDs := strings.Fields(string(top))

	apps, err := gpuComputePIDs(ctx)
	if err != nil {
		return nil, nil, err
	}
	for _, pid := range apps {
		switch {
		case slices.Contains(loadPIDs, pid), slices.Contains(otherPIDs, pid):
		case slices.Contains(containerPIDs, pid):
			loadPIDs = append(loadPIDs, pid)
		default:
			otherPIDs = append(otherPIDs, pid)
		}
	}
	return loadPIDs, otherPIDs, nil
}

// processGPUWatts sums Kepler's GPU power for pids, and reports whether Kepler
// has all of them.
func processGPUWatts(s *common.MetricsSnapshot, pids []string) (float64, bool) {
	total := 0.0
	for _, pid := range pids {
		watts, found := s.GetValue("kepler_process_gpu_watts", map[string]string{"pid": pid})
		if !found {
			return 0, false
		}
		total += watts
	}
	return total, true
}

// waitForGPULoadPIDs waits until the load container's processes show up in
// nvidia-smi as using the GPU, and returns their host PIDs.
func waitForGPULoadPIDs(t *testing.T, load *gpuLoad) []string {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), gpuSettleTimeout)
	defer cancel()

	var pids []string
	err := common.WaitForCondition(ctx, time.Second, func() bool {
		loadPIDs, _, err := gpuLoadPIDs(ctx, load)
		if err != nil || len(loadPIDs) == 0 {
			return false
		}
		pids = loadPIDs
		return true
	})
	if err != nil {
		logCtx, logCancel := context.WithTimeout(context.Background(), commandTimeout)
		defer logCancel()
		logs, _ := exec.CommandContext(logCtx, "docker", "logs", "--tail", "20", load.name).CombinedOutput()
		require.NoError(t, err, "GPU load never showed up in nvidia-smi, container logs:\n%s", logs)
	}
	return pids
}
