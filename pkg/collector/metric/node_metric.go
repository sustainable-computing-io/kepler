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
	"github.com/sustainable-computing-io/kepler/pkg/power/accelerator/gpu"
	qat "github.com/sustainable-computing-io/kepler/pkg/power/accelerator/qat/source"
	"github.com/sustainable-computing-io/kepler/pkg/power/components/source"
)

const (
	CORE      = "core"
	DRAM      = "dram"
	UNCORE    = "uncore"
	PKG       = "package"
	GPU       = "gpu"
	OTHER     = "other"
	PLATFORM  = "platform"
	FREQUENCY = "frequency"
)

var (
	NodeName            = GetNodeName()
	NodeCPUArchitecture = getCPUArch()
	NodeCPUPackageMap   = getCPUPackageMap()

	// NodeMetricNames holds the name of the system metadata information.
	NodeMetadataFeatureNames []string = []string{"cpu_architecture"}
	// SystemMetadata holds the metadata regarding the system information
	NodeMetadataFeatureValues []string = []string{NodeCPUArchitecture}
)

type NodeMetrics struct {
	ResourceUsage map[string]float64

	// Absolute energy is the sum of Idle + Dynamic energy.
	AbsEnergyInCore     *types.UInt64StatCollection
	AbsEnergyInDRAM     *types.UInt64StatCollection
	AbsEnergyInUncore   *types.UInt64StatCollection
	AbsEnergyInPkg      *types.UInt64StatCollection
	AbsEnergyInGPU      *types.UInt64StatCollection
	AbsEnergyInOther    *types.UInt64StatCollection
	AbsEnergyInPlatform *types.UInt64StatCollection

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

	// Accelerator-QAT Utilization
	QATUtilization map[string]qat.DeviceUtilizationSample
}

