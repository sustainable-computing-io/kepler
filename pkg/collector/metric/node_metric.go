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

package metric

import (
	"fmt"
	"math"
	"strconv"

	"k8s.io/klog/v2"

	"github.com/sustainable-computing-io/kepler/pkg/collector/metric/types"
	"github.com/sustainable-computing-io/kepler/pkg/config"
	"github.com/sustainable-computing-io/kepler/pkg/power/accelerator"
	"github.com/sustainable-computing-io/kepler/pkg/power/components/source"
)

const (
	CORE      = "core"
	DRAM      = "dram"
	UNCORE    = "uncore"
	PKG       = "pkg"
	GPU       = "gpu"
	OTHER     = "other"
	PLATFORM  = "platform"
	FREQUENCY = "frequency"
)

var (
	NodeName            = getNodeName()
	NodeCPUArchitecture = getCPUArch()
	NodeCPUPackageMap   = getCPUPackageMap()

	// NodeMetricNames holds the name of the system metadata information.
	NodeMetadataNames []string = []string{"cpu_architecture"}
	// SystemMetadata holds the metadata regarding the system information
	NodeMetadataValues []string = []string{NodeCPUArchitecture}
)

type NodeMetrics struct {
	ResourceUsage map[string]float64

	TotalEnergyInCore     *types.UInt64StatCollection
	TotalEnergyInDRAM     *types.UInt64StatCollection
	TotalEnergyInUncore   *types.UInt64StatCollection
	TotalEnergyInPkg      *types.UInt64StatCollection
	TotalEnergyInGPU      *types.UInt64StatCollection
	TotalEnergyInOther    *types.UInt64StatCollection
	TotalEnergyInPlatform *types.UInt64StatCollection

	DynEnergyInCore     *types.UInt64StatCollection
	DynEnergyInDRAM     *types.UInt64StatCollection
	DynEnergyInUncore   *types.UInt64StatCollection
	DynEnergyInPkg      *types.UInt64StatCollection
	DynEnergyInGPU      *types.UInt64StatCollection
	DynEnergyInOther    *types.UInt64StatCollection
	DynEnergyInPlatform *types.UInt64StatCollection

	IdleEnergyInCore     *types.UInt64StatCollection
	IdleEnergyInDRAM     *types.UInt64StatCollection
	IdleEnergyInUncore   *types.UInt64StatCollection
	IdleEnergyInPkg      *types.UInt64StatCollection
	IdleEnergyInGPU      *types.UInt64StatCollection
	IdleEnergyInOther    *types.UInt64StatCollection
	IdleEnergyInPlatform *types.UInt64StatCollection

	CPUFrequency map[int32]uint64

	// IdleCPUUtilization is used to determine idle periods
	IdleCPUUtilization uint64
	FoundNewIdleState  bool
}

