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
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sustainable-computing-io/kepler/pkg/collector/stats"
	"github.com/sustainable-computing-io/kepler/pkg/config"
	"github.com/sustainable-computing-io/kepler/pkg/sensors/components"
	"github.com/sustainable-computing-io/kepler/pkg/sensors/platform"
	"github.com/sustainable-computing-io/kepler/pkg/utils"
)

var _ = Describe("ProcessPower", func() {
	var (
		processStats map[uint64]*stats.ProcessStats
		nodeStats    stats.NodeStats

		systemMetaDataFeatureNames  = []string{"cpu_architecture"}
		systemMetaDataFeatureValues = []string{"Sandy Bridge"}
	)

	Context("with manually defined node power", func() {
		BeforeEach(func() {
			// we need to disable the system real time power metrics for testing since we add mock values or use power model estimator
			components.SetIsSystemCollectionSupported(false)
			platform.SetIsSystemCollectionSupported(false)
			stats.SetMockedCollectorMetrics()

			processStats = stats.CreateMockedProcessStats(2)
			nodeStats = stats.CreateMockedNodeStats()
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
		})

		// By default the Ratio power mode is used to get the process power
		It("Get process power with Ratio power model and node component power", func() {
			configStr := "CONTAINER_COMPONENTS_ESTIMATOR=false\n"
			os.Setenv("MODEL_CONFIG", configStr)

			// getEstimatorMetrics
			CreatePowerEstimatorModels(stats.ProcessFeaturesNames, systemMetaDataFeatureNames, systemMetaDataFeatureValues)

			// initialize the node energy with aggregated energy, which will be used to calculate delta energy
			// add first values to be the idle power
			nodeStats.EnergyUsage[config.AbsEnergyInPkg].SetDeltaStat(utils.GenericSocketID, 5000) // mili joules
			nodeStats.EnergyUsage[config.AbsEnergyInCore].SetDeltaStat(utils.GenericSocketID, 5000)
			nodeStats.EnergyUsage[config.AbsEnergyInDRAM].SetDeltaStat(utils.GenericSocketID, 5000)
			// add second values to have dynamic power
			nodeStats.EnergyUsage[config.AbsEnergyInPkg].SetDeltaStat(utils.GenericSocketID, 10000)
			nodeStats.EnergyUsage[config.AbsEnergyInCore].SetDeltaStat(utils.GenericSocketID, 10000)
			nodeStats.EnergyUsage[config.AbsEnergyInDRAM].SetDeltaStat(utils.GenericSocketID, 10000)
			nodeStats.UpdateIdleEnergyWithMinValue(true)
			// add second values to have dynamic power
			nodeStats.EnergyUsage[config.AbsEnergyInPkg].SetDeltaStat(utils.GenericSocketID, 45000)
			nodeStats.EnergyUsage[config.AbsEnergyInCore].SetDeltaStat(utils.GenericSocketID, 45000)
			nodeStats.EnergyUsage[config.AbsEnergyInDRAM].SetDeltaStat(utils.GenericSocketID, 45000)
			nodeStats.UpdateDynEnergy()

			// calculate process energy consumption
			UpdateProcessEnergy(processStats, &nodeStats)

			// The default process power model is the Ratio, then process energy consumption will be as follows:
			// The node components dynamic power were set to 35000mJ, since the kepler interval is 3s, the power is 11667mJ
			// The test created 2 processes with 30000 CPU Instructions
			// So the node total CPU Instructions is 60000
			// The process power will be (30000/60000)*11667 = 5834
			// Then, the process energy will be 5834*3 = 17502 mJ
			Expect(processStats[uint64(1)].EnergyUsage[config.DynEnergyInPkg].Stat[utils.GenericSocketID].GetDelta()).To(Equal(uint64(17502)))
		})

		It("Get process power with Ratio power model and node platform power ", func() {
			configStr := "CONTAINER_COMPONENTS_ESTIMATOR=false\n"
			os.Setenv("MODEL_CONFIG", configStr)

			// getEstimatorMetrics
			CreatePowerEstimatorModels(stats.ProcessFeaturesNames, systemMetaDataFeatureNames, systemMetaDataFeatureValues)

			// initialize the node energy with aggregated energy, which will be used to calculate delta energy
			// add first values to be the idle power
			nodeStats.EnergyUsage[config.AbsEnergyInPlatform].SetDeltaStat(utils.GenericSocketID, 5000) // mili joules
			// add second values to have dynamic power
			nodeStats.EnergyUsage[config.AbsEnergyInPlatform].SetDeltaStat(utils.GenericSocketID, 10000)
			nodeStats.UpdateIdleEnergyWithMinValue(true)
			// add second values to have dynamic power
			nodeStats.EnergyUsage[config.AbsEnergyInPlatform].SetDeltaStat(utils.GenericSocketID, 45000)
			nodeStats.UpdateDynEnergy()

			// calculate process energy consumption
			UpdateProcessEnergy(processStats, &nodeStats)

			// The default process power model is the Ratio, then process energy consumption will be as follows:
			// The node components dynamic power were set to 35000mJ, since the kepler interval is 3s, the power is 11667mJ
			// The test created 2 processes with 30000 CPU Instructions
			// So the node total CPU Instructions is 60000
			// The process power will be (30000/60000)*11667 = 5834
			// Then, the process energy will be 5834*3 = 17502 mJ
			Expect(processStats[uint64(1)].EnergyUsage[config.DynEnergyInPlatform].Stat[utils.GenericSocketID].GetDelta()).To(Equal(uint64(17502)))
		})

		// TODO: Get process power with no dependency and no node power.
		// The current LR model has some problems, all the model weights are negative, which means that the energy consumption will decrease with larger resource utilization.
		// Consequently the dynamic power will be 0 since the idle power with 0 resource utilization will be higher than the absolute power with non zero utilization
		// Because of that we removed this test until we have a power model that is more accurate.
		// It("Get process power with Ratio power model and node power with Linear Regression Power Model", func() {
		// 	// the default power models will be linear regression for node power estimation and ratio for process power estimation
		// 	CreatePowerEstimatorModels(processFeatureNames, systemMetaDataFeatureNames, systemMetaDataFeatureValues)
		// 	// add node energy via estimator, using the LR model (the default configured one)
		// 	// UpdateNodePlatformEnergy(nodeStats) currently we do not have the node platform abs power model, we will include that in the future
		// 	UpdateNodeComponentEnergy(nodeStats)
		// 	// calculate process energy consumption
		// 	UpdateProcessEnergy(processStats, nodeStats)
		// 	Expect(processStats["processA"].DynEnergyInPkg.Delta).To(Equal(uint64(???)))
		// })
	})
})
