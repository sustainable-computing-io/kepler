package stats

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sustainable-computing-io/kepler/pkg/config"
)

var _ = Describe("Stats", func() {
	It("Test InitAvailableParamAndMetrics", func() {
		_, err := config.Initialize(".")
		Expect(err).NotTo(HaveOccurred())

		config.SetEnabledHardwareCounterMetrics(false)
		exp := []string{}
		Expect(len(GetProcessFeatureNames()) >= len(exp)).To(BeTrue())
	})
})
