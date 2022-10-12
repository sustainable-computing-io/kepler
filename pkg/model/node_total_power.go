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
	"github.com/sustainable-computing-io/kepler/pkg/model/types"
)

var (
	NodeTotalPowerModelValid bool
	NodeTotalPowerModelFunc  func([][]float64, []string) ([]float64, error)

	// TODO: be configured by config package
	NodeTotalPowerModelConfig types.ModelConfig = types.ModelConfig{UseEstimatorSidecar: false}
)

func InitNodeTotalPowerEstimator(usageMetrics, systemFeatures, systemValues []string) {
	var estimateFunc interface{}
	// init func for NodeTotalPower
	NodeTotalPowerModelValid, estimateFunc = initEstimateFunction(NodeTotalPowerModelConfig, types.AbsPower, types.AbsModelWeight, usageMetrics, systemFeatures, systemValues, true)
	if NodeTotalPowerModelValid {
		NodeTotalPowerModelFunc = estimateFunc.(func([][]float64, []string) ([]float64, error))
	}
}

// GetNodeTotalPower returns a single estimated value of node total power
func GetNodeTotalPower(usageValues []float64, systemValues []string) (valid bool, value uint64) {
	valid = false
	value = 0
	if NodeTotalPowerModelValid {
		powers, err := NodeTotalPowerModelFunc([][]float64{usageValues}, systemValues)
		if err != nil || len(powers) == 0 {
			return
		}
		valid = true
		value = uint64(powers[0])
		return
	}
	return
}
