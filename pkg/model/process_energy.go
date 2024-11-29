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
	"fmt"

	"github.com/sustainable-computing-io/kepler/pkg/bpf"
	"github.com/sustainable-computing-io/kepler/pkg/collector/stats"
	"github.com/sustainable-computing-io/kepler/pkg/config"
	"github.com/sustainable-computing-io/kepler/pkg/model/types"
	"github.com/sustainable-computing-io/kepler/pkg/node"
	acc "github.com/sustainable-computing-io/kepler/pkg/sensors/accelerator"
	"github.com/sustainable-computing-io/kepler/pkg/sensors/components/source"
	"github.com/sustainable-computing-io/kepler/pkg/utils"
	"k8s.io/klog/v2"
)

var (
	processPlatformPowerModel  PowerModelInterface
	processComponentPowerModel PowerModelInterface
)

// createProcessPowerModelConfig: the process component power model must be set by default.
func createProcessPowerModelConfig(powerSourceTarget string, processFeatureNames []string, energySource string) (modelConfig *types.ModelConfig) {
	systemMetaDataFeatureNames := node.MetadataFeatureNames()
	systemMetaDataFeatureValues := node.MetadataFeatureValues()
	bpfSupportedMetrics := bpf.DefaultSupportedMetrics()
	modelConfig = CreatePowerModelConfig(powerSourceTarget)
	if modelConfig == nil {
		return nil
	}
	if modelConfig.InitModelURL == "" {
		modelConfig.InitModelFilepath = config.GetDefaultPowerModelURL(modelConfig.ModelOutputType.String(), energySource)
	}
	modelConfig.ProcessFeatureNames = processFeatureNames
	modelConfig.SystemMetaDataFeatureNames = systemMetaDataFeatureNames
	modelConfig.SystemMetaDataFeatureValues = systemMetaDataFeatureValues

	// Ratio power model has different features than the other estimators.
	// Ratio power model has node resource and power consumption as features, as it is used to calculate the ratio.
	if modelConfig.ModelType == types.Ratio {
		if powerSourceTarget == config.ProcessComponentsPowerKey() {
			pkgUsageMetric := config.CoreUsageMetric()
			coreUsageMetric := config.CoreUsageMetric()
			dramUsageMetric := config.DRAMUsageMetric()
			if !bpfSupportedMetrics.HardwareCounters.Has(config.CPUTime) {
				// Given that there is no HW counter in  some scenarios (e.g. on VMs), we have to use CPUTime data.
				// Although a busy CPU is more likely to be accessing memory the CPU utilization (CPUTime) does not directly
				// represent memory access, but it remains the only viable proxy available to approximate such information.
				pkgUsageMetric, coreUsageMetric, dramUsageMetric = config.CPUTime, config.CPUTime, config.CPUTime
			}
			// ProcessFeatureNames contains the metrics that represents the process resource utilization
			modelConfig.ProcessFeatureNames = []string{
				pkgUsageMetric,              // for PKG resource usage
				coreUsageMetric,             // for CORE resource usage
				dramUsageMetric,             // for DRAM resource usage
				config.GeneralUsageMetric(), // for UNCORE resource usage
				config.GeneralUsageMetric(), // for OTHER resource usage
				config.GPUUsageMetric(),     // for GPU resource usage
			}
			// NodeFeatureNames contains the metrics that represents the node resource utilization plus the dynamic and idle power power consumption
			modelConfig.NodeFeatureNames = modelConfig.ProcessFeatureNames
			modelConfig.NodeFeatureNames = append(modelConfig.NodeFeatureNames, []string{
				config.DynEnergyInPkg,     // for dynamic PKG power consumption
				config.DynEnergyInCore,    // for dynamic CORE power consumption
				config.DynEnergyInDRAM,    // for dynamic DRAM power consumption
				config.DynEnergyInUnCore,  // for dynamic UNCORE power consumption
				config.DynEnergyInOther,   // for dynamic OTHER power consumption
				config.DynEnergyInGPU,     // for dynamic GPU power consumption
				config.IdleEnergyInPkg,    // for idle PKG power consumption
				config.IdleEnergyInCore,   // for idle CORE power consumption
				config.IdleEnergyInDRAM,   // for idle DRAM power consumption
				config.IdleEnergyInUnCore, // for idle UNCORE power consumption
				config.IdleEnergyInOther,  // for idle OTHER power consumption
				config.IdleEnergyInGPU,    // for idle GPU power consumption
			}...)
		} else if powerSourceTarget == config.ProcessPlatformPowerKey() {
			platformUsageMetric := config.CoreUsageMetric()
			if !bpfSupportedMetrics.HardwareCounters.Has(config.CPUTime) {
				// Given that there is no HW counter in  some scenarios (e.g. on VMs), we have to use CPUTime data.
				platformUsageMetric = config.CPUTime
			}
			modelConfig.ProcessFeatureNames = []string{
				platformUsageMetric, // for PLATFORM resource usage
			}
			modelConfig.NodeFeatureNames = modelConfig.ProcessFeatureNames
			modelConfig.NodeFeatureNames = append(modelConfig.NodeFeatureNames, []string{
				config.DynEnergyInPlatform,  // for dynamic PLATFORM power consumption
				config.IdleEnergyInPlatform, // for idle PLATFORM power consumption
			}...)
		}
	}

	return modelConfig
}

