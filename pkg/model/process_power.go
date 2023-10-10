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

//nolint:dupl // we should have only the container package
package model

import (
	"fmt"

	"github.com/sustainable-computing-io/kepler/pkg/bpfassets/attacher"
	collector_metric "github.com/sustainable-computing-io/kepler/pkg/collector/metric"
	"github.com/sustainable-computing-io/kepler/pkg/config"
	"github.com/sustainable-computing-io/kepler/pkg/model/types"
	"github.com/sustainable-computing-io/kepler/pkg/power/accelerator/gpu"
	"github.com/sustainable-computing-io/kepler/pkg/power/components/source"
	"k8s.io/klog/v2"
)

var (
	ProcessPlatformPowerModel  PowerMoldelInterface
	ProcessComponentPowerModel PowerMoldelInterface

	// cgroupOnly
)

// createProcessPowerModelConfig: the process component power model must be set by default.
func createProcessPowerModelConfig(powerSourceTarget string, processFeatureNames, systemMetaDataFeatureNames, systemMetaDataFeatureValues []string, energySource string) *types.ModelConfig {
	modelConfig := CreatePowerModelConfig(powerSourceTarget)
	if modelConfig.InitModelURL == "" {
		modelConfig.InitModelFilepath = config.GetDefaultPowerModelURL(modelConfig.ModelOutputType.String(), energySource)
	}
	modelConfig.ContainerFeatureNames = processFeatureNames
	modelConfig.SystemMetaDataFeatureNames = systemMetaDataFeatureNames
	modelConfig.SystemMetaDataFeatureValues = systemMetaDataFeatureValues

	// Ratio power model has different features than the other estimators.
	// Ratio power model has node resource and power consumption as features, as it is used to calculate the ratio.
	if modelConfig.ModelType == types.Ratio {
		if powerSourceTarget == config.ProcessComponentsPowerKey {
			pkgUsageMetric := config.CoreUsageMetric
			coreUsageMetric := config.CoreUsageMetric
			dramUsageMetric := config.DRAMUsageMetric
			if !attacher.HardwareCountersEnabled {
				// Given that there is no HW counter in  some scenarios (e.g. on VMs), we have to use CPUTime data.
				// Although a busy CPU is more likely to be accessing memory the CPU utilization (CPUTime) does not directly
				// represent memory access, but it remains the only viable proxy available to approximate such information.
				pkgUsageMetric, coreUsageMetric, dramUsageMetric = config.CPUTime, config.CPUTime, config.CPUTime
			}
			// ProcessFeatureNames contains the metrics that represents the process resource utilization
			modelConfig.ContainerFeatureNames = []string{
				pkgUsageMetric,            // for PKG resource usage
				coreUsageMetric,           // for CORE resource usage
				dramUsageMetric,           // for DRAM resource usage
				config.GeneralUsageMetric, // for UNCORE resource usage
				config.GeneralUsageMetric, // for OTHER resource usage
				config.GpuUsageMetric,     // for GPU resource usage
			}
			// NodeFeatureNames contains the metrics that represents the node resource utilization plus the dynamic and idle power power consumption
			modelConfig.NodeFeatureNames = modelConfig.ContainerFeatureNames
			modelConfig.NodeFeatureNames = append(modelConfig.NodeFeatureNames, []string{
				collector_metric.PKG + "_DYN",     // for dynamic PKG power consumption
				collector_metric.CORE + "_DYN",    // for dynamic CORE power consumption
				collector_metric.DRAM + "_DYN",    // for dynamic DRAM power consumption
				collector_metric.UNCORE + "_DYN",  // for dynamic UNCORE power consumption
				collector_metric.OTHER + "_DYN",   // for dynamic OTHER power consumption
				collector_metric.GPU + "_DYN",     // for dynamic GPU power consumption
				collector_metric.PKG + "_IDLE",    // for idle PKG power consumption
				collector_metric.CORE + "_IDLE",   // for idle CORE power consumption
				collector_metric.DRAM + "_IDLE",   // for idle DRAM power consumption
				collector_metric.UNCORE + "_IDLE", // for idle UNCORE power consumption
				collector_metric.OTHER + "_IDLE",  // for idle OTHER power consumption
				collector_metric.GPU + "_IDLE",    // for idle GPU power consumption
			}...)
		} else if powerSourceTarget == config.ProcessPlatformPowerKey {
			platformUsageMetric := config.CoreUsageMetric
			if !attacher.HardwareCountersEnabled {
				// Given that there is no HW counter in  some scenarios (e.g. on VMs), we have to use CPUTime data.
				platformUsageMetric = config.CPUTime
			}
			modelConfig.ContainerFeatureNames = []string{
				platformUsageMetric, // for PLATFORM resource usage
			}
			modelConfig.NodeFeatureNames = modelConfig.ContainerFeatureNames
			modelConfig.NodeFeatureNames = append(modelConfig.NodeFeatureNames, []string{
				collector_metric.PLATFORM + "_DYN",  // for dynamic PLATFORM power consumption
				collector_metric.PLATFORM + "_IDLE", // for idle PLATFORM power consumption
			}...)
		}
	}

	return modelConfig
}

