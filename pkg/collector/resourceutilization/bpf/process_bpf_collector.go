/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package bpf

import "C"
import (
	"unsafe"

	"github.com/sustainable-computing-io/kepler/pkg/bpfassets/attacher"
	"github.com/sustainable-computing-io/kepler/pkg/cgroup"
	"github.com/sustainable-computing-io/kepler/pkg/collector/stats"
	"github.com/sustainable-computing-io/kepler/pkg/config"
	"github.com/sustainable-computing-io/kepler/pkg/libvirt"
	"github.com/sustainable-computing-io/kepler/pkg/utils"

	"k8s.io/klog/v2"
)

type ProcessBPFMetrics = attacher.ProcessBPFMetrics

// update software counter metrics
func updateSWCounters(key uint64, ct *ProcessBPFMetrics, processStats map[uint64]*stats.ProcessStats) {
	// update ebpf metrics
	// first update CPU time and Page Cache Hit
	processStats[key].ResourceUsage[config.CPUTime].AddDeltaStat(utils.GenericSocketID, ct.ProcessRunTime)
	processStats[key].ResourceUsage[config.TaskClock].AddDeltaStat(utils.GenericSocketID, ct.TaskClockTime)
	processStats[key].ResourceUsage[config.PageCacheHit].AddDeltaStat(utils.GenericSocketID, ct.PageCacheHit/(1000*1000))
	// update IRQ vector. Soft IRQ events has the events ordered
	for i, event := range attacher.SoftIRQEvents {
		processStats[key].ResourceUsage[event].AddDeltaStat(utils.GenericSocketID, uint64(ct.VecNR[i]))
	}
}

// update hardware counter metrics
func updateHWCounters(key uint64, ct *ProcessBPFMetrics, processStats map[uint64]*stats.ProcessStats) {
	// update HW counters
	for _, counterKey := range stats.AvailableBPFHWCounters {
		var val uint64
		var event string
		switch counterKey {
		case config.CPUCycle:
			val = ct.CPUCycles
			event = config.CPUCycle
		case config.CPURefCycle:
			val = ct.CPUCycles
			event = config.CPURefCycle
		case config.CPUInstruction:
			val = ct.CPUInstr
			event = config.CPUInstruction
		case config.CacheMiss:
			val = ct.CacheMisses
			event = config.CacheMiss
		case config.TaskClock:
			val = ct.TaskClockTime
			event = config.TaskClock
		default:
			klog.Errorf("counter %s is not supported\n", counterKey)
		}
		processStats[key].ResourceUsage[event].AddDeltaStat(utils.GenericSocketID, val)
	}
}

// UpdateProcessBPFMetrics reads the BPF tables with process/pid/cgroupid metrics (CPU time, available HW counters)
func UpdateProcessBPFMetrics(processStats map[uint64]*stats.ProcessStats) {
	processesData, err := attacher.CollectProcesses()
	if err != nil {
		klog.Errorln("could not collect ebpf metrics")
		return
	}
	for _, ct := range processesData {
		comm := C.GoString((*C.char)(unsafe.Pointer(&ct.Command)))

		if ct.PID != 0 {
			klog.V(6).Infof("process %s (pid=%d, cgroup=%d) has %d task clock time %d CPU cycles, %d instructions, %d cache misses, %d page cache hits",
				comm, ct.PID, ct.CGroupID, ct.TaskClockTime, ct.CPUCycles, ct.CPUInstr, ct.CacheMisses, ct.PageCacheHit)
		}
		// skip process without resource utilization
		if ct.TaskClockTime == 0 && ct.CacheMisses == 0 && ct.PageCacheHit == 0 {
			continue
		}

		// if the pid is within a container, it will have a container ID
		containerID, err := cgroup.GetContainerID(ct.CGroupID, ct.PID, config.EnabledEBPFCgroupID)
		if err != nil {
			klog.V(6).Infof("failed to resolve container for PID %v (command=%s): %v, set containerID=%s", ct.PID, comm, err, utils.SystemProcessName)
		}

		// if the pid is within a VM, it will have an VM ID
		vmID := utils.EmptyString
		if config.IsExposeVMStatsEnabled() {
			vmID, err = libvirt.GetVMID(ct.PID)
			if err != nil {
				klog.V(6).Infof("failed to resolve VM ID for PID %v (command=%s): %v", ct.PID, comm, err)
			}
		}

		mapKey := ct.PID
		if ct.CGroupID == 1 && config.EnabledEBPFCgroupID {
			// we aggregate all kernel process to minimize overhead
			// all kernel process has cgroup id as 1 and pid 1 is also a kernel process
			mapKey = 1
		}

		var ok bool
		var pStat *stats.ProcessStats
		if pStat, ok = processStats[mapKey]; !ok {
			pStat = stats.NewProcessStats(ct.PID, ct.CGroupID, containerID, vmID, comm)
			processStats[mapKey] = pStat
		} else if pStat.Command == "" {
			pStat.Command = comm
		}
		// when the process metrics are updated, reset the idle counter
		pStat.IdleCounter = 0

		updateSWCounters(mapKey, &ct, processStats)
		updateHWCounters(mapKey, &ct, processStats)
	}
}
