/*
Copyright 2021.

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

package regressor

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"net/http"
	"net/http/httptest"

	"github.com/sustainable-computing-io/kepler/pkg/config"
	"github.com/sustainable-computing-io/kepler/pkg/model/types"
)

var _ = Describe("Test Linear Predictor Unit", func() {
	It("Get Node Platform Power By Linear Regression with ModelServerEndpoint", func() {
		testServer := httptest.NewServer(http.HandlerFunc(getDummyWeights))
		modelWeightFilepath := config.GetDefaultPowerModelURL(types.AbsPower.String(), types.PlatformEnergySource)
		r := genRegressor(types.AbsPower, types.PlatformEnergySource, testServer.URL, "", modelWeightFilepath, types.LinearRegressionTrainer)
		err := r.Start()
		Expect(err).To(BeNil())
		r.ResetSampleIdx()
		r.AddNodeFeatureValues(nodeFeatureValues) // add samples to the power model
		powers, err := r.GetPlatformPower(false)
		Expect(err).NotTo(HaveOccurred())
		Expect(len(powers)).Should(Equal(1))
		// TODO: verify if the power makes sense
		Expect(powers[0]).Should(BeEquivalentTo(3))
	})
	It("Get Node Components Power By Linear Regression with ModelServerEndpoint", func() {
		testServer := httptest.NewServer(http.HandlerFunc(getDummyWeights))
		modelWeightFilepath := config.GetDefaultPowerModelURL(types.AbsPower.String(), types.ComponentEnergySource)
		r := genRegressor(types.AbsPower, types.ComponentEnergySource, testServer.URL, "", modelWeightFilepath, types.LinearRegressionTrainer)
		err := r.Start()
		Expect(err).To(BeNil())
		r.ResetSampleIdx()
		r.AddNodeFeatureValues(nodeFeatureValues) // add samples to the power model
		compPowers, err := r.GetComponentsPower(false)
		Expect(err).NotTo(HaveOccurred())
		Expect(len(compPowers)).Should(Equal(1))
		// TODO: verify if the power makes sense
		Expect(compPowers[0].Core).Should(BeEquivalentTo(3000))
	})
})
