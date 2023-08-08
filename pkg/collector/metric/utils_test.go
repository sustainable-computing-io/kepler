package metric

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/sustainable-computing-io/kepler/pkg/config"
)

func clearPlatformDependentAvailability() {
	AvailableHWCounters = []string{}
	AvailableCGroupMetrics = []string{}
	AvailableKubeletMetrics = []string{}

	ContainerUintFeaturesNames = getcontainerUintFeatureNames()
	ContainerFeaturesNames = []string{}
	ContainerFeaturesNames = append(ContainerFeaturesNames, ContainerUintFeaturesNames...)
	ContainerFeaturesNames = append(ContainerFeaturesNames, blockDeviceLabel)
}

var _ = Describe("Test Metric Unit", func() {

	It("Test getPrometheusMetrics", func() {
		clearPlatformDependentAvailability()

		exp := []string{config.BlockDevicesIO}
		cur := getPrometheusMetrics()
		if len(cur) > len(exp) {
			Expect(exp).To(Equal(cur[len(cur)-len(exp):]))
		} else {
			Expect(exp).To(Equal(cur))
		}
	})

	It("Test isCounterStatEnabled for True", func() {
		AvailableHWCounters = []string{config.BlockDevicesIO}
		exp := isCounterStatEnabled(config.CPUTime)
		Expect(exp).To(BeFalse())
	})

	It("Test isCounterStatEnabled for False", func() {
		AvailableHWCounters = []string{config.BlockDevicesIO}
		exp := isCounterStatEnabled("")
		Expect(exp).To(BeFalse())
	})

	It("Test setEnabledMetrics", func() {
		config.ExposeHardwareCounterMetrics = false
		clearPlatformDependentAvailability()
		setEnabledMetrics()
		exp := []string{config.BlockDevicesIO}
		if len(ContainerFeaturesNames) > len(exp) {
			Expect(exp).To(Equal(ContainerFeaturesNames[len(ContainerFeaturesNames)-len(exp):]))
		} else {
			Expect(exp).To(Equal(ContainerFeaturesNames))
		}
	})
})
