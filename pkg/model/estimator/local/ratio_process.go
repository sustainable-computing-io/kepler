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
ratio_process.go
calculate processess' component and other power by ratio approach when node power is available.
*/

package local

import (
	"math"

	"github.com/sustainable-computing-io/kepler/pkg/config"
	"github.com/sustainable-computing-io/kepler/pkg/power/accelerator"
	"k8s.io/klog/v2"

	collector_metric "github.com/sustainable-computing-io/kepler/pkg/collector/metric"
)

// UpdateProcessEnergyByRatioPowerModel calculates the process energy consumption based on the energy consumption of the container that contains all the processes (which is a special container metrics for all system processes)
func UpdateProcessEnergyByRatioPowerModel(processMetrics map[uint64]*collector_metric.ProcessMetrics, systemProcessMetrics *collector_metric.ContainerMetrics) {
	pkgDynPower := float64(systemProcessMetrics.DynEnergyInPkg.Delta)
	coreDynPower := float64(systemProcessMetrics.DynEnergyInCore.Delta)
	uncoreDynPower := float64(systemProcessMetrics.DynEnergyInUncore.Delta)
	dramDynPower := float64(systemProcessMetrics.DynEnergyInDRAM.Delta)
	otherDynPower := float64(systemProcessMetrics.DynEnergyInOther.Delta)
	gpuDynPower := float64(systemProcessMetrics.DynEnergyInGPU.Delta)

	processNumber := float64(len(processMetrics))
	// evenly divide the idle power to all processes. TODO: use the process resource request
	pkgIdlePowerPerProcess := systemProcessMetrics.IdleEnergyInPkg.Delta / uint64(processNumber)
	coreIdlePowerPerProcess := systemProcessMetrics.IdleEnergyInCore.Delta / uint64(processNumber)
	uncoreIdlePowerPerProcess := systemProcessMetrics.IdleEnergyInUncore.Delta / uint64(processNumber)
	dramIdlePowerPerProcess := systemProcessMetrics.IdleEnergyInDRAM.Delta / uint64(processNumber)
	otherIdlePowerPerProcess := systemProcessMetrics.IdleEnergyInOther.Delta / uint64(processNumber)

	for pid, process := range processMetrics {
		var processResUsage, containerTotalResUsage float64

		// calculate the process package/socket energy consumption
		if _, ok := process.CounterStats[config.CoreUsageMetric]; ok {
			processResUsage = float64(process.CounterStats[config.CoreUsageMetric].Delta)
			containerTotalResUsage = float64(systemProcessMetrics.CounterStats[config.CoreUsageMetric].Delta)
			processPkgEnergy := getEnergyRatio(processResUsage, containerTotalResUsage, pkgDynPower, processNumber)
			if err := processMetrics[pid].DynEnergyInPkg.AddNewDelta(processPkgEnergy); err != nil {
				klog.Infoln(err)
			}
			// calculate the process core energy consumption
			processCoreEnergy := getEnergyRatio(processResUsage, containerTotalResUsage, coreDynPower, processNumber)
			if err := processMetrics[pid].DynEnergyInCore.AddNewDelta(processCoreEnergy); err != nil {
				klog.Infoln(err)
			}
		}

		// calculate the process uncore energy consumption
		processUncoreEnergy := uint64(math.Ceil(uncoreDynPower / processNumber))
		if err := processMetrics[pid].DynEnergyInUncore.AddNewDelta(processUncoreEnergy); err != nil {
			klog.Infoln(err)
		}

		// calculate the process dram energy consumption
		if _, ok := process.CounterStats[config.DRAMUsageMetric]; ok {
			processResUsage = float64(process.CounterStats[config.DRAMUsageMetric].Delta)
			containerTotalResUsage = float64(systemProcessMetrics.CounterStats[config.DRAMUsageMetric].Delta)
			processDramEnergy := getEnergyRatio(processResUsage, containerTotalResUsage, dramDynPower, processNumber)
			if err := processMetrics[pid].DynEnergyInDRAM.AddNewDelta(processDramEnergy); err != nil {
				klog.Infoln(err)
			}
		}

		// calculate the process gpu energy consumption
		if accelerator.IsGPUCollectionSupported() {
			processResUsage = float64(process.CounterStats[config.GpuUsageMetric].Delta)
			containerTotalResUsage = float64(systemProcessMetrics.CounterStats[config.GpuUsageMetric].Delta)
			processGPUEnergy := getEnergyRatio(processResUsage, containerTotalResUsage, gpuDynPower, processNumber)
			if err := processMetrics[pid].DynEnergyInGPU.AddNewDelta(processGPUEnergy); err != nil {
				klog.Infoln(err)
			} else {
				klog.V(5).Infof("gpu power ratio: pid %v processResUsage: %f, nodeTotalResUsage: %f, nodeResEnergyUtilization: %f, processNumber: %f processGPUEnergy: %v",
					pid, processResUsage, containerTotalResUsage, gpuDynPower, processNumber, processMetrics[pid].DynEnergyInGPU.Delta)
			}
		}

		// calculate the process host other components energy consumption
		processOtherHostComponentsEnergy := uint64(math.Ceil(otherDynPower / processNumber))
		if err := processMetrics[pid].DynEnergyInOther.AddNewDelta(processOtherHostComponentsEnergy); err != nil {
			klog.Infoln(err)
		}

		// Idle energy
		if err := processMetrics[pid].IdleEnergyInPkg.AddNewDelta(pkgIdlePowerPerProcess); err != nil {
			klog.Infoln(err)
		}
		if err := processMetrics[pid].IdleEnergyInCore.AddNewDelta(coreIdlePowerPerProcess); err != nil {
			klog.Infoln(err)
		}
		if err := processMetrics[pid].IdleEnergyInUncore.AddNewDelta(uncoreIdlePowerPerProcess); err != nil {
			klog.Infoln(err)
		}
		if err := processMetrics[pid].IdleEnergyInDRAM.AddNewDelta(dramIdlePowerPerProcess); err != nil {
			klog.Infoln(err)
		}
		if err := processMetrics[pid].IdleEnergyInOther.AddNewDelta(otherIdlePowerPerProcess); err != nil {
			klog.Infoln(err)
		}
	}
}
