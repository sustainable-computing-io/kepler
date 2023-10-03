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
	"github.com/sustainable-computing-io/kepler/pkg/collector/metric/types"
	"github.com/sustainable-computing-io/kepler/pkg/config"
	"github.com/sustainable-computing-io/kepler/pkg/power/components/source"
)

var _ = Describe("Test Ratio Unit", func() {
	It("GetContainerEnergyRatio", func() {

		containersMetrics := map[string]*collector_metric.ContainerMetrics{}
		containersMetrics["containerA"] = collector_metric.NewContainerMetrics("containerA", "podA", "test", "containerA")
		containersMetrics["containerA"].BPFStats[config.CoreUsageMetric] = &types.UInt64Stat{}
		err := containersMetrics["containerA"].BPFStats[config.CoreUsageMetric].AddNewDelta(30000)
		Expect(err).NotTo(HaveOccurred())
		containersMetrics["containerB"] = collector_metric.NewContainerMetrics("containerB", "podB", "test", "containerB")
		containersMetrics["containerB"].BPFStats[config.CoreUsageMetric] = &types.UInt64Stat{}
		err = containersMetrics["containerB"].BPFStats[config.CoreUsageMetric].AddNewDelta(30000)
		Expect(err).NotTo(HaveOccurred())
		containersMetrics["containerC"] = collector_metric.NewContainerMetrics("containerC", "podC", "test", "containerC")
		containersMetrics["containerC"].BPFStats[config.CoreUsageMetric] = &types.UInt64Stat{}
		err = containersMetrics["containerC"].BPFStats[config.CoreUsageMetric].AddNewDelta(30000)
		Expect(err).NotTo(HaveOccurred())

		nodeMetrics := collector_metric.NewNodeMetrics()
		collector_metric.ContainerFeaturesNames = []string{config.CoreUsageMetric}
		collector_metric.NodeMetadataFeatureNames = []string{"cpu_architecture"}
		collector_metric.NodeMetadataFeatureValues = []string{"Sandy Bridge"}
		nodeMetrics.AddNodeResUsageFromContainerResUsage(containersMetrics)
		Expect(nodeMetrics.ResourceUsage[config.CoreUsageMetric]).Should(BeEquivalentTo(90000))

		// manually add node mock values
		// initialize the node energy with aggregated energy, which will be used to calculate delta energy
		nodePlatformEnergy := map[string]float64{}
		// initialize the node energy with aggregated energy, which will be used to calculate delta energy
		nodePlatformEnergy["sensor0"] = 5000 // mJ
		nodeMetrics.SetNodePlatformEnergy(nodePlatformEnergy, true, false)
		nodeMetrics.UpdateIdleEnergyWithMinValue(true)
		// the second node energy will represent the idle and dynamic power. The idle power is only calculated after there at at least two delta values
		nodePlatformEnergy["sensor0"] = 35000
		nodeMetrics.SetNodePlatformEnergy(nodePlatformEnergy, true, false)
		nodeMetrics.UpdateDynEnergy()

		// initialize the node energy with aggregated energy, which will be used to calculate delta energy
		// note that NodeComponentsEnergy contains aggregated energy over time
		componentsEnergies := make(map[int]source.NodeComponentsEnergy)
		componentsEnergies[0] = source.NodeComponentsEnergy{
			Pkg:    5000, // mJ
			Core:   5000,
			DRAM:   5000,
			Uncore: 5000,
		}
		nodeMetrics.SetNodeComponentsEnergy(componentsEnergies, false, false)
		componentsEnergies[0] = source.NodeComponentsEnergy{
			Pkg:    10000, // mJ
			Core:   10000,
			DRAM:   10000,
			Uncore: 10000,
		}
		// the second node energy will force to calculate a delta. The delta is calculates after added at least two aggregated metric
		nodeMetrics.SetNodeComponentsEnergy(componentsEnergies, false, false)
		nodeMetrics.UpdateIdleEnergyWithMinValue(true)
		// the third node energy will represent the idle and dynamic power. The idle power is only calculated after there at at least two delta values
		componentsEnergies[0] = source.NodeComponentsEnergy{
			Pkg:    45000, // 35000mJ delta, which is 5000mJ idle, 30000mJ dynamic power
			Core:   45000,
			DRAM:   45000,
			Uncore: 45000,
		}
		nodeMetrics.SetNodeComponentsEnergy(componentsEnergies, false, false)
		nodeMetrics.UpdateDynEnergy()

		Expect(nodeMetrics.GetSumDeltaDynEnergyFromAllSources(collector_metric.PKG)).Should(BeEquivalentTo(30000))
		Expect(nodeMetrics.GetSumDeltaDynEnergyFromAllSources(collector_metric.CORE)).Should(BeEquivalentTo(30000))
		Expect(nodeMetrics.GetSumDeltaDynEnergyFromAllSources(collector_metric.UNCORE)).Should(BeEquivalentTo(30000))
		Expect(nodeMetrics.GetSumDeltaDynEnergyFromAllSources(collector_metric.DRAM)).Should(BeEquivalentTo(30000))
		Expect(nodeMetrics.GetSumDeltaDynEnergyFromAllSources(collector_metric.PLATFORM)).Should(BeEquivalentTo(30000))

		// The default estimator model is the ratio
		model := RatioPowerModel{
			ContainerFeatureNames: []string{
				config.CoreUsageMetric,    // for PKG resource usage
				config.CoreUsageMetric,    // for CORE resource usage
				config.DRAMUsageMetric,    // for DRAM resource usage
				config.GeneralUsageMetric, // for UNCORE resource usage
				config.GeneralUsageMetric, // for OTHER resource usage
				config.GpuUsageMetric,     // for GPU resource usage
			},
			NodeFeatureNames: []string{
				config.CoreUsageMetric,            // for PKG resource usage
				config.CoreUsageMetric,            // for CORE resource usage
				config.DRAMUsageMetric,            // for DRAM resource usage
				config.GeneralUsageMetric,         // for UNCORE resource usage
				config.GeneralUsageMetric,         // for OTHER resource usage
				config.GpuUsageMetric,             // for GPU resource usage
				collector_metric.PKG + "_DYN",     // for dynamic PKG power consumption
				collector_metric.CORE + "_DYN",    // for dynamic CORE power consumption
				collector_metric.DRAM + "_DYN",    // for dynamic PKG power consumption
				collector_metric.UNCORE + "_DYN",  // for dynamic UNCORE power consumption
				collector_metric.OTHER + "_DYN",   // for dynamic OTHER power consumption
				collector_metric.GPU + "_DYN",     // for dynamic GPU power consumption
				collector_metric.PKG + "_IDLE",    // for idle PKG power consumption
				collector_metric.CORE + "_IDLE",   // for idle CORE power consumption
				collector_metric.DRAM + "_IDLE",   // for idle PKG power consumption
				collector_metric.UNCORE + "_IDLE", // for idle UNCORE power consumption
				collector_metric.OTHER + "_IDLE",  // for idle OTHER power consumption
				collector_metric.GPU + "_IDLE",    // for idle GPU power consumption
			},
		}
		model.ResetSampleIdx()
		// Add container metrics
		for _, c := range containersMetrics {
			// add samples to estimate the components (CPU and DRAM) power
			if model.IsEnabled() {
				// Add container metrics
				featureValues := c.ToEstimatorValues(model.GetContainerFeatureNamesList(), true) // add node features with normalized values
				model.AddContainerFeatureValues(featureValues)
			}
		}
		// Add node metrics.
		if model.IsEnabled() {
			featureValues := nodeMetrics.ToEstimatorValues(model.GetNodeFeatureNamesList(), true) // add node features with normalized values
			model.AddNodeFeatureValues(featureValues)
		}

		containerPower, err := model.GetComponentsPower(false)
		Expect(err).NotTo(HaveOccurred())

		// The node energy consumption was manually set as 30J (30000mJ), but since the interval is 3s, the power is 10J
		// There are 3 containers consuming 30000 ns, but since the interval is 3s, the normalized utilization is 10ms
		// Then, the container resource usage ratio will be 0.3334. Consequently, the container power will be 0.3334 * the node power (10000mJ) = 3334
		Expect(containerPower[0].Pkg).Should(BeEquivalentTo(uint64(3334)))
		Expect(containerPower[1].Pkg).Should(BeEquivalentTo(uint64(3334)))
		Expect(containerPower[2].Pkg).Should(BeEquivalentTo(uint64(3334)))
	})
})
