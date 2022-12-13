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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"

	"github.com/sustainable-computing-io/kepler/pkg/config"
	"github.com/sustainable-computing-io/kepler/pkg/model/types"
	"k8s.io/klog/v2"
)

// ModelRequest defines a request to Kepler Model Server to get model weights
type ModelRequest struct {
	ModelName    string   `json:"model_name"`
	MetricNames  []string `json:"metrics"`
	SelectFilter string   `json:"filter"`
	OutputType   string   `json:"output_type"`
}

/*
ModelWeights, AllWeight, CategoricalFeature, NormalizedNumericalFeature define structure of model weight
{
"All_Weights":

		{
		"Bias_Weight": 1.0,
		"Categorical_Variables": {"cpu_architecture": {"Sky Lake": {"weight": 1.0}}},
		"Numerical_Variables": {"cpu_cycles": {"mean": 0, "variance": 1.0, "weight": 1.0}}
		}
	}
*/
type ModelWeights struct {
	AllWeights `json:"All_Weights"`
}
type AllWeights struct {
	BiasWeight           float64                                  `json:"Bias_Weight"`
	CategoricalVariables map[string]map[string]CategoricalFeature `json:"Categorical_Variables"`
	NumericalVariables   map[string]NormalizedNumericalFeature    `json:"Numerical_Variables"`
}
type CategoricalFeature struct {
	Weight float64 `json:"weight"`
}
type NormalizedNumericalFeature struct {
	Mean     float64 `json:"mean"`
	Variance float64 `json:"variance"`
	Weight   float64 `json:"weight"`
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

// predict applies normalization and linear regression to usageValues and systemValues
func (weights ModelWeights) predict(usageMetrics []string, usageValues [][]float64, systemFeatures, systemValues []string) []float64 {
	categoricalWeights, numericalWeights := weights.getIndexedWeights(usageMetrics, systemFeatures)
	basePower := weights.AllWeights.BiasWeight
	for index, coeffMap := range categoricalWeights {
		basePower += coeffMap[systemValues[index]].Weight
	}
	var powers []float64
	for _, vals := range usageValues {
		power := basePower
		for index, coeff := range numericalWeights {
			if coeff.Weight == 0 {
				continue
			}
			// Normalize each Numerical Feature's prediction given Keras calculated Mean and Variance.
			normalizedX := (vals[index] - coeff.Mean) / math.Sqrt(coeff.Variance)
			power += coeff.Weight * normalizedX
		}
		powers = append(powers, power)
	}
	return powers
}

/*
ComponentModelWeights defines structure for multiple (power component's) weights
{
"core":

	{"All_Weights":
	  {
	  "Bias_Weight": 1.0,
	  "Categorical_Variables": {"cpu_architecture": {"Sky Lake": {"weight": 1.0}}},
	  "Numerical_Variables": {"cpu_cycles": {"mean": 0, "variance": 1.0, "weight": 1.0}}
	  }
	},

"dram":
{"All_Weights":

	  {
	  "Bias_Weight": 1.0,
	  "Categorical_Variables": {"cpu_architecture": {"Sky Lake": {"weight": 1.0}}},
	  "Numerical_Variables": {"cache_miss": {"mean": 0, "variance": 1.0, "weight": 1.0}}
	  }
	}
*/
type ComponentModelWeights map[string]ModelWeights

// LinearRegressor defines power estimator with linear regression approach
type LinearRegressor struct {
	Endpoint       string
	UsageMetrics   []string
	OutputType     types.ModelOutputType
	SystemFeatures []string
	ModelName      string
	SelectFilter   string
	InitModelURL   string
	valid          bool
	modelWeight    interface{}
}

// Init returns valid if model weight is obtainable
func (r *LinearRegressor) Init() bool {
	var err error
	var weight interface{}
	outputStr := r.OutputType.String()
	// try getting weight from model server if it is enabled
	if config.ModelServerEnable && config.ModelServerEndpoint != "" {
		weight, err = r.getWeightFromServer()
		klog.V(3).Infof("LR Model (%s): getWeightFromServer: %v", outputStr, weight)
	}
	if weight == nil && r.InitModelURL != "" {
		// next try loading from URL by config
		weight, err = r.loadWeightFromURL()
		klog.V(3).Infof("LR Model (%s): loadWeightFromURL(%v): %v", outputStr, r.InitModelURL, weight)
	}
	if weight != nil {
		r.valid = true
		r.modelWeight = weight
	} else {
		if err == nil {
			klog.V(3).Infof("LR Model (%s): no config", outputStr)
		} else {
			klog.V(3).Infof("LR Model (%s): %v", outputStr, err)
		}
		r.valid = false
	}
	return r.valid
}

// getWeightFromServer tries getting weights for Kepler Model Server
func (r *LinearRegressor) getWeightFromServer() (interface{}, error) {
	modelRequest := ModelRequest{
		ModelName:    r.ModelName,
		MetricNames:  append(r.UsageMetrics, r.SystemFeatures...),
		SelectFilter: r.SelectFilter,
		OutputType:   r.OutputType.String(),
	}
	modelRequestJSON, err := json.Marshal(modelRequest)
	if err != nil {
		return nil, fmt.Errorf("marshal error: %v (%v)", err, modelRequest)
	}

	request, err := http.NewRequestWithContext(context.Background(), http.MethodPost, r.Endpoint, bytes.NewBuffer(modelRequestJSON))
	if err != nil {
		return nil, fmt.Errorf("connection error: %s (%v)", r.Endpoint, err)
	}

	request.Header.Set("Content-Type", "application/json; charset=UTF-8")

	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("connection error: %v (%v)", err, r.Endpoint)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status not ok: %v (%v)", response.Status, modelRequest)
	}
	body, _ := io.ReadAll(response.Body)

	if types.IsComponentType(r.OutputType) {
		var response ComponentModelWeights
		err = json.Unmarshal(body, &response)
		if err != nil {
			return nil, fmt.Errorf("model unmarshal error: %v (%s)", err, string(body))
		}
		return response, nil
	} else {
		var response ModelWeights
		err = json.Unmarshal(body, &response)
		if err != nil {
			return nil, fmt.Errorf("model unmarshal error: %v (%s)", err, string(body))
		}
		return response, nil
	}
}

