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
	"net/http"
	"os"

	"github.com/sustainable-computing-io/kepler/pkg/config"
	"github.com/sustainable-computing-io/kepler/pkg/model/types"
	"github.com/sustainable-computing-io/kepler/pkg/model/utils"
	"github.com/sustainable-computing-io/kepler/pkg/power/components/source"
	"k8s.io/klog/v2"

	collector_metric "github.com/sustainable-computing-io/kepler/pkg/collector/metric"
)

const (
	// TODO: determine node type dynamically
	defaultNodeType int = 1
)

// ModelRequest defines a request to Kepler Model Server to get model weights
type ModelRequest struct {
	MetricNames  []string `json:"metrics"`
	OutputType   string   `json:"output_type"`
	EnergySource string   `json:"source"`
	NodeType     int      `json:"node_type"`
	Weight       bool     `json:"weight"`
	TrainerName  string   `json:"trainer_name"`
	SelectFilter string   `json:"filter"`
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
	Scale  float64 `json:"scale"` // to normalize the data
	Weight float64 `json:"weight"`
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

// predict applies normalization and linear regression to usageMetricValues and systemMetaDataFeatureValues
func (weights ModelWeights) predict(usageMetricNames []string, usageMetricValues [][]float64, systemMetaDataFeatureNames, systemMetaDataFeatureValues []string) []float64 {
	categoricalWeights, numericalWeights := weights.getIndexedWeights(usageMetricNames, systemMetaDataFeatureNames)
	basePower := weights.AllWeights.BiasWeight
	for index, coeffMap := range categoricalWeights {
		basePower += coeffMap[systemMetaDataFeatureValues[index]].Weight
	}
	var powers []float64
	for _, vals := range usageMetricValues {
		power := basePower
		for index, coeff := range numericalWeights {
			if coeff.Weight == 0 {
				continue
			}
			normalizedX := vals[index] / coeff.Scale
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
	ModelServerEndpoint  string
	OutputType           types.ModelOutputType
	EnergySource         string
	TrainerName          string
	SelectFilter         string
	ModelWeightsURL      string
	ModelWeightsFilepath string

	FloatFeatureNames           []string
	SystemMetaDataFeatureNames  []string
	SystemMetaDataFeatureValues []string

	floatFeatureValues [][]float64 // metrics per process/container/pod/node
	// idle power is calculated with the minimal resource utilization, which means that the system is at rest
	// due to performance reasons, we keep a shadow copy of the floatFeatureValues with 1 values
	floatFeatureValuesForIdlePower [][]float64 // metrics per process/container/pod/node
	// xidx represents the instance slide window position, where an instance can be process/container/pod/node
	xidx int

	enabled     bool
	modelWeight *ComponentModelWeights
}

// Start returns nil if model weight is obtainable
func (r *LinearRegressor) Start() error {
	var err error
	var weight *ComponentModelWeights
	outputStr := r.OutputType.String()
	r.enabled = false
	// try getting weight from model server if it is enabled
	if config.ModelServerEnable && config.ModelServerEndpoint != "" {
		weight, err = r.getWeightFromServer()
		klog.V(3).Infof("LR Model (%s): getWeightFromServer: %v (error: %v)", outputStr, weight, err)
	}
	if weight == nil {
		// next try loading from URL by config
		weight, err = r.loadWeightFromURLorLocal()
		klog.V(3).Infof("LR Model (%s): loadWeightFromURLorLocal(%v): %v (error: %v)", outputStr, r.ModelWeightsURL, weight, err)
	}
	if weight != nil {
		r.enabled = true
		r.modelWeight = weight
		return nil
	} else {
		if err == nil {
			err = fmt.Errorf("the model LR (%s): has no config", outputStr)
		}
		klog.V(3).Infof("LR Model (%s): %v", outputStr, err)
	}
	return err
}

// getWeightFromServer tries getting weights for Kepler Model Server
func (r *LinearRegressor) getWeightFromServer() (*ComponentModelWeights, error) {
	modelRequest := ModelRequest{
		MetricNames:  append(r.FloatFeatureNames, r.SystemMetaDataFeatureNames...),
		OutputType:   r.OutputType.String(),
		EnergySource: r.EnergySource,
		TrainerName:  r.TrainerName,
		SelectFilter: r.SelectFilter,
		NodeType:     defaultNodeType,
		Weight:       true,
	}
	modelRequestJSON, err := json.Marshal(modelRequest)
	if err != nil {
		return nil, fmt.Errorf("marshal error: %v (%v)", err, modelRequest)
	}

	request, err := http.NewRequestWithContext(context.Background(), http.MethodPost, r.ModelServerEndpoint, bytes.NewBuffer(modelRequestJSON))
	if err != nil {
		return nil, fmt.Errorf("connection error: %s (%v)", r.ModelServerEndpoint, err)
	}

	request.Header.Set("Content-Type", "application/json; charset=UTF-8")

	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("connection error: %v (%v)", err, r.ModelServerEndpoint)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status not ok: %v (%v)", response.Status, modelRequest)
	}
	body, _ := io.ReadAll(response.Body)

	var powerResonse ComponentModelWeights
	err = json.Unmarshal(body, &powerResonse)
	if err != nil {
		return nil, fmt.Errorf("model unmarshal error: %v (%s)", err, string(body))
	}
	return &powerResonse, nil
}

// loadWeightFromURLorLocal get weight from either local or URL
// if string start with '/', we take it as local file
func (r *LinearRegressor) loadWeightFromURLorLocal() (*ComponentModelWeights, error) {
	var body []byte
	var err error

	body, err = r.loadWeightFromURL()
	if err != nil {
		body, err = r.loadWeightFromLocal()
		if err != nil {
			return nil, err
		}
	}
	var content ComponentModelWeights
	err = json.Unmarshal(body, &content)
	if err != nil {
		return nil, fmt.Errorf("model unmarshal error: %v (%s)", err, string(body))
	}
	return &content, nil
}

// loadWeightFromLocal tries loading weights from local file given by r.ModelWeightsURL
func (r *LinearRegressor) loadWeightFromLocal() ([]byte, error) {
	data, err := os.ReadFile(r.ModelWeightsFilepath)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// loadWeightFromURL tries loading weights from initial model URL
func (r *LinearRegressor) loadWeightFromURL() ([]byte, error) {
	if r.ModelWeightsURL == "" {
		return nil, fmt.Errorf("ModelWeightsURL is empty")
	}
	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, r.ModelWeightsURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("connection error: %s (%v)", r.ModelWeightsURL, err)
	}
	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("connection error: %v (%v)", err, r.ModelWeightsURL)
	}

	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	return body, nil
}

// GetPlatformPower applies ModelWeight prediction and return a list of power associated to each process/container/pod
func (r *LinearRegressor) GetPlatformPower(isIdlePower bool) ([]float64, error) {
	if !r.enabled {
		return []float64{}, fmt.Errorf("disabled power model call: %s", r.OutputType.String())
	}
	if r.modelWeight != nil {
		floatFeatureValues := r.floatFeatureValues[0:r.xidx]
		if isIdlePower {
			floatFeatureValues = r.floatFeatureValuesForIdlePower[0:r.xidx]
		}
		if modelWeight, found := (*r.modelWeight)[collector_metric.PLATFORM]; found {
			power := modelWeight.predict(
				r.FloatFeatureNames, floatFeatureValues,
				r.SystemMetaDataFeatureNames, r.SystemMetaDataFeatureValues)
			return power, nil
		}
		return []float64{}, fmt.Errorf("model Weight for model type %s is not valid: %v", r.OutputType.String(), r.modelWeight)
	}
	return []float64{}, fmt.Errorf("model Weight for model type %s is nil", r.OutputType.String())
}

// GetComponentsPower applies each component's ModelWeight prediction and return a map of component power associated to each process/container/pod
func (r *LinearRegressor) GetComponentsPower(isIdlePower bool) ([]source.NodeComponentsEnergy, error) {
	if !r.enabled {
		return []source.NodeComponentsEnergy{}, fmt.Errorf("disabled power model call: %s", r.OutputType.String())
	}
	if r.modelWeight == nil {
		r.enabled = false
		return []source.NodeComponentsEnergy{}, fmt.Errorf("model weight is not set")
	}
	compPowers := make(map[string][]float64)
	for comp, weight := range *r.modelWeight {
		floatFeatureValues := r.floatFeatureValues[0:r.xidx]
		if isIdlePower {
			floatFeatureValues = r.floatFeatureValuesForIdlePower[0:r.xidx]
		}
		compPowers[comp] = weight.predict(
			r.FloatFeatureNames, floatFeatureValues,
			r.SystemMetaDataFeatureNames, r.SystemMetaDataFeatureValues)
	}

	nodeComponentsPower := []source.NodeComponentsEnergy{}
	num := r.xidx // number of processes/containers/pods
	for index := 0; index < num; index++ {
		pkgPower := utils.GetComponentPower(compPowers, collector_metric.PKG, index)
		corePower := utils.GetComponentPower(compPowers, collector_metric.CORE, index)
		uncorePower := utils.GetComponentPower(compPowers, collector_metric.UNCORE, index)
		dramPower := utils.GetComponentPower(compPowers, collector_metric.DRAM, index)
		nodeComponentsPower = append(nodeComponentsPower, utils.FillNodeComponentsPower(pkgPower, corePower, uncorePower, dramPower))
	}

	return nodeComponentsPower, nil
}

// GetComponentsPower returns GPU Power in Watts associated to each each process/container/pod
func (r *LinearRegressor) GetGPUPower(isIdlePower bool) ([]float64, error) {
	return []float64{}, fmt.Errorf("current power model does not support GPUs")
}

func (r *LinearRegressor) addFloatFeatureValues(x []float64) {
	for i, feature := range x {
		// floatFeatureValues is a cyclic list, where we only append a new value if it is necessary.
		if r.xidx < len(r.floatFeatureValues) {
			if i < len(r.floatFeatureValues[r.xidx]) {
				r.floatFeatureValues[r.xidx][i] = feature
				// we don't need to add idle power since it is already set as 0
			} else {
				r.floatFeatureValues[r.xidx] = append(r.floatFeatureValues[r.xidx], feature)
				r.floatFeatureValuesForIdlePower[r.xidx] = append(r.floatFeatureValuesForIdlePower[r.xidx], 0)
			}
		} else {
			// add new container
			r.floatFeatureValues = append(r.floatFeatureValues, []float64{})
			r.floatFeatureValuesForIdlePower = append(r.floatFeatureValuesForIdlePower, []float64{})
			// add feature of new container
			r.floatFeatureValues[r.xidx] = append(r.floatFeatureValues[r.xidx], feature)
			r.floatFeatureValuesForIdlePower[r.xidx] = append(r.floatFeatureValuesForIdlePower[r.xidx], 0)
		}
	}
	r.xidx += 1 // mode pointer to next container
}

// AddContainerFeatureValues adds the the x for prediction, which are the explanatory variables (or the independent variable) of regression.
// LinearRegressor is trained off-line then we cannot Add training samples. We might implement it in the future.
// The LinearRegressor does not differentiate node or container power estimation, the difference will only be the amount of resource utilization
func (r *LinearRegressor) AddContainerFeatureValues(x []float64) {
	r.addFloatFeatureValues(x)
}

// AddNodeFeatureValues adds the the x for prediction, which is the variable used to calculate the ratio.
// LinearRegressor is not trained, then we cannot Add training samples, only samples for prediction.
// The LinearRegressor does not differentiate node or container power estimation, the difference will only be the amount of resource utilization
func (r *LinearRegressor) AddNodeFeatureValues(x []float64) {
	r.addFloatFeatureValues(x)
}

// AddDesiredOutValue adds the the y, which is the response variable (or the dependent variable) of regression.
// LinearRegressor is trained off-line then we do not add Y for trainning. We might implement it in the future.
func (r *LinearRegressor) AddDesiredOutValue(y float64) {
}

// ResetSampleIdx set the sample vector index to 0 to overwrite the old samples with new ones for trainning or prediction.
func (r *LinearRegressor) ResetSampleIdx() {
	r.xidx = 0
}

// Train triggers the regressiong fit after adding data points to create a new power model.
// LinearRegressor is trained off-line then we cannot trigger the trainning. We might implement it in the future.
func (r *LinearRegressor) Train() error {
	return nil
}

// IsEnabled returns true if the power model was trained and is active
func (r *LinearRegressor) IsEnabled() bool {
	return r.enabled
}

// GetModelType returns the model type
func (r *LinearRegressor) GetModelType() types.ModelType {
	return types.LinearRegressor
}

// GetContainerFeatureNamesList returns the list of float features that the model was configured to use
// The LinearRegressor does not differentiate node or container power estimation, the difference will only be the amount of resource utilization
func (r *LinearRegressor) GetContainerFeatureNamesList() []string {
	return r.FloatFeatureNames
}

// GetNodeFeatureNamesList returns the list of float features that the model was configured to use
// The LinearRegressor does not differentiate node or container power estimation, the difference will only be the amount of resource utilization
func (r *LinearRegressor) GetNodeFeatureNamesList() []string {
	return r.FloatFeatureNames
}
