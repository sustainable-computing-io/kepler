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

	"github.com/sustainable-computing-io/kepler/pkg/model"
	"github.com/sustainable-computing-io/kepler/pkg/power/rapl/source"
)

const (
	estimatorID     string = "estimator"
	estimatorPkgKey string = "0"
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
	Usage          map[string]float64
	EnergyInCore   *UInt64StatCollection
	EnergyInDRAM   *UInt64StatCollection
	EnergyInUncore *UInt64StatCollection
	EnergyInPkg    *UInt64StatCollection
	EnergyInGPU    *UInt64StatCollection
	EnergyInOther  *UInt64StatCollection
	SensorEnergy   *UInt64StatCollection
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
		SensorEnergy: &UInt64StatCollection{
			Stat: make(map[string]*UInt64Stat),
		},
	}
}

func (ne *NodeMetrics) ResetCurr() {
	ne.Usage = make(map[string]float64)
	ne.EnergyInCore.ResetCurr()
	ne.EnergyInDRAM.ResetCurr()
	ne.EnergyInUncore.ResetCurr()
	ne.EnergyInPkg.ResetCurr()
	ne.EnergyInGPU.ResetCurr()
	ne.EnergyInOther.ResetCurr()
	ne.SensorEnergy.ResetCurr()
}

func (ne NodeMetrics) sumUsage(podMetricValues [][]float64) (nodeUsageValues []float64, nodeUsageMap map[string]float64) {
	nodeUsageValues = make([]float64, len(NodeMetadataNames))
	nodeUsageMap = make(map[string]float64)
	podNumber := len(podMetricValues)
	for metricIndex, metric := range NodeMetadataNames {
		nodeUsageMap[metric] = 0
		for podIndex := 0; podIndex < podNumber; podIndex++ {
			nodeUsageMap[metric] += podMetricValues[podIndex][metricIndex]
			nodeUsageValues[metricIndex] += podMetricValues[podIndex][metricIndex]
		}
	}
	return
}

func (ne *NodeMetrics) addMeasuredSensorData(sensorEnergy map[string]float64) {
	for sensorID, energy := range sensorEnergy {
		ne.SensorEnergy.AddAggrStat(sensorID, uint64(math.Ceil(energy)))
	}
}

// if could not collect host energy sensor metric
func (ne *NodeMetrics) addEstimatedSensorData(nodeUsageValues []float64) {
	if model.NodeTotalPowerModelValid {
		if valid, estimatedTotalPower := model.GetNodeTotalPower(nodeUsageValues, NodeMetadataValues); valid {
			ne.SensorEnergy.AddCurrStat(estimatorID, estimatedTotalPower)
		}
	}
}

func (ne *NodeMetrics) addMeasuredRAPLData(pkgEnergy map[int]source.RAPLEnergy) {
	for pkgID, energy := range pkgEnergy {
		key := strconv.Itoa(pkgID)
		ne.EnergyInCore.AddAggrStat(key, energy.Core)
		ne.EnergyInDRAM.AddAggrStat(key, energy.DRAM)
		ne.EnergyInUncore.AddAggrStat(key, energy.Uncore)
		ne.EnergyInPkg.AddAggrStat(key, energy.Pkg)
	}
}

// if could not collect RAPL metrics
func (ne *NodeMetrics) addEstimatedRAPLData(nodeUsageValues []float64) {
	if model.NodeComponentPowerModelValid {
		if valid, estimatedComponentPower := model.GetNodeComponentPowers(nodeUsageValues, NodeMetadataValues); valid {
			ne.EnergyInCore.AddCurrStat(estimatorPkgKey, estimatedComponentPower.Core)
			ne.EnergyInDRAM.AddCurrStat(estimatorPkgKey, estimatedComponentPower.DRAM)
			ne.EnergyInUncore.AddCurrStat(estimatorPkgKey, estimatedComponentPower.Uncore)
			ne.EnergyInPkg.AddCurrStat(estimatorPkgKey, estimatedComponentPower.Pkg)
		}
	}
}

