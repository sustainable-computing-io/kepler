package collector

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	collector_metric "github.com/sustainable-computing-io/kepler/pkg/collector/metric"
	"github.com/sustainable-computing-io/kepler/pkg/model"

	"github.com/sustainable-computing-io/kepler/pkg/bpfassets/attacher"
	"github.com/sustainable-computing-io/kepler/pkg/cgroup"
	"github.com/sustainable-computing-io/kepler/pkg/config"
	"github.com/sustainable-computing-io/kepler/pkg/power/accelerator/gpu"
	"github.com/sustainable-computing-io/kepler/pkg/power/components"
	"github.com/sustainable-computing-io/kepler/pkg/power/components/source"
	"github.com/sustainable-computing-io/kepler/pkg/power/platform"
)

// we need to add all metric to a container, otherwise it will not create the right usageMetric with all elements. The usageMetric is used in the Prediction Power Models
// TODO: do not use a fixed usageMetric array in the power models, a structured data is more disarable.
func setCollectorMetrics() {
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
	containerMetrics.CgroupStatMap[config.CgroupfsCPU].SetAggrStat(containerName, 40000)
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

func createMockNodeMetrics(containersMetrics map[string]*collector_metric.ContainerMetrics) collector_metric.NodeMetrics {
	nodeMetrics := *collector_metric.NewNodeMetrics()
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
	nodeMetrics.SetNodeComponentsEnergy(componentsEnergies, false, false)
	componentsEnergies[machineSocketID] = source.NodeComponentsEnergy{
		Pkg:  18,
		Core: 15,
		DRAM: 11,
	}
	nodeMetrics.SetNodeComponentsEnergy(componentsEnergies, false, false)

	return nodeMetrics
}

func newMockCollector() *Collector {
	metricCollector := NewCollector()
	metricCollector.ContainersMetrics = createMockContainersMetrics()
	metricCollector.ProcessMetrics = map[uint64]*collector_metric.ProcessMetrics{}
	metricCollector.VMMetrics = map[uint64]*collector_metric.VMMetrics{}
	metricCollector.NodeMetrics = createMockNodeMetrics(metricCollector.ContainersMetrics)

	return metricCollector
}

var _ = Describe("Test Collector Unit", func() {
	BeforeEach(func() {
		if gpu.IsGPUCollectionSupported() {
			err := gpu.Init() // create structure instances that will be accessed to create a containerMetric
			Expect(err).NotTo(HaveOccurred())
		}
		gpu.SetGPUCollectionSupported(true)
		cgroup.AddContainerIDToCache(0, "containerA")
		cgroup.AddContainerIDToCache(1, "containerB")
		setCollectorMetrics()
		// we need to disable the system real time power metrics for testing since we add mock values or use power model estimator
		components.SetIsSystemCollectionSupported(false)
		platform.SetIsSystemCollectionSupported(false)
	})

	It("Get container power", func() {
		attacher.HardwareCountersEnabled = false
		metricCollector := newMockCollector()
		// The default estimator model is the ratio
		model.CreatePowerEstimatorModels(collector_metric.ContainerFeaturesNames, collector_metric.NodeMetadataFeatureNames, collector_metric.NodeMetadataFeatureValues)
		// update container and node metrics
		metricCollector.updateGPUMetrics()
		metricCollector.updateNodeResourceUsage()
		metricCollector.updateNodeEnergyMetrics()
		metricCollector.updateContainerEnergy()
		Expect(metricCollector.ContainersMetrics["containerA"].DynEnergyInPkg.Delta).ShouldNot(BeNil())
	})

	It("HandleInactiveContainers without error", func() {
		metricCollector := newMockCollector()
		foundContainer := make(map[string]bool)
		foundContainer["containerA"] = true
		foundContainer["containerB"] = true
		metricCollector.handleInactiveContainers(foundContainer)
		Expect(len(metricCollector.ContainersMetrics)).Should(Equal(2))
	})

})
