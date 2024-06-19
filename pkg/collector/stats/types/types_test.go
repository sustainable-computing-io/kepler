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
	"sync"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sustainable-computing-io/kepler/pkg/collector/stats/types"
)

var _ = Describe("Types", func() {
	Context("UInt64Stat", func() {
		It("ResetDeltaValues", func() {
			Instance := types.NewUInt64Stat(0, 1)
			Instance.ResetDeltaValues()
			Expect(Instance.GetDelta()).To(Equal(uint64(0)))
		})
		It("AddNewDelta", func() {
			Instance := types.NewUInt64Stat(0, 1)
			err := Instance.AddNewDelta(uint64(1))
			Expect(err).NotTo(HaveOccurred())
			Expect(Instance.GetDelta()).To(Equal(uint64(2)))
			Expect(Instance.GetAggr()).To(Equal(uint64(1)))
		})
		It("SetNewDelta", func() {
			Instance := types.NewUInt64Stat(0, 1)
			err := Instance.SetNewDelta(uint64(1))
			Expect(err).NotTo(HaveOccurred())
			Expect(Instance.GetAggr()).To(Equal(uint64(1)))
			Expect(Instance.GetDelta()).To(Equal(uint64(1)))
		})
		It("SetNewDeltaValue", func() {
			Instance := types.NewUInt64Stat(0, 1)
			err := Instance.SetNewDeltaValue(uint64(0), false)
			Expect(err).NotTo(HaveOccurred())
			Expect(Instance.GetDelta()).To(Equal(uint64(1)))
			Expect(Instance.GetAggr()).To(Equal(uint64(0)))
		})
		It("SetNewAggr with 0", func() {
			Instance := types.NewUInt64Stat(1, 1)
			err := Instance.SetNewAggr(uint64(0))
			Expect(err).NotTo(HaveOccurred())
			Expect(Instance.GetDelta()).To(Equal(uint64(1)))
			Expect(Instance.GetAggr()).To(Equal(uint64(1)))
		})

		It("SetNewAggr if equal", func() {
			Instance := types.NewUInt64Stat(1, 1)
			err := Instance.SetNewAggr(uint64(1))
			Expect(err).NotTo(HaveOccurred())
			Expect(Instance.GetDelta()).To(Equal(uint64(1)))
			Expect(Instance.GetAggr()).To(Equal(uint64(1)))
		})

		It("SetNewAggr with new value", func() {
			Instance := types.NewUInt64Stat(1, 1)
			err := Instance.SetNewAggr(uint64(2))
			Expect(err).NotTo(HaveOccurred())
			Expect(Instance.GetDelta()).To(Equal(uint64(1)))
			Expect(Instance.GetAggr()).To(Equal(uint64(2)))
		})
		It("Can be modified by multiple goroutines", func() {
			Instance := types.NewUInt64Stat(0, 0)
			wg := sync.WaitGroup{}
			errChan := make(chan error, 1000)
			for i := 0; i < 1000; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					errChan <- Instance.AddNewDelta(1)
				}()
			}
			wg.Wait()
			close(errChan)
			for e := range errChan {
				Expect(e).NotTo(HaveOccurred())
			}
			Expect(Instance.GetDelta()).To(Equal(uint64(1000)))
		})
	})
	Context("UInt64StatCollection", func() {
		var instance types.UInt64StatCollection

		BeforeEach(func() {
			instance = make(types.UInt64StatCollection)
		})
		It("SetAggrStat", func() {
			instance.SetAggrStat("SetAggrStat", uint64(1))
			Expect(instance["SetAggrStat"].GetAggr()).To(Equal(uint64(1)))
			instance.SetAggrStat("SetAggrStat", uint64(2))
			Expect(instance["SetAggrStat"].GetAggr()).To(Equal(uint64(2)))
			Expect(instance["SetAggrStat"].GetDelta()).To(Equal(uint64(1)))
			instance.SetAggrStat("SetAggrStat", uint64(0))
			Expect(instance["SetAggrStat"].GetAggr()).To(Equal(uint64(2)))
			Expect(instance["SetAggrStat"].GetDelta()).To(Equal(uint64(1)))
		})
		It("AddDeltaStat", func() {
			instance.AddDeltaStat("AddDeltaStat", uint64(1))
			Expect(instance["AddDeltaStat"].GetAggr()).To(Equal(uint64(1)))
			instance.AddDeltaStat("AddDeltaStat", uint64(2))
			Expect(instance["AddDeltaStat"].GetAggr()).To(Equal(uint64(3)))
			Expect(instance["AddDeltaStat"].GetDelta()).To(Equal(uint64(3)))
			instance.AddDeltaStat("AddDeltaStat", uint64(0))
			Expect(instance["AddDeltaStat"].GetAggr()).To(Equal(uint64(3)))
			Expect(instance["AddDeltaStat"].GetDelta()).To(Equal(uint64(3)))
		})
		It("SetDeltaStat", func() {
			instance.SetDeltaStat("SetDeltaStat", uint64(1))
			Expect(instance["SetDeltaStat"].GetAggr()).To(Equal(uint64(1)))
			instance.SetDeltaStat("SetDeltaStat", uint64(2))
			Expect(instance["SetDeltaStat"].GetAggr()).To(Equal(uint64(3)))
			Expect(instance["SetDeltaStat"].GetDelta()).To(Equal(uint64(2)))
			instance.SetDeltaStat("SetDeltaStat", uint64(0))
			Expect(instance["SetDeltaStat"].GetAggr()).To(Equal(uint64(3)))
			Expect(instance["SetDeltaStat"].GetDelta()).To(Equal(uint64(2)))
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
			Expect(instance["ResetDeltaValues"].GetAggr()).To(Equal(uint64(1)))
			instance.SetDeltaStat("ResetDeltaValues1", uint64(1))
			Expect(instance["ResetDeltaValues1"].GetAggr()).To(Equal(uint64(1)))
			instance.ResetDeltaValues()
			Expect(instance["ResetDeltaValues"].GetDelta()).To(Equal(uint64(0)))
			Expect(instance["ResetDeltaValues1"].GetDelta()).To(Equal(uint64(0)))
		})
	})
})