func NewNodeMetrics() *NodeMetrics {
	return &NodeMetrics{
		ResourceUsage: make(map[string]float64),
		TotalEnergyInCore: &types.UInt64StatCollection{
			Stat: make(map[string]*types.UInt64Stat),
		},
		TotalEnergyInDRAM: &types.UInt64StatCollection{
			Stat: make(map[string]*types.UInt64Stat),
		},
		TotalEnergyInUncore: &types.UInt64StatCollection{
			Stat: make(map[string]*types.UInt64Stat),
		},
		TotalEnergyInPkg: &types.UInt64StatCollection{
			Stat: make(map[string]*types.UInt64Stat),
		},
		TotalEnergyInGPU: &types.UInt64StatCollection{
			Stat: make(map[string]*types.UInt64Stat),
		},
		TotalEnergyInOther: &types.UInt64StatCollection{
			Stat: make(map[string]*types.UInt64Stat),
		},
		TotalEnergyInPlatform: &types.UInt64StatCollection{
			Stat: make(map[string]*types.UInt64Stat),
		},

		DynEnergyInCore: &types.UInt64StatCollection{
			Stat: make(map[string]*types.UInt64Stat),
		},
		DynEnergyInDRAM: &types.UInt64StatCollection{
			Stat: make(map[string]*types.UInt64Stat),
		},
		DynEnergyInUncore: &types.UInt64StatCollection{
			Stat: make(map[string]*types.UInt64Stat),
		},
		DynEnergyInPkg: &types.UInt64StatCollection{
			Stat: make(map[string]*types.UInt64Stat),
		},
		DynEnergyInGPU: &types.UInt64StatCollection{
			Stat: make(map[string]*types.UInt64Stat),
		},
		DynEnergyInOther: &types.UInt64StatCollection{
			Stat: make(map[string]*types.UInt64Stat),
		},
		DynEnergyInPlatform: &types.UInt64StatCollection{
			Stat: make(map[string]*types.UInt64Stat),
		},

		IdleEnergyInCore: &types.UInt64StatCollection{
			Stat: make(map[string]*types.UInt64Stat),
		},
		IdleEnergyInDRAM: &types.UInt64StatCollection{
			Stat: make(map[string]*types.UInt64Stat),
		},
		IdleEnergyInUncore: &types.UInt64StatCollection{
			Stat: make(map[string]*types.UInt64Stat),
		},
		IdleEnergyInPkg: &types.UInt64StatCollection{
			Stat: make(map[string]*types.UInt64Stat),
		},
		IdleEnergyInGPU: &types.UInt64StatCollection{
			Stat: make(map[string]*types.UInt64Stat),
		},
		IdleEnergyInOther: &types.UInt64StatCollection{
			Stat: make(map[string]*types.UInt64Stat),
		},
		IdleEnergyInPlatform: &types.UInt64StatCollection{
			Stat: make(map[string]*types.UInt64Stat),
		},
	}
}

func (ne *NodeMetrics) ResetDeltaValues() {
	ne.TotalEnergyInCore.ResetDeltaValues()
	ne.TotalEnergyInDRAM.ResetDeltaValues()
	ne.TotalEnergyInUncore.ResetDeltaValues()
	ne.TotalEnergyInPkg.ResetDeltaValues()
	ne.TotalEnergyInGPU.ResetDeltaValues()
	ne.TotalEnergyInPlatform.ResetDeltaValues()
	ne.DynEnergyInCore.ResetDeltaValues()
	ne.DynEnergyInDRAM.ResetDeltaValues()
	ne.DynEnergyInUncore.ResetDeltaValues()
	ne.DynEnergyInPkg.ResetDeltaValues()
	// gpu metric
	if config.EnabledGPU && accelerator.IsGPUCollectionSupported() {
		ne.DynEnergyInGPU.ResetDeltaValues()
	}
	ne.DynEnergyInPlatform.ResetDeltaValues()
	ne.ResourceUsage = make(map[string]float64)
}

// AddNodeResResourceUsageFromContainerResResourceUsage adds the sum of all container resource usage as the node resource usage
func (ne *NodeMetrics) AddNodeResUsageFromContainerResUsage(containersMetrics map[string]*ContainerMetrics) {
	var IdleCPUUtilization uint64
	nodeResourceUsage := make(map[string]float64)
	for _, metricName := range ContainerMetricNames {
		nodeResourceUsage[metricName] = 0
		for _, container := range containersMetrics {
			delta, _, _ := container.getIntDeltaAndAggrValue(metricName)
			nodeResourceUsage[metricName] += float64(delta)
			if metricName == config.CPUInstruction {
				IdleCPUUtilization += delta
			}
		}
	}
	ne.ResourceUsage = nodeResourceUsage
	if (ne.IdleCPUUtilization > IdleCPUUtilization) || (ne.IdleCPUUtilization == 0) {
		ne.FoundNewIdleState = true
		ne.IdleCPUUtilization = IdleCPUUtilization
	}
}

// SetLastestPlatformEnergy adds the lastest energy consumption from the node sensor
func (ne *NodeMetrics) SetLastestPlatformEnergy(platformEnergy map[string]float64, gauge bool) {
	for sensorID, energy := range platformEnergy {
		if gauge {
			ne.TotalEnergyInPlatform.SetDeltaStat(sensorID, uint64(math.Ceil(energy)))
		} else {
			ne.TotalEnergyInPlatform.SetAggrStat(sensorID, uint64(math.Ceil(energy)))
		}
	}
}

