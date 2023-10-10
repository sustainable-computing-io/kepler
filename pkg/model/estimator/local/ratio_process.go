/*
Copyright 2023.

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
calculate Processs' component and other power by ratio approach when node power is available.
*/

//nolint:dupl // the ratio process should be removed
package local

import (
	"math"

	"github.com/sustainable-computing-io/kepler/pkg/model/types"
	"github.com/sustainable-computing-io/kepler/pkg/power/components/source"
)

const MaxProcesss = 2000

// RatioProcessPowerModel stores the feature samples of each process for prediction
// The feature will added ordered from cycles, instructions and cache-misses
type RatioProcessPowerModel struct {
	NodeFeatureNames    []string
	ProcessFeatureNames []string

	processFeatureValues [][]float64 // metrics per process/process/pod
	nodeFeatureValues    []float64   // metrics per process/process/pod
	// xidx represents the features slide window position
	xidx int
}

func (r *RatioProcessPowerModel) getPowerByRatio(processIdx, resUsageFeature, nodePowerFeature int, numProcesss float64) float64 {
	var power float64
	nodeResUsage := r.nodeFeatureValues[resUsageFeature]
	nodePower := r.nodeFeatureValues[nodePowerFeature]
	if nodeResUsage == 0 || resUsageFeature == int(UncoreUsageMetric) {
		power = nodePower / numProcesss
	} else {
		processResUsage := r.processFeatureValues[processIdx][resUsageFeature]
		power = (processResUsage / nodeResUsage) * nodePower
	}
	return math.Ceil(power)
}

// GetPlatformPower applies ModelWeight prediction and return a list of total powers
func (r *RatioProcessPowerModel) GetPlatformPower(isIdlePower bool) ([]float64, error) {
	var processPlatformPower []float64

	// the number of processes is used to evernly divide the power consumption for OTHER and UNCORE
	// we do not use CPU utilization for OTHER and UNCORE because they are not necessarily directly
	numProcesss := float64(r.xidx)

	// estimate the power for each process
	for processIdx := 0; processIdx < r.xidx; processIdx++ {
		var processPower float64
		if isIdlePower {
			// TODO: idle power should be divided accordinly to the process requested resource
			processPower = r.nodeFeatureValues[PlatformIdlePower] / numProcesss
		} else {
			processPower = r.getPowerByRatio(processIdx, int(PlatformUsageMetric), int(PlatformDynPower), numProcesss)
		}
		processPlatformPower = append(processPlatformPower, processPower)
	}
	return processPlatformPower, nil
}

// GetComponentsPower applies each component's ModelWeight prediction and return a map of component powers
func (r *RatioProcessPowerModel) GetComponentsPower(isIdlePower bool) ([]source.NodeComponentsEnergy, error) {
	nodeComponentsPowerOfAllProcesss := []source.NodeComponentsEnergy{}

	// the number of processes is used to evernly divide the power consumption for OTHER and UNCORE
	// we do not use CPU utilization for OTHER and UNCORE because they are not necessarily directly
	numProcesss := float64(r.xidx)

	// estimate the power for each process
	for processIdx := 0; processIdx < r.xidx; processIdx++ {
		var processPower uint64
		processNodeComponentsPower := source.NodeComponentsEnergy{}

		// PKG power
		// TODO: idle power should be divided accordinly to the process requested resource
		if isIdlePower {
			processPower = uint64(r.nodeFeatureValues[PkgIdlePower] / numProcesss)
		} else {
			processPower = uint64(r.getPowerByRatio(processIdx, int(PkgUsageMetric), int(PkgDynPower), numProcesss))
		}
		processNodeComponentsPower.Pkg = processPower

		// CORE power
		if isIdlePower {
			processPower = uint64(r.nodeFeatureValues[CoreIdlePower] / numProcesss)
		} else {
			processPower = uint64(r.getPowerByRatio(processIdx, int(CoreUsageMetric), int(CoreDynPower), numProcesss))
		}
		processNodeComponentsPower.Core = processPower

		// DRAM power
		if isIdlePower {
			processPower = uint64(r.nodeFeatureValues[DramIdlePower] / numProcesss)
		} else {
			processPower = uint64(r.getPowerByRatio(processIdx, int(DramUsageMetric), int(DramDynPower), numProcesss))
		}
		processNodeComponentsPower.DRAM = processPower

		// UNCORE power
		if isIdlePower {
			processPower = uint64(r.nodeFeatureValues[UncoreIdlePower] / numProcesss)
		} else {
			processPower = uint64(r.getPowerByRatio(processIdx, int(UncoreUsageMetric), int(UncoreDynPower), numProcesss))
		}
		processNodeComponentsPower.Uncore = processPower

		nodeComponentsPowerOfAllProcesss = append(nodeComponentsPowerOfAllProcesss, processNodeComponentsPower)
	}
	return nodeComponentsPowerOfAllProcesss, nil
}

