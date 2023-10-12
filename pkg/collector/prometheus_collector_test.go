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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	collector_metric "github.com/sustainable-computing-io/kepler/pkg/collector/metric"
	"github.com/sustainable-computing-io/kepler/pkg/collector/metric/types"
	"github.com/sustainable-computing-io/kepler/pkg/config"
	"github.com/sustainable-computing-io/kepler/pkg/model"
	"github.com/sustainable-computing-io/kepler/pkg/power/accelerator/gpu"
	"github.com/sustainable-computing-io/kepler/pkg/power/components"
	"github.com/sustainable-computing-io/kepler/pkg/power/components/source"
	"github.com/sustainable-computing-io/kepler/pkg/power/platform"
)

const (
	nodeEnergyMetric             = "kepler_node_platform_joules_total"
	nodePackageEnergyMetric      = "kepler_node_package_joules_total"
	containerCPUCoreEnergyMetric = "kepler_container_package_joules_total"

	SampleCurr = 100
	SampleAggr = 1000
)

func convertPromToValue(body []byte, metric string) (float64, error) {
	regStr := fmt.Sprintf(`%s{[^{}]*}.*`, metric)
	r := regexp.MustCompile(regStr)
	match := r.FindString(string(body))
	splits := strings.Split(match, " ")
	fmt.Fprintf(GinkgoWriter, "splits=%v regStr=%v\n", splits, regStr)
	return strconv.ParseFloat(splits[1], 64)
}

func newMockPrometheusExporter() *PrometheusCollector {
	if gpu.IsGPUCollectionSupported() {
		err := gpu.Init() // create structure instances that will be accessed to create a containerMetric
		Expect(err).NotTo(HaveOccurred())
	}
	exporter := NewPrometheusExporter()
	exporter.NodeMetrics = collector_metric.NewNodeMetrics()
	exporter.ContainersMetrics = &map[string]*collector_metric.ContainerMetrics{}
	exporter.ProcessMetrics = &map[uint64]*collector_metric.ProcessMetrics{}
	exporter.VMMetrics = &map[uint64]*collector_metric.VMMetrics{}
	exporter.SamplePeriodSec = 3.0
	collector_metric.ContainerFeaturesNames = []string{config.CoreUsageMetric}
	collector_metric.NodeMetadataFeatureNames = []string{"cpu_architecture"}
	collector_metric.NodeMetadataFeatureValues = []string{"Sandy Bridge"}
	return exporter
}

var _ = Describe("Test Prometheus Collector Unit", func() {
	It("Init and Run", func() {
		exporter := newMockPrometheusExporter()
		// we need to disable the system real time power metrics for testing since we add mock values or use power model estimator
		components.SetIsSystemCollectionSupported(false)
		platform.SetIsSystemCollectionSupported(false)

		// add container mock values
		(*exporter.ContainersMetrics)["containerA"] = collector_metric.NewContainerMetrics("containerA", "podA", "test", "containerA")
		(*exporter.ContainersMetrics)["containerA"].BPFStats[config.CoreUsageMetric] = &types.UInt64Stat{}
		err := (*exporter.ContainersMetrics)["containerA"].BPFStats[config.CoreUsageMetric].AddNewDelta(30000)
		Expect(err).NotTo(HaveOccurred())
		(*exporter.ContainersMetrics)["containerB"] = collector_metric.NewContainerMetrics("containerB", "podB", "test", "containerB")
		(*exporter.ContainersMetrics)["containerB"].BPFStats[config.CoreUsageMetric] = &types.UInt64Stat{}
		err = (*exporter.ContainersMetrics)["containerB"].BPFStats[config.CoreUsageMetric].AddNewDelta(30000)
		Expect(err).NotTo(HaveOccurred())
		exporter.NodeMetrics.AddNodeResUsageFromContainerResUsage(*exporter.ContainersMetrics)

		// add node mock values
		// initialize the node energy with aggregated energy, which will be used to calculate delta energy
		nodePlatformEnergy := map[string]float64{}
		// initialize the node energy with aggregated energy, which will be used to calculate delta energy
		nodePlatformEnergy["sensor0"] = 5000 // mJ
		exporter.NodeMetrics.SetNodePlatformEnergy(nodePlatformEnergy, true, false)
		exporter.NodeMetrics.UpdateIdleEnergyWithMinValue(true)
		// the second node energy will represent the idle and dynamic power. The idle power is only calculated after there at at least two delta values
		nodePlatformEnergy["sensor0"] = 35000
		exporter.NodeMetrics.SetNodePlatformEnergy(nodePlatformEnergy, true, false)
		exporter.NodeMetrics.UpdateDynEnergy()

		// initialize the node energy with aggregated energy, which will be used to calculate delta energy
		// note that NodeComponentsEnergy contains aggregated energy over time
		componentsEnergies := make(map[int]source.NodeComponentsEnergy)
		componentsEnergies[0] = source.NodeComponentsEnergy{
			Pkg:    5000, // mJ
			Core:   5000,
			DRAM:   5000,
			Uncore: 5000,
		}
		exporter.NodeMetrics.SetNodeComponentsEnergy(componentsEnergies, false, false)
		componentsEnergies[0] = source.NodeComponentsEnergy{
			Pkg:    10000, // mJ
			Core:   10000,
			DRAM:   10000,
			Uncore: 10000,
		}
		// the second node energy will force to calculate a delta. The delta is calculates after added at least two aggregated metric
		exporter.NodeMetrics.SetNodeComponentsEnergy(componentsEnergies, false, false)
		exporter.NodeMetrics.UpdateIdleEnergyWithMinValue(true)
		// the third node energy will represent the idle and dynamic power. The idle power is only calculated after there at at least two delta values
		componentsEnergies[0] = source.NodeComponentsEnergy{
			Pkg:    45000, // 35000mJ delta, which is 5000mJ idle, 30000mJ dynamic power
			Core:   45000,
			DRAM:   45000,
			Uncore: 45000,
		}
		exporter.NodeMetrics.SetNodeComponentsEnergy(componentsEnergies, false, false)
		exporter.NodeMetrics.UpdateDynEnergy()

		model.CreatePowerEstimatorModels(collector_metric.ContainerFeaturesNames, collector_metric.NodeMetadataFeatureNames, collector_metric.NodeMetadataFeatureValues)
		model.UpdateContainerEnergy((*exporter.ContainersMetrics), exporter.NodeMetrics)

		// get metrics from prometheus
		err = prometheus.Register(exporter)
		Expect(err).NotTo(HaveOccurred())

		// check if prometheus is replying
		req, _ := http.NewRequest("GET", "", http.NoBody)
		res := httptest.NewRecorder()
		handler := promhttp.Handler()
		handler.ServeHTTP(res, req)
		body, _ := io.ReadAll(res.Body)
		Expect(len(body)).Should(BeNumerically(">", 0))
		fmt.Fprintf(GinkgoWriter, "Result:\n %s\n", body)

		// check pkg energy
		val, err := convertPromToValue(body, nodePackageEnergyMetric)
		Expect(err).NotTo(HaveOccurred())
		Expect(val).Should(BeEquivalentTo(30)) // J

		// check sample node energy
		val, err = convertPromToValue(body, nodeEnergyMetric)
		Expect(err).NotTo(HaveOccurred())
		Expect(val).To(Equal(float64(60))) // J

		// check sample pod
		val, err = convertPromToValue(body, containerCPUCoreEnergyMetric)
		Expect(err).NotTo(HaveOccurred())
		// The pkg dynamic energy is 30J, the container cpu usage is 50%, so the dynamic energy is 15J
		Expect(val).To(Equal(float64(15))) // J
	})
})
