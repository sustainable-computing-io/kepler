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

package stats

import (
	"strconv"

	"github.com/sustainable-computing-io/kepler/pkg/config"
	acc "github.com/sustainable-computing-io/kepler/pkg/sensors/accelerator"
	"k8s.io/klog/v2"
)

const (
	MockedSocketID = "socket0"
)

// SetMockedCollectorMetrics adds all metric to a process, otherwise it will not create the right usageMetric with all elements. The usageMetric is used in the Prediction Power Models
// TODO: do not use a fixed usageMetric array in the power models, a structured data is more disarable.
func SetMockedCollectorMetrics() {
	config.GetConfig()
	if gpu := acc.GetActiveAcceleratorByType(config.GPU); gpu != nil {
		err := gpu.Device().Init() // create structure instances that will be accessed to create a processMetric
		klog.Fatalln(err)
	}
}

// CreateMockedProcessStats adds two containers with all metrics initialized
func CreateMockedProcessStats(numContainers int) map[uint64]*ProcessStats {
	processMetrics := map[uint64]*ProcessStats{}
	for i := 1; i <= numContainers; i++ {
		processMetrics[uint64(i)] = createMockedProcessMetric(i)
	}
	return processMetrics
}

// createMockedProcessMetric creates a new process metric with data
func createMockedProcessMetric(idx int) *ProcessStats {
	containerID := "container" + strconv.Itoa(idx)
	vmID := "vm" + strconv.Itoa(idx)
	command := "command" + strconv.Itoa(idx)
	uintPid := uint64(idx)
	processMetrics := NewProcessStats(uintPid, uintPid, containerID, vmID, command)
	// counter - attacher package
	processMetrics.ResourceUsage[config.CPUCycle].SetDeltaStat(MockedSocketID, 30000)
	processMetrics.ResourceUsage[config.CPUInstruction].SetDeltaStat(MockedSocketID, 30000)
	processMetrics.ResourceUsage[config.CacheMiss].SetDeltaStat(MockedSocketID, 30000)
	// bpf - cpu time
	processMetrics.ResourceUsage[config.CPUTime].SetDeltaStat(MockedSocketID, 30000) // config.CPUTime
	return processMetrics
}

// CreateMockedNodeStats creates a node metric with power consumption and add the process resource utilization
func CreateMockedNodeStats() NodeStats {
	nodeMetrics := NewNodeStats()
	// add power metrics
	// add first values to be the idle power
	nodeMetrics.EnergyUsage[config.AbsEnergyInPkg].SetDeltaStat(MockedSocketID, 5000) // mili joules
	nodeMetrics.EnergyUsage[config.AbsEnergyInCore].SetDeltaStat(MockedSocketID, 5000)
	nodeMetrics.EnergyUsage[config.AbsEnergyInDRAM].SetDeltaStat(MockedSocketID, 5000)
	nodeMetrics.EnergyUsage[config.AbsEnergyInPlatform].SetDeltaStat(MockedSocketID, 5000)
	// add second values to have dynamic power
	nodeMetrics.EnergyUsage[config.AbsEnergyInPkg].SetDeltaStat(MockedSocketID, 10000)
	nodeMetrics.EnergyUsage[config.AbsEnergyInCore].SetDeltaStat(MockedSocketID, 10000)
	nodeMetrics.EnergyUsage[config.AbsEnergyInDRAM].SetDeltaStat(MockedSocketID, 10000)
	nodeMetrics.EnergyUsage[config.AbsEnergyInPlatform].SetDeltaStat(MockedSocketID, 10000)
	nodeMetrics.UpdateIdleEnergyWithMinValue(true)
	// add second values to have dynamic power
	nodeMetrics.EnergyUsage[config.AbsEnergyInPkg].SetDeltaStat(MockedSocketID, 45000)
	nodeMetrics.EnergyUsage[config.AbsEnergyInCore].SetDeltaStat(MockedSocketID, 45000)
	nodeMetrics.EnergyUsage[config.AbsEnergyInDRAM].SetDeltaStat(MockedSocketID, 45000)
	nodeMetrics.EnergyUsage[config.AbsEnergyInPlatform].SetDeltaStat(MockedSocketID, 45000)
	nodeMetrics.UpdateDynEnergy()

	return *nodeMetrics
}
