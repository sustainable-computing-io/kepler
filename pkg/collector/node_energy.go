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

type CurrNodeEnergy struct {
	Usage          map[string]float64
	EnergyInCore   uint64
	EnergyInDRAM   uint64
	EnergyInUncore uint64
	EnergyInPkg    uint64
	EnergyInGPU    uint64
	EnergyInOther  uint64
	SensorEnergy   uint64
}

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

func (v CurrNodeEnergy) ToPrometheusValues() []string {
	nodeValues := []string{nodeName, cpuArch}
	for _, metric := range metricNames {
		nodeValues = append(nodeValues, strconv.FormatUint(uint64(currNodeEnergy.Usage[metric]), 10))
	}
	for ekey, _ := range ENERGY_LABELS {
		val := float64(v.GetPrometheusEnergyValue(ekey)) / 1000.0 // Joule
		nodeValues = append(nodeValues, fmt.Sprintf("%f", val))
	}
	return nodeValues
}

func (v CurrNodeEnergy) String() string {
	return fmt.Sprintf("node energy (mJ): \n"+
		"\tePkg: %d (eCore: %d eDram: %d eUncore: %d) eGPU: %d eOther: %d \n",
		v.EnergyInPkg, v.EnergyInCore, v.EnergyInDRAM, v.EnergyInUncore, v.EnergyInGPU, v.EnergyInOther)
}

func (v CurrNodeEnergy) GetPrometheusEnergyValue(ekey string) (val uint64) {
	switch ekey {
	case "core":
		val = v.EnergyInCore
	case "dram":
		val = v.EnergyInDRAM
	case "uncore":
		val = v.EnergyInUncore
	case "pkg":
		val = v.EnergyInPkg
	case "gpu":
		val = v.EnergyInGPU
	case "other":
		val = v.EnergyInOther
	}
	return
}
