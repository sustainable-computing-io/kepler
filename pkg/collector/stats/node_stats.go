/*
Copyright 2021-2024.

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

package stats

import (
	"fmt"

	"github.com/sustainable-computing-io/kepler/pkg/config"
	"github.com/sustainable-computing-io/kepler/pkg/node"
	acc "github.com/sustainable-computing-io/kepler/pkg/sensors/accelerator"
	"github.com/sustainable-computing-io/kepler/pkg/utils"
)

type NodeStats struct {
	Stats

	// IdleResUtilization is used to determine idle pmap[string]eriods
	IdleResUtilization map[string]uint64

	// nodeInfo allows access to node information
	nodeInfo node.Node
}

func NewNodeStats() *NodeStats {
	return &NodeStats{
		Stats:              *NewStats(),
		IdleResUtilization: map[string]uint64{},
		nodeInfo:           node.NewNodeInfo(),
	}
}

// ResetDeltaValues reset all delta values to 0
func (ne *NodeStats) ResetDeltaValues() {
	ne.Stats.ResetDeltaValues()
}

func (ne *NodeStats) UpdateIdleEnergyWithMinValue(isComponentsSystemCollectionSupported bool) {
	// gpu metric
	if config.IsGPUEnabled() {
		if acc.GetActiveAcceleratorByType(config.GPU) != nil {
			ne.CalcIdleEnergy(config.AbsEnergyInGPU, config.IdleEnergyInGPU, config.GPUComputeUtilization)
		}
	}

	if isComponentsSystemCollectionSupported {
		ne.CalcIdleEnergy(config.AbsEnergyInCore, config.IdleEnergyInCore, config.CPUTime)
		ne.CalcIdleEnergy(config.AbsEnergyInDRAM, config.IdleEnergyInDRAM, config.CPUTime) // TODO: we should use another resource for DRAM
		ne.CalcIdleEnergy(config.AbsEnergyInUnCore, config.IdleEnergyInUnCore, config.CPUTime)
		ne.CalcIdleEnergy(config.AbsEnergyInPkg, config.IdleEnergyInPkg, config.CPUTime)
		ne.CalcIdleEnergy(config.AbsEnergyInPlatform, config.IdleEnergyInPlatform, config.CPUTime)
	}
}

func (ne *NodeStats) CalcIdleEnergy(absM, idleM, resouceUtil string) {
	newTotalResUtilization := ne.ResourceUsage[resouceUtil].SumAllDeltaValues()
	currIdleTotalResUtilization := ne.IdleResUtilization[resouceUtil]

	for socketID, value := range ne.EnergyUsage[absM] {
		newIdleDelta := value.GetDelta()
		if newIdleDelta == 0 {
			// during the first power collection iterations, the delta values could be 0, so we skip until there are delta values
			continue
		}

		// add any value if there is no idle power yet
		if _, exist := ne.EnergyUsage[idleM][socketID]; !exist {
			ne.EnergyUsage[idleM].SetDeltaStat(socketID, newIdleDelta)
			// store the current CPU utilization to find a new idle power later
			ne.IdleResUtilization[resouceUtil] = newTotalResUtilization
		} else {
			currIdleDelta := ne.EnergyUsage[idleM][socketID].GetDelta()
			// verify if there is a new minimal energy consumption for the given resource
			// TODO: fix verifying the aggregated resource utilization from all sockets, the update the energy per socket can lead to inconsistency
			if (newTotalResUtilization <= currIdleTotalResUtilization) || (currIdleDelta == 0) {
				if (currIdleDelta == 0) || (currIdleDelta >= newIdleDelta) {
					ne.EnergyUsage[idleM].SetDeltaStat(socketID, newIdleDelta)
					ne.IdleResUtilization[resouceUtil] = newTotalResUtilization
					continue
				}
			}
			if currIdleDelta == 0 {
				continue
			}
			// as the dynamic and absolute power, the idle power is also a counter to be exported to prometheus
			// therefore, we accumulate the minimal found idle if no new one was found
			ne.EnergyUsage[idleM].SetDeltaStat(socketID, currIdleDelta)
		}
	}
}

// SetNodeOtherComponentsEnergy adds the latest energy consumption collected from the other node's components than CPU and DRAM
// Other components energy is a special case where the energy is calculated and not measured
func (ne *NodeStats) SetNodeOtherComponentsEnergy() {
	// calculate dynamic energy in other components
	dynCPUComponentsEnergy := ne.EnergyUsage[config.DynEnergyInPkg].SumAllDeltaValues() +
		ne.EnergyUsage[config.DynEnergyInDRAM].SumAllDeltaValues() +
		ne.EnergyUsage[config.DynEnergyInGPU].SumAllDeltaValues()

	dynPlatformEnergy := ne.EnergyUsage[config.DynEnergyInPlatform].SumAllDeltaValues()

	if dynPlatformEnergy >= dynCPUComponentsEnergy {
		otherComponentEnergy := dynPlatformEnergy - dynCPUComponentsEnergy
		ne.EnergyUsage[config.DynEnergyInOther].SetDeltaStat(utils.GenericSocketID, otherComponentEnergy)
	}

	// calculate idle energy in other components
	idleCPUComponentsEnergy := ne.EnergyUsage[config.IdleEnergyInPkg].SumAllDeltaValues() +
		ne.EnergyUsage[config.IdleEnergyInDRAM].SumAllDeltaValues() +
		ne.EnergyUsage[config.IdleEnergyInGPU].SumAllDeltaValues()

	idlePlatformEnergy := ne.EnergyUsage[config.IdleEnergyInPlatform].SumAllDeltaValues()

	if idlePlatformEnergy >= idleCPUComponentsEnergy {
		otherComponentEnergy := idlePlatformEnergy - idleCPUComponentsEnergy
		ne.EnergyUsage[config.IdleEnergyInOther].SetDeltaStat(utils.GenericSocketID, otherComponentEnergy)
	}
}

func (ne *NodeStats) String() string {
	return fmt.Sprintf("node energy (mJ): \n"+
		"%v\n", ne.Stats.String(),
	)
}

func (ne *NodeStats) MetadataFeatureNames() []string {
	return ne.nodeInfo.MetadataFeatureNames()
}

func (ne *NodeStats) MetadataFeatureValues() []string {
	return ne.nodeInfo.MetadataFeatureValues()
}

func (ne *NodeStats) CPUArchitecture() string {
	return ne.nodeInfo.CPUArchitecture()
}

func (ne *NodeStats) NodeName() string {
	return ne.nodeInfo.Name()
}
