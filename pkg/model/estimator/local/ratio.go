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
ratio.go
calculate Processes' component and other power by ratio approach when node power is available.
*/

package local

import (
	"math"

	"github.com/sustainable-computing-io/kepler/pkg/model/types"
	"github.com/sustainable-computing-io/kepler/pkg/sensors/components/source"
)

// The ratio features list follows this order
type ComponentFeatures int
type PlaformFeatures int

// The ratio power model can be used for both node Component and Platform power estimation.
// The models for Component and Platform power estimation have different number of features.
const (
	PkgUsageMetric ComponentFeatures = iota
	CoreUsageMetric
	DramUsageMetric
	UncoreUsageMetric
	OtherUsageMetric
	GpuUsageMetric
	PkgDynPower
	CoreDynPower
	DramDynPower
	UncoreDynPower
	OtherDynPower
	GpuDynPower
	PkgIdlePower
	CoreIdlePower
	DramIdlePower
	UncoreIdlePower
	OtherIdlePower
	GpuIdlePower
)
const (
	// Node features are at the end of the feature list
	PlatformUsageMetric PlaformFeatures = iota
	PlatformDynPower
	PlatformIdlePower
)

// RatioPowerModel stores the feature samples of each process for prediction
// The feature will added ordered from cycles, instructions and cache-misses
type RatioPowerModel struct {
	NodeFeatureNames    []string
	ProcessFeatureNames []string

	processFeatureValues [][]float64 // metrics per process/process/pod
	nodeFeatureValues    []float64   // node metrics
	// xidx represents the features slide window position
	xidx int
}

func (r *RatioPowerModel) getPowerByRatio(processIdx, resUsageFeature, nodePowerFeature int, numProcesses float64) float64 {
	var power float64
	nodeResUsage := r.nodeFeatureValues[resUsageFeature]
	nodePower := r.nodeFeatureValues[nodePowerFeature]
	if nodeResUsage == 0 || resUsageFeature == int(UncoreUsageMetric) {
		power = nodePower / numProcesses
	} else {
		processResUsage := r.processFeatureValues[processIdx][resUsageFeature]
		power = (processResUsage / nodeResUsage) * nodePower
	}

	return math.Ceil(power)
}

// GetPlatformPower applies ModelWeight prediction and return a list of total powers
func (r *RatioPowerModel) GetPlatformPower(isIdlePower bool) ([]float64, error) {
	var processPlatformPower []float64

	// the number of processes is used to evernly divide the power consumption for OTHER and UNCORE
	// we do not use CPU utilization for OTHER and UNCORE because they are not necessarily directly
	numProcesses := float64(r.xidx)

	// estimate the power for each process
	for processIdx := 0; processIdx < r.xidx; processIdx++ {
		var processPower float64
		if isIdlePower {
			// TODO: idle power should be divided accordinly to the process requested resource
			processPower = r.nodeFeatureValues[PlatformIdlePower] / numProcesses
		} else {
			processPower = r.getPowerByRatio(processIdx, int(PlatformUsageMetric), int(PlatformDynPower), numProcesses)
		}
		processPlatformPower = append(processPlatformPower, processPower)
	}
	return processPlatformPower, nil
}

func uint64Division(x, y float64) uint64 {
	return uint64(math.Ceil(x / y))
}

// GetComponentsPower applies each component's ModelWeight prediction and return a map of component powers
func (r *RatioPowerModel) GetComponentsPower(isIdlePower bool) ([]source.NodeComponentsEnergy, error) {
	nodeComponentsPowerOfAllProcesses := []source.NodeComponentsEnergy{}

	// the number of processes is used to evernly divide the power consumption for OTHER and UNCORE
	// we do not use CPU utilization for OTHER and UNCORE because they are not necessarily directly
	numProcesses := float64(r.xidx)

	// estimate the power for each process
	for processIdx := 0; processIdx < r.xidx; processIdx++ {
		var processPower uint64
		processNodeComponentsPower := source.NodeComponentsEnergy{}

		// PKG power
		// TODO: idle power should be divided accordinly to the process requested resource
		if isIdlePower {
			processPower = uint64Division(r.nodeFeatureValues[PkgIdlePower], numProcesses)
		} else {
			processPower = uint64(r.getPowerByRatio(processIdx, int(PkgUsageMetric), int(PkgDynPower), numProcesses))
		}
		processNodeComponentsPower.Pkg = processPower

		// CORE power
		if isIdlePower {
			processPower = uint64Division(r.nodeFeatureValues[CoreIdlePower], numProcesses)
		} else {
			processPower = uint64(r.getPowerByRatio(processIdx, int(CoreUsageMetric), int(CoreDynPower), numProcesses))
		}
		processNodeComponentsPower.Core = processPower

		// DRAM power
		if isIdlePower {
			processPower = uint64Division(r.nodeFeatureValues[DramIdlePower], numProcesses)
		} else {
			processPower = uint64(r.getPowerByRatio(processIdx, int(DramUsageMetric), int(DramDynPower), numProcesses))
		}
		processNodeComponentsPower.DRAM = processPower

		// UNCORE power
		if isIdlePower {
			processPower = uint64Division(r.nodeFeatureValues[UncoreIdlePower], numProcesses)
		} else {
			processPower = uint64(r.getPowerByRatio(processIdx, int(UncoreUsageMetric), int(UncoreDynPower), numProcesses))
		}
		processNodeComponentsPower.Uncore = processPower

		nodeComponentsPowerOfAllProcesses = append(nodeComponentsPowerOfAllProcesses, processNodeComponentsPower)
	}
	return nodeComponentsPowerOfAllProcesses, nil
}

