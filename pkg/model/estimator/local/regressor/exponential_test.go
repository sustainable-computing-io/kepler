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

	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"

	"github.com/sustainable-computing-io/kepler/pkg/config"
	"github.com/sustainable-computing-io/kepler/pkg/model/types"
)

var (
	exponentialCurveFits = []float64{1, 1, 1}
)

func getDummyExponentialWeights(w http.ResponseWriter, r *http.Request) {
	reqBody, err := io.ReadAll(r.Body)
	if err != nil {
		panic(err)
	}
	var req ModelRequest
	err = json.Unmarshal(reqBody, &req)
	if err != nil {
		panic(err)
	}
	if req.EnergySource == types.ComponentEnergySource {
		err = json.NewEncoder(w).Encode(GenComponentModelWeights(exponentialCurveFits))
	} else {
		err = json.NewEncoder(w).Encode(GenPlatformModelWeights(exponentialCurveFits))
	}
	if err != nil {
		panic(err)
	}
}

var _ = Describe("Test Exponential Predictor Unit", func() {
	It("Get Node Platform Power By Exponential Predictor with ModelServerEndpoint", func() {
		testServer := httptest.NewServer(http.HandlerFunc(getDummyExponentialWeights))
		modelWeightFilepath := config.GetDefaultPowerModelURL(types.AbsPower.String(), types.PlatformEnergySource)
		r := genRegressor(types.AbsPower, types.PlatformEnergySource, testServer.URL, "", modelWeightFilepath, types.ExponentialTrainer)
		err := r.Start()
		Expect(err).To(BeNil())
		r.ResetSampleIdx()
		r.AddNodeFeatureValues(nodeFeatureValues) // add samples to the power model
		powers, err := r.GetPlatformPower(false)
		Expect(err).NotTo(HaveOccurred())
		Expect(len(powers)).Should(Equal(1))
		// TODO: verify if the power makes sense
		Expect(int(powers[0])).Should(BeEquivalentTo(4))
	})
	It("Get Node Components Power By Exponential Predictor with ModelServerEndpoint", func() {
		testServer := httptest.NewServer(http.HandlerFunc(getDummyExponentialWeights))
		modelWeightFilepath := config.GetDefaultPowerModelURL(types.AbsPower.String(), types.ComponentEnergySource)
		r := genRegressor(types.AbsPower, types.ComponentEnergySource, testServer.URL, "", modelWeightFilepath, types.ExponentialTrainer)
		err := r.Start()
		Expect(err).To(BeNil())
		r.ResetSampleIdx()
		r.AddNodeFeatureValues(nodeFeatureValues) // add samples to the power model
		compPowers, err := r.GetComponentsPower(false)
		Expect(err).NotTo(HaveOccurred())
		Expect(len(compPowers)).Should(Equal(1))
		// TODO: verify if the power makes sense
		Expect(int(compPowers[0].Core/1000) * 1000).Should(BeEquivalentTo(4000))
	})
})
