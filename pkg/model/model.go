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
	"strings"

	"github.com/sustainable-computing-io/kepler/pkg/config"
	"github.com/sustainable-computing-io/kepler/pkg/model/estimator/local"
	"github.com/sustainable-computing-io/kepler/pkg/model/estimator/local/regressor"
	"github.com/sustainable-computing-io/kepler/pkg/model/estimator/sidecar"
	"github.com/sustainable-computing-io/kepler/pkg/model/types"
	"github.com/sustainable-computing-io/kepler/pkg/sensors/components/source"
	"k8s.io/klog/v2"
)

const (
	idlePower = true  // idlePower is used to define if the function will update idle power
	absPower  = false // absPower is used to define if the function will NOT update idle power, but instead an absulute power
	gauge     = true  // gauge is used to define if the function will update a gauge value
	counter   = false // gauge is used to define if the function will NOT update a gauge value, but instead a counter value
)

var (
	EstimatorSidecarSocket = "/tmp/estimator.sock"
)

// PowerModelInterface defines the power model skeleton
type PowerModelInterface interface {
	// AddProcessFeatureValues adds the new x as a point for training or prediction. Where x are explanatory variable (or the independent variable).
	// x values are added to a sliding window with circular list for dynamic data flow
	AddProcessFeatureValues(x []float64)
	// AddNodeFeatureValues adds the new x as a point for training or prediction. Where x are explanatory variable (or the independent variable).
	AddNodeFeatureValues(x []float64)
	// AddDesiredOutValue adds the new y as a point for training. Where y the response variable (or the dependent variable).
	AddDesiredOutValue(y float64)
	// ResetSampleIdx set the sample sliding window index, setting to 0 to overwrite the old samples with new ones for training or prediction.
	ResetSampleIdx()
	// Train triggers the regression fit after adding data points to create a new power model
	Train() error
	// IsEnabled returns true if the power model was trained and is active
	IsEnabled() bool
	// GetModelType returns if the model is Ratio, Regressor or EstimatorSidecar
	GetModelType() types.ModelType
	// GetProcessFeatureNamesList returns the list of process features that the model was configured to use
	GetProcessFeatureNamesList() []string
	// GetNodeFeatureNamesList returns the list of node features that the model was configured to use
	GetNodeFeatureNamesList() []string
	// GetPlatformPower returns the total Platform Power in Watts associated to each process/process/pod
	// If isIdlePower is true, return the idle power, otherwise return the dynamic or absolute power depending on the model.
	GetPlatformPower(isIdlePower bool) ([]float64, error)
	// GetComponentsPower returns RAPL components Power in Watts associated to each each process/process/pod
	// If isIdlePower is true, return the idle power, otherwise return the dynamic or absolute power depending on the model.
	GetComponentsPower(isIdlePower bool) ([]source.NodeComponentsEnergy, error)
	// GetComponentsPower returns GPU Power in Watts associated to each each process/process/pod
	// If isIdlePower is true, return the idle power, otherwise return the dynamic or absolute power depending on the model.
	GetGPUPower(isIdlePower bool) ([]float64, error)
}

// CreatePowerEstimatorModels checks validity of power model and set estimate functions
func CreatePowerEstimatorModels(processFeatureNames, systemMetaDataFeatureNames, systemMetaDataFeatureValues []string) {
	config.InitModelConfigMap()
	CreateProcessPowerEstimatorModel(processFeatureNames, systemMetaDataFeatureNames, systemMetaDataFeatureValues)
	// Node power estimator uses the process features to estimate node power, expect for the Ratio power model that contains additional metrics.
	CreateNodePlatformPoweEstimatorModel(processFeatureNames, systemMetaDataFeatureNames, systemMetaDataFeatureValues)
	CreateNodeComponentPoweEstimatorModel(processFeatureNames, systemMetaDataFeatureNames, systemMetaDataFeatureValues)
}

