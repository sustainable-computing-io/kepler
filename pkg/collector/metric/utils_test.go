package metric

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/sustainable-computing-io/kepler/pkg/attacher"
)

func clearPlatformDependentAvailability() {
	AvailableCounters = []string{}
	AvailableCgroupMetrics = []string{}
	AvailableKubeletMetrics = []string{}

	ContainerUintFeaturesNames = getcontainerUintFeatureNames()
	ContainerFeaturesNames = []string{}
	ContainerFeaturesNames = append(ContainerFeaturesNames, ContainerUintFeaturesNames...)
	ContainerMetricNames = getEstimatorMetrics()
}

var _ = Describe("Test Metric Unit", func() {

	It("Test getcontainerUintFeatureNames", func() {
		clearPlatformDependentAvailability()

		exp := []string{"cpu_time", "bytes_read", "bytes_writes"}

		cur := getcontainerUintFeatureNames()
		Expect(exp).To(Equal(cur))
	})

	It("Test getPrometheusMetrics", func() {
		clearPlatformDependentAvailability()

		exp := []string{"curr_cpu_time", "total_cpu_time", "curr_bytes_read", "total_bytes_read", "curr_bytes_writes", "total_bytes_writes", "block_devices_used"}
		if attacher.EnableCPUFreq {
			exp = []string{"curr_cpu_time", "total_cpu_time", "curr_bytes_read", "total_bytes_read", "curr_bytes_writes", "total_bytes_writes", "avg_cpu_frequency", "block_devices_used"}
		}
		cur := getPrometheusMetrics()
		Expect(exp).To(Equal(cur))
	})

	It("Test getEstimatorMetrics", func() {
		clearPlatformDependentAvailability()

		exp := []string{"cpu_time", "bytes_read", "bytes_writes", "block_devices_used"}
		cur := getEstimatorMetrics()
		Expect(exp).To(Equal(cur))
	})
})
