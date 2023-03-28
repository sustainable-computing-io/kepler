package metric

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ProcessMetric", func() {

	It("Test GetBasicValues", func() {
		p := NewProcessMetrics(0, "12345678901234567890")
		exp := []string{"1234567890"}
		cur := p.GetBasicValues()
		Expect(exp).To(Equal(cur))
	})

	It("Test ResetDeltaValues", func() {
		p := NewProcessMetrics(0, "command")
		p.ResetDeltaValues()
		Expect(p.CPUTime.Delta).To(Equal(uint64(0)))
	})

	It("Test SumAllDynDeltaValues", func() {
		p := NewProcessMetrics(0, "command")
		exp := p.DynEnergyInPkg.Delta + p.DynEnergyInGPU.Delta + p.DynEnergyInOther.Delta
		cur := p.SumAllDynDeltaValues()
		Expect(exp).To(Equal(cur))
	})

	It("Test SumAllDynAggrValues", func() {
		p := NewProcessMetrics(0, "command")
		exp := p.DynEnergyInPkg.Aggr + p.DynEnergyInGPU.Aggr + p.DynEnergyInOther.Aggr
		cur := p.SumAllDynAggrValues()
		Expect(exp).To(Equal(cur))
	})
})
