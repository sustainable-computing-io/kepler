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
)

var _ = Describe("Test Container Metric", func() {

	c := ContainerMetrics{
		DynEnergyInCore:   &UInt64Stat{Delta: uint64(1), Aggr: uint64(2)},
		DynEnergyInDRAM:   &UInt64Stat{Delta: uint64(3), Aggr: uint64(4)},
		DynEnergyInUncore: &UInt64Stat{Delta: uint64(5), Aggr: uint64(6)},
		DynEnergyInPkg:    &UInt64Stat{Delta: uint64(7), Aggr: uint64(8)},
		DynEnergyInGPU:    &UInt64Stat{Delta: uint64(9), Aggr: uint64(10)},
		DynEnergyInOther:  &UInt64Stat{Delta: uint64(11), Aggr: uint64(12)},
		CgroupFSStats: map[string]*UInt64StatCollection{
			CORE: {
				Stat: map[string]*UInt64Stat{
					"usage": {Delta: uint64(13), Aggr: uint64(14)},
				},
			},
			DRAM: {
				Stat: map[string]*UInt64Stat{
					"usage": {Delta: uint64(15), Aggr: uint64(16)},
				},
			},
			UNCORE: {
				Stat: map[string]*UInt64Stat{
					"usage": {Delta: uint64(17), Aggr: uint64(18)},
				},
			},
			PKG: {
				Stat: map[string]*UInt64Stat{
					"usage": {Delta: uint64(19), Aggr: uint64(20)},
				},
			},
			GPU: {
				Stat: map[string]*UInt64Stat{
					"usage": {Delta: uint64(21), Aggr: uint64(22)},
				},
			},
			OTHER: {
				Stat: map[string]*UInt64Stat{
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
})
