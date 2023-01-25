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
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/jszwec/csvutil"
	"k8s.io/klog/v2"

	"github.com/sustainable-computing-io/kepler/pkg/config"
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
	CPUModelDataPath = "/var/lib/kepler/data/normalized_cpu_arch.csv"

	NodeName            = getNodeName()
	NodeCPUArchitecture = getCPUArch()

	// TODO: the metada should be a map and not two slices
	// NodeMetricNames holds the name of the system metadata information.
	NodeMetadataNames []string = []string{"cpu_architecture"}
	// SystemMetadata holds the metadata regarding the system information
	NodeMetadataValues []string = []string{NodeCPUArchitecture}
)

type CPUModelData struct {
	Name         string `csv:"Name"`
	Architecture string `csv:"Architecture"`
}

func getNodeName() string {
	nodeName, err := os.Hostname()
	if err != nil {
		klog.Fatalf("could not get the node name: %s", err)
	}
	return nodeName
}

func getCPUArch() string {
	arch, err := getCPUArchitecture()
	if err == nil {
		return arch
	}
	return "unknown"
}

func getCPUArchitecture() (string, error) {
	// check if there is a CPU architecture override
	cpuArchOverride := config.CPUArchOverride
	if len(cpuArchOverride) > 0 {
		klog.V(2).Infof("cpu arch override: %v\n", cpuArchOverride)
		return cpuArchOverride, nil
	}
	output, err := exec.Command("archspec", "cpu").Output()
	if err != nil {
		return "", err
	}
	myCPUModel := strings.TrimSuffix(string(output), "\n")
	file, err := os.Open(CPUModelDataPath)
	if err != nil {
		return "", err
	}
	reader := csv.NewReader(file)

	dec, err := csvutil.NewDecoder(reader)
	if err != nil {
		return "", err
	}

	for {
		var p CPUModelData
		if err := dec.Decode(&p); err == io.EOF {
			break
		}
		if strings.HasPrefix(myCPUModel, p.Name) {
			return p.Architecture, nil
		}
	}

	return "", fmt.Errorf("no CPU power model found for architecture %s", myCPUModel)
}

type NodeMetrics struct {
	ResourceUsage map[string]float64

	TotalEnergyInCore     *UInt64StatCollection
	TotalEnergyInDRAM     *UInt64StatCollection
	TotalEnergyInUncore   *UInt64StatCollection
	TotalEnergyInPkg      *UInt64StatCollection
	TotalEnergyInGPU      *UInt64StatCollection
	TotalEnergyInOther    *UInt64StatCollection
	TotalEnergyInPlatform *UInt64StatCollection

	DynEnergyInCore     *UInt64StatCollection
	DynEnergyInDRAM     *UInt64StatCollection
	DynEnergyInUncore   *UInt64StatCollection
	DynEnergyInPkg      *UInt64StatCollection
	DynEnergyInGPU      *UInt64StatCollection
	DynEnergyInOther    *UInt64StatCollection
	DynEnergyInPlatform *UInt64StatCollection

	IdleEnergyInCore     *UInt64StatCollection
	IdleEnergyInDRAM     *UInt64StatCollection
	IdleEnergyInUncore   *UInt64StatCollection
	IdleEnergyInPkg      *UInt64StatCollection
	IdleEnergyInGPU      *UInt64StatCollection
	IdleEnergyInOther    *UInt64StatCollection
	IdleEnergyInPlatform *UInt64StatCollection

	CPUFrequency map[int32]uint64

	// IdleCPUUtilization is used to determine idle periods
	IdleCPUUtilization uint64
	FoundNewIdleState  bool
}

