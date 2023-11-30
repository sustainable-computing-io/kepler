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
	"os"
	"testing"

	"github.com/sustainable-computing-io/kepler/pkg/collector"
	"github.com/sustainable-computing-io/kepler/pkg/collector/stats"
	"github.com/sustainable-computing-io/kepler/pkg/model"
)

const (
	configStrEstimator = "CONTAINER_COMPONENTS_ESTIMATOR=false\n"
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

func BenchmarkUpdateProcessWith1000Process(b *testing.B) {
	// disable side car estimator
	os.Setenv("MODEL_CONFIG", configStrEstimator)
	benchmarkNtesting(b, 1000)
}

func BenchmarkUpdateProcessWith2000Process(b *testing.B) {
	os.Setenv("MODEL_CONFIG", configStrEstimator)
	benchmarkNtesting(b, 2000)
}

func BenchmarkUpdateProcessWith4000Process(b *testing.B) {
	os.Setenv("MODEL_CONFIG", configStrEstimator)
	benchmarkNtesting(b, 4000)
}

func BenchmarkUpdateProcessWith8000Process(b *testing.B) {
	os.Setenv("MODEL_CONFIG", configStrEstimator)
	benchmarkNtesting(b, 8000)
}