// createPowerModelEstimator called by CreatePowerEstimatorModels to initiate estimate function for each power model.
// To estimate the power using the trained models with the model server, we can choose between using the EstimatorSidecar or the Regressor.
// For the built-in Power Model, we have the option to use the Ratio power model.
func createPowerModelEstimator(modelConfig *types.ModelConfig) (PowerModelInterface, error) {
	switch modelConfig.ModelType {
	case types.Ratio:
		model := &local.RatioPowerModel{
			ProcessFeatureNames: modelConfig.ProcessFeatureNames,
			NodeFeatureNames:    modelConfig.NodeFeatureNames,
		}
		klog.V(3).Infof("Using Power Model Ratio")
		return model, nil

	case types.Regressor:
		var featuresNames []string
		if modelConfig.IsNodePowerModel {
			featuresNames = modelConfig.NodeFeatureNames
		} else {
			featuresNames = modelConfig.ProcessFeatureNames
		}
		model := &regressor.Regressor{
			ModelServerEndpoint:         config.ModelServerEndpoint,
			OutputType:                  modelConfig.ModelOutputType,
			EnergySource:                modelConfig.EnergySource,
			TrainerName:                 modelConfig.TrainerName,
			SelectFilter:                modelConfig.SelectFilter,
			ModelWeightsURL:             modelConfig.InitModelURL,
			ModelWeightsFilepath:        modelConfig.InitModelFilepath,
			FloatFeatureNames:           featuresNames,
			SystemMetaDataFeatureNames:  modelConfig.SystemMetaDataFeatureNames,
			SystemMetaDataFeatureValues: modelConfig.SystemMetaDataFeatureValues,
		}
		err := model.Start()
		if err != nil {
			return nil, err
		}
		klog.V(3).Infof("Using Power Model %s", modelConfig.ModelOutputType.String())
		return model, nil

	case types.EstimatorSidecar:
		var featuresNames []string
		if modelConfig.IsNodePowerModel {
			featuresNames = modelConfig.NodeFeatureNames
		} else {
			featuresNames = modelConfig.ProcessFeatureNames
		}
		model := &sidecar.EstimatorSidecar{
			Socket:                      EstimatorSidecarSocket,
			OutputType:                  modelConfig.ModelOutputType,
			TrainerName:                 modelConfig.TrainerName,
			SelectFilter:                modelConfig.SelectFilter,
			FloatFeatureNames:           featuresNames,
			SystemMetaDataFeatureNames:  modelConfig.SystemMetaDataFeatureNames,
			SystemMetaDataFeatureValues: modelConfig.SystemMetaDataFeatureValues,
			EnergySource:                modelConfig.EnergySource,
		}
		err := model.Start()
		if err != nil {
			return nil, err
		}
		klog.V(3).Infof("Using Power Model %s", modelConfig.ModelOutputType.String())
		return model, nil
	}

	err := fmt.Errorf("power Model %s is not supported", modelConfig.ModelType.String())
	klog.V(3).Infof("%v", err)
	return nil, err
}

// CreatePowerModelConfig loads the power model configurations from the ConfigMap, including the Model type, name, filter, URL to download data, and model output type.
// The powerSourceTarget parameter acts as a prefix, which can have values like NODE_TOTAL, NODE_COMPONENTS, CONTAINER_COMPONENTS, etc.
// The complete variable name is created by combining the prefix with the specific attribute.
// For example, if the model name (which the key is MODEL) is under NODE_TOTAL, it will be called NODE_TOTAL_MODEL.
func CreatePowerModelConfig(powerSourceTarget string) *types.ModelConfig {
	modelType := getPowerModelType(powerSourceTarget)
	modelOutputType := getPowerModelOutputType(powerSourceTarget)
	energySource := getPowerModelEnergySource(powerSourceTarget)
	if modelOutputType == types.Unsupported || energySource == "" {
		klog.V(3).Infof("unsupported power source target %s", powerSourceTarget)
		return nil
	}

	modelConfig := types.ModelConfig{
		ModelType:        modelType,
		ModelOutputType:  modelOutputType,
		TrainerName:      getPowerModelTrainerName(powerSourceTarget),
		SelectFilter:     getPowerModelFilter(powerSourceTarget),
		InitModelURL:     getPowerModelDownloadURL(powerSourceTarget),
		IsNodePowerModel: isNodeLevel(powerSourceTarget),
		EnergySource:     energySource,
		NodeFeatureNames: []string{},
	}

	klog.V(3).Infof("Model Config %s: %+v", powerSourceTarget, modelConfig)
	return &modelConfig
}

