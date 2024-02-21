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

package energy

import (
	"fmt"
	"strconv"
	"sync"

	"github.com/sustainable-computing-io/kepler/pkg/collector/stats"
	"github.com/sustainable-computing-io/kepler/pkg/config"
	"github.com/sustainable-computing-io/kepler/pkg/model"
	"github.com/sustainable-computing-io/kepler/pkg/sensors/accelerator/gpu"
	"github.com/sustainable-computing-io/kepler/pkg/sensors/components"
	"github.com/sustainable-computing-io/kepler/pkg/sensors/platform"

	"k8s.io/klog/v2"
)

// UpdatePlatformEnergy updates the node platform power consumption, i.e, the node total power consumption
func UpdatePlatformEnergy(nodeStats *stats.NodeStats) {
	if platform.IsSystemCollectionSupported() {
		nodePlatformEnergy, _ := platform.GetAbsEnergyFromPlatform()
		for sourceID, energy := range nodePlatformEnergy {
			nodeStats.EnergyUsage[config.AbsEnergyInPlatform].SetDeltaStat(sourceID, uint64(energy))
		}
	} else if model.IsNodePlatformPowerModelEnabled() {
		model.UpdateNodePlatformEnergy(nodeStats)
	}
}

// UpdateNodeComponentsEnergy updates each node component power consumption, i.e., the CPU core, uncore, package/socket and DRAM
func UpdateNodeComponentsEnergy(nodeStats *stats.NodeStats, wg *sync.WaitGroup) {
	defer wg.Done()
	if components.IsSystemCollectionSupported() {
		nodeComponentsEnergy := components.GetAbsEnergyFromNodeComponents()
		// the RAPL metrics return counter metrics not gauge
		for socket, energy := range nodeComponentsEnergy {
			strID := strconv.Itoa(socket)
			nodeStats.EnergyUsage[config.AbsEnergyInPkg].SetAggrStat(strID, energy.Pkg)
			nodeStats.EnergyUsage[config.AbsEnergyInCore].SetAggrStat(strID, energy.Core)
			nodeStats.EnergyUsage[config.AbsEnergyInUnCore].SetAggrStat(strID, energy.Uncore)
			nodeStats.EnergyUsage[config.AbsEnergyInDRAM].SetAggrStat(strID, energy.DRAM)
		}
	} else if model.IsNodeComponentPowerModelEnabled() {
		model.UpdateNodeComponentEnergy(nodeStats)
	} else {
		klog.V(5).Info("No nodeComponentsEnergy found, node components energy metrics is not exposed ")
	}
}

// UpdateNodeGPUEnergy updates each GPU power consumption. Right now we don't support other types of accelerators
func UpdateNodeGPUEnergy(nodeStats *stats.NodeStats, wg *sync.WaitGroup) {
	defer wg.Done()
	if config.EnabledGPU && gpu.IsGPUCollectionSupported() {
		gpuEnergy := gpu.GetAbsEnergyFromGPU()
		for gpu, energy := range gpuEnergy {
			nodeStats.EnergyUsage[config.AbsEnergyInGPU].SetDeltaStat(fmt.Sprintf("%d", gpu), uint64(energy))
		}
	}
}

// UpdateNodeIdleEnergy calculates the node idle energy consumption based on the minimum power consumption when real-time system power metrics are accessible.
// When the node power model estimator is utilized, the idle power is updated with the estimated power considering minimal resource utilization.
func UpdateNodeIdleEnergy(nodeStats *stats.NodeStats) {
	isComponentsSystemCollectionSupported := components.IsSystemCollectionSupported()
	// the idle energy is only updated if we find the node using less resources than previously observed
	// TODO: Use regression to estimate the idle power when real-time system power metrics are available, instead of relying on the minimum power consumption.
	nodeStats.UpdateIdleEnergyWithMinValue(isComponentsSystemCollectionSupported)
	if !isComponentsSystemCollectionSupported {
		// if power collection on components is not supported, try using estimator to update idle energy
		if model.IsNodeComponentPowerModelEnabled() {
			model.UpdateNodeComponentIdleEnergy(nodeStats)
		}
		if model.IsNodePlatformPowerModelEnabled() {
			model.UpdateNodePlatformIdleEnergy(nodeStats)
		}
	}
}

// UpdateNodeEnergyMetrics updates the node energy consumption of each component
func UpdateNodeEnergyMetrics(nodeStats *stats.NodeStats) {
	var wgNode sync.WaitGroup
	wgNode.Add(2)
	go UpdateNodeComponentsEnergy(nodeStats, &wgNode)
	go UpdateNodeGPUEnergy(nodeStats, &wgNode)
	wgNode.Wait()
	// update platform power later to avoid race condition when using estimation power model
	UpdatePlatformEnergy(nodeStats)
	// after updating the total energy we calculate the idle, dynamic and other components energy
	UpdateNodeIdleEnergy(nodeStats)
	nodeStats.UpdateDynEnergy()
	nodeStats.SetNodeOtherComponentsEnergy()
}
