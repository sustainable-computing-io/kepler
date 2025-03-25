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

// =============================================================================
// Idle Energy Calculator
// =============================================================================

// IdleEnergyResult contains the results of an idle energy calculation using regression.
type IdleEnergyResult struct {
	CalculatedIdleEnergy float64   // Estimated baseline idle energy in milijoules
	Slope                float64   // Slope of the linear regression model (milijoules/milliseconds)
	Spread               float64   // Percent Spread between min/max utilization datapoints
	History              []float64 // Historical idle energy values for consistency checking
}

// IdleEnergyConfig defines thresholds and parameters for valid idle energy.
// These values ensure calculated idle energy is accurate.
type IdleEnergyConfig struct {
	MinSpread    float64 // Minimum required utilization spread (0.0-1.0) between min/max utilization datapoints
	MinIntercept float64 // Minimum acceptable intercept value in milijoules
	MinSlope     float64 // Minimum acceptable slope value (milijoules/milliseconds)
	HistorySize  int     // Number of historical values to maintain for consistency checks
}

// IdleEnergyCalculatorInitialization contains initialization parameters for a new IdleEnergyCalculator.
// This struct ensures proper setup of the initial state for energy calculations.
type IdleEnergyCalculatorInitialization struct {
	InitialUtilizationX float64          // Initial resource utilization measurement
	InitialUtilizationY float64          // Initial energy measurement
	Config              IdleEnergyConfig // Configuration for IdleEnergyCalculator
}

// IdleEnergyModelInputs contains parameters for calculating idle energy.
// This struct bundles all required inputs for CalcIdleEnergyLR method.
type IdleEnergyModelInputs struct {
	AbsoluteEnergyType      string           // Absolute energy metric name
	IdleEnergyType          string           // Idle energy metric name
	ResourceUtilizationType string           // Resource utilization metric name
	ScrapeInterval          uint64           // Duration between power sensor scrapes in seconds
	CPUCount                uint64           // Number of available CPUs
	Config                  IdleEnergyConfig // Configuration for IdleEnergyCalculator
}

// NewEnergyPoint represents a single observation of resource utilization and energy consumption.
type NewEnergyPoint struct {
	NewResUtil            float64 // Current resource utilization measurement
	NewEnergyDelta        float64 // Delta energy consumption since last measurement
	maxTheoreticalCPUTime float64 // Theoretical maximum CPU time for spread normalization
}

// IdleEnergyCalculator computes and maintains idle energy estimates using regression.
// It tracks utilization boundaries, validates models against thresholds, and maintains historical data
// to ensure consistent idle energy estimates.
type IdleEnergyCalculator struct {
	MinUtilization      *Coordinate       // Minimum observed utilization (utilization, energy) point
	MaxUtilization      *Coordinate       // Maximum observed utilization (utilization, energy) point
	Config              IdleEnergyConfig  // Configuration for IdleEnergyCalculator
	Result              *IdleEnergyResult // Results of an idle energy calculation using regression.
	IsIdlePowerReliable bool              // Flag indicating if current estimate is ready to be exported
}

// NewIdleEnergyCalculator initializes an idle energy calculator
//
// Parameters:
//   - input: An IdleEnergyCalculatorInitialization
//
// Returns:
//   - A pointer to a new IdleEnergyCalculator instance.
//
// Initialization Logic:
//   - Sets both min/max utilization to the same initial coordinates
//   - Sets Config for IdleEnergyCalculator
//   - Pre-allocates history storage based on config.HistorySize
//   - Initializes all result fields to zero values
func NewIdleEnergyCalculator(input IdleEnergyCalculatorInitialization) *IdleEnergyCalculator {
	return &IdleEnergyCalculator{
		MinUtilization: &Coordinate{input.InitialUtilizationX, input.InitialUtilizationY},
		MaxUtilization: &Coordinate{input.InitialUtilizationX, input.InitialUtilizationY},
		Config:         input.Config,
		Result: &IdleEnergyResult{
			CalculatedIdleEnergy: 0.0,
			Spread:               0.0,
			History:              make([]float64, 0, input.Config.HistorySize),
		},
	}
}