func (ne *NodeMetrics) SetValues(sensorEnergy map[string]float64, pkgEnergy map[int]source.RAPLEnergy, gpuEnergy []uint32, containerMetricValues [][]float64) {
	nodeUsageValues, nodeUsageMap := ne.sumUsage(containerMetricValues)
	ne.Usage = nodeUsageMap

	// adding total host energy consumption
	if len(sensorEnergy) > 0 {
		ne.addMeasuredSensorData(sensorEnergy)
	} else {
		ne.addEstimatedSensorData(nodeUsageValues)
	}

	// adding each host component energy consumption
	if len(pkgEnergy) > 0 {
		ne.addMeasuredRAPLData(pkgEnergy)
	} else {
		ne.addEstimatedRAPLData(nodeUsageValues)
	}

	// adding the host gpu energy consumption
	for gpuID, energy := range gpuEnergy {
		key := strconv.Itoa(gpuID)
		ne.EnergyInGPU.AddCurrStat(key, uint64(energy))
	}

	klog.V(3).Infof("node energy stat core %v dram %v uncore %v pkg %v gpu %v sensor %v\n", ne.EnergyInCore, ne.EnergyInDRAM, ne.EnergyInUncore, ne.EnergyInPkg, ne.EnergyInGPU, ne.SensorEnergy)
	totalSensorDelta := ne.SensorEnergy.Curr()
	totalPkgDelta := ne.EnergyInPkg.Curr()
	totalDramDelta := ne.EnergyInDRAM.Curr()
	totalGPUDelta := ne.EnergyInGPU.Curr()
	if totalSensorDelta > (totalPkgDelta + totalDramDelta + totalGPUDelta) {
		energyOtherComponents := totalSensorDelta - (totalPkgDelta + totalDramDelta + totalGPUDelta)
		key := strconv.Itoa(len(ne.EnergyInOther.Stat))
		ne.EnergyInOther.AddCurrStat(key, energyOtherComponents)
	}
}

func (ne *NodeMetrics) getEnergyValue(ekey string) (val uint64) {
	switch ekey {
	case "core":
		val = ne.EnergyInCore.Curr()
	case "dram":
		val = ne.EnergyInDRAM.Curr()
	case "uncore":
		val = ne.EnergyInUncore.Curr()
	case "pkg":
		val = ne.EnergyInPkg.Curr()
	case "gpu":
		val = ne.EnergyInGPU.Curr()
	case "other":
		val = ne.EnergyInOther.Curr()
	}
	return
}

func (ne *NodeMetrics) GetPrometheusEnergyValue(ekey string) (val uint64) {
	return ne.getEnergyValue(ekey)
}

func (ne *NodeMetrics) GetNodeComponentPower() source.RAPLPower {
	coreValue := ne.EnergyInCore.Curr()
	uncoreValue := ne.EnergyInUncore.Curr()
	pkgValue := ne.EnergyInPkg.Curr()
	if pkgValue == 0 {
		pkgValue = coreValue + uncoreValue
	}
	if coreValue == 0 {
		coreValue = pkgValue - uncoreValue
	}
	return source.RAPLPower{
		Core:   coreValue,
		Uncore: uncoreValue,
		Pkg:    pkgValue,
		DRAM:   ne.EnergyInDRAM.Curr(),
	}
}

func (ne *NodeMetrics) String() string {
	return fmt.Sprintf("node energy (mJ): \n"+
		"\tePkg: %d (eCore: %d eDram: %d eUncore: %d) eGPU: %d eOther: %d \n",
		ne.EnergyInPkg.Curr(), ne.EnergyInCore.Curr(), ne.EnergyInDRAM.Curr(), ne.EnergyInUncore.Curr(), ne.EnergyInGPU.Curr(), ne.EnergyInOther.Curr())
}
