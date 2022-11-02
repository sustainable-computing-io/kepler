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

package model

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sustainable-computing-io/kepler/pkg/power/rapl/source"
)

var _ = Describe("PodPower", func() {
	Context("with edge case", func() {
		It("should return array with length", func() {
			containerMetricValuesOnly := make([][]float64, 0)
			NodeMetadataValues := make([]string, 0)
			nodeTotalPowerPerComponents := source.RAPLPower{}
			componentPodPowers, otherPodPowers := GetContainerPower(
				containerMetricValuesOnly,
				NodeMetadataValues,
				0,
				0,
				nodeTotalPowerPerComponents)
			lenComponentPodPowers := len(componentPodPowers)
			lenOtherPodPowers := len(otherPodPowers)
			Expect(lenComponentPodPowers).Should(BeNumerically(">", -1))
			Expect(lenOtherPodPowers).Should(BeNumerically(">", -1))
			Expect(lenComponentPodPowers).Should(BeNumerically("==", lenOtherPodPowers))
		})
	})
})
