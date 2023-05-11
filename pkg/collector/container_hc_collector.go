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
	"github.com/sustainable-computing-io/kepler/pkg/utils"

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
	VecNR          [config.MaxIRQ]uint16 // irq counter, 10 is the max number of irq vectors
	Command        [16]byte
}

// updateBasicBPF
func (c *Collector) updateBasicBPF(containerID string, ct *ProcessBPFMetrics, isSystemProcess bool) {
	// update ebpf metrics
	// first update CPU time
	err := c.ContainersMetrics[containerID].CPUTime.AddNewDelta(ct.ProcessRunTime)
	if err != nil {
		klog.V(5).Infoln(err)
	}
	// update IRQ vector
	for i := 0; i < config.MaxIRQ; i++ {
		err := c.ContainersMetrics[containerID].SoftIRQCount[i].AddNewDelta(uint64(ct.VecNR[i]))
		if err != nil {
			klog.V(5).Infoln(err)
		}
	}
	// track system process metrics
	if isSystemProcess && config.EnableProcessMetrics {
		for i := 0; i < config.MaxIRQ; i++ {
			err := c.ProcessMetrics[ct.PID].SoftIRQCount[i].AddNewDelta(uint64(ct.VecNR[i]))
			if err != nil {
				klog.V(5).Infoln(err)
			}
		}
	}
}

// updateHWCounters
func (c *Collector) updateHWCounters(containerID string, ct *ProcessBPFMetrics, isSystemProcess bool) {
	// update HW counters
	for _, counterKey := range collector_metric.AvailableHWCounters {
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
		err := c.ContainersMetrics[containerID].CounterStats[counterKey].AddNewDelta(val)
		if err != nil {
			klog.V(5).Infoln(err)
		}
		// track system process metrics
		if isSystemProcess && config.EnableProcessMetrics {
			err := c.ProcessMetrics[ct.PID].CounterStats[counterKey].AddNewDelta(val)
			if err != nil {
				klog.V(5).Infoln(err)
			}
		}
	}
}

// updateBPFMetrics reads the BPF tables with process/pid/cgroupid metrics (CPU time, available HW counters)
func (c *Collector) updateBPFMetrics() {
	if c.bpfHCMeter == nil {
		return
	}
	foundContainer := make(map[string]bool)
	foundProcess := make(map[uint64]bool)
	keysToDelete := [][]byte{}
	var ct ProcessBPFMetrics
	for it := c.bpfHCMeter.Table.Iter(); it.Next(); {
		key := it.Key()
		data := it.Leaf()
		err := binary.Read(bytes.NewBuffer(data), utils.DetermineHostByteOrder(), &ct)
		if err != nil {
			klog.V(5).Infof("failed to decode received data: %v", err)
			continue // this only happens if there is a problem in the bpf code
		}

		// if ebpf map batch deletion operation is supported we add the key to the list otherwise we delete the key
		if config.EnabledBPFBatchDelete {
			keysToDelete = append(keysToDelete, key)
		} else {
			err = c.bpfHCMeter.Table.Delete(key) // deleting the element to reset the counter values
			if err != nil {
				klog.Infof("could not delete bpf table elements, err: %v", err)
			}
		}

		containerID, err := cgroup.GetContainerID(ct.CGroupID, ct.PID, config.EnabledEBPFCgroupID)
		if err != nil {
			klog.V(5).Infof("failed to resolve container for cGroup ID %v: %v, set containerID=%s", ct.CGroupID, err, c.systemProcessName)
		}

		isSystemProcess := containerID == c.systemProcessName

		c.createContainersMetricsIfNotExist(containerID, ct.CGroupID, ct.PID, config.EnabledEBPFCgroupID)
		c.ContainersMetrics[containerID].PID = ct.PID
		comm := C.GoString((*C.char)(unsafe.Pointer(&ct.Command)))
		// System process is the aggregation of all background process running outside kubernetes
		// this means that the list of process might be very large, so we will not add this information to the cache
		if !isSystemProcess {
			c.ContainersMetrics[containerID].SetLatestProcess(ct.CGroupID, ct.PID, comm)
		} else if config.EnableProcessMetrics {
			c.createProcessMetricsIfNotExist(ct.PID, comm)
			err := c.ProcessMetrics[ct.PID].CPUTime.AddNewDelta(ct.ProcessRunTime)
			if err != nil {
				klog.V(5).Infoln(err)
			}
		}

		c.ContainersMetrics[containerID].CurrProcesses++

		c.updateBasicBPF(containerID, &ct, isSystemProcess)
		c.updateHWCounters(containerID, &ct, isSystemProcess)

		// TODO: improve the removal of deleted containers from ContainersMetrics. Currently we verify the maxInactiveContainers using the foundContainer map
		foundContainer[containerID] = true
		if isSystemProcess {
			foundProcess[ct.PID] = true
		}
	}
	if config.EnabledBPFBatchDelete {
		err := attacher.TableDeleteBatch(c.bpfHCMeter.Module, c.bpfHCMeter.TableName, keysToDelete)
		// if the kernel does not support delete batch we delete all keys one by one
		if err != nil {
			c.bpfHCMeter.Table.DeleteAll()
			// if batch delete is not supported we disable it for the next time
			config.EnabledBPFBatchDelete = false
			klog.Infof("resetting EnabledBPFBatchDelete to %v", config.EnabledBPFBatchDelete)
		}
	}
	c.handleInactiveContainers(foundContainer)
	if config.EnableProcessMetrics {
		c.handleInactiveProcesses(foundProcess)
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
