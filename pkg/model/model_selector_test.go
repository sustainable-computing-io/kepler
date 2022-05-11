package model

import (
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func getValueMapFromStr(featureStr string) map[string]float32 {
	valueMap := make(map[string]float32)
	for _, feature := range strings.Split(featureStr, ";") {
		valueMap[feature] = fillValue
	}
	return valueMap
}

var _ = Describe("Test power model selector", func() {

	It("load power model with single feature", func() {
		valueMap := getValueMapFromStr(singleFeature)
		_, features, err := SelectModel(valueMap)
		Expect(err).NotTo(HaveOccurred())
		Expect(len(features)).To(Equal(len(valueMap)))
	})

	It("load power model with multiple feature", func() {
		valueMap := getValueMapFromStr(multipleFeatures)
		_, features, err := SelectModel(valueMap)
		Expect(err).NotTo(HaveOccurred())
		Expect(len(features)).Should(BeNumerically(">=", len(valueMap)))
	})

})
