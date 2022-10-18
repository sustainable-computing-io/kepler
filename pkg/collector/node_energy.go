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
	"encoding/csv"
	"fmt"
	"io"
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
	cpuModelDataPath = "/var/lib/kepler/data/normalized_cpu_arch.csv"

	nodeName, _ = os.Hostname()
	cpuArch     = getCPUArch()
)

type CPUModelData struct {
	Name         string `csv:"Name"`
	Architecture string `csv:"Architecture"`
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
	file, err := os.Open(cpuModelDataPath)
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

func getCPUArch() string {
	arch, err := getCPUArchitecture()
	if err == nil {
		return arch
	}
	return "unknown"
}

type NodeEnergy struct {
	Usage          map[string]float64
	EnergyInCore   *UInt64StatCollection
	EnergyInDRAM   *UInt64StatCollection
	EnergyInUncore *UInt64StatCollection
	EnergyInPkg    *UInt64StatCollection
	EnergyInGPU    uint64
	EnergyInOther  uint64
	SensorEnergy   *UInt64StatCollection
}

func NewNodeEnergy() *NodeEnergy {
	return &NodeEnergy{
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
		SensorEnergy: &UInt64StatCollection{
			Stat: make(map[string]*UInt64Stat),
		},
	}
}

func (v *NodeEnergy) ResetCurr() {
	v.Usage = make(map[string]float64)
	v.EnergyInCore.ResetCurr()
	v.EnergyInDRAM.ResetCurr()
	v.EnergyInUncore.ResetCurr()
	v.EnergyInPkg.ResetCurr()
	v.EnergyInGPU = uint64(0)
	v.EnergyInOther = uint64(0)
	v.SensorEnergy.ResetCurr()
}

func (v NodeEnergy) sumUsage(podMetricValues [][]float64) (nodeUsageValues []float64, nodeUsageMap map[string]float64) {
	nodeUsageValues = make([]float64, len(metricNames))
	nodeUsageMap = make(map[string]float64)
	podNumber := len(podMetricValues)
	for metricIndex, metric := range metricNames {
		nodeUsageMap[metric] = 0
		for podIndex := 0; podIndex < podNumber; podIndex++ {
			nodeUsageMap[metric] += podMetricValues[podIndex][metricIndex]
			nodeUsageValues[metricIndex] += podMetricValues[podIndex][metricIndex]
		}
	}
	return
}

func (v *NodeEnergy) SetValues(sensorEnergy map[string]float64, pkgEnergy map[int]source.RAPLEnergy, totalGPUDelta uint64, podMetricValues [][]float64) {
	nodeUsageValues, nodeUsageMap := v.sumUsage(podMetricValues)
	v.Usage = nodeUsageMap
	for sensorID, energy := range sensorEnergy {
		v.SensorEnergy.AddAggrStat(sensorID, uint64(energy))
	}
	if len(sensorEnergy) == 0 && model.NodeTotalPowerModelValid {
		// no energy sensor
		valid, estimatedTotalPower := model.GetNodeTotalPower(nodeUsageValues, systemValues)
		if valid {
			v.SensorEnergy.AddCurrStat(estimatorID, estimatedTotalPower)
		}
	}
	for pkgID, energy := range pkgEnergy {
		key := strconv.Itoa(pkgID)
		v.EnergyInCore.AddAggrStat(key, energy.Core)
		v.EnergyInDRAM.AddAggrStat(key, energy.DRAM)
		v.EnergyInUncore.AddAggrStat(key, energy.Uncore)
		v.EnergyInPkg.AddAggrStat(key, energy.Pkg)
	}
	if len(pkgEnergy) == 0 && model.NodeComponentPowerModelValid {
		// no RAPL
		valid, estimatedComponentPower := model.GetNodeComponentPowers(nodeUsageValues, systemValues)
		if valid {
			v.EnergyInCore.AddCurrStat(estimatorPkgKey, estimatedComponentPower.Core)
			v.EnergyInDRAM.AddCurrStat(estimatorPkgKey, estimatedComponentPower.DRAM)
			v.EnergyInUncore.AddCurrStat(estimatorPkgKey, estimatedComponentPower.Uncore)
			v.EnergyInPkg.AddCurrStat(estimatorPkgKey, estimatedComponentPower.Pkg)
		}
	}
	v.EnergyInGPU = totalGPUDelta
	klog.V(3).Infof("node energy stat core %v dram %v uncore %v pkg %v gpu %v sensor %v\n", v.EnergyInCore, v.EnergyInDRAM, v.EnergyInUncore, v.EnergyInPkg, v.EnergyInGPU, v.SensorEnergy)
	totalSensorDelta := v.SensorEnergy.Curr()
	totalPkgDelta := v.EnergyInPkg.Curr()
	totalDramDelta := v.EnergyInDRAM.Curr()
	if totalSensorDelta > (totalPkgDelta + totalGPUDelta) {
		v.EnergyInOther = totalSensorDelta - (totalPkgDelta + totalGPUDelta + totalDramDelta)
	}
}

func (v *NodeEnergy) ToPrometheusValues() []string {
	nodeValues := []string{nodeName, cpuArch}
	for _, metric := range metricNames {
		nodeValues = append(nodeValues, strconv.FormatUint(uint64(v.Usage[metric]), 10))
	}
	for _, ekey := range EnergyLabelsKeys {
		val := float64(v.GetPrometheusEnergyValue(ekey)) / 1000.0 // Joule
		nodeValues = append(nodeValues, fmt.Sprintf("%f", val))
	}
	return nodeValues
}

func (v *NodeEnergy) getEnergyValue(ekey string) (val uint64) {
	switch ekey {
	case "core":
		val = v.EnergyInCore.Curr()
	case "dram":
		val = v.EnergyInDRAM.Curr()
	case "uncore":
		val = v.EnergyInUncore.Curr()
	case "pkg":
		val = v.EnergyInPkg.Curr()
	case "gpu":
		val = v.EnergyInGPU
	case "other":
		val = v.EnergyInOther
	}
	return
}

func (v *NodeEnergy) GetPrometheusEnergyValue(ekey string) (val uint64) {
	return v.getEnergyValue(ekey)
}

func (v *NodeEnergy) Curr() uint64 {
	return v.EnergyInPkg.Curr() + v.EnergyInGPU + v.EnergyInOther
}

func (v *NodeEnergy) GetCurrEnergyPerpkgID(pkgIDKey string) (coreDelta, dramDelta, uncoreDelta uint64) {
	coreDelta = v.EnergyInCore.Stat[pkgIDKey].Curr
	dramDelta = v.EnergyInDRAM.Stat[pkgIDKey].Curr
	uncoreDelta = v.EnergyInUncore.Stat[pkgIDKey].Curr
	return
}

func (v *NodeEnergy) GetNodeComponentPower() source.RAPLPower {
	coreValue := v.EnergyInCore.Curr()
	uncoreValue := v.EnergyInUncore.Curr()
	pkgValue := v.EnergyInPkg.Curr()
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
		DRAM:   v.EnergyInDRAM.Curr(),
	}
}

func (v NodeEnergy) String() string {
	return fmt.Sprintf("node energy (mJ): \n"+
		"\tePkg: %d (eCore: %d eDram: %d eUncore: %d) eGPU: %d eOther: %d \n",
		v.EnergyInPkg.Curr(), v.EnergyInCore.Curr(), v.EnergyInDRAM.Curr(), v.EnergyInUncore.Curr(), v.EnergyInGPU, v.EnergyInOther)
}
