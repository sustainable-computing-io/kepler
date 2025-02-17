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
	"k8s.io/klog/v2"

	"github.com/sustainable-computing-io/kepler/pkg/config"
	"github.com/sustainable-computing-io/kepler/pkg/model/types"
)

var (
	testNodeFeatureValues = []float64{1000, 1000, 1000, 1000}
	modelProcessFeatures  = []string{config.CPUTime, config.PageCacheHit}
)

func testModel(modelURL, trainerType string, core, dram, pkg int) {
	r := genRegressor(types.AbsPower, types.ComponentEnergySource, "", modelURL, "", trainerType)
	r.FloatFeatureNames = modelProcessFeatures
	err := r.Start()
	Expect(err).To(BeNil())

	r.ResetSampleIdx()
	r.AddNodeFeatureValues(testNodeFeatureValues)

	powers, err := r.GetComponentsPower(false)
	Expect(err).NotTo(HaveOccurred())
	Expect(len(powers)).Should(Equal(1))

	klog.Infof("Core: %v, DRAM: %v, Pkg: %v", powers[0].Core, powers[0].DRAM, powers[0].Pkg)
	Expect(powers[0].Core).Should(BeEquivalentTo(core))
	Expect(powers[0].DRAM).Should(BeEquivalentTo(dram))
	Expect(powers[0].Pkg).Should(BeEquivalentTo(pkg))
}

var _ = Describe("Test Regressor Weight Unit (models from URL)", func() {
	It("Get Node Components Power By SGD Regression with model from URL", func() {
		// Test power calculation. The results should match those from estimator
		// https://github.com/sustainable-computing-io/kepler-model-server/pull/493#discussion_r1795610556
		testModel("https://raw.githubusercontent.com/sustainable-computing-io/kepler-model-db/refs/heads/main/models/v0.7/ec2-0.7.11/rapl-sysfs/AbsPower/BPFOnly/SGDRegressorTrainer_0.json",
			types.LinearRegressionTrainer, 146994, 18704, 146994)
	})

	/* FIXME: the test result is Core: 70824, DRAM: 21137, Pkg: 70824, but the estimator result is Core: 59316, DRAM: 14266, Pkg: 59316, per // https://github.com/sustainable-computing-io/kepler-model-server/pull/493#discussion_r1795610556
	It("Get Node Components Power By Logarithmic Regression with model from URL", func() {
		testModel("https://raw.githubusercontent.com/sustainable-computing-io/kepler-model-db/refs/heads/main/models/v0.7/ec2-0.7.11/rapl-sysfs/AbsPower/BPFOnly/LogarithmicRegressionTrainer_0.json",
			types.LogarithmicTrainer, 59316, 14266, 59316)
	})
	*/

})
