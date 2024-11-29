//go:build !darwin
// +build !darwin

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

	"github.com/sustainable-computing-io/kepler/pkg/bpf"
	"github.com/sustainable-computing-io/kepler/pkg/cgroup"
	"github.com/sustainable-computing-io/kepler/pkg/collector/stats"
	"github.com/sustainable-computing-io/kepler/pkg/config"
	"github.com/sustainable-computing-io/kepler/pkg/libvirt"
	"github.com/sustainable-computing-io/kepler/pkg/utils"

	"k8s.io/klog/v2"
)

type ProcessBPFMetrics = bpf.ProcessMetrics

// update software counter metrics
func updateSWCounters(key uint64, ct *ProcessBPFMetrics, processStats map[uint64]*stats.ProcessStats, bpfSupportedMetrics bpf.SupportedMetrics) {
	// update ebpf metrics
	// first update CPU time and Page Cache Hit
	for counterKey := range bpfSupportedMetrics.SoftwareCounters {
		switch counterKey {
		case config.CPUTime:
			processStats[key].ResourceUsage[config.CPUTime].AddDeltaStat(utils.GenericSocketID, ct.ProcessRunTime/1000 /* convert microseconds to milliseconds */)
		case config.PageCacheHit:
			processStats[key].ResourceUsage[config.PageCacheHit].AddDeltaStat(utils.GenericSocketID, ct.PageCacheHit/(1000*1000))
		case config.IRQNetTXLabel:
			processStats[key].ResourceUsage[config.IRQNetTXLabel].AddDeltaStat(utils.GenericSocketID, uint64(ct.VecNr[bpf.IRQNetTX]))
		case config.IRQNetRXLabel:
			processStats[key].ResourceUsage[config.IRQNetRXLabel].AddDeltaStat(utils.GenericSocketID, uint64(ct.VecNr[bpf.IRQNetRX]))
		case config.IRQBlockLabel:
			processStats[key].ResourceUsage[config.IRQBlockLabel].AddDeltaStat(utils.GenericSocketID, uint64(ct.VecNr[bpf.IRQBlock]))
		default:
			klog.Errorf("counter %s is not supported\n", counterKey)
		}
	}
}

// update hardware counter metrics
func updateHWCounters(key uint64, ct *ProcessBPFMetrics, processStats map[uint64]*stats.ProcessStats, bpfSupportedMetrics bpf.SupportedMetrics) {
	for counterKey := range bpfSupportedMetrics.HardwareCounters {
		var val uint64
		var event string
		switch counterKey {
		case config.CPUCycle:
			val = ct.CpuCycles
			event = config.CPUCycle
		case config.CPURefCycle:
			val = ct.CpuCycles
			event = config.CPURefCycle
		case config.CPUInstruction:
			val = ct.CpuInstr
			event = config.CPUInstruction
		case config.CacheMiss:
			val = ct.CacheMiss
			event = config.CacheMiss
		default:
			klog.Errorf("counter %s is not supported\n", counterKey)
		}
		processStats[key].ResourceUsage[event].AddDeltaStat(utils.GenericSocketID, val)
	}
}

// UpdateProcessBPFMetrics reads the BPF tables with process/pid/cgroupid metrics (CPU time, available HW counters)
func UpdateProcessBPFMetrics(bpfExporter bpf.Exporter, processStats map[uint64]*stats.ProcessStats) {
	processesData, err := bpfExporter.CollectProcesses()
	if err != nil {
		klog.Errorln("could not collect ebpf metrics")
		return
	}
	for _, ct := range processesData {
		comm := C.GoString((*C.char)(unsafe.Pointer(&ct.Comm)))

		if ct.Pid == 0 && config.ExcludeSwapperProcess() {
			// exclude swapper process
			continue
		}

		if ct.Pid != 0 {
			klog.V(6).Infof("process %s (pid=%d, cgroup=%d) has %d process run time, %d CPU cycles, %d instructions, %d cache misses, %d page cache hits",
				comm, ct.Pid, ct.CgroupId, ct.ProcessRunTime, ct.CpuCycles, ct.CpuInstr, ct.CacheMiss, ct.PageCacheHit)
		}

		// if the pid is within a container, it will have a container ID
		containerID, err := cgroup.GetContainerID(ct.CgroupId, ct.Pid, config.EnabledEBPFCgroupID())
		if err != nil {
			klog.V(6).Infof("failed to resolve container for PID %v (command=%s): %v, set containerID=%s", ct.Pid, comm, err, utils.SystemProcessName)
		}

		// if the pid is within a VM, it will have an VM ID
		vmID := utils.EmptyString
		if config.IsExposeVMStatsEnabled() {
			vmID, err = libvirt.GetVMID(ct.Pid)
			if err != nil {
				klog.V(6).Infof("failed to resolve VM ID for PID %v (command=%s): %v", ct.Pid, comm, err)
			}
		}

		mapKey := ct.Pid
		process := comm
		if ct.CgroupId == 1 && config.EnabledEBPFCgroupID() {
			// we aggregate all kernel process to minimize overhead
			// all kernel process has cgroup id as 1 and pid 1 is also a kernel process
			mapKey = 1
			process = "kernel_processes"
		}

		bpfSupportedMetrics := bpfExporter.SupportedMetrics()
		var ok bool
		var pStat *stats.ProcessStats
		if pStat, ok = processStats[mapKey]; !ok {
			pStat = stats.NewProcessStats(mapKey, ct.CgroupId, containerID, vmID, process)
			processStats[mapKey] = pStat
		} else if pStat.Command == "" {
			pStat.Command = comm
		}
		// when the process metrics are updated, reset the idle counter
		pStat.IdleCounter = 0

		updateSWCounters(mapKey, &ct, processStats, bpfSupportedMetrics)
		updateHWCounters(mapKey, &ct, processStats, bpfSupportedMetrics)
	}
}
