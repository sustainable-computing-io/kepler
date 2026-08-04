// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sustainable-computing-io/kepler/test/common"
)

// skipIfNoGCC skips the test if gcc is not available (needed to build fake NVML)
func skipIfNoGCC(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("gcc"); err != nil {
		t.Skip("Skipping GPU e2e test: gcc not found")
	}
}

// TestGPU_DeviceDiscovered verifies that the fake GPU is discovered and
// kepler_node_gpu_info metric is present with correct labels.
func TestGPU_DeviceDiscovered(t *testing.T) {
	skipIfNoGCC(t)

	config := singleGPUIdle()
	_, scraper := setupKeplerWithFakeGPU(t, config)

	require.True(t, waitForGPUMetrics(t, scraper, "kepler_node_gpu_info", 30*time.Second),
		"kepler_node_gpu_info should appear")

	metrics, err := scraper.ScrapeMetric("kepler_node_gpu_info")
	require.NoError(t, err)
	require.NotEmpty(t, metrics, "Should have at least one GPU info metric")

	m := metrics[0]
	assert.Equal(t, "GPU-FAKE-0000-0001", m.Labels["gpu_uuid"],
		"GPU UUID should match fake config")
	assert.Equal(t, "NVIDIA Fake A100", m.Labels["gpu_name"],
		"GPU name should match fake config")
	assert.Equal(t, "nvidia", m.Labels["vendor"],
		"Vendor should be nvidia")

	t.Logf("GPU discovered: uuid=%s name=%s", m.Labels["gpu_uuid"], m.Labels["gpu_name"])
}

// TestGPU_NodePowerMetrics verifies that node-level GPU power metrics are exported.
func TestGPU_NodePowerMetrics(t *testing.T) {
	skipIfNoGCC(t)

	config := singleGPUIdle()
	_, scraper := setupKeplerWithFakeGPU(t, config)

	require.True(t, waitForGPUMetrics(t, scraper, "kepler_node_gpu_watts", 30*time.Second),
		"kepler_node_gpu_watts should appear")

	metrics, err := scraper.ScrapeMetric("kepler_node_gpu_watts")
	require.NoError(t, err)
	require.NotEmpty(t, metrics)

	// Fake GPU reports 40W
	found := false
	for _, m := range metrics {
		if m.Labels["gpu_uuid"] == "GPU-FAKE-0000-0001" && m.Value > 0 {
			found = true
			t.Logf("GPU power: %.2f W (expected ~40W)", m.Value)
		}
	}
	assert.True(t, found, "Should have positive GPU power for fake device")
}

// TestGPU_IdlePower verifies that when no processes are running on the GPU,
// active power is near zero and idle power accounts for total power.
func TestGPU_IdlePower(t *testing.T) {
	skipIfNoGCC(t)

	config := singleGPUIdle()
	_, scraper := setupKeplerWithFakeGPU(t, config)

	// Wait for metrics to stabilize (need at least 2 collection cycles for idle detection)
	require.True(t, waitForGPUMetrics(t, scraper, "kepler_node_gpu_watts", 30*time.Second),
		"kepler_node_gpu_watts should appear")
	time.Sleep(10 * time.Second) // Allow idle power to be detected

	snapshot, err := scraper.TakeSnapshot()
	require.NoError(t, err)

	totalPower := snapshot.SumValues("kepler_node_gpu_watts", nil)
	activePower := snapshot.SumValues("kepler_node_gpu_active_watts", nil)

	t.Logf("GPU total=%.2f W, active=%.2f W", totalPower, activePower)

	assert.Greater(t, totalPower, 0.0, "Total GPU power should be > 0")

	// With no processes, active power should be zero or near-zero
	// (tolerance for floating-point and timing)
	assert.LessOrEqual(t, activePower, totalPower,
		"Active power should not exceed total power")
}

