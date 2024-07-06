package stats

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sustainable-computing-io/kepler/pkg/bpf"
	"github.com/sustainable-computing-io/kepler/pkg/config"
)

var _ = Describe("Stats", func() {
	It("Test InitAvailableParamAndMetrics", func() {
		config.ExposeHardwareCounterMetrics = false
		supportedMetrics := bpf.DefaultSupportedMetrics()
		InitAvailableParamAndMetrics()
		exp := []string{}
		Expect(len(GetProcessFeatureNames(supportedMetrics)) >= len(exp)).To(BeTrue())
	})
})
