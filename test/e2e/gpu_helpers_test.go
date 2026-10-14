// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/sustainable-computing-io/kepler/test/common"
)

const gpuMetricsPort = 28283

// fakeNVMLDevice represents a GPU device in the fake NVML config
type fakeNVMLDevice struct {
	UUID                  string            `json:"uuid"`
	Name                  string            `json:"name"`
	PowerUsageMilliWatts  uint              `json:"powerUsageMilliWatts"`
	ComputeMode           int               `json:"computeMode"`
	MIGEnabled            bool              `json:"migEnabled"`
	MaxMIGDevices         int               `json:"maxMigDevices"`
	Processes             []fakeNVMLProcess `json:"processes"`
}

// fakeNVMLProcess represents a process using the GPU
type fakeNVMLProcess struct {
	PID        int    `json:"pid"`
	MemoryUsed uint64 `json:"memoryUsed"`
	SmUtil     uint   `json:"smUtil"`
}

// fakeNVMLConfig is the top-level config for the fake NVML library
type fakeNVMLConfig struct {
	Devices []fakeNVMLDevice `json:"devices"`
}

// buildFakeNVML compiles the fake libnvidia-ml.so.1 and returns the directory containing it
func buildFakeNVML(t *testing.T) string {
	t.Helper()

	// Look for source files relative to common locations
	candidates := []string{
		"test/e2e/fake_nvml",
		"fake_nvml",
		"../e2e/fake_nvml",
	}

	var fakeDir string
	for _, c := range candidates {
		if absPath, err := filepath.Abs(c); err == nil {
			if _, err := os.Stat(filepath.Join(absPath, "fake_nvml.c")); err == nil {
				fakeDir = absPath
				break
			}
		}
	}
	if fakeDir == "" {
		t.Fatal("Cannot find fake_nvml source directory")
	}

	srcFiles := []string{
		filepath.Join(fakeDir, "fake_nvml.c"),
		filepath.Join(fakeDir, "cJSON.c"),
	}
	for _, f := range srcFiles {
		if _, err := os.Stat(f); os.IsNotExist(err) {
			t.Fatalf("Source file not found: %s", f)
		}
	}

	outDir := t.TempDir()
	outPath := filepath.Join(outDir, "libnvidia-ml.so.1")

	cmd := exec.Command("gcc",
		"-shared", "-fPIC",
		"-o", outPath,
		srcFiles[0], srcFiles[1],
		"-lpthread",
	)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to compile fake NVML: %v", err)
	}

	t.Logf("Built fake NVML at %s", outPath)
	return outDir
}

// writeFakeNVMLConfig writes a JSON config file for the fake NVML library
func writeFakeNVMLConfig(t *testing.T, config fakeNVMLConfig) string {
	t.Helper()

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal fake NVML config: %v", err)
	}

	configPath := filepath.Join(t.TempDir(), "fake_nvml_config.json")
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatalf("Failed to write fake NVML config: %v", err)
	}

	return configPath
}

// writeGPUKeplerConfig writes a Kepler config YAML with GPU enabled
func writeGPUKeplerConfig(t *testing.T) string {
	t.Helper()

	config := `# SPDX-FileCopyrightText: 2025 The Kepler Authors
# SPDX-License-Identifier: Apache-2.0
log:
  level: debug
  format: text
monitor:
  interval: 3s
  staleness: 10s
  maxTerminated: 100
  minTerminatedEnergyThreshold: 1
host:
  procfs: /proc
  sysfs: /sys
exporter:
  prometheus:
    enabled: true
    metricsLevel:
      - node
      - process
      - container
      - vm
      - pod
web:
  listenAddresses:
    - :` + fmt.Sprintf("%d", gpuMetricsPort) + `
kube:
  enabled: false
dev:
  fake-cpu-meter:
    enabled: true
experimental:
  gpu:
    enabled: true
    idlePower: 0
`
	configPath := filepath.Join(t.TempDir(), "gpu-e2e-config.yaml")
	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		t.Fatalf("Failed to write GPU kepler config: %v", err)
	}

	return configPath
}

// setupKeplerWithFakeGPU starts Kepler with the fake NVML library
func setupKeplerWithFakeGPU(t *testing.T, config fakeNVMLConfig) (*KeplerInstance, *common.MetricsScraper) {
	t.Helper()

	// Build fake NVML shared library
	libDir := buildFakeNVML(t)

	// Write fake NVML config JSON
	nvmlConfigPath := writeFakeNVMLConfig(t, config)

	// Write Kepler config with GPU enabled
	keplerConfig := writeGPUKeplerConfig(t)

	// Start Kepler with env vars pointing to the fake library
	k := startKepler(t,
		withLogOutput(os.Stderr),
		withConfigFile(keplerConfig),
		withPort(gpuMetricsPort),
		withEnv("LD_LIBRARY_PATH", libDir),
		withEnv("FAKE_NVML_CONFIG", nvmlConfigPath),
	)

	return k, common.NewMetricsScraper(k.MetricsURL())
}

// singleGPUIdle returns a config for 1 idle GPU with no processes
func singleGPUIdle() fakeNVMLConfig {
	return fakeNVMLConfig{
		Devices: []fakeNVMLDevice{{
			UUID:                 "GPU-FAKE-0000-0001",
			Name:                 "NVIDIA Fake A100",
			PowerUsageMilliWatts: 40000, // 40W idle
			ComputeMode:          0,     // default (time-slicing)
			Processes:            nil,
		}},
	}
}

// singleGPUWithProcesses returns a config for 1 GPU with active processes
func singleGPUWithProcesses(processes []fakeNVMLProcess) fakeNVMLConfig {
	return fakeNVMLConfig{
		Devices: []fakeNVMLDevice{{
			UUID:                 "GPU-FAKE-0000-0001",
			Name:                 "NVIDIA Fake A100",
			PowerUsageMilliWatts: 150000, // 150W under load
			ComputeMode:          0,
			Processes:            processes,
		}},
	}
}

// waitForGPUMetrics waits for GPU metrics to appear in Kepler's output
func waitForGPUMetrics(t *testing.T, scraper *common.MetricsScraper, metricName string, timeout time.Duration) bool {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	err := common.WaitForCondition(ctx, 2*time.Second, func() bool {
		metrics, err := scraper.ScrapeMetric(metricName)
		if err != nil {
			return false
		}
		return len(metrics) > 0
	})
	if err != nil {
		t.Logf("Timeout waiting for GPU metric %s", metricName)
		return false
	}
	return true
}