func getModelConfigKey(modelItem, attribute string) string {
	return fmt.Sprintf("%s_%s", modelItem, attribute)
}

// getPowerModelType return the model type for a given power source, such as platform or components power sources
// The default power model type is Ratio
func getPowerModelType(powerSourceTarget string) (modelType types.ModelType) {
	useEstimatorSidecarStr := config.ModelConfigValues[getModelConfigKey(powerSourceTarget, config.EstimatorEnabledKey)]
	if strings.EqualFold(useEstimatorSidecarStr, "true") {
		modelType = types.EstimatorSidecar
		return
	}
	useLocalRegressor := config.ModelConfigValues[getModelConfigKey(powerSourceTarget, config.LocalRegressorEnabledKey)]
	if strings.EqualFold(useLocalRegressor, "true") {
		modelType = types.Regressor
		return
	}
	// set the default node power model as Regressor
	if powerSourceTarget == config.NodePlatformPowerKey || powerSourceTarget == config.NodeComponentsPowerKey {
		modelType = types.Regressor
		return
	}
	// set the default process power model as Ratio
	modelType = types.Ratio
	return
}

// getPowerModelTrainerName return the trainer name for a given power source, such as platform or components power sources
func getPowerModelTrainerName(powerSourceTarget string) (trainerName string) {
	trainerName = config.ModelConfigValues[getModelConfigKey(powerSourceTarget, config.FixedTrainerNameKey)]
	return
}

// getPowerModelFilter return the model filter for a given power source, such as platform or components power sources
// The model filter is used to select a model, for example selecting a model with the acceptable error: 'mae:0.5'
func getPowerModelFilter(powerSourceTarget string) (selectFilter string) {
	selectFilter = config.ModelConfigValues[getModelConfigKey(powerSourceTarget, config.ModelFiltersKey)]
	return
}

// getPowerModelDownloadURL return the url to download the pre-trained power model for a given power source, such as platform or components power sources
func getPowerModelDownloadURL(powerSourceTarget string) (url string) {
	url = config.ModelConfigValues[getModelConfigKey(powerSourceTarget, config.InitModelURLKey)]
	return
}

// getPowerModelEnergySource returns the energy source.
// It's important to note that although the Prometheus metrics may set the energy source to "trained power model"
// when hardware sensors are not available, the power model creation requires both ComponentEnergySource and
// PlatformEnergySource values. Therefore, we must not replace it here
func getPowerModelEnergySource(powerSourceTarget string) (energySource string) {
	switch powerSourceTarget {
	case config.ContainerPlatformPowerKey:
		return types.PlatformEnergySource
	case config.ContainerComponentsPowerKey:
		return types.ComponentEnergySource
	case config.ProcessPlatformPowerKey:
		return types.PlatformEnergySource
	case config.ProcessComponentsPowerKey:
		return types.ComponentEnergySource
	case config.NodePlatformPowerKey:
		return types.PlatformEnergySource
	case config.NodeComponentsPowerKey:
		return types.ComponentEnergySource
	}
	return ""
}

// getPowerModelOutputType return the model output type for a given power source, such as platform, components, process or node power sources.
// getPowerModelOutputType only affects Regressor or EstimatorSidecar model. The Ratio model does not download data from the Model Server.
// AbsPower for Node, DynPower for process and process
func getPowerModelOutputType(powerSourceTarget string) types.ModelOutputType {
	switch powerSourceTarget {
	case config.ProcessComponentsPowerKey:
		return types.DynPower
	case config.ProcessPlatformPowerKey:
		return types.DynPower
	case config.NodePlatformPowerKey:
		return types.AbsPower
	case config.NodeComponentsPowerKey:
		return types.AbsPower
	}
	return types.Unsupported
}

// isNodeLevel return the true if current power key is node platform or node components
func isNodeLevel(powerSourceTarget string) bool {
	switch powerSourceTarget {
	case config.NodePlatformPowerKey:
		return true
	case config.NodeComponentsPowerKey:
		return true
	}
	return false
}
