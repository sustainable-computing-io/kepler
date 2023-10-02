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

/*
lr.go
estimate (node/pod) component and total power by linear regression approach when trained model weights are available.
The model weights can be obtained by Kepler Model Server or configured initial model URL.
*/

package local

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"

	collector_metric "github.com/sustainable-computing-io/kepler/pkg/collector/metric"
	"github.com/sustainable-computing-io/kepler/pkg/config"
	"github.com/sustainable-computing-io/kepler/pkg/model/types"
)

var (
	SampleDynPowerValue float64 = 100.0

	containerFeatureNames = []string{
		config.CPUCycle,
		config.CPUInstruction,
		config.CacheMiss,
		config.CgroupfsMemory,
		config.CgroupfsKernelMemory,
		config.CgroupfsTCPMemory,
		config.CgroupfsCPU,
		config.CgroupfsSystemCPU,
		config.CgroupfsUserCPU,
		config.CgroupfsReadIO,
		config.CgroupfsWriteIO,
		config.BlockDevicesIO,
		config.KubeletCPUUsage,
		config.KubeletMemoryUsage,
	}
	systemMetaDataFeatureNames = []string{"cpu_architecture"}
	containerFeatureValues     = [][]float64{
		{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}, // container A
		{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}, // container B
	}
	nodeFeatureValues           = []float64{2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2}
	systemMetaDataFeatureValues = []string{"Sandy Bridge"}
)

var (
	SampleCategoricalFeatures = map[string]CategoricalFeature{
		"Sandy Bridge": {
			Weight: 1.0,
		},
	}
	SampleCoreNumericalVars = map[string]NormalizedNumericalFeature{
		"cpu_cycles": {Weight: 1.0, Scale: 1},
	}
	SampleDramNumbericalVars = map[string]NormalizedNumericalFeature{
		"cache_miss": {Weight: 1.0, Scale: 1},
	}
	SampleComponentWeightResponse = ComponentModelWeights{
		collector_metric.CORE: genWeights(SampleCoreNumericalVars),
		collector_metric.DRAM: genWeights(SampleDramNumbericalVars),
	}
	SamplePlatformWeightResponse = ComponentModelWeights{
		collector_metric.PLATFORM: genWeights(SampleCoreNumericalVars),
	}
)

func genWeights(numericalVars map[string]NormalizedNumericalFeature) ModelWeights {
	return ModelWeights{
		AllWeights{
			BiasWeight:           1.0,
			CategoricalVariables: map[string]map[string]CategoricalFeature{"cpu_architecture": SampleCategoricalFeatures},
			NumericalVariables:   numericalVars,
		},
	}
}

func getDummyWeights(w http.ResponseWriter, r *http.Request) {
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
		err = json.NewEncoder(w).Encode(SampleComponentWeightResponse)
	} else {
		err = json.NewEncoder(w).Encode(SamplePlatformWeightResponse)
	}
	if err != nil {
		panic(err)
	}
}

func genLinearRegressor(outputType types.ModelOutputType, energySource, modelServerEndpoint, modelWeightsURL, modelWeightFilepath string) LinearRegressor {
	config.ModelServerEnable = true
	config.ModelServerEndpoint = modelServerEndpoint
	return LinearRegressor{
		ModelServerEndpoint:         modelServerEndpoint,
		OutputType:                  outputType,
		EnergySource:                energySource,
		FloatFeatureNames:           containerFeatureNames,
		SystemMetaDataFeatureNames:  systemMetaDataFeatureNames,
		SystemMetaDataFeatureValues: systemMetaDataFeatureValues,
		ModelWeightsURL:             modelWeightsURL,
		ModelWeightsFilepath:        modelWeightFilepath,
	}
}

