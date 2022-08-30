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
	"fmt"
	"github.com/sustainable-computing-io/kepler/pkg/power/rapl/source"
	"os"
	"strconv"
)

var (
	nodeName, _ = os.Hostname()
	cpuArch     = getCPUArch()
)

func getCPUArch() string {
	arch, err := source.GetCPUArchitecture()
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

func (v *NodeEnergy) SetValues(sensorEnergy map[string]float64, pkgEnergy map[int]source.PackageEnergy, totalGPUDelta uint64, usage map[string]float64) {
	fmt.Printf("%v %v\n", sensorEnergy, pkgEnergy)
	for sensorID, energy := range sensorEnergy {
		v.SensorEnergy.AddStat(sensorID, uint64(energy))
	}
	for pkgID, energy := range pkgEnergy {
		key := strconv.Itoa(pkgID)
		v.EnergyInCore.AddStat(key, energy.Core)
		v.EnergyInDRAM.AddStat(key, energy.DRAM)
		v.EnergyInUncore.AddStat(key, energy.Uncore)
		v.EnergyInPkg.AddStat(key, energy.Pkg)
	}
	fmt.Printf("%v %v\n", v.EnergyInPkg, v.SensorEnergy)
	v.EnergyInGPU = totalGPUDelta
	totalSensorDelta := v.SensorEnergy.Curr()
	totalPkgDelta := v.EnergyInPkg.Curr()
	if totalSensorDelta > (totalPkgDelta + totalGPUDelta) {
		v.EnergyInOther = totalSensorDelta - (totalPkgDelta + totalGPUDelta)
	}
	v.Usage = usage
}

func (v *NodeEnergy) ToPrometheusValues() []string {
	nodeValues := []string{nodeName, cpuArch}
	for _, metric := range metricNames {
		nodeValues = append(nodeValues, strconv.FormatUint(uint64(v.Usage[metric]), 10))
	}
	for ekey, _ := range ENERGY_LABELS {
		val := float64(v.GetPrometheusEnergyValue(ekey)) / 1000.0 // Joule
		nodeValues = append(nodeValues, fmt.Sprintf("%f", val))
	}
	return nodeValues
}

func (v *NodeEnergy) GetPrometheusEnergyValue(ekey string) (val uint64) {
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

func (v *NodeEnergy) Curr() uint64 {
	return v.EnergyInPkg.Curr() + v.EnergyInGPU + v.EnergyInOther
}

func (v NodeEnergy) String() string {
	return fmt.Sprintf("node energy (mJ): \n"+
		"\tePkg: %d (eCore: %d eDram: %d eUncore: %d) eGPU: %d eOther: %d \n",
		v.EnergyInPkg.Curr(), v.EnergyInCore.Curr(), v.EnergyInDRAM.Curr(), v.EnergyInUncore.Curr(), v.EnergyInGPU, v.EnergyInOther)
}