// UpdateIdleEnergy updates the idle energy calculation based on a new data point.
// It evaluates the new data point, updates the minimum and maximum utilization values based on new datapoint,
// and recalculates the idle energy if necessary.
//
// Invariant Preconditions:
//   - ic.MinUtilization.X <= ic.MaxUtilization.X
//
// Parameters:
//   - datapoint: A NewEnergyPoint
//
// Behavior:
//   - If the new data point falls between the current minimum and maximum utilization points,
//     it is considered an excess data point and is not used to update the idle energy calculation.
//   - If the new data point updates the minimum or maximum utilization values, a new model
//     is calculated to determine the idle energy.
//   - The function ensures that the calculated idle energy meets the minimum intercept and slope
//     requirements specified in the configuration. Failing to meet these requirements will cause the
//     the datapoint to be discarded.
//   - The history of calculated idle energy values is maintained, with the size restricted by the
//     configured history size to ensure only new calculated idle energy values are kept for
//     consistency.
func (ic *IdleEnergyCalculator) UpdateIdleEnergy(datapoint NewEnergyPoint) {
	newResUtil := datapoint.NewResUtil
	newEnergyDelta := datapoint.NewEnergyDelta
	maxTheoreticalCPUTime := datapoint.maxTheoreticalCPUTime
	klog.V(5).Infof("New Datapoint Candidate: (%f, %f)", newResUtil, newEnergyDelta)
	klog.V(5).Infof("Current Min Utilization Datapoint: (%f, %f)", ic.MinUtilization.X, ic.MinUtilization.Y)
	klog.V(5).Infof("Current Max Utilization Datapoint: (%f, %f)", ic.MaxUtilization.X, ic.MaxUtilization.Y)
	klog.V(5).Infof("Current History: (%v)", ic.Result.History)
	klog.V(5).Infof("Current Spread: (%v)", ic.Result.Spread)

	// Check if datapoint falls between Min and Max Utilization points
	if ic.isExcessDatapoint(datapoint) {
		klog.V(5).Infof("Excess Datapoint: (%f, %f)", newResUtil, newEnergyDelta)
		klog.V(5).Infof("Push Idle Energy to history")
		appendToSliceWithSizeRestriction(&ic.Result.History, ic.Config.HistorySize, ic.Result.CalculatedIdleEnergy)
		return
	}

	newMinUtilizationX, newMinUtilizationY, newMaxUtilizationX, newMaxUtilizationY := ic.updateUtilizationValues(
		datapoint,
	)

	klog.V(5).Infof("Candidate Min Utilization Datapoint: (%f, %f)", newMinUtilizationX, newMinUtilizationY)
	klog.V(5).Infof("Candidate Max Utilization Datapoint: (%f, %f)", newMaxUtilizationX, newMaxUtilizationY)

	// note minutilization == maxutilization only occurs when we only have one value at the very beginning
	// in that case, we can rely on the default values provided by NewIdleEnergyCalculator
	if newMinUtilizationX == newMaxUtilizationX {
		// update min,max utilization with potentially new values
		ic.MinUtilization.X = newMinUtilizationX
		ic.MinUtilization.Y = newMinUtilizationY
		ic.MaxUtilization.X = newMaxUtilizationX
		ic.MaxUtilization.Y = newMaxUtilizationY
		appendToSliceWithSizeRestriction(&ic.Result.History, ic.Config.HistorySize, ic.Result.CalculatedIdleEnergy)
		return
	}

	if newMinUtilizationX < newMaxUtilizationX {
		linearModel := CalculateLR(newMinUtilizationX, newMinUtilizationY, newMaxUtilizationX, newMaxUtilizationY)
		klog.V(5).Infof("Calculated Intercept: %f, Calculated Slope: %f", linearModel.intercept, linearModel.slope)
		if linearModel.intercept >= ic.Config.MinIntercept && linearModel.slope >= ic.Config.MinSlope {
			klog.V(5).Infof("Successfully passed Min Intercept (%f) and Min Slope (%f) Requirements", ic.Config.MinIntercept, ic.Config.MinSlope)
			// update min,max utilization, result idle energy
			ic.MinUtilization.X = newMinUtilizationX
			ic.MinUtilization.Y = newMinUtilizationY
			ic.MaxUtilization.X = newMaxUtilizationX
			ic.MaxUtilization.Y = newMaxUtilizationY
			ic.Result.CalculatedIdleEnergy = linearModel.intercept
			ic.Result.Slope = linearModel.slope
			ic.Result.Spread = math.Abs(ic.MinUtilization.X/maxTheoreticalCPUTime - ic.MaxUtilization.X/maxTheoreticalCPUTime)
		}
		appendToSliceWithSizeRestriction(&ic.Result.History, ic.Config.HistorySize, ic.Result.CalculatedIdleEnergy)
	}
	klog.V(5).Infof("New Min Utilization Datapoint: (%f, %f)", ic.MinUtilization.X, ic.MinUtilization.Y)
	klog.V(5).Infof("New Max Utilization Datapoint: (%f, %f)", ic.MaxUtilization.X, ic.MaxUtilization.Y)
	klog.V(5).Infof("New Calculated Idle Energy: (%f)", ic.Result.CalculatedIdleEnergy)
	klog.V(5).Infof("New History: (%v)", ic.Result.History)
	klog.V(5).Infof("New Spread: (%f)", ic.Result.Spread)
}

