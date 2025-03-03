/*
Copyright 2021-2024.

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

package stats

import (
	"fmt"

	"math"

	"github.com/sustainable-computing-io/kepler/pkg/config"
	"github.com/sustainable-computing-io/kepler/pkg/node"
	acc "github.com/sustainable-computing-io/kepler/pkg/sensors/accelerator"
	"github.com/sustainable-computing-io/kepler/pkg/utils"
	"k8s.io/klog/v2"
)

var (
	// Modify Manually
	spreadDiff           = 0.3
	historyLength        = 10
	energyTypeToMinSlope = map[string]float64{
		config.IdleEnergyInCore:   0.01,
		config.IdleEnergyInPkg:    0.3,
		config.IdleEnergyInUnCore: 0.3,
		config.IdleEnergyInDRAM:   0.3,
	}
	energyTypeToMinIntercept = map[string]float64{
		config.IdleEnergyInCore:   0,
		config.IdleEnergyInPkg:    0,
		config.IdleEnergyInUnCore: 0,
		config.IdleEnergyInDRAM:   0,
	}
)

type IdleEnergyResult struct {
	calculatedIdleEnergy float64
	diff                 float64
	history              []float64
}

type IdleEnergyCalculator struct {
	// Min CPU TIME
	minUtilization *Coordinate
	// Max CPU Time
	maxUtilization *Coordinate
	// Min y-intercept
	minIntercept float64
	// Min slope
	minSlope float64

	result *IdleEnergyResult
}

func (ic *IdleEnergyCalculator) UpdateIdleEnergy(newResutilization float64, newEnergyDelta float64, maxTheoreticalCPUTime float64) {
	klog.V(5).Infof("New Datapoint Candidate: (%f, %f)", newResutilization, newEnergyDelta)
	klog.V(5).Infof("Current Min Utilization Datapoint: (%f, %f)", ic.minUtilization.X, ic.minUtilization.Y)
	klog.V(5).Infof("Current Max Utilization Datapoint: (%f, %f)", ic.maxUtilization.X, ic.maxUtilization.Y)
	klog.V(5).Infof("Current History: (%v)", ic.result.history)
	klog.V(5).Infof("Current Spread: (%v)", ic.result.diff)
	if newResutilization > ic.minUtilization.X && newResutilization < ic.maxUtilization.X {
		klog.V(5).Infof("Excess Datapoint: (%f, %f)", newResutilization, newEnergyDelta)
		// Record History
		klog.V(5).Infof("Push Idle Energy to history")
		appendToSliceWithSizeRestriction(&ic.result.history, historyLength, ic.result.calculatedIdleEnergy)
		return
	}
	var newMinUtilizationX float64 = ic.minUtilization.X
	var newMinUtilizationY float64 = ic.minUtilization.Y
	var newMaxUtilizationX float64 = ic.maxUtilization.X
	var newMaxUtilizationY float64 = ic.maxUtilization.Y
	if newResutilization <= ic.minUtilization.X {
		if newResutilization == ic.minUtilization.X {
			klog.V(5).Infof("Modified Min Utilization Y Value")
			//ic.minUtilization.Y = math.Min(newEnergyDelta, ic.minUtilization.Y)
			newMinUtilizationY = math.Min(newEnergyDelta, newMinUtilizationY)
		} else {
			klog.V(5).Infof("Modified Min Utilization X, Y Value")
			// ic.minUtilization.X = newResutilization
			// ic.minUtilization.Y = newEnergyDelta
			newMinUtilizationX = newResutilization
			newMinUtilizationY = newEnergyDelta
		}
	}

	if ic.maxUtilization.X <= newResutilization {
		if newResutilization == ic.maxUtilization.X {
			klog.V(5).Infof("Modified Max Utilization Y Value")
			//ic.maxUtilization.Y = math.Min(newEnergyDelta, ic.maxUtilization.Y)
			newMaxUtilizationY = math.Min(newEnergyDelta, newMaxUtilizationY)
		} else {
			klog.V(5).Infof("Modified Max Utilization X, Y Value")
			// replace maxUtilization X and Y
			//ic.maxUtilization.X = newResutilization
			//ic.maxUtilization.Y = newEnergyDelta
			newMaxUtilizationX = newResutilization
			newMaxUtilizationY = newEnergyDelta
		}
	}

	// log candidates
	klog.V(5).Infof("Candidate Min Utilization Datapoint: (%f, %f)", newMinUtilizationX, newMinUtilizationY)
	klog.V(5).Infof("Candidate Max Utilization Datapoint: (%f, %f)", newMaxUtilizationX, newMaxUtilizationY)

	// note minutilization == maxutilization only occurs when we only have one value at the very beginning
	// in that case, we can rely on the default values provided by NewIdleEnergyCalculator
	//if ic.minUtilization.X < ic.maxUtilization.X {
	if newMinUtilizationX < newMaxUtilizationX {
		linearModel := CalculateLR(newMinUtilizationX, newMinUtilizationY, newMaxUtilizationX, newMaxUtilizationY)
		klog.V(5).Infof("Calculated Intercept: %f, Calculated Slope: %f", linearModel.intercept, linearModel.slope)
		if linearModel.intercept >= ic.minIntercept && linearModel.slope >= ic.minSlope {
			klog.V(5).Infof("Successfully passed Min Intercept (%f) and Min Slope (%f) Requirements", ic.minIntercept, ic.minSlope)
			// update min,max utilization, result idle energy
			ic.minUtilization.X = newMinUtilizationX
			ic.minUtilization.Y = newMinUtilizationY
			ic.maxUtilization.X = newMaxUtilizationX
			ic.maxUtilization.Y = newMaxUtilizationY
			ic.result.calculatedIdleEnergy = linearModel.intercept
			// record history
			appendToSliceWithSizeRestriction(&ic.result.history, historyLength, ic.result.calculatedIdleEnergy)
			// calculate diff
			ic.result.diff = math.Abs(ic.minUtilization.X/maxTheoreticalCPUTime - ic.maxUtilization.X/maxTheoreticalCPUTime)
		}
	}
	// log new minUtilization and maxUtilization
	klog.V(5).Infof("New Min Utilization Datapoint: (%f, %f)", ic.minUtilization.X, ic.minUtilization.Y)
	klog.V(5).Infof("New Max Utilization Datapoint: (%f, %f)", ic.maxUtilization.X, ic.maxUtilization.Y)
	klog.V(5).Infof("New Calculated Idle Energy: (%f)", ic.result.calculatedIdleEnergy)
	klog.V(5).Infof("New History: (%v)", ic.result.history)
	klog.V(5).Infof("New Spread/Diff: (%f)", ic.result.diff)

}

func CalculateLR(x_one, y_one, x_two, y_two float64) *LinearModel {
	slope := (y_two - y_one) / (x_two - x_one)
	klog.V(5).Infof("Slope Calculation: %f", slope)
	intercept := y_one - slope*x_one
	klog.V(5).Infof("Calculated Idle Energy: %f", intercept)
	return &LinearModel{
		intercept: intercept,
		slope:     slope,
	}
}

func NewEnergyCoord(resourceUtilization, energyUsage float64) *Coordinate {
	return &Coordinate{
		X: resourceUtilization,
		Y: energyUsage,
	}
}

func NewIdleEnergyCalculator(minUtilization, maxUtilization *Coordinate, minIntercept, minSlope float64) *IdleEnergyCalculator {
	return &IdleEnergyCalculator{
		minUtilization: minUtilization,
		maxUtilization: maxUtilization,
		minIntercept:   minIntercept,
		minSlope:       minSlope,
		result: &IdleEnergyResult{
			calculatedIdleEnergy: 0.0,
			diff:                 0.0,
			history:              make([]float64, 0, historyLength),
		},
	}
}

type NodeStats struct {
	Stats

	// IdleResUtilization is used to determine idle pmap[string]eriods
	IdleResUtilization map[string]uint64

	// idle energy
	IdleEnergy map[string]*IdleEnergyCalculator

	// flag to check if idle calculation is reliable
	isIdlePowerReliable bool

	// nodeInfo allows access to node information
	nodeInfo node.Node
}

func NewNodeStats() *NodeStats {
	return &NodeStats{
		Stats:              *NewStats(),
		IdleResUtilization: map[string]uint64{},
		IdleEnergy:         map[string]*IdleEnergyCalculator{},
		nodeInfo:           node.NewNodeInfo(),
	}
}

// ResetDeltaValues reset all delta values to 0
func (ne *NodeStats) ResetDeltaValues() {
	ne.Stats.ResetDeltaValues()
}

func (ne *NodeStats) UpdateIdleEnergyWithLinearRegresion(isComponentsSystemCollectionSupported bool) {
	if config.IsGPUEnabled() {
		if acc.GetActiveAcceleratorByType(config.GPU) != nil {
			ne.CalcIdleEnergyLR(config.AbsEnergyInGPU, config.IdleEnergyInGPU, config.GPUComputeUtilization)
		}
	}

	if isComponentsSystemCollectionSupported {
		ne.CalcIdleEnergyLR(config.AbsEnergyInCore, config.IdleEnergyInCore, config.CPUTime)
		ne.CalcIdleEnergyLR(config.AbsEnergyInDRAM, config.IdleEnergyInDRAM, config.CPUTime) // TODO: we should use another resource for DRAM
		ne.CalcIdleEnergyLR(config.AbsEnergyInUnCore, config.IdleEnergyInUnCore, config.CPUTime)
		ne.CalcIdleEnergyLR(config.AbsEnergyInPkg, config.IdleEnergyInPkg, config.CPUTime)
		//ne.CalcIdleEnergyLR(config.AbsEnergyInPlatform, config.IdleEnergyInPlatform, config.CPUTime) // Platform Idle Power should never be included
	}
}

func (ne *NodeStats) CalcIdleEnergyLR(absM, idleM, resouceUtil string) {
	totalResUtilization := ne.ResourceUsage[resouceUtil].SumAllDeltaValues()
	totalEnergy := ne.EnergyUsage[absM].SumAllDeltaValues()
	klog.V(5).Infof("Energy Type: %s", absM)
	klog.V(5).Infof("Total Energy: %d", totalEnergy)
	if totalEnergy == 0 {
		// Insufficient Sample Size by default
		klog.V(5).Infof("Skipping Idle Energy")
		return
	}

	if _, exists := ne.IdleEnergy[idleM]; !exists {
		// Insufficient Sample Size by default
		initialMinUtilization := NewEnergyCoord(
			float64(totalResUtilization),
			float64(totalEnergy),
		)
		initialMaxUtilization := NewEnergyCoord(
			float64(totalResUtilization),
			float64(totalEnergy),
		)
		ne.IdleEnergy[idleM] = NewIdleEnergyCalculator(
			initialMinUtilization,
			initialMaxUtilization,
			energyTypeToMinIntercept[idleM],
			energyTypeToMinSlope[idleM],
		)
		klog.V(5).Infof("Initialize Idle Energy (%s): %f", idleM, ne.IdleEnergy[idleM].result.calculatedIdleEnergy)
	} else {
		cpuCount := ne.nodeInfo.CPUCount()
		klog.V(5).Infof("Sample Period: %d", config.SamplePeriodSec())
		maxTheoreticalCPUTime := cpuCount * config.SamplePeriodSec() * 1000 // CPUTime is in milliseconds
		klog.V(5).Infof("Maximum CPU Time per Sample Period: %d", maxTheoreticalCPUTime)
		ne.IdleEnergy[idleM].UpdateIdleEnergy(float64(totalResUtilization), float64(totalEnergy), float64(maxTheoreticalCPUTime))
		klog.V(5).Infof("Stored Idle Energy (%s): %f", idleM, ne.IdleEnergy[idleM].result.calculatedIdleEnergy)
		klog.V(5).Infof("Checking if Stored Idle Energy is reliable")
		result := ne.IdleEnergy[idleM].result
		if !ne.isIdlePowerReliable {
			klog.V(5).Infof("Checking if Sample Size is large enough")
			if result.diff >= spreadDiff {
				klog.V(5).Infof("Sample Size is large enough!")
				klog.V(5).Infof("Checking if Idle Energy is consistent")
				if percentDiff(getAverage(result.history), result.calculatedIdleEnergy) <= 0.1 {
					klog.V(5).Infof("Idle Energy is Consistent! Idle Energy is ready to be passed.")
					ne.isIdlePowerReliable = true
					ne.EnergyUsage[idleM].SetDeltaStat("0", uint64(result.calculatedIdleEnergy))
				} else {
					klog.V(5).Infof("Idle Energy is not Consistent yet. Continue Checks")
				}
			} else {
				klog.V(5).Infof("Sample Size is not large enough. Continue Checks")
			}
		} else {
			klog.V(5).Infof("Idle Energy has already been approved. Idle Energy is ready to be passed (Regardless).")
			ne.EnergyUsage[idleM].SetDeltaStat("0", uint64(result.calculatedIdleEnergy))
		}
	}
}

func (ne *NodeStats) UpdateIdleEnergyWithMinValue(isComponentsSystemCollectionSupported bool) {
	// gpu metric
	if config.IsGPUEnabled() {
		if acc.GetActiveAcceleratorByType(config.GPU) != nil {
			ne.CalcIdleEnergy(config.AbsEnergyInGPU, config.IdleEnergyInGPU, config.GPUComputeUtilization)
		}
	}

	if isComponentsSystemCollectionSupported {
		ne.CalcIdleEnergy(config.AbsEnergyInCore, config.IdleEnergyInCore, config.CPUTime)
		ne.CalcIdleEnergy(config.AbsEnergyInDRAM, config.IdleEnergyInDRAM, config.CPUTime) // TODO: we should use another resource for DRAM
		ne.CalcIdleEnergy(config.AbsEnergyInUnCore, config.IdleEnergyInUnCore, config.CPUTime)
		ne.CalcIdleEnergy(config.AbsEnergyInPkg, config.IdleEnergyInPkg, config.CPUTime)
		ne.CalcIdleEnergy(config.AbsEnergyInPlatform, config.IdleEnergyInPlatform, config.CPUTime)
	}
}

func (ne *NodeStats) CalcIdleEnergy(absM, idleM, resouceUtil string) {
	newTotalResUtilization := ne.ResourceUsage[resouceUtil].SumAllDeltaValues()
	currIdleTotalResUtilization := ne.IdleResUtilization[resouceUtil]
	klog.V(5).Infof("Old Idle Energy: %s", absM)
	klog.V(5).Infof("Old Idle Calculation Res util: %d", newTotalResUtilization)

	for socketID, value := range ne.EnergyUsage[absM] {
		newIdleDelta := value.GetDelta()
		klog.V(5).Infof("Socket ID: %s", socketID)
		klog.V(5).Infof("Old Idle calculation: %d", newIdleDelta)
		if newIdleDelta == 0 {
			// during the first power collection iterations, the delta values could be 0, so we skip until there are delta values
			continue
		}

		// add any value if there is no idle power yet
		if _, exist := ne.EnergyUsage[idleM][socketID]; !exist {
			ne.EnergyUsage[idleM].SetDeltaStat(socketID, newIdleDelta)
			// store the current CPU utilization to find a new idle power later
			ne.IdleResUtilization[resouceUtil] = newTotalResUtilization
		} else {
			currIdleDelta := ne.EnergyUsage[idleM][socketID].GetDelta()
			// verify if there is a new minimal energy consumption for the given resource
			// TODO: fix verifying the aggregated resource utilization from all sockets, the update the energy per socket can lead to inconsistency
			if (newTotalResUtilization <= currIdleTotalResUtilization) || (currIdleDelta == 0) {
				if (currIdleDelta == 0) || (currIdleDelta >= newIdleDelta) {
					ne.EnergyUsage[idleM].SetDeltaStat(socketID, newIdleDelta)
					ne.IdleResUtilization[resouceUtil] = newTotalResUtilization
					continue
				}
			}
			if currIdleDelta == 0 {
				continue
			}
			// as the dynamic and absolute power, the idle power is also a counter to be exported to prometheus
			// therefore, we accumulate the minimal found idle if no new one was found
			ne.EnergyUsage[idleM].SetDeltaStat(socketID, currIdleDelta)
		}
	}
}

// SetNodeOtherComponentsEnergy adds the latest energy consumption collected from the other node's components than CPU and DRAM
// Other components energy is a special case where the energy is calculated and not measured
func (ne *NodeStats) SetNodeOtherComponentsEnergy() {
	// calculate dynamic energy in other components
	dynCPUComponentsEnergy := ne.EnergyUsage[config.DynEnergyInPkg].SumAllDeltaValues() +
		ne.EnergyUsage[config.DynEnergyInDRAM].SumAllDeltaValues() +
		ne.EnergyUsage[config.DynEnergyInGPU].SumAllDeltaValues()

	dynPlatformEnergy := ne.EnergyUsage[config.DynEnergyInPlatform].SumAllDeltaValues()

	if dynPlatformEnergy >= dynCPUComponentsEnergy {
		otherComponentEnergy := dynPlatformEnergy - dynCPUComponentsEnergy
		ne.EnergyUsage[config.DynEnergyInOther].SetDeltaStat(utils.GenericSocketID, otherComponentEnergy)
	}

	// calculate idle energy in other components
	idleCPUComponentsEnergy := ne.EnergyUsage[config.IdleEnergyInPkg].SumAllDeltaValues() +
		ne.EnergyUsage[config.IdleEnergyInDRAM].SumAllDeltaValues() +
		ne.EnergyUsage[config.IdleEnergyInGPU].SumAllDeltaValues()

	idlePlatformEnergy := ne.EnergyUsage[config.IdleEnergyInPlatform].SumAllDeltaValues()

	if idlePlatformEnergy >= idleCPUComponentsEnergy {
		otherComponentEnergy := idlePlatformEnergy - idleCPUComponentsEnergy
		ne.EnergyUsage[config.IdleEnergyInOther].SetDeltaStat(utils.GenericSocketID, otherComponentEnergy)
	}
}

func (ne *NodeStats) String() string {
	return fmt.Sprintf("node energy (mJ): \n"+
		"%v\n", ne.Stats.String(),
	)
}

func (ne *NodeStats) MetadataFeatureNames() []string {
	return ne.nodeInfo.MetadataFeatureNames()
}

func (ne *NodeStats) MetadataFeatureValues() []string {
	return ne.nodeInfo.MetadataFeatureValues()
}

func (ne *NodeStats) CPUArchitecture() string {
	return ne.nodeInfo.CPUArchitecture()
}

func (ne *NodeStats) NodeName() string {
	return ne.nodeInfo.Name()
}
