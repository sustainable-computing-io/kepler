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

package metric

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sustainable-computing-io/kepler/pkg/collector/metric/types"
)

var _ = Describe("Test Container Metric", func() {

	c := ContainerMetrics{
		ProcessMetrics: ProcessMetrics{
			DynEnergyInCore:   &types.UInt64Stat{Delta: uint64(1), Aggr: uint64(2)},
			DynEnergyInDRAM:   &types.UInt64Stat{Delta: uint64(3), Aggr: uint64(4)},
			DynEnergyInUncore: &types.UInt64Stat{Delta: uint64(5), Aggr: uint64(6)},
			DynEnergyInPkg:    &types.UInt64Stat{Delta: uint64(7), Aggr: uint64(8)},
			DynEnergyInGPU:    &types.UInt64Stat{Delta: uint64(9), Aggr: uint64(10)},
			DynEnergyInOther:  &types.UInt64Stat{Delta: uint64(11), Aggr: uint64(12)},
		},
		CgroupStatMap: map[string]*types.UInt64StatCollection{
			CORE: {
				Stat: map[string]*types.UInt64Stat{
					"usage": {Delta: uint64(13), Aggr: uint64(14)},
				},
			},
			DRAM: {
				Stat: map[string]*types.UInt64Stat{
					"usage": {Delta: uint64(15), Aggr: uint64(16)},
				},
			},
			UNCORE: {
				Stat: map[string]*types.UInt64Stat{
					"usage": {Delta: uint64(17), Aggr: uint64(18)},
				},
			},
			PKG: {
				Stat: map[string]*types.UInt64Stat{
					"usage": {Delta: uint64(19), Aggr: uint64(20)},
				},
			},
			GPU: {
				Stat: map[string]*types.UInt64Stat{
					"usage": {Delta: uint64(21), Aggr: uint64(22)},
				},
			},
			OTHER: {
				Stat: map[string]*types.UInt64Stat{
					"usage": {Delta: uint64(23), Aggr: uint64(24)},
				},
			},
		},
	}

	It("Test getIntDeltaAndAggrValue", func() {
		delta, aggr, err := c.getIntDeltaAndAggrValue(CORE)
		Expect(err).NotTo(HaveOccurred())
		Expect(delta).To(Equal(uint64(13)))
		Expect(aggr).To(Equal(uint64(14)))
		delta, aggr, err = c.getIntDeltaAndAggrValue(DRAM)
		Expect(err).NotTo(HaveOccurred())
		Expect(delta).To(Equal(uint64(15)))
		Expect(aggr).To(Equal(uint64(16)))
		delta, aggr, err = c.getIntDeltaAndAggrValue(UNCORE)
		Expect(err).NotTo(HaveOccurred())
		Expect(delta).To(Equal(uint64(17)))
		Expect(aggr).To(Equal(uint64(18)))
		delta, aggr, err = c.getIntDeltaAndAggrValue(PKG)
		Expect(err).NotTo(HaveOccurred())
		Expect(delta).To(Equal(uint64(19)))
		Expect(aggr).To(Equal(uint64(20)))
		delta, aggr, err = c.getIntDeltaAndAggrValue(GPU)
		Expect(err).NotTo(HaveOccurred())
		Expect(delta).To(Equal(uint64(21)))
		Expect(aggr).To(Equal(uint64(22)))
		delta, aggr, err = c.getIntDeltaAndAggrValue(OTHER)
		Expect(err).NotTo(HaveOccurred())
		Expect(delta).To(Equal(uint64(23)))
		Expect(aggr).To(Equal(uint64(24)))
	})

	It("Test ResetDeltaValues", func() {
		instance := NewContainerMetrics("container", "PodName", "Namespace", "container")
		instance.ResetDeltaValues()
		Expect(0).To(Equal(instance.CurrProcesses))
	})

	It("Test SumAllDynDeltaValues", func() {
		instance := NewContainerMetrics("container", "PodName", "Namespace", "container")
		exp := instance.DynEnergyInPkg.Delta + instance.DynEnergyInGPU.Delta + instance.DynEnergyInOther.Delta
		cur := instance.SumAllDynDeltaValues()
		Expect(exp).To(Equal(cur))
	})

	It("Test SumAllDynAggrValues", func() {
		instance := NewContainerMetrics("container", "PodName", "Namespace", "container")
		exp := instance.DynEnergyInPkg.Aggr + instance.DynEnergyInGPU.Aggr + instance.DynEnergyInOther.Aggr
		cur := instance.SumAllDynAggrValues()
		Expect(exp).To(Equal(cur))
	})
})
