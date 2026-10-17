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
)

// setupKeplerForGPUTest starts Kepler for a GPU test. With -gpu.real it does
// not need RAPL, so the GPU tests also run on GPU hosts without RAPL, such as
// cloud VMs. e2e-gpu-config.yaml uses the fake CPU meter on those hosts.
func setupKeplerForGPUTest(t *testing.T) (*KeplerInstance, *common.MetricsScraper) {
	t.Helper()

	if !testConfig.realGPU {
		requireE2EPrerequisites(t)
	}

	k := startKepler(t, withLogOutput(os.Stderr))
	return k, common.NewMetricsScraper(k.MetricsURL())
}

// requireRealGPU skips the test unless it runs with -gpu.real, where Kepler
// reads a real NVIDIA GPU through NVML.
func requireRealGPU(t *testing.T) {
	t.Helper()

	if !testConfig.realGPU {
		t.Skip("Skipping: needs a real GPU, run with -gpu.real")
	}
	_, err := exec.LookPath("nvidia-smi")
	require.NoError(t, err, "-gpu.real needs nvidia-smi")
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
	pid := waitForGPULoadPID(t, load)
	t.Logf("GPU load running: container=%s pid=%s", load.containerID[:12], pid)

	// Wait for a refresh taken while the load holds the GPU at full power:
	// the load has GPU power, and Kepler's total matches nvidia-smi, so the
	// refresh did not land while the GPU was still ramping up.
	ctx, cancel := context.WithTimeout(context.Background(), gpuSettleTimeout)
	defer cancel()

	var snapshot *common.MetricsSnapshot
	var loadSMI map[string]float64
	err = common.WaitForCondition(ctx, 2*time.Second, func() bool {
		s, err := scraper.TakeSnapshot()
		if err != nil {
			return false
		}
		active := s.SumValues("kepler_node_gpu_active_watts", nil)
		loadWatts, found := s.GetValue("kepler_process_gpu_watts", map[string]string{"pid": pid})
		if !found || loadWatts <= 0 || active < minLoadActiveWatts {
			return false
		}
		smi, err := nvidiaSMIWatts(ctx)
		if err != nil || !gpuWattsAgree(gpuWattsByUUID(s.GetAllWithName("kepler_node_gpu_watts")), smi) {
			return false
		}
		snapshot, loadSMI = s, smi
		return true
	})
	require.NoError(t, err, "Kepler never attributed GPU power to the load (pid %s)", pid)

	total := snapshot.SumValues("kepler_node_gpu_watts", nil)
	idle := snapshot.SumValues("kepler_node_gpu_idle_watts", nil)
	active := snapshot.SumValues("kepler_node_gpu_active_watts", nil)
	running := snapshot.SumValues("kepler_process_gpu_watts", map[string]string{"state": "running"})
	loadWatts, _ := snapshot.GetValue("kepler_process_gpu_watts", map[string]string{"pid": pid})
	containerWatts, containerFound := snapshot.GetValue("kepler_container_gpu_watts",
		map[string]string{"container_id": load.containerID})

	t.Logf("under load: nvidia-smi=%.2f W, node total=%.2f W, idle=%.2f W, active=%.2f W, "+
		"running processes=%.2f W, load pid=%.2f W, load container=%.2f W",
		sumWatts(loadSMI), total, idle, active, running, loadWatts, containerWatts)

	// Running processes share all of the node's active GPU power.
	assertWithinTolerance(t, active, running, powerTolerance, absolutePowerTolerance,
		"sum of process GPU power should equal node active GPU power")

	// The load is the only process using the GPU, so it gets nearly all of it.
	assert.GreaterOrEqual(t, loadWatts, 0.9*active,
		"the load should get most of the active GPU power")

	for _, m := range snapshot.GetAllWithName("kepler_process_gpu_watts") {
		if m.Labels["pid"] == pid {
			assert.Equal(t, load.containerID, m.Labels["container_id"],
				"the load process should be attributed to its container")
		}
	}

	require.True(t, containerFound, "kepler_container_gpu_watts should exist for the load container")
	assertWithinTolerance(t, loadWatts, containerWatts, powerTolerance, absolutePowerTolerance,
		"the load container should get its process's GPU power")
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
// reports, e.g. dcgmproftester13 for CUDA 13.
func gpuLoadBinary(t *testing.T) string {
	t.Helper()

	if testConfig.gpuLoadBinary != "" {
		return testConfig.gpuLoadBinary
	}
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "nvidia-smi").Output()
	require.NoError(t, err, "nvidia-smi failed")
	match := regexp.MustCompile(`CUDA Version: (\d+)`).FindSubmatch(out)
	require.NotNil(t, match, "nvidia-smi did not report a CUDA version")
	return "/usr/bin/dcgmproftester" + string(match[1])
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
	// Only GPU 0: dcgmproftester starts a worker for each GPU it can see, and
	// the attribution checks expect one load process to hold all of the node's
	// active GPU power. Any other GPUs stay idle.
	cmd := exec.CommandContext(ctx, "docker", "run", "--detach",
		"--gpus", "device=0", "--cap-add", "SYS_ADMIN", "--name", name,
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

// waitForGPULoadPID waits until one of the load container's processes shows
// up in nvidia-smi as using the GPU, and returns its host PID.
func waitForGPULoadPID(t *testing.T, load *gpuLoad) string {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), gpuSettleTimeout)
	defer cancel()

	var pid string
	err := common.WaitForCondition(ctx, time.Second, func() bool {
		top, err := exec.CommandContext(ctx, "docker", "top", load.name, "-eo", "pid").Output()
		if err != nil {
			return false
		}
		containerPIDs := strings.Fields(string(top))

		apps, err := exec.CommandContext(ctx, "nvidia-smi", "--query-compute-apps=pid",
			"--format=csv,noheader").Output()
		if err != nil {
			return false
		}
		for _, gpuPID := range strings.Fields(string(apps)) {
			if slices.Contains(containerPIDs, gpuPID) {
				pid = gpuPID
				return true
			}
		}
		return false
	})
	if err != nil {
		logCtx, logCancel := context.WithTimeout(context.Background(), commandTimeout)
		defer logCancel()
		logs, _ := exec.CommandContext(logCtx, "docker", "logs", "--tail", "20", load.name).CombinedOutput()
		require.NoError(t, err, "GPU load never showed up in nvidia-smi, container logs:\n%s", logs)
	}
	return pid
}
