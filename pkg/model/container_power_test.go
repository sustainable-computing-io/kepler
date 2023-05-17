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

package model

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	collector_metric "github.com/sustainable-computing-io/kepler/pkg/collector/metric"
	"github.com/sustainable-computing-io/kepler/pkg/config"
	"github.com/sustainable-computing-io/kepler/pkg/power/accelerator"
	"github.com/sustainable-computing-io/kepler/pkg/power/components/source"
)

// we need to add all metric to a container, otherwise it will not create the right usageMetric with all elements. The usageMetric is used in the Prediction Power Models
// TODO: do not use a fixed usageMetric array in the power models, a structured data is more disarable.
func setCollectorMetrics() {
	if accelerator.IsGPUCollectionSupported() {
		err := accelerator.Init() // create structure instances that will be accessed to create a containerMetric
		Expect(err).NotTo(HaveOccurred())
	}
	// initialize the Available metrics since they are used to create a new containersMetrics instance
	collector_metric.AvailableHWCounters = []string{config.CPUCycle, config.CPUInstruction, config.CacheMiss}
	collector_metric.AvailableCGroupMetrics = []string{config.CgroupfsMemory, config.CgroupfsKernelMemory, config.CgroupfsTCPMemory, config.CgroupfsCPU,
		config.CgroupfsSystemCPU, config.CgroupfsUserCPU, config.CgroupfsReadIO, config.CgroupfsWriteIO, config.BlockDevicesIO}
	collector_metric.AvailableKubeletMetrics = []string{config.KubeletContainerCPU, config.KubeletContainerMemory, config.KubeletNodeCPU, config.KubeletNodeMemory}
	collector_metric.ContainerUintFeaturesNames = append(collector_metric.ContainerUintFeaturesNames, collector_metric.AvailableEBPFCounters...)
	collector_metric.ContainerUintFeaturesNames = append(collector_metric.ContainerUintFeaturesNames, collector_metric.AvailableHWCounters...)
	collector_metric.ContainerUintFeaturesNames = append(collector_metric.ContainerUintFeaturesNames, collector_metric.AvailableCGroupMetrics...)
	collector_metric.ContainerUintFeaturesNames = append(collector_metric.ContainerUintFeaturesNames, collector_metric.AvailableKubeletMetrics...)
	// ContainerMetricNames is used by the nodeMetrics to extract the resource usage. Only the metrics in ContainerMetricNames will be used.
	collector_metric.ContainerMetricNames = collector_metric.ContainerUintFeaturesNames
}

// add two containers with all metrics initialized
func createMockContainersMetrics() map[string]*collector_metric.ContainerMetrics {
	containersMetrics := map[string]*collector_metric.ContainerMetrics{}
	containersMetrics["containerA"] = createMockContainerMetrics("containerA", "podA", "test")
	containersMetrics["containerB"] = createMockContainerMetrics("containerB", "podB", "test")

	return containersMetrics
}

// see usageMetrics for the list of used metrics. For the sake of visibility we add all metrics, but only few of them will be used.
func createMockContainerMetrics(containerName, podName, namespace string) *collector_metric.ContainerMetrics {
	containerMetrics := collector_metric.NewContainerMetrics(containerName, podName, namespace, containerName)
	// counter - attacher package
	err := containerMetrics.CounterStats[config.CPUCycle].AddNewDelta(10)
	Expect(err).NotTo(HaveOccurred())
	err = containerMetrics.CounterStats[config.CPUInstruction].AddNewDelta(10)
	Expect(err).NotTo(HaveOccurred())
	err = containerMetrics.CounterStats[config.CacheMiss].AddNewDelta(10)
	Expect(err).NotTo(HaveOccurred())
	// bpf - cpu time
	err = containerMetrics.CPUTime.AddNewDelta(10) // config.CPUTime
	Expect(err).NotTo(HaveOccurred())
	// cgroup - cgroup package
	// we need to add two aggregated values to the stats so that it can calculate a current value (i.e. agg diff)
	containerMetrics.CgroupStatMap[config.CgroupfsMemory].SetAggrStat(containerName, 10)
	containerMetrics.CgroupStatMap[config.CgroupfsMemory].SetAggrStat(containerName, 20)
	containerMetrics.CgroupStatMap[config.CgroupfsKernelMemory].SetAggrStat(containerName, 10) // not used
	containerMetrics.CgroupStatMap[config.CgroupfsKernelMemory].SetAggrStat(containerName, 20) // not used
	containerMetrics.CgroupStatMap[config.CgroupfsTCPMemory].SetAggrStat(containerName, 10)    // not used
	containerMetrics.CgroupStatMap[config.CgroupfsTCPMemory].SetAggrStat(containerName, 20)    // not used
	containerMetrics.CgroupStatMap[config.CgroupfsCPU].SetAggrStat(containerName, 10)
	containerMetrics.CgroupStatMap[config.CgroupfsCPU].SetAggrStat(containerName, 20)
	containerMetrics.CgroupStatMap[config.CgroupfsSystemCPU].SetAggrStat(containerName, 10)
	containerMetrics.CgroupStatMap[config.CgroupfsSystemCPU].SetAggrStat(containerName, 20)
	containerMetrics.CgroupStatMap[config.CgroupfsUserCPU].SetAggrStat(containerName, 10)
	containerMetrics.CgroupStatMap[config.CgroupfsUserCPU].SetAggrStat(containerName, 20)
	containerMetrics.CgroupStatMap[config.CgroupfsReadIO].SetAggrStat(containerName, 10)  // not used
	containerMetrics.CgroupStatMap[config.CgroupfsReadIO].SetAggrStat(containerName, 20)  // not used
	containerMetrics.CgroupStatMap[config.CgroupfsWriteIO].SetAggrStat(containerName, 10) // not used
	containerMetrics.CgroupStatMap[config.CgroupfsWriteIO].SetAggrStat(containerName, 20) // not used
	containerMetrics.CgroupStatMap[config.BlockDevicesIO].SetAggrStat(containerName, 10)  // not used
	containerMetrics.CgroupStatMap[config.BlockDevicesIO].SetAggrStat(containerName, 20)  // not used
	// kubelet - cgroup package
	err = containerMetrics.KubeletStats[config.KubeletContainerCPU].SetNewAggr(10) // not used
	Expect(err).NotTo(HaveOccurred())
	err = containerMetrics.KubeletStats[config.KubeletContainerCPU].SetNewAggr(20) // not used
	Expect(err).NotTo(HaveOccurred())
	err = containerMetrics.KubeletStats[config.KubeletContainerMemory].SetNewAggr(10) // not used
	Expect(err).NotTo(HaveOccurred())
	err = containerMetrics.KubeletStats[config.KubeletContainerMemory].SetNewAggr(20) // not used
	Expect(err).NotTo(HaveOccurred())
	err = containerMetrics.KubeletStats[config.KubeletNodeCPU].SetNewAggr(10) // not used
	Expect(err).NotTo(HaveOccurred())
	err = containerMetrics.KubeletStats[config.KubeletNodeCPU].SetNewAggr(20) // not used
	Expect(err).NotTo(HaveOccurred())
	err = containerMetrics.KubeletStats[config.KubeletNodeMemory].SetNewAggr(10) // not used
	Expect(err).NotTo(HaveOccurred())
	err = containerMetrics.KubeletStats[config.KubeletNodeMemory].SetNewAggr(20) // not used
	Expect(err).NotTo(HaveOccurred())
	return containerMetrics
}

