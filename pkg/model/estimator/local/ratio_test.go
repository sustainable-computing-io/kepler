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

	"github.com/sustainable-computing-io/kepler/pkg/power/rapl/source"
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
	It("GetPodPowerRatio", func() {
		corePower := []float64{10, 10}
		dramPower := []float64{2, 2}
		uncorePower := []float64{1, 1}
		pkgPower := []float64{15, 15}
		totalCorePower, totalDRAMPower, totalUncorePower, totalPkgPower, _ := getSumDelta(corePower, dramPower, uncorePower, pkgPower, empty)
		Expect(totalCorePower).Should(BeEquivalentTo(20))
		Expect(totalDRAMPower).Should(BeEquivalentTo(4))
		Expect(totalUncorePower).Should(BeEquivalentTo(2))
		Expect(totalPkgPower).Should(BeEquivalentTo(30))
		nodeComponentPower := source.RAPLPower{
			Core:   totalCorePower,
			Uncore: totalUncorePower,
			DRAM:   totalDRAMPower,
			Pkg:    totalPkgPower,
		}
		otherNodePower := uint64(10)
		componentPowers, otherPodPowers := GetPodPowerRatio(usageValues, otherNodePower, nodeComponentPower)
		Expect(len(componentPowers)).Should(Equal(len(usageValues)))
		Expect(len(otherPodPowers)).Should(Equal(len(usageValues)))
		Expect(componentPowers[0].Core).Should(Equal(componentPowers[1].Core))
		Expect(componentPowers[0].Core).Should(BeEquivalentTo(10))
		Expect(componentPowers[0].DRAM).Should(BeEquivalentTo(2))
		Expect(componentPowers[0].Uncore).Should(BeEquivalentTo(1))
		Expect(componentPowers[0].Pkg).Should(BeEquivalentTo(15))
		Expect(otherPodPowers[0]).Should(BeEquivalentTo(5))
	})
})
