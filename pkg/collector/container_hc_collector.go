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

package collector

import (
	"strconv"
	"unsafe"

	"github.com/sustainable-computing-io/kepler/pkg/bpfassets/attacher"
	"github.com/sustainable-computing-io/kepler/pkg/cgroup"
	collector_metric "github.com/sustainable-computing-io/kepler/pkg/collector/metric"
	"github.com/sustainable-computing-io/kepler/pkg/config"
	"github.com/sustainable-computing-io/kepler/pkg/libvirt"

	"k8s.io/klog/v2"
)

// #define CPU_VECTOR_SIZE 128
import "C"

type ProcessBPFMetrics = attacher.ProcessBPFMetrics

// updateSWCounters
func (c *Collector) updateSWCounters(containerID string, ct *ProcessBPFMetrics, isSystemProcess, isSystemVM bool) {
	// update ebpf metrics
	// first update CPU time and Page Cache Hit
	err := c.ContainersMetrics[containerID].BPFStats[config.CPUTime].AddNewDelta(ct.ProcessRunTime)
	if err != nil {
		klog.V(5).Infoln(err)
	}
	err = c.ContainersMetrics[containerID].BPFStats[config.PageCacheHit].AddNewDelta(ct.PageCacheHit / 1000)
	if err != nil {
		klog.V(5).Infoln(err)
	}

	// update IRQ vector. Soft IRQ events has the events ordered
	for i, event := range attacher.SoftIRQEvents {
		err := c.ContainersMetrics[containerID].BPFStats[event].AddNewDelta(uint64(ct.VecNR[i]))
		if err != nil {
			klog.V(5).Infoln(err)
		}
	}
	// track system process metrics
	if isSystemProcess && config.EnableProcessMetrics {
		for i, event := range attacher.SoftIRQEvents {
			err := c.ProcessMetrics[ct.PID].BPFStats[event].AddNewDelta(uint64(ct.VecNR[i]))
			if err != nil {
				klog.V(5).Infoln(err)
			}
		}
	}
	// track virtual machine metrics
	// TODO: remove duplicated code, we don;t need to have a map with duplicated data for VMs
	if isSystemVM && config.EnableProcessMetrics {
		for _, event := range attacher.SoftIRQEvents {
			c.VMMetrics[ct.PID].BPFStats[event] = c.ProcessMetrics[ct.PID].BPFStats[event]
		}
	}
}

// updateHWCounters
func (c *Collector) updateHWCounters(containerID string, ct *ProcessBPFMetrics, isSystemProcess, isSystemVM bool) {
	// update HW counters
	for _, counterKey := range collector_metric.AvailableBPFHWCounters {
		var val uint64
		switch counterKey {
		case attacher.CPUCycleLabel:
			val = ct.CPUCycles
		case attacher.CPUInstructionLabel:
			val = ct.CPUInstr
		case attacher.CacheMissLabel:
			val = ct.CacheMisses
		default:
			val = 0
		}
		err := c.ContainersMetrics[containerID].BPFStats[counterKey].AddNewDelta(val)
		if err != nil {
			klog.V(5).Infoln(err)
		}
		// track system process metrics
		if isSystemProcess && config.EnableProcessMetrics {
			err := c.ProcessMetrics[ct.PID].BPFStats[counterKey].AddNewDelta(val)
			if err != nil {
				klog.V(5).Infoln(err)
			}
		}
		// track virtual machine metrics
		// TODO: remove VMMetrics and ContainersMetrics, we should keep only ProcessMetrics to avoid data and code duplication for efficiency and consistency
		if isSystemVM && config.EnableProcessMetrics {
			c.VMMetrics[ct.PID].BPFStats = c.ProcessMetrics[ct.PID].BPFStats
		}
	}
}

