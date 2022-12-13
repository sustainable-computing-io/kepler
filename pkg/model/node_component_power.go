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
	"github.com/sustainable-computing-io/kepler/pkg/power/components/source"
)

var (
	NodeComponentPowerModelEnabled bool
	NodeComponentPowerModelFunc    func([][]float64, []string) (map[string][]float64, error)

	defaultAbsCompURL = "https://raw.githubusercontent.com/sustainable-computing-io/kepler-model-server/main/tests/test_models/AbsComponentModelWeight/Full/KerasCompWeightFullPipeline/KerasCompWeightFullPipeline.json"
)

// initContainerComponentPowerModelConfig: the container component power model must be set by default.
func initNodeComponentPowerModelConfig() types.ModelConfig {
	modelConfig := InitModelConfig(config.NodeComponentsKey)
	if modelConfig.InitModelURL == "" {
		modelConfig.InitModelURL = defaultAbsCompURL
	}
	return modelConfig
}

func InitNodeComponentPowerEstimator(usageMetrics, systemFeatures, systemValues []string) {
	var estimateFunc interface{}
	nodeComponentPowerModelConfig := initNodeComponentPowerModelConfig()
	// init func for NodeComponentPower
	NodeComponentPowerModelEnabled, estimateFunc = initEstimateFunction(nodeComponentPowerModelConfig, types.AbsComponentPower, types.AbsComponentModelWeight, usageMetrics, systemFeatures, systemValues, false)
	if NodeComponentPowerModelEnabled {
		NodeComponentPowerModelFunc = estimateFunc.(func([][]float64, []string) (map[string][]float64, error))
	}
}

// IsNodeComponentPowerModelEnabled returns if the estimator has been enabled or not
func IsNodeComponentPowerModelEnabled() bool {
	return NodeComponentPowerModelEnabled
}

// GetNodeTotalEnergy returns estimated RAPL power
// func GetNodeComponentPowers(usageValues []float64, systemValues []string) (results source.RAPLPower) {
func GetNodeComponentPowers(nodeMetrics collector_metric.NodeMetrics) (nodeComponentsEnergy map[int]source.NodeComponentsEnergy) {
	nodeComponentsEnergy = map[int]source.NodeComponentsEnergy{}
	// TODO: make the estimator also retrieve the socket ID, we are estimating that the node will have only socket
	socketID := 0
	if NodeComponentPowerModelEnabled {
		nodeMetricResourceUsageValuesOnly := nodeMetricsToArray(nodeMetrics)
		powers, err := NodeComponentPowerModelFunc(nodeMetricResourceUsageValuesOnly, collector_metric.NodeMetadataValues)
		if err != nil {
			return
		}
		pkgPower := getComponentPower(powers, "pkg", socketID)
		corePower := getComponentPower(powers, "core", socketID)
		uncorePower := getComponentPower(powers, "uncore", socketID)
		dramPower := getComponentPower(powers, "dram", socketID)
		nodeComponentsEnergy[socketID] = fillRAPLPower(pkgPower, corePower, uncorePower, dramPower)
		return
	}
	return
}
