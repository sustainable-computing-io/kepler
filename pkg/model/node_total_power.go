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
	"github.com/sustainable-computing-io/kepler/pkg/model/types"

	collector_metric "github.com/sustainable-computing-io/kepler/pkg/collector/metric"
)

const (
	estimatorACPISensorID string = "estimator"
)

var (
	NodePlatformPowerModelEnabled bool
	NodeTotalPowerModelFunc       func([][]float64, []string) ([]float64, error)
)

func InitNodeTotalPowerEstimator(usageMetrics, systemFeatures, systemValues []string) {
	var estimateFunc interface{}
	nodePlatformPowerModelConfig := InitModelConfig(config.NodeTotalKey)
	// init func for NodeTotalPower
	NodePlatformPowerModelEnabled, estimateFunc = initEstimateFunction(nodePlatformPowerModelConfig, types.AbsPower, types.AbsModelWeight, usageMetrics, systemFeatures, systemValues, true)
	if NodePlatformPowerModelEnabled {
		NodeTotalPowerModelFunc = estimateFunc.(func([][]float64, []string) ([]float64, error))
	}
}

// IsNodePlatformPowerModelEnabled returns if the estimator has been enabled or not
func IsNodePlatformPowerModelEnabled() bool {
	return NodePlatformPowerModelEnabled
}

// GetNodeTotalEnergy returns a single estimated value of node total power
func GetEstimatedNodePlatformPower(nodeMetrics collector_metric.NodeMetrics) (platformEnergy map[string]float64) {
	platformEnergy = map[string]float64{}
	platformEnergy[estimatorACPISensorID] = 0
	if NodePlatformPowerModelEnabled {
		// convert the resource usage map to an array since the model server does not receive structured data
		nodeMetricResourceUsageValuesOnly := nodeMetricsToArray(nodeMetrics)
		powers, err := NodeTotalPowerModelFunc(nodeMetricResourceUsageValuesOnly, collector_metric.NodeMetadataValues)
		if err != nil || len(powers) == 0 {
			return
		}
		platformEnergy[estimatorACPISensorID] = powers[0]
		return
	}
	return
}
