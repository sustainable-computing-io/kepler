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

type ModelOutputType int

var (
	ModelOutputTypeConverter = []string{
		"AbsPower", "AbsModelWeight", "AbsComponentPower", "AbsComponentModelWeight", "DynPower", "DynModelWeight", "DynComponentPower", "DynComponentModelWeight",
	}
)

const (
	AbsPower ModelOutputType = iota + 1
	AbsModelWeight
	AbsComponentPower
	AbsComponentModelWeight
	DynPower
	DynModelWeight
	DynComponentPower
	DynComponentModelWeight
)

func (s ModelOutputType) String() string {
	if int(s) <= len(ModelOutputTypeConverter) {
		return ModelOutputTypeConverter[s-1]
	}
	return "unknown"
}

func IsWeightType(outputType ModelOutputType) bool {
	switch outputType {
	case AbsModelWeight, AbsComponentModelWeight, DynModelWeight, DynComponentModelWeight:
		return true
	}
	return false
}

func IsComponentType(outputType ModelOutputType) bool {
	switch outputType {
	case AbsComponentModelWeight, AbsComponentPower, DynComponentModelWeight, DynComponentPower:
		return true
	}
	return false
}

type ModelConfig struct {
	UseEstimatorSidecar bool
	SelectedModel       string
	SelectFilter        string
	InitModelURL        string
}
