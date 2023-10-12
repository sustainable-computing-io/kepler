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
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	collector_metric "github.com/sustainable-computing-io/kepler/pkg/collector/metric"
	"github.com/sustainable-computing-io/kepler/pkg/config"
	"github.com/sustainable-computing-io/kepler/pkg/power/accelerator/gpu"
	"github.com/sustainable-computing-io/kepler/pkg/power/components"
	"github.com/sustainable-computing-io/kepler/pkg/power/components/source"
	"github.com/sustainable-computing-io/kepler/pkg/power/platform"
)

// we need to add all metric to a container, otherwise it will not create the right usageMetric with all elements. The usageMetric is used in the Prediction Power Models
// TODO: do not use a fixed usageMetric array in the power models, a structured data is more disarable.
func setCollectorMetrics() {
	if gpu.IsGPUCollectionSupported() {
		err := gpu.Init() // create structure instances that will be accessed to create a containerMetric
		Expect(err).NotTo(HaveOccurred())
	}
	// initialize the Available metrics since they are used to create a new containersMetrics instance
	collector_metric.AvailableBPFHWCounters = []string{
		config.CPUCycle,
		config.CPUInstruction,
		config.CacheMiss,
	}
	collector_metric.AvailableBPFSWCounters = []string{
		config.CPUTime,
		config.PageCacheHit,
	}
	collector_metric.AvailableCGroupMetrics = []string{
		config.CgroupfsMemory,
		config.CgroupfsKernelMemory,
		config.CgroupfsTCPMemory,
		config.CgroupfsCPU,
		config.CgroupfsSystemCPU,
		config.CgroupfsUserCPU,
		config.CgroupfsReadIO,
		config.CgroupfsWriteIO,
		config.BlockDevicesIO,
	}
	collector_metric.AvailableKubeletMetrics = []string{
		config.KubeletCPUUsage,
		config.KubeletMemoryUsage,
	}
	collector_metric.ContainerUintFeaturesNames = []string{}
	collector_metric.ContainerUintFeaturesNames = append(collector_metric.ContainerUintFeaturesNames, collector_metric.AvailableBPFSWCounters...)
	collector_metric.ContainerUintFeaturesNames = append(collector_metric.ContainerUintFeaturesNames, collector_metric.AvailableBPFHWCounters...)
	collector_metric.ContainerUintFeaturesNames = append(collector_metric.ContainerUintFeaturesNames, collector_metric.AvailableCGroupMetrics...)
	collector_metric.ContainerUintFeaturesNames = append(collector_metric.ContainerUintFeaturesNames, collector_metric.AvailableKubeletMetrics...)
	// ContainerFeaturesNames is used by the nodeMetrics to extract the resource usage. Only the metrics in ContainerFeaturesNames will be used.
	collector_metric.ContainerFeaturesNames = collector_metric.ContainerUintFeaturesNames
	collector_metric.CPUHardwareCounterEnabled = true
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
	err := containerMetrics.BPFStats[config.CPUCycle].AddNewDelta(30000)
	Expect(err).NotTo(HaveOccurred())
	err = containerMetrics.BPFStats[config.CPUInstruction].AddNewDelta(30000)
	Expect(err).NotTo(HaveOccurred())
	err = containerMetrics.BPFStats[config.CacheMiss].AddNewDelta(30000)
	Expect(err).NotTo(HaveOccurred())
	// bpf - cpu time
	err = containerMetrics.BPFStats[config.CPUTime].AddNewDelta(30000) // config.CPUTime
	Expect(err).NotTo(HaveOccurred())
	// cgroup - cgroup package
	// we need to add two aggregated values to the stats so that it can calculate a current value (i.e. agg diff)
	// cgroup memory is expressed in bytes, to make it meaninful we set 1MB
	// since kepler collects metrics for every 3s, the metrics will be normalized by 3 to estimate power, because of that to make the test easier we create delta  as multiple of 3
	containerMetrics.CgroupStatMap[config.CgroupfsMemory].SetAggrStat(containerName, 1000000)
	containerMetrics.CgroupStatMap[config.CgroupfsMemory].SetAggrStat(containerName, 4000000) // delta is 30MB
	// cgroup kernel memory is expressed in bytes, to make it meaninful we set 100KB
	containerMetrics.CgroupStatMap[config.CgroupfsKernelMemory].SetAggrStat(containerName, 100000)
	containerMetrics.CgroupStatMap[config.CgroupfsKernelMemory].SetAggrStat(containerName, 400000) // delta is 30KB
	containerMetrics.CgroupStatMap[config.CgroupfsTCPMemory].SetAggrStat(containerName, 100000)
	containerMetrics.CgroupStatMap[config.CgroupfsTCPMemory].SetAggrStat(containerName, 400000)
	// cgroup cpu time is expressed in microseconds, to make it meaninful we set it to 10ms
	containerMetrics.CgroupStatMap[config.CgroupfsCPU].SetAggrStat(containerName, 10000)
	containerMetrics.CgroupStatMap[config.CgroupfsCPU].SetAggrStat(containerName, 40000) // delta is 30ms
	// cgroup cpu time is expressed in microseconds, to make it meaninful we set it to 1ms
	containerMetrics.CgroupStatMap[config.CgroupfsSystemCPU].SetAggrStat(containerName, 1000)
	containerMetrics.CgroupStatMap[config.CgroupfsSystemCPU].SetAggrStat(containerName, 4000)
	containerMetrics.CgroupStatMap[config.CgroupfsUserCPU].SetAggrStat(containerName, 1000)
	containerMetrics.CgroupStatMap[config.CgroupfsUserCPU].SetAggrStat(containerName, 4000)
	// cgroup read and write IO are expressed in bytes, to make it meaninful we set 10KB
	containerMetrics.CgroupStatMap[config.CgroupfsReadIO].SetAggrStat(containerName, 10000)
	containerMetrics.CgroupStatMap[config.CgroupfsReadIO].SetAggrStat(containerName, 40000)
	containerMetrics.CgroupStatMap[config.CgroupfsWriteIO].SetAggrStat(containerName, 10000)
	containerMetrics.CgroupStatMap[config.CgroupfsWriteIO].SetAggrStat(containerName, 40000)
	containerMetrics.CgroupStatMap[config.BlockDevicesIO].SetAggrStat(containerName, 1)
	containerMetrics.CgroupStatMap[config.BlockDevicesIO].SetAggrStat(containerName, 1)
	// kubelet - cgroup package
	err = containerMetrics.KubeletStats[config.KubeletCPUUsage].SetNewAggr(10000)
	Expect(err).NotTo(HaveOccurred())
	err = containerMetrics.KubeletStats[config.KubeletCPUUsage].SetNewAggr(40000)
	Expect(err).NotTo(HaveOccurred())
	err = containerMetrics.KubeletStats[config.KubeletMemoryUsage].SetNewAggr(1000000)
	Expect(err).NotTo(HaveOccurred())
	err = containerMetrics.KubeletStats[config.KubeletMemoryUsage].SetNewAggr(4000000)
	Expect(err).NotTo(HaveOccurred())
	return containerMetrics
}

