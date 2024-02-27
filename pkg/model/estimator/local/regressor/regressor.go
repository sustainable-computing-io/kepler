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
	"github.com/sustainable-computing-io/kepler/pkg/sensors/components/source"
	"k8s.io/klog/v2"
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

// Predictor defines required implementation for power prediction
type Predictor interface {
	predict(usageMetricNames []string, usageMetricValues [][]float64, systemMetaDataFeatureNames, systemMetaDataFeatureValues []string) []float64
}

// Regressor defines power estimator with regression approach
type Regressor struct {
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

	floatFeatureValues [][]float64 // metrics per process/process/pod/node
	// idle power is calculated with the minimal resource utilization, which means that the system is at rest
	// due to performance reasons, we keep a shadow copy of the floatFeatureValues with 1 values
	floatFeatureValuesForIdlePower [][]float64 // metrics per process/process/pod/node
	// xidx represents the instance slide window position, where an instance can be process/process/pod/node
	xidx int

	enabled         bool
	modelWeight     *ComponentModelWeights
	modelPredictors map[string]Predictor
}

// Start returns nil if model weight is obtainable
func (r *Regressor) Start() error {
	var err error
	var weight *ComponentModelWeights
	outputStr := r.OutputType.String()
	r.enabled = false
	// try getting weight from model server if it is enabled
	if config.ModelServerEnable && config.ModelServerEndpoint != "" {
		weight, err = r.getWeightFromServer()
		klog.V(3).Infof("Regression Model (%s): getWeightFromServer: %v (error: %v)", outputStr, weight, err)
	}
	if weight == nil {
		// next try loading from URL by config
		weight, err = r.loadWeightFromURLorLocal()
		klog.V(3).Infof("Regression Model (%s): loadWeightFromURLorLocal(%v): %v (error: %v)", outputStr, r.ModelWeightsURL, weight, err)
	}
	if weight != nil {
		r.enabled = true
		r.modelWeight = weight
		r.modelPredictors = map[string]Predictor{}
		for component, allWeights := range *weight {
			var predictor Predictor
			predictor, err = r.createPredictor(allWeights)
			if err != nil {
				return err
			}
			r.modelPredictors[component] = predictor
		}
		return nil
	} else {
		if err == nil {
			err = fmt.Errorf("the regression model (%s): has no config", outputStr)
		}
		klog.V(3).Infof("Regression Model (%s): %v", outputStr, err)
	}
	return err
}