// loadWeightFromURL tries loading weights from initial model URL
func (r *LinearRegressor) loadWeightFromURL() (interface{}, error) {
	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, r.InitModelURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("connection error: %s (%v)", r.InitModelURL, err)
	}
	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("connection error: %v (%v)", err, r.InitModelURL)
	}

	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	if types.IsComponentType(r.OutputType) {
		var content ComponentModelWeights
		err = json.Unmarshal(body, &content)
		if err != nil {
			return nil, fmt.Errorf("model unmarshal error: %v (%s)", err, string(body))
		}
		return content, nil
	} else {
		var content ModelWeights
		err = json.Unmarshal(body, &content)
		if err != nil {
			return nil, fmt.Errorf("model unmarshal error: %v (%s)", err, string(body))
		}
		return content, nil
	}
}

// GetTotalPower applies ModelWeight prediction and return a list of total powers
func (r *LinearRegressor) GetTotalPower(usageValues [][]float64, systemValues []string) ([]float64, error) {
	if !r.valid {
		return []float64{}, fmt.Errorf("invalid power model call: %s", r.OutputType.String())
	}
	if r.modelWeight != nil {
		return r.modelWeight.(ModelWeights).predict(r.UsageMetrics, usageValues, r.SystemFeatures, systemValues), nil
	}
	return []float64{}, fmt.Errorf("model Weight for model type %s is nil", r.OutputType.String())
}

// GetComponentPower applies each component's ModelWeight prediction and return a map of component powers
func (r *LinearRegressor) GetComponentPower(usageValues [][]float64, systemValues []string) (map[string][]float64, error) {
	if !r.valid {
		return map[string][]float64{}, fmt.Errorf("invalid power model call: %s", r.OutputType.String())
	}
	compPowers := make(map[string][]float64)
	for comp, weight := range r.modelWeight.(ComponentModelWeights) {
		compPowers[comp] = weight.predict(r.UsageMetrics, usageValues, r.SystemFeatures, systemValues)
	}
	return compPowers, nil
}
