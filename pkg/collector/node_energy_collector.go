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

package collector

import (
	"github.com/sustainable-computing-io/kepler/pkg/power/gpu"
	"github.com/sustainable-computing-io/kepler/pkg/power/rapl"
	"github.com/sustainable-computing-io/kepler/pkg/power/rapl/source"
)

// updateMeasuredNodeEnergy reads sensor/pkg energies in mJ
func (c *Collector) updateMeasuredNodeEnergy() {
	c.NodeSensorEnergy, _ = c.acpiPowerMeter.GetEnergyFromHost()
	c.NodePkgEnergy = rapl.GetRAPLEnergy()
	c.NodeGPUEnergy = gpu.GetGpuEnergy()
	c.NodeCPUFrequency = c.acpiPowerMeter.GetCPUCoreFrequency()
}

// updateContainerEnergy matches the container resource usage with the node energy consumption
// The node resource usage is computed by the total container containerMetricValuesOnly
// The container metrics are for the kubernetes containers and system/OS processes
// TODO: verify if the cgroup metrics are also accounting for the OS, not only containers
func (c *Collector) updateNoderMetrics(containerMetricValuesOnly [][]float64) (nodeTotalPower, nodeTotalGPUPower uint64, nodeTotalPowerPerComponents source.RAPLPower) {
	c.updateMeasuredNodeEnergy() // collect new energy metrics from the system if possible
	c.NodeMetrics.SetValues(c.NodeSensorEnergy, c.NodePkgEnergy, c.NodeGPUEnergy, containerMetricValuesOnly)
	nodeTotalPower = c.NodeMetrics.EnergyInPkg.Curr() + c.NodeMetrics.EnergyInDRAM.Curr() + c.NodeMetrics.EnergyInGPU.Curr() + c.NodeMetrics.EnergyInOther.Curr()
	nodeTotalGPUPower = c.NodeMetrics.EnergyInGPU.Curr()
	nodeTotalPowerPerComponents = c.NodeMetrics.GetNodeComponentPower()
	return
}
