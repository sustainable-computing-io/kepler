package metric

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sustainable-computing-io/kepler/pkg/config"
)

var _ = Describe("Stats", func() {
	It("Test InitAvailableParamAndMetrics", func() {
		config.ExposeHardwareCounterMetrics = false
		clearPlatformDependentAvailability()
		// why metric depends on cgroup?
		// why here is a null pointer?
		InitAvailableParamAndMetrics()
		exp := []string{"bytes_read", "bytes_writes", "block_devices_used"}
		Expect(len(ContainerMetricNames) >= len(exp)).To(BeTrue())
	})
})
