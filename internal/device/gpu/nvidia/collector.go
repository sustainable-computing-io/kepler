// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package nvidia

import (
	"log/slog"
	"sync"

	"golang.org/x/sync/singleflight"

	"github.com/sustainable-computing-io/kepler/internal/device"
	"github.com/sustainable-computing-io/kepler/internal/device/gpu"
)

func init() {
	gpu.Register(gpu.VendorNVIDIA, func(logger *slog.Logger) (gpu.GPUPowerMeter, error) {
		return NewGPUPowerCollector(logger)
	})
}

// GPUPowerCollector implements gpu.GPUPowerMeter for NVIDIA GPUs.
// It uses NVML for device discovery, power readings, and process utilization.
type GPUPowerCollector struct {
	logger   *slog.Logger
	nvml     NVMLBackend
	detector SharingModeDetector

	devices      []gpu.GPUDevice
	sharingModes map[int]gpu.SharingMode

	// minObservedPower tracks minimum power per device for idle detection
	minObservedPower map[string]float64

	mu sync.RWMutex

	// Singleflight to coalesce concurrent GetProcessPower calls.
	// Prometheus scrapes can overlap - this ensures only one NVML collection
	// runs at a time, preventing contention and gaps in metrics.
	processPowerGroup singleflight.Group
}

// NewGPUPowerCollector creates a new NVIDIA GPU power collector.
func NewGPUPowerCollector(logger *slog.Logger) (*GPUPowerCollector, error) {
	if logger == nil {
		logger = slog.Default()
	}

	nvmlBackend := NewNVMLBackend(logger)

	return &GPUPowerCollector{
		logger:           logger.With("component", "nvidia-gpu-collector"),
		nvml:             nvmlBackend,
		minObservedPower: make(map[string]float64),
		sharingModes:     make(map[int]gpu.SharingMode),
	}, nil
}

// Name returns the service name
func (c *GPUPowerCollector) Name() string {
	return "nvidia-gpu-power-collector"
}

// Init initializes the NVML backend and discovers devices
func (c *GPUPowerCollector) Init() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.nvml.Init(); err != nil {
		return err
	}

	devices, err := c.nvml.DiscoverDevices()
	if err != nil {
		return err
	}
	c.devices = devices

	// Initialize detector and detect sharing modes
	c.detector = NewSharingModeDetector(c.logger, c.nvml)
	modes, err := c.detector.DetectAllModes()
	if err != nil {
		c.logger.Warn("failed to detect sharing modes", "error", err)
	}
	c.sharingModes = modes

	// Log detected modes
	for idx, mode := range modes {
		c.logger.Info("GPU sharing mode detected",
			"device", idx,
			"mode", mode.String())
	}

	return nil
}

// Shutdown cleans up NVML resources
func (c *GPUPowerCollector) Shutdown() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.nvml.Shutdown()
}

// Vendor returns the GPU vendor
func (c *GPUPowerCollector) Vendor() gpu.Vendor {
	return gpu.VendorNVIDIA
}

// Devices returns all discovered GPU devices
func (c *GPUPowerCollector) Devices() []gpu.GPUDevice {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.devices
}

// GetPowerUsage returns the current power consumption for a device
func (c *GPUPowerCollector) GetPowerUsage(deviceIndex int) (device.Power, error) {
	dev, err := c.nvml.GetDevice(deviceIndex)
	if err != nil {
		return 0, err
	}

	power, err := dev.GetPowerUsage()
	if err != nil {
		return 0, err
	}

	// Track minimum observed power for idle detection
	c.mu.Lock()
	uuid := dev.UUID()
	powerWatts := power.Watts()
	if min, exists := c.minObservedPower[uuid]; !exists || powerWatts < min {
		c.minObservedPower[uuid] = powerWatts
	}
	c.mu.Unlock()

	return power, nil
}

