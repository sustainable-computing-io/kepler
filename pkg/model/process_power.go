/*
Copyright 2023.

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
	ProcessTotalPowerModelValid, ProcessComponentPowerModelValid bool
	ProcessTotalPowerModelFunc                                   func([][]float64, []string) ([]float64, error)
	ProcessComponentPowerModelFunc                               func([][]float64, []string) (map[string][]float64, error)

	// counterOnly
	defaultCounterDynCompURL = "https://raw.githubusercontent.com/sustainable-computing-io/kepler-model-server/main/tests/test_models/DynComponentModelWeight/CounterOnly/ScikitMixed/ScikitMixed.json"
)

// initProcessComponentPowerModelConfig: the Process component power model must be set by default.
//
//nolint:all
func initProcessComponentPowerModelConfig() types.ModelConfig {
	modelConfig := InitModelConfig(config.ProcessComponentsKey)
	if modelConfig.InitModelURL == "" {
		modelConfig.InitModelURL = defaultCounterDynCompURL
	}
	return modelConfig
}

func InitProcessPowerEstimator(usageMetrics, systemFeatures, systemValues []string) {
	ProcessTotalPowerModelConfig := InitModelConfig(config.ProcessTotalKey)
	var estimateFunc interface{}
	// init func for ProcessTotalPower
	ProcessTotalPowerModelValid, estimateFunc = initEstimateFunction(ProcessTotalPowerModelConfig, types.DynPower, types.DynModelWeight, usageMetrics, systemFeatures, systemValues, true)
	if ProcessTotalPowerModelValid {
		ProcessTotalPowerModelFunc = estimateFunc.(func([][]float64, []string) ([]float64, error))
	}
	ProcessComponentPowerModelConfig := initProcessComponentPowerModelConfig()
	// init func for ProcessComponentPower
	ProcessComponentPowerModelValid, estimateFunc = initEstimateFunction(ProcessComponentPowerModelConfig, types.DynComponentPower, types.DynComponentModelWeight, usageMetrics, systemFeatures, systemValues, false)
	if ProcessComponentPowerModelValid {
		ProcessComponentPowerModelFunc = estimateFunc.(func([][]float64, []string) (map[string][]float64, error))
	}
}

// updateProcessEnergy returns Process energy consumption for each node component
func UpdateProcessEnergy(processMetrics map[uint64]*collector_metric.ProcessMetrics, systemContainerMetrics *collector_metric.ContainerMetrics) {
	var wg sync.WaitGroup
	// If the node can expose power measurement per component, we can use the RATIO power model
	// Otherwise, we estimate it from trained power model
	if components.IsSystemCollectionSupported() {
		wg.Add(6)
		go local.UpdateProcessComponentEnergyByRatioPowerModel(processMetrics, systemContainerMetrics, collector_metric.PKG, config.CoreUsageMetric, &wg)
		go local.UpdateProcessComponentEnergyByRatioPowerModel(processMetrics, systemContainerMetrics, collector_metric.CORE, config.CoreUsageMetric, &wg)
		go local.UpdateProcessComponentEnergyByRatioPowerModel(processMetrics, systemContainerMetrics, collector_metric.PLATFORM, config.CoreUsageMetric, &wg)
		go local.UpdateProcessComponentEnergyByRatioPowerModel(processMetrics, systemContainerMetrics, collector_metric.DRAM, config.DRAMUsageMetric, &wg)
		// If the resource usage metrics is empty, we evenly divide the power consumption of the resource across all processes
		go local.UpdateProcessComponentEnergyByRatioPowerModel(processMetrics, systemContainerMetrics, collector_metric.UNCORE, "", &wg)
		go local.UpdateProcessComponentEnergyByRatioPowerModel(processMetrics, systemContainerMetrics, collector_metric.OTHER, "", &wg)
	} else {
		updateProcessEnergyByTrainedPowerModel(processMetrics)
	}

	// Currently, we do not have a power model that can forecast GPU power in the absence of real-time power metrics.
	// Generally, if we can obtain GPU metrics, we can also acquire GPU power metrics.
	if accelerator.IsGPUCollectionSupported() {
		wg.Add(1)
		go local.UpdateProcessComponentEnergyByRatioPowerModel(processMetrics, systemContainerMetrics, collector_metric.GPU, config.GpuUsageMetric, &wg)
	}
	wg.Wait()
}

func updateProcessEnergyByTrainedPowerModel(processsMetrics map[uint64]*collector_metric.ProcessMetrics) {
	var enabled bool
	// convert the Process metrics map to an array since the model server does not receive structured data
	// TODO: send data to model server via protobuf instead of no structured data
	processMetricValuesOnly, processIDList := processMetricsToArray(processsMetrics)

	totalPowerValid, totalProcessPowers := getProcessTotalPower(processMetricValuesOnly)

	enabled, processComponentPowers := getProcessComponentPowers(processMetricValuesOnly)
	if !enabled {
		klog.V(5).Infoln("No ProcessComponentPower Model")
		return
	}
	processOtherPowers := make([]uint64, len(processComponentPowers))
	if totalPowerValid {
		for index, componentPower := range processComponentPowers {
			// TODO: include GPU into consideration
			processOtherPowers[index] = uint64(totalProcessPowers[index]) - componentPower.Pkg - componentPower.DRAM
		}
	}

	// update the Process's components energy consumption
	// TODO: the model server does not predict GPU
	for i, ProcessID := range processIDList {
		if err := processsMetrics[ProcessID].DynEnergyInCore.AddNewDelta(processComponentPowers[i].Core); err != nil {
			klog.V(5).Infoln(err)
		}
		if err := processsMetrics[ProcessID].DynEnergyInDRAM.AddNewDelta(processComponentPowers[i].DRAM); err != nil {
			klog.V(5).Infoln(err)
		}
		if err := processsMetrics[ProcessID].DynEnergyInUncore.AddNewDelta(processComponentPowers[i].Uncore); err != nil {
			klog.V(5).Infoln(err)
		}
		if err := processsMetrics[ProcessID].DynEnergyInPkg.AddNewDelta(processComponentPowers[i].Pkg); err != nil {
			klog.V(5).Infoln(err)
		}
		if err := processsMetrics[ProcessID].DynEnergyInOther.AddNewDelta(processOtherPowers[i]); err != nil {
			klog.V(5).Infoln(err)
		}
	}
}

// getProcessTotalPower returns estimated pods' total power
func getProcessTotalPower(processMetricValuesOnly [][]float64) (valid bool, results []float64) {
	valid = false
	if ProcessTotalPowerModelValid {
		powers, err := ProcessTotalPowerModelFunc(processMetricValuesOnly, collector_metric.NodeMetadataValues)
		if err != nil || len(powers) == 0 {
			return
		}
		results = powers
		valid = true
		return
	}
	return
}

// getProcessComponentPowers returns estimated pods' RAPL power
func getProcessComponentPowers(processMetricValuesOnly [][]float64) (bool, []source.NodeComponentsEnergy) {
	processNumber := len(processMetricValuesOnly)
	if ProcessComponentPowerModelValid {
		powers, err := ProcessComponentPowerModelFunc(processMetricValuesOnly, collector_metric.NodeMetadataValues)
		if err != nil {
			return false, make([]source.NodeComponentsEnergy, processNumber)
		}
		raplPowers := make([]source.NodeComponentsEnergy, processNumber)
		for index := 0; index < processNumber; index++ {
			pkgPower := getComponentPower(powers, "pkg", index)
			corePower := getComponentPower(powers, "core", index)
			uncorePower := getComponentPower(powers, "uncore", index)
			dramPower := getComponentPower(powers, "dram", index)
			raplPowers[index] = fillRAPLPower(pkgPower, corePower, uncorePower, dramPower)
		}
		return true, raplPowers
	}
	return false, make([]source.NodeComponentsEnergy, processNumber)
}
