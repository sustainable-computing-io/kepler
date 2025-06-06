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

	"github.com/sustainable-computing-io/kepler/pkg/config"
	"github.com/sustainable-computing-io/kepler/pkg/model/types"
)

var (
	exponentialCurveFits          = []float64{1, 1, 1}
	dummyExponentialWeightHandler = genHandlerFunc(exponentialCurveFits, types.ExponentialTrainer)
)

var _ = Describe("Test Exponential Predictor Unit", func() {
	BeforeEach(func() {
		_, err := config.Initialize(".")
		Expect(err).ShouldNot(HaveOccurred())
	})

	It("Get Node Platform Power By Exponential Regression", func() {
		powers := GetNodePlatformPowerFromDummyServer(dummyExponentialWeightHandler, types.ExponentialTrainer)
		Expect(simplifyOutputInMilliJoules(powers[0])).Should(BeEquivalentTo(4000))
	})

	It("Get Node Components Power By Exponential Regression", func() {
		compPowers := GetNodeComponentsPowerFromDummyServer(dummyExponentialWeightHandler, types.ExponentialTrainer)
		Expect(simplifyOutputInMilliJoules(compPowers[0].Core)).Should(BeEquivalentTo(4000))
	})
})
