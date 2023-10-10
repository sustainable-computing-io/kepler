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
calculate Containers' component and other power by ratio approach when node power is available.
*/

//nolint:dupl // the ratio process should be removed
package local

import (
	"math"

	"github.com/sustainable-computing-io/kepler/pkg/model/types"
	"github.com/sustainable-computing-io/kepler/pkg/power/components/source"
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

// RatioPowerModel stores the feature samples of each container for prediction
// The feature will added ordered from cycles, instructions and cache-misses
type RatioPowerModel struct {
	NodeFeatureNames      []string
	ContainerFeatureNames []string

	containerFeatureValues [][]float64 // metrics per process/container/pod
	nodeFeatureValues      []float64   // node metrics
	// xidx represents the features slide window position
	xidx int
}

func (r *RatioPowerModel) getPowerByRatio(containerIdx, resUsageFeature, nodePowerFeature int, numContainers float64) float64 {
	var power float64
	nodeResUsage := r.nodeFeatureValues[resUsageFeature]
	nodePower := r.nodeFeatureValues[nodePowerFeature]
	if nodeResUsage == 0 || resUsageFeature == int(UncoreUsageMetric) {
		power = nodePower / numContainers
	} else {
		containerResUsage := r.containerFeatureValues[containerIdx][resUsageFeature]
		power = (containerResUsage / nodeResUsage) * nodePower
	}
	return math.Ceil(power)
}

// GetPlatformPower applies ModelWeight prediction and return a list of total powers
func (r *RatioPowerModel) GetPlatformPower(isIdlePower bool) ([]float64, error) {
	var containerPlatformPower []float64

	// the number of containers is used to evernly divide the power consumption for OTHER and UNCORE
	// we do not use CPU utilization for OTHER and UNCORE because they are not necessarily directly
	numContainers := float64(r.xidx)

	// estimate the power for each container
	for containerIdx := 0; containerIdx < r.xidx; containerIdx++ {
		var containerPower float64
		if isIdlePower {
			// TODO: idle power should be divided accordinly to the container requested resource
			containerPower = r.nodeFeatureValues[PlatformIdlePower] / numContainers
		} else {
			containerPower = r.getPowerByRatio(containerIdx, int(PlatformUsageMetric), int(PlatformDynPower), numContainers)
		}
		containerPlatformPower = append(containerPlatformPower, containerPower)
	}
	return containerPlatformPower, nil
}

// GetComponentsPower applies each component's ModelWeight prediction and return a map of component powers
func (r *RatioPowerModel) GetComponentsPower(isIdlePower bool) ([]source.NodeComponentsEnergy, error) {
	nodeComponentsPowerOfAllContainers := []source.NodeComponentsEnergy{}

	// the number of containers is used to evernly divide the power consumption for OTHER and UNCORE
	// we do not use CPU utilization for OTHER and UNCORE because they are not necessarily directly
	numContainers := float64(r.xidx)

	// estimate the power for each container
	for containerIdx := 0; containerIdx < r.xidx; containerIdx++ {
		var containerPower uint64
		containerNodeComponentsPower := source.NodeComponentsEnergy{}

		// PKG power
		// TODO: idle power should be divided accordinly to the container requested resource
		if isIdlePower {
			containerPower = uint64(r.nodeFeatureValues[PkgIdlePower] / numContainers)
		} else {
			containerPower = uint64(r.getPowerByRatio(containerIdx, int(PkgUsageMetric), int(PkgDynPower), numContainers))
		}
		containerNodeComponentsPower.Pkg = containerPower

		// CORE power
		if isIdlePower {
			containerPower = uint64(r.nodeFeatureValues[CoreIdlePower] / numContainers)
		} else {
			containerPower = uint64(r.getPowerByRatio(containerIdx, int(CoreUsageMetric), int(CoreDynPower), numContainers))
		}
		containerNodeComponentsPower.Core = containerPower

		// DRAM power
		if isIdlePower {
			containerPower = uint64(r.nodeFeatureValues[DramIdlePower] / numContainers)
		} else {
			containerPower = uint64(r.getPowerByRatio(containerIdx, int(DramUsageMetric), int(DramDynPower), numContainers))
		}
		containerNodeComponentsPower.DRAM = containerPower

		// UNCORE power
		if isIdlePower {
			containerPower = uint64(r.nodeFeatureValues[UncoreIdlePower] / numContainers)
		} else {
			containerPower = uint64(r.getPowerByRatio(containerIdx, int(UncoreUsageMetric), int(UncoreDynPower), numContainers))
		}
		containerNodeComponentsPower.Uncore = containerPower

		nodeComponentsPowerOfAllContainers = append(nodeComponentsPowerOfAllContainers, containerNodeComponentsPower)
	}
	return nodeComponentsPowerOfAllContainers, nil
}

// GetComponentsPower returns GPU Power in Watts associated to each each process/container/pod
func (r *RatioPowerModel) GetGPUPower(isIdlePower bool) ([]float64, error) {
	nodeComponentsPowerOfAllContainers := []float64{}

	// the number of containers is used to evernly divide the power consumption for OTHER and UNCORE
	// we do not use CPU utilization for OTHER and UNCORE because they are not necessarily directly
	numContainers := float64(r.xidx)

	// estimate the power for each container
	for containerIdx := 0; containerIdx < r.xidx; containerIdx++ {
		var containerPower float64

		// TODO: idle power should be divided accordinly to the container requested resource
		if isIdlePower {
			containerPower = r.nodeFeatureValues[GpuIdlePower] / numContainers
		} else {
			containerPower = r.getPowerByRatio(containerIdx, int(GpuUsageMetric), int(GpuDynPower), numContainers)
		}
		nodeComponentsPowerOfAllContainers = append(nodeComponentsPowerOfAllContainers, containerPower)
	}
	return nodeComponentsPowerOfAllContainers, nil
}

// AddContainerFeatureValues adds the the x for prediction, which is the variable used to calculate the ratio.
// RatioPowerModel is not trained, then we cannot Add training samples, only samples for prediction.
func (r *RatioPowerModel) AddContainerFeatureValues(x []float64) {
	for i, feature := range x {
		// containerFeatureValues is a cyclic list, where we only append a new value if it is necessary.
		if r.xidx < len(r.containerFeatureValues) {
			if i < len(r.containerFeatureValues[r.xidx]) {
				r.containerFeatureValues[r.xidx][i] = feature
			} else {
				r.containerFeatureValues[r.xidx] = append(r.containerFeatureValues[r.xidx], feature)
			}
		} else {
			// add new container
			r.containerFeatureValues = append(r.containerFeatureValues, []float64{})
			// add feature of new container
			r.containerFeatureValues[r.xidx] = append(r.containerFeatureValues[r.xidx], feature)
		}
	}
	r.xidx += 1 // mode pointer to next container
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

// GetcontainerFeatureNamesList returns the list of container features that the model was configured to use
func (r *RatioPowerModel) GetContainerFeatureNamesList() []string {
	return r.ContainerFeatureNames
}

// GetNodeFeatureNamesList returns the list of container features that the model was configured to use
func (r *RatioPowerModel) GetNodeFeatureNamesList() []string {
	return r.NodeFeatureNames
}
