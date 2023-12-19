package stats

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sustainable-computing-io/kepler/pkg/config"
)

var _ = Describe("VMMetric", func() {

	It("Test ResetDeltaValues", func() {
		SetMockedCollectorMetrics()
		vm := NewVMStats(0, "name")
		vm.ResourceUsage[config.CPUTime].AddDeltaStat("socket0", 30000)
		vm.ResetDeltaValues()
		Expect(vm.ResourceUsage[config.CPUTime].SumAllDeltaValues()).To(Equal(uint64(0)))
	})
})
