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

	"github.com/sustainable-computing-io/kepler/pkg/collector/stats"
	"github.com/sustainable-computing-io/kepler/pkg/config"
	"github.com/sustainable-computing-io/kepler/pkg/utils"
)

var _ = Describe("Test Ratio Unit", func() {
	It("GetProcessEnergyRatio", func() {
		stats.SetMockedCollectorMetrics()
		processStats := stats.CreateMockedProcessStats(3)

		nodeStats := stats.CreateMockedNodeStats()
		Expect(nodeStats.EnergyUsage[config.DynEnergyInPkg].SumAllDeltaValues()).Should(BeEquivalentTo(35000))
		Expect(nodeStats.EnergyUsage[config.DynEnergyInCore].SumAllDeltaValues()).Should(BeEquivalentTo(35000))
		Expect(nodeStats.EnergyUsage[config.DynEnergyInUnCore].SumAllDeltaValues()).Should(BeEquivalentTo(0))
		Expect(nodeStats.EnergyUsage[config.DynEnergyInDRAM].SumAllDeltaValues()).Should(BeEquivalentTo(35000))
		Expect(nodeStats.EnergyUsage[config.DynEnergyInPlatform].SumAllDeltaValues()).Should(BeEquivalentTo(35000))

		for _, pMetric := range processStats {
			val := pMetric.ResourceUsage[config.CPUCycle].Stat[stats.MockedSocketID].GetDelta()
			nodeStats.ResourceUsage[config.CPUCycle].AddDeltaStat(stats.MockedSocketID, val)

			val = pMetric.ResourceUsage[config.CPUInstruction].Stat[stats.MockedSocketID].GetDelta()
			nodeStats.ResourceUsage[config.CPUInstruction].AddDeltaStat(stats.MockedSocketID, val)

			val = pMetric.ResourceUsage[config.CacheMiss].Stat[stats.MockedSocketID].GetDelta()
			nodeStats.ResourceUsage[config.CacheMiss].AddDeltaStat(stats.MockedSocketID, val)

			val = pMetric.ResourceUsage[config.CPUTime].Stat[stats.MockedSocketID].GetDelta()
			nodeStats.ResourceUsage[config.CPUTime].AddDeltaStat(stats.MockedSocketID, val)
		}
		Expect(nodeStats.ResourceUsage[config.CoreUsageMetric].Stat[utils.GenericSocketID].GetDelta()).Should(BeEquivalentTo(90000))

		// The default estimator model is the ratio
		model := RatioPowerModel{
			ProcessFeatureNames: []string{
				config.CoreUsageMetric,    // for PKG resource usage
				config.CoreUsageMetric,    // for CORE resource usage
				config.DRAMUsageMetric,    // for DRAM resource usage
				config.GeneralUsageMetric, // for UNCORE resource usage
				config.GeneralUsageMetric, // for OTHER resource usage
				config.GpuUsageMetric,     // for GPU resource usage
			},
			NodeFeatureNames: []string{
				config.CoreUsageMetric,    // for PKG resource usage
				config.CoreUsageMetric,    // for CORE resource usage
				config.DRAMUsageMetric,    // for DRAM resource usage
				config.GeneralUsageMetric, // for UNCORE resource usage
				config.GeneralUsageMetric, // for OTHER resource usage
				config.GpuUsageMetric,     // for GPU resource usage
				config.DynEnergyInPkg,     // for dynamic PKG power consumption
				config.DynEnergyInCore,    // for dynamic CORE power consumption
				config.DynEnergyInDRAM,    // for dynamic PKG power consumption
				config.DynEnergyInUnCore,  // for dynamic UNCORE power consumption
				config.DynEnergyInOther,   // for dynamic OTHER power consumption
				config.DynEnergyInGPU,     // for dynamic GPU power consumption
				config.IdleEnergyInPkg,    // for idle PKG power consumption
				config.IdleEnergyInCore,   // for idle CORE power consumption
				config.IdleEnergyInDRAM,   // for idle PKG power consumption
				config.IdleEnergyInUnCore, // for idle UNCORE power consumption
				config.IdleEnergyInOther,  // for idle OTHER power consumption
				config.IdleEnergyInGPU,    // for idle GPU power consumption
			},
		}
		model.ResetSampleIdx()
		// Add process metrics
		for _, c := range processStats {
			// add samples to estimate the components (CPU and DRAM) power
			if model.IsEnabled() {
				// Add process metrics
				featureValues := c.ToEstimatorValues(model.GetProcessFeatureNamesList(), true) // add node features with normalized values
				model.AddProcessFeatureValues(featureValues)
			}
		}
		// Add node metrics.
		if model.IsEnabled() {
			featureValues := nodeStats.ToEstimatorValues(model.GetNodeFeatureNamesList(), true) // add node features with normalized values
			model.AddNodeFeatureValues(featureValues)
		}

		processPower, err := model.GetComponentsPower(false)
		Expect(err).NotTo(HaveOccurred())

		// The node energy consumption was manually set as 35J (35000mJ), but since the interval is 3s, the power is 11667mJ
		// There are 3 processes consuming 30000ns, but since the interval is 3s, the normalized utilization is 10ms
		// Then, the process resource usage ratio will be 0.3334. Consequently, the process power will be 0.3334 * the node power (11667mJ) = 3334
		// Which is 3889mJ
		Expect(processPower[0].Pkg).Should(BeEquivalentTo(uint64(3889)))
		Expect(processPower[1].Pkg).Should(BeEquivalentTo(uint64(3889)))
		Expect(processPower[2].Pkg).Should(BeEquivalentTo(uint64(3889)))
	})
})
