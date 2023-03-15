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

package local

import (
	"math"

	"github.com/sustainable-computing-io/kepler/pkg/config"
	"github.com/sustainable-computing-io/kepler/pkg/power/accelerator"
	"k8s.io/klog/v2"

	collector_metric "github.com/sustainable-computing-io/kepler/pkg/collector/metric"
)

func getSumMetricValues(metricValues [][]float64) (sumMetricValues []float64) {
	if len(metricValues) == 0 {
		return
	}
	sumMetricValues = make([]float64, len(metricValues[0]))
	for _, values := range metricValues {
		for index, metricValue := range values {
			sumMetricValues[index] += metricValue
		}
	}
	return
}

func getEnergyRatio(unitResUsage, totalResUsage, resEnergyUtilization, totalNumber float64) uint64 {
	var power float64
	if totalResUsage > 0 {
		power = (unitResUsage / totalResUsage) * resEnergyUtilization
	} else {
		// TODO: we should not equaly divide the energy consumption across the all processes or containers. If a hardware counter metrics is not available we should use cgroup metrics.
		power = resEnergyUtilization / totalNumber
	}
	return uint64(math.Ceil(power))
}

// UpdateContainerEnergyByRatioPowerModel calculates the container energy consumption based on the resource utilization ratio
func UpdateContainerEnergyByRatioPowerModel(containersMetrics map[string]*collector_metric.ContainerMetrics, nodeMetrics *collector_metric.NodeMetrics) {
	pkgDynPower := float64(nodeMetrics.GetSumDeltaDynEnergyFromAllSources(collector_metric.PKG))
	coreDynPower := float64(nodeMetrics.GetSumDeltaDynEnergyFromAllSources(collector_metric.CORE))
	uncoreDynPower := float64(nodeMetrics.GetSumDeltaDynEnergyFromAllSources(collector_metric.UNCORE))
	dramDynPower := float64(nodeMetrics.GetSumDeltaDynEnergyFromAllSources(collector_metric.DRAM))
	otherDynPower := float64(nodeMetrics.GetSumDeltaDynEnergyFromAllSources(collector_metric.OTHER))
	gpuDynPower := float64(nodeMetrics.GetSumDeltaDynEnergyFromAllSources(collector_metric.GPU))

	containerNumber := float64(len(containersMetrics))
	// evenly divide the idle power to all containers. TODO: use the container resource request
	pkgIdlePowerPerContainer := nodeMetrics.GetSumDeltaIdleEnergyromAllSources(collector_metric.PKG) / uint64(containerNumber)
	coreIdlePowerPerContainer := nodeMetrics.GetSumDeltaIdleEnergyromAllSources(collector_metric.CORE) / uint64(containerNumber)
	uncoreIdlePowerPerContainer := nodeMetrics.GetSumDeltaIdleEnergyromAllSources(collector_metric.UNCORE) / uint64(containerNumber)
	dramIdlePowerPerContainer := nodeMetrics.GetSumDeltaIdleEnergyromAllSources(collector_metric.DRAM) / uint64(containerNumber)
	otherIdlePowerPerContainer := nodeMetrics.GetSumDeltaIdleEnergyromAllSources(collector_metric.OTHER) / uint64(containerNumber)

	containerUncoreEnergy := uint64(math.Ceil(uncoreDynPower / containerNumber))
	containerOtherHostComponentsEnergy := uint64(math.Ceil(otherDynPower / containerNumber))
	NodeCoreUsageMetric := nodeMetrics.GetNodeResUsagePerResType(config.CoreUsageMetric)
	NodeDRAMUsageMetric := nodeMetrics.GetNodeResUsagePerResType(config.DRAMUsageMetric)
	NodeGpuUsageMetric := nodeMetrics.GetNodeResUsagePerResType(config.GpuUsageMetric)
	for containerID, container := range containersMetrics {
		var containerResUsage, nodeTotalResUsage float64

		// calculate the container package/socket energy consumption
		if _, ok := container.CounterStats[config.CoreUsageMetric]; ok {
			containerResUsage = float64(container.CounterStats[config.CoreUsageMetric].Delta)
			nodeTotalResUsage = NodeCoreUsageMetric
			containerPkgEnergy := getEnergyRatio(containerResUsage, nodeTotalResUsage, pkgDynPower, containerNumber)
			if err := containersMetrics[containerID].DynEnergyInPkg.AddNewDelta(containerPkgEnergy); err != nil {
				klog.Infoln(err)
			}

			// calculate the container core energy consumption
			containerCoreEnergy := getEnergyRatio(containerResUsage, nodeTotalResUsage, coreDynPower, containerNumber)
			if err := containersMetrics[containerID].DynEnergyInCore.AddNewDelta(containerCoreEnergy); err != nil {
				klog.Infoln(err)
			}
		}

		// calculate the container uncore energy consumption
		if err := containersMetrics[containerID].DynEnergyInUncore.AddNewDelta(containerUncoreEnergy); err != nil {
			klog.Infoln(err)
		}

		// calculate the container dram energy consumption
		if _, ok := container.CounterStats[config.DRAMUsageMetric]; ok {
			containerResUsage = float64(container.CounterStats[config.DRAMUsageMetric].Delta)
			nodeTotalResUsage = NodeDRAMUsageMetric
			containerDramEnergy := getEnergyRatio(containerResUsage, nodeTotalResUsage, dramDynPower, containerNumber)
			if err := containersMetrics[containerID].DynEnergyInDRAM.AddNewDelta(containerDramEnergy); err != nil {
				klog.Infoln(err)
			}
		}

		// calculate the container gpu energy consumption
		if accelerator.IsGPUCollectionSupported() {
			containerResUsage = float64(container.CounterStats[config.GpuUsageMetric].Delta)
			nodeTotalResUsage = NodeGpuUsageMetric
			containerGPUEnergy := getEnergyRatio(containerResUsage, nodeTotalResUsage, gpuDynPower, containerNumber)
			if err := containersMetrics[containerID].DynEnergyInGPU.AddNewDelta(containerGPUEnergy); err != nil {
				klog.Infoln(err)
			} else {
				klog.V(5).Infof("gpu power ratio: containerID %v containerResUsage: %f, nodeTotalResUsage: %f, nodeResEnergyUtilization: %f, containerNumber: %f containerGPUEnergy: %v",
					containerID, containerResUsage, nodeTotalResUsage, gpuDynPower, containerNumber, containersMetrics[containerID].DynEnergyInGPU.Delta)
			}
		}

		// calculate the container host other components energy consumption
		if err := containersMetrics[containerID].DynEnergyInOther.AddNewDelta(containerOtherHostComponentsEnergy); err != nil {
			klog.Infoln(err)
		}
		// Idle energy
		if err := containersMetrics[containerID].IdleEnergyInPkg.AddNewDelta(pkgIdlePowerPerContainer); err != nil {
			klog.Infoln(err)
		}
		if err := containersMetrics[containerID].IdleEnergyInCore.AddNewDelta(coreIdlePowerPerContainer); err != nil {
			klog.Infoln(err)
		}
		if err := containersMetrics[containerID].IdleEnergyInUncore.AddNewDelta(uncoreIdlePowerPerContainer); err != nil {
			klog.Infoln(err)
		}
		if err := containersMetrics[containerID].IdleEnergyInDRAM.AddNewDelta(dramIdlePowerPerContainer); err != nil {
			klog.Infoln(err)
		}
		if err := containersMetrics[containerID].IdleEnergyInOther.AddNewDelta(otherIdlePowerPerContainer); err != nil {
			klog.Infoln(err)
		}
	}
}
