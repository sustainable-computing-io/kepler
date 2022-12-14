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

	"github.com/sustainable-computing-io/kepler/pkg/bpfassets/attacher"
	"github.com/sustainable-computing-io/kepler/pkg/cgroup"
	collector_metric "github.com/sustainable-computing-io/kepler/pkg/collector/metric"
	"github.com/sustainable-computing-io/kepler/pkg/config"

	"k8s.io/klog/v2"
)

// #define CPU_VECTOR_SIZE 128
import "C"

// TODO in sync with bpf program
type ProcessBPFMetrics struct {
	CGroupID       uint64
	PID            uint64
	ProcessRunTime uint64
	CPUCycles      uint64
	CPUInstr       uint64
	CacheMisses    uint64
	Command        [16]byte
}

// resetBPFTables reset BPF module's tables
func (c *Collector) resetBPFTables() {
	c.bpfHCMeter.Table.DeleteAll()
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

		containerID, _ := cgroup.GetContainerID(ct.CGroupID, ct.PID, config.EnabledEBPFCgroupID)
		if err != nil {
			klog.V(5).Infof("failed to resolve container for cGroup ID %v: %v, set containerID=%s", ct.CGroupID, err, c.systemProcessName)
		}
		// TODO: improve the removal of deleted containers from ContainersMetrics. Currently we verify the maxInactiveContainers using the foundContainer map
		foundContainer[containerID] = true

		c.createContainersMetricsIfNotExist(containerID, ct.CGroupID, ct.PID, config.EnabledEBPFCgroupID)

		// System process is the aggregation of all background process running outside kubernetes
		// this means that the list of process might be very large, so we will not add this information to the cache
		if containerID != c.systemProcessName {
			c.ContainersMetrics[containerID].SetLatestProcess(ct.CGroupID, ct.PID, C.GoString(comm))
		}

		if err = c.ContainersMetrics[containerID].CPUTime.AddNewCurr(ct.ProcessRunTime); err != nil {
			klog.V(5).Infoln(err)
		}

		for _, counterKey := range collector_metric.AvailableCounters {
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
			if err = c.ContainersMetrics[containerID].CounterStats[counterKey].AddNewCurr(val); err != nil {
				klog.V(5).Infoln(err)
			}
		}

		c.ContainersMetrics[containerID].CurrProcesses++
		// system process should not include container event
		if containerID != c.systemProcessName {
			// TODO: move to container-level section
			rBytes, wBytes, disks, err := cgroup.ReadCgroupIOStat(ct.CGroupID, ct.PID)
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
