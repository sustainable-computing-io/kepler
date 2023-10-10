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

//nolint:dupl // process metrics should be here not in another package
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
	ContainerPlatformPowerModel  PowerMoldelInterface
	ContainerComponentPowerModel PowerMoldelInterface
)

// createContainerPowerModelConfig: the container component power model must be set by default.
func createContainerPowerModelConfig(powerSourceTarget string, containerFeatureNames, systemMetaDataFeatureNames, systemMetaDataFeatureValues []string, energySource string) (modelConfig *types.ModelConfig) {
	modelConfig = CreatePowerModelConfig(powerSourceTarget)
	if modelConfig == nil {
		return nil
	}
	if modelConfig.InitModelURL == "" {
		modelConfig.InitModelFilepath = config.GetDefaultPowerModelURL(modelConfig.ModelOutputType.String(), energySource)
	}
	modelConfig.ContainerFeatureNames = containerFeatureNames
	modelConfig.SystemMetaDataFeatureNames = systemMetaDataFeatureNames
	modelConfig.SystemMetaDataFeatureValues = systemMetaDataFeatureValues

	// Ratio power model has different features than the other estimators.
	// Ratio power model has node resource and power consumption as features, as it is used to calculate the ratio.
	if modelConfig.ModelType == types.Ratio {
		if powerSourceTarget == config.ContainerComponentsPowerKey {
			pkgUsageMetric := config.CoreUsageMetric
			coreUsageMetric := config.CoreUsageMetric
			dramUsageMetric := config.DRAMUsageMetric
			if !attacher.HardwareCountersEnabled {
				// Given that there is no HW counter in  some scenarios (e.g. on VMs), we have to use CPUTime data.
				// Although a busy CPU is more likely to be accessing memory the CPU utilization (CPUTime) does not directly
				// represent memory access, but it remains the only viable proxy available to approximate such information.
				pkgUsageMetric, coreUsageMetric, dramUsageMetric = config.CPUTime, config.CPUTime, config.CPUTime
			}
			// ContainerFeatureNames contains the metrics that represents the container resource utilization
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
		} else if powerSourceTarget == config.ContainerPlatformPowerKey {
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

func CreateContainerPowerEstimatorModel(containerFeatureNames, systemMetaDataFeatureNames, systemMetaDataFeatureValues []string) {
	var err error
	modelConfig := createContainerPowerModelConfig(config.ContainerPlatformPowerKey, containerFeatureNames, systemMetaDataFeatureNames, systemMetaDataFeatureValues, types.PlatformEnergySource)
	modelConfig.IsNodePowerModel = false
	ContainerPlatformPowerModel, err = createPowerModelEstimator(modelConfig)
	if err == nil {
		klog.Infof("Using the %s Power Model to estimate Container Platform Power", modelConfig.ModelType.String()+"/"+modelConfig.ModelOutputType.String())
		klog.Infof("Container feature names: %v", modelConfig.ContainerFeatureNames)
	} else {
		klog.Infof("Failed to create %s Power Model to estimate Container Platform Power: %v\n", modelConfig.ModelType.String()+"/"+modelConfig.ModelOutputType.String(), err)
	}

	modelConfig = createContainerPowerModelConfig(config.ContainerComponentsPowerKey, containerFeatureNames, systemMetaDataFeatureNames, systemMetaDataFeatureValues, types.ComponentEnergySource)
	modelConfig.IsNodePowerModel = false
	ContainerComponentPowerModel, err = createPowerModelEstimator(modelConfig)
	if err == nil {
		klog.Infof("Using the %s Power Model to estimate Container Component Power", modelConfig.ModelType.String()+"/"+modelConfig.ModelOutputType.String())
		klog.Infof("Container feature names: %v", modelConfig.ContainerFeatureNames)
	} else {
		klog.Infof("Failed to create %s Power Model to estimate Container Component Power: %v\n", modelConfig.ModelType.String()+"/"+modelConfig.ModelOutputType.String(), err)
	}
}

// UpdateContainerEnergy resets the power model samples, add new samples to the power models, then estimates the idle and dynamic energy
func UpdateContainerEnergy(containersMetrics map[string]*collector_metric.ContainerMetrics, nodeMetrics *collector_metric.NodeMetrics) {
	if ContainerPlatformPowerModel == nil {
		klog.Errorln("Container Platform Power Model was not created")
	}
	if ContainerComponentPowerModel == nil {
		klog.Errorln("Container Component Power Model was not created")
	}
	// reset power sample slide window
	ContainerPlatformPowerModel.ResetSampleIdx()
	ContainerComponentPowerModel.ResetSampleIdx()

	// add features values for prediction
	containerIDList := addSamplesToPowerModels(containersMetrics, nodeMetrics)

	addEstimatedEnergy(containerIDList, containersMetrics, idlePower)
	addEstimatedEnergy(containerIDList, containersMetrics, absPower)
}

// addSamplesToPowerModels converts container's metrics to array to add the samples to the power model
func addSamplesToPowerModels(containersMetrics map[string]*collector_metric.ContainerMetrics, nodeMetrics *collector_metric.NodeMetrics) []string {
	containerIDList := []string{}
	// Add container metrics
	for containerID, c := range containersMetrics {
		// add samples to estimate the platform power
		if ContainerPlatformPowerModel.IsEnabled() {
			featureValues := c.ToEstimatorValues(ContainerPlatformPowerModel.GetContainerFeatureNamesList(), true) // add container features with normalized values
			ContainerPlatformPowerModel.AddContainerFeatureValues(featureValues)
		}

		// add samples to estimate the components (CPU and DRAM) power
		if ContainerComponentPowerModel.IsEnabled() {
			// Add container metrics
			featureValues := c.ToEstimatorValues(ContainerComponentPowerModel.GetContainerFeatureNamesList(), true) // add node features with normalized values
			ContainerComponentPowerModel.AddContainerFeatureValues(featureValues)
		}

		containerIDList = append(containerIDList, containerID)
	}
	// Add node metrics.
	if ContainerPlatformPowerModel.IsEnabled() {
		featureValues := nodeMetrics.ToEstimatorValues(ContainerPlatformPowerModel.GetNodeFeatureNamesList(), true) // add node features with normalized values
		ContainerPlatformPowerModel.AddNodeFeatureValues(featureValues)
	}
	if ContainerComponentPowerModel.IsEnabled() {
		featureValues := nodeMetrics.ToEstimatorValues(ContainerComponentPowerModel.GetNodeFeatureNamesList(), true) // add node features with normalized values
		ContainerComponentPowerModel.AddNodeFeatureValues(featureValues)
	}
	return containerIDList
}

// addEstimatedEnergy estimates the idle power consumption
func addEstimatedEnergy(containerIDList []string, containersMetrics map[string]*collector_metric.ContainerMetrics, isIdlePower bool) {
	var err error
	var containerGPUPower []float64
	var containerPlatformPower []float64
	var containerComponentsPower []source.NodeComponentsEnergy

	errComp := fmt.Errorf("component power model is not enabled")
	errGPU := fmt.Errorf("gpu power model is not enabled")
	errPlat := fmt.Errorf("plat power model is not enabled")

	// estimate the associated power comsumption of all RAPL node components for each container
	if ContainerComponentPowerModel.IsEnabled() {
		containerComponentsPower, errComp = ContainerComponentPowerModel.GetComponentsPower(isIdlePower)
		if errComp != nil {
			klog.V(5).Infoln("Could not estimate the Container Components Power")
		}
		// estimate the associated power comsumption of GPU for each container
		if gpu.IsGPUCollectionSupported() {
			containerGPUPower, errGPU = ContainerComponentPowerModel.GetGPUPower(isIdlePower)
			if errGPU != nil {
				klog.V(5).Infoln("Could not estimate the Container GPU Power")
			}
		}
	}
	// estimate the associated power comsumption of platform for each container
	if ContainerPlatformPowerModel.IsEnabled() {
		containerPlatformPower, errPlat = ContainerPlatformPowerModel.GetPlatformPower(isIdlePower)
		if errPlat != nil {
			klog.V(5).Infoln("Could not estimate the Container Platform Power")
		}
	}

	var energy uint64
	for i, containerID := range containerIDList {
		if errComp == nil {
			// add PKG power consumption
			// since Kepler collects metrics at intervals of SamplePeriodSec, which is greater than 1 second, it is necessary to calculate the energy consumption for the entire waiting period
			energy = containerComponentsPower[i].Pkg * config.SamplePeriodSec
			if isIdlePower {
				err = containersMetrics[containerID].IdleEnergyInPkg.AddNewDelta(energy)
			} else {
				err = containersMetrics[containerID].DynEnergyInPkg.AddNewDelta(energy)
			}
			if err != nil {
				klog.V(5).Infoln(err)
			}

			// add CORE power consumption
			energy = containerComponentsPower[i].Core * config.SamplePeriodSec
			if isIdlePower {
				err = containersMetrics[containerID].IdleEnergyInCore.AddNewDelta(energy)
			} else {
				err = containersMetrics[containerID].DynEnergyInCore.AddNewDelta(energy)
			}
			if err != nil {
				klog.V(5).Infoln(err)
			}

			// add DRAM power consumption
			energy = containerComponentsPower[i].DRAM * config.SamplePeriodSec
			if isIdlePower {
				err = containersMetrics[containerID].IdleEnergyInDRAM.AddNewDelta(energy)
			} else {
				err = containersMetrics[containerID].DynEnergyInDRAM.AddNewDelta(energy)
			}
			if err != nil {
				klog.V(5).Infoln(err)
			}

			// add Uncore power consumption
			energy = containerComponentsPower[i].Uncore * config.SamplePeriodSec
			if isIdlePower {
				err = containersMetrics[containerID].IdleEnergyInUncore.AddNewDelta(energy)
			} else {
				err = containersMetrics[containerID].DynEnergyInUncore.AddNewDelta(energy)
			}
			if err != nil {
				klog.V(5).Infoln(err)
			}

			// add GPU power consumption
			if errGPU == nil {
				energy = uint64(containerGPUPower[i]) * (config.SamplePeriodSec)
				if isIdlePower {
					err = containersMetrics[containerID].IdleEnergyInGPU.AddNewDelta(energy)
				} else {
					err = containersMetrics[containerID].DynEnergyInGPU.AddNewDelta(energy)
				}
				if err != nil {
					klog.V(5).Infoln(err)
				}
			}
		}

		if errPlat == nil {
			energy = uint64(containerPlatformPower[i]) * config.SamplePeriodSec
			if isIdlePower {
				err = containersMetrics[containerID].IdleEnergyInPlatform.AddNewDelta(energy)
			} else {
				err = containersMetrics[containerID].DynEnergyInPlatform.AddNewDelta(energy)
			}
			if err != nil {
				klog.V(5).Infoln(err)
			}
		}

		// estimate other components power if both platform and components power are available
		if errComp == nil && errPlat == nil {
			// TODO: verify if Platform power also includes the GPU into consideration
			otherPower := containerPlatformPower[i] - float64(containerComponentsPower[i].Pkg) - float64(containerComponentsPower[i].DRAM)
			if otherPower < 0 {
				otherPower = 0
			}
			energy = uint64(otherPower) * config.SamplePeriodSec
			if isIdlePower {
				err = containersMetrics[containerID].IdleEnergyInOther.AddNewDelta(energy)
			} else {
				err = containersMetrics[containerID].DynEnergyInOther.AddNewDelta(energy)
			}
			if err != nil {
				klog.V(5).Infoln(err)
			}
		}
	}
}
