/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package local

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("XGBoost", func() {
	var _ = Describe("Test XGBoost load and predict", func() {
		It("XGBoost load and predict", func() {
			model := &XGBoostModelWeight{}
			err := model.LoadFromJson("test_data/dmlc_xgboost_model.json")
			Expect(err).NotTo(HaveOccurred())
			defer model.Close()

			data := []float32{1.0, 2.0, 3.0}
			expectedPredictions := []float64{0.5, 0.5, 0.5}

			predictions, err := model.PredictFromData(data)
			Expect(err).NotTo(HaveOccurred())
			/* the data has 3 rows and 1 column, there should be 3 preditions */
			Expect(len(predictions)).To(Equal(len(expectedPredictions)))
			for i, prediction := range predictions {
				Expect(prediction).To(Equal(expectedPredictions[i]))
			}
		})
	})
})
