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

	"github.com/sustainable-computing-io/kepler/pkg/model/types"
)

var (
	logarithmicCurveFits          = []float64{1, 1, 1}
	dummyLogarithmicWeightHandler = genHandlerFunc(logarithmicCurveFits)
)

var _ = Describe("Test Logarithmic Predictor Unit", func() {
	It("Get Node Platform Power By Logarithmic Regression", func() {
		powers := GetNodePlatformPowerFromDummyServer(dummyLogarithmicWeightHandler, types.LogarithmicTrainer)
		Expect(int(powers[0])).Should(BeEquivalentTo(2))
	})

	It("Get Node Components Power By Logarithmic Regression", func() {
		compPowers := GetNodeComponentsPowerFromDummyServer(dummyLogarithmicWeightHandler, types.LogarithmicTrainer)
		Expect(int(compPowers[0].Core/1000) * 1000).Should(BeEquivalentTo(2000))
	})
})
