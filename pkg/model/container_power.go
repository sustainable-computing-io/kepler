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

//nolint:dupl // refactor this
package model

import (
	"sync"

	collector_metric "github.com/sustainable-computing-io/kepler/pkg/collector/metric"
	"github.com/sustainable-computing-io/kepler/pkg/config"
	"github.com/sustainable-computing-io/kepler/pkg/model/estimator/local"
	"github.com/sustainable-computing-io/kepler/pkg/model/types"
	"github.com/sustainable-computing-io/kepler/pkg/power/accelerator"
	"github.com/sustainable-computing-io/kepler/pkg/power/components"
	"github.com/sustainable-computing-io/kepler/pkg/power/components/source"
	"k8s.io/klog/v2"
)

var (
	ContainerTotalPowerModelValid, ContainerComponentPowerModelValid bool
	ContainerTotalPowerModelFunc                                     func([][]float64, []string) ([]float64, error)
	ContainerComponentPowerModelFunc                                 func([][]float64, []string) (map[string][]float64, error)

	// cgroupOnly
	defaultDynCompURL = "/var/lib/kepler/data/ScikitMixed.json"
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

// UpdateContainerEnergy returns container energy consumption for each node component
func UpdateContainerEnergy(containersMetrics map[string]*collector_metric.ContainerMetrics, nodeMetrics *collector_metric.NodeMetrics) {
	var wg sync.WaitGroup
	// If the node can expose power measurement per component, we can use the RATIO power model
	// Otherwise, we estimate it from trained power model
	if components.IsSystemCollectionSupported() {
		wg.Add(6)
		go local.UpdateContainerComponentEnergyByRatioPowerModel(containersMetrics, nodeMetrics, collector_metric.PKG, config.CoreUsageMetric, &wg)
		go local.UpdateContainerComponentEnergyByRatioPowerModel(containersMetrics, nodeMetrics, collector_metric.CORE, config.CoreUsageMetric, &wg)
		go local.UpdateContainerComponentEnergyByRatioPowerModel(containersMetrics, nodeMetrics, collector_metric.DRAM, config.DRAMUsageMetric, &wg)
		go local.UpdateContainerComponentEnergyByRatioPowerModel(containersMetrics, nodeMetrics, collector_metric.PLATFORM, config.CoreUsageMetric, &wg)
		// If the resource usage metrics is empty, we evenly divide the power consumption of the resource across all containers
		go local.UpdateContainerComponentEnergyByRatioPowerModel(containersMetrics, nodeMetrics, collector_metric.UNCORE, "", &wg)
		go local.UpdateContainerComponentEnergyByRatioPowerModel(containersMetrics, nodeMetrics, collector_metric.OTHER, "", &wg)
	} else {
		// The estimator power model updates the power consumption of Pkg, Core, Dram, Uncore and Other
		UpdateContainerEnergyByTrainedPowerModel(containersMetrics)
	}

	// Currently, we do not have a power model that can forecast GPU power in the absence of real-time power metrics.
	// Generally, if we can obtain GPU metrics, we can also acquire GPU power metrics.
	if accelerator.IsGPUCollectionSupported() {
		wg.Add(1)
		go local.UpdateContainerComponentEnergyByRatioPowerModel(containersMetrics, nodeMetrics, collector_metric.GPU, config.GpuUsageMetric, &wg)
	}
	wg.Wait()
}

func UpdateContainerEnergyByTrainedPowerModel(containersMetrics map[string]*collector_metric.ContainerMetrics) {
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
		if err := containersMetrics[containerID].DynEnergyInCore.AddNewDelta(containerComponentPowers[i].Core); err != nil {
			klog.V(5).Infoln(err)
		}
		if err := containersMetrics[containerID].DynEnergyInDRAM.AddNewDelta(containerComponentPowers[i].DRAM); err != nil {
			klog.V(5).Infoln(err)
		}
		if err := containersMetrics[containerID].DynEnergyInUncore.AddNewDelta(containerComponentPowers[i].Uncore); err != nil {
			klog.V(5).Infoln(err)
		}
		if err := containersMetrics[containerID].DynEnergyInPkg.AddNewDelta(containerComponentPowers[i].Pkg); err != nil {
			klog.V(5).Infoln(err)
		}
		if err := containersMetrics[containerID].DynEnergyInOther.AddNewDelta(containerOtherPowers[i]); err != nil {
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

// getContainerComponentPowers returns estimated pods' RAPL power
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
