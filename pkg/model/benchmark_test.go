/*
Copyright 2023.

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
package model_test

import (
	"strconv"
	"testing"

	collector_metric "github.com/sustainable-computing-io/kepler/pkg/collector/metric"
	"github.com/sustainable-computing-io/kepler/pkg/collector/metric/types"
	"github.com/sustainable-computing-io/kepler/pkg/config"
	"github.com/sustainable-computing-io/kepler/pkg/model"
)

func benchmarkNtesting(b *testing.B, continerNumber int) {
	containersMetrics := map[string]*collector_metric.ContainerMetrics{}
	collector_metric.ContainerMetricNames = []string{config.CoreUsageMetric}
	b.ReportAllocs()
	for n := 0; n < continerNumber; n++ {
		containersMetrics["container"+strconv.Itoa(n)] = collector_metric.NewContainerMetrics("container"+strconv.Itoa(n), "podA", "test", "container"+strconv.Itoa(n))
		containersMetrics["container"+strconv.Itoa(n)].CounterStats[config.CoreUsageMetric] = &types.UInt64Stat{}
		_ = containersMetrics["container"+strconv.Itoa(n)].CounterStats[config.CoreUsageMetric].AddNewDelta(100)
	}
	b.ResetTimer()
	model.UpdateContainerEnergyByTrainedPowerModel(containersMetrics)
	b.StopTimer()
}

func BenchmarkUpdateContainerEnergyByTrainedPowerModelWith1000Contianer(b *testing.B) {
	benchmarkNtesting(b, 1000)
}

func BenchmarkUpdateContainerEnergyByTrainedPowerModelWith2000Contianer(b *testing.B) {
	benchmarkNtesting(b, 2000)
}

func BenchmarkUpdateContainerEnergyByTrainedPowerModelWith5000Contianer(b *testing.B) {
	benchmarkNtesting(b, 5000)
}

func BenchmarkUpdateContainerEnergyByTrainedPowerModelWith10000Contianer(b *testing.B) {
	benchmarkNtesting(b, 10000)
}
