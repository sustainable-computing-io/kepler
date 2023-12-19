/*
Copyright 2023.
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
package types_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sustainable-computing-io/kepler/pkg/collector/stats/types"
)

var _ = Describe("Types", func() {
	Context("UInt64Stat", func() {
		It("ResetDeltaValues", func() {
			Instance := types.UInt64Stat{
				Aggr:  0,
				Delta: 1,
			}
			Instance.ResetDeltaValues()
			Expect(Instance.Delta).To(Equal(uint64(0)))
		})
		It("AddNewDelta", func() {
			Instance := types.UInt64Stat{
				Aggr:  0,
				Delta: 1,
			}
			err := Instance.AddNewDelta(uint64(1))
			Expect(err).NotTo(HaveOccurred())
			Expect(Instance.Delta).To(Equal(uint64(2)))
			Expect(Instance.Aggr).To(Equal(uint64(1)))
		})
		It("SetNewDelta", func() {
			Instance := types.UInt64Stat{
				Aggr:  0,
				Delta: 1,
			}
			err := Instance.SetNewDelta(uint64(1))
			Expect(err).NotTo(HaveOccurred())
			Expect(Instance.Delta).To(Equal(uint64(1)))
			Expect(Instance.Aggr).To(Equal(uint64(1)))
		})
		It("SetNewDeltaValue", func() {
			Instance := types.UInt64Stat{
				Aggr:  0,
				Delta: 1,
			}
			err := Instance.SetNewDeltaValue(uint64(0), false)
			Expect(err).NotTo(HaveOccurred())
			Expect(Instance.Delta).To(Equal(uint64(1)))
			Expect(Instance.Aggr).To(Equal(uint64(0)))
		})
		It("SetNewAggr with 0", func() {
			Instance := types.UInt64Stat{
				Aggr:  1,
				Delta: 1,
			}
			err := Instance.SetNewAggr(uint64(0))
			Expect(err).NotTo(HaveOccurred())
			Expect(Instance.Delta).To(Equal(uint64(1)))
			Expect(Instance.Aggr).To(Equal(uint64(1)))
		})

		It("SetNewAggr if equal", func() {
			Instance := types.UInt64Stat{
				Aggr:  1,
				Delta: 1,
			}
			err := Instance.SetNewAggr(uint64(1))
			Expect(err).NotTo(HaveOccurred())
			Expect(Instance.Delta).To(Equal(uint64(1)))
			Expect(Instance.Aggr).To(Equal(uint64(1)))
		})

		It("SetNewAggr with new value", func() {
			Instance := types.UInt64Stat{
				Aggr:  1,
				Delta: 1,
			}
			err := Instance.SetNewAggr(uint64(2))
			Expect(err).NotTo(HaveOccurred())
			Expect(Instance.Delta).To(Equal(uint64(1)))
			Expect(Instance.Aggr).To(Equal(uint64(2)))
		})
	})
	Context("UInt64StatCollection", func() {
		var instance types.UInt64StatCollection

		BeforeEach(func() {
			instance = types.UInt64StatCollection{
				Stat: make(map[string]*types.UInt64Stat),
			}
		})
		It("SetAggrStat", func() {
			instance.SetAggrStat("SetAggrStat", uint64(1))
			Expect(instance.Stat["SetAggrStat"].Aggr).To(Equal(uint64(1)))
			instance.SetAggrStat("SetAggrStat", uint64(2))
			Expect(instance.Stat["SetAggrStat"].Aggr).To(Equal(uint64(2)))
			Expect(instance.Stat["SetAggrStat"].Delta).To(Equal(uint64(1)))
			instance.SetAggrStat("SetAggrStat", uint64(0))
			Expect(instance.Stat["SetAggrStat"].Aggr).To(Equal(uint64(2)))
			Expect(instance.Stat["SetAggrStat"].Delta).To(Equal(uint64(1)))
		})
		It("AddDeltaStat", func() {
			instance.AddDeltaStat("AddDeltaStat", uint64(1))
			Expect(instance.Stat["AddDeltaStat"].Aggr).To(Equal(uint64(1)))
			instance.AddDeltaStat("AddDeltaStat", uint64(2))
			Expect(instance.Stat["AddDeltaStat"].Aggr).To(Equal(uint64(3)))
			Expect(instance.Stat["AddDeltaStat"].Delta).To(Equal(uint64(3)))
			instance.AddDeltaStat("AddDeltaStat", uint64(0))
			Expect(instance.Stat["AddDeltaStat"].Aggr).To(Equal(uint64(3)))
			Expect(instance.Stat["AddDeltaStat"].Delta).To(Equal(uint64(3)))
		})
		It("SetDeltaStat", func() {
			instance.SetDeltaStat("SetDeltaStat", uint64(1))
			Expect(instance.Stat["SetDeltaStat"].Aggr).To(Equal(uint64(1)))
			instance.SetDeltaStat("SetDeltaStat", uint64(2))
			Expect(instance.Stat["SetDeltaStat"].Aggr).To(Equal(uint64(3)))
			Expect(instance.Stat["SetDeltaStat"].Delta).To(Equal(uint64(2)))
			instance.SetDeltaStat("SetDeltaStat", uint64(0))
			Expect(instance.Stat["SetDeltaStat"].Aggr).To(Equal(uint64(3)))
			Expect(instance.Stat["SetDeltaStat"].Delta).To(Equal(uint64(2)))
		})
		It("SumAllDeltaValues", func() {
			value := instance.SumAllDeltaValues()
			Expect(value).To(Equal(uint64(0)))
			instance.SetDeltaStat("SumAllDeltaValues", uint64(2))
			value = instance.SumAllDeltaValues()
			Expect(value).To(Equal(uint64(2)))
			instance.SetDeltaStat("SumAllDeltaValues", uint64(0))
			value = instance.SumAllDeltaValues()
			Expect(value).To(Equal(uint64(2)))
			instance.SetDeltaStat("SumAllDeltaValues1", uint64(2))
			value = instance.SumAllDeltaValues()
			Expect(value).To(Equal(uint64(4)))
		})
		It("SumAllAggrValues", func() {
			value := instance.SumAllAggrValues()
			Expect(value).To(Equal(uint64(0)))
			instance.SetAggrStat("SumAllAggrValues", uint64(2))
			value = instance.SumAllAggrValues()
			Expect(value).To(Equal(uint64(2)))
			instance.SetAggrStat("SumAllAggrValues", uint64(0))
			value = instance.SumAllAggrValues()
			Expect(value).To(Equal(uint64(2)))
			instance.SetAggrStat("SumAllAggrValues1", uint64(2))
			value = instance.SumAllAggrValues()
			Expect(value).To(Equal(uint64(4)))
		})
		It("ResetDeltaValues", func() {
			instance.SetDeltaStat("ResetDeltaValues", uint64(1))
			Expect(instance.Stat["ResetDeltaValues"].Aggr).To(Equal(uint64(1)))
			instance.SetDeltaStat("ResetDeltaValues1", uint64(1))
			Expect(instance.Stat["ResetDeltaValues1"].Aggr).To(Equal(uint64(1)))
			instance.ResetDeltaValues()
			Expect(instance.Stat["ResetDeltaValues"].Delta).To(Equal(uint64(0)))
			Expect(instance.Stat["ResetDeltaValues1"].Delta).To(Equal(uint64(0)))
		})
	})
})
