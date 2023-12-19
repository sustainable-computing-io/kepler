package stats

import (
	"runtime"

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
		if runtime.GOOS == "linux" {
			exp := []string{
				config.BytesReadIO,
				config.BytesWriteIO,
				config.BlockDevicesIO,
			}
			Expect(len(ProcessFeaturesNames) >= len(exp)).To(BeTrue())
		}
		if runtime.GOOS == "darwin" {
			exp := []string{config.BlockDevicesIO}
			Expect(len(ProcessFeaturesNames) >= len(exp)).To(BeTrue())
		}
	})
})
