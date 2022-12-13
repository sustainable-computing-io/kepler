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
	collector_metric "github.com/sustainable-computing-io/kepler/pkg/collector/metric"
	"github.com/sustainable-computing-io/kepler/pkg/config"
	"github.com/sustainable-computing-io/kepler/pkg/model/estimator/local"
	"github.com/sustainable-computing-io/kepler/pkg/model/types"
	"github.com/sustainable-computing-io/kepler/pkg/power/components"
	"github.com/sustainable-computing-io/kepler/pkg/power/components/source"
	"k8s.io/klog/v2"
)

var (
	ContainerTotalPowerModelValid, ContainerComponentPowerModelValid bool
	ContainerTotalPowerModelFunc                                     func([][]float64, []string) ([]float64, error)
	ContainerComponentPowerModelFunc                                 func([][]float64, []string) (map[string][]float64, error)

	// cgroupOnly
	defaultDynCompURL = "https://raw.githubusercontent.com/sustainable-computing-io/kepler-model-server/main/tests/test_models/DynComponentModelWeight/CgroupOnly/ScikitMixed/ScikitMixed.json"
)

// initContainerComponentPowerModelConfig: the container component power model must be set by default.
func initContainerComponentPowerModelConfig() types.ModelConfig {
	modelConfig := InitModelConfig(config.ContainerComponentsKey)
	if modelConfig.InitModelURL == "" {
		modelConfig.InitModelURL = defaultDynCompURL
	}
	return modelConfig
}

func InitContainerPowerEstimator(usageMetrics, systemFeatures, systemValues []string) {
	containerTotalPowerModelConfig := InitModelConfig(config.ContainerTotalKey)
	var estimateFunc interface{}
	// init func for ContainerTotalPower
	ContainerTotalPowerModelValid, estimateFunc = initEstimateFunction(containerTotalPowerModelConfig, types.DynPower, types.DynModelWeight, usageMetrics, systemFeatures, systemValues, true)
	if ContainerTotalPowerModelValid {
		ContainerTotalPowerModelFunc = estimateFunc.(func([][]float64, []string) ([]float64, error))
	}
	containerComponentPowerModelConfig := initContainerComponentPowerModelConfig()
	// init func for ContainerComponentPower
	ContainerComponentPowerModelValid, estimateFunc = initEstimateFunction(containerComponentPowerModelConfig, types.DynComponentPower, types.DynComponentModelWeight, usageMetrics, systemFeatures, systemValues, false)
	if ContainerComponentPowerModelValid {
		ContainerComponentPowerModelFunc = estimateFunc.(func([][]float64, []string) (map[string][]float64, error))
	}
}

// The current implementation from the model server returns a list of the container energy.
// The list follows the order of the container containerMetricValuesOnly for the container id...
// TODO: make model server return a list of elemets that also contains the containerID to enforce consistency
func getContainerMetricsList(containersMetrics map[string]*collector_metric.ContainerMetrics) (containerMetricValuesOnly [][]float64) {
	// convert to pod metrics to array
	for _, c := range containersMetrics {
		values := c.ToEstimatorValues()
		containerMetricValuesOnly = append(containerMetricValuesOnly, values)
	}
	return
}

// UpdateContainerEnergy returns container energy consumption for each node component
func UpdateContainerEnergy(containersMetrics map[string]*collector_metric.ContainerMetrics, nodeMetrics collector_metric.NodeMetrics) {
	// If the node can expose power measurement per component, we can use the RATIO power model
	// Otherwise, we estimate it from trained power model
	if components.IsSystemCollectionSupported() {
		local.UpdateContainerEnergyByRatioPowerModel(containersMetrics, nodeMetrics)
	} else {
		updateContainerEnergyByTrainedPowerModel(containersMetrics)
	}
}

func updateContainerEnergyByTrainedPowerModel(containersMetrics map[string]*collector_metric.ContainerMetrics) {
	var enabled bool
	// convert the container metrics map to an array since the model server does not receive structured data
	// TODO: send data to model server via protobuf instead of no structured data
	containerMetricValuesOnly, containerIDList := containerMetricsToArray(containersMetrics)

	totalPowerValid, totalContainerPowers := getContainerTotalPower(containerMetricValuesOnly)

	enabled, containerComponentPowers := getContainerComponentPowers(containerMetricValuesOnly)
	if !enabled {
		klog.V(5).Infoln("No ContainerComponentPower Model")
		return
	}
	containerOtherPowers := make([]uint64, len(containerComponentPowers))
	if totalPowerValid {
		for index, componentPower := range containerComponentPowers {
			// TODO: include GPU into consideration
			containerOtherPowers[index] = uint64(totalContainerPowers[index]) - componentPower.Pkg - componentPower.DRAM
		}
	}

	// update the container's components energy consumption
	// TODO: the model server does not predict GPU
	for i, containerID := range containerIDList {
		if err := containersMetrics[containerID].EnergyInCore.AddNewCurr(containerComponentPowers[i].Core); err != nil {
			klog.V(5).Infoln(err)
		}
		if err := containersMetrics[containerID].EnergyInDRAM.AddNewCurr(containerComponentPowers[i].DRAM); err != nil {
			klog.V(5).Infoln(err)
		}
		if err := containersMetrics[containerID].EnergyInUncore.AddNewCurr(containerComponentPowers[i].Uncore); err != nil {
			klog.V(5).Infoln(err)
		}
		if err := containersMetrics[containerID].EnergyInPkg.AddNewCurr(containerComponentPowers[i].Pkg); err != nil {
			klog.V(5).Infoln(err)
		}
		if err := containersMetrics[containerID].EnergyInOther.AddNewCurr(containerOtherPowers[i]); err != nil {
			klog.V(5).Infoln(err)
		}
	}
}

// getContainerTotalPower returns estimated pods' total power
func getContainerTotalPower(containerMetricValuesOnly [][]float64) (valid bool, results []float64) {
	valid = false
	if ContainerTotalPowerModelValid {
		powers, err := ContainerTotalPowerModelFunc(containerMetricValuesOnly, collector_metric.NodeMetadataValues)
		if err != nil || len(powers) == 0 {
			return
		}
		results = powers
		valid = true
		return
	}
	return
}

// getContainerTotalPower returns estimated pods' RAPL power
func getContainerComponentPowers(containerMetricValuesOnly [][]float64) (bool, []source.NodeComponentsEnergy) {
	podNumber := len(containerMetricValuesOnly)
	if ContainerComponentPowerModelValid {
		powers, err := ContainerComponentPowerModelFunc(containerMetricValuesOnly, collector_metric.NodeMetadataValues)
		if err != nil {
			return false, make([]source.NodeComponentsEnergy, podNumber)
		}
		raplPowers := make([]source.NodeComponentsEnergy, podNumber)
		for index := 0; index < podNumber; index++ {
			pkgPower := getComponentPower(powers, "pkg", index)
			corePower := getComponentPower(powers, "core", index)
			uncorePower := getComponentPower(powers, "uncore", index)
			dramPower := getComponentPower(powers, "dram", index)
			raplPowers[index] = fillRAPLPower(pkgPower, corePower, uncorePower, dramPower)
		}
		return true, raplPowers
	}
	return false, make([]source.NodeComponentsEnergy, podNumber)
}
