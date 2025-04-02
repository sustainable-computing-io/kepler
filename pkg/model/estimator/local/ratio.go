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
type (
	ComponentFeatures int
	PlaformFeatures   int
)

// The ratio power model can be used for both node Component and Platform power estimation.
// The models for Component and Platform power estimation have different number of features.
const (
	PkgUsageMetric ComponentFeatures = iota
	CoreUsageMetric
	DramUsageMetric
	UncoreUsageMetric
	OtherUsageMetric
	GPUUsageMetric
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

func (c ComponentFeatures) String() string {
	return []string{
		"PkgUsageMetric ",
		"CoreUsageMetric",
		"DramUsageMetric",
		"UncoreUsageMetric",
		"OtherUsageMetric",
		"GPUUsageMetric",
		"PkgDynPower",
		"CoreDynPower",
		"DramDynPower",
		"UncoreDynPower",
		"OtherDynPower",
		"GpuDynPower",
		"PkgIdlePower",
		"CoreIdlePower",
		"DramIdlePower",
		"UncoreIdlePower",
		"OtherIdlePower",
		"GpuIdlePower",
	}[c]
}

const (
	// Node features are at the end of the feature list
	PlatformUsageMetric PlaformFeatures = iota
	PlatformDynPower
	PlatformIdlePower
)

func (c PlaformFeatures) String() string {
	return []string{
		"PlatformUsageMetric",
		"PlatformDynPower",
		"PlatformIdlePower",
	}[c]
}

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

func (r *RatioPowerModel) getPowerByRatio(processIdx, resUsageFeature, nodePowerFeature int, numProcesses float64) uint64 {
	nodePower := math.Ceil(r.nodeFeatureValues[nodePowerFeature])
	if nodePower == 0 {
		return 0
	}

	var power float64
	nodeResUsage := r.nodeFeatureValues[resUsageFeature]
	if nodeResUsage == 0 || resUsageFeature == int(UncoreUsageMetric) {
		power = nodePower / numProcesses
	} else {
		processResUsage := r.processFeatureValues[processIdx][resUsageFeature]
		power = processResUsage / nodeResUsage * nodePower
	}

	return uint64(math.Ceil(power))
}

// GetPlatformPower applies ModelWeight prediction and return a list of total powers per process
func (r *RatioPowerModel) GetPlatformPower(isIdlePower bool) ([]uint64, error) {
	numProcesses := float64(r.xidx)

	var procPlatformPower []uint64
	var attributed uint64

	for i := 0; i < r.xidx; i++ {
		var procPower uint64
		if isIdlePower {
			// TODO: idle power should be divided accordingly to the process requested resource
			procPower = uint64Division(r.nodeFeatureValues[PlatformIdlePower], numProcesses)
		} else {
			procPower = r.getPowerByRatio(i, int(PlatformUsageMetric), int(PlatformDynPower), numProcesses)
		}
		procPlatformPower = append(procPlatformPower, procPower)
		attributed += procPower
	}

	// estimate the power for each process
	var nodeTotal uint64
	if isIdlePower {
		nodeTotal = uint64(math.Round(r.nodeFeatureValues[PlatformIdlePower]))
	} else {
		nodeTotal = uint64(math.Round(r.nodeFeatureValues[PlatformDynPower]))
	}

	if nodeTotal == attributed || r.xidx == 0 {
		return procPlatformPower, nil
	}

	// distribute the round off error to all process
	for i := 0; i < r.xidx && nodeTotal != attributed; i++ {
		// NOTE: with uinit64, you can't x += sign(diff) since -1 isn't an Uint

		if attributed < nodeTotal {
			procPlatformPower[i]++
			attributed++
		} else {
			procPlatformPower[i]--
			attributed--
		}
	}

	return procPlatformPower, nil
}

func uint64Division(x, y float64) uint64 {
	return uint64(math.Ceil(x / y))
}

func (r *RatioPowerModel) componentIdlePower() []source.NodeComponentsEnergy {
	// the number of processes is used to evenly divide the power consumption for OTHER and UNCORE
	// we do not use CPU utilization for OTHER and UNCORE because they are not necessarily directly
	//
	// estimate the power for each process
	procIdlePower := []source.NodeComponentsEnergy{}
	var pkgIdle, coreIdle, uncoreIdle, dramIdle uint64

	numProcs := float64(r.xidx)

	for i := 0; i < r.xidx; i++ {
		procPower := source.NodeComponentsEnergy{}
		procPower.Pkg = uint64Division(r.nodeFeatureValues[PkgIdlePower], numProcs)
		procPower.Core = uint64Division(r.nodeFeatureValues[CoreIdlePower], numProcs)
		procPower.DRAM = uint64Division(r.nodeFeatureValues[DramIdlePower], numProcs)
		procPower.Uncore = uint64Division(r.nodeFeatureValues[UncoreIdlePower], numProcs)

		pkgIdle += procPower.Pkg
		coreIdle += procPower.Core
		dramIdle += procPower.DRAM
		uncoreIdle += procPower.Uncore

		procIdlePower = append(procIdlePower, procPower)
	}

	r.distributeRoundOffError(procIdlePower, PkgIdlePower, pkgIdle)
	r.distributeRoundOffError(procIdlePower, CoreIdlePower, coreIdle)
	r.distributeRoundOffError(procIdlePower, DramIdlePower, dramIdle)
	r.distributeRoundOffError(procIdlePower, UncoreIdlePower, uncoreIdle)

	return procIdlePower
}

func (r *RatioPowerModel) componentDynamicPower() []source.NodeComponentsEnergy {
	procDynPower := []source.NodeComponentsEnergy{}
	var pkgDyn, coreDyn, uncoreDyn, dramDyn uint64

	totalProcs := float64(r.xidx)
	for idx := 0; idx < r.xidx; idx++ {
		procPower := source.NodeComponentsEnergy{}
		procPower.Pkg = r.getPowerByRatio(idx, int(PkgUsageMetric), int(PkgDynPower), totalProcs)
		procPower.Core = r.getPowerByRatio(idx, int(CoreUsageMetric), int(CoreDynPower), totalProcs)
		procPower.DRAM = r.getPowerByRatio(idx, int(DramUsageMetric), int(DramDynPower), totalProcs)
		procPower.Uncore = r.getPowerByRatio(idx, int(UncoreUsageMetric), int(UncoreDynPower), totalProcs)

		pkgDyn += procPower.Pkg
		coreDyn += procPower.Core
		dramDyn += procPower.DRAM
		uncoreDyn += procPower.Uncore

		procDynPower = append(procDynPower, procPower)
	}
	r.distributeRoundOffError(procDynPower, PkgDynPower, pkgDyn)
	r.distributeRoundOffError(procDynPower, CoreDynPower, coreDyn)
	r.distributeRoundOffError(procDynPower, DramDynPower, dramDyn)
	r.distributeRoundOffError(procDynPower, UncoreDynPower, uncoreDyn)

	return procDynPower
}

// GetComponentsPower applies each component's ModelWeight prediction and return a map of component powers
func (r *RatioPowerModel) GetComponentsPower(isIdlePower bool) ([]source.NodeComponentsEnergy, error) {
	if isIdlePower {
		return r.componentIdlePower(), nil
	}
	return r.componentDynamicPower(), nil
}

func (r *RatioPowerModel) distributeRoundOffError(processes []source.NodeComponentsEnergy, feature ComponentFeatures, procTotal uint64) {
	nodeTotal := uint64(math.Ceil(r.nodeFeatureValues[feature]))

	if nodeTotal > procTotal {
		diff := nodeTotal - procTotal
		for i := 0; i < r.xidx && diff > 0; i++ {
			switch feature {
			case PkgDynPower:
				processes[i].Pkg++
			case CoreDynPower:
				processes[i].Core++
			case DramDynPower:
				processes[i].DRAM++
			case UncoreDynPower:
				processes[i].Uncore++
			}
			diff--
		}
	} else if nodeTotal < procTotal {
		diff := procTotal - nodeTotal
		for i := 0; i < r.xidx && diff > 0; i++ {
			var x *uint64

			switch feature {
			case PkgDynPower, PkgIdlePower:
				x = &processes[i].Pkg
			case CoreDynPower, CoreIdlePower:
				x = &processes[i].Core
			case DramDynPower, DramIdlePower:
				x = &processes[i].DRAM
			case UncoreDynPower, UncoreIdlePower:
				x = &processes[i].Uncore
			}

			// avoid underflow
			if *x > 0 {
				*x--
				diff--
			}
		}
	}
}

// GetComponentsPower returns GPU Power in Watts associated to each each process/process/pod
func (r *RatioPowerModel) GetGPUPower(isIdlePower bool) ([]uint64, error) {
	nodeComponentsPowerOfAllProcesses := []uint64{}

	// the number of processes is used to evernly divide the power consumption for OTHER and UNCORE
	// we do not use CPU utilization for OTHER and UNCORE because they are not necessarily directly
	numProcesses := float64(r.xidx)

	// estimate the power for each process
	for processIdx := 0; processIdx < r.xidx; processIdx++ {
		var processPower uint64

		// TODO: idle power should be divided accordingly to the process requested resource
		if isIdlePower {
			processPower = uint64Division(r.nodeFeatureValues[GpuIdlePower], numProcesses)
		} else {
			processPower = r.getPowerByRatio(processIdx, int(GPUUsageMetric), int(GpuDynPower), numProcesses)
		}
		nodeComponentsPowerOfAllProcesses = append(nodeComponentsPowerOfAllProcesses, processPower)
	}
	return nodeComponentsPowerOfAllProcesses, nil
}

// AddProcessFeatureValues adds the x for prediction, which is the variable used to calculate the ratio.
// RatioPowerModel is not trained, then we cannot Add training samples, only samples for prediction.
func (r *RatioPowerModel) AddProcessFeatureValues(x []float64) {
	if r.xidx >= len(r.processFeatureValues) {
		r.processFeatureValues = append(r.processFeatureValues, []float64{})
	}

	r.processFeatureValues[r.xidx] = make([]float64, len(x))
	copy(r.processFeatureValues[r.xidx], x)
	r.xidx++
}

// AddNodeFeatureValues adds the the x for prediction, which is the variable used to calculate the ratio.
// RatioPowerModel is not trained, then we cannot Add training samples, only samples for prediction.
func (r *RatioPowerModel) AddNodeFeatureValues(x []float64) {
	r.nodeFeatureValues = make([]float64, len(x))
	copy(r.nodeFeatureValues, x)
}

// AddDesiredOutValue adds the the y, which is the response variable (or the dependent variable) of regression.
// RatioPowerModel is not trained, then we cannot Add training samples.
func (r *RatioPowerModel) AddDesiredOutValue(y float64) {
}

// ResetSampleIdx set the sample vector index to 0 to overwrite the old samples with new ones for training or prediction.
func (r *RatioPowerModel) ResetSampleIdx() {
	r.xidx = 0
	r.processFeatureValues = r.processFeatureValues[:0]
	r.nodeFeatureValues = r.nodeFeatureValues[:0]
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
