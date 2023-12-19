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
package stats_test

import (
	"testing"

	"github.com/sustainable-computing-io/kepler/pkg/collector"
	"github.com/sustainable-computing-io/kepler/pkg/collector/stats"
	"github.com/sustainable-computing-io/kepler/pkg/model"
)

func benchmarkNtesting(b *testing.B, processNumber int) {
	// enable metrics
	stats.SetMockedCollectorMetrics()
	// create node node metrics
	metricCollector := collector.NewCollector()

	// create processes
	metricCollector.ProcessStats = stats.CreateMockedProcessStats(processNumber)
	metricCollector.NodeStats = stats.CreateMockedNodeStats()
	// aggregate processes' resource utilization metrics to containers, virtual machines and nodes
	metricCollector.AggregateProcessResourceUtilizationMetrics()

	// The default estimator model is the ratio
	model.CreatePowerEstimatorModels(stats.ProcessFeaturesNames, stats.NodeMetadataFeatureNames, stats.NodeMetadataFeatureValues)

	// update container and node metrics
	b.ReportAllocs()
	b.ResetTimer()
	metricCollector.UpdateProcessEnergyUtilizationMetrics()
	metricCollector.AggregateProcessEnergyUtilizationMetrics()
	b.StopTimer()
}

func BenchmarkAddNodeResUsageFromContainerResUsageWith1000Container(b *testing.B) {
	benchmarkNtesting(b, 1000)
}

func BenchmarkAddNodeResUsageFromContainerResUsageWith2000Container(b *testing.B) {
	benchmarkNtesting(b, 2000)
}

func BenchmarkAddNodeResUsageFromContainerResUsageWith4000Container(b *testing.B) {
	benchmarkNtesting(b, 4000)
}

func BenchmarkAddNodeResUsageFromContainerResUsageWith8000Container(b *testing.B) {
	benchmarkNtesting(b, 8000)
}
