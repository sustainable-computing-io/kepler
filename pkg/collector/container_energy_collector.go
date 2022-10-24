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
	"math"

	"k8s.io/klog/v2"

	collector_metric "github.com/sustainable-computing-io/kepler/pkg/collector/metric"
	"github.com/sustainable-computing-io/kepler/pkg/config"
	"github.com/sustainable-computing-io/kepler/pkg/model"
	"github.com/sustainable-computing-io/kepler/pkg/power/gpu"
	"github.com/sustainable-computing-io/kepler/pkg/power/rapl/source"
)

// The updateContainerResourceUsageMetricsr first updates the bpf which is resposible to include new containers in the ContainersMetrics collection
// the bpf collects metrics per processes and then map the process ids to container ids
// TODO: when bpf is not running, the ContainersMetrics will not be updated with new containers
// the ContainersMetrics will only have the containers that were identified during the initialization (initContainersMetrics)
func (c *Collector) updateContainerResourceUsageMetrics() {
	c.updateBPFMetrics() // collect new hardware counter metrics if possible
	// TODO: collect cgroup metrics only from cgroup to avoid unnecessary overhead to kubelet
	c.updateCgroupMetrics()  // collect new cgroup metrics from cgroup
	c.updateKubeletMetrics() // collect new cgroup metrics from kubelet
}

// The current implementation from the model server returns a list of the container energy.
// The list follows the order of the container containerMetricValuesOnly for the container id...
// TODO: add the updated resource usage and metadata metric to ContainersMetrics structure directly. We will need less collections
func (c *Collector) getContainerMetricsList() (containerMetricValuesOnly [][]float64, containerIDList []string, containerGPUDelta map[string]float64) {
	// TO-DO: handle metrics read by GPU device in the same way as the other usage metrics
	// read gpu power
	var gpuPerPid map[uint32]float64
	if config.EnabledGPU {
		gpuPerPid, _ = gpu.GetCurrGpuEnergyPerPid() // power not energy
	}

	// convert to pod metrics to array
	containerGPUDelta = make(map[string]float64)
	for containerID, c := range c.ContainersMetrics {
		values := c.ToEstimatorValues()
		containerMetricValuesOnly = append(containerMetricValuesOnly, values)
		containerIDList = append(containerIDList, containerID)

		// match container pid to gpu
		if config.EnabledGPU {
			for pid := range c.PIDS {
				gpuPower := gpuPerPid[uint32(pid)]
				containerGPUDelta[containerID] += gpuPower
			}
		}
	}
	return
}

// updateContainerEnergy matches the container resource usage with the node energy consumption
func (c *Collector) updateContainerEnergy(containerMetricValuesOnly [][]float64, containerIDList []string, containerGPUDelta map[string]float64,
	nodeTotalPower uint64, nodeTotalGPUPower uint64, nodeTotalPowerPerComponents source.RAPLPower) {
	// The current implementation from the model server returns a list of the container energy. The list follows the order of the container containerMetricValuesOnly for the container id...
	// TODO: change the model server to return a map with the containerID and its metrics to be more transparent and prevent errors.
	containerComponentPowers, containerOtherPowers := model.GetContainerPower(
		containerMetricValuesOnly, collector_metric.NodeMetadataValues,
		nodeTotalPower, nodeTotalGPUPower, nodeTotalPowerPerComponents,
	)

	// set container energy
	for i, containerID := range containerIDList {
		if err := c.ContainersMetrics[containerID].EnergyInCore.AddNewCurr(containerComponentPowers[i].Core); err != nil {
			klog.V(5).Infoln(err)
		}
		if err := c.ContainersMetrics[containerID].EnergyInDRAM.AddNewCurr(containerComponentPowers[i].DRAM); err != nil {
			klog.V(5).Infoln(err)
		}
		if err := c.ContainersMetrics[containerID].EnergyInUncore.AddNewCurr(containerComponentPowers[i].Uncore); err != nil {
			klog.V(5).Infoln(err)
		}
		if err := c.ContainersMetrics[containerID].EnergyInPkg.AddNewCurr(containerComponentPowers[i].Pkg); err != nil {
			klog.V(5).Infoln(err)
		}
		containerGPU := uint64(math.Ceil(containerGPUDelta[containerID]))
		if err := c.ContainersMetrics[containerID].EnergyInGPU.AddNewCurr(containerGPU); err != nil {
			klog.V(5).Infoln(err)
		}
		if err := c.ContainersMetrics[containerID].EnergyInOther.AddNewCurr(containerOtherPowers[i]); err != nil {
			klog.V(5).Infoln(err)
		}
	}
}
