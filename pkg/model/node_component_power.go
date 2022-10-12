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
	"github.com/sustainable-computing-io/kepler/pkg/power/rapl/source"
)

var (
	NodeComponentPowerModelValid bool
	NodeComponentPowerModelFunc  func([][]float64, []string) (map[string][]float64, error)

	// TODO: be configured by config package
	NodeComponentPowerModelConfig types.ModelConfig = types.ModelConfig{UseEstimatorSidecar: false}
)

func InitNodeComponentPowerEstimator(usageMetrics, systemFeatures, systemValues []string) {
	var estimateFunc interface{}
	// init func for NodeComponentPower
	NodeComponentPowerModelValid, estimateFunc = initEstimateFunction(NodeComponentPowerModelConfig, types.AbsComponentPower, types.AbsComponentModelWeight, usageMetrics, systemFeatures, systemValues, false)
	if NodeComponentPowerModelValid {
		NodeComponentPowerModelFunc = estimateFunc.(func([][]float64, []string) (map[string][]float64, error))
	}
}

// GetNodeTotalPower returns estimated RAPL power
func GetNodeComponentPowers(usageValues []float64, systemValues []string) (valid bool, results source.RAPLPower) {
	results = source.RAPLPower{}
	valid = false
	if NodeComponentPowerModelValid {
		powers, err := NodeComponentPowerModelFunc([][]float64{usageValues}, systemValues)
		if err != nil {
			return
		}
		pkgPower := getComponentPower(powers, "pkg", 0)
		corePower := getComponentPower(powers, "core", 0)
		uncorePower := getComponentPower(powers, "uncore", 0)
		dramPower := getComponentPower(powers, "dram", 0)
		valid = true
		results = fillRAPLPower(pkgPower, corePower, uncorePower, dramPower)
		return
	}
	return
}