// SetNodeComponentsEnergy adds the lastest energy consumption collected from the node's components (e.g., using RAPL)
func (ne *NodeMetrics) SetNodeComponentsEnergy(componentsEnergy map[int]source.NodeComponentsEnergy, gauge bool) {
	for pkgID, energy := range componentsEnergy {
		key := strconv.Itoa(pkgID)
		if gauge {
			ne.TotalEnergyInCore.SetDeltaStat(key, energy.Core)
			ne.TotalEnergyInDRAM.SetDeltaStat(key, energy.DRAM)
			ne.TotalEnergyInUncore.SetDeltaStat(key, energy.Uncore)
			ne.TotalEnergyInPkg.SetDeltaStat(key, energy.Pkg)
		} else {
			ne.TotalEnergyInCore.SetAggrStat(key, energy.Core)
			ne.TotalEnergyInDRAM.SetAggrStat(key, energy.DRAM)
			ne.TotalEnergyInUncore.SetAggrStat(key, energy.Uncore)
			ne.TotalEnergyInPkg.SetAggrStat(key, energy.Pkg)
		}
	}
}

// AddNodeGPUEnergy adds the lastest energy consumption of each GPU power consumption.
// Right now we don't support other types of accelerators than GPU, but we will in the future.
func (ne *NodeMetrics) AddNodeGPUEnergy(gpuEnergy []uint32) {
	for gpuID, energy := range gpuEnergy {
		key := strconv.Itoa(gpuID)
		ne.TotalEnergyInGPU.SetDeltaStat(key, uint64(energy))
	}
}

func (ne *NodeMetrics) UpdateIdleEnergy() {
	ne.CalcIdleEnergy(CORE)
	ne.CalcIdleEnergy(DRAM)
	ne.CalcIdleEnergy(UNCORE)
	ne.CalcIdleEnergy(PKG)
	// gpu metric
	if config.EnabledGPU && accelerator.IsGPUCollectionSupported() {
		ne.CalcIdleEnergy(GPU)
	}
	ne.CalcIdleEnergy(PLATFORM)
	// reset
	ne.FoundNewIdleState = false
}

func (ne *NodeMetrics) CalcIdleEnergy(component string) {
	toalStatCollection := ne.getTotalEnergyStatCollection(component)
	idleStatCollection := ne.getIdleEnergyStatCollection(component)
	for id := range toalStatCollection.Stat {
		delta := toalStatCollection.Stat[id].Delta
		if _, exist := idleStatCollection.Stat[id]; !exist {
			idleStatCollection.SetDeltaStat(id, delta)
		} else {
			idleDelta := idleStatCollection.Stat[id].Delta
			// only updates the idle energy if the resource utilization is low. i.e., ne.FoundNewIdleState == true
			if ((idleDelta == 0) || (idleDelta > delta)) && ((ne.FoundNewIdleState) || (ne.IdleCPUUtilization == 0)) {
				idleStatCollection.SetDeltaStat(id, delta)
			} else {
				idleStatCollection.SetDeltaStat(id, idleDelta)
			}
		}
	}
}

// UpdateDynEnergy calculates the dynamic energy
func (ne *NodeMetrics) UpdateDynEnergy() {
	for pkgID := range ne.TotalEnergyInPkg.Stat {
		ne.CalcDynEnergy(PKG, pkgID)
		ne.CalcDynEnergy(CORE, pkgID)
		ne.CalcDynEnergy(UNCORE, pkgID)
		ne.CalcDynEnergy(DRAM, pkgID)
	}
	for sensorID := range ne.TotalEnergyInPlatform.Stat {
		ne.CalcDynEnergy(PLATFORM, sensorID)
	}
	// gpu metric
	if config.EnabledGPU && accelerator.IsGPUCollectionSupported() {
		for gpuID := range ne.TotalEnergyInGPU.Stat {
			ne.CalcDynEnergy(GPU, gpuID)
		}
	}
}

