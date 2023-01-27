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
	"github.com/sustainable-computing-io/kepler/pkg/power/components/source"
)

const (
	jouleToMiliJoule = 1000
)

// getComponentPower called by getPodComponentPowers to check if component key is present in powers response and fills with single 0
func getComponentPower(powers map[string][]float64, componentKey string, index int) uint64 {
	values := powers[componentKey]
	if index >= len(values) {
		return 0
	} else {
		return uint64(values[index] * jouleToMiliJoule)
	}
}

// fillRAPLPower fills missing component (pkg or core) power
func fillRAPLPower(pkgPower, corePower, uncorePower, dramPower uint64) source.NodeComponentsEnergy {
	if pkgPower < corePower+uncorePower {
		pkgPower = corePower + uncorePower
	}
	if corePower == 0 {
		corePower = pkgPower - uncorePower
	}
	return source.NodeComponentsEnergy{
		Core:   corePower,
		Uncore: uncorePower,
		DRAM:   dramPower,
		Pkg:    pkgPower,
	}
}

// containerMetricsToArray converts to container metrics map to array
// The current implementation from the model server use this arry and returns an array with the container energy.
// The list follows the order of the container list for the container id...
// TODO: make model server return a list of elemets that also contains the containerID to enforce consistency
func containerMetricsToArray(containersMetrics map[string]*collector_metric.ContainerMetrics) (containerMetricValuesOnly [][]float64, containerIDList []string) {
	for containerID, c := range containersMetrics {
		values := c.ToEstimatorValues()
		containerMetricValuesOnly = append(containerMetricValuesOnly, values)
		containerIDList = append(containerIDList, containerID)
	}
	return
}

// TODO: as in containerMetricsToArray, consider exchange a protobuf stricture istead of simple arrays to make it more predictable and consistent
func nodeMetricsToArray(nodeMetrics *collector_metric.NodeMetrics) [][]float64 {
	nodeMetricResourceUsageValuesOnly := []float64{}
	for _, metricName := range collector_metric.ContainerMetricNames {
		nodeMetricResourceUsageValuesOnly = append(nodeMetricResourceUsageValuesOnly, nodeMetrics.ResourceUsage[metricName])
	}
	// the estimator expect a matrix
	return [][]float64{nodeMetricResourceUsageValuesOnly}
}

// processMetricsToArray converts to process metrics map to array
func processMetricsToArray(processMetrics map[uint64]*collector_metric.ProcessMetrics) (processMetricValuesOnly [][]float64, pidList []uint64) {
	for pid, c := range processMetrics {
		values := c.ToEstimatorValues()
		processMetricValuesOnly = append(processMetricValuesOnly, values)
		pidList = append(pidList, pid)
	}
	return
}
