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
	AvailableCGroupMetrics = []string{config.CgroupfsMemory, config.CgroupfsKernelMemory, config.CgroupfsTCPMemory}
	ContainerUintFeaturesNames = append(ContainerUintFeaturesNames, AvailableCGroupMetrics...)
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
	containerMetrics := NewContainerMetrics(containerName, podName, namespace, containerName)
	// cgroup - cgroup package
	// we need to add two aggregated values to the stats so that it can calculate a current value (i.e. agg diff)
	containerMetrics.CgroupStatMap[config.CgroupfsMemory].SetAggrStat(containerName, 100)
	containerMetrics.CgroupStatMap[config.CgroupfsMemory].SetAggrStat(containerName, 110)
	containerMetrics.CgroupStatMap[config.CgroupfsKernelMemory].SetAggrStat(containerName, 200)
	containerMetrics.CgroupStatMap[config.CgroupfsKernelMemory].SetAggrStat(containerName, 220)
	containerMetrics.CgroupStatMap[config.CgroupfsTCPMemory].SetAggrStat(containerName, 300)
	containerMetrics.CgroupStatMap[config.CgroupfsTCPMemory].SetAggrStat(containerName, 330)
	return containerMetrics
}

func createMockNodeMetrics(containersMetrics map[string]*ContainerMetrics) *NodeMetrics {
	nodeMetrics := NewNodeMetrics()
	nodeMetrics.AddNodeResUsageFromContainerResUsage(containersMetrics)
	componentsEnergies := make(map[int]source.NodeComponentsEnergy)
	machineSocketID := 0

	// the NodeComponentsEnergy is the aggregated energy consumption of the node components
	// then, the components energy consumption is added to the in the nodeMetrics as Agg data
	// this means that, to have a Delta value, we must have at least two Agg data (to have Agg diff)
	// therefore, we need to add two values for NodeComponentsEnergy to have energy values to test
	componentsEnergies[machineSocketID] = source.NodeComponentsEnergy{
		Pkg:  10,
		Core: 10,
		DRAM: 10,
	}
	nodeMetrics.SetNodeComponentsEnergy(componentsEnergies, false)
	componentsEnergies[machineSocketID] = source.NodeComponentsEnergy{
		Pkg:  18,
		Core: 15,
		DRAM: 11,
	}
	nodeMetrics.SetNodeComponentsEnergy(componentsEnergies, false)

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

	It("test AddNodeGPUEnergy", func() {
		gpuEnergy := make([]uint32, 1)
		nodeMetrics.AddNodeGPUEnergy(gpuEnergy)
	})

	It("test ResetDeltaValues", func() {
		nodeMetrics.ResetDeltaValues()
		Expect("0 (15)").To(Equal(nodeMetrics.TotalEnergyInCore.String()))
	})

	It("test UpdateIdleEnergy", func() {
		nodeMetrics.UpdateIdleEnergy()
		Expect(nodeMetrics.FoundNewIdleState).To(BeFalse())
	})

	It("test String", func() {
		str := nodeMetrics.String()
		Expect("node delta energy (mJ):").To(Equal(str[0:len("node delta energy (mJ):")]))
	})

	It("test GetNodeResUsagePerResType", func() {
		val, _ := nodeMetrics.GetNodeResUsagePerResType("")
		Expect(float64(0)).To(Equal(val))
	})
})
