// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package nvidia

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
	for pid, share := range entry.shares {
		result[pid] += activePower * share
	}
	c.logger.Debug("attributed GPU power after process exit to the exited processes",
		"device", key.device, "mig", key.mig, "gpuInstanceID", key.instance,
		"activePower", activePower, "processes", len(entry.shares))
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

func mergePower(dst, src map[uint32]float64) {
	for pid, watts := range src {
		dst[pid] += watts
	}
}
