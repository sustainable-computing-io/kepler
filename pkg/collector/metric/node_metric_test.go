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

package metric

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sustainable-computing-io/kepler/pkg/config"
	"github.com/sustainable-computing-io/kepler/pkg/power/components/source"
)

// we need to add all metric to a container, otherwise it will not create the right usageMetric with all elements. The usageMetric is used in the Prediction Power Models
// TODO: do not use a fixed usageMetric array in the power models, a structured data is more disarable.
func setCollectorMetrics() {
	// initialize the Available metrics since they are used to create a new containersMetrics instance
	AvailableCgroupMetrics = []string{config.CgroupfsMemory, config.CgroupfsKernelMemory, config.CgroupfsTCPMemory}
	ContainerUintFeaturesNames = append(ContainerUintFeaturesNames, AvailableCgroupMetrics...)
	// ContainerMetricNames is used by the nodeMetrics to extract the resource usage. Only the metrics in ContainerMetricNames will be used.
	ContainerMetricNames = ContainerUintFeaturesNames
}

// add two containers with all metrics initialized
func createMockContainersMetrics() map[string]*ContainerMetrics {
	containersMetrics := map[string]*ContainerMetrics{}
	containersMetrics["containerA"] = createMockContainerMetrics("containerA", "podA", "test")

	return containersMetrics
}

// see usageMetrics for the list of used metrics. For the sake of visibility we add all metrics, but only few of them will be used.
func createMockContainerMetrics(containerName, podName, namespace string) *ContainerMetrics {
	containerMetrics := NewContainerMetrics(containerName, podName, namespace)
	// cgroup - cgroup package
	// we need to add two aggregated values to the stats so that it can calculate a current value (i.e. agg diff)
	containerMetrics.CgroupFSStats[config.CgroupfsMemory].AddAggrStat(containerName, 100)
	containerMetrics.CgroupFSStats[config.CgroupfsMemory].AddAggrStat(containerName, 110)
	containerMetrics.CgroupFSStats[config.CgroupfsKernelMemory].AddAggrStat(containerName, 200)
	containerMetrics.CgroupFSStats[config.CgroupfsKernelMemory].AddAggrStat(containerName, 220)
	containerMetrics.CgroupFSStats[config.CgroupfsTCPMemory].AddAggrStat(containerName, 300)
	containerMetrics.CgroupFSStats[config.CgroupfsTCPMemory].AddAggrStat(containerName, 330)
	return containerMetrics
}

func createMockNodeMetrics(containersMetrics map[string]*ContainerMetrics) *NodeMetrics {
	nodeMetrics := NewNodeMetrics()
	nodeMetrics.AddNodeResUsageFromContainerResUsage(containersMetrics)
	componentsEnergies := make(map[int]source.NodeComponentsEnergy)
	machineSocketID := 0

	// the NodeComponentsEnergy is the aggregated energy consumption of the node components
	// then, the components energy consumption is added to the in the nodeMetrics as Agg data
	// this means that, to have a Curr value, we must have at least two Agg data (to have Agg diff)
	// therefore, we need to add two values for NodeComponentsEnergy to have energy values to test
	componentsEnergies[machineSocketID] = source.NodeComponentsEnergy{
		Pkg:  10,
		Core: 10,
		DRAM: 10,
	}
	nodeMetrics.AddNodeComponentsEnergy(componentsEnergies)
	componentsEnergies[machineSocketID] = source.NodeComponentsEnergy{
		Pkg:  18,
		Core: 15,
		DRAM: 11,
	}
	nodeMetrics.AddNodeComponentsEnergy(componentsEnergies)

	return nodeMetrics
}

var _ = Describe("Test Node Metric", func() {
	var (
		containerMetrics map[string]*ContainerMetrics
		nodeMetrics      *NodeMetrics
	)

	BeforeEach(func() {
		setCollectorMetrics()
		containerMetrics = createMockContainersMetrics()
		nodeMetrics = createMockNodeMetrics(containerMetrics)
	})

	It("Test nodeMetrics ResourceUsage", func() {
		v, ok := nodeMetrics.ResourceUsage[config.CgroupfsMemory]
		Expect(ok).To(Equal(true))
		Expect(v).To(Equal(float64(10)))
	})

	It("Test GetPrometheusEnergyValue", func() {
		out := nodeMetrics.GetEnergyValue(CORE)
		Expect(out).To(Equal(uint64(5)))
	})

	It("Test getEnergyValue dram", func() {
		cur := nodeMetrics.GetEnergyValue(DRAM)
		Expect(nodeMetrics.EnergyInDRAM.Curr()).To(Equal(cur))
	})

	It("Test getEnergyValue uncore", func() {
		cur := nodeMetrics.GetEnergyValue(UNCORE)
		Expect(nodeMetrics.EnergyInUncore.Curr()).To(Equal(cur))
	})

	It("Test getEnergyValue pkg", func() {
		cur := nodeMetrics.GetEnergyValue(PKG)
		Expect(nodeMetrics.EnergyInPkg.Curr()).To(Equal(cur))
	})

	It("Test getEnergyValue gpu", func() {
		cur := nodeMetrics.GetEnergyValue(GPU)
		Expect(nodeMetrics.EnergyInGPU.Curr()).To(Equal(cur))
	})

	It("Test getEnergyValue other", func() {
		cur := nodeMetrics.GetEnergyValue(OTHER)
		Expect(nodeMetrics.EnergyInOther.Curr()).To(Equal(cur))
	})

	It("test AddNodeGPUEnergy", func() {
		gpuEnergy := make([]uint32, 1)
		nodeMetrics.AddNodeGPUEnergy(gpuEnergy)
	})

	It("test GetNodeTotalEnergyPerComponent", func() {
		cur := nodeMetrics.GetNodeTotalEnergyPerComponent()
		Expect(uint64(5)).To(Equal(cur.Core))
		Expect(uint64(0)).To(Equal(cur.Uncore))
		Expect(uint64(8)).To(Equal(cur.Pkg))
	})

	It("test GetNodeTotalEnergy", func() {
		cur := nodeMetrics.GetNodeTotalEnergy()
		Expect(uint64(9)).To(Equal(cur))
	})
})