func createMockNodeMetrics(containersMetrics map[string]*collector_metric.ContainerMetrics) *collector_metric.NodeMetrics {
	nodeMetrics := collector_metric.NewNodeMetrics()
	nodeMetrics.AddNodeResUsageFromContainerResUsage(containersMetrics)

	return nodeMetrics
}

var _ = Describe("ContainerPower", func() {
	var (
		containersMetrics map[string]*collector_metric.ContainerMetrics
		nodeMetrics       *collector_metric.NodeMetrics

		machineSensorID = "sensor0"
		machineSocketID = 0

		systemMetaDataFeatureNames  = []string{"cpu_architecture"}
		systemMetaDataFeatureValues = []string{"Sandy Bridge"}
	)

	Context("with manually defined node power", func() {
		BeforeEach(func() {
			// we need to disable the system real time power metrics for testing since we add mock values or use power model estimator
			components.SetIsSystemCollectionSupported(false)
			platform.SetIsSystemCollectionSupported(false)
			setCollectorMetrics()

			containersMetrics = createMockContainersMetrics()
			nodeMetrics = createMockNodeMetrics(containersMetrics)
		})

		// By default the Ratio power mode is used to get the container power
		It("Get container power with Ratio power model and node component power", func() {
			configStr := "CONTAINER_COMPONENTS_ESTIMATOR=false\n"
			os.Setenv("MODEL_CONFIG", configStr)

			// getEstimatorMetrics
			CreatePowerEstimatorModels(containerFeatureNames, systemMetaDataFeatureNames, systemMetaDataFeatureValues)

			// initialize the node energy with aggregated energy, which will be used to calculate delta energy
			// note that NodeComponentsEnergy contains aggregated energy over time
			componentsEnergies := make(map[int]source.NodeComponentsEnergy)
			componentsEnergies[machineSocketID] = source.NodeComponentsEnergy{
				Pkg:    5000, // mJ
				Core:   5000,
				DRAM:   5000,
				Uncore: 5000,
			}
			nodeMetrics.SetNodeComponentsEnergy(componentsEnergies, counter, absPower)
			componentsEnergies[machineSocketID] = source.NodeComponentsEnergy{
				Pkg:    10000, // mJ
				Core:   10000,
				DRAM:   10000,
				Uncore: 10000,
			}
			// the second node energy will force to calculate a delta. The delta is calculates after added at least two aggregated metric
			nodeMetrics.SetNodeComponentsEnergy(componentsEnergies, counter, absPower)
			nodeMetrics.UpdateIdleEnergyWithMinValue(true)
			// the third node energy will represent the idle and dynamic power. The idle power is only calculated after there at at least two delta values
			componentsEnergies[machineSocketID] = source.NodeComponentsEnergy{
				Pkg:    45000, // 35000mJ delta, which is 5000mJ idle, 30000mJ dynamic power
				Core:   45000,
				DRAM:   45000,
				Uncore: 45000,
			}
			nodeMetrics.SetNodeComponentsEnergy(componentsEnergies, counter, absPower)
			nodeMetrics.UpdateDynEnergy()

			// calculate container energy consumption
			UpdateContainerEnergy(containersMetrics, nodeMetrics)

			// The default container power model is the Ratio, then container energy consumption will be as follows:
			// The node components dynamic power were set to 30000mJ, since the kepler interval is 3s, the power is 10000mJ
			// The test created 2 containers with 30000 CPU Instructions, since the kepler interval is 3s, the normalized value is 10000
			// So the node total CPU Instructions is 20000
			// The container power will be (10000/20000)*10000 = 5000
			// Then, the container energy will be 5000*3 = 15000 mJ
			Expect(containersMetrics["containerA"].DynEnergyInPkg.Delta).To(Equal(uint64(15000)))
		})

		It("Get container power with Ratio power model and node platform power ", func() {
			configStr := "CONTAINER_COMPONENTS_ESTIMATOR=false\n"
			os.Setenv("MODEL_CONFIG", configStr)

			// getEstimatorMetrics
			CreatePowerEstimatorModels(containerFeatureNames, systemMetaDataFeatureNames, systemMetaDataFeatureValues)

			// manually add the node power  metrics
			// initialize the node energy with aggregated energy, which will be used to calculate delta energy
			nodePlatformEnergy := map[string]float64{}
			// initialize the node energy with aggregated energy, which will be used to calculate delta energy
			nodePlatformEnergy[machineSensorID] = 5000 // mJ
			nodeMetrics.SetNodePlatformEnergy(nodePlatformEnergy, gauge, absPower)
			nodeMetrics.UpdateIdleEnergyWithMinValue(true)
			// the second node energy will represent the idle and dynamic power. The idle power is only calculated after there at at least two delta values
			nodePlatformEnergy[machineSensorID] = 35000
			nodeMetrics.SetNodePlatformEnergy(nodePlatformEnergy, gauge, absPower)
			nodeMetrics.UpdateDynEnergy()

			// calculate container energy consumption
			UpdateContainerEnergy(containersMetrics, nodeMetrics)

			// The default container power model is the Ratio, then container energy consumption will be as follows:
			// The node components dynamic power were set to 30000mJ, since the kepler interval is 3s, the power is 10000mJ
			// The test created 2 containers with 30000 CPU Instructions, since the kepler interval is 3s, the normalized value is 10000
			// So the node total CPU Instructions is 20000
			// The container power will be (10000/20000)*10000 = 5000
			// Then, the container energy will be 5000*3 = 15000 mJ
			Expect(containersMetrics["containerA"].DynEnergyInPlatform.Delta).To(Equal(uint64(15000)))
		})

		// TODO: Get container power with no dependency and no node power.
		// The current LR model has some problems, all the model weights are negative, which means that the energy consumption will decrease with larger resource utilization.
		// Consequently the dynamic power will be 0 since the idle power with 0 resource utilization will be higher than the absolute power with non zero utilization
		// Because of that we removed this test until we have a power model that is more accurate.
		// It("Get container power with Ratio power model and node power with Linear Regression Power Model", func() {
		// 	// the default power models will be linear regression for node power estimation and ratio for container power estimation
		// 	CreatePowerEstimatorModels(containerFeatureNames, systemMetaDataFeatureNames, systemMetaDataFeatureValues)
		// 	// add node energy via estimator, using the LR model (the default configured one)
		// 	// UpdateNodePlatformEnergy(nodeMetrics) currently we do not have the node platform abs power model, we will include that in the future
		// 	UpdateNodeComponentEnergy(nodeMetrics)
		// 	// calculate container energy consumption
		// 	UpdateContainerEnergy(containersMetrics, nodeMetrics)
		// 	Expect(containersMetrics["containerA"].DynEnergyInPkg.Delta).To(Equal(uint64(???)))
		// })
	})
})