// GetComponentsPower returns GPU Power in Watts associated to each each process/process/pod
func (r *RatioPowerModel) GetGPUPower(isIdlePower bool) ([]float64, error) {
	nodeComponentsPowerOfAllProcesses := []float64{}

	// the number of processes is used to evernly divide the power consumption for OTHER and UNCORE
	// we do not use CPU utilization for OTHER and UNCORE because they are not necessarily directly
	numProcesses := float64(r.xidx)

	// estimate the power for each process
	for processIdx := 0; processIdx < r.xidx; processIdx++ {
		var processPower float64

		// TODO: idle power should be divided accordinly to the process requested resource
		if isIdlePower {
			processPower = r.nodeFeatureValues[GpuIdlePower] / numProcesses
		} else {
			processPower = r.getPowerByRatio(processIdx, int(GpuUsageMetric), int(GpuDynPower), numProcesses)
		}
		nodeComponentsPowerOfAllProcesses = append(nodeComponentsPowerOfAllProcesses, processPower)
	}
	return nodeComponentsPowerOfAllProcesses, nil
}

// AddProcessFeatureValues adds the the x for prediction, which is the variable used to calculate the ratio.
// RatioPowerModel is not trained, then we cannot Add training samples, only samples for prediction.
func (r *RatioPowerModel) AddProcessFeatureValues(x []float64) {
	for i, feature := range x {
		// processFeatureValues is a cyclic list, where we only append a new value if it is necessary.
		if r.xidx < len(r.processFeatureValues) {
			if i < len(r.processFeatureValues[r.xidx]) {
				r.processFeatureValues[r.xidx][i] = feature
			} else {
				r.processFeatureValues[r.xidx] = append(r.processFeatureValues[r.xidx], feature)
			}
		} else {
			// add new process
			r.processFeatureValues = append(r.processFeatureValues, []float64{})
			// add feature of new process
			r.processFeatureValues[r.xidx] = append(r.processFeatureValues[r.xidx], feature)
		}
	}
	r.xidx += 1 // mode pointer to next process
}

// AddNodeFeatureValues adds the the x for prediction, which is the variable used to calculate the ratio.
// RatioPowerModel is not trained, then we cannot Add training samples, only samples for prediction.
func (r *RatioPowerModel) AddNodeFeatureValues(x []float64) {
	for i, feature := range x {
		if i < len(r.nodeFeatureValues) {
			r.nodeFeatureValues[i] = feature
		} else {
			r.nodeFeatureValues = append(r.nodeFeatureValues, feature)
		}
	}
}

// AddDesiredOutValue adds the the y, which is the response variable (or the dependent variable) of regression.
// RatioPowerModel is not trained, then we cannot Add training samples.
func (r *RatioPowerModel) AddDesiredOutValue(y float64) {
}

// ResetSampleIdx set the sample vector index to 0 to overwrite the old samples with new ones for trainning or prediction.
func (r *RatioPowerModel) ResetSampleIdx() {
	r.xidx = 0
}

// RatioPowerModel is not trained, then this function does nothing.
func (r *RatioPowerModel) Train() error {
	return nil
}

// IsEnabled returns true as Ratio Power model is always active
func (r *RatioPowerModel) IsEnabled() bool {
	return true
}

// GetModelType returns the model type
func (r *RatioPowerModel) GetModelType() types.ModelType {
	return types.Ratio
}

// GetProcessFeatureNamesList returns the list of process features that the model was configured to use
func (r *RatioPowerModel) GetProcessFeatureNamesList() []string {
	return r.ProcessFeatureNames
}

// GetNodeFeatureNamesList returns the list of process features that the model was configured to use
func (r *RatioPowerModel) GetNodeFeatureNamesList() []string {
	return r.NodeFeatureNames
}