// isExcessDatapoint checks if a given data point falls between the current minimum
// and maximum utilization points. Such data points are considered excess and are not
// used to update the idle energy calculation.
//
// Parameters:
//   - datapoint: A NewEnergyPoint
//
// Returns:
//   - A boolean indicating whether the data point is excess (true) or not (false).
func (ic *IdleEnergyCalculator) isExcessDatapoint(datapoint NewEnergyPoint) bool {
	return datapoint.NewResUtil > ic.MinUtilization.X && datapoint.NewResUtil < ic.MaxUtilization.X
}

// updateUtilizationValues updates the minimum and maximum utilization values based on a new data point.
//
// Parameters:
//   - datapoint: A NewEnergyPoint
//
// Returns:
//   - Four values representing the updated minimum and maximum utilization points
//     (Resource Utilization and Energy Delta coordinates).
func (ic *IdleEnergyCalculator) updateUtilizationValues(datapoint NewEnergyPoint) (
	newMinUtilizationX, newMinUtilizationY,
	newMaxUtilizationX, newMaxUtilizationY float64,
) {
	newResUtil := datapoint.NewResUtil
	newEnergyDelta := datapoint.NewEnergyDelta
	newMinUtilizationY = ic.MinUtilization.Y
	newMaxUtilizationY = ic.MaxUtilization.Y
	newMinUtilizationX = math.Min(ic.MinUtilization.X, newResUtil)
	newMaxUtilizationX = math.Max(ic.MaxUtilization.X, newResUtil)
	if newResUtil == ic.MinUtilization.X {
		newMinUtilizationY = math.Min(ic.MinUtilization.Y, newEnergyDelta)
		klog.V(5).Infof("Modified Min Utilization Y Value: (%f, %f)", newMinUtilizationX, newMinUtilizationY)
	} else if newResUtil < ic.MinUtilization.X {
		newMinUtilizationY = newEnergyDelta
		klog.V(5).Infof("Modified Min Utilization X Y Value: (%f, %f)", newMinUtilizationX, newMinUtilizationY)
	}

	if newResUtil == ic.MaxUtilization.X {
		newMaxUtilizationY = math.Min(ic.MaxUtilization.Y, newEnergyDelta)
		klog.V(5).Infof("Modified Max Utilization Y Value: (%f, %f)", newMaxUtilizationX, newMaxUtilizationY)
	} else if newResUtil > ic.MaxUtilization.X {
		newMaxUtilizationY = newEnergyDelta
		klog.V(5).Infof("Modified Max Utilization X Y Value: (%f, %f)", newMaxUtilizationX, newMaxUtilizationY)
	}
	return newMinUtilizationX, newMinUtilizationY, newMaxUtilizationX, newMaxUtilizationY
}

