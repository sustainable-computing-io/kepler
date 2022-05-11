package model

import (
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const (
	modelName        = "Sequential"
	singleFeature    = "CPU_TIME"
	multipleFeatures = "CPU_TIME;CPU_TIME_mean;CPU_TIME_max;CPU_TIME_std"

	fillValue = 0.1
)

func getValueMap(model *PowerModel) map[string]float32 {
	valueMap := make(map[string]float32)
	for _, feature := range model.Features {
		valueMap[feature] = fillValue
	}
	return valueMap
}

var _ = Describe("Test power model handler", func() {

	It("load power model with single feature", func() {
		modelFilename := fmt.Sprintf("%s_cols_%s.tflite", modelName, singleFeature)
		features := strings.Split(singleFeature, ";")
		model, err := LoadPowerModel(modelFilename, features)
		Expect(err).NotTo(HaveOccurred())
		Expect(len(model.Features)).To(Equal(1))
		valueMap := getValueMap(model)
		result := model.GetPower(valueMap)
		fmt.Printf("power = %.3f\n", result)
	})

	It("load power model with multiple feature", func() {
		modelFilename := fmt.Sprintf("%s_cols_%s.tflite", modelName, multipleFeatures)
		features := strings.Split(multipleFeatures, ";")
		model, err := LoadPowerModel(modelFilename, features)
		Expect(err).NotTo(HaveOccurred())
		Expect(len(model.Features)).Should(BeNumerically(">", 0))
		valueMap := getValueMap(model)
		result := model.GetPower(valueMap)
		fmt.Printf("power = %.3f\n", result)
	})

})
