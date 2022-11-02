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
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"

	"github.com/sustainable-computing-io/kepler/pkg/model/types"
)

var (
	SampleDynPowerValue float64 = 100.0

	usageMetrics   = []string{"bytes_read", "bytes_writes", "cache_miss", "cgroupfs_cpu_usage_us", "cgroupfs_memory_usage_bytes", "cgroupfs_system_cpu_usage_us", "cgroupfs_user_cpu_usage_us", "cpu_cycles", "cpu_instr", "cpu_time"}
	systemFeatures = []string{"cpu_architecture"}
	usageValues    = [][]float64{{1, 1, 1, 1, 1, 1, 1, 1, 1, 1}, {1, 1, 1, 1, 1, 1, 1, 1, 1, 1}}
	nodeUsageValue = usageValues[0]
	systemValues   = []string{"Sandy Bridge"}
	empty          = []float64{}
)

var (
	SampleCategoricalFeatures = map[string]CategoricalFeature{
		"Sandy Bridge": {
			Weight: 1.0,
		},
	}
	SampleCoreNumericalVars = map[string]NormalizedNumericalFeature{
		"cpu_cycles": {Weight: 1.0, Mean: 0, Variance: 1},
	}
	SampleDramNumbericalVars = map[string]NormalizedNumericalFeature{
		"cache_miss": {Weight: 1.0, Mean: 0, Variance: 1},
	}
	SampleComponentWeightResponse = ComponentModelWeights{
		"core": genWeights(SampleCoreNumericalVars),
		"dram": genWeights(SampleDramNumbericalVars),
	}
	SamplePowerWeightResponse = genWeights(SampleCoreNumericalVars)

	modelServerPort = 8100
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
	if strings.Contains(req.OutputType, "ComponentModelWeight") {
		err = json.NewEncoder(w).Encode(SampleComponentWeightResponse)
	} else {
		err = json.NewEncoder(w).Encode(SamplePowerWeightResponse)
	}
	if err != nil {
		panic(err)
	}
}

func getHandler(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler.ServeHTTP(w, r)
	})
}

func dummyModelWeightServer(start, quit chan bool) {
	http.HandleFunc("/model", getDummyWeights)
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", modelServerPort))
	if err != nil {
		fmt.Println(modelServerPort)
		panic(err)
	}
	defer listener.Close()
	go func() {
		<-quit
		listener.Close()
	}()
	start <- true
	log.Printf("Server ends: %v\n", http.Serve(listener, getHandler(http.DefaultServeMux)))
}

func genLinearRegressor(outputType types.ModelOutputType, endpoint, initModelURL string) LinearRegressor {
	return LinearRegressor{
		Endpoint:       endpoint,
		UsageMetrics:   usageMetrics,
		OutputType:     outputType,
		SystemFeatures: systemFeatures,
		InitModelURL:   initModelURL,
	}
}

var _ = Describe("Test LR Weight Unit", func() {
	It("UseWeightFromModelServer", func() {
		start := make(chan bool)
		quit := make(chan bool)
		defer close(quit)
		defer close(start)
		go dummyModelWeightServer(start, quit)
		<-start

		// NodeTotalPower
		endpoint := "http://127.0.0.1:8100/model"
		r := genLinearRegressor(types.AbsModelWeight, endpoint, "")
		valid := r.Init()
		Expect(valid).To(Equal(true))
		powers, err := r.GetTotalPower([][]float64{nodeUsageValue}, systemValues)
		Expect(err).NotTo(HaveOccurred())
		Expect(len(powers)).Should(Equal(1))
		Expect(powers[0]).Should(BeEquivalentTo(3))

		// NodeComponentPower
		r = genLinearRegressor(types.AbsComponentModelWeight, endpoint, "")
		valid = r.Init()
		Expect(valid).To(Equal(true))
		compPowers, err := r.GetComponentPower([][]float64{nodeUsageValue}, systemValues)
		Expect(err).NotTo(HaveOccurred())
		Expect(len(compPowers["core"])).Should(Equal(1))
		Expect(compPowers["core"][0]).Should(BeEquivalentTo(3))

		// PodTotalPower
		r = genLinearRegressor(types.DynModelWeight, endpoint, "")
		valid = r.Init()
		Expect(valid).To(Equal(true))
		powers, err = r.GetTotalPower(usageValues, systemValues)
		Expect(err).NotTo(HaveOccurred())
		Expect(len(powers)).Should(Equal(len(usageValues)))
		Expect(powers[0]).Should(BeEquivalentTo(3))

		// PodComponentPower
		r = genLinearRegressor(types.DynComponentModelWeight, endpoint, "")
		valid = r.Init()
		Expect(valid).To(Equal(true))
		compPowers, err = r.GetComponentPower(usageValues, systemValues)
		Expect(err).NotTo(HaveOccurred())
		Expect(len(compPowers["core"])).Should(Equal(len(usageValues)))
		Expect(compPowers["core"][0]).Should(BeEquivalentTo(3))

		quit <- true
	})
	It("UseInitModelURL", func() {
		// NodeComponentPower
		initModelURL := "https://raw.githubusercontent.com/sustainable-computing-io/kepler-model-server/main/tests/test_models/AbsComponentModelWeight/Full/KerasCompWeightFullPipeline/KerasCompWeightFullPipeline.json"
		r := genLinearRegressor(types.AbsComponentModelWeight, "", initModelURL)
		valid := r.Init()
		Expect(valid).To(Equal(true))
		_, err := r.GetComponentPower([][]float64{nodeUsageValue}, systemValues)
		Expect(err).NotTo(HaveOccurred())

		// PodComponentPower
		initModelURL = "https://raw.githubusercontent.com/sustainable-computing-io/kepler-model-server/main/tests/test_models/DynComponentModelWeight/CgroupOnly/ScikitMixed/ScikitMixed.json"
		r = genLinearRegressor(types.DynComponentModelWeight, "", initModelURL)
		valid = r.Init()
		Expect(valid).To(Equal(true))
		_, err = r.GetComponentPower(usageValues, systemValues)
		Expect(err).NotTo(HaveOccurred())
	})
})
