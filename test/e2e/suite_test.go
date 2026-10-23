// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"flag"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sustainable-computing-io/kepler/test/common"
)

// Timing constants for e2e tests.
// Based on Kepler's monitor.interval (3s) from e2e-config.yaml.
const (
	// waitForMetricsAvailable: shorter wait for basic metric availability checks.
	// Requires ~1-2 collection cycles.
	waitForMetricsAvailable = 5 * time.Second

	// waitBetweenSnapshots: interval between metric snapshots for delta/trend tests.
	waitBetweenSnapshots = 5 * time.Second

	// waitBetweenSamples: short interval between samples in loop tests.
	waitBetweenSamples = 3 * time.Second

	// waitForProcessStabilization: short wait for process to stabilize after start.
	waitForProcessStabilization = 500 * time.Millisecond
)

// testConfig holds test configuration from flags
var testConfig = struct {
	keplerBinary  string
	metricsPort   int
	configFile    string
	gpuConfigFile string
	fakeGPU       bool
	gpuLoadImage  string
	gpuLoadBinary string
}{
	keplerBinary:  "bin/kepler",
	metricsPort:   28282,
	configFile:    "",
	gpuConfigFile: "",
	fakeGPU:       false,
	gpuLoadImage:  "nvcr.io/nvidia/cloud-native/dcgm:4.6.1-1-ubuntu24.04",
	gpuLoadBinary: "",
}

func init() {
	flag.StringVar(&testConfig.keplerBinary, "kepler.binary", testConfig.keplerBinary,
		"Path to Kepler binary")
	flag.IntVar(&testConfig.metricsPort, "kepler.port", testConfig.metricsPort,
		"Port for Kepler metrics endpoint")
	flag.StringVar(&testConfig.configFile, "kepler.config", testConfig.configFile,
		"Path to Kepler config file")
	flag.BoolVar(&testConfig.fakeGPU, "gpu.fake", testConfig.fakeGPU,
		"Run the GPU tests against the fake GPU meter instead of a real NVIDIA GPU")
	flag.StringVar(&testConfig.gpuLoadImage, "gpu.load-image", testConfig.gpuLoadImage,
		"Container image with dcgmproftester, used to load a real GPU")
	flag.StringVar(&testConfig.gpuLoadBinary, "gpu.load-binary", testConfig.gpuLoadBinary,
		"dcgmproftester binary inside the GPU load image (default: the one matching the driver's CUDA version)")
}

func TestMain(m *testing.M) {
	flag.Parse()

	// Find config file if not specified. The GPU tests read a real GPU with
	// e2e-gpu-config.yaml unless -gpu.fake is set. Only they use it: it selects
	// the fake CPU meter, which would let the CPU tests pass without reading RAPL.
	explicitConfig := testConfig.configFile != ""
	if !explicitConfig {
		testConfig.configFile = findConfigFile("e2e-config.yaml")
	}
	testConfig.gpuConfigFile = testConfig.configFile
	if !testConfig.fakeGPU && !explicitConfig {
		testConfig.gpuConfigFile = findConfigFile("e2e-gpu-config.yaml")
	}

	os.Exit(m.Run())
}

func findConfigFile(name string) string {
	candidates := []string{
		filepath.Join("test/testdata", name),
		filepath.Join("../testdata", name),
		filepath.Join("testdata", name),
	}

	for _, c := range candidates {
		if absPath, err := filepath.Abs(c); err == nil {
			if _, err := os.Stat(absPath); err == nil {
				return absPath
			}
		}
	}
	return ""
}

// setupKeplerForTest starts Kepler for a test
func setupKeplerForTest(t *testing.T) (*KeplerInstance, *common.MetricsScraper) {
	t.Helper()
	requireE2EPrerequisites(t)

	k := startKepler(t, withLogOutput(os.Stderr))
	return k, common.NewMetricsScraper(k.MetricsURL())
}

// setupKeplerWithWorkloadSupport starts Kepler with workload prerequisites
func setupKeplerWithWorkloadSupport(t *testing.T) (*KeplerInstance, *common.MetricsScraper) {
	t.Helper()
	requireWorkloadPrerequisites(t)

	k := startKepler(t, withLogOutput(os.Stderr))
	return k, common.NewMetricsScraper(k.MetricsURL())
}