func NewNodeMetrics() *NodeMetrics {
	return &NodeMetrics{
		TotalEnergyInCore: &UInt64StatCollection{
			Stat: make(map[string]*UInt64Stat),
		},
		TotalEnergyInDRAM: &UInt64StatCollection{
			Stat: make(map[string]*UInt64Stat),
		},
		TotalEnergyInUncore: &UInt64StatCollection{
			Stat: make(map[string]*UInt64Stat),
		},
		TotalEnergyInPkg: &UInt64StatCollection{
			Stat: make(map[string]*UInt64Stat),
		},
		TotalEnergyInGPU: &UInt64StatCollection{
			Stat: make(map[string]*UInt64Stat),
		},
		TotalEnergyInOther: &UInt64StatCollection{
			Stat: make(map[string]*UInt64Stat),
		},
		TotalEnergyInPlatform: &UInt64StatCollection{
			Stat: make(map[string]*UInt64Stat),
		},

		DynEnergyInCore: &UInt64StatCollection{
			Stat: make(map[string]*UInt64Stat),
		},
		DynEnergyInDRAM: &UInt64StatCollection{
			Stat: make(map[string]*UInt64Stat),
		},
		DynEnergyInUncore: &UInt64StatCollection{
			Stat: make(map[string]*UInt64Stat),
		},
		DynEnergyInPkg: &UInt64StatCollection{
			Stat: make(map[string]*UInt64Stat),
		},
		DynEnergyInGPU: &UInt64StatCollection{
			Stat: make(map[string]*UInt64Stat),
		},
		DynEnergyInOther: &UInt64StatCollection{
			Stat: make(map[string]*UInt64Stat),
		},
		DynEnergyInPlatform: &UInt64StatCollection{
			Stat: make(map[string]*UInt64Stat),
		},

		IdleEnergyInCore: &UInt64StatCollection{
			Stat: make(map[string]*UInt64Stat),
		},
		IdleEnergyInDRAM: &UInt64StatCollection{
			Stat: make(map[string]*UInt64Stat),
		},
		IdleEnergyInUncore: &UInt64StatCollection{
			Stat: make(map[string]*UInt64Stat),
		},
		IdleEnergyInPkg: &UInt64StatCollection{
			Stat: make(map[string]*UInt64Stat),
		},
		IdleEnergyInGPU: &UInt64StatCollection{
			Stat: make(map[string]*UInt64Stat),
		},
		IdleEnergyInOther: &UInt64StatCollection{
			Stat: make(map[string]*UInt64Stat),
		},
		IdleEnergyInPlatform: &UInt64StatCollection{
			Stat: make(map[string]*UInt64Stat),
		},
	}
}

func (ne *NodeMetrics) ResetDeltaValues() {
	ne.ResourceUsage = make(map[string]float64)
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
	ne.DynEnergyInGPU.ResetDeltaValues()
	ne.DynEnergyInPlatform.ResetDeltaValues()
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
func (ne *NodeMetrics) SetLastestPlatformEnergy(platformEnergy map[string]float64) {
	for sensorID, energy := range platformEnergy {
		ne.TotalEnergyInPlatform.SetDeltaStat(sensorID, uint64(math.Ceil(energy)))
	}
}

// SetNodeComponentsEnergy adds the lastest energy consumption collected from the node's components (e.g., using RAPL)
func (ne *NodeMetrics) SetNodeComponentsEnergy(componentsEnergy map[int]source.NodeComponentsEnergy) {
	for pkgID, energy := range componentsEnergy {
		key := strconv.Itoa(pkgID)
		ne.TotalEnergyInCore.SetAggrStat(key, energy.Core)
		ne.TotalEnergyInDRAM.SetAggrStat(key, energy.DRAM)
		ne.TotalEnergyInUncore.SetAggrStat(key, energy.Uncore)
		ne.TotalEnergyInPkg.SetAggrStat(key, energy.Pkg)
	}
}

// AddNodeGPUEnergy adds the lastest energy consumption of each GPU power consumption.
// Right now we don't support other types of accelerators than GPU, but we will in the future.
func (ne *NodeMetrics) AddNodeGPUEnergy(gpuEnergy []uint32) {
	for gpuID, energy := range gpuEnergy {
		key := strconv.Itoa(gpuID)
		ne.TotalEnergyInGPU.AddDeltaStat(key, uint64(energy))
	}
}

func (ne *NodeMetrics) UpdateIdleEnergy() {
	ne.CalcIdleEnergy(CORE)
	ne.CalcIdleEnergy(DRAM)
	ne.CalcIdleEnergy(UNCORE)
	ne.CalcIdleEnergy(PKG)
	ne.CalcIdleEnergy(GPU)
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

func (ne *NodeMetrics) GetNodeResUsagePerResType(resource string) float64 {
	return ne.ResourceUsage[resource]
}

func (ne *NodeMetrics) String() string {
	return fmt.Sprintf("node energy (mJ): \n"+
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

func (ne *NodeMetrics) getTotalEnergyStatCollection(component string) (energyStat *UInt64StatCollection) {
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

func (ne *NodeMetrics) getDynEnergyStatCollection(component string) (energyStat *UInt64StatCollection) {
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

func (ne *NodeMetrics) getIdleEnergyStatCollection(component string) (energyStat *UInt64StatCollection) {
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
