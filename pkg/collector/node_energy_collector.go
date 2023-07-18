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
	"sync"

	"github.com/sustainable-computing-io/kepler/pkg/bpfassets/attacher"
	"github.com/sustainable-computing-io/kepler/pkg/config"
	"github.com/sustainable-computing-io/kepler/pkg/model"
	"github.com/sustainable-computing-io/kepler/pkg/power/accelerator"
	"github.com/sustainable-computing-io/kepler/pkg/power/components"
	"github.com/sustainable-computing-io/kepler/pkg/power/platform"

	"k8s.io/klog/v2"
)

// updateNodeResourceUsage updates node resource usage with the total container resource usage
// The container metrics are for the kubernetes containers and system/OS processes
// TODO: verify if the cgroup metrics are also accounting for the OS, not only containers
func (c *Collector) updateNodeResourceUsage() {
	c.NodeMetrics.AddNodeResUsageFromContainerResUsage(c.ContainersMetrics)
}

// updateMeasuredNodeEnergy updates the node platfomr power consumption, i.e, the node total power consumption
func (c *Collector) updatePlatformEnergy(wg *sync.WaitGroup) {
	defer wg.Done()
	if platform.IsSystemCollectionSupported() {
		nodePlatformEnergy, _ := platform.GetEnergyFromPlatform()
		c.NodeMetrics.SetLastestPlatformEnergy(nodePlatformEnergy, true)
	} else if model.IsNodePlatformPowerModelEnabled() {
		nodePlatformEnergy := model.GetEstimatedNodePlatformPower(&c.NodeMetrics)
		c.NodeMetrics.SetLastestPlatformEnergy(nodePlatformEnergy, true)
	}
}

// updateMeasuredNodeEnergy updates each node component power consumption, i.e., the CPU core, uncore, package/socket and DRAM
func (c *Collector) updateNodeComponentsEnergy(wg *sync.WaitGroup) {
	defer wg.Done()
	if components.IsSystemCollectionSupported() {
		klog.V(5).Info("System energy collection is supported")
		nodeComponentsEnergy := components.GetNodeComponentsEnergy()
		// the RAPL metrics return counter metrics not gauge
		c.NodeMetrics.SetNodeComponentsEnergy(nodeComponentsEnergy, false)
	} else if model.IsNodeComponentPowerModelEnabled() {
		klog.V(5).Info("Node components power model collection is supported")
		nodeComponentsEnergy := model.GetNodeComponentPowers(&c.NodeMetrics)
		// the node components power model returns gauge mentrics
		c.NodeMetrics.SetNodeComponentsEnergy(nodeComponentsEnergy, true)
	} else {
		klog.V(1).Info("No nodeComponentsEnergy found, node components energy metrics is not exposed ")
	}
}

// updateNodeGPUEnergy updates each GPU power consumption. Right now we don't support other types of accelerators
func (c *Collector) updateNodeGPUEnergy(wg *sync.WaitGroup) {
	defer wg.Done()
	if config.EnabledGPU {
		gpuEnergy := accelerator.GetGpuEnergyPerGPU()
		c.NodeMetrics.AddNodeGPUEnergy(gpuEnergy)
	}
}

// updateNodeAvgCPUFrequency updates the average CPU frequency in each core
func (c *Collector) updateNodeAvgCPUFrequency(wg *sync.WaitGroup) {
	defer wg.Done()
	// update the cpu frequency using hardware counters when available because reading files can be very expensive
	if attacher.HardwareCountersEnabled {
		cpuFreq, err := attacher.CollectCPUFreq()
		if err == nil {
			c.NodeMetrics.CPUFrequency = cpuFreq
		}
	}
}

// updateNodeEnergyMetrics updates the node energy consumption of each component
func (c *Collector) updateNodeEnergyMetrics() {
	var wgNode sync.WaitGroup
	wgNode.Add(4)
	go c.updatePlatformEnergy(&wgNode)
	go c.updateNodeComponentsEnergy(&wgNode)
	go c.updateNodeAvgCPUFrequency(&wgNode)
	go c.updateNodeGPUEnergy(&wgNode)
	wgNode.Wait()
	// after updating the total energy we calculate the dynamic energy
	// the idle energy is only updated if we find the node using less resources than previously observed
	c.NodeMetrics.UpdateIdleEnergy()
	c.NodeMetrics.UpdateDynEnergy()
	c.NodeMetrics.SetNodeOtherComponentsEnergy()
}
