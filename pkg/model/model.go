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
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"

	"github.com/jszwec/csvutil"
	"github.com/sustainable-computing-io/kepler/pkg/power/rapl/source"
)

type Coeff struct {
	Architecture  string  `csv:"architecture"`
	CPUTime       float64 `csv:"cpu_time"`
	CPUCycle      float64 `csv:"cpu_cycle"`
	CPUInstr      float64 `csv:"cpu_instruction"`
	MemoryUsage   float64 `csv:"memory_usage"`
	CacheMisses   float64 `csv:"cache_misses"`
	InterceptCore float64 `csv:"intercept_core"`
	InterceptDram float64 `csv:"intercept_dram"`
}

type EnergyPrediction struct {
	Architecture   string
	CPUTime        float64
	CPUCycle       float64
	CPUInstr       float64
	ResidentMemory float64
	CacheMisses    float64
}

type CategoricalFeature struct {
	Name   string  `json:"name"`
	Weight float64 `json:"weight"`
}

type NormalizedNumericalFeature struct {
	Mean     float64 `json:"mean"`
	Variance float64 `json:"variance"`
	Weight   float64 `json:"weight"`
}

type CoreModelServerCoeff struct {
	AllWeights struct {
		BiasWeight           float64 `json:"Bias_Weight"`
		CategoricalVariables struct {
			CPUArchitecture []CategoricalFeature `json:"cpu_architecture"`
		} `json:"Categorical_Variables"`
		NumericalVariables struct {
			CPUCycle NormalizedNumericalFeature `json:"cpu_cycles"`
			CPUTime  NormalizedNumericalFeature `json:"cpu_time"`
			CPUInstr NormalizedNumericalFeature `json:"cpu_instr"`
		} `json:"Numerical_Variables"`
	} `json:"All_Weights"`
}

type DramModelServerCoeff struct {
	AllWeights struct {
		BiasWeight           float64 `json:"Bias_Weight"`
		CategoricalVariables struct {
			CPUArchitecture []CategoricalFeature `json:"cpu_architecture"`
		} `json:"Categorical_Variables"`
		NumericalVariables struct {
			CacheMisses    NormalizedNumericalFeature `json:"cache_misses"`
			ResidentMemory NormalizedNumericalFeature `json:"container_memory_working_set_bytes"`
		} `json:"Numerical_Variables"`
	} `json:"All_Weights"`
}

type LinearEnergyModelServerCoeff struct {
	CoreModelServerCoeff
	DramModelServerCoeff
}

var (
	// obtained the coeff via regression
	BareMetalCoeff = Coeff{
		CPUTime:       0.0,
		CPUCycle:      0.0000005268224465,
		CPUInstr:      0.0000005484982329,
		InterceptCore: 152121.0472,

		MemoryUsage:   0.0,
		CacheMisses:   0.000004112383656,
		InterceptDram: -23.70284983,
	}
	// if per counters are not avail on VMs, don't use them
	VMCoeff = Coeff{
		CPUTime:     1.0,
		CPUCycle:    0,
		CPUInstr:    0,
		MemoryUsage: 1.0,
		CacheMisses: 0,
	}
	RunTimeCoeff Coeff

	modelServerEndpoint string

	powerModelPath = "/var/lib/kepler/data/power_model.csv"
)

func SetVMCoeff() {
	RunTimeCoeff = VMCoeff
}

func SetBMCoeff() {
	// use the default one if no model found
	RunTimeCoeff = BareMetalCoeff
	arch, err := source.GetCPUArchitecture()
	if err == nil {
		cpuArch := arch
		file, err := os.Open(powerModelPath)
		if err == nil {
			reader := csv.NewReader(file)

			dec, err := csvutil.NewDecoder(reader)
			if err == nil {
				for {
					var p Coeff
					if err := dec.Decode(&p); err == io.EOF {
						break
					}
					if p.Architecture == cpuArch {
						fmt.Printf("use model %v\n", p)
						RunTimeCoeff = p
					}
				}
			}
		}
	}
}

func SetRuntimeCoeff(coeff Coeff) {
	RunTimeCoeff = coeff
}

func SetModelServerEndpoint(ep string) {
	modelServerEndpoint = ep
}

func getRequest(endpoint string) (*http.Response, error) {
	req, err := http.NewRequest("GET", endpoint, http.NoBody)
	if err != nil {
		return nil, errors.New("could not create request for Model Server Endpoint " + endpoint)
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, errors.New("request for Model Server Endpoint failed " + endpoint)
	}
	return res, nil
}

