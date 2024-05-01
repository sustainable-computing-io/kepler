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
	acc "github.com/sustainable-computing-io/kepler/pkg/sensors/accelerator"
	"github.com/sustainable-computing-io/kepler/pkg/sensors/components/source"
	"github.com/sustainable-computing-io/kepler/pkg/utils"
	"k8s.io/klog/v2"
)

var (
	ProcessPlatformPowerModel  PowerModelInterface
	ProcessComponentPowerModel PowerModelInterface
)

// createProcessPowerModelConfig: the process component power model must be set by default.
func createProcessPowerModelConfig(powerSourceTarget string, processFeatureNames, systemMetaDataFeatureNames, systemMetaDataFeatureValues []string, energySource string, bpfSupportedMetrics bpf.SupportedMetrics) (modelConfig *types.ModelConfig) {
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
		if powerSourceTarget == config.ProcessComponentsPowerKey {
			pkgUsageMetric := config.CoreUsageMetric
			coreUsageMetric := config.CoreUsageMetric
			dramUsageMetric := config.DRAMUsageMetric
			if !bpfSupportedMetrics.HardwareCounters.Has(config.CPUTime) {
				// Given that there is no HW counter in  some scenarios (e.g. on VMs), we have to use CPUTime data.
				// Although a busy CPU is more likely to be accessing memory the CPU utilization (CPUTime) does not directly
				// represent memory access, but it remains the only viable proxy available to approximate such information.
				pkgUsageMetric, coreUsageMetric, dramUsageMetric = config.CPUTime, config.CPUTime, config.CPUTime
			}
			// ProcessFeatureNames contains the metrics that represents the process resource utilization
			modelConfig.ProcessFeatureNames = []string{
				pkgUsageMetric,            // for PKG resource usage
				coreUsageMetric,           // for CORE resource usage
				dramUsageMetric,           // for DRAM resource usage
				config.GeneralUsageMetric, // for UNCORE resource usage
				config.GeneralUsageMetric, // for OTHER resource usage
				config.GpuUsageMetric,     // for GPU resource usage
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
		} else if powerSourceTarget == config.ProcessPlatformPowerKey {
			platformUsageMetric := config.CoreUsageMetric
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

func CreateProcessPowerEstimatorModel(processFeatureNames, systemMetaDataFeatureNames, systemMetaDataFeatureValues []string, bpfSupportedMetrics bpf.SupportedMetrics) {
	var err error
	modelConfig := createProcessPowerModelConfig(config.ProcessPlatformPowerKey, processFeatureNames, systemMetaDataFeatureNames, systemMetaDataFeatureValues, types.PlatformEnergySource, bpfSupportedMetrics)
	modelConfig.IsNodePowerModel = false
	ProcessPlatformPowerModel, err = createPowerModelEstimator(modelConfig)
	if err == nil {
		klog.V(1).Infof("Using the %s Power Model to estimate Process Platform Power", modelConfig.ModelType.String()+"/"+modelConfig.ModelOutputType.String())
		klog.V(1).Infof("Process feature names: %v", modelConfig.ProcessFeatureNames)
	} else {
		klog.Infof("Failed to create %s Power Model to estimate Process Platform Power: %v\n", modelConfig.ModelType.String()+"/"+modelConfig.ModelOutputType.String(), err)
	}

	modelConfig = createProcessPowerModelConfig(config.ProcessComponentsPowerKey, processFeatureNames, systemMetaDataFeatureNames, systemMetaDataFeatureValues, types.ComponentEnergySource, bpfSupportedMetrics)
	modelConfig.IsNodePowerModel = false
	ProcessComponentPowerModel, err = createPowerModelEstimator(modelConfig)
	if err == nil {
		klog.V(1).Infof("Using the %s Power Model to estimate Process Component Power", modelConfig.ModelType.String()+"/"+modelConfig.ModelOutputType.String())
		klog.V(1).Infof("Process feature names: %v", modelConfig.ProcessFeatureNames)
	} else {
		klog.Infof("Failed to create %s Power Model to estimate Process Component Power: %v\n", modelConfig.ModelType.String()+"/"+modelConfig.ModelOutputType.String(), err)
	}
}

// UpdateProcessEnergy resets the power model samples, add new samples to the power models, then estimates the idle and dynamic energy
func UpdateProcessEnergy(processesMetrics map[uint64]*stats.ProcessStats, nodeMetrics *stats.NodeStats) {
	if ProcessPlatformPowerModel == nil {
		klog.Errorln("Process Platform Power Model was not created")
	}
	if ProcessComponentPowerModel == nil {
		klog.Errorln("Process Component Power Model was not created")
	}
	// reset power sample slide window
	ProcessPlatformPowerModel.ResetSampleIdx()
	ProcessComponentPowerModel.ResetSampleIdx()

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
		if ProcessPlatformPowerModel.IsEnabled() {
			featureValues := c.ToEstimatorValues(ProcessPlatformPowerModel.GetProcessFeatureNamesList(), true) // add process features with normalized values
			ProcessPlatformPowerModel.AddProcessFeatureValues(featureValues)
		}

		// add samples to estimate the components (CPU and DRAM) power
		if ProcessComponentPowerModel.IsEnabled() {
			// Add process metrics
			featureValues := c.ToEstimatorValues(ProcessComponentPowerModel.GetProcessFeatureNamesList(), true) // add node features with normalized values
			ProcessComponentPowerModel.AddProcessFeatureValues(featureValues)
		}

		processIDList = append(processIDList, processID)
	}
	// Add node metrics.
	if ProcessPlatformPowerModel.IsEnabled() {
		featureValues := nodeMetrics.ToEstimatorValues(ProcessPlatformPowerModel.GetNodeFeatureNamesList(), true) // add node features with normalized values
		ProcessPlatformPowerModel.AddNodeFeatureValues(featureValues)
	}
	if ProcessComponentPowerModel.IsEnabled() {
		featureValues := nodeMetrics.ToEstimatorValues(ProcessComponentPowerModel.GetNodeFeatureNamesList(), true) // add node features with normalized values
		ProcessComponentPowerModel.AddNodeFeatureValues(featureValues)
	}
	return processIDList
}

// addEstimatedEnergy estimates the idle power consumption
func addEstimatedEnergy(processIDList []uint64, processesMetrics map[uint64]*stats.ProcessStats, isIdlePower bool) {
	var err error
	var processGPUPower []uint64
	var processPlatformPower []uint64
	var processComponentsPower []source.NodeComponentsEnergy

	errComp := fmt.Errorf("component power model is not enabled")
	errGPU := fmt.Errorf("gpu power model is not enabled")
	errPlat := fmt.Errorf("plat power model is not enabled")

	// estimate the associated power comsumption of all RAPL node components for each process
	if ProcessComponentPowerModel.IsEnabled() {
		processComponentsPower, errComp = ProcessComponentPowerModel.GetComponentsPower(isIdlePower)
		if errComp != nil {
			klog.V(5).Infoln("Could not estimate the Process Components Power")
		}
		// estimate the associated power comsumption of GPU for each process
		if config.EnabledGPU {
			if _, err := acc.GetActiveAcceleratorsByType("gpu"); err == nil {
				processGPUPower, errGPU = ProcessComponentPowerModel.GetGPUPower(isIdlePower)
				if errGPU != nil {
					klog.V(5).Infoln("Could not estimate the Process GPU Power")
				}
			}
		}
	}
	// estimate the associated power comsumption of platform for each process
	if ProcessPlatformPowerModel.IsEnabled() {
		processPlatformPower, errPlat = ProcessPlatformPowerModel.GetPlatformPower(isIdlePower)
		if errPlat != nil {
			klog.V(5).Infoln("Could not estimate the Process Platform Power")
		}
	}

	var energy uint64
	for i, processID := range processIDList {
		if errComp == nil {
			// add PKG power consumption
			// since Kepler collects metrics at intervals of SamplePeriodSec, which is greater than 1 second, it is necessary to calculate the energy consumption for the entire waiting period
			energy = processComponentsPower[i].Pkg * config.SamplePeriodSec
			if isIdlePower {
				processesMetrics[processID].EnergyUsage[config.IdleEnergyInPkg].SetDeltaStat(utils.GenericSocketID, energy)
			} else {
				processesMetrics[processID].EnergyUsage[config.DynEnergyInPkg].SetDeltaStat(utils.GenericSocketID, energy)
			}
			if err != nil {
				klog.V(5).Infoln(err)
			}

			// add CORE power consumption
			energy = processComponentsPower[i].Core * config.SamplePeriodSec
			if isIdlePower {
				processesMetrics[processID].EnergyUsage[config.IdleEnergyInCore].SetDeltaStat(utils.GenericSocketID, energy)
			} else {
				processesMetrics[processID].EnergyUsage[config.DynEnergyInCore].SetDeltaStat(utils.GenericSocketID, energy)
			}
			if err != nil {
				klog.V(5).Infoln(err)
			}

			// add DRAM power consumption
			energy = processComponentsPower[i].DRAM * config.SamplePeriodSec
			if isIdlePower {
				processesMetrics[processID].EnergyUsage[config.IdleEnergyInDRAM].SetDeltaStat(utils.GenericSocketID, energy)
			} else {
				processesMetrics[processID].EnergyUsage[config.DynEnergyInDRAM].SetDeltaStat(utils.GenericSocketID, energy)
			}
			if err != nil {
				klog.V(5).Infoln(err)
			}

			// add Uncore power consumption
			energy = processComponentsPower[i].Uncore * config.SamplePeriodSec
			if isIdlePower {
				processesMetrics[processID].EnergyUsage[config.IdleEnergyInUnCore].SetDeltaStat(utils.GenericSocketID, energy)
			} else {
				processesMetrics[processID].EnergyUsage[config.DynEnergyInUnCore].SetDeltaStat(utils.GenericSocketID, energy)
			}
			if err != nil {
				klog.V(5).Infoln(err)
			}

			// add GPU power consumption
			if errGPU == nil {
				energy = processGPUPower[i] * (config.SamplePeriodSec)
				if isIdlePower {
					processesMetrics[processID].EnergyUsage[config.IdleEnergyInGPU].SetDeltaStat(utils.GenericSocketID, energy)
				} else {
					processesMetrics[processID].EnergyUsage[config.DynEnergyInGPU].SetDeltaStat(utils.GenericSocketID, energy)
				}
				if err != nil {
					klog.V(5).Infoln(err)
				}
			}
		}

		if errPlat == nil {
			energy = processPlatformPower[i] * config.SamplePeriodSec
			if isIdlePower {
				processesMetrics[processID].EnergyUsage[config.IdleEnergyInPlatform].SetDeltaStat(utils.GenericSocketID, energy)
			} else {
				processesMetrics[processID].EnergyUsage[config.DynEnergyInPlatform].SetDeltaStat(utils.GenericSocketID, energy)
			}
			if err != nil {
				klog.V(5).Infoln(err)
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
			energy = otherPower * config.SamplePeriodSec
			if isIdlePower {
				processesMetrics[processID].EnergyUsage[config.IdleEnergyInOther].SetDeltaStat(utils.GenericSocketID, energy)
			} else {
				processesMetrics[processID].EnergyUsage[config.DynEnergyInOther].SetDeltaStat(utils.GenericSocketID, energy)
			}
			if err != nil {
				klog.V(5).Infoln(err)
			}
		}
	}
}
