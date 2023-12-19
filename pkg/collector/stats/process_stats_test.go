package stats

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sustainable-computing-io/kepler/pkg/config"
)

var _ = Describe("ProcessMetric", func() {

	It("Test ResetDeltaValues", func() {
		SetMockedCollectorMetrics()
		metrics := CreateMockedProcessStats(1)
		p := metrics[uint64(1)]
		p.ResetDeltaValues()
		Expect(p.ResourceUsage[config.CPUTime].SumAllDeltaValues()).To(Equal(uint64(0)))
	})
})
