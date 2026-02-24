// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package nvidia

import (
	"context"
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

	// minObservedPower tracks minimum power per device UUID, updated only when
	// no compute processes are running (true idle).
	minObservedPower map[string]float64

	// idleObserved tracks whether we have seen a true idle reading per GPU UUID.
	// Until true idle is observed, we use idlePower (if configured) or 0.
	idleObserved map[string]bool

	// idlePower is a user-configured idle power in Watts.
	// When set (> 0), always used instead of observed idle power. 0 means auto-detect.
	idlePower float64

	// MIG support
	dcgm                 DCGMBackend
	dcgmEndpoint         string // pre-configured endpoint, set before Init()
	migInstancesByDevice map[int][]MIGGPUInstance

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
		idleObserved:     make(map[string]bool),
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

	// Log detected modes and check for MIG
	hasMIG := false
	for idx, mode := range modes {
		c.logger.Info("GPU sharing mode detected",
			"device", idx,
			"mode", mode.String())
		if mode == gpu.SharingModePartitioned {
			hasMIG = true
		}
	}

	// Initialize DCGM backend if MIG is detected
	if hasMIG {
		dcgm := NewDCGMExporterBackend(c.logger)

		// If endpoint was pre-configured via SetDCGMEndpoint, it's already set
		if c.dcgmEndpoint != "" {
			dcgm.SetEndpoint(c.dcgmEndpoint)
		}

		if err := dcgm.Init(context.Background()); err != nil {
			c.logger.Warn("DCGM backend initialization failed, MIG power attribution will use fallback",
				"error", err)
		} else {
			c.dcgm = dcgm
		}

		// Cache MIG hierarchy from NVML (static topology)
		if err := c.cacheMIGHierarchy(); err != nil {
			c.logger.Warn("failed to cache MIG hierarchy", "error", err)
		}
	}

	return nil
}

// Shutdown cleans up NVML and DCGM resources
func (c *GPUPowerCollector) Shutdown() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.dcgm != nil && c.dcgm.IsInitialized() {
		if err := c.dcgm.Shutdown(); err != nil {
			c.logger.Warn("DCGM shutdown error", "error", err)
		}
	}

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

	return dev.GetPowerUsage()
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

// getDevicePowerStatsLocked is the internal version that assumes the lock is already held.
// It only updates minObservedPower when no compute processes are running (true idle),
// preventing false baselines when Kepler starts under GPU load.
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
	uuid := dev.UUID()

	// Check if the GPU is truly idle (no compute processes running)
	procs, err := dev.GetComputeRunningProcesses()
	if err != nil {
		// Non-fatal: log and skip idle detection for this reading
		c.logger.Debug("GetComputeRunningProcesses failed, skipping idle detection",
			"device", deviceIndex, "error", err)
	} else if len(procs) == 0 {
		// GPU is truly idle — update minimum observed power
		if min, exists := c.minObservedPower[uuid]; !exists || totalPower < min {
			c.minObservedPower[uuid] = totalPower
			c.logger.Debug("updated idle power baseline",
				"device", deviceIndex, "uuid", uuid, "idlePower", totalPower)
		}
		c.idleObserved[uuid] = true
	}

	// Determine idle power:
	// 1. User-configured default (if > 0)
	// 2. Observed idle power (if we've seen true idle)
	// 3. Conservative fallback: 0 (all power attributed as active)
	var idlePower float64
	switch {
	case c.idlePower > 0:
		idlePower = c.idlePower
	case c.idleObserved[uuid]:
		idlePower = c.minObservedPower[uuid]
	default:
		idlePower = 0
	}

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

