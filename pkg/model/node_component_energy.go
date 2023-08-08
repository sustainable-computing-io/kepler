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
	"github.com/sustainable-computing-io/kepler/pkg/model/types"
	"github.com/sustainable-computing-io/kepler/pkg/power/components"
	"github.com/sustainable-computing-io/kepler/pkg/power/components/source"
	"k8s.io/klog/v2"
)

var (
	// the absulute power model includes both the absolute and idle power consumption
	NodeComponentPowerModel PowerMoldelInterface

	defaultAbsCompURL = "/var/lib/kepler/data/KerasCompWeightFullPipeline.json"
)

// createNodeComponentPowerModelConfig: the node component power model url must be set by default.
func createNodeComponentPowerModelConfig(nodeFeatureNames, systemMetaDataFeatureNames, systemMetaDataFeatureValues []string) *types.ModelConfig {
	modelConfig := CreatePowerModelConfig(config.NodeComponentsPowerKey)
	if modelConfig.InitModelURL == "" {
		modelConfig.InitModelURL = defaultAbsCompURL
	}
	modelConfig.NodeFeatureNames = nodeFeatureNames
	modelConfig.SystemMetaDataFeatureNames = systemMetaDataFeatureNames
	modelConfig.SystemMetaDataFeatureValues = systemMetaDataFeatureValues
	modelConfig.IsNodePowerModel = true
	return modelConfig
}

// CreateNodeComponentPoweEstimatorModel only create a new power model estimater if node components power metrics are not available
func CreateNodeComponentPoweEstimatorModel(nodeFeatureNames, systemMetaDataFeatureNames, systemMetaDataFeatureValues []string) {
	var err error
	if !components.IsSystemCollectionSupported() {
		modelConfig := createNodeComponentPowerModelConfig(nodeFeatureNames, systemMetaDataFeatureNames, systemMetaDataFeatureValues)
		// init func for NodeComponentPower
		NodeComponentPowerModel, err = createPowerModelEstimator(modelConfig)
		if err == nil {
			klog.Infof("Using the %s Power Model to estimate Node Component Power", modelConfig.ModelType.String()+"/"+modelConfig.ModelOutputType.String())
		} else {
			klog.Infof("Failed to create %s Power Model to estimate Node Component Power: %v\n", modelConfig.ModelType.String()+"/"+modelConfig.ModelOutputType.String(), err)
		}
	}
}

// IsNodeComponentPowerModelEnabled returns if the estimator has been enabled or not
func IsNodeComponentPowerModelEnabled() bool {
	if NodePlatformPowerModel == nil {
		return false
	}
	return NodeComponentPowerModel.IsEnabled()
}

// GetNodeComponentPowers returns estimated RAPL power for the node
func GetNodeComponentPowers(nodeMetrics *collector_metric.NodeMetrics, isIdlePower bool) (nodeComponentsEnergy map[int]source.NodeComponentsEnergy) {
	if NodeComponentPowerModel == nil {
		klog.Errorln("Node Component Power Model was not created")
	}
	nodeComponentsEnergy = map[int]source.NodeComponentsEnergy{}
	// assuming that the absolute power is always called before estimating idle power, we only add feature values for absolute power as it also initialize the idle feature values
	if !isIdlePower {
		// reset power model features sample list for new estimation
		NodeComponentPowerModel.ResetSampleIdx()
		featureValues := nodeMetrics.ToEstimatorValues(NodeComponentPowerModel.GetNodeFeatureNamesList(), true) // add container features with normalized values
		NodeComponentPowerModel.AddNodeFeatureValues(featureValues)                                             // add samples to estimation
	}
	powers, err := NodeComponentPowerModel.GetComponentsPower(isIdlePower)
	if err != nil {
		klog.Infof("Failed to get node components power %v\n", err)
		return
	}
	// TODO: Estimate the power per socket. Right now we send the aggregated values for all sockets
	for socketID, values := range powers {
		nodeComponentsEnergy[socketID] = values
	}
	return
}

// UpdateNodeComponentEnergy resets the power model samples, add new samples to the power models, then estimates the idle and absolute energy
func UpdateNodeComponentEnergy(nodeMetrics *collector_metric.NodeMetrics) {
	componentPower := GetNodeComponentPowers(nodeMetrics, absPower)
	for id := range componentPower {
		var ok bool
		var power source.NodeComponentsEnergy
		if power, ok = componentPower[id]; !ok {
			continue
		}
		// convert power to energy
		power.Pkg *= config.SamplePeriodSec
		power.Core *= config.SamplePeriodSec
		power.DRAM *= config.SamplePeriodSec
		power.Uncore *= config.SamplePeriodSec
		power.Core *= config.SamplePeriodSec
		componentPower[id] = power
	}
	nodeMetrics.SetNodeComponentsEnergy(componentPower, gauge, absPower)

	componentPower = GetNodeComponentPowers(nodeMetrics, idlePower)
	for id := range componentPower {
		var ok bool
		var power source.NodeComponentsEnergy
		if power, ok = componentPower[id]; !ok {
			continue
		}
		// convert power to energy
		power.Pkg *= config.SamplePeriodSec
		power.Core *= config.SamplePeriodSec
		power.DRAM *= config.SamplePeriodSec
		power.Uncore *= config.SamplePeriodSec
		power.Core *= config.SamplePeriodSec
		componentPower[id] = power
	}
	nodeMetrics.SetNodeComponentsEnergy(componentPower, gauge, idlePower)
	// After the node component idle and absulute energy was updated, we need to update the dynamic power
	nodeMetrics.UpdateDynEnergy()
}
