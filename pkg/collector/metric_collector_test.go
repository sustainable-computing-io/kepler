package collector

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sustainable-computing-io/kepler/pkg/collector/stats"
	"github.com/sustainable-computing-io/kepler/pkg/model"

	"github.com/sustainable-computing-io/kepler/pkg/bpfassets/attacher"
	"github.com/sustainable-computing-io/kepler/pkg/cgroup"
	"github.com/sustainable-computing-io/kepler/pkg/config"
	"github.com/sustainable-computing-io/kepler/pkg/sensors/accelerator/gpu"
	"github.com/sustainable-computing-io/kepler/pkg/sensors/components"
	"github.com/sustainable-computing-io/kepler/pkg/sensors/platform"
)

func newMockCollector() *Collector {
	if gpu.IsGPUCollectionSupported() {
		err := gpu.Init() // create structure instances that will be accessed to create a containerMetric
		Expect(err).NotTo(HaveOccurred())
	}
	// we need to disable the system real time power metrics for testing since we add mock values or use power model estimator
	components.SetIsSystemCollectionSupported(false)
	platform.SetIsSystemCollectionSupported(false)
	gpu.SetGPUCollectionSupported(false)

	cgroup.AddContainerIDToCache(0, "container1")
	cgroup.AddContainerIDToCache(1, "container2")
	stats.SetMockedCollectorMetrics()

	metricCollector := NewCollector()
	metricCollector.ProcessStats = stats.CreateMockedProcessStats(2)
	metricCollector.NodeStats = stats.CreateMockedNodeStats()
	// aggregate processes' resource utilization metrics to containers, virtual machines and nodes
	metricCollector.AggregateProcessResourceUtilizationMetrics()

	return metricCollector
}

var _ = Describe("Test Collector Unit", func() {

	It("Get container power", func() {
		attacher.HardwareCountersEnabled = false
		metricCollector := newMockCollector()
		// The default estimator model is the ratio
		model.CreatePowerEstimatorModels(stats.ProcessFeaturesNames, stats.NodeMetadataFeatureNames, stats.NodeMetadataFeatureValues)
		// update container and node metrics
		metricCollector.UpdateProcessEnergyUtilizationMetrics()
		metricCollector.AggregateProcessEnergyUtilizationMetrics()
		dynEnergyInPkg := metricCollector.ContainerStats["container1"].EnergyUsage[config.DynEnergyInPkg].SumAllDeltaValues()
		// The node components dynamic power were set to 35000mJ, since the kepler interval is 3s, the power is 11667mJ
		// The test created 2 processes with 30000 CPU Instructions
		// So the node total CPU Instructions is 60000
		// The process power will be (30000/60000)*11667 = 5834
		// Then, the process energy will be 5834*3 = 17502 mJ
		Expect(dynEnergyInPkg).Should(Equal(uint64(17502)))
	})

	It("HandleInactiveContainers without error", func() {
		metricCollector := newMockCollector()
		foundContainer := make(map[string]bool)
		foundContainer["container1"] = true
		foundContainer["container2"] = true
		metricCollector.handleInactiveContainers(foundContainer)
		Expect(len(metricCollector.ContainerStats)).Should(Equal(2))
	})

})