func NewNodeMetrics() *NodeMetrics {
	return &NodeMetrics{
		ResourceUsage: make(map[string]float64),
		AbsEnergyInCore: &types.UInt64StatCollection{
			Stat: make(map[string]*types.UInt64Stat),
		},
		AbsEnergyInDRAM: &types.UInt64StatCollection{
			Stat: make(map[string]*types.UInt64Stat),
		},
		AbsEnergyInUncore: &types.UInt64StatCollection{
			Stat: make(map[string]*types.UInt64Stat),
		},
		AbsEnergyInPkg: &types.UInt64StatCollection{
			Stat: make(map[string]*types.UInt64Stat),
		},
		AbsEnergyInGPU: &types.UInt64StatCollection{
			Stat: make(map[string]*types.UInt64Stat),
		},
		AbsEnergyInOther: &types.UInt64StatCollection{
			Stat: make(map[string]*types.UInt64Stat),
		},
		AbsEnergyInPlatform: &types.UInt64StatCollection{
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
	ne.AbsEnergyInCore.ResetDeltaValues()
	ne.AbsEnergyInDRAM.ResetDeltaValues()
	ne.AbsEnergyInUncore.ResetDeltaValues()
	ne.AbsEnergyInPkg.ResetDeltaValues()
	ne.AbsEnergyInGPU.ResetDeltaValues()
	ne.AbsEnergyInPlatform.ResetDeltaValues()
	ne.DynEnergyInCore.ResetDeltaValues()
	ne.DynEnergyInDRAM.ResetDeltaValues()
	ne.DynEnergyInUncore.ResetDeltaValues()
	ne.DynEnergyInPkg.ResetDeltaValues()
	// gpu metric
	if config.EnabledGPU && gpu.IsGPUCollectionSupported() {
		ne.DynEnergyInGPU.ResetDeltaValues()
	}
	ne.DynEnergyInPlatform.ResetDeltaValues()
	ne.ResourceUsage = make(map[string]float64)
}

// AddNodeResResourceUsageFromContainerResResourceUsage adds the sum of all container resource usage as the node resource usage
func (ne *NodeMetrics) AddNodeResUsageFromContainerResUsage(containersMetrics map[string]*ContainerMetrics) {
	var IdleCPUUtilization uint64
	nodeResourceUsage := make(map[string]float64)
	for _, metricName := range ContainerFeaturesNames {
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

func normalize(val float64, shouldNormalize bool) float64 {
	if shouldNormalize {
		return val / float64(config.SamplePeriodSec)
	}
	return val
}

// ToEstimatorValues return values regarding metricNames.
// The metrics can be related to resource utilization or power consumption.
// Since Kepler collects metrics at intervals of SamplePeriodSec, which is greater than 1 second, and the power models are trained to estimate power in 1 second interval.
// It is necessary to normalize the resource utilization by the SamplePeriodSec. Note that this is important because the power curve can be different for higher or lower resource usage within 1 second interval.
func (ne *NodeMetrics) ToEstimatorValues(featuresName []string, shouldNormalize bool) []float64 {
	featureValues := []float64{}
	for _, feature := range featuresName {
		// verify all metrics that are part of the node resource usage metrics
		if value, exists := ne.ResourceUsage[feature]; exists {
			featureValues = append(featureValues, normalize(value, shouldNormalize))
			continue
		}
		// some features are not related to resource utilization, such as power metrics
		switch feature {
		case config.GeneralUsageMetric: // for UNCORE and OTHER resource usage
			featureValues = append(featureValues, 0)

		case config.GpuUsageMetric: // for GPU resource usage
			featureValues = append(featureValues, normalize(ne.ResourceUsage[config.GpuUsageMetric], shouldNormalize))

		case PKG + "_DYN": // for dynamic PKG power consumption
			featureValues = append(featureValues, normalize(float64(ne.GetSumDeltaDynEnergyFromAllSources(PKG)), shouldNormalize))

		case CORE + "_DYN": // for dynamic CORE power consumption
			featureValues = append(featureValues, normalize(float64(ne.GetSumDeltaDynEnergyFromAllSources(CORE)), shouldNormalize))

		case DRAM + "_DYN": // for dynamic PKG power consumption
			featureValues = append(featureValues, normalize(float64(ne.GetSumDeltaDynEnergyFromAllSources(DRAM)), shouldNormalize))

		case UNCORE + "_DYN": // for dynamic UNCORE power consumption
			featureValues = append(featureValues, normalize(float64(ne.GetSumDeltaDynEnergyFromAllSources(UNCORE)), shouldNormalize))

		case OTHER + "_DYN": // for dynamic OTHER power consumption
			featureValues = append(featureValues, normalize(float64(ne.GetSumDeltaDynEnergyFromAllSources(OTHER)), shouldNormalize))

		case PLATFORM + "_DYN": // for dynamic PLATFORM power consumption
			featureValues = append(featureValues, normalize(float64(ne.GetSumDeltaDynEnergyFromAllSources(PLATFORM)), shouldNormalize))

		case GPU + "_DYN": // for dynamic GPU power consumption
			featureValues = append(featureValues, normalize(float64(ne.GetSumDeltaDynEnergyFromAllSources(GPU)), shouldNormalize))

		case PKG + "_IDLE": // for idle PKG power consumption
			featureValues = append(featureValues, normalize(float64(ne.GetSumDeltaIdleEnergyFromAllSources(PKG)), shouldNormalize))

		case CORE + "_IDLE": // for idle CORE power consumption
			featureValues = append(featureValues, normalize(float64(ne.GetSumDeltaIdleEnergyFromAllSources(CORE)), shouldNormalize))

		case DRAM + "_IDLE": // for idle PKG power consumption
			featureValues = append(featureValues, normalize(float64(ne.GetSumDeltaIdleEnergyFromAllSources(DRAM)), shouldNormalize))

		case UNCORE + "_IDLE": // for idle UNCORE power consumption
			featureValues = append(featureValues, normalize(float64(ne.GetSumDeltaIdleEnergyFromAllSources(UNCORE)), shouldNormalize))

		case OTHER + "_IDLE": // for idle OTHER power consumption
			featureValues = append(featureValues, normalize(float64(ne.GetSumDeltaIdleEnergyFromAllSources(OTHER)), shouldNormalize))

		case PLATFORM + "_IDLE": // for idle PLATFORM power consumption
			featureValues = append(featureValues, normalize(float64(ne.GetSumDeltaIdleEnergyFromAllSources(PLATFORM)), shouldNormalize))

		case GPU + "_IDLE": // for idle GPU power consumption
			featureValues = append(featureValues, normalize(float64(ne.GetSumDeltaIdleEnergyFromAllSources(GPU)), shouldNormalize))

		default:
			klog.V(5).Infof("Unknown node feature: %s, adding 0 value", feature)
			featureValues = append(featureValues, 0)
		}
	}
	return featureValues
}

// SetNodePlatformEnergy adds the idle or absolute energy consumption from the node sensor.
// Absolute energy is the sum of Idle + Dynamic energy.
func (ne *NodeMetrics) SetNodePlatformEnergy(platformEnergy map[string]float64, gauge, isIdleEnergy bool) {
	for sensorID, energy := range platformEnergy {
		if gauge {
			if isIdleEnergy {
				ne.IdleEnergyInPlatform.SetDeltaStat(sensorID, uint64(math.Ceil(energy)))
			} else {
				ne.AbsEnergyInPlatform.SetDeltaStat(sensorID, uint64(math.Ceil(energy)))
			}
		} else {
			if isIdleEnergy {
				ne.IdleEnergyInPlatform.SetAggrStat(sensorID, uint64(math.Ceil(energy)))
			} else {
				ne.AbsEnergyInPlatform.SetAggrStat(sensorID, uint64(math.Ceil(energy)))
			}
		}
	}
}

// SetNodeComponentsEnergy adds the idle or absolute energy consumption collected from the node's components (e.g., using RAPL).
// Absolute energy is the sum of Idle + Dynamic energy.
func (ne *NodeMetrics) SetNodeComponentsEnergy(componentsEnergy map[int]source.NodeComponentsEnergy, gauge, isIdleEnergy bool) {
	for pkgID, energy := range componentsEnergy {
		key := strconv.Itoa(pkgID)
		if gauge {
			if isIdleEnergy {
				ne.IdleEnergyInCore.SetDeltaStat(key, energy.Core)
				ne.IdleEnergyInDRAM.SetDeltaStat(key, energy.DRAM)
				ne.IdleEnergyInUncore.SetDeltaStat(key, energy.Uncore)
				ne.IdleEnergyInPkg.SetDeltaStat(key, energy.Pkg)
			} else {
				ne.AbsEnergyInCore.SetDeltaStat(key, energy.Core)
				ne.AbsEnergyInDRAM.SetDeltaStat(key, energy.DRAM)
				ne.AbsEnergyInUncore.SetDeltaStat(key, energy.Uncore)
				ne.AbsEnergyInPkg.SetDeltaStat(key, energy.Pkg)
			}
		} else {
			if isIdleEnergy {
				ne.IdleEnergyInCore.SetAggrStat(key, energy.Core)
				ne.IdleEnergyInDRAM.SetAggrStat(key, energy.DRAM)
				ne.IdleEnergyInUncore.SetAggrStat(key, energy.Uncore)
				ne.IdleEnergyInPkg.SetAggrStat(key, energy.Pkg)
			} else {
				ne.AbsEnergyInCore.SetAggrStat(key, energy.Core)
				ne.AbsEnergyInDRAM.SetAggrStat(key, energy.DRAM)
				ne.AbsEnergyInUncore.SetAggrStat(key, energy.Uncore)
				ne.AbsEnergyInPkg.SetAggrStat(key, energy.Pkg)
			}
		}
	}
}

// SetNodeGPUEnergy adds the lastest energy consumption of each GPU power consumption.
// Right now we don't support other types of accelerators than GPU, but we will in the future.
func (ne *NodeMetrics) SetNodeGPUEnergy(gpuEnergy []uint32, isIdleEnergy bool) {
	for gpuID, energy := range gpuEnergy {
		key := strconv.Itoa(gpuID)
		if isIdleEnergy {
			ne.IdleEnergyInGPU.SetDeltaStat(key, uint64(energy))
		} else {
			ne.AbsEnergyInGPU.SetDeltaStat(key, uint64(energy))
		}
	}
}

func (ne *NodeMetrics) UpdateIdleEnergyWithMinValue(isComponentsSystemCollectionSupported bool) {
	// gpu metric
	if config.EnabledGPU && gpu.IsGPUCollectionSupported() {
		ne.CalcIdleEnergy(GPU)
	}

	if isComponentsSystemCollectionSupported {
		ne.CalcIdleEnergy(CORE)
		ne.CalcIdleEnergy(DRAM)
		ne.CalcIdleEnergy(UNCORE)
		ne.CalcIdleEnergy(PKG)
		ne.CalcIdleEnergy(PLATFORM)
	}
	// reset
	ne.FoundNewIdleState = false
}

func (ne *NodeMetrics) CalcIdleEnergy(component string) {
	totalStatCollection := ne.getAbsoluteEnergyStatCollection(component)
	idleStatCollection := ne.getIdleEnergyStatCollection(component)
	for id := range totalStatCollection.Stat {
		delta := totalStatCollection.Stat[id].Delta
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
	for pkgID := range ne.AbsEnergyInPkg.Stat {
		ne.CalcDynEnergy(PKG, pkgID)
		ne.CalcDynEnergy(CORE, pkgID)
		ne.CalcDynEnergy(UNCORE, pkgID)
		ne.CalcDynEnergy(DRAM, pkgID)
	}
	for sensorID := range ne.AbsEnergyInPlatform.Stat {
		ne.CalcDynEnergy(PLATFORM, sensorID)
	}
	// gpu metric
	if config.EnabledGPU && gpu.IsGPUCollectionSupported() {
		for gpuID := range ne.AbsEnergyInGPU.Stat {
			ne.CalcDynEnergy(GPU, gpuID)
		}
	}
}

func (ne *NodeMetrics) CalcDynEnergy(component, id string) {
	total := ne.getAbsoluteEnergyStatCollection(component).Stat[id].Delta
	klog.V(5).Infof("Energy stat: %v (%s)", ne.getIdleEnergyStatCollection(component).Stat, id)
	idle := uint64(0)
	if idleStat, found := ne.getIdleEnergyStatCollection(component).Stat[id]; found {
		idle = idleStat.Delta
	}
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
		ne.AbsEnergyInPkg.SumAllDeltaValues(), ne.AbsEnergyInCore.SumAllDeltaValues(), ne.AbsEnergyInDRAM.SumAllDeltaValues(), ne.AbsEnergyInUncore.SumAllDeltaValues(), ne.AbsEnergyInGPU.SumAllDeltaValues(), ne.AbsEnergyInOther.SumAllDeltaValues())
}

// GetAggrDynEnergyPerID returns the aggr dynamic energy from all source (e.g. package or gpu ids)
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

// GetSumAggrDynEnergyFromAllSources returns the sum of aggr dynamic energy of all source (e.g. package or gpu ids)
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

// GetSumDeltaIdleEnergyFromAllSources returns the sum of delta idle energy of all source (e.g. package or gpu ids)
func (ne *NodeMetrics) GetSumDeltaIdleEnergyFromAllSources(component string) uint64 {
	var idleEnergy uint64
	for _, val := range ne.getIdleEnergyStatCollection(component).Stat {
		idleEnergy += val.Delta
	}
	return idleEnergy
}

// GetSumAggrIdleEnergyFromAllSources returns the sum of aggr idle energy of all source (e.g. package or gpu ids)
func (ne *NodeMetrics) GetSumAggrIdleEnergyFromAllSources(component string) uint64 {
	var idleEnergy uint64
	for _, val := range ne.getIdleEnergyStatCollection(component).Stat {
		idleEnergy += val.Aggr
	}
	return idleEnergy
}

func (ne *NodeMetrics) getAbsoluteEnergyStatCollection(component string) (energyStat *types.UInt64StatCollection) {
	switch component {
	case PKG:
		return ne.AbsEnergyInPkg
	case CORE:
		return ne.AbsEnergyInCore
	case DRAM:
		return ne.AbsEnergyInDRAM
	case UNCORE:
		return ne.AbsEnergyInUncore
	case GPU:
		return ne.AbsEnergyInGPU
	case OTHER:
		return ne.AbsEnergyInOther
	case PLATFORM:
		return ne.AbsEnergyInPlatform
	default:
		klog.Fatalf("AbsoluteEnergy component type %s is unknown\n", component)
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