// =============================================================================
// Linear Regression Calculations
// =============================================================================

// LinearModel represents a linear regression model with an intercept and slope.
// It is used to model the relationship between resource utilization (X) and energy delta (Y).
type LinearModel struct {
	intercept float64 // The intercept of the linear model.
	slope     float64 // The slope of the linear model.
}

// CalculateLR computes a linear regression model based on two data points.
// It calculates the slope and intercept of the line that passes through the points
// (xOne, yOne) and (xTwo, yTwo).
//
// Parameters:
//   - xOne: The X-coordinate of the first data point (resource utilization).
//   - yOne: The Y-coordinate of the first data point (energy delta).
//   - xTwo: The X-coordinate of the second data point (resource utilization).
//   - yTwo: The Y-coordinate of the second data point (energy delta).
//
// Returns:
//   - A LinearModel
//
// Behavior:
//   - The slope is calculated as (yTwo - yOne) / (xTwo - xOne).
//   - The intercept is calculated as yOne - slope * xOne.
func CalculateLR(xOne, yOne, xTwo, yTwo float64) LinearModel {
	slope := (yTwo - yOne) / (xTwo - xOne)
	klog.V(5).Infof("Slope Calculation: %f", slope)
	intercept := yOne - slope*xOne
	klog.V(5).Infof("Calculated Idle Energy: %f", intercept)
	return LinearModel{
		intercept: intercept,
		slope:     slope,
	}
}

// =============================================================================
// NodeStats Implementation
// =============================================================================