func (ne *NodeMetrics) CalcDynEnergy(component, id string) {
	total := ne.getTotalEnergyStatCollection(component).Stat[id].Delta
	idle := ne.getIdleEnergyStatCollection(component).Stat[id].Delta
	dyn := calcDynEnergy(total, idle)
	ne.getDynEnergyStatCollection(component).SetDeltaStat(id, dyn)
}

func calcDynEnergy(totalE, idleE uint64) uint64 {
	if (totalE == 0) || (idleE == 0) || (totalE < idleE) {
		return 0
	}
	return totalE - idleE
}

// SetNodeOtherComponentsEnergy adds the lastest energy consumption collected from the other node's components than CPU and DRAM
// Other components energy is a special case where the energy is calculated and not measured
func (ne *NodeMetrics) SetNodeOtherComponentsEnergy() {
	dynCPUComponentsEnergy := ne.DynEnergyInPkg.SumAllDeltaValues() +
		ne.DynEnergyInDRAM.SumAllDeltaValues() +
		ne.DynEnergyInGPU.SumAllDeltaValues()
	dynPlatformEnergy := ne.DynEnergyInPlatform.SumAllDeltaValues()
	if dynPlatformEnergy > dynCPUComponentsEnergy {
		otherComponentEnergy := dynPlatformEnergy - dynCPUComponentsEnergy
		ne.DynEnergyInOther.SetDeltaStat(OTHER, otherComponentEnergy)
	}

	idleCPUComponentsEnergy := ne.IdleEnergyInPkg.SumAllDeltaValues() +
		ne.IdleEnergyInDRAM.SumAllDeltaValues() +
		ne.IdleEnergyInGPU.SumAllDeltaValues()
	idlePlatformEnergy := ne.IdleEnergyInPlatform.SumAllDeltaValues()
	if idlePlatformEnergy > idleCPUComponentsEnergy {
		otherComponentEnergy := idlePlatformEnergy - idleCPUComponentsEnergy
		ne.IdleEnergyInOther.SetDeltaStat(OTHER, otherComponentEnergy)
	}
}

func (ne *NodeMetrics) GetNodeResUsagePerResType(resource string) (float64, error) {
	data, ok := ne.ResourceUsage[resource]
	if ok {
		return data, nil
	}
	return 0, fmt.Errorf("resource %s not found", resource)
}

func (ne *NodeMetrics) String() string {
	return fmt.Sprintf("node delta energy (mJ): \n"+
		"\tePkg: %d (eCore: %d eDram: %d eUncore: %d) eGPU: %d eOther: %d \n",
		ne.TotalEnergyInPkg.SumAllDeltaValues(), ne.TotalEnergyInCore.SumAllDeltaValues(), ne.TotalEnergyInDRAM.SumAllDeltaValues(), ne.TotalEnergyInUncore.SumAllDeltaValues(), ne.TotalEnergyInGPU.SumAllDeltaValues(), ne.TotalEnergyInOther.SumAllDeltaValues())
}

// GetAggrDynEnergyPerID returns the delta dynamic energy from all source (e.g. package or gpu ids)
func (ne *NodeMetrics) GetAggrDynEnergyPerID(component, id string) uint64 {
	statCollection := ne.getDynEnergyStatCollection(component)
	if _, exist := statCollection.Stat[id]; exist {
		return statCollection.Stat[id].Aggr
	}
	return uint64(0)
}

// GetDeltaDynEnergyPerID returns the delta dynamic energy from all source (e.g. package or gpu ids)
func (ne *NodeMetrics) GetDeltaDynEnergyPerID(component, id string) uint64 {
	statCollection := ne.getDynEnergyStatCollection(component)
	if _, exist := statCollection.Stat[id]; exist {
		return statCollection.Stat[id].Delta
	}
	return uint64(0)
}

// GetSumAggrDynEnergyFromAllSources returns the sum of delta dynamic energy of all source (e.g. package or gpu ids)
func (ne *NodeMetrics) GetSumAggrDynEnergyFromAllSources(component string) uint64 {
	var dynamicEnergy uint64
	for _, val := range ne.getDynEnergyStatCollection(component).Stat {
		dynamicEnergy += val.Aggr
	}
	return dynamicEnergy
}