func GetCoeffFromModelServer() (*LinearEnergyModelServerCoeff, error) {
	if modelServerEndpoint == "" {
		return nil, nil
	}
	coreRes, coreErr := getRequest(modelServerEndpoint + "/model-weights/")
	dramRes, dramErr := getRequest(modelServerEndpoint + "/model-weights/dram_model")
	if coreErr != nil {
		return nil, coreErr
	}
	if dramErr != nil {
		return nil, dramErr
	}
	var coreModelServerCoeff CoreModelServerCoeff
	var dramModelServerCoeff DramModelServerCoeff
	coreBodyError := json.NewDecoder(coreRes.Body).Decode(&coreModelServerCoeff)
	dramBodyError := json.NewDecoder(dramRes.Body).Decode(&dramModelServerCoeff)
	defer coreRes.Body.Close()
	defer dramRes.Body.Close()
	if coreBodyError != nil || dramBodyError != nil {
		return nil, errors.New("failed to parse response body")
	}
	energyModelServerCoeff := LinearEnergyModelServerCoeff{coreModelServerCoeff, dramModelServerCoeff}
	return &energyModelServerCoeff, nil
}

// Retrieve corresponding coefficient given Categorical Feature name

func retrieveCoeffForCategoricalVariable(categoricalPrediction string, allCategoricalFeatures []CategoricalFeature) (float64, error) {
	for _, architecture := range allCategoricalFeatures {
		if architecture.Name == categoricalPrediction {
			return architecture.Weight, nil
		}
	}
	return -1, errors.New("architecture feature does not exist")
}

// Using Direct Access instead of Dynamic lookup to retrieve Numerical Weights. Direct access is more efficient and easier
// to implement, but it makes the code less flexible if more fields need to be added to DramModelServerCoeff or
// CoreModelServerCoeff.

func predictLinearDramEnergyConsumption(prediction *EnergyPrediction, dramModelServerCoeff *DramModelServerCoeff) (float64, error) {
	var energyPrediction float64 = 0
	dramCPUArchitectureWeights := dramModelServerCoeff.AllWeights.CategoricalVariables.CPUArchitecture
	numericalWeights := dramModelServerCoeff.AllWeights.NumericalVariables
	biasWeight := dramModelServerCoeff.AllWeights.BiasWeight
	energyPrediction += biasWeight
	weightRes, err := retrieveCoeffForCategoricalVariable(prediction.Architecture, dramCPUArchitectureWeights)
	if err != nil {
		return -1, err
	}
	energyPrediction += weightRes
	// Normalize each Numerical Feature's prediction given Keras calculated Mean and Variance.
	normalizedCacheMissPredict := (prediction.CacheMisses - numericalWeights.CacheMisses.Mean) / math.Sqrt(numericalWeights.CacheMisses.Variance)
	energyPrediction += numericalWeights.CacheMisses.Weight * normalizedCacheMissPredict
	normalizedResidentMemoryPredict := (prediction.ResidentMemory - numericalWeights.ResidentMemory.Mean) / math.Sqrt(numericalWeights.ResidentMemory.Variance)
	energyPrediction += numericalWeights.ResidentMemory.Weight * normalizedResidentMemoryPredict
	return energyPrediction, nil
}

func predictLinearCoreEnergyConsumption(prediction *EnergyPrediction, coreModelServerCoeff *CoreModelServerCoeff) (float64, error) {
	var energyPrediction float64 = 0
	coreCPUArchitectureWeights := coreModelServerCoeff.AllWeights.CategoricalVariables.CPUArchitecture
	numericalWeights := coreModelServerCoeff.AllWeights.NumericalVariables
	biasWeight := coreModelServerCoeff.AllWeights.BiasWeight
	energyPrediction += biasWeight
	weightRes, err := retrieveCoeffForCategoricalVariable(prediction.Architecture, coreCPUArchitectureWeights)
	if err != nil {
		return -1, err
	}
	energyPrediction += weightRes
	// Normalize each Numerical Feature's prediction given Keras calculated Mean and Variance.
	normalizedCPUTimePredict := (prediction.CPUTime - numericalWeights.CPUTime.Mean) / math.Sqrt(numericalWeights.CPUTime.Variance)
	energyPrediction += numericalWeights.CPUTime.Weight * normalizedCPUTimePredict
	normalizedCPUCyclePredict := (prediction.CPUCycle - numericalWeights.CPUCycle.Mean) / math.Sqrt(numericalWeights.CPUCycle.Variance)
	energyPrediction += numericalWeights.CPUCycle.Weight * normalizedCPUCyclePredict
	normalizedCPUInstrPredict := (prediction.CPUInstr - numericalWeights.CPUInstr.Mean) / math.Sqrt(numericalWeights.CPUInstr.Variance)
	energyPrediction += numericalWeights.CPUInstr.Weight * normalizedCPUInstrPredict
	return energyPrediction, nil
}