var _ = Describe("Test LR Weight Unit", func() {
	Context("with dummy model server", func() {
		It("Get Node Platform Power By Linear Regression with ModelServerEndpoint", func() {
			testServer := httptest.NewServer(http.HandlerFunc(getDummyWeights))
			modelWeightFilepath := config.GetDefaultPowerModelURL(types.AbsPower.String(), types.PlatformEnergySource)
			r := genLinearRegressor(types.AbsPower, types.PlatformEnergySource, testServer.URL, "", modelWeightFilepath)
			err := r.Start()
			Expect(err).To(BeNil())
			r.ResetSampleIdx()
			r.AddNodeFeatureValues(nodeFeatureValues) // add samples to the power model
			powers, err := r.GetPlatformPower(false)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(powers)).Should(Equal(1))
			// TODO: verify if the power makes sense
			Expect(powers[0]).Should(BeEquivalentTo(4))
		})

		It("Get Node Components Power By Linear Regression Estimator with ModelServerEndpoint", func() {
			testServer := httptest.NewServer(http.HandlerFunc(getDummyWeights))
			modelWeightFilepath := config.GetDefaultPowerModelURL(types.AbsPower.String(), types.ComponentEnergySource)
			r := genLinearRegressor(types.AbsPower, types.ComponentEnergySource, testServer.URL, "", modelWeightFilepath)
			err := r.Start()
			Expect(err).To(BeNil())
			r.ResetSampleIdx()
			r.AddNodeFeatureValues(nodeFeatureValues) // add samples to the power model
			compPowers, err := r.GetComponentsPower(false)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(compPowers)).Should(Equal(1))
			// TODO: verify if the power makes sense
			Expect(compPowers[0].Core).Should(BeEquivalentTo(4000))
		})

		It("Get Container Platform Power By Linear Regression Estimator with ModelServerEndpoint", func() {
			testServer := httptest.NewServer(http.HandlerFunc(getDummyWeights))
			modelWeightFilepath := config.GetDefaultPowerModelURL(types.DynPower.String(), types.PlatformEnergySource)
			r := genLinearRegressor(types.DynPower, types.PlatformEnergySource, testServer.URL, "", modelWeightFilepath)
			err := r.Start()
			Expect(err).To(BeNil())
			r.ResetSampleIdx()
			for _, containerFeatureValues := range containerFeatureValues {
				r.AddContainerFeatureValues(containerFeatureValues) // add samples to the power model
			}
			powers, err := r.GetPlatformPower(false)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(powers)).Should(Equal(len(containerFeatureValues)))
			// TODO: verify if the power makes sense
			Expect(powers[0]).Should(BeEquivalentTo(3))
		})

		It("Get Container Components Power By Linear Regression Estimator with ModelServerEndpoint", func() {
			testServer := httptest.NewServer(http.HandlerFunc(getDummyWeights))
			modelWeightFilepath := config.GetDefaultPowerModelURL(types.DynPower.String(), types.ComponentEnergySource)
			r := genLinearRegressor(types.DynPower, types.ComponentEnergySource, testServer.URL, "", modelWeightFilepath)
			err := r.Start()
			Expect(err).To(BeNil())
			r.ResetSampleIdx()
			for _, containerFeatureValues := range containerFeatureValues {
				r.AddContainerFeatureValues(containerFeatureValues) // add samples to the power model
			}
			compPowers, err := r.GetComponentsPower(false)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(compPowers)).Should(Equal(len(containerFeatureValues)))
			// TODO: verify if the power makes sense
			Expect(compPowers[0].Core).Should(BeEquivalentTo(3000))
		})
	})

	Context("without model server", func() {
		It("Get Node Platform Power By Linear Regression Estimator without ModelServerEndpoint", func() {
			/// Estimate Node Components Power using Linear Regression
			modelWeightFilepath := config.GetDefaultPowerModelURL(types.AbsPower.String(), types.ComponentEnergySource)
			initModelURL := "https://raw.githubusercontent.com/sustainable-computing-io/kepler-model-db/main/models/v0.6/nx12/std_v0.6/acpi/AbsPower/BPFOnly/SGDRegressorTrainer_1.json"
			r := genLinearRegressor(types.AbsPower, types.PlatformEnergySource, "", initModelURL, modelWeightFilepath)
			err := r.Start()
			Expect(err).To(BeNil())
			r.ResetSampleIdx()
			r.AddNodeFeatureValues(nodeFeatureValues) // add samples to the power model
			_, err = r.GetComponentsPower(false)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Get Node Components Power By Linear Regression Estimator without ModelServerEndpoint", func() {
			/// Estimate Node Components Power using Linear Regression
			modelWeightFilepath := config.GetDefaultPowerModelURL(types.AbsPower.String(), types.ComponentEnergySource)
			initModelURL := "https://raw.githubusercontent.com/sustainable-computing-io/kepler-model-db/main/models/v0.6/nx12/std_v0.6/rapl/AbsPower/BPFOnly/SGDRegressorTrainer_1.json"
			r := genLinearRegressor(types.AbsPower, types.ComponentEnergySource, "", initModelURL, modelWeightFilepath)
			err := r.Start()
			Expect(err).To(BeNil())
			r.ResetSampleIdx()
			r.AddNodeFeatureValues(nodeFeatureValues) // add samples to the power model
			_, err = r.GetComponentsPower(false)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Get Container Components Power By Linear Regression Estimator without ModelServerEndpoint", func() {
			// Estimate Container Components Power using Linear Regression
			modelWeightFilepath := config.GetDefaultPowerModelURL(types.DynPower.String(), types.ComponentEnergySource)
			initModelURL := "https://raw.githubusercontent.com/sustainable-computing-io/kepler-model-db/main/models/v0.6/nx12/std_v0.6/acpi/DynPower/BPFOnly/SGDRegressorTrainer_1.json"
			r := genLinearRegressor(types.DynPower, types.PlatformEnergySource, "", initModelURL, modelWeightFilepath)
			err := r.Start()
			Expect(err).To(BeNil())
			r.ResetSampleIdx()
			for _, containerFeatureValues := range containerFeatureValues {
				r.AddContainerFeatureValues(containerFeatureValues) // add samples to the power model
			}
			_, err = r.GetComponentsPower(false)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Get Container Components Power By Linear Regression Estimator without ModelServerEndpoint", func() {
			// Estimate Container Components Power using Linear Regression
			modelWeightFilepath := config.GetDefaultPowerModelURL(types.DynPower.String(), types.ComponentEnergySource)
			initModelURL := "https://raw.githubusercontent.com/sustainable-computing-io/kepler-model-db/main/models/v0.6/nx12/std_v0.6/rapl/DynPower/BPFOnly/SGDRegressorTrainer_1.json"
			r := genLinearRegressor(types.DynPower, types.ComponentEnergySource, "", initModelURL, modelWeightFilepath)
			err := r.Start()
			Expect(err).To(BeNil())
			r.ResetSampleIdx()
			for _, containerFeatureValues := range containerFeatureValues {
				r.AddContainerFeatureValues(containerFeatureValues) // add samples to the power model
			}
			_, err = r.GetComponentsPower(false)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