// GetSumDeltaDynEnergyFromAllSources returns the sum of delta dynamic energy of all source (e.g. package or gpu ids)
func (ne *NodeMetrics) GetSumDeltaDynEnergyFromAllSources(component string) uint64 {
	var dynamicEnergy uint64
	for _, val := range ne.getDynEnergyStatCollection(component).Stat {
		dynamicEnergy += val.Delta
	}
	return dynamicEnergy
}

// GetAggrIdleEnergyPerID returns the aggr idle energy for a given id
func (ne *NodeMetrics) GetAggrIdleEnergyPerID(component, id string) uint64 {
	statCollection := ne.getIdleEnergyStatCollection(component)
	if _, exist := statCollection.Stat[id]; exist {
		return statCollection.Stat[id].Aggr
	}
	return uint64(0)
}

// GetDeltaIdleEnergyPerID returns the delta idle energy from all source (e.g. package or gpu ids)
func (ne *NodeMetrics) GetDeltaIdleEnergyPerID(component, id string) uint64 {
	statCollection := ne.getIdleEnergyStatCollection(component)
	if _, exist := statCollection.Stat[id]; exist {
		return statCollection.Stat[id].Delta
	}
	return uint64(0)
}

// GetSumDeltaIdleEnergyromAllSources returns the sum of delta idle energy of all source (e.g. package or gpu ids)
func (ne *NodeMetrics) GetSumDeltaIdleEnergyromAllSources(component string) uint64 {
	var idleEnergy uint64
	for _, val := range ne.getIdleEnergyStatCollection(component).Stat {
		idleEnergy += val.Delta
	}
	return idleEnergy
}

// GetSumAggrIdleEnergyromAllSources returns the sum of delta idle energy of all source (e.g. package or gpu ids)
func (ne *NodeMetrics) GetSumAggrIdleEnergyromAllSources(component string) uint64 {
	var idleEnergy uint64
	for _, val := range ne.getIdleEnergyStatCollection(component).Stat {
		idleEnergy += val.Aggr
	}
	return idleEnergy
}

func (ne *NodeMetrics) getTotalEnergyStatCollection(component string) (energyStat *types.UInt64StatCollection) {
	switch component {
	case PKG:
		return ne.TotalEnergyInPkg
	case CORE:
		return ne.TotalEnergyInCore
	case DRAM:
		return ne.TotalEnergyInDRAM
	case UNCORE:
		return ne.TotalEnergyInUncore
	case GPU:
		return ne.TotalEnergyInGPU
	case OTHER:
		return ne.TotalEnergyInOther
	case PLATFORM:
		return ne.TotalEnergyInPlatform
	default:
		klog.Fatalf("TotalEnergy component type %s is unknown\n", component)
	}
	return
}

func (ne *NodeMetrics) getDynEnergyStatCollection(component string) (energyStat *types.UInt64StatCollection) {
	switch component {
	case PKG:
		return ne.DynEnergyInPkg
	case CORE:
		return ne.DynEnergyInCore
	case DRAM:
		return ne.DynEnergyInDRAM
	case UNCORE:
		return ne.DynEnergyInUncore
	case GPU:
		return ne.DynEnergyInGPU
	case OTHER:
		return ne.DynEnergyInOther
	case PLATFORM:
		return ne.DynEnergyInPlatform
	default:
		klog.Fatalf("DynEnergy component type %s is unknown\n", component)
	}
	return
}

func (ne *NodeMetrics) getIdleEnergyStatCollection(component string) (energyStat *types.UInt64StatCollection) {
	switch component {
	case PKG:
		return ne.IdleEnergyInPkg
	case CORE:
		return ne.IdleEnergyInCore
	case DRAM:
		return ne.IdleEnergyInDRAM
	case UNCORE:
		return ne.IdleEnergyInUncore
	case GPU:
		return ne.IdleEnergyInGPU
	case OTHER:
		return ne.IdleEnergyInOther
	case PLATFORM:
		return ne.IdleEnergyInPlatform
	default:
		klog.Fatalf("IdleEnergy component type %s is unknown\n", component)
	}
	return
}
