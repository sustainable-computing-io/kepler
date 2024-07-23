package bpf

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("PerCPUCounter", func() {

	var counter PerCPUCounter
	BeforeEach(func() {
		counter = NewPerCPUCounter()
	})

	It("should record the correct delta for one time period", func() {
		counter.Start(1, 1, 10)
		key := uint64(1)<<32 | uint64(1)
		Expect(counter.Values[key]).To(Equal(uint64(10)))
		counter.Stop(1, 1, 21)
		Expect(counter.Values).NotTo(ContainElement(key))
		Expect(counter.Total).To(Equal(uint64(11)))
	})

	It("should record the correct delta for an additional time period", func() {
		counter.Start(1, 1, 10)
		key := uint64(1)<<32 | uint64(1)
		Expect(counter.Values[key]).To(Equal(uint64(10)))
		counter.Stop(1, 1, 21)

		Expect(counter.Values).NotTo(ContainElement(key))
		Expect(counter.Total).To(Equal(uint64(11)))
		counter.Start(1, 1, 30)

		Expect(counter.Values[key]).To(Equal(uint64(30)))
		counter.Stop(1, 1, 42)
		Expect(counter.Values).NotTo(ContainElement(key))
		Expect(counter.Total).To(Equal(uint64(23)))
	})

	It("should not increment Total if Start() has not been called", func() {
		counter.Stop(1, 1, 42)
		Expect(counter.Total).To(Equal(uint64(0)))
	})

})