// GetTotalEnergy returns cumulative energy consumption for a device
func (c *GPUPowerCollector) GetTotalEnergy(deviceIndex int) (device.Energy, error) {
	dev, err := c.nvml.GetDevice(deviceIndex)
	if err != nil {
		return 0, err
	}

	return dev.GetTotalEnergy()
}

// GetDevicePowerStats returns power statistics including idle power detection
func (c *GPUPowerCollector) GetDevicePowerStats(deviceIndex int) (gpu.GPUPowerStats, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.getDevicePowerStatsLocked(deviceIndex)
}

// getDevicePowerStatsLocked is the internal version that assumes the lock is already held
func (c *GPUPowerCollector) getDevicePowerStatsLocked(deviceIndex int) (gpu.GPUPowerStats, error) {
	dev, err := c.nvml.GetDevice(deviceIndex)
	if err != nil {
		return gpu.GPUPowerStats{}, err
	}

	power, err := dev.GetPowerUsage()
	if err != nil {
		return gpu.GPUPowerStats{}, err
	}

	totalPower := power.Watts()

	// Update minimum observed power
	uuid := dev.UUID()
	if min, exists := c.minObservedPower[uuid]; !exists || totalPower < min {
		c.minObservedPower[uuid] = totalPower
	}
	idlePower := c.minObservedPower[uuid]

	activePower := totalPower - idlePower
	if activePower < 0 {
		activePower = 0
	}

	return gpu.GPUPowerStats{
		TotalPower:  totalPower,
		IdlePower:   idlePower,
		ActivePower: activePower,
	}, nil
}

// processPowerResult wraps the result for singleflight (which only returns interface{})
type processPowerResult struct {
	power map[uint32]float64
	err   error
}

// GetProcessPower returns power attribution per process.
// Uses singleflight to coalesce concurrent Prometheus scrape calls - only one
// NVML collection runs at a time, preventing contention and gaps in metrics.
func (c *GPUPowerCollector) GetProcessPower() (map[uint32]float64, error) {
	result, _, _ := c.processPowerGroup.Do("process-power", func() (interface{}, error) {
		return c.collectProcessPower(), nil
	})

	r := result.(processPowerResult)
	return r.power, r.err
}

// collectProcessPower is the internal implementation called via singleflight.
func (c *GPUPowerCollector) collectProcessPower() processPowerResult {
	c.mu.Lock()
	defer c.mu.Unlock()

	result := make(map[uint32]float64)

	for _, dev := range c.devices {
		mode := c.sharingModes[dev.Index]

		switch mode {
		case gpu.SharingModePartitioned:
			// Partitioned (MIG) support will be added in PR-3
			c.logger.Debug("partitioned mode detected, skipping (not yet implemented)",
				"device", dev.Index)
			continue

		case gpu.SharingModeExclusive:
			if err := c.attributeExclusive(dev.Index, result); err != nil {
				c.logger.Debug("exclusive attribution failed",
					"device", dev.Index, "error", err)
			}

		default: // Time-slicing
			if err := c.attributeTimeSlicing(dev.Index, result); err != nil {
				c.logger.Debug("time-slicing attribution failed",
					"device", dev.Index, "error", err)
			}
		}
	}

	return processPowerResult{power: result, err: nil}
}

// attributeExclusive assigns 100% of active power to the single process
// NOTE: caller must hold c.mu lock
func (c *GPUPowerCollector) attributeExclusive(deviceIndex int, result map[uint32]float64) error {
	nvmlDev, err := c.nvml.GetDevice(deviceIndex)
	if err != nil {
		return err
	}

	// Get active power
	stats, err := c.getDevicePowerStatsLocked(deviceIndex)
	if err != nil {
		return err
	}

	// Get running processes
	procs, err := nvmlDev.GetComputeRunningProcesses()
	if err != nil {
		return err
	}

	if len(procs) == 0 {
		return nil
	}

	// In exclusive mode, attribute all active power to the single process
	// (or split equally if somehow multiple processes exist)
	powerPerProc := stats.ActivePower / float64(len(procs))
	for _, p := range procs {
		result[p.PID] += powerPerProc
	}

	return nil
}

