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
	"fmt"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sustainable-computing-io/kepler/pkg/collector/stats"
	"github.com/sustainable-computing-io/kepler/pkg/config"
	"github.com/sustainable-computing-io/kepler/pkg/model/utils"
	"github.com/sustainable-computing-io/kepler/pkg/sensors/components"
	"github.com/sustainable-computing-io/kepler/pkg/sensors/components/source"
	"github.com/sustainable-computing-io/kepler/pkg/sensors/platform"
)

var _ = Describe("Test Model Unit", func() {

	BeforeEach(func() {
		source.SystemCollectionSupported = false // disable the system power collection to use the prediction power model
		stats.SetMockedCollectorMetrics()

		configStr := "CONTAINER_COMPONENTS_INIT_URL=https://raw.githubusercontent.com/sustainable-computing-io/kepler-model-server/test_models/tests/test_models/DynComponentModelWeight/CgroupOnly/ScikitMixed/ScikitMixed.json\n"
		os.Setenv("MODEL_CONFIG", configStr)
		// we need to disable the system real time power metrics for testing since we add mock values or use power model estimator
		components.SetIsSystemCollectionSupported(false)
		platform.SetIsSystemCollectionSupported(false)
	})

	It("Test GetModelConfigMap()", func() {
		configStr := "CONTAINER_COMPONENTS_ESTIMATOR=true\nCONTAINER_COMPONENTS_INIT_URL=https://raw.githubusercontent.com/sustainable-computing-io/kepler-model-server/test_models/tests/test_models/DynComponentPower/CgroupOnly/ScikitMixed/ScikitMixed.json\n"
		os.Setenv("MODEL_CONFIG", configStr)
		configValues := config.GetModelConfigMap()
		modelItem := "CONTAINER_COMPONENTS"
		fmt.Printf("%s: %s", getModelConfigKey(modelItem, config.EstimatorEnabledKey), configValues[getModelConfigKey(modelItem, config.EstimatorEnabledKey)])
		useEstimatorSidecarStr := configValues[getModelConfigKey(modelItem, config.EstimatorEnabledKey)]
		Expect(useEstimatorSidecarStr).To(Equal("true"))
		initModelURL := configValues[getModelConfigKey(modelItem, config.InitModelURLKey)]
		Expect(initModelURL).NotTo(Equal(""))
	})

	Context("utils", func() {
		DescribeTable("Test GetCoreRatio()", func(isIdlePower bool, inCoreRatio float64, expectedCoreRatio float64) {
			coreRatio := utils.GetCoreRatio(isIdlePower, inCoreRatio)
			Expect(coreRatio).To(Equal(expectedCoreRatio))
		},
			Entry("DynPower with valid coreRatio < 1", false, 0.5, 1.0),
			Entry("IdlePower with valid coreRatio < 1", true, 0.5, 0.5),
			Entry("IdlePower with valid coreRatio = 1", true, 1.0, 1.0),
			Entry("IdlePower with invalid coreRatio = 0", true, 0.0, 1.0),
			Entry("IdlePower with invalid coreRatio = -1", true, -1.0, 1.0),
			Entry("IdlePower with invalid coreRatio > 1", true, 1.2, 1.0),
		)

		DescribeTable("Test GetModelNameFromURL()", func(url, expectedModelName string) {
			Expect(utils.GetModelNameFromURL(url)).To(Equal(expectedModelName))
		},
			Entry("empty", "", ""),
			Entry("some model with multiple subfolders", "http://some/path/to/some_model.json", "some_model"),
			Entry("some model with direct path", "http://some_model.json", "some_model"),
			Entry("some model without file extension", "http://some_model", "some_model"),
		)
	})

})