func CreateProcessPowerEstimatorModel(processFeatureNames []string) {
	keys := map[string]string{
		config.ProcessPlatformPowerKey():   types.PlatformEnergySource,
		config.ProcessComponentsPowerKey(): types.ComponentEnergySource,
	}
	for k, v := range keys {
		modelConfig := createProcessPowerModelConfig(k, processFeatureNames, v)
		modelConfig.IsNodePowerModel = false
		m, err := createPowerModelEstimator(modelConfig)
		switch k {
		case config.ProcessPlatformPowerKey():
			processPlatformPowerModel = m
		case config.ProcessComponentsPowerKey():
			processComponentPowerModel = m
		}
		if err != nil {
			klog.Infof("Failed to create %s Power Model to estimate %s Power: %v\n", modelConfig.ModelType.String()+"/"+modelConfig.ModelOutputType.String(), k, err)
		} else {
			klog.V(1).Infof("Using the %s Power Model to estimate %s Power", m.GetModelType(), k)
			klog.V(1).Infof("Feature names: %v", m.GetProcessFeatureNamesList())
		}
	}
}

// UpdateProcessEnergy resets the power model samples, add new samples to the power models, then estimates the idle and dynamic energy
func UpdateProcessEnergy(processesMetrics map[uint64]*stats.ProcessStats, nodeMetrics *stats.NodeStats) {
	if processPlatformPowerModel == nil {
		klog.Errorln("Process Platform Power Model was not created")
	}
	if processComponentPowerModel == nil {
		klog.Errorln("Process Component Power Model was not created")
	}
	// reset power sample slide window
	processPlatformPowerModel.ResetSampleIdx()
	processComponentPowerModel.ResetSampleIdx()

	// add features values for prediction
	processIDList := addSamplesToPowerModels(processesMetrics, nodeMetrics)
	addEstimatedEnergy(processIDList, processesMetrics, idlePower)
	addEstimatedEnergy(processIDList, processesMetrics, absPower)
}

// addSamplesToPowerModels converts process's metrics to array to add the samples to the power model
func addSamplesToPowerModels(processesMetrics map[uint64]*stats.ProcessStats, nodeMetrics *stats.NodeStats) []uint64 {
	processIDList := []uint64{}
	// Add process metrics
	for processID, c := range processesMetrics {
		// add samples to estimate the platform power
		if processPlatformPowerModel.IsEnabled() {
			featureValues := c.ToEstimatorValues(processPlatformPowerModel.GetProcessFeatureNamesList(), true) // add process features with normalized values
			processPlatformPowerModel.AddProcessFeatureValues(featureValues)
		}

		// add samples to estimate the components (CPU and DRAM) power
		if processComponentPowerModel.IsEnabled() {
			// Add process metrics
			featureValues := c.ToEstimatorValues(processComponentPowerModel.GetProcessFeatureNamesList(), true) // add node features with normalized values
			processComponentPowerModel.AddProcessFeatureValues(featureValues)
		}

		processIDList = append(processIDList, processID)
	}
	// Add node metrics.
	if processPlatformPowerModel.IsEnabled() {
		featureValues := nodeMetrics.ToEstimatorValues(processPlatformPowerModel.GetNodeFeatureNamesList(), true) // add node features with normalized values
		processPlatformPowerModel.AddNodeFeatureValues(featureValues)
	}
	if processComponentPowerModel.IsEnabled() {
		featureValues := nodeMetrics.ToEstimatorValues(processComponentPowerModel.GetNodeFeatureNamesList(), true) // add node features with normalized values
		processComponentPowerModel.AddNodeFeatureValues(featureValues)
	}
	return processIDList
}

