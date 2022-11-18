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

	"github.com/sustainable-computing-io/kepler/pkg/power/components/source"
)

const (
	CORE   = "core"
	DRAM   = "dram"
	UNCORE = "uncore"
	PKG    = "pkg"
	GPU    = "gpu"
	OTHER  = "other"
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
	cpuArchOverride := os.Getenv("CPU_ARCH_OVERRIDE")
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
	ResourceUsage    map[string]float64
	EnergyInCore     *UInt64StatCollection
	EnergyInDRAM     *UInt64StatCollection
	EnergyInUncore   *UInt64StatCollection
	EnergyInPkg      *UInt64StatCollection
	EnergyInGPU      *UInt64StatCollection
	EnergyInOther    *UInt64StatCollection
	EnergyInPlatform *UInt64StatCollection
}

func NewNodeMetrics() *NodeMetrics {
	return &NodeMetrics{
		EnergyInCore: &UInt64StatCollection{
			Stat: make(map[string]*UInt64Stat),
		},
		EnergyInDRAM: &UInt64StatCollection{
			Stat: make(map[string]*UInt64Stat),
		},
		EnergyInUncore: &UInt64StatCollection{
			Stat: make(map[string]*UInt64Stat),
		},
		EnergyInPkg: &UInt64StatCollection{
			Stat: make(map[string]*UInt64Stat),
		},
		EnergyInGPU: &UInt64StatCollection{
			Stat: make(map[string]*UInt64Stat),
		},
		EnergyInOther: &UInt64StatCollection{
			Stat: make(map[string]*UInt64Stat),
		},
		EnergyInPlatform: &UInt64StatCollection{
			Stat: make(map[string]*UInt64Stat),
		},
	}
}

func (ne *NodeMetrics) ResetCurr() {
	ne.ResourceUsage = make(map[string]float64)
	ne.EnergyInCore.ResetCurr()
	ne.EnergyInDRAM.ResetCurr()
	ne.EnergyInUncore.ResetCurr()
	ne.EnergyInPkg.ResetCurr()
	ne.EnergyInGPU.ResetCurr()
	ne.EnergyInOther.ResetCurr()
	ne.EnergyInPlatform.ResetCurr()
}

// AddNodeResResourceUsageFromContainerResResourceUsage adds the sum of all container resource usage as the node resource usage
func (ne *NodeMetrics) AddNodeResUsageFromContainerResUsage(containersMetrics map[string]*ContainerMetrics) {
	nodeResourceUsage := make(map[string]float64)
	for _, metricName := range ContainerMetricNames {
		nodeResourceUsage[metricName] = 0
		for _, container := range containersMetrics {
			// TODO: refactor the extractUIntCurrAggr, this is not an intuitive function name
			curr, _, _ := container.extractUIntCurrAggr(metricName)
			nodeResourceUsage[metricName] += float64(curr)
		}
	}
	ne.ResourceUsage = nodeResourceUsage
}

// AddLastestPlatformEnergy adds the lastest energy consumption from the node sensor
func (ne *NodeMetrics) AddLastestPlatformEnergy(platformEnergy map[string]float64) {
	for sensorID, energy := range platformEnergy {
		ne.EnergyInPlatform.AddAggrStat(sensorID, uint64(math.Ceil(energy)))
	}
}

// AddNodeComponentsEnergy adds the lastest energy consumption collected from the node's components (e.g., using RAPL)
func (ne *NodeMetrics) AddNodeComponentsEnergy(componentsEnergy map[int]source.NodeComponentsEnergy) {
	for pkgID, energy := range componentsEnergy {
		key := strconv.Itoa(pkgID)
		ne.EnergyInCore.AddAggrStat(key, energy.Core)
		ne.EnergyInDRAM.AddAggrStat(key, energy.DRAM)
		ne.EnergyInUncore.AddAggrStat(key, energy.Uncore)
		ne.EnergyInPkg.AddAggrStat(key, energy.Pkg)
	}
}

// AddNodeGPUEnergy adds the lastest energy consumption of each GPU power consumption.
// Right now we don't support other types of accelerators than GPU, but we will in the future.
func (ne *NodeMetrics) AddNodeGPUEnergy(gpuEnergy []uint32) {
	for gpuID, energy := range gpuEnergy {
		key := strconv.Itoa(gpuID)
		ne.EnergyInGPU.AddCurrStat(key, uint64(energy))
	}
}

func (ne *NodeMetrics) GetEnergyValue(ekey string) (val uint64) {
	switch ekey {
	case CORE:
		val = ne.EnergyInCore.Curr()
	case DRAM:
		val = ne.EnergyInDRAM.Curr()
	case UNCORE:
		val = ne.EnergyInUncore.Curr()
	case PKG:
		val = ne.EnergyInPkg.Curr()
	case GPU:
		val = ne.EnergyInGPU.Curr()
	case OTHER:
		val = ne.EnergyInOther.Curr()
	}
	return
}

// TODO: fix me: we shouldn't use the total node power as Pkg and not Pkg as Core power
// See https://github.com/sustainable-computing-io/kepler/issues/295
func (ne *NodeMetrics) GetNodeTotalEnergyPerComponent() source.NodeComponentsEnergy {
	coreValue := ne.EnergyInCore.Curr()
	uncoreValue := ne.EnergyInUncore.Curr()
	pkgValue := ne.EnergyInPkg.Curr()
	if pkgValue == 0 {
		// if RAPL is not available, but the node power from sensor is available
		pkgValue = ne.EnergyInOther.Curr()
	}
	if coreValue == 0 {
		coreValue = pkgValue - uncoreValue
	}
	return source.NodeComponentsEnergy{
		Core:   coreValue,
		Uncore: uncoreValue,
		Pkg:    pkgValue,
		DRAM:   ne.EnergyInDRAM.Curr(),
	}
}

func (ne *NodeMetrics) GetNodeTotalEnergy() (totalPower uint64) {
	return ne.EnergyInPkg.Curr() +
		ne.EnergyInDRAM.Curr() +
		ne.EnergyInGPU.Curr() +
		ne.EnergyInOther.Curr()
}

func (ne *NodeMetrics) GetNodeResUsagePerResType(resource string) float64 {
	return ne.ResourceUsage[resource]
}

func (ne *NodeMetrics) String() string {
	return fmt.Sprintf("node energy (mJ): \n"+
		"\tePkg: %d (eCore: %d eDram: %d eUncore: %d) eGPU: %d eOther: %d \n",
		ne.EnergyInPkg.Curr(), ne.EnergyInCore.Curr(), ne.EnergyInDRAM.Curr(), ne.EnergyInUncore.Curr(), ne.EnergyInGPU.Curr(), ne.EnergyInOther.Curr())
}