// updateBPFMetrics reads the BPF tables with process/pid/cgroupid metrics (CPU time, available HW counters)
func (c *Collector) updateBPFMetrics() {
	foundContainer := make(map[string]bool)
	foundProcess := make(map[uint64]bool)
	foundVM := make(map[uint64]bool)
	processesData, err := attacher.CollectProcesses()
	vmPIDList, _ := libvirt.GetCurrentVMPID()
	if err != nil {
		return
	}
	// TODO(mamaral): instead of aggregate the process data per container ID, keep the data per cgroup, so that we can show cgroup metrics of system processes
	// this will also help to expose metrics per processes without code duplication
	for _, ct := range processesData {
		comm := C.GoString((*C.char)(unsafe.Pointer(&ct.Command)))
		containerID, err := cgroup.GetContainerID(ct.CGroupID, ct.PID, config.EnabledEBPFCgroupID)
		if err != nil {
			klog.V(5).Infof("failed to resolve container for cGroup ID %v (command=%s): %v, set containerID=%s", ct.CGroupID, comm, err, c.systemProcessName)
		}
		c.createContainersMetricsIfNotExist(containerID, ct.CGroupID, ct.PID, config.EnabledEBPFCgroupID)

		// System process is the aggregation of all background process running outside kubernetes
		// this means that the list of process might be very large, so we will not add this information to the cache
		isSystemProcess := containerID == c.systemProcessName
		if !isSystemProcess {
			c.ContainersMetrics[containerID].SetLatestProcess(ct.CGroupID, ct.PID, comm)
		} else if config.EnableProcessMetrics {
			c.createProcessMetricsIfNotExist(ct.PID, comm)
			err := c.ProcessMetrics[ct.PID].BPFStats[config.CPUTime].AddNewDelta(ct.ProcessRunTime)
			if err != nil {
				klog.V(5).Infoln(err)
			}
			for vmpid, name := range vmPIDList {
				pid, _ := strconv.ParseUint(vmpid, 10, 64)

				if pid == ct.PID {
					c.createVMMetricsIfNotExist(ct.PID, name)
					foundVM[pid] = true
					c.VMMetrics[ct.PID].BPFStats = c.ProcessMetrics[ct.PID].BPFStats
				}
			}
		}

		c.ContainersMetrics[containerID].CurrProcesses++

		c.updateSWCounters(containerID, &ct, isSystemProcess, foundVM[ct.PID])
		c.updateHWCounters(containerID, &ct, isSystemProcess, foundVM[ct.PID])

		// TODO: improve the removal of deleted containers from ContainersMetrics. Currently we verify the maxInactiveContainers using the foundContainer map
		foundContainer[containerID] = true
		if isSystemProcess {
			foundProcess[ct.PID] = true
		}
	}
	c.handleInactiveContainers(foundContainer)
	if config.EnableProcessMetrics {
		c.handleInactiveProcesses(foundProcess)
		c.handleInactiveVM(foundVM)
	}
}

// handleInactiveContainers
func (c *Collector) handleInactiveContainers(foundContainer map[string]bool) {
	numOfInactive := len(c.ContainersMetrics) - len(foundContainer)
	if numOfInactive > maxInactiveContainers {
		aliveContainers, err := cgroup.GetAliveContainers()
		if err != nil {
			klog.V(5).Infoln(err)
			return
		}
		for containerID := range c.ContainersMetrics {
			if containerID == c.systemProcessName {
				continue
			}
			if _, found := aliveContainers[containerID]; !found {
				delete(c.ContainersMetrics, containerID)
			}
		}
	}
}

// handleInactiveProcesses
func (c *Collector) handleInactiveProcesses(foundProcess map[uint64]bool) {
	numOfInactive := len(c.ProcessMetrics) - len(foundProcess)
	if numOfInactive > maxInactiveProcesses {
		for pid := range c.ProcessMetrics {
			if _, found := foundProcess[pid]; !found {
				delete(c.ProcessMetrics, pid)
			}
		}
	}
}

// handleInactiveVirtualMachine
func (c *Collector) handleInactiveVM(foundVM map[uint64]bool) {
	numOfInactive := len(c.VMMetrics) - len(foundVM)
	if numOfInactive > maxInactiveVM {
		for pid := range c.VMMetrics {
			if _, found := foundVM[pid]; !found {
				delete(c.VMMetrics, pid)
			}
		}
	}
}