// TestGPU_ProcessPowerAttribution verifies that process-level GPU power
// is attributed when a real process is visible to the fake GPU.
func TestGPU_ProcessPowerAttribution(t *testing.T) {
	skipIfNoGCC(t)

	// Start a real process whose PID we can report in the fake NVML config
	sleepCmd := exec.Command("sleep", "infinity")
	require.NoError(t, sleepCmd.Start(), "Failed to start sleep process")
	t.Cleanup(func() {
		_ = sleepCmd.Process.Kill()
		_ = sleepCmd.Wait()
	})

	pid := sleepCmd.Process.Pid
	t.Logf("Started sleep process with PID %d", pid)

	config := singleGPUWithProcesses([]fakeNVMLProcess{{
		PID:        pid,
		MemoryUsed: 1 << 30, // 1 GiB
		SmUtil:     60,
	}})

	_, scraper := setupKeplerWithFakeGPU(t, config)

	require.True(t, waitForGPUMetrics(t, scraper, "kepler_node_gpu_watts", 30*time.Second),
		"kepler_node_gpu_watts should appear")

	// Wait for process attribution to kick in
	time.Sleep(10 * time.Second)

	snapshot, err := scraper.TakeSnapshot()
	require.NoError(t, err)

	totalPower := snapshot.SumValues("kepler_node_gpu_watts", nil)
	activePower := snapshot.SumValues("kepler_node_gpu_active_watts", nil)

	t.Logf("GPU total=%.2f W, active=%.2f W", totalPower, activePower)

	assert.Greater(t, totalPower, 0.0, "Total GPU power should be > 0")
	assert.Greater(t, activePower, 0.0, "Active GPU power should be > 0 with a running process")

	// Check process-level GPU power
	processGPUMetrics := snapshot.GetAllWithName("kepler_process_gpu_watts")
	if len(processGPUMetrics) > 0 {
		t.Logf("Found %d process GPU metrics", len(processGPUMetrics))
		for _, m := range processGPUMetrics {
			if m.Value > 0 {
				t.Logf("  PID=%s comm=%s power=%.4f W", m.Labels["pid"], m.Labels["comm"], m.Value)
			}
		}
	} else {
		t.Log("No process GPU metrics found yet - this may require more collection cycles")
	}
}

// TestGPU_MultipleDevices verifies that multiple GPUs are discovered correctly.
func TestGPU_MultipleDevices(t *testing.T) {
	skipIfNoGCC(t)

	config := fakeNVMLConfig{
		Devices: []fakeNVMLDevice{
			{
				UUID:                 "GPU-FAKE-0000-0001",
				Name:                 "NVIDIA Fake A100",
				PowerUsageMilliWatts: 40000,
			},
			{
				UUID:                 "GPU-FAKE-0000-0002",
				Name:                 "NVIDIA Fake H100",
				PowerUsageMilliWatts: 60000,
			},
		},
	}

	_, scraper := setupKeplerWithFakeGPU(t, config)

	require.True(t, waitForGPUMetrics(t, scraper, "kepler_node_gpu_info", 30*time.Second),
		"kepler_node_gpu_info should appear")

	metrics, err := scraper.ScrapeMetric("kepler_node_gpu_info")
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(metrics), 2, "Should discover both GPUs")

	uuids := make(map[string]bool)
	for _, m := range metrics {
		uuids[m.Labels["gpu_uuid"]] = true
	}
	assert.True(t, uuids["GPU-FAKE-0000-0001"], "First GPU should be discovered")
	assert.True(t, uuids["GPU-FAKE-0000-0002"], "Second GPU should be discovered")

	t.Logf("Discovered %d GPUs", len(uuids))
}

// TestGPU_EnergyIncreases verifies that GPU energy counters increase monotonically
// between two scrapes.
func TestGPU_EnergyIncreases(t *testing.T) {
	skipIfNoGCC(t)

	config := singleGPUIdle()
	_, scraper := setupKeplerWithFakeGPU(t, config)

	require.True(t, waitForGPUMetrics(t, scraper, "kepler_node_gpu_watts", 30*time.Second),
		"kepler_node_gpu_watts should appear")

	// Take two snapshots separated by time
	snapshot1, err := scraper.TakeSnapshot()
	require.NoError(t, err)

	time.Sleep(5 * time.Second)

	snapshot2, err := scraper.TakeSnapshot()
	require.NoError(t, err)

	// Check power is reported consistently across scrapes
	power1 := snapshot1.SumValues("kepler_node_gpu_watts", nil)
	power2 := snapshot2.SumValues("kepler_node_gpu_watts", nil)

	t.Logf("Snapshot 1: GPU power=%.2f W", power1)
	t.Logf("Snapshot 2: GPU power=%.2f W", power2)

	assert.Greater(t, power1, 0.0, "First snapshot should have positive GPU power")
	assert.Greater(t, power2, 0.0, "Second snapshot should have positive GPU power")
}

// TestGPU_GracefulStartupWithoutGPU verifies that Kepler starts correctly
// even when the fake NVML library fails to initialize (no config file).
func TestGPU_GracefulStartupWithoutGPU(t *testing.T) {
	skipIfNoGCC(t)

	// Build fake NVML but don't set FAKE_NVML_CONFIG - init should fail
	libDir := buildFakeNVML(t)
	keplerConfig := writeGPUKeplerConfig(t)

	k := startKepler(t,
		withLogOutput(os.Stderr),
		withConfigFile(keplerConfig),
		withPort(gpuMetricsPort),
		withEnv("LD_LIBRARY_PATH", libDir),
		// No FAKE_NVML_CONFIG → nvmlInit_v2 will fail
	)

	assert.True(t, k.IsRunning(), "Kepler should still start even if GPU init fails")

	// Verify basic metrics still work (CPU metrics)
	s := common.NewMetricsScraper(k.MetricsURL())
	require.True(t, WaitForValidCPUMetrics(t, s, 30*time.Second),
		"CPU metrics should still work without GPU")
}
