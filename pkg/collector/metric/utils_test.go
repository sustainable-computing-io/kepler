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
	ContainerMetricNames = getEstimatorMetrics()
}

var _ = Describe("Test Metric Unit", func() {

	It("Test getPrometheusMetrics", func() {
		clearPlatformDependentAvailability()

		exp := []string{"block_devices_used"}
		cur := getPrometheusMetrics()
		if len(cur) > len(exp) {
			Expect(exp).To(Equal(cur[len(cur)-len(exp):]))
		} else {
			Expect(exp).To(Equal(cur))
		}
	})

	It("Test getEstimatorMetrics", func() {
		clearPlatformDependentAvailability()

		exp := []string{"block_devices_used"}
		cur := getEstimatorMetrics()
		if len(cur) > len(exp) {
			Expect(exp).To(Equal(cur[len(cur)-len(exp):]))
		} else {
			Expect(exp).To(Equal(cur))
		}
	})

	It("Test isCounterStatEnabled for True", func() {
		AvailableHWCounters = []string{"block_devices_used"}
		exp := isCounterStatEnabled("cpu_time")
		Expect(exp).To(BeFalse())
	})

	It("Test isCounterStatEnabled for False", func() {
		AvailableHWCounters = []string{"block_devices_used"}
		exp := isCounterStatEnabled("")
		Expect(exp).To(BeFalse())
	})

	It("Test setEnabledMetrics", func() {
		config.ExposeHardwareCounterMetrics = false
		clearPlatformDependentAvailability()
		setEnabledMetrics()
		exp := []string{"block_devices_used"}
		if len(ContainerMetricNames) > len(exp) {
			Expect(exp).To(Equal(ContainerMetricNames[len(ContainerMetricNames)-len(exp):]))
		} else {
			Expect(exp).To(Equal(ContainerMetricNames))
		}
	})
})
