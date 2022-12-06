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

package local

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	collector_metric "github.com/sustainable-computing-io/kepler/pkg/collector/metric"
	"github.com/sustainable-computing-io/kepler/pkg/config"
	"github.com/sustainable-computing-io/kepler/pkg/power/components/source"
)

func getSumDelta(corePower, dramPower, uncorePower, pkgPower, gpuPower []float64) (totalCorePower, totalDRAMPower, totalUncorePower, totalPkgPower, totalGPUPower uint64) {
	for i, val := range pkgPower {
		totalCorePower += uint64(corePower[i])
		totalDRAMPower += uint64(dramPower[i])
		totalUncorePower += uint64(uncorePower[i])
		totalPkgPower += uint64(val)
	}
	for _, val := range gpuPower {
		totalGPUPower += uint64(val)
	}
	return
}

var _ = Describe("Test Ratio Unit", func() {
	It("GetContainerEnergyRatio", func() {

		containersMetrics := map[string]*collector_metric.ContainerMetrics{}
		containersMetrics["containerA"] = collector_metric.NewContainerMetrics("containerA", "podA", "test")
		containersMetrics["containerA"].CounterStats[config.CoreUsageMetric] = &collector_metric.UInt64Stat{}
		err := containersMetrics["containerA"].CounterStats[config.CoreUsageMetric].AddNewCurr(100)
		Expect(err).NotTo(HaveOccurred())
		containersMetrics["containerB"] = collector_metric.NewContainerMetrics("containerB", "podB", "test")
		containersMetrics["containerB"].CounterStats[config.CoreUsageMetric] = &collector_metric.UInt64Stat{}
		err = containersMetrics["containerB"].CounterStats[config.CoreUsageMetric].AddNewCurr(100)
		Expect(err).NotTo(HaveOccurred())

		nodeMetrics := *collector_metric.NewNodeMetrics()
		collector_metric.ContainerMetricNames = []string{config.CoreUsageMetric}
		nodeMetrics.AddNodeResUsageFromContainerResUsage(containersMetrics)
		Expect(nodeMetrics.ResourceUsage[config.CoreUsageMetric]).Should(BeEquivalentTo(200))

		componentsEnergies := make(map[int]source.NodeComponentsEnergy)
		componentsEnergies[0] = source.NodeComponentsEnergy{
			Core:   1,
			DRAM:   1,
			Uncore: 1,
			Pkg:    1,
		}
		nodeMetrics.AddNodeComponentsEnergy(componentsEnergies)
		componentsEnergies[0] = source.NodeComponentsEnergy{
			Core:   10,
			DRAM:   3,
			Uncore: 2,
			Pkg:    20,
		}
		nodeMetrics.AddNodeComponentsEnergy(componentsEnergies)
		Expect(nodeMetrics.EnergyInCore.Aggr()).Should(BeEquivalentTo(10))
		Expect(nodeMetrics.EnergyInDRAM.Aggr()).Should(BeEquivalentTo(3))
		Expect(nodeMetrics.EnergyInUncore.Aggr()).Should(BeEquivalentTo(2))
		Expect(nodeMetrics.EnergyInPkg.Aggr()).Should(BeEquivalentTo(20))

		nodePlatformEnergy := map[string]float64{}
		nodePlatformEnergy["sensor0"] = 40
		nodeMetrics.AddLastestPlatformEnergy(nodePlatformEnergy) // must be higher than components energy
		Expect(nodeMetrics.EnergyInPlatform.Aggr()).Should(BeEquivalentTo(40))

		UpdateContainerEnergyByRatioPowerModel(containersMetrics, nodeMetrics)
		Expect(containersMetrics["containerA"].EnergyInPkg.Curr).Should(BeEquivalentTo(10))
	})
})
