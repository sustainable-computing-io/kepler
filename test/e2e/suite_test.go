// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"flag"
	"os"
	"path/filepath"
	"testing"
	"time"
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
	keplerBinary string
	metricsPort  int
	configFile   string
}{
	keplerBinary: "bin/kepler",
	metricsPort:  28282,
	configFile:   "",
}

func init() {
	flag.StringVar(&testConfig.keplerBinary, "kepler.binary", testConfig.keplerBinary,
		"Path to Kepler binary")
	flag.IntVar(&testConfig.metricsPort, "kepler.port", testConfig.metricsPort,
		"Port for Kepler metrics endpoint")
	flag.StringVar(&testConfig.configFile, "kepler.config", testConfig.configFile,
		"Path to Kepler config file")
}

func TestMain(m *testing.M) {
	flag.Parse()

	// Find config file if not specified
	if testConfig.configFile == "" {
		testConfig.configFile = findConfigFile()
	}

	os.Exit(m.Run())
}

func findConfigFile() string {
	candidates := []string{
		"test/testdata/e2e-config.yaml",
		"../testdata/e2e-config.yaml",
		"testdata/e2e-config.yaml",
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
func setupKeplerForTest(t *testing.T) (*KeplerInstance, *MetricsScraper) {
	t.Helper()
	requireE2EPrerequisites(t)

	k := startKepler(t, withLogOutput(os.Stderr))
	return k, NewMetricsScraper(k.MetricsURL())
}

// setupKeplerWithWorkloadSupport starts Kepler with workload prerequisites
func setupKeplerWithWorkloadSupport(t *testing.T) (*KeplerInstance, *MetricsScraper) {
	t.Helper()
	requireWorkloadPrerequisites(t)

	k := startKepler(t, withLogOutput(os.Stderr))
	return k, NewMetricsScraper(k.MetricsURL())
}
