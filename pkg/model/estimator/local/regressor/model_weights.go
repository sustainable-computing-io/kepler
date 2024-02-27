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
	"fmt"
)

var (
	ErrModelWeightsInvalid = fmt.Errorf("ModelWeights is invalid")
)

/*
ModelWeights, AllWeight define structure of feature scale and model weight.

General curvefit uses CurveFit_Weights.
"All_Weights":
	{
		"Categorical_Variables": {"cpu_architecture": {"x86": {"weight": 1.0}}},
		"Numerical_Variables": {"bpf_cpu_time_ms": {"scale": 1.0}, ...},
		"CurveFit_Weights": [1.0, ...]}
	}

Linear uses Numerical_Variables weights and Bias_Weight.
"All_Weights":
	{
		"Categorical_Variables": {"cpu_architecture": {"Sky Lake": {"weight": 1.0}}},
		"Numerical_Variables": {"bpf_cpu_time_ms": {"scale": 1.0, "weight": 1.0}, ...}
		"Bias_Weight": 1.0,
	}
*/

type ModelWeights struct {
	AllWeights `json:"All_Weights"`
}

// getIndexedWeights maps weight index with usageMetrics
func (weights ModelWeights) getIndexedWeights(usageMetrics, systemFeatures []string) (categoricalWeights []map[string]CategoricalFeature, numericalWeights []NormalizedNumericalFeature) {
	w := weights.AllWeights
	for _, m := range systemFeatures {
		categoricalWeights = append(categoricalWeights, w.CategoricalVariables[m])
	}
	for _, m := range usageMetrics {
		numericalWeights = append(numericalWeights, w.NumericalVariables[m])
	}
	return
}

func (weights ModelWeights) getX(usageMetricNames []string, usageMetricValues [][]float64, systemMetaDataFeatureNames, systemMetaDataFeatureValues []string) (categoricalX []float64, numericalX [][]float64, numericalWeights []NormalizedNumericalFeature) {
	categoricalWeights, numericalWeights := weights.getIndexedWeights(usageMetricNames, systemMetaDataFeatureNames)
	categoricalX = make([]float64, len(categoricalWeights))
	numericalX = make([][]float64, len(usageMetricValues))
	for i, coeffMap := range categoricalWeights {
		categoricalX[i] = coeffMap[systemMetaDataFeatureValues[i]].Weight
	}
	for i, vals := range usageMetricValues {
		numericalX[i] = make([]float64, len(numericalWeights))
		for j, coeff := range numericalWeights {
			if coeff.Scale == 0 {
				continue
			}
			numericalX[i][j] = vals[j] / coeff.Scale
		}
	}
	return categoricalX, numericalX, numericalWeights
}

type AllWeights struct {
	CategoricalVariables map[string]map[string]CategoricalFeature `json:"Categorical_Variables"`
	NumericalVariables   map[string]NormalizedNumericalFeature    `json:"Numerical_Variables"`
	BiasWeight           float64                                  `json:"Bias_Weight,omitempty"`
	CurveFitWeights      []float64                                `json:"CurveFit_Weights,omitempty"`
}

type CategoricalFeature struct {
	Weight float64 `json:"weight"`
}
type NormalizedNumericalFeature struct {
	Scale  float64 `json:"scale"` // to normalize the data
	Weight float64 `json:"weight,omitempty"`
}

type ComponentModelWeights map[string]ModelWeights
