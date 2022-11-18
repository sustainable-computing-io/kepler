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

func getSumMetricValues(containerMetricValues [][]float64) (sumMetricValues []float64) {
	if len(containerMetricValues) == 0 {
		return
	}
	sumMetricValues = make([]float64, len(containerMetricValues[0]))
	for _, values := range containerMetricValues {
		for index, containerMetricValue := range values {
			sumMetricValues[index] += containerMetricValue
		}
	}
	return
}

func getEnergyRatio(containerResUsage, nodeTotalResUsage, nodeResEnergyUtilization, containerNumber float64) uint64 {
	var power float64
	if nodeTotalResUsage > 0 {
		power = (containerResUsage / nodeTotalResUsage) * nodeResEnergyUtilization
	} else {
		// TODO: we should not equaly divide the energy consumptio across the containers. If a hardware counter metrics is not available we should use cgroup metrics.
		power = nodeResEnergyUtilization / containerNumber
	}
	return uint64(math.Ceil(power))
}

// UpdateContainerEnergyByRatioPowerModel calculates the container energy consumption based on the resource utilization ratio
func UpdateContainerEnergyByRatioPowerModel(containersMetrics map[string]*collector_metric.ContainerMetrics, nodeMetrics collector_metric.NodeMetrics) {
	nodeTotalEnergyPerComponent := nodeMetrics.GetNodeTotalEnergyPerComponent()
	containerNumber := float64(len(containersMetrics))

	for containerID, container := range containersMetrics {
		var containerResUsage, nodeTotalResUsage, nodeResEnergyUtilization float64

		// calculate the container package/socket energy consumption
		if _, ok := container.CounterStats[config.CoreUsageMetric]; ok {
			containerResUsage = float64(container.CounterStats[config.CoreUsageMetric].Curr)
			nodeTotalResUsage = nodeMetrics.GetNodeResUsagePerResType(config.CoreUsageMetric)
			nodeResEnergyUtilization = float64(nodeTotalEnergyPerComponent.Pkg)
			containerPkgEnergy := getEnergyRatio(containerResUsage, nodeTotalResUsage, nodeResEnergyUtilization, containerNumber)
			if err := containersMetrics[containerID].EnergyInPkg.AddNewCurr(containerPkgEnergy); err != nil {
				klog.Infoln(err)
			}

			// calculate the container core energy consumption
			nodeResEnergyUtilization = float64(nodeTotalEnergyPerComponent.Core)
			containerCoreEnergy := getEnergyRatio(containerResUsage, nodeTotalResUsage, nodeResEnergyUtilization, containerNumber)
			if err := containersMetrics[containerID].EnergyInCore.AddNewCurr(containerCoreEnergy); err != nil {
				klog.Infoln(err)
			}
		}

		// calculate the container uncore energy consumption
		nodeResEnergyUtilization = float64(nodeTotalEnergyPerComponent.Uncore)
		containerUncoreEnergy := uint64(math.Ceil(nodeResEnergyUtilization / containerNumber))
		if err := containersMetrics[containerID].EnergyInUncore.AddNewCurr(containerUncoreEnergy); err != nil {
			klog.Infoln(err)
		}

		// calculate the container dram energy consumption
		if _, ok := container.CounterStats[config.DRAMUsageMetric]; ok {
			containerResUsage = float64(container.CounterStats[config.DRAMUsageMetric].Curr)
			nodeTotalResUsage = nodeMetrics.GetNodeResUsagePerResType(config.DRAMUsageMetric)
			nodeResEnergyUtilization = float64(nodeTotalEnergyPerComponent.DRAM)
			containerDramEnergy := getEnergyRatio(containerResUsage, nodeTotalResUsage, nodeResEnergyUtilization, containerNumber)
			if err := containersMetrics[containerID].EnergyInDRAM.AddNewCurr(containerDramEnergy); err != nil {
				klog.Infoln(err)
			}
		}

		// calculate the container gpu energy consumption
		if accelerator.IsGPUCollectionSupported() {
			containerResUsage = float64(container.CounterStats[config.GpuUsageMetric].Curr)
			nodeTotalResUsage = nodeMetrics.GetNodeResUsagePerResType(config.GpuUsageMetric)
			nodeResEnergyUtilization = float64(nodeMetrics.GetEnergyValue(collector_metric.GPU))
			containerGPUEnergy := getEnergyRatio(containerResUsage, nodeTotalResUsage, nodeResEnergyUtilization, containerNumber)
			if err := containersMetrics[containerID].EnergyInGPU.AddNewCurr(containerGPUEnergy); err != nil {
				klog.Infoln(err)
			}
		}

		// calculate the container host other components energy consumption
		nodeResEnergyUtilization = float64(nodeMetrics.GetEnergyValue(collector_metric.OTHER))
		containerOtherHostComponentsEnergy := uint64(math.Ceil(nodeResEnergyUtilization / containerNumber))
		if err := containersMetrics[containerID].EnergyInOther.AddNewCurr(containerOtherHostComponentsEnergy); err != nil {
			klog.Infoln(err)
		}
	}
}
