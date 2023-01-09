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
	"github.com/sustainable-computing-io/kepler/pkg/config"
	"github.com/sustainable-computing-io/kepler/pkg/model/estimator/local"
	"github.com/sustainable-computing-io/kepler/pkg/model/estimator/sidecar"
	"github.com/sustainable-computing-io/kepler/pkg/model/types"
	"k8s.io/klog/v2"
)

var (
	EstimatorSidecarSocket = "/tmp/estimator.sock"
)

// InitEstimateFunctions checks validity of power model and set estimate functions
func InitEstimateFunctions(usageMetrics, systemFeatures, systemValues []string) {
	config.InitModelConfigMap()
	InitNodeTotalPowerEstimator(usageMetrics, systemFeatures, systemValues)
	InitNodeComponentPowerEstimator(usageMetrics, systemFeatures, systemValues)
	InitContainerPowerEstimator(usageMetrics, systemFeatures, systemValues)
}

// initEstimateFunction called by InitEstimateFunctions to initiate estimate function for each power model
func initEstimateFunction(modelConfig types.ModelConfig, archiveType, modelWeightType types.ModelOutputType, usageMetrics, systemFeatures, systemValues []string, isTotalPower bool) (valid bool, estimateFunc interface{}) {
	if modelConfig.UseEstimatorSidecar {
		// init EstimatorSidecarConnector
		c := sidecar.EstimatorSidecarConnector{
			Socket:         EstimatorSidecarSocket,
			UsageMetrics:   usageMetrics,
			OutputType:     archiveType,
			SystemFeatures: systemFeatures,
			ModelName:      modelConfig.SelectedModel,
			SelectFilter:   modelConfig.SelectFilter,
		}
		valid = c.Init(systemValues)
		if valid {
			if isTotalPower {
				estimateFunc = c.GetTotalPower
			} else {
				estimateFunc = c.GetComponentPower
			}
		}
		klog.V(3).Infof("Model %s initiated (%v)", archiveType.String(), valid)
	} else {
		// init LinearRegressor
		r := local.LinearRegressor{
			Endpoint:       config.ModelServerEndpoint,
			UsageMetrics:   usageMetrics,
			OutputType:     modelWeightType,
			SystemFeatures: systemFeatures,
			ModelName:      modelConfig.SelectedModel,
			SelectFilter:   modelConfig.SelectFilter,
			InitModelURL:   modelConfig.InitModelURL,
		}
		valid = r.Init()
		if isTotalPower {
			estimateFunc = r.GetTotalPower
		} else {
			estimateFunc = r.GetComponentPower
		}
		klog.V(3).Infof("Model %s initiated (%v)", modelWeightType.String(), valid)
	}
	return valid, estimateFunc
}

func InitModelConfig(modelItem string) types.ModelConfig {
	useEstimatorSidecar, selectedModel, selectFilter, initModelURL := config.GetModelConfig(modelItem)
	modelConfig := types.ModelConfig{UseEstimatorSidecar: useEstimatorSidecar, SelectedModel: selectedModel, SelectFilter: selectFilter, InitModelURL: initModelURL}
	klog.V(3).Infof("Model Config %s: %v", modelItem, modelConfig)
	return modelConfig
}
