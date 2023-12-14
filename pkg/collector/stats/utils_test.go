package stats

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/sustainable-computing-io/kepler/pkg/config"
)

func clearPlatformDependentAvailability() {
	AvailableBPFSWCounters = []string{config.CPUTime}
	AvailableBPFHWCounters = []string{}
	AvailableCGroupMetrics = []string{}

	ProcessFeaturesNames = getProcessFeatureNames()
}

var _ = Describe("Test Metric Unit", func() {
	It("Test isCounterStatEnabled for True", func() {
		AvailableBPFHWCounters = []string{config.BlockDevicesIO}
		exp := isCounterStatEnabled(config.CPUTime)
		Expect(exp).To(BeFalse())
	})

	It("Test isCounterStatEnabled for False", func() {
		AvailableBPFHWCounters = []string{config.BlockDevicesIO}
		exp := isCounterStatEnabled("")
		Expect(exp).To(BeFalse())
	})

	It("Test setEnabledProcessMetrics", func() {
		config.ExposeHardwareCounterMetrics = false
		clearPlatformDependentAvailability()
		setEnabledProcessMetrics()
		exp := []string{config.CPUTime}
		Expect(exp).To(Equal(ProcessFeaturesNames))
	})
})