// attributeTimeSlicing distributes power based on SM utilization
// NOTE: caller must hold c.mu lock
func (c *GPUPowerCollector) attributeTimeSlicing(deviceIndex int, result map[uint32]float64) error {
	nvmlDev, err := c.nvml.GetDevice(deviceIndex)
	if err != nil {
		return err
	}

	// Get active power
	stats, err := c.getDevicePowerStatsLocked(deviceIndex)
	if err != nil {
		return err
	}

	// Step 1: Get list of running processes (authoritative list)
	runningProcs, err := nvmlDev.GetComputeRunningProcesses()
	if err != nil {
		c.logger.Debug("GetComputeRunningProcesses failed", "device", deviceIndex, "error", err)
		return err
	}

	if len(runningProcs) == 0 {
		return nil
	}

	// Step 2: Get process utilization samples (always pass 0 to get latest samples)
	utils, err := nvmlDev.GetProcessUtilization(0)
	if err != nil {
		// Fall back to equal distribution among running processes
		c.logger.Debug("GetProcessUtilization unavailable, using equal distribution",
			"device", deviceIndex, "error", err)
		powerPerProc := stats.ActivePower / float64(len(runningProcs))
		for _, p := range runningProcs {
			result[p.PID] += powerPerProc
		}
		return nil
	}

	// Step 3: Build utilization map by PID
	utilMap := make(map[uint32]uint32) // PID -> ComputeUtil
	for _, pu := range utils {
		// Keep the highest utilization for each PID (samples may have duplicates)
		if existing, ok := utilMap[pu.PID]; !ok || pu.ComputeUtil > existing {
			utilMap[pu.PID] = pu.ComputeUtil
		}
	}

	c.logger.Debug("GetProcessUtilization result",
		"device", deviceIndex,
		"runningProcs", len(runningProcs),
		"utilSamples", len(utils),
		"utilMapSize", len(utilMap),
		"totalPower", stats.TotalPower,
		"idlePower", stats.IdlePower,
		"activePower", stats.ActivePower)

	// Step 4: Calculate total SM utilization across running processes
	var totalSmUtil uint32
	for _, proc := range runningProcs {
		if smUtil, ok := utilMap[proc.PID]; ok {
			totalSmUtil += smUtil
		}
	}

	// If no utilization data, distribute equally among running processes
	if totalSmUtil == 0 {
		powerPerProc := stats.ActivePower / float64(len(runningProcs))
		for _, proc := range runningProcs {
			result[proc.PID] += powerPerProc
		}
		c.logger.Debug("no utilization data, using equal distribution",
			"device", deviceIndex,
			"processes", len(runningProcs),
			"powerPerProcess", powerPerProc)
		return nil
	}

	// Step 5: Distribute active power proportionally to SM utilization
	for _, proc := range runningProcs {
		smUtil := utilMap[proc.PID] // 0 if not in map
		fraction := float64(smUtil) / float64(totalSmUtil)
		result[proc.PID] += stats.ActivePower * fraction
	}

	return nil
}

// GetProcessInfo returns detailed GPU metrics per process
func (c *GPUPowerCollector) GetProcessInfo() ([]gpu.ProcessGPUInfo, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var allProcs []gpu.ProcessGPUInfo

	for _, dev := range c.devices {
		nvmlDev, err := c.nvml.GetDevice(dev.Index)
		if err != nil {
			continue
		}

		procs, err := nvmlDev.GetComputeRunningProcesses()
		if err != nil {
			continue
		}

		allProcs = append(allProcs, procs...)
	}

	return allProcs, nil
}

// Ensure GPUPowerCollector implements gpu.GPUPowerMeter
var _ gpu.GPUPowerMeter = (*GPUPowerCollector)(nil)
