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
	"strconv"
	"testing"

	collector_metric "github.com/sustainable-computing-io/kepler/pkg/collector/metric"
	"github.com/sustainable-computing-io/kepler/pkg/collector/metric/types"
	"github.com/sustainable-computing-io/kepler/pkg/config"
	"github.com/sustainable-computing-io/kepler/pkg/model"
	"github.com/sustainable-computing-io/kepler/pkg/power/components"
	"github.com/sustainable-computing-io/kepler/pkg/power/components/source"
	"github.com/sustainable-computing-io/kepler/pkg/power/platform"
)

const (
	configStrEstimator = "CONTAINER_COMPONENTS_ESTIMATOR=false\n"
)

func benchmarkNtesting(b *testing.B, containerNumber int) {
	nodeMetrics := collector_metric.NewNodeMetrics()
	collector_metric.ContainerFeaturesNames = []string{config.CoreUsageMetric}
	collector_metric.NodeMetadataFeatureNames = []string{"cpu_architecture"}
	collector_metric.NodeMetadataFeatureValues = []string{"Sandy Bridge"}
	// we need to disable the system real time power metrics for testing since we add mock values or use power model estimator
	components.SetIsSystemCollectionSupported(false)
	platform.SetIsSystemCollectionSupported(false)

	nodePlatformEnergy := map[string]float64{}

	nodePlatformEnergy["sensor0"] = 10
	nodeMetrics.SetNodePlatformEnergy(nodePlatformEnergy, true, false)
	nodeMetrics.UpdateIdleEnergyWithMinValue(true)

	nodePlatformEnergy["sensor0"] = 20
	nodeMetrics.SetNodePlatformEnergy(nodePlatformEnergy, true, false)
	nodeMetrics.UpdateIdleEnergyWithMinValue(true)
	nodeMetrics.UpdateDynEnergy()

	componentsEnergies := make(map[int]source.NodeComponentsEnergy)
	componentsEnergies[0] = source.NodeComponentsEnergy{
		Pkg:    5,
		Core:   5,
		DRAM:   5,
		Uncore: 5,
	}
	nodeMetrics.SetNodeComponentsEnergy(componentsEnergies, false, false)
	componentsEnergies[0] = source.NodeComponentsEnergy{
		Pkg:    10,
		Core:   10,
		DRAM:   10,
		Uncore: 10,
	}
	nodeMetrics.SetNodeComponentsEnergy(componentsEnergies, false, false)
	nodeMetrics.UpdateIdleEnergyWithMinValue(true)
	componentsEnergies[0] = source.NodeComponentsEnergy{
		Pkg:    uint64(containerNumber),
		Core:   uint64(containerNumber),
		DRAM:   uint64(containerNumber),
		Uncore: uint64(containerNumber),
	}
	nodeMetrics.SetNodeComponentsEnergy(componentsEnergies, false, false)

	nodeMetrics.UpdateIdleEnergyWithMinValue(true)
	nodeMetrics.UpdateDynEnergy()
	b.ReportAllocs()
	containersMetrics := map[string]*collector_metric.ContainerMetrics{}
	for n := 0; n < containerNumber; n++ {
		containersMetrics["container"+strconv.Itoa(n)] = collector_metric.NewContainerMetrics("container"+strconv.Itoa(n), "podA", "test", "container"+strconv.Itoa(n))
		containersMetrics["container"+strconv.Itoa(n)].BPFStats[config.CoreUsageMetric] = &types.UInt64Stat{}
		_ = containersMetrics["container"+strconv.Itoa(n)].BPFStats[config.CoreUsageMetric].AddNewDelta(30000)
	}
	nodeMetrics.AddNodeResUsageFromContainerResUsage(containersMetrics)
	b.ResetTimer()
	model.CreatePowerEstimatorModels(collector_metric.ContainerFeaturesNames, collector_metric.NodeMetadataFeatureNames, collector_metric.NodeMetadataFeatureValues)
	model.UpdateContainerEnergy(containersMetrics, nodeMetrics)
	b.StopTimer()
}

func BenchmarkUpdateContainerWith1000Container(b *testing.B) {
	// disable side car estimator
	os.Setenv("MODEL_CONFIG", configStrEstimator)
	benchmarkNtesting(b, 1000)
}

func BenchmarkUpdateContainerWith2000Container(b *testing.B) {
	os.Setenv("MODEL_CONFIG", configStrEstimator)
	benchmarkNtesting(b, 2000)
}

func BenchmarkUpdateContainerWith5000Container(b *testing.B) {
	os.Setenv("MODEL_CONFIG", configStrEstimator)
	benchmarkNtesting(b, 5000)
}

func BenchmarkUpdateContainerWith10000Container(b *testing.B) {
	os.Setenv("MODEL_CONFIG", configStrEstimator)
	benchmarkNtesting(b, 10000)
}