// getWeightFromServer tries getting weights for Kepler Model Server
func (r *Regressor) getWeightFromServer() (*ComponentModelWeights, error) {
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
func (r *Regressor) loadWeightFromURLorLocal() (*ComponentModelWeights, error) {
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
func (r *Regressor) loadWeightFromLocal() ([]byte, error) {
	data, err := os.ReadFile(r.ModelWeightsFilepath)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// loadWeightFromURL tries loading weights from initial model URL
func (r *Regressor) loadWeightFromURL() ([]byte, error) {
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

// Create Predictor based on trainer name
func (r *Regressor) createPredictor(weight ModelWeights) (predictor Predictor, err error) {
	switch r.TrainerName {
	case "SGDRegressorTrainer":
		predictor, err = NewLinearPredictor(weight)
	case "LogarithmicRegressionTrainer":
		predictor, err = NewLogarithmicPredictor(weight)
	case "LogisticRegressionTrainer":
		predictor, err = NewLogisticPredictor(weight)
	case "ExponentialRegressionTrainer":
		predictor, err = NewExponentialPredictor(weight)
	default:
		predictor, err = NewLinearPredictor(weight)
	}
	return
}

// GetPlatformPower applies ModelWeight prediction and return a list of power associated to each process/process/pod
func (r *Regressor) GetPlatformPower(isIdlePower bool) ([]float64, error) {
	if !r.enabled {
		return []float64{}, fmt.Errorf("disabled power model call: %s", r.OutputType.String())
	}
	if r.modelPredictors != nil {
		floatFeatureValues := r.floatFeatureValues[0:r.xidx]
		if isIdlePower {
			floatFeatureValues = r.floatFeatureValuesForIdlePower[0:r.xidx]
		}
		if predictor, found := (r.modelPredictors)[config.PLATFORM]; found {
			power := predictor.predict(
				r.FloatFeatureNames, floatFeatureValues,
				r.SystemMetaDataFeatureNames, r.SystemMetaDataFeatureValues)
			return power, nil
		}
		return []float64{}, fmt.Errorf("model Weight for model type %s is not valid: %v", r.OutputType.String(), r.modelWeight)
	}
	return []float64{}, fmt.Errorf("model Weight for model type %s is nil", r.OutputType.String())
}

// GetComponentsPower applies each component's ModelWeight prediction and return a map of component power associated to each process/process/pod
func (r *Regressor) GetComponentsPower(isIdlePower bool) ([]source.NodeComponentsEnergy, error) {
	if !r.enabled {
		return []source.NodeComponentsEnergy{}, fmt.Errorf("disabled power model call: %s", r.OutputType.String())
	}
	if r.modelPredictors == nil {
		r.enabled = false
		return []source.NodeComponentsEnergy{}, fmt.Errorf("model weight is not set")
	}
	compPowers := make(map[string][]float64)
	for comp, predictor := range r.modelPredictors {
		floatFeatureValues := r.floatFeatureValues[0:r.xidx]
		if isIdlePower {
			floatFeatureValues = r.floatFeatureValuesForIdlePower[0:r.xidx]
		}
		compPowers[comp] = predictor.predict(
			r.FloatFeatureNames, floatFeatureValues,
			r.SystemMetaDataFeatureNames, r.SystemMetaDataFeatureValues)
	}

	nodeComponentsPower := []source.NodeComponentsEnergy{}
	num := r.xidx // number of processes
	for index := 0; index < num; index++ {
		pkgPower := utils.GetComponentPower(compPowers, config.PKG, index)
		corePower := utils.GetComponentPower(compPowers, config.CORE, index)
		uncorePower := utils.GetComponentPower(compPowers, config.UNCORE, index)
		dramPower := utils.GetComponentPower(compPowers, config.DRAM, index)
		nodeComponentsPower = append(nodeComponentsPower, utils.FillNodeComponentsPower(pkgPower, corePower, uncorePower, dramPower))
	}

	return nodeComponentsPower, nil
}

// GetComponentsPower returns GPU Power in Watts associated to each each process
func (r *Regressor) GetGPUPower(isIdlePower bool) ([]float64, error) {
	return []float64{}, fmt.Errorf("current power model does not support GPUs")
}

func (r *Regressor) addFloatFeatureValues(x []float64) {
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
			// add new process
			r.floatFeatureValues = append(r.floatFeatureValues, []float64{})
			r.floatFeatureValuesForIdlePower = append(r.floatFeatureValuesForIdlePower, []float64{})
			// add feature of new process
			r.floatFeatureValues[r.xidx] = append(r.floatFeatureValues[r.xidx], feature)
			r.floatFeatureValuesForIdlePower[r.xidx] = append(r.floatFeatureValuesForIdlePower[r.xidx], 0)
		}
	}
	r.xidx += 1 // mode pointer to next process
}

// AddProcessFeatureValues adds the the x for prediction, which are the explanatory variables (or the independent variable) of regression.
// Regressor is trained off-line then we cannot Add training samples. We might implement it in the future.
// The Regressor does not differentiate node or process power estimation, the difference will only be the amount of resource utilization
func (r *Regressor) AddProcessFeatureValues(x []float64) {
	r.addFloatFeatureValues(x)
}

// AddNodeFeatureValues adds the the x for prediction, which is the variable used to calculate the ratio.
// Regressor is not trained, then we cannot Add training samples, only samples for prediction.
// The Regressor does not differentiate node or process power estimation, the difference will only be the amount of resource utilization
func (r *Regressor) AddNodeFeatureValues(x []float64) {
	r.addFloatFeatureValues(x)
}

// AddDesiredOutValue adds the the y, which is the response variable (or the dependent variable) of regression.
// Regressor is trained off-line then we do not add Y for trainning. We might implement it in the future.
func (r *Regressor) AddDesiredOutValue(y float64) {
}

// ResetSampleIdx set the sample vector index to 0 to overwrite the old samples with new ones for trainning or prediction.
func (r *Regressor) ResetSampleIdx() {
	r.xidx = 0
}

// Train triggers the regressiong fit after adding data points to create a new power model.
// Regressor is trained off-line then we cannot trigger the trainning. We might implement it in the future.
func (r *Regressor) Train() error {
	return nil
}

// IsEnabled returns true if the power model was trained and is active
func (r *Regressor) IsEnabled() bool {
	return r.enabled
}

// GetModelType returns the model type
func (r *Regressor) GetModelType() types.ModelType {
	return types.Regressor
}

// GetProcessFeatureNamesList returns the list of float features that the model was configured to use
// The Regressor does not differentiate node or process power estimation, the difference will only be the amount of resource utilization
func (r *Regressor) GetProcessFeatureNamesList() []string {
	return r.FloatFeatureNames
}

// GetNodeFeatureNamesList returns the list of float features that the model was configured to use
// The Regressor does not differentiate node or process power estimation, the difference will only be the amount of resource utilization
func (r *Regressor) GetNodeFeatureNamesList() []string {
	return r.FloatFeatureNames
}