// GetComponentsPower returns GPU Power in Watts associated to each each process/process/pod
func (r *RatioProcessPowerModel) GetGPUPower(isIdlePower bool) ([]float64, error) {
	nodeComponentsPowerOfAllProcesss := []float64{}

	// the number of processes is used to evernly divide the power consumption for OTHER and UNCORE
	// we do not use CPU utilization for OTHER and UNCORE because they are not necessarily directly
	numProcesss := float64(r.xidx)

	// estimate the power for each process
	for processIdx := 0; processIdx < r.xidx; processIdx++ {
		var processPower float64

		// TODO: idle power should be divided accordinly to the process requested resource
		if isIdlePower {
			processPower = r.nodeFeatureValues[GpuIdlePower] / numProcesss
		} else {
			processPower = r.getPowerByRatio(processIdx, int(GpuUsageMetric), int(GpuDynPower), numProcesss)
		}
		nodeComponentsPowerOfAllProcesss = append(nodeComponentsPowerOfAllProcesss, processPower)
	}
	return nodeComponentsPowerOfAllProcesss, nil
}

// AddProcessFeatureValues adds the the x for prediction, which is the variable used to calculate the ratio.
// RatioProcessPowerModel is not trained, then we cannot Add training samples, only samples for prediction.
func (r *RatioProcessPowerModel) AddProcessFeatureValues(x []float64) {
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
// RatioProcessPowerModel is not trained, then we cannot Add training samples, only samples for prediction.
func (r *RatioProcessPowerModel) AddNodeFeatureValues(x []float64) {
	for i, feature := range x {
		if i < len(r.nodeFeatureValues) {
			r.nodeFeatureValues[i] = feature
		} else {
			r.nodeFeatureValues = append(r.nodeFeatureValues, feature)
		}
	}
}

// AddDesiredOutValue adds the the y, which is the response variable (or the dependent variable) of regression.
// RatioProcessPowerModel is not trained, then we cannot Add training samples.
func (r *RatioProcessPowerModel) AddDesiredOutValue(y float64) {
}

// ResetSampleIdx set the sample vector index to 0 to overwrite the old samples with new ones for trainning or prediction.
func (r *RatioProcessPowerModel) ResetSampleIdx() {
	r.xidx = 0
	// reset the list if the size is very high to avoid use too much memory
	// we should not reset the list too often to do not put pressure in the garbage collector
	if len(r.processFeatureValues) > MaxProcesss {
		r.processFeatureValues = [][]float64{}
	}
}

// RatioProcessPowerModel is not trained, then this function does nothing.
func (r *RatioProcessPowerModel) Train() error {
	return nil
}

// IsEnabled returns true as Ratio Power model is always active
func (r *RatioProcessPowerModel) IsEnabled() bool {
	return true
}

// GetModelType returns the model type
func (r *RatioProcessPowerModel) GetModelType() types.ModelType {
	return types.Ratio
}

// GetprocessFeatureNamesList returns the list of process features that the model was configured to use
func (r *RatioProcessPowerModel) GetProcessFeatureNamesList() []string {
	return r.ProcessFeatureNames
}

// GetNodeFeatureNamesList returns the list of process features that the model was configured to use
func (r *RatioProcessPowerModel) GetNodeFeatureNamesList() []string {
	return r.NodeFeatureNames
}
