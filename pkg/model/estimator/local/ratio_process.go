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
	"sync"

	"k8s.io/klog/v2"

	collector_metric "github.com/sustainable-computing-io/kepler/pkg/collector/metric"
	"github.com/sustainable-computing-io/kepler/pkg/collector/metric/types"
	"github.com/sustainable-computing-io/kepler/pkg/config"
)

// getDataFromAlternativeMetric is used when there is no Hardware counter so we have to use cgroup or eBPF (CPUTime)
func getDataFromAlternativeMetric(cputime *types.UInt64Stat, usageMetrics string) float64 {
	// Given there is no HW counter, we have to use cgroup data, and Only CPUTime is available today
	// So busy CPU is more likely to be accessing memory. Although CPU utilization (CPUTime) does not
	// directly represent memory access, it remains the only viable proxy available to approximate such information reliably.
	switch usageMetrics {
	case config.CoreUsageMetric:
	case config.DRAMUsageMetric:
		return float64(cputime.Delta)
	case config.GpuUsageMetric:
		// FIXME: find cgroup data in cgroup
		return 0
	default:
		return 0
	}
	return 0
}

func getNodeTotalProcessResourceUsage(containerMetrics *collector_metric.ContainerMetrics, usageMetric string) float64 {
	var nodeTotalResourceUsage float64
	if _, ok := containerMetrics.CounterStats[usageMetric]; ok {
		nodeTotalResourceUsage = float64(containerMetrics.CounterStats[usageMetric].Delta)
	} else {
		// this will be system container (all processes's CPU Time)
		nodeTotalResourceUsage = getDataFromAlternativeMetric(containerMetrics.CPUTime, usageMetric)
	}
	return nodeTotalResourceUsage
}

func getProcessResUsage(process *collector_metric.ProcessMetrics, usageMetric string) float64 {
	var processResUsage float64
	if _, ok := process.CounterStats[usageMetric]; ok {
		processResUsage = float64(process.CounterStats[usageMetric].Delta)
	} else {
		processResUsage = getDataFromAlternativeMetric(process.CPUTime, usageMetric)
	}
	return processResUsage
}

// TODO: we should not calculate the process power based on the container power. Instead of using the container metrics we should use the system metrics and we do in the container power model.
// UpdateProcessComponentEnergyByRatioPowerModel calculates the process energy consumption based on the energy consumption of the container that contains all the processes
func UpdateProcessComponentEnergyByRatioPowerModel(processMetrics map[uint64]*collector_metric.ProcessMetrics, containerMetrics *collector_metric.ContainerMetrics, component, usageMetric string, wg *sync.WaitGroup) {
	defer wg.Done()
	nodeTotalResourceUsage := float64(-1)
	processesNumber := float64(len(processMetrics))
	totalDynPower := float64(containerMetrics.GetDynEnergyStat(component).Delta)

	// evenly divide the idle power to all containers. TODO: use the container resource request
	idlePowerPerProcess := containerMetrics.GetIdleEnergyStat(component).Delta / uint64(processesNumber)

	// if usageMetric exist, divide the power using the ratio. Otherwise, evenly divide the power.
	if usageMetric != "" {
		nodeTotalResourceUsage = getNodeTotalProcessResourceUsage(containerMetrics, usageMetric)
	}

	for pid, process := range processMetrics {
		processResUsage := getProcessResUsage(process, usageMetric)
		processPkgEnergy := getEnergyRatio(processResUsage, nodeTotalResourceUsage, totalDynPower, processesNumber)
		if err := processMetrics[pid].GetDynEnergyStat(component).AddNewDelta(processPkgEnergy); err != nil {
			klog.Infoln(err)
		}
		if err := processMetrics[pid].GetIdleEnergyStat(component).AddNewDelta(idlePowerPerProcess); err != nil {
			klog.Infoln(err)
		}
	}
}
