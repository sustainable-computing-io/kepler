package collector

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Test for active containers", func() {
	It("Get Active CPUs", func() {
		var cpuTime [128]uint16
		cpuTime[3] = 1
		cpuTime[5] = 1
		cpuTime[12] = 1
		ac := getActiveCPUs(&cpuTime)
		Expect(3).To(Equal(len(ac)))
		Expect(int32(3)).To(Equal(ac[0]))
		Expect(int32(5)).To(Equal(ac[1]))
		Expect(int32(12)).To(Equal(ac[2]))
	})

	It("getAVGCPUFreqAndTotalCPUTime", func() {
		var cpuTime [128]uint16
		cpuTime[3] = 1
		cpuFrequency := make(map[int32]uint64)
		cpuFrequency[int32(3)] = 1
		avgFreq, totalCPUTime, activeCPUs := getAVGCPUFreqAndTotalCPUTime(cpuFrequency, &cpuTime)
		Expect(float64(1)).To(Equal(avgFreq))
		Expect(uint64(1)).To(Equal(totalCPUTime))
		Expect(int32(3)).To(Equal(activeCPUs[0]))
	})
})