func CreateProcessPowerEstimatorModel(processFeatureNames, systemMetaDataFeatureNames, systemMetaDataFeatureValues []string) {
	var err error
	modelConfig := createProcessPowerModelConfig(config.ProcessPlatformPowerKey, processFeatureNames, systemMetaDataFeatureNames, systemMetaDataFeatureValues, types.PlatformEnergySource)
	modelConfig.IsNodePowerModel = false
	ProcessPlatformPowerModel, err = createPowerModelEstimator(modelConfig)
	if err == nil {
		klog.Infof("Using the %s Power Model to estimate Process Platform Power", modelConfig.ModelType.String()+"/"+modelConfig.ModelOutputType.String())
		klog.Infof("Container feature names: %v", modelConfig.ContainerFeatureNames)
	} else {
		klog.Infof("Failed to create %s Power Model to estimate Process Platform Power: %v\n", modelConfig.ModelType.String()+"/"+modelConfig.ModelOutputType.String(), err)
	}

	modelConfig = createProcessPowerModelConfig(config.ProcessComponentsPowerKey, processFeatureNames, systemMetaDataFeatureNames, systemMetaDataFeatureValues, types.ComponentEnergySource)
	modelConfig.IsNodePowerModel = false
	ProcessComponentPowerModel, err = createPowerModelEstimator(modelConfig)
	if err == nil {
		klog.Infof("Using the %s Power Model to estimate Process Component Power", modelConfig.ModelType.String()+"/"+modelConfig.ModelOutputType.String())
		klog.Infof("Container feature names: %v", modelConfig.ContainerFeatureNames)
	} else {
		klog.Infof("Failed to create %s Power Model to estimate Process Component Power: %v\n", modelConfig.ModelType.String()+"/"+modelConfig.ModelOutputType.String(), err)
	}
}

// UpdateProcessEnergy resets the power model samples, add new samples to the power models, then estimates the idle and dynamic energy
func UpdateProcessEnergy(processMetrics map[uint64]*collector_metric.ProcessMetrics, nodeMetrics *collector_metric.NodeMetrics) {
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
	processIDList := addProcessSamplesToPowerModels(processMetrics, nodeMetrics)

	addProcessEstimatedEnergy(processIDList, processMetrics, idlePower)
	addProcessEstimatedEnergy(processIDList, processMetrics, absPower)
}

