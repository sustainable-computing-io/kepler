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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sustainable-computing-io/kepler/pkg/power/rapl/source"
)

var (
	SampleDynPowerValue float64 = 100.0

	systemFeatures = []string{"cpu_architecture"}
	usageValues    = [][]float64{{1, 1, 1, 1, 1, 1, 1, 1, 1, 1}, {1, 1, 1, 1, 1, 1, 1, 1, 1, 1}}
	systemValues   = []string{"Sandy Bridge"}
)

var _ = Describe("Test Model Unit", func() {
	It("Get container power with no dependency and no node power ", func() {
		// collector/metrics.go - getEstimatorMetrics
		InitEstimateFunctions(usageMetrics, systemFeatures, systemValues)
		Expect(PodComponentPowerModelValid).To(Equal(true))
		// collector/reader.go
		totalNodePower := uint64(0)
		totalGPUPower := uint64(0)
		nodeComponentPowers := source.RAPLPower{}
		containerComponentPowers, containerOtherPowers := GetContainerPower(usageValues, systemValues, totalNodePower, totalGPUPower, nodeComponentPowers)
		Expect(len(containerOtherPowers)).To(Equal(len(usageValues)))
		Expect(len(containerComponentPowers)).Should(Equal(len(usageValues)))
	})
	It("Get container power with no dependency but with total node power ", func() {
		// collector/metrics.go - getEstimatorMetrics
		InitEstimateFunctions(usageMetrics, systemFeatures, systemValues)
		Expect(PodComponentPowerModelValid).To(Equal(true))
		// collector/reader.go
		totalNodePower := uint64(10000)
		totalGPUPower := uint64(1000)
		nodeComponentPowers := source.RAPLPower{}
		containerComponentPowers, containerOtherPowers := GetContainerPower(usageValues, systemValues, totalNodePower, totalGPUPower, nodeComponentPowers)
		Expect(len(containerOtherPowers)).To(Equal(len(usageValues)))
		Expect(len(containerComponentPowers)).Should(Equal(len(usageValues)))
	})
	It("Get container power with no dependency but with all node power ", func() {
		// collector/metrics.go - getEstimatorMetrics
		InitEstimateFunctions(usageMetrics, systemFeatures, systemValues)
		Expect(PodComponentPowerModelValid).To(Equal(true))
		// collector/reader.go
		totalNodePower := uint64(10000)
		totalGPUPower := uint64(1000)
		nodeComponentPowers := source.RAPLPower{
			Pkg:  8000,
			Core: 5000,
			DRAM: 1000,
		}
		containerComponentPowers, containerOtherPowers := GetContainerPower(usageValues, systemValues, totalNodePower, totalGPUPower, nodeComponentPowers)
		Expect(len(containerOtherPowers)).To(Equal(len(usageValues)))
		Expect(len(containerComponentPowers)).Should(Equal(len(usageValues)))
	})
})