func createMockNodeMetrics(containersMetrics map[string]*collector_metric.ContainerMetrics) collector_metric.NodeMetrics {
	nodeMetrics := *collector_metric.NewNodeMetrics()
	nodeMetrics.AddNodeResUsageFromContainerResUsage(containersMetrics)

	return nodeMetrics
}

var _ = Describe("ContainerPower", func() {
	var (
		containersMetrics map[string]*collector_metric.ContainerMetrics
		nodeMetrics       collector_metric.NodeMetrics

		machineSensorID = "sensor0"
		machineSocketID = 0

		systemFeatures = []string{"cpu_architecture"}
		systemValues   = []string{"Sandy Bridge"}
	)

	Context("with edge case", func() {
		BeforeEach(func() {
			source.SystemCollectionSupported = false // disable the system power collection to use the prediction power model
			setCollectorMetrics()
			containersMetrics = createMockContainersMetrics()
			nodeMetrics = createMockNodeMetrics(containersMetrics)

			defaultAbsCompURL = "https://raw.githubusercontent.com/sustainable-computing-io/kepler-model-server/main/tests/test_models/AbsComponentModelWeight/Full/KerasCompWeightFullPipeline/KerasCompWeightFullPipeline.json"
			defaultDynCompURL = "https://raw.githubusercontent.com/sustainable-computing-io/kepler-model-server/main/tests/test_models/DynComponentModelWeight/CgroupOnly/ScikitMixed/ScikitMixed.json"
		})

		// Currently, the model server test models only have data for the DynComponentModelWeight. We cannot get weights for the AbsModelWeight, AbsComponentModelWeight and DynModelWeight
		// Therefore, we can only test this the DynComponentModelWeight component
		// TODO: the make the usage of this different models more transparent, it is currently very hard to know what is going on...
		It("should return container with energy metric ", func() {
			// getEstimatorMetrics
			InitEstimateFunctions(usageMetrics, systemFeatures, systemValues)
			Expect(ContainerComponentPowerModelValid).To(Equal(true))

			// update container and node metrics
			componentsEnergies := make(map[int]source.NodeComponentsEnergy)
			// the NodeComponentsEnergy is the aggregated energy consumption of the node components
			// then, the components energy consumption is added to the in the nodeMetrics as Agg data
			// this means that, to have a Curr value, we must have at least two Agg data (to have Agg diff)
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

			nodePlatformEnergy := map[string]float64{}
			nodePlatformEnergy[machineSensorID] = 0 // empty
			nodeMetrics.SetLastestPlatformEnergy(nodePlatformEnergy, true)

			// calculate container energy consumption
			UpdateContainerEnergy(containersMetrics, &nodeMetrics)

			// Unit test use is reported by default settings through LR model
			// and following will be reported so EnergyInPkg.Delta will be 9512
			Expect(containersMetrics["containerA"].DynEnergyInPkg.Delta).To(Equal(uint64(9512)))
		})
	})
})
