package collector

import (
	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/sustainable-computing-io/kepler/pkg/attacher"
)

func clearPlatformDependentAvailability() {
	availableCounters       = []string{}
	availableCgroupMetrics  = []string{}
	availableKubeletMetrics = []string{}
	uintFeatures = getUIntFeatures()
	features = append(FLOAT_FEATURES, uintFeatures...)
}

var _ = Describe("Test Metric Unit", func() {
	It("Check feature values", func() {
		setPodStatProm()
		Expect(len(uintFeatures)).Should(BeNumerically(">", 0))
		Expect(len(podEnergyLabels)).Should(BeNumerically(">", 0))
		Expect(len(podEnergyLabels)).Should(BeNumerically(">", 0))
		fmt.Printf("%v\n%v\n%v\n", uintFeatures, podEnergyLabels, podEnergyLabels)
	})

	It("Test getUIntFeatures", func() {
		clearPlatformDependentAvailability()
		
		exp := []string{"cpu_time", "bytes_read", "bytes_writes"}

		cur := getUIntFeatures()
		Expect(exp).To(Equal(cur))
	})

	It("Test getPrometheusMetrics", func() {
		clearPlatformDependentAvailability()

		exp := []string{"curr_cpu_time", "total_cpu_time", "curr_bytes_read", "total_bytes_read", "curr_bytes_writes", "total_bytes_writes", "block_devices_used"}
		cur := getPrometheusMetrics()
		Expect(exp).To(Equal(cur))

		attacher.EnableCPUFreq = true
		exp = []string{"curr_cpu_time", "total_cpu_time", "curr_bytes_read", "total_bytes_read", "curr_bytes_writes", "total_bytes_writes", "avg_cpu_frequency", "block_devices_used"}
		cur = getPrometheusMetrics()
		Expect(exp).To(Equal(cur))
	})

	It("Test getEstimatorMetrics", func() {
		clearPlatformDependentAvailability()

		exp := []string{"curr_cpu_time", "curr_bytes_read", "curr_bytes_writes", "block_devices_used"}
		cur := getEstimatorMetrics()
		Expect(exp).To(Equal(cur))
	})

	It("Check convert values", func() {
		setPodStatProm()
		v := NewPodEnergy("podA", "default")
		v.EnergyInCore = &UInt64Stat{
			Curr: 10,
			Aggr: 10,
		}
		v.CgroupFSStats = map[string]*UInt64StatCollection{
			CPU_USAGE_TOTAL_KEY: &UInt64StatCollection{
				Stat: map[string]*UInt64Stat{
					"cA": &UInt64Stat{
						Curr: SAMPLE_CURR,
						Aggr: SAMPLE_AGGR,
					},
				},
			},
		}
		collectedValues := v.ToPrometheusValues()
		Expect(len(collectedValues)).To(Equal(len(podEnergyLabels)))
		fmt.Printf("%v\n", collectedValues)
	})

})
