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

package model

import (
	"github.com/sustainable-computing-io/kepler/pkg/model/estimator/local"
	"github.com/sustainable-computing-io/kepler/pkg/model/types"
	"github.com/sustainable-computing-io/kepler/pkg/power/rapl/source"
	"k8s.io/klog/v2"
)

var (
	PodTotalPowerModelValid, PodComponentPowerModelValid bool
	PodTotalPowerModelFunc                               func([][]float64, []string) ([]float64, error)
	PodComponentPowerModelFunc                           func([][]float64, []string) (map[string][]float64, error)

	// TODO: be configured by config package
	// cgroupOnly
	dynCompURL                                     = "https://raw.githubusercontent.com/sustainable-computing-io/kepler-model-server/main/tests/test_models/DynComponentModelWeight/CgroupOnly/ScikitMixed/ScikitMixed.json"
	PodTotalPowerModelConfig     types.ModelConfig = types.ModelConfig{UseEstimatorSidecar: false}
	PodComponentPowerModelConfig types.ModelConfig = types.ModelConfig{UseEstimatorSidecar: false, InitModelURL: dynCompURL}
)

func InitPodPowerEstimator(usageMetrics, systemFeatures, systemValues []string) {
	var estimateFunc interface{}
	// init func for PodTotalPower
	PodTotalPowerModelValid, estimateFunc = initEstimateFunction(PodTotalPowerModelConfig, types.DynPower, types.DynModelWeight, usageMetrics, systemFeatures, systemValues, true)
	if PodTotalPowerModelValid {
		PodTotalPowerModelFunc = estimateFunc.(func([][]float64, []string) ([]float64, error))
	}
	// init func for PodComponentPower
	PodComponentPowerModelValid, estimateFunc = initEstimateFunction(PodComponentPowerModelConfig, types.DynComponentPower, types.DynComponentModelWeight, usageMetrics, systemFeatures, systemValues, false)
	if PodComponentPowerModelValid {
		PodComponentPowerModelFunc = estimateFunc.(func([][]float64, []string) (map[string][]float64, error))
	}
}

// GetContainerPower returns pods' RAPL power and other power
func GetContainerPower(usageValues [][]float64, systemValues []string, nodeTotalPower, nodeTotalGPUPower uint64, nodeTotalPowerPerComponents source.RAPLPower) (componentPodPowers []source.RAPLPower, otherPodPowers []uint64) {
	componentPodPowers = make([]source.RAPLPower, len(usageValues))
	otherPodPowers = make([]uint64, len(usageValues))
	if len(usageValues) == 0 {
		// in some edges case otherPodPowers will happens an out of bounds error
		// directly return to avoid panic
		return
	}

	if nodeTotalPowerPerComponents.Pkg > 0 {
		if nodeTotalPower < nodeTotalPowerPerComponents.Pkg+nodeTotalPowerPerComponents.DRAM+nodeTotalGPUPower {
			// case: NodeTotalPower is invalid but NodeComponentPower model is available, set = pkg+DRAM+GPU
			nodeTotalPower = nodeTotalPowerPerComponents.Pkg + nodeTotalPowerPerComponents.DRAM + nodeTotalGPUPower
		}
	} else if nodeTotalPower > 0 {
		// case: no NodeComponentPower model but NodeTotalPower model is available, set = total-GPU, DRAM=0
		socketPower := nodeTotalPower - nodeTotalGPUPower
		nodeTotalPowerPerComponents = source.RAPLPower{
			Pkg:  socketPower,
			Core: socketPower,
		}
	}

	if nodeTotalPower > 0 {
		// total power all set, use ratio
		nodeOtherPower := nodeTotalPower - nodeTotalPowerPerComponents.Pkg - nodeTotalPowerPerComponents.DRAM - nodeTotalGPUPower
		componentPodPowers, otherPodPowers = local.GetPodPowerRatio(usageValues, nodeOtherPower, nodeTotalPowerPerComponents)
	} else {
		// otherwise, use trained power model
		totalPowerValid, totalPodPowers := getPodTotalPower(usageValues, systemValues)
		var valid bool
		valid, componentPodPowers = getPodComponentPowers(usageValues, systemValues)
		if !valid {
			klog.V(5).Infoln("No PodComponentPower Model")
			return
		}
		otherPodPowers = make([]uint64, len(componentPodPowers))
		if totalPowerValid {
			for index, componentPower := range componentPodPowers {
				// TODO: include GPU into consideration
				otherPodPowers[index] = uint64(totalPodPowers[index]) - componentPower.Pkg - componentPower.DRAM
			}
		}
	}
	return componentPodPowers, otherPodPowers
}

// getPodTotalPower returns estimated pods' total power
func getPodTotalPower(usageValues [][]float64, systemValues []string) (valid bool, results []float64) {
	valid = false
	if PodTotalPowerModelValid {
		powers, err := PodTotalPowerModelFunc(usageValues, systemValues)
		if err != nil || len(powers) == 0 {
			return
		}
		results = powers
		valid = true
		return
	}
	return
}

// getPodTotalPower returns estimated pods' RAPL power
func getPodComponentPowers(usageValues [][]float64, systemValues []string) (bool, []source.RAPLPower) {
	podNumber := len(usageValues)
	if PodComponentPowerModelValid {
		powers, err := PodComponentPowerModelFunc(usageValues, systemValues)
		if err != nil {
			return false, make([]source.RAPLPower, podNumber)
		}
		raplPowers := make([]source.RAPLPower, podNumber)
		for index := 0; index < podNumber; index++ {
			pkgPower := getComponentPower(powers, "pkg", index)
			corePower := getComponentPower(powers, "core", index)
			uncorePower := getComponentPower(powers, "uncore", index)
			dramPower := getComponentPower(powers, "dram", index)
			raplPowers[index] = fillRAPLPower(pkgPower, corePower, uncorePower, dramPower)
		}
		return true, raplPowers
	}
	return false, make([]source.RAPLPower, podNumber)
}
