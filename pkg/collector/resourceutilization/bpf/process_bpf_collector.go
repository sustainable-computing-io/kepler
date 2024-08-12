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
	"github.com/sustainable-computing-io/kepler/pkg/bpf"
	"github.com/sustainable-computing-io/kepler/pkg/cgroup"
	"github.com/sustainable-computing-io/kepler/pkg/collector/stats"
	"github.com/sustainable-computing-io/kepler/pkg/comm"
	"github.com/sustainable-computing-io/kepler/pkg/config"
	"github.com/sustainable-computing-io/kepler/pkg/libvirt"
	"github.com/sustainable-computing-io/kepler/pkg/utils"

	"k8s.io/klog/v2"
)

type ProcessBPFMetrics = bpf.ProcessMetrics

var commResolver = comm.NewCommResolver()

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
			processStats[key].ResourceUsage[config.IRQNetTXLabel].AddDeltaStat(utils.GenericSocketID, ct.NetTxIRQ)
		case config.IRQNetRXLabel:
			processStats[key].ResourceUsage[config.IRQNetRXLabel].AddDeltaStat(utils.GenericSocketID, ct.NetRxIRQ)
		case config.IRQBlockLabel:
			processStats[key].ResourceUsage[config.IRQBlockLabel].AddDeltaStat(utils.GenericSocketID, ct.NetBlockIRQ)
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
			val = ct.CPUCyles
			event = config.CPUCycle
		case config.CPURefCycle:
			val = ct.CPUCyles
			event = config.CPURefCycle
		case config.CPUInstruction:
			val = ct.CPUInstructions
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

	// Clear the cache of any PIDs freed this sample period.
	// This is safe given that the *stats.ProcessStats.Command is only updated if it is not already known.
	// If it is a long-running process, the comm will be preserved from the previous sample period.
	commResolver.Clear(processesData.FreedPIDs)

	for _, ct := range processesData.Metrics {
		processComm, err := commResolver.ResolveComm(int(ct.Pid))
		if err != nil {
			// skip process that is not running
			klog.V(6).Infof("failed to resolve comm for PID %v: %v, set comm=%s", ct.Pid, err, utils.SystemProcessName)
			continue
		}

		if ct.Pid != 0 {
			klog.V(6).Infof("process %s (pid=%d, cgroup=%d) has %d CPU cycles, %d instructions, %d cache misses, %d page cache hits",
				processComm, ct.Pid, ct.CGroupID, ct.CPUCyles, ct.CPUInstructions, ct.CacheMiss, ct.PageCacheHit)
		}
		// skip process without resource utilization
		if ct.CacheMiss == 0 && ct.PageCacheHit == 0 {
			continue
		}

		// if the pid is within a container, it will have a container ID
		containerID, err := cgroup.GetContainerID(ct.CGroupID, ct.Pid, config.EnabledEBPFCgroupID)
		if err != nil {
			klog.V(6).Infof("failed to resolve container for PID %v (command=%s): %v, set containerID=%s", ct.Pid, processComm, err, utils.SystemProcessName)
		}

		// if the pid is within a VM, it will have an VM ID
		vmID := utils.EmptyString
		if config.IsExposeVMStatsEnabled() {
			vmID, err = libvirt.GetVMID(ct.Pid)
			if err != nil {
				klog.V(6).Infof("failed to resolve VM ID for PID %v (command=%s): %v", ct.Pid, processComm, err)
			}
		}

		mapKey := ct.Pid
		if ct.CGroupID == 1 && config.EnabledEBPFCgroupID {
			// we aggregate all kernel process to minimize overhead
			// all kernel process has cgroup id as 1 and pid 1 is also a kernel process
			mapKey = 1
		}

		bpfSupportedMetrics := bpfExporter.SupportedMetrics()
		var ok bool
		var pStat *stats.ProcessStats
		if pStat, ok = processStats[mapKey]; !ok {
			pStat = stats.NewProcessStats(ct.Pid, ct.CGroupID, containerID, vmID, processComm, bpfSupportedMetrics)
			processStats[mapKey] = pStat
		} else if pStat.Command == "" {
			pStat.Command = processComm
		}

		// when the process metrics are updated, reset the idle counter
		pStat.IdleCounter = 0

		updateSWCounters(mapKey, &ct, processStats, bpfSupportedMetrics)
		updateHWCounters(mapKey, &ct, processStats, bpfSupportedMetrics)
	}
}
