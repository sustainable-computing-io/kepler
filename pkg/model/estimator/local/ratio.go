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
	"sync"

	"k8s.io/klog/v2"

	collector_metric "github.com/sustainable-computing-io/kepler/pkg/collector/metric"
	"github.com/sustainable-computing-io/kepler/pkg/config"
)

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

// UpdateContainerComponentEnergyByRatioPowerModel calculates the container energy consumption based on the resource utilization ratio
func UpdateContainerComponentEnergyByRatioPowerModel(containersMetrics map[string]*collector_metric.ContainerMetrics, nodeMetrics *collector_metric.NodeMetrics, component, usageMetric string, wg *sync.WaitGroup) {
	defer wg.Done()
	nodeTotalResourceUsage := float64(0)
	containerNumber := float64(len(containersMetrics))
	totalDynPower := float64(nodeMetrics.GetSumDeltaDynEnergyFromAllSources(component))
	totalIdlePower := float64(nodeMetrics.GetSumDeltaIdleEnergyromAllSources(component))
	// evenly divide the idle power to all containers. TODO: use the container resource limit
	idlePowerPerContainer := uint64(totalIdlePower / containerNumber)

	// if usageMetric exist, divide the power using the ratio. Otherwise, evenly divide the power.
	if usageMetric != "" {
		nodeTotalResourceUsage = nodeMetrics.GetNodeResUsagePerResType(usageMetric)
	}

	for containerID, container := range containersMetrics {
		containerResUsage := float64(0)
		if _, ok := container.CounterStats[usageMetric]; ok {
			containerResUsage = float64(container.CounterStats[usageMetric].Delta)
		} else if usageMetric == config.CPUTime {
			containerResUsage = float64(container.CPUTime.Delta)
		}
		if containerResUsage > 0 {
			containerEnergy := getEnergyRatio(containerResUsage, nodeTotalResourceUsage, totalDynPower, containerNumber)
			if err := containersMetrics[containerID].GetDynEnergyStat(component).AddNewDelta(containerEnergy); err != nil {
				klog.Infoln(err)
			}
		}
		if err := containersMetrics[containerID].GetIdleEnergyStat(component).AddNewDelta(idlePowerPerContainer); err != nil {
			klog.Infoln(err)
		}
	}
}
