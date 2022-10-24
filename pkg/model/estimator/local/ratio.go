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

/*
ratio.go
calculate Pods' component and other power by ratio approach when node power is available.
*/

package local

import (
	"math"

	"github.com/sustainable-computing-io/kepler/pkg/config"
	"github.com/sustainable-computing-io/kepler/pkg/power/rapl/source"
)

var (
	coreMetricIndex    int = -1
	dramMetricIndex    int = -1
	uncoreMetricIndex  int = -1
	generalMetricIndex int = -1
)

func getSumMetricValues(podMetricValues [][]float64) (sumMetricValues []float64) {
	if len(podMetricValues) == 0 {
		return
	}
	sumMetricValues = make([]float64, len(podMetricValues[0]))
	for _, values := range podMetricValues {
		for index, podMetricValue := range values {
			sumMetricValues[index] += podMetricValue
		}
	}
	return
}

func InitMetricIndexes(metricNames []string) {
	for index, metricName := range metricNames {
		if metricName == config.CoreUsageMetric {
			coreMetricIndex = index
		}
		if metricName == config.DRAMUsageMetric {
			dramMetricIndex = index
		}
		if metricName == config.UncoreUsageMetric {
			uncoreMetricIndex = index
		}
		if metricName == config.GeneralUsageMetric {
			generalMetricIndex = index
		}
	}
}

func getRatio(podMetricValue []float64, metricIndex int, totalPower uint64, podNumber float64, sumMetricValues []float64) uint64 {
	var power float64
	if metricIndex >= 0 && sumMetricValues[metricIndex] > 0 {
		power = podMetricValue[metricIndex] / sumMetricValues[metricIndex] * float64(totalPower)
	} else {
		power = float64(totalPower) / podNumber
	}
	return uint64(math.Ceil(power))
}

func GetPodPowerRatio(podMetricValues [][]float64, otherNodePower uint64, nodeComponentPower source.RAPLPower) (componentPowers []source.RAPLPower, otherPodPowers []uint64) {
	sumMetricValues := getSumMetricValues(podMetricValues)
	podNumber := len(podMetricValues)
	componentPowers = make([]source.RAPLPower, podNumber)
	otherPodPowers = make([]uint64, podNumber)
	podNumberDivision := float64(podNumber)

	// Package (PKG) domain measures the energy consumption of the entire socket, including the consumption of all the cores, integrated graphics and
	// also the "unknown" components such as last level caches and memory controllers
	pkgUnknownValue := nodeComponentPower.Pkg - nodeComponentPower.Core - nodeComponentPower.Uncore

	// find ratio power
	for index, podMetricValue := range podMetricValues {
		coreValue := getRatio(podMetricValue, coreMetricIndex, nodeComponentPower.Core, podNumberDivision, sumMetricValues)
		uncoreValue := getRatio(podMetricValue, uncoreMetricIndex, nodeComponentPower.Uncore, podNumberDivision, sumMetricValues)
		unknownValue := getRatio(podMetricValue, generalMetricIndex, pkgUnknownValue, podNumberDivision, sumMetricValues)
		dramValue := getRatio(podMetricValue, dramMetricIndex, nodeComponentPower.DRAM, podNumberDivision, sumMetricValues)
		otherValue := getRatio(podMetricValue, generalMetricIndex, otherNodePower, podNumberDivision, sumMetricValues)
		pkgValue := coreValue + uncoreValue + unknownValue
		componentPowers[index] = source.RAPLPower{
			Pkg:    pkgValue,
			Core:   coreValue,
			Uncore: uncoreValue,
			DRAM:   dramValue,
		}
		otherPodPowers[index] = otherValue
	}
	return
}
