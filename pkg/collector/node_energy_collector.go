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
	"github.com/sustainable-computing-io/kepler/pkg/power/accelerator/gpu"
	"github.com/sustainable-computing-io/kepler/pkg/power/components"
	"github.com/sustainable-computing-io/kepler/pkg/power/platform"

	"k8s.io/klog/v2"
)

const (
	idlePower = true
	absPower  = false
	gauge     = true
	counter   = false
)

// updateNodeResourceUsage updates node resource usage with the total container resource usage
// The container metrics are for the kubernetes containers and system/OS processes
// TODO: verify if the cgroup metrics are also accounting for the OS, not only containers
func (c *Collector) updateNodeResourceUsage() {
	c.NodeMetrics.AddNodeResUsageFromContainerResUsage(c.ContainersMetrics)
}

// updateMeasuredNodeEnergy updates the node platform power consumption, i.e, the node total power consumption
func (c *Collector) updatePlatformEnergy() {
	if platform.IsSystemCollectionSupported() {
		nodePlatformEnergy, _ := platform.GetAbsEnergyFromPlatform()
		c.NodeMetrics.SetNodePlatformEnergy(nodePlatformEnergy, gauge, absPower)
	} else if model.IsNodePlatformPowerModelEnabled() {
		model.UpdateNodePlatformEnergy(&c.NodeMetrics)
	}
}

// updateMeasuredNodeEnergy updates each node component power consumption, i.e., the CPU core, uncore, package/socket and DRAM
func (c *Collector) updateNodeComponentsEnergy(wg *sync.WaitGroup) {
	defer wg.Done()
	if components.IsSystemCollectionSupported() {
		nodeComponentsEnergy := components.GetAbsEnergyFromNodeComponents()
		// the RAPL metrics return counter metrics not gauge
		c.NodeMetrics.SetNodeComponentsEnergy(nodeComponentsEnergy, counter, absPower)
	} else if model.IsNodeComponentPowerModelEnabled() {
		model.UpdateNodeComponentEnergy(&c.NodeMetrics)
	} else {
		klog.V(5).Info("No nodeComponentsEnergy found, node components energy metrics is not exposed ")
	}
}

// updateNodeGPUEnergy updates each GPU power consumption. Right now we don't support other types of accelerators
func (c *Collector) updateNodeGPUEnergy(wg *sync.WaitGroup) {
	defer wg.Done()
	if config.EnabledGPU {
		gpuEnergy := gpu.GetAbsEnergyFromGPU()
		c.NodeMetrics.SetNodeGPUEnergy(gpuEnergy, absPower)
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

// updateNodeIdleEnergy calculates the node idle energy consumption based on the minimum power consumption when real-time system power metrics are accessible.
// When the node power model estimator is utilized, the idle power is updated with the estimated power considering minimal resource utilization.
func (c *Collector) updateNodeIdleEnergy() {
	isComponentsSystemCollectionSupported := components.IsSystemCollectionSupported()
	// the idle energy is only updated if we find the node using less resources than previously observed
	// TODO: Use regression to estimate the idle power when real-time system power metrics are available, instead of relying on the minimum power consumption.
	c.NodeMetrics.UpdateIdleEnergyWithMinValue(isComponentsSystemCollectionSupported)
	if !isComponentsSystemCollectionSupported {
		// if power collection on components is not supported, try using estimator to update idle energy
		if model.IsNodeComponentPowerModelEnabled() {
			model.UpdateNodeComponentIdleEnergy(&c.NodeMetrics)
		}
		if model.IsNodePlatformPowerModelEnabled() {
			model.UpdateNodePlatformIdleEnergy(&c.NodeMetrics)
		}
	}
}

// updateNodeEnergyMetrics updates the node energy consumption of each component
func (c *Collector) updateNodeEnergyMetrics() {
	var wgNode sync.WaitGroup
	wgNode.Add(3)
	go c.updateNodeComponentsEnergy(&wgNode)
	go c.updateNodeAvgCPUFrequency(&wgNode)
	go c.updateNodeGPUEnergy(&wgNode)
	wgNode.Wait()
	// update platform power later to avoid race condition when using estimation power model
	c.updatePlatformEnergy()

	// after updating the total energy we calculate the idle, dynamic and other components energy
	c.updateNodeIdleEnergy()
	c.NodeMetrics.UpdateDynEnergy()
	c.NodeMetrics.SetNodeOtherComponentsEnergy()
}
