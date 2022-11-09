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
		EnergyInCore:   &UInt64Stat{Curr: uint64(1), Aggr: uint64(2)},
		EnergyInDRAM:   &UInt64Stat{Curr: uint64(3), Aggr: uint64(4)},
		EnergyInUncore: &UInt64Stat{Curr: uint64(5), Aggr: uint64(6)},
		EnergyInPkg:    &UInt64Stat{Curr: uint64(7), Aggr: uint64(8)},
		EnergyInGPU:    &UInt64Stat{Curr: uint64(9), Aggr: uint64(10)},
		EnergyInOther:  &UInt64Stat{Curr: uint64(11), Aggr: uint64(12)},
		CgroupFSStats: map[string]*UInt64StatCollection{
			"core": {
				Stat: map[string]*UInt64Stat{
					"usage": {Curr: uint64(13), Aggr: uint64(14)},
				},
			},
			"dram": {
				Stat: map[string]*UInt64Stat{
					"usage": {Curr: uint64(15), Aggr: uint64(16)},
				},
			},
			"uncore": {
				Stat: map[string]*UInt64Stat{
					"usage": {Curr: uint64(17), Aggr: uint64(18)},
				},
			},
			"pkg": {
				Stat: map[string]*UInt64Stat{
					"usage": {Curr: uint64(19), Aggr: uint64(20)},
				},
			},
			"gpu": {
				Stat: map[string]*UInt64Stat{
					"usage": {Curr: uint64(21), Aggr: uint64(22)},
				},
			},
			"other": {
				Stat: map[string]*UInt64Stat{
					"usage": {Curr: uint64(23), Aggr: uint64(24)},
				},
			},
		},
	}

	It("Test GetPrometheusEnergyValue", func() {
		Expect(c.GetPrometheusEnergyValue("core", true)).To(Equal(float64(1)))
		Expect(c.GetPrometheusEnergyValue("dram", true)).To(Equal(float64(3)))
		Expect(c.GetPrometheusEnergyValue("uncore", true)).To(Equal(float64(5)))
		Expect(c.GetPrometheusEnergyValue("pkg", true)).To(Equal(float64(7)))
		Expect(c.GetPrometheusEnergyValue("gpu", true)).To(Equal(float64(9)))
		Expect(c.GetPrometheusEnergyValue("other", true)).To(Equal(float64(11)))
	})

	It("Test extractUIntCurrAggr", func() {
		curr, aggr, err := c.extractUIntCurrAggr("core")
		Expect(err).NotTo(HaveOccurred())
		Expect(curr).To(Equal(uint64(13)))
		Expect(aggr).To(Equal(uint64(14)))
		curr, aggr, err = c.extractUIntCurrAggr("dram")
		Expect(err).NotTo(HaveOccurred())
		Expect(curr).To(Equal(uint64(15)))
		Expect(aggr).To(Equal(uint64(16)))
		curr, aggr, err = c.extractUIntCurrAggr("uncore")
		Expect(err).NotTo(HaveOccurred())
		Expect(curr).To(Equal(uint64(17)))
		Expect(aggr).To(Equal(uint64(18)))
		curr, aggr, err = c.extractUIntCurrAggr("pkg")
		Expect(err).NotTo(HaveOccurred())
		Expect(curr).To(Equal(uint64(19)))
		Expect(aggr).To(Equal(uint64(20)))
		curr, aggr, err = c.extractUIntCurrAggr("gpu")
		Expect(err).NotTo(HaveOccurred())
		Expect(curr).To(Equal(uint64(21)))
		Expect(aggr).To(Equal(uint64(22)))
		curr, aggr, err = c.extractUIntCurrAggr("other")
		Expect(err).To(BeNil())
		Expect(curr).To(Equal(uint64(23)))
		Expect(aggr).To(Equal(uint64(24)))
	})
})