// addEstimatedEnergy estimates the idle power consumption
func addEstimatedEnergy(processIDList []uint64, processesMetrics map[uint64]*stats.ProcessStats, isIdlePower bool) {
	var processGPUPower []uint64
	var processPlatformPower []uint64
	var processComponentsPower []source.NodeComponentsEnergy

	errComp := fmt.Errorf("component power model is not enabled")
	errGPU := fmt.Errorf("gpu power model is not enabled")
	errPlat := fmt.Errorf("plat power model is not enabled")

	// estimate the associated power consumption of all RAPL node components for each process
	if processComponentPowerModel.IsEnabled() {
		processComponentsPower, errComp = processComponentPowerModel.GetComponentsPower(isIdlePower)
		if errComp != nil {
			klog.V(5).Infoln("Could not estimate the Process Components Power")
		}
		// estimate the associated power consumption of GPU for each process
		if config.IsGPUEnabled() {
			if gpu := acc.GetActiveAcceleratorByType(config.GPU); gpu != nil {
				processGPUPower, errGPU = processComponentPowerModel.GetGPUPower(isIdlePower)
				if errGPU != nil {
					klog.V(5).Infoln("Could not estimate the Process GPU Power")
				}
			}
		}
	}
	// estimate the associated power consumption of platform for each process
	if processPlatformPowerModel.IsEnabled() {
		processPlatformPower, errPlat = processPlatformPowerModel.GetPlatformPower(isIdlePower)
		if errPlat != nil {
			klog.V(5).Infoln("Could not estimate the Process Platform Power")
		}
	}

	var energy uint64
	for i, processID := range processIDList {
		if errComp == nil {
			// add PKG power consumption
			// since Kepler collects metrics at intervals of SamplePeriodSec, which is greater than 1 second, it is necessary to calculate the energy consumption for the entire waiting period
			energy = processComponentsPower[i].Pkg * config.SamplePeriodSec()
			if isIdlePower {
				processesMetrics[processID].EnergyUsage[config.IdleEnergyInPkg].SetDeltaStat(utils.GenericSocketID, energy)
			} else {
				processesMetrics[processID].EnergyUsage[config.DynEnergyInPkg].SetDeltaStat(utils.GenericSocketID, energy)
			}

			// add CORE power consumption
			energy = processComponentsPower[i].Core * config.SamplePeriodSec()
			if isIdlePower {
				processesMetrics[processID].EnergyUsage[config.IdleEnergyInCore].SetDeltaStat(utils.GenericSocketID, energy)
			} else {
				processesMetrics[processID].EnergyUsage[config.DynEnergyInCore].SetDeltaStat(utils.GenericSocketID, energy)
			}

			// add DRAM power consumption
			energy = processComponentsPower[i].DRAM * config.SamplePeriodSec()
			if isIdlePower {
				processesMetrics[processID].EnergyUsage[config.IdleEnergyInDRAM].SetDeltaStat(utils.GenericSocketID, energy)
			} else {
				processesMetrics[processID].EnergyUsage[config.DynEnergyInDRAM].SetDeltaStat(utils.GenericSocketID, energy)
			}

			// add Uncore power consumption
			energy = processComponentsPower[i].Uncore * config.SamplePeriodSec()
			if isIdlePower {
				processesMetrics[processID].EnergyUsage[config.IdleEnergyInUnCore].SetDeltaStat(utils.GenericSocketID, energy)
			} else {
				processesMetrics[processID].EnergyUsage[config.DynEnergyInUnCore].SetDeltaStat(utils.GenericSocketID, energy)
			}

			// add GPU power consumption
			if errGPU == nil {
				energy = processGPUPower[i] * (config.SamplePeriodSec())
				if isIdlePower {
					processesMetrics[processID].EnergyUsage[config.IdleEnergyInGPU].SetDeltaStat(utils.GenericSocketID, energy)
				} else {
					processesMetrics[processID].EnergyUsage[config.DynEnergyInGPU].SetDeltaStat(utils.GenericSocketID, energy)
				}
			}
		}

		if errPlat == nil {
			energy = processPlatformPower[i] * config.SamplePeriodSec()
			if isIdlePower {
				processesMetrics[processID].EnergyUsage[config.IdleEnergyInPlatform].SetDeltaStat(utils.GenericSocketID, energy)
			} else {
				processesMetrics[processID].EnergyUsage[config.DynEnergyInPlatform].SetDeltaStat(utils.GenericSocketID, energy)
			}
		}

		// estimate other components power if both platform and components power are available
		if errComp == nil && errPlat == nil {
			// TODO: verify if Platform power also includes the GPU into consideration
			var otherPower uint64
			if processPlatformPower[i] <= (processComponentsPower[i].Pkg + processComponentsPower[i].DRAM) {
				otherPower = 0
			} else {
				otherPower = processPlatformPower[i] - processComponentsPower[i].Pkg - processComponentsPower[i].DRAM
			}
			energy = otherPower * config.SamplePeriodSec()
			if isIdlePower {
				processesMetrics[processID].EnergyUsage[config.IdleEnergyInOther].SetDeltaStat(utils.GenericSocketID, energy)
			} else {
				processesMetrics[processID].EnergyUsage[config.DynEnergyInOther].SetDeltaStat(utils.GenericSocketID, energy)
			}
		}
	}
}
