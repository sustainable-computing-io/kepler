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
	"sync"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	collector_metric "github.com/sustainable-computing-io/kepler/pkg/collector/metric"
	"github.com/sustainable-computing-io/kepler/pkg/collector/metric/types"
	"github.com/sustainable-computing-io/kepler/pkg/config"
	"github.com/sustainable-computing-io/kepler/pkg/power/components/source"
)

var _ = Describe("Test Ratio Unit", func() {
	It("GetContainerEnergyRatio", func() {

		containersMetrics := map[string]*collector_metric.ContainerMetrics{}
		containersMetrics["containerA"] = collector_metric.NewContainerMetrics("containerA", "podA", "test", "containerA")
		containersMetrics["containerA"].CounterStats[config.CoreUsageMetric] = &types.UInt64Stat{}
		err := containersMetrics["containerA"].CounterStats[config.CoreUsageMetric].AddNewDelta(100)
		Expect(err).NotTo(HaveOccurred())
		containersMetrics["containerB"] = collector_metric.NewContainerMetrics("containerB", "podB", "test", "containerB")
		containersMetrics["containerB"].CounterStats[config.CoreUsageMetric] = &types.UInt64Stat{}
		err = containersMetrics["containerB"].CounterStats[config.CoreUsageMetric].AddNewDelta(100)
		Expect(err).NotTo(HaveOccurred())
		containersMetrics["containerC"] = collector_metric.NewContainerMetrics("containerC", "podC", "test", "containerC")
		containersMetrics["containerC"].CounterStats[config.CoreUsageMetric] = &types.UInt64Stat{}
		err = containersMetrics["containerC"].CounterStats[config.CoreUsageMetric].AddNewDelta(100)
		Expect(err).NotTo(HaveOccurred())

		nodeMetrics := collector_metric.NewNodeMetrics()
		collector_metric.ContainerMetricNames = []string{config.CoreUsageMetric}
		nodeMetrics.AddNodeResUsageFromContainerResUsage(containersMetrics)
		Expect(nodeMetrics.ResourceUsage[config.CoreUsageMetric]).Should(BeEquivalentTo(300))

		// add node mock values
		// initialize the node energy with aggregated energy, which will be used to calculate delta energy
		nodePlatformEnergy := map[string]float64{}
		// initialize the node energy with aggregated energy, which will be used to calculate delta energy
		nodePlatformEnergy["sensor0"] = 10
		nodeMetrics.SetLastestPlatformEnergy(nodePlatformEnergy, true)
		nodeMetrics.UpdateIdleEnergy()
		// the second node energy will represent the idle and dynamic power
		nodePlatformEnergy["sensor0"] = 20 // 5J idle, 5J dynamic power
		nodeMetrics.SetLastestPlatformEnergy(nodePlatformEnergy, true)
		nodeMetrics.UpdateIdleEnergy()
		nodeMetrics.UpdateDynEnergy()

		// initialize the node energy with aggregated energy, which will be used to calculate delta energy
		// note that NodeComponentsEnergy contains aggregated energy over time
		componentsEnergies := make(map[int]source.NodeComponentsEnergy)
		componentsEnergies[0] = source.NodeComponentsEnergy{
			Pkg:    5,
			Core:   5,
			DRAM:   5,
			Uncore: 5,
		}
		nodeMetrics.SetNodeComponentsEnergy(componentsEnergies, false)
		componentsEnergies[0] = source.NodeComponentsEnergy{
			Pkg:    10,
			Core:   10,
			DRAM:   10,
			Uncore: 10,
		}
		// the second node energy will force to calculate a delta. The delta is calculates after added at least two aggregated metric
		nodeMetrics.SetNodeComponentsEnergy(componentsEnergies, false)
		nodeMetrics.UpdateIdleEnergy()
		// the third node energy will represent the idle and dynamic power. The idle power is only calculated after there at at least two delta values
		componentsEnergies[0] = source.NodeComponentsEnergy{
			Pkg:    40, // 10J delta, which is 5J idle, 5J dynamic power
			Core:   40, // 10J delta, which is 5J idle, 5J dynamic power
			DRAM:   40, // 10J delta, which is 5J idle, 5J dynamic power
			Uncore: 40, // 10J delta, which is 5J idle, 5J dynamic power
		}
		nodeMetrics.SetNodeComponentsEnergy(componentsEnergies, false)
		nodeMetrics.UpdateIdleEnergy()
		nodeMetrics.UpdateDynEnergy()

		Expect(nodeMetrics.GetSumDeltaDynEnergyFromAllSources(collector_metric.PKG)).Should(BeEquivalentTo(25))
		Expect(nodeMetrics.GetSumDeltaDynEnergyFromAllSources(collector_metric.CORE)).Should(BeEquivalentTo(25))
		Expect(nodeMetrics.GetSumDeltaDynEnergyFromAllSources(collector_metric.UNCORE)).Should(BeEquivalentTo(25))
		Expect(nodeMetrics.GetSumDeltaDynEnergyFromAllSources(collector_metric.DRAM)).Should(BeEquivalentTo(25))
		Expect(nodeMetrics.GetSumDeltaDynEnergyFromAllSources(collector_metric.PLATFORM)).Should(BeEquivalentTo(10))

		Expect(nodeMetrics.GetSumAggrDynEnergyFromAllSources(collector_metric.PLATFORM)).Should(BeEquivalentTo(20))

		var wg sync.WaitGroup
		wg.Add(1)
		go UpdateContainerComponentEnergyByRatioPowerModel(containersMetrics, nodeMetrics, collector_metric.PKG, config.CoreUsageMetric, &wg)
		wg.Wait()
		// The pkg dynamic energy is 5mJ, the container cpu usage is 50%, so the dynamic energy is 2.5mJ = ~3mJ
		Expect(containersMetrics["containerA"].DynEnergyInPkg.Delta).Should(BeEquivalentTo(uint64(9)))
		Expect(containersMetrics["containerB"].DynEnergyInPkg.Delta).Should(BeEquivalentTo(uint64(9)))
		Expect(containersMetrics["containerC"].DynEnergyInPkg.Delta).Should(BeEquivalentTo(uint64(9)))
	})
})
