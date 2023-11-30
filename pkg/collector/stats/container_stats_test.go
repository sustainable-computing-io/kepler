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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sustainable-computing-io/kepler/pkg/config"
)

var _ = Describe("Test Container Metric", func() {

	It("Test ResetDeltaValues", func() {
		SetMockedCollectorMetrics()
		c := NewContainerStats("containerA", "podA", "test", "containerIDA")
		c.ResourceUsage[config.CPUCycle].SetDeltaStat(MockedSocketID, 30000)
		c.ResourceUsage[config.CPUInstruction].SetDeltaStat(MockedSocketID, 30000)
		c.ResourceUsage[config.CacheMiss].SetDeltaStat(MockedSocketID, 30000)
		// add first values to be the idle power
		c.EnergyUsage[config.AbsEnergyInPkg].SetDeltaStat(MockedSocketID, 10)
		c.EnergyUsage[config.AbsEnergyInCore].SetDeltaStat(MockedSocketID, 10)
		c.EnergyUsage[config.AbsEnergyInDRAM].SetDeltaStat(MockedSocketID, 10)
		// add second values to have dynamic power
		c.EnergyUsage[config.AbsEnergyInPkg].SetDeltaStat(MockedSocketID, 18)
		c.EnergyUsage[config.AbsEnergyInCore].SetDeltaStat(MockedSocketID, 15)
		c.EnergyUsage[config.AbsEnergyInDRAM].SetDeltaStat(MockedSocketID, 11)

		c.ResetDeltaValues()

		delta := c.ResourceUsage[config.CacheMiss].SumAllDeltaValues()
		Expect(delta).To(Equal(uint64(0)))
	})
})
