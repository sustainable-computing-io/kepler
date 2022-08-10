package collector

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"fmt"
)



var _ = Describe("Test Metric Unit", func() {
	It("Check feature values", func() {
		Expect(len(uintFeatures)).Should(BeNumerically(">", 0))
		Expect(len(collectedLabel)).Should(BeNumerically(">", 0))
		Expect(len(podEnergyLabels)).Should(BeNumerically(">", 0))
		fmt.Printf("%v\n%v\n%v\n", uintFeatures, collectedLabel, podEnergyLabels)
	})


	It("Check convert values", func() {

		podEnergy := &PodEnergy{
			PodName: "podA",
			Namespace: "default",
			AggEnergyInCore: 10,
			CgroupFSStats: map[string]*UInt64Stat{},
		}

		collectedValues := convertCollectedValues(FLOAT_FEATURES, uintFeatures, podEnergy)
		Expect(len(collectedValues)).To(Equal(len(collectedLabel)))
		fmt.Printf("%v\n", collectedValues)
	})

})