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

	"github.com/sustainable-computing-io/kepler/pkg/collector/stats"
	"github.com/sustainable-computing-io/kepler/pkg/config"
	"github.com/sustainable-computing-io/kepler/pkg/model/types"
	"github.com/sustainable-computing-io/kepler/pkg/node"
	"github.com/sustainable-computing-io/kepler/pkg/sensors/components"
	"github.com/sustainable-computing-io/kepler/pkg/sensors/components/source"
	"k8s.io/klog/v2"
)

// the absolute power model includes both the absolute and idle power consumption
var nodeComponentPowerModel PowerModelInterface

// createNodeComponentPowerModelConfig: the node component power model url must be set by default.
func createNodeComponentPowerModelConfig(nodeFeatureNames []string) *types.ModelConfig {
	systemMetaDataFeatureNames := node.MetadataFeatureNames()
	systemMetaDataFeatureValues := node.MetadataFeatureValues()
	modelConfig := CreatePowerModelConfig(config.NodeComponentsPowerKey())
	if modelConfig.InitModelURL == "" {
		modelConfig.InitModelFilepath = config.GetDefaultPowerModelURL(modelConfig.ModelOutputType.String(), types.ComponentEnergySource)
	}
	modelConfig.NodeFeatureNames = nodeFeatureNames
	modelConfig.SystemMetaDataFeatureNames = systemMetaDataFeatureNames
	modelConfig.SystemMetaDataFeatureValues = systemMetaDataFeatureValues
	modelConfig.IsNodePowerModel = true
	return modelConfig
}

// CreateNodeComponentPowerEstimatorModel only create a new power model estimator if node components power metrics are not available
func CreateNodeComponentPowerEstimatorModel(nodeFeatureNames []string) {
	var err error
	if !components.IsSystemCollectionSupported() {
		modelConfig := createNodeComponentPowerModelConfig(nodeFeatureNames)
		// init func for NodeComponentPower
		nodeComponentPowerModel, err = createPowerModelEstimator(modelConfig)
		if err == nil {
			klog.V(1).Infof("Using the %s Power Model to estimate Node Component Power", modelConfig.ModelType.String()+"/"+modelConfig.ModelOutputType.String())
		} else {
			klog.Infof("Failed to create %s Power Model to estimate Node Component Power: %v\n", modelConfig.ModelType.String()+"/"+modelConfig.ModelOutputType.String(), err)
		}
	} else {
		klog.Infof("Skipping creation of Node Component Power Model since the system collection is supported")
		return
	}
}

// IsNodeComponentPowerModelEnabled returns if the estimator has been enabled or not
func IsNodeComponentPowerModelEnabled() bool {
	if nodeComponentPowerModel == nil {
		return false
	}
	return nodeComponentPowerModel.IsEnabled()
}

// GetNodeComponentPowers returns estimated RAPL power for the node
func GetNodeComponentPowers(nodeMetrics *stats.NodeStats, isIdlePower bool) (nodeComponentsEnergy map[int]source.NodeComponentsEnergy) {
	if nodeComponentPowerModel == nil {
		klog.Errorln("Node Component Power Model was not created")
	}
	nodeComponentsEnergy = map[int]source.NodeComponentsEnergy{}
	// assuming that the absolute power is always called before estimating idle power, we only add feature values for absolute power as it also initialize the idle feature values
	if !isIdlePower {
		// reset power model features sample list for new estimation
		nodeComponentPowerModel.ResetSampleIdx()
		featureValues := nodeMetrics.ToEstimatorValues(nodeComponentPowerModel.GetNodeFeatureNamesList(), true) // add container features with normalized values
		nodeComponentPowerModel.AddNodeFeatureValues(featureValues)                                             // add samples to estimation
	}
	powers, err := nodeComponentPowerModel.GetComponentsPower(isIdlePower)
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

// UpdateNodeComponentEnergy sets the power model samples, get absolute powers, and set gauge value for each component energy
func UpdateNodeComponentEnergy(nodeMetrics *stats.NodeStats) {
	addEnergy(nodeMetrics, nodeMetrics.AbsEnergyMetrics(), absPower)
}

// UpdateNodeComponentIdleEnergy sets the power model samples to zeros, get idle powers, and set gauge value for each component idle energy
func UpdateNodeComponentIdleEnergy(nodeMetrics *stats.NodeStats) {
	addEnergy(nodeMetrics, nodeMetrics.IdleEnergyMetrics(), idlePower)
}

func addEnergy(nodeMetrics *stats.NodeStats, metrics []string, isIdle bool) {
	for socket, power := range GetNodeComponentPowers(nodeMetrics, isIdle) {
		strID := fmt.Sprintf("%d", socket)
		nodeMetrics.EnergyUsage[metrics[0]].SetDeltaStat(strID, power.Core*config.SamplePeriodSec())
		nodeMetrics.EnergyUsage[metrics[1]].SetDeltaStat(strID, power.DRAM*config.SamplePeriodSec())
		nodeMetrics.EnergyUsage[metrics[2]].SetDeltaStat(strID, power.Uncore*config.SamplePeriodSec())
		nodeMetrics.EnergyUsage[metrics[3]].SetDeltaStat(strID, power.Pkg*config.SamplePeriodSec())
	}
}
