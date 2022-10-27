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
	"bytes"
	"encoding/binary"

	"unsafe"

	"github.com/sustainable-computing-io/kepler/pkg/attacher"
	"github.com/sustainable-computing-io/kepler/pkg/cgroup"
	collector_metric "github.com/sustainable-computing-io/kepler/pkg/collector/metric"

	"k8s.io/klog/v2"
)

// #define CPU_VECTOR_SIZE 128
import "C"

// TODO in sync with bpf program
type ProcessBPFMetrics struct {
	CGroupPID      uint64
	PID            uint64
	ProcessRunTime uint64
	CPUCycles      uint64
	CPUInstr       uint64
	CacheMisses    uint64
	Command        [16]byte
	CPUTime        [C.CPU_VECTOR_SIZE]uint16
}

// resetBPFTables reset BPF module's tables
func (c *Collector) resetBPFTables() {
	c.bpfHCMeter.Table.DeleteAll()
	c.bpfHCMeter.TimeTable.DeleteAll()
}

// updateBPFMetrics reads the BPF tables with process/pid/cgroupid metrics (CPU time, available HW counters)
func (c *Collector) updateBPFMetrics() {
	if c.bpfHCMeter == nil {
		return
	}
	foundContainer := make(map[string]bool)
	var ct ProcessBPFMetrics
	for it := c.bpfHCMeter.Table.Iter(); it.Next(); {
		data := it.Leaf()
		err := binary.Read(bytes.NewBuffer(data), binary.LittleEndian, &ct)
		if err != nil {
			klog.V(5).Infof("failed to decode received data: %v", err)
			continue
		}
		comm := (*C.char)(unsafe.Pointer(&ct.Command))

		containerID, _ := cgroup.GetContainerID(ct.CGroupPID, ct.PID)
		if err != nil {
			klog.V(5).Infof("failed to resolve container for cGroup ID %v: %v, set containerID=%s", ct.CGroupPID, err, c.systemProcessName)
		}
		// TODO: improve the removal of deleted containers from ContainersMetrics. Currently we verify the maxInactiveContainers using the foundContainer map
		foundContainer[containerID] = true

		if _, ok := c.ContainersMetrics[containerID]; !ok {
			podName, _ := cgroup.GetPodName(ct.CGroupPID, ct.PID)
			containerName, _ := cgroup.GetContainerName(ct.CGroupPID, ct.PID)
			namespace := c.systemProcessNamespace
			if containerName == c.systemProcessName {
				containerID = c.systemProcessName
			} else {
				namespace, err = cgroup.GetPodNameSpace(ct.CGroupPID, ct.PID)
				if err != nil {
					klog.V(5).Infof("failed to find namespace for cGroup ID %v: %v", ct.CGroupPID, err)
					namespace = "unknown"
				}
			}
			c.ContainersMetrics[containerID] = collector_metric.NewContainerMetrics(containerName, podName, namespace)
		}

		// System process is the aggregation of all background process running outside kubernetes
		// this means that the list of process might be very large, so we will not add this information to the cache
		if containerID != c.systemProcessName {
			c.ContainersMetrics[containerID].SetLatestProcess(ct.CGroupPID, ct.PID, C.GoString(comm))
		}

		var activeCPUs []int32
		var avgFreq float64
		var totalCPUTime uint64
		if attacher.EnableCPUFreq {
			avgFreq, totalCPUTime, activeCPUs = getAVGCPUFreqAndTotalCPUTime(c.NodeCPUFrequency, &ct.CPUTime)
			c.ContainersMetrics[containerID].AvgCPUFreq = avgFreq
		} else {
			totalCPUTime = ct.ProcessRunTime
			activeCPUs = getActiveCPUs(&ct.CPUTime)
		}

		for _, cpu := range activeCPUs {
			c.ContainersMetrics[containerID].CurrCPUTimePerCPU[uint32(cpu)] += uint64(ct.CPUTime[cpu])
		}

		if err = c.ContainersMetrics[containerID].CPUTime.AddNewCurr(totalCPUTime); err != nil {
			klog.V(5).Infoln(err)
		}

		for _, counterKey := range collector_metric.AvailableCounters {
			var val uint64
			switch counterKey {
			case attacher.CPUCycleLable:
				val = ct.CPUCycles
			case attacher.CPUInstructionLabel:
				val = ct.CPUInstr
			case attacher.CacheMissLabel:
				val = ct.CacheMisses
			default:
				val = 0
			}
			if err = c.ContainersMetrics[containerID].CounterStats[counterKey].AddNewCurr(val); err != nil {
				klog.V(5).Infoln(err)
			}
		}

		c.ContainersMetrics[containerID].CurrProcesses++
		// system process should not include container event
		if containerID != c.systemProcessName {
			// TODO: move to container-level section
			rBytes, wBytes, disks, err := cgroup.ReadCgroupIOStat(ct.CGroupPID, ct.PID)
			if err == nil {
				if disks > c.ContainersMetrics[containerID].Disks {
					c.ContainersMetrics[containerID].Disks = disks
				}
				c.ContainersMetrics[containerID].BytesRead.AddAggrStat(containerID, rBytes)
				c.ContainersMetrics[containerID].BytesWrite.AddAggrStat(containerID, wBytes)
			}
		}
	}
	c.resetBPFTables()
	c.handleInactiveContainers(foundContainer)
}

// getAVGCPUFreqAndTotalCPUTime calculates the weighted cpu frequency average
func getAVGCPUFreqAndTotalCPUTime(cpuFrequency map[int32]uint64, cpuTime *[C.CPU_VECTOR_SIZE]uint16) (avgFreq float64, totalCPUTime uint64, activeCPUs []int32) {
	totalFreq := float64(0)
	totalFreqWithoutWeight := float64(0)
	for cpu, freq := range cpuFrequency {
		if int(cpu) > len((*cpuTime))-1 {
			break
		}
		totalCPUTime += uint64(cpuTime[cpu])
		totalFreqWithoutWeight += float64(freq)
	}
	if totalCPUTime == 0 {
		if len(cpuFrequency) == 0 {
			return
		}
		avgFreq = totalFreqWithoutWeight / float64(len(cpuFrequency))
	} else {
		for cpu, freq := range cpuFrequency {
			if int(cpu) > len((*cpuTime))-1 {
				break
			}
			if cpuTime[cpu] != 0 {
				totalFreq += float64(freq) * (float64(cpuTime[cpu]) / float64(totalCPUTime))
				activeCPUs = append(activeCPUs, cpu)
			}
		}
		avgFreq = totalFreqWithoutWeight / float64(len(cpuFrequency))
	}
	return
}

// getActiveCPUs returns active cpu(vcpu) (in case that frequency is not active)
func getActiveCPUs(cpuTime *[C.CPU_VECTOR_SIZE]uint16) (activeCPUs []int32) {
	for cpu := range cpuTime {
		if cpuTime[cpu] != 0 {
			activeCPUs = append(activeCPUs, int32(cpu))
		}
	}
	return
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
