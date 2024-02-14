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
	"github.com/sustainable-computing-io/kepler/pkg/sensors/platform"
	"k8s.io/klog/v2"
)

const (
	estimatorACPISensorID string = "estimator"
)

var (
	NodePlatformPowerModel PowerModelInterface
)

// CreateNodeComponentPoweEstimatorModel only create a new power model estimater if node platform power metrics are not available
func CreateNodePlatformPoweEstimatorModel(nodeFeatureNames, systemMetaDataFeatureNames, systemMetaDataFeatureValues []string) {
	if !platform.IsSystemCollectionSupported() {
		modelConfig := CreatePowerModelConfig(config.NodePlatformPowerKey)
		if modelConfig.InitModelURL == "" {
			modelConfig.InitModelFilepath = config.GetDefaultPowerModelURL(modelConfig.ModelOutputType.String(), types.PlatformEnergySource)
		}
		modelConfig.NodeFeatureNames = nodeFeatureNames
		modelConfig.SystemMetaDataFeatureNames = systemMetaDataFeatureNames
		modelConfig.SystemMetaDataFeatureValues = systemMetaDataFeatureValues
		modelConfig.IsNodePowerModel = true
		// init func for NodeTotalPower
		var err error
		NodePlatformPowerModel, err = createPowerModelEstimator(modelConfig)
		if err == nil {
			klog.V(1).Infof("Using the %s Power Model to estimate Node Platform Power", modelConfig.ModelType.String()+"/"+modelConfig.ModelOutputType.String())
		} else {
			klog.Infof("Failed to create %s Power Model to estimate Node Platform Power: %v\n", modelConfig.ModelType.String()+"/"+modelConfig.ModelOutputType.String(), err)
		}
	}
}

// IsNodePlatformPowerModelEnabled returns if the estimator has been enabled or not
func IsNodePlatformPowerModelEnabled() bool {
	if NodePlatformPowerModel == nil {
		return false
	}
	return NodePlatformPowerModel.IsEnabled()
}

// GetNodePlatformPower returns a single estimated value of node total power
func GetNodePlatformPower(nodeMetrics *stats.NodeStats, isIdlePower bool) (platformEnergy map[string]float64) {
	if NodePlatformPowerModel == nil {
		klog.Errorln("Node Platform Power Model was not created")
	}
	platformEnergy = map[string]float64{}
	// reset power model features sample list for new estimation
	NodePlatformPowerModel.ResetSampleIdx()
	// converts to node metrics map to array to add the samples to the power model
	// the featureList is defined in the container power model file and the features varies accordinly to the selected power model
	featureValues := nodeMetrics.ToEstimatorValues(NodePlatformPowerModel.GetNodeFeatureNamesList(), true) // add container features with normalized values
	NodePlatformPowerModel.AddNodeFeatureValues(featureValues)                                             // add samples to estimation
	powers, err := NodePlatformPowerModel.GetPlatformPower(isIdlePower)
	if err != nil {
		klog.Infof("Failed to get node platform power %v\n", err)
		return
	}
	// TODO: Estimate the power per socket. Right now we send the aggregated values for all sockets
	for socketID, values := range powers {
		platformEnergy[estimatorACPISensorID+fmt.Sprint(socketID)] = values
	}
	return
}

// UpdateNodePlatformEnergy sets the power model samples, get absolute powers, and set platform energy
func UpdateNodePlatformEnergy(nodeMetrics *stats.NodeStats) {
	platformPower := GetNodePlatformPower(nodeMetrics, absPower)
	for sourceID, power := range platformPower {
		nodeMetrics.EnergyUsage[config.AbsEnergyInPlatform].SetDeltaStat(sourceID, uint64(power)*config.SamplePeriodSec)
	}
}

// UpdateNodePlatformIdleEnergy sets the power model samples to zeros, get idle powers, and set platform energy
func UpdateNodePlatformIdleEnergy(nodeMetrics *stats.NodeStats) {
	platformPower := GetNodePlatformPower(nodeMetrics, idlePower)
	for sourceID, power := range platformPower {
		nodeMetrics.EnergyUsage[config.IdleEnergyInPlatform].SetDeltaStat(sourceID, uint64(power)*config.SamplePeriodSec)
	}
}
