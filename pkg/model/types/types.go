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

package types

type ModelType int
type ModelOutputType int

var (
	ModelOutputTypeConverter = []string{
		"AbsPower", "DynPower",
	}
	ModelTypeConverter = []string{
		"Ratio", "LinearRegressor", "EstimatorSidecar",
	}
)

const (
	// Power Model types
	Ratio            ModelType = iota + 1 // estimation happens within kepler without using Model Server
	LinearRegressor                       // estimation happens within kepler, but pre-trained model parameters are downloaded externally
	EstimatorSidecar                      // estimation happens in the sidecar with a loaded pre-trained power model
)
const (
	// Power Model Output types
	// Absolute Power Model (AbsPower): is the power model trained by measured power (including the idle power)
	// Dynamic Power Model (DynPower): is the power model trained by dynamic power (AbsPower - idle power)
	AbsPower ModelOutputType = iota + 1
	DynPower
	Unsupported
)

var (
	// Define energy source
	PlatformEnergySource  = "acpi"
	ComponentEnergySource = "rapl"
)

func (s ModelOutputType) String() string {
	if int(s) <= len(ModelOutputTypeConverter) {
		return ModelOutputTypeConverter[s-1]
	}
	return "unknown"
}

func (s ModelType) String() string {
	if int(s) <= len(ModelTypeConverter) {
		return ModelTypeConverter[s-1]
	}
	return "unknown"
}

type ModelConfig struct {
	// model configuration
	ModelType         ModelType
	ModelOutputType   ModelOutputType
	TrainerName       string
	EnergySource      string
	SelectFilter      string
	InitModelURL      string
	InitModelFilepath string

	IsNodePowerModel bool

	// initial samples to start the model
	ContainerFeatureNames       []string
	NodeFeatureNames            []string
	SystemMetaDataFeatureNames  []string
	SystemMetaDataFeatureValues []string
}