// addProcessSamplesToPowerModels converts process's metrics to array to add the samples to the power model
func addProcessSamplesToPowerModels(processMetrics map[uint64]*collector_metric.ProcessMetrics, nodeMetrics *collector_metric.NodeMetrics) []uint64 {
	processIDList := []uint64{}
	// Add process metrics
	for processID, c := range processMetrics {
		// add samples to estimate the platform power
		if ProcessPlatformPowerModel.IsEnabled() {
			featureValues := c.ToEstimatorValues(ProcessPlatformPowerModel.GetContainerFeatureNamesList(), true) // add process features with normalized values
			ProcessPlatformPowerModel.AddContainerFeatureValues(featureValues)
		}

		// add samples to estimate the components (CPU and DRAM) power
		if ProcessComponentPowerModel.IsEnabled() {
			// Add process metrics
			featureValues := c.ToEstimatorValues(ProcessComponentPowerModel.GetContainerFeatureNamesList(), true) // add node features with normalized values
			ProcessComponentPowerModel.AddContainerFeatureValues(featureValues)
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

// addProcessEstimatedEnergy estimates the idle power consumption
func addProcessEstimatedEnergy(processIDList []uint64, processMetrics map[uint64]*collector_metric.ProcessMetrics, isIdlePower bool) {
	var err error
	var processGPUPower []float64
	var processPlatformPower []float64
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
		if gpu.IsGPUCollectionSupported() {
			processGPUPower, errGPU = ProcessComponentPowerModel.GetGPUPower(isIdlePower)
			if errGPU != nil {
				klog.V(5).Infoln("Could not estimate the Process GPU Power")
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
				err = processMetrics[processID].IdleEnergyInPkg.AddNewDelta(energy)
			} else {
				err = processMetrics[processID].DynEnergyInPkg.AddNewDelta(energy)
			}
			if err != nil {
				klog.V(5).Infoln(err)
			}

			// add CORE power consumption
			energy = processComponentsPower[i].Core * config.SamplePeriodSec
			if isIdlePower {
				err = processMetrics[processID].IdleEnergyInCore.AddNewDelta(energy)
			} else {
				err = processMetrics[processID].DynEnergyInCore.AddNewDelta(energy)
			}
			if err != nil {
				klog.V(5).Infoln(err)
			}

			// add DRAM power consumption
			energy = processComponentsPower[i].DRAM * config.SamplePeriodSec
			if isIdlePower {
				err = processMetrics[processID].IdleEnergyInDRAM.AddNewDelta(energy)
			} else {
				err = processMetrics[processID].DynEnergyInDRAM.AddNewDelta(energy)
			}
			if err != nil {
				klog.V(5).Infoln(err)
			}

			// add Uncore power consumption
			energy = processComponentsPower[i].Uncore * config.SamplePeriodSec
			if isIdlePower {
				err = processMetrics[processID].IdleEnergyInUncore.AddNewDelta(energy)
			} else {
				err = processMetrics[processID].DynEnergyInUncore.AddNewDelta(energy)
			}
			if err != nil {
				klog.V(5).Infoln(err)
			}

			// add GPU power consumption
			if errGPU == nil {
				energy = uint64(processGPUPower[i]) * (config.SamplePeriodSec)
				if isIdlePower {
					err = processMetrics[processID].IdleEnergyInGPU.AddNewDelta(energy)
				} else {
					err = processMetrics[processID].DynEnergyInGPU.AddNewDelta(energy)
				}
				if err != nil {
					klog.V(5).Infoln(err)
				}
			}
		}

		if errPlat == nil {
			energy = uint64(processPlatformPower[i]) * config.SamplePeriodSec
			if isIdlePower {
				err = processMetrics[processID].IdleEnergyInPlatform.AddNewDelta(energy)
			} else {
				err = processMetrics[processID].DynEnergyInPlatform.AddNewDelta(energy)
			}
			if err != nil {
				klog.V(5).Infoln(err)
			}
		}

		// estimate other components power if both platform and components power are available
		if errComp == nil && errPlat == nil {
			// TODO: verify if Platform power also includes the GPU into consideration
			otherPower := processPlatformPower[i] - float64(processComponentsPower[i].Pkg) - float64(processComponentsPower[i].DRAM)
			if otherPower < 0 {
				otherPower = 0
			}
			energy = uint64(otherPower) * config.SamplePeriodSec
			if isIdlePower {
				err = processMetrics[processID].IdleEnergyInOther.AddNewDelta(energy)
			} else {
				err = processMetrics[processID].DynEnergyInOther.AddNewDelta(energy)
			}
			if err != nil {
				klog.V(5).Infoln(err)
			}
		}
	}
}
