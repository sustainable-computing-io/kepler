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
	"github.com/sustainable-computing-io/kepler/pkg/model/estimator/local"
	"github.com/sustainable-computing-io/kepler/pkg/model/estimator/sidecar"
	"github.com/sustainable-computing-io/kepler/pkg/model/types"
)

var (
	EstimatorSidecarSocket = "/tmp/estimator.sock"

	// TODO: be configured by config package
	modelServerEndpoint = "http://kepler-model-server.monitoring.cluster.local:8100/model"
)

// InitEstimateFunctions checks validity of power model and set estimate functions
func InitEstimateFunctions(usageMetrics, systemFeatures, systemValues []string) {
	InitNodeTotalPowerEstimator(usageMetrics, systemFeatures, systemValues)
	InitNodeComponentPowerEstimator(usageMetrics, systemFeatures, systemValues)
	InitContainerPowerEstimator(usageMetrics, systemFeatures, systemValues)
}

// initEstimateFunction called by InitEstimateFunctions to initiate estimate function for each power model
func initEstimateFunction(modelConfig types.ModelConfig, archiveType, modelWeightType types.ModelOutputType, usageMetrics, systemFeatures, systemValues []string, isTotalPower bool) (valid bool, estimateFunc interface{}) {
	if modelConfig.UseEstimatorSidecar {
		// try init EstimatorSidecarConnector
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
			return
		}
	}
	// set UseEstimatorSidecar to false as cannot init valid EstimatorSidecarConnector
	modelConfig.UseEstimatorSidecar = false
	// try init LinearRegressor
	r := local.LinearRegressor{
		Endpoint:       modelServerEndpoint,
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
	return valid, estimateFunc
}