// SetIdlePower sets the configured idle power in Watts.
// When set (> 0), this value always takes precedence over observed idle power.
// Set to 0 to use auto-detected idle power. Negative values are clamped to 0.
func (c *GPUPowerCollector) SetIdlePower(watts float64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if watts < 0 {
		watts = 0
	}
	c.idlePower = watts
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
			if err := c.attributePartitioned(dev.Index, result); err != nil {
				c.logger.Debug("MIG attribution failed",
					"device", dev.Index, "error", err)
			}

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

// SetDCGMEndpoint sets the dcgm-exporter endpoint URL.
// Must be called before Init() to take effect.
func (c *GPUPowerCollector) SetDCGMEndpoint(endpoint string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.dcgmEndpoint = endpoint
}

// cacheMIGHierarchy enumerates all MIG instances from NVML at startup.
// This is static data — MIG topology doesn't change without admin action.
// NOTE: caller must hold c.mu lock
func (c *GPUPowerCollector) cacheMIGHierarchy() error {
	c.migInstancesByDevice = make(map[int][]MIGGPUInstance)
	var totalInstances int

	for deviceIndex, mode := range c.sharingModes {
		if mode != gpu.SharingModePartitioned {
			continue
		}

		dev, err := c.nvml.GetDevice(deviceIndex)
		if err != nil {
			c.logger.Warn("failed to get device for MIG enumeration",
				"device", deviceIndex, "error", err)
			continue
		}

		// Enumerate MIG instances from NVML (finds ALL instances, not just active ones)
		nvmlInstances, err := dev.GetMIGInstances()
		if err != nil {
			c.logger.Warn("failed to enumerate MIG instances",
				"device", deviceIndex, "error", err)
			continue
		}

		for _, inst := range nvmlInstances {
			c.migInstancesByDevice[deviceIndex] = append(c.migInstancesByDevice[deviceIndex],
				MIGGPUInstance{
					ParentGPUIndex: deviceIndex,
					GPUInstanceID:  inst.GPUInstanceID,
				})
			totalInstances++
		}

		c.logger.Debug("enumerated MIG instances for device",
			"device", deviceIndex, "instances", len(nvmlInstances))
	}

	c.logger.Info("cached MIG hierarchy from NVML",
		"totalInstances", totalInstances)
	return nil
}

// attributePartitioned handles power attribution for MIG (partitioned) mode.
//
// Uses DCGM GR_ENGINE_ACTIVE metric per MIG instance to distribute the parent
// GPU's active power, then within each MIG instance distributes to processes
// using NVML's per-process utilization.
//
// Falls back to equal distribution among all running processes if DCGM is unavailable.
// NOTE: caller must hold c.mu lock
func (c *GPUPowerCollector) attributePartitioned(deviceIndex int, result map[uint32]float64) error {
	nvmlDev, err := c.nvml.GetDevice(deviceIndex)
	if err != nil {
		return err
	}

	// Get active power (reuses idle detection logic)
	stats, err := c.getDevicePowerStatsLocked(deviceIndex)
	if err != nil {
		return err
	}

	// Use cached MIG hierarchy from NVML
	instances := c.migInstancesByDevice[deviceIndex]

	// Fallback to equal distribution among running processes when DCGM is
	// unavailable (not deployed, unreachable, or initialization failed).
	if c.dcgm == nil || !c.dcgm.IsInitialized() || len(instances) == 0 {
		c.logger.Debug("DCGM not available or no MIG instances, using fallback",
			"dcgm_available", c.dcgm != nil && c.dcgm.IsInitialized(),
			"instances", len(instances))
		return c.attributePartitionedFallback(nvmlDev, stats.ActivePower, result)
	}

	// Collect activity and process data for each MIG instance
	type instanceData struct {
		activity    float64
		processUtil map[uint32]uint32 // PID -> SmUtil (0 if unavailable)
	}
	var data []instanceData
	var totalActivity float64

	for _, gi := range instances {
		activity, err := c.dcgm.GetMIGInstanceActivity(context.Background(), deviceIndex, gi.GPUInstanceID)
		if err != nil {
			c.logger.Debug("GetMIGInstanceActivity failed",
				"device", deviceIndex,
				"gpuInstanceID", gi.GPUInstanceID,
				"error", err)
			continue
		}

		// Skip NVML calls for idle MIG instances (major optimization).
		// Activity from dcgm-exporter is cached and cheap to query.
		// NVML calls (GetMIGDeviceByInstanceID, GetProcessUtilization) are slow.
		if activity == 0 {
			continue
		}

		migDevice, err := nvmlDev.GetMIGDeviceByInstanceID(gi.GPUInstanceID)
		if err != nil {
			c.logger.Debug("GetMIGDeviceByInstanceID failed",
				"device", deviceIndex,
				"gpuInstanceID", gi.GPUInstanceID,
				"error", err)
			continue
		}

		// Try GetProcessUtilization first (includes PIDs + utilization)
		processUtil := make(map[uint32]uint32)
		if utils, err := migDevice.GetProcessUtilization(0); err == nil && len(utils) > 0 {
			for _, u := range utils {
				processUtil[u.PID] = u.ComputeUtil
			}
		} else {
			// Fallback: get PIDs from GetComputeRunningProcesses (equal distribution)
			procs, err := migDevice.GetComputeRunningProcesses()
			if err != nil || len(procs) == 0 {
				continue
			}
			for _, p := range procs {
				processUtil[p.PID] = 0
			}
		}

		if len(processUtil) == 0 {
			continue
		}

		data = append(data, instanceData{activity: activity, processUtil: processUtil})
		totalActivity += activity
	}

	if len(data) == 0 || totalActivity == 0 {
		return nil
	}

	// Distribute active power based on activity ratio per MIG instance,
	// then within each instance by process utilization
	for _, d := range data {
		migPower := stats.ActivePower * (d.activity / totalActivity)
		distributeByUtilization(result, d.processUtil, migPower)
	}

	return nil
}

// attributePartitionedFallback distributes power equally among all running processes
// across all MIG instances when DCGM is not available.
// NOTE: caller must hold c.mu lock
func (c *GPUPowerCollector) attributePartitionedFallback(nvmlDev NVMLDevice, activePower float64, result map[uint32]float64) error {
	// Get all running processes from the parent device
	procs, err := nvmlDev.GetComputeRunningProcesses()
	if err != nil {
		return err
	}

	if len(procs) == 0 {
		return nil
	}

	powerPerProc := activePower / float64(len(procs))
	for _, p := range procs {
		result[p.PID] += powerPerProc
	}

	return nil
}

// distributeByUtilization distributes power among processes based on SM utilization.
// Falls back to equal distribution if all utilization values are zero.
func distributeByUtilization(result map[uint32]float64, processUtil map[uint32]uint32, power float64) {
	var totalUtil uint32
	for _, util := range processUtil {
		totalUtil += util
	}

	if totalUtil > 0 {
		for pid, util := range processUtil {
			result[pid] += power * float64(util) / float64(totalUtil)
		}
	} else {
		powerPerProcess := power / float64(len(processUtil))
		for pid := range processUtil {
			result[pid] += powerPerProcess
		}
	}
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
