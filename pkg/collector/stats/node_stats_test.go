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

package stats

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sustainable-computing-io/kepler/pkg/config"
)

var _ = Describe("Test Node Metric", func() {
	var (
		processMetrics map[uint64]*ProcessStats
		nodeMetrics    NodeStats
	)

	BeforeEach(func() {
		SetMockedCollectorMetrics()
		processMetrics = CreateMockedProcessStats(2)
		nodeMetrics = CreateMockedNodeStats()
		for _, pMetric := range processMetrics {
			val := pMetric.ResourceUsage[config.CPUCycle].Stat[MockedSocketID].GetDelta()
			nodeMetrics.ResourceUsage[config.CPUCycle].AddDeltaStat(MockedSocketID, val)

			val = pMetric.ResourceUsage[config.CPUInstruction].Stat[MockedSocketID].GetDelta()
			nodeMetrics.ResourceUsage[config.CPUInstruction].AddDeltaStat(MockedSocketID, val)

			val = pMetric.ResourceUsage[config.CacheMiss].Stat[MockedSocketID].GetDelta()
			nodeMetrics.ResourceUsage[config.CacheMiss].AddDeltaStat(MockedSocketID, val)

			val = pMetric.ResourceUsage[config.CPUTime].Stat[MockedSocketID].GetDelta()
			nodeMetrics.ResourceUsage[config.CPUTime].AddDeltaStat(MockedSocketID, val)
		}
	})

	It("Test nodeMetrics ResourceUsage", func() {
		v, ok := nodeMetrics.ResourceUsage[config.CPUCycle]
		Expect(ok).To(Equal(true))
		Expect(v.Stat[MockedSocketID].GetDelta()).To(Equal(uint64(60000)))
	})

	It("test SetNodeGPUEnergy", func() {
		gpuPower := uint64(1)
		nodeMetrics.EnergyUsage[config.AbsEnergyInGPU].AddDeltaStat(MockedSocketID, gpuPower)
		fmt.Fprintln(GinkgoWriter, "nodeMetrics", nodeMetrics.EnergyUsage[config.AbsEnergyInGPU])
		val := nodeMetrics.EnergyUsage[config.AbsEnergyInGPU].SumAllDeltaValues()
		Expect(val).To(Equal(gpuPower))
	})

	It("test ResetDeltaValues", func() {
		nodeMetrics.ResetDeltaValues()
		val := nodeMetrics.EnergyUsage[config.AbsEnergyInCore].SumAllDeltaValues()
		Expect(val).To(Equal(uint64(0)))
	})

	It("test UpdateIdleEnergyWithMinValue", func() {
		nodeMetrics.UpdateIdleEnergyWithMinValue(true)
		Expect(nodeMetrics.EnergyUsage[config.IdleEnergyInPkg].SumAllDeltaValues()).ShouldNot(BeNil())
	})

	It("test String", func() {
		str := nodeMetrics.String()
		Expect("node energy (mJ):").To(Equal(str[0:len("node energy (mJ):")]))
	})
})
