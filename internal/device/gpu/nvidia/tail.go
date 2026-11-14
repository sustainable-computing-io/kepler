// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package nvidia

import "github.com/sustainable-computing-io/kepler/internal/device/gpu"

// After a GPU process exits, the GPU can stay clocked up for a few seconds. NVML no longer lists
// the process, so the active power read on the first collection after the exit would go to no
// process at all. To keep it with the workload that caused it, the collector remembers each
// process's share of active power from the last attribution that found processes, and applies
// those shares once, on the first collection that finds the process list empty. Power read on
// later collections stays unattributed.

// tailKey identifies a set of cached shares: a whole GPU, or one MIG instance of a GPU in
// partitioned mode.
type tailKey struct {
	device   int
	instance uint
	mig      bool
}

func deviceTailKey(deviceIndex int) tailKey {
	return tailKey{device: deviceIndex}
}

func migTailKey(deviceIndex int, gpuInstanceID uint) tailKey {
	return tailKey{device: deviceIndex, instance: gpuInstanceID, mig: true}
}

// tailShares holds each PID's share of the device's active power from the last attribution
// that found processes.
type tailShares struct {
	shares map[uint32]float64
	// used is set once the shares have been applied to an empty collection. Used shares are
	// kept until the next collection, so that idle detection skips the whole tail collection.
	used bool
}

// recordTailShares replaces the cached shares for key with the shares in attributed, the power
// given to each PID out of activePower. Any non-empty process list replaces the previous shares,
// so a PID that is reused later does not receive power from an earlier process.
// NOTE: caller must hold c.mu lock
func (c *GPUPowerCollector) recordTailShares(key tailKey, attributed map[uint32]float64, activePower float64) {
	if activePower <= 0 || len(attributed) == 0 {
		delete(c.tailShares, key)
		return
	}

	shares := make(map[uint32]float64, len(attributed))
	for pid, watts := range attributed {
		shares[pid] = watts / activePower
	}

	if c.tailShares == nil {
		c.tailShares = make(map[tailKey]*tailShares)
	}
	c.tailShares[key] = &tailShares{shares: shares}
}

// applyTailShares attributes activePower with the cached shares for key, if they have not been
// used yet. It is called when the process list for key is empty.
// NOTE: caller must hold c.mu lock
func (c *GPUPowerCollector) applyTailShares(key tailKey, activePower float64, result map[uint32]float64) {
	entry, ok := c.tailShares[key]
	if !ok || entry.used {
		return
	}
	entry.used = true

	if activePower <= 0 {
		return
	}
	if c.tailPower == nil {
		c.tailPower = make(map[uint32]float64)
	}
	for pid, share := range entry.shares {
		watts := activePower * share
		result[pid] += watts
		c.tailPower[pid] += watts
	}
	c.logger.Debug("attributed GPU power after process exit to the exited processes",
		"device", key.device, "mig", key.mig, "gpuInstanceID", key.instance,
		"activePower", activePower, "processes", len(entry.shares))
}

// GetProcessTailPower returns the power in the last GetProcessPower result that was read after
// the processes exited, by PID. It implements gpu.TailPowerReporter.
func (c *GPUPowerCollector) GetProcessTailPower() map[uint32]float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	tail := make(map[uint32]float64, len(c.tailPower))
	for pid, watts := range c.tailPower {
		tail[pid] = watts
	}
	return tail
}

// dropUsedTailShares removes the shares that were applied on the previous collection.
// NOTE: caller must hold c.mu lock
func (c *GPUPowerCollector) dropUsedTailShares() {
	for key, entry := range c.tailShares {
		if entry.used {
			delete(c.tailShares, key)
		}
	}
}

// hasTailShares reports whether the device, or one of its MIG instances, has cached shares.
// While it does, an empty process list may be the tail of an exited process rather than idle.
// NOTE: caller must hold c.mu lock
func (c *GPUPowerCollector) hasTailShares(deviceIndex int) bool {
	for key := range c.tailShares {
		if key.device == deviceIndex {
			return true
		}
	}
	return false
}

// dropTailSharesOfOtherPath removes the device's shares cached by the attribution path that
// is not used on this collection: per MIG instance when DCGM is used, per device otherwise.
// NOTE: caller must hold c.mu lock
func (c *GPUPowerCollector) dropTailSharesOfOtherPath(deviceIndex int, perInstance bool) {
	for key := range c.tailShares {
		if key.device == deviceIndex && key.mig != perInstance {
			delete(c.tailShares, key)
		}
	}
}

// keepTailSharesOf trims the cached shares for key to the running PIDs and reports whether
// they can be kept. They can when every running PID has a share, so the shares still describe
// the processes that are running. The shares of PIDs that left go to the ones still running,
// so the instance keeps the same total share.
// NOTE: caller must hold c.mu lock
func (c *GPUPowerCollector) keepTailSharesOf(key tailKey, running []gpu.ProcessGPUInfo) bool {
	entry, ok := c.tailShares[key]
	if !ok {
		return false
	}
	var total, kept float64
	for _, share := range entry.shares {
		total += share
	}
	trimmed := make(map[uint32]float64, len(running))
	for _, p := range running {
		share, ok := entry.shares[p.PID]
		if !ok {
			return false
		}
		trimmed[p.PID] = share
		kept += share
	}
	if kept <= 0 {
		return false
	}
	for pid, share := range trimmed {
		trimmed[pid] = share * total / kept
	}
	entry.shares = trimmed
	return true
}

// unusedTailShareTotal returns the sum of the cached shares for key, or 0 if there are none or
// they were used already.
// NOTE: caller must hold c.mu lock
func (c *GPUPowerCollector) unusedTailShareTotal(key tailKey) float64 {
	entry, ok := c.tailShares[key]
	if !ok || entry.used {
		return 0
	}
	var total float64
	for _, share := range entry.shares {
		total += share
	}
	return total
}

// migInstanceProcesses lists the processes running on a MIG instance. ok is false when the
// instance could not be read.
func (c *GPUPowerCollector) migInstanceProcesses(nvmlDev NVMLDevice, gpuInstanceID uint) ([]gpu.ProcessGPUInfo, bool) {
	migDevice, err := nvmlDev.GetMIGDeviceByInstanceID(gpuInstanceID)
	if err != nil {
		c.logger.Warn("failed to get MIG device handle", "gpuInstanceID", gpuInstanceID, "error", err)
		return nil, false
	}
	procs, err := migDevice.GetComputeRunningProcesses()
	if err != nil {
		return nil, false
	}
	return procs, true
}

func mergePower(dst, src map[uint32]float64) {
	for pid, watts := range src {
		dst[pid] += watts
	}
}