type NodeStats struct {
	Stats

	// IdleResUtilization is used to determine idle pmap[string]eriods
	IdleResUtilization map[string]uint64

	// Idle Energy stores calculated idle energy for some defined power
	IdleEnergy map[string]*IdleEnergyCalculator

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

// UpdateIdleEnergyWithLinearRegression updates the idle energy calculations for a node
// using linear regression if the system supports component-level energy collection.
// It calculates idle energy for different energy types (e.g., package, DRAM, core, uncore)
// based on resource utilization and energy consumption data.
//
// Parameters:
//   - isComponentsSystemCollectionSupported: A boolean indicating whether the system supports
//     component-level energy collection.
//   - duration: The scrape interval duration in nanoseconds.
//   - cpuCount: The number of CPUs on the node.
//
// Behavior:
//   - If component-level energy collection is supported, the function calculates idle energy
//     for package, DRAM, core, and uncore energy types using the CalcIdleEnergyLR function.
//   - It uses a fixed minimum spread (0.5) and history size (10) for the idle energy calculations.
//   - The function assumes CPU time as the resource utilization type for all energy types.
func (ne *NodeStats) UpdateIdleEnergyWithLinearRegression(isComponentsSystemCollectionSupported bool, duration, cpuCount uint64) {
	if isComponentsSystemCollectionSupported {
		minSpread := 0.5
		historySize := 10
		ne.CalcIdleEnergyLR(&IdleEnergyModelInputs{
			AbsoluteEnergyType:      config.AbsEnergyInPkg,
			IdleEnergyType:          config.IdleEnergyInPkg,
			ResourceUtilizationType: config.CPUTime,
			ScrapeInterval:          duration,
			CPUCount:                cpuCount,
			Config: IdleEnergyConfig{
				MinSpread:    minSpread,
				MinIntercept: 0.0,
				MinSlope:     0.01,
				HistorySize:  historySize,
			},
		})
		ne.CalcIdleEnergyLR(&IdleEnergyModelInputs{
			AbsoluteEnergyType:      config.AbsEnergyInDRAM,
			IdleEnergyType:          config.IdleEnergyInDRAM,
			ResourceUtilizationType: config.CPUTime,
			ScrapeInterval:          duration,
			CPUCount:                cpuCount,
			Config: IdleEnergyConfig{
				MinSpread:    minSpread,
				MinIntercept: 0.0,
				MinSlope:     0.01,
				HistorySize:  historySize,
			},
		}) // TODO: we should use another resource for DRAM
		ne.CalcIdleEnergyLR(&IdleEnergyModelInputs{
			AbsoluteEnergyType:      config.AbsEnergyInCore,
			IdleEnergyType:          config.IdleEnergyInCore,
			ResourceUtilizationType: config.CPUTime,
			ScrapeInterval:          duration,
			CPUCount:                cpuCount,
			Config: IdleEnergyConfig{
				MinSpread:    minSpread,
				MinIntercept: 0.0,
				MinSlope:     0.01,
				HistorySize:  historySize,
			},
		})
		ne.CalcIdleEnergyLR(&IdleEnergyModelInputs{
			AbsoluteEnergyType:      config.AbsEnergyInUnCore,
			IdleEnergyType:          config.IdleEnergyInUnCore,
			ResourceUtilizationType: config.CPUTime,
			ScrapeInterval:          duration,
			CPUCount:                cpuCount,
			Config: IdleEnergyConfig{
				MinSpread:    minSpread,
				MinIntercept: 0.0,
				MinSlope:     0.01,
				HistorySize:  historySize,
			},
		})
	}
}

// CalcIdleEnergyLR calculates the idle energy for a specific energy type using linear regression.
// It initializes or updates the idle energy calculator based on the provided inputs.
//
// Parameters:
//   - input: An IdleEnergyModelInputs
//
// Behavior:
//   - If the total energy consumption is zero, the function skips the calculation due to
//     insufficient data.
//   - If the idle energy calculator for the given energy type does not exist, it initializes
//     a new calculator.
//   - If the calculator already exists, it updates the idle energy calculation using the
//     UpdateIdleEnergy method.
//   - The function checks if the calculated idle energy is reliable using checkIdlePowerReliability.
//   - Once idle energy is marked as reliable, it is permanent
//   - If the idle energy is reliable, it distributes the idle energy among the sockets using
//     distributeIdleEnergyAmongSockets (Note this is incorrect and must be corrected in future refactoring).
func (ne *NodeStats) CalcIdleEnergyLR(input *IdleEnergyModelInputs) {
	absType := input.AbsoluteEnergyType
	idleType := input.IdleEnergyType
	recourceUtilType := input.ResourceUtilizationType
	scrapeInterval := input.ScrapeInterval
	cpuCount := input.CPUCount

	totalResUtilization := ne.ResourceUsage[recourceUtilType].SumAllDeltaValues()
	totalEnergy := ne.EnergyUsage[absType].SumAllDeltaValues()
	klog.V(5).Infof("Does Res utilization have multiple packages?: %s", ne.ResourceUsage[recourceUtilType].String())
	for key := range ne.ResourceUsage[recourceUtilType] {
		klog.V(5).Infof("Key: %s", key)
	}
	klog.V(5).Infof("Energy Type: %s", absType)
	klog.V(5).Infof("Total Energy: %d", totalEnergy)
	if totalEnergy == 0 {
		// Insufficient Sample Size
		klog.V(5).Infof("Skipping Idle Energy")
		return
	}

	if _, exists := ne.IdleEnergy[idleType]; !exists {
		ne.IdleEnergy[idleType] = NewIdleEnergyCalculator(
			IdleEnergyCalculatorInitialization{
				InitialUtilizationX: float64(totalResUtilization),
				InitialUtilizationY: float64(totalEnergy),
				Config:              input.Config,
			},
		)
		klog.V(5).Infof("Initialize Idle Energy (%s): %f", idleType, ne.IdleEnergy[idleType].Result.CalculatedIdleEnergy)
	} else {
		klog.V(5).Infof("Sample Period: %d", scrapeInterval)
		maxTheoreticalCPUTime := cpuCount * scrapeInterval * 1000
		klog.V(5).Infof("Maximum CPU Time per Sample Period: %d", maxTheoreticalCPUTime)
		ne.IdleEnergy[idleType].UpdateIdleEnergy(
			NewEnergyPoint{
				float64(totalResUtilization),
				float64(totalEnergy),
				float64(maxTheoreticalCPUTime),
			},
		)
		klog.V(5).Infof("Stored Idle Energy (%s): %f", idleType, ne.IdleEnergy[idleType].Result.CalculatedIdleEnergy)
		klog.V(5).Infof("Check if Stored Idle Energy is reliable")
		calculator := ne.IdleEnergy[idleType]
		if !ne.IdleEnergy[idleType].IsIdlePowerReliable {
			ne.IdleEnergy[idleType].IsIdlePowerReliable = ne.checkIdlePowerReliability(calculator)
		}
		if ne.IdleEnergy[idleType].IsIdlePowerReliable {
			klog.V(5).Infof("Idle Energy has been approved. Passing Idle Energy.")
			klog.V(5).Infof("Total Idle Energy: %f", calculator.Result.CalculatedIdleEnergy)
			// Idle Energy Can now be divided among the sockets equally
			ne.distributeIdleEnergyAmongSockets(absType, idleType, calculator.Result.CalculatedIdleEnergy)
		}
	}
}

// distributeIdleEnergyAmongSockets distributes the total idle energy equally among all sockets
// for a given energy type. It updates the energy usage statistics for each socket with the
// allocated idle energy.
//
// Parameters:
//   - absM: The absolute energy type (e.g., abs package, DRAM).
//   - idleM: The idle energy type (e.g., idle package, idle DRAM).
//   - totalIdleEnergy: The total idle energy to be distributed.
//
// Behavior:
//   - The function calculates the idle energy per socket by dividing the total idle energy
//     by the number of sockets.
//   - It updates the energy usage statistics for each socket with the allocated idle energy.
func (ne *NodeStats) distributeIdleEnergyAmongSockets(absM, idleM string, totalIdleEnergy float64) {
	idleEnergyPerSocket := totalIdleEnergy / float64(len(ne.EnergyUsage[absM]))
	for socketID := range ne.EnergyUsage[absM] {
		klog.V(5).Infof("Socket ID: %s, Allocated Idle Energy: %f", socketID, idleEnergyPerSocket)
		ne.EnergyUsage[idleM].SetDeltaStat(socketID, uint64(idleEnergyPerSocket))
	}
}

// checkIdlePowerReliability checks if the calculated idle energy is reliable based on the
// spread of the data points and the consistency of the idle energy values in the history.
//
// Parameters:
//   - calculator: A pointer to the IdleEnergyCalculator
//
// Returns:
//   - A boolean indicating whether the idle energy is reliable (true) or not (false).
//
// Behavior:
//   - The function checks if the spread of the data points is greater than or equal to the
//     minimum required spread (MinSpread).
//   - It checks if the calculated idle energy is consistent with the average of the historical
//     idle energy values (within 10% difference).
//   - If both checks pass, the idle energy is considered reliable.
func (ne *NodeStats) checkIdlePowerReliability(calculator *IdleEnergyCalculator) bool {
	klog.V(5).Infof("Check if Sample Size is large enough")
	if calculator.Result.Spread < calculator.Config.MinSpread {
		klog.V(5).Infof("Sample is size is not large enough")
		return false
	}
	klog.V(5).Infof("Sample Size is large enough!")
	klog.V(5).Infof("Check if Idle Energy is consistent")
	if percentDiff(getAverage(calculator.Result.History), calculator.Result.CalculatedIdleEnergy) > 0.1 {
		klog.V(5).Infof("Idle Energy is not consistent")
		return false
	}
	klog.V(5).Infof("Idle Energy is Consistent! Idle Energy is ready to be passed.")
	return true
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

	for socketID, value := range ne.EnergyUsage[absM] {
		newIdleDelta := value.GetDelta()
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

// CPUCount returns number of CPUs on node
//
// Returns:
//   - Number of CPUs on node in uint64
func (ne *NodeStats) CPUCount() uint64 {
	return ne.nodeInfo.CPUCount()
}

func (ne *NodeStats) NodeName() string {
	return ne.nodeInfo.Name()
}
