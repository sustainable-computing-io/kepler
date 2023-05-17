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
	"sync"

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
	"github.com/sustainable-computing-io/kepler/pkg/power/accelerator"
	"github.com/sustainable-computing-io/kepler/pkg/power/components/source"

	"github.com/sustainable-computing-io/kepler/pkg/model/estimator/local"
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
	if accelerator.IsGPUCollectionSupported() {
		err := accelerator.Init() // create structure instances that will be accessed to create a containerMetric
		Expect(err).NotTo(HaveOccurred())
	}
	exporter := NewPrometheusExporter()
	exporter.NodeMetrics = collector_metric.NewNodeMetrics()
	exporter.ContainersMetrics = &map[string]*collector_metric.ContainerMetrics{}
	exporter.ProcessMetrics = &map[uint64]*collector_metric.ProcessMetrics{}
	exporter.SamplePeriodSec = 3.0
	collector_metric.ContainerMetricNames = []string{config.CoreUsageMetric}
	return exporter
}

var _ = Describe("Test Prometheus Collector Unit", func() {
	It("Init and Run", func() {
		exporter := newMockPrometheusExporter()

		// add container mock values
		(*exporter.ContainersMetrics)["containerA"] = collector_metric.NewContainerMetrics("containerA", "podA", "test", "containerA")
		(*exporter.ContainersMetrics)["containerA"].CounterStats[config.CoreUsageMetric] = &types.UInt64Stat{}
		err := (*exporter.ContainersMetrics)["containerA"].CounterStats[config.CoreUsageMetric].AddNewDelta(100)
		Expect(err).NotTo(HaveOccurred())
		(*exporter.ContainersMetrics)["containerB"] = collector_metric.NewContainerMetrics("containerB", "podB", "test", "containerB")
		(*exporter.ContainersMetrics)["containerB"].CounterStats[config.CoreUsageMetric] = &types.UInt64Stat{}
		err = (*exporter.ContainersMetrics)["containerB"].CounterStats[config.CoreUsageMetric].AddNewDelta(100)
		Expect(err).NotTo(HaveOccurred())
		exporter.NodeMetrics.AddNodeResUsageFromContainerResUsage(*exporter.ContainersMetrics)

		// add node mock values
		// initialize the node energy with aggregated energy, which will be used to calculate delta energy
		nodePlatformEnergy := map[string]float64{}
		// initialize the node energy with aggregated energy, which will be used to calculate delta energy
		nodePlatformEnergy["sensor0"] = 5
		exporter.NodeMetrics.SetLastestPlatformEnergy(nodePlatformEnergy, true)
		exporter.NodeMetrics.UpdateIdleEnergy()
		// the second node energy will represent the idle and dynamic power
		nodePlatformEnergy["sensor0"] = 10 // 5J idle, 5J dynamic power
		exporter.NodeMetrics.SetLastestPlatformEnergy(nodePlatformEnergy, true)
		exporter.NodeMetrics.UpdateIdleEnergy()
		exporter.NodeMetrics.UpdateDynEnergy()

		// initialize the node energy with aggregated energy, which will be used to calculate delta energy
		// note that NodeComponentsEnergy contains aggregated energy over time
		componentsEnergies := make(map[int]source.NodeComponentsEnergy)
		componentsEnergies[0] = source.NodeComponentsEnergy{
			Pkg:    5,
			Core:   5,
			DRAM:   5,
			Uncore: 5,
		}
		exporter.NodeMetrics.SetNodeComponentsEnergy(componentsEnergies, false)
		componentsEnergies[0] = source.NodeComponentsEnergy{
			Pkg:    10,
			Core:   10,
			DRAM:   10,
			Uncore: 10,
		}
		// the second node energy will force to calculate a delta. The delta is calculates after added at least two aggregated metric
		exporter.NodeMetrics.SetNodeComponentsEnergy(componentsEnergies, false)
		exporter.NodeMetrics.UpdateIdleEnergy()
		// the third node energy will represent the idle and dynamic power. The idle power is only calculated after there at at least two delta values
		componentsEnergies[0] = source.NodeComponentsEnergy{
			Pkg:    20, // 10J delta, which is 5J idle, 5J dynamic power
			Core:   20, // 10J delta, which is 5J idle, 5J dynamic power
			DRAM:   20, // 10J delta, which is 5J idle, 5J dynamic power
			Uncore: 20, // 10J delta, which is 5J idle, 5J dynamic power
		}
		exporter.NodeMetrics.SetNodeComponentsEnergy(componentsEnergies, false)
		exporter.NodeMetrics.UpdateIdleEnergy()
		exporter.NodeMetrics.UpdateDynEnergy()
		var wg sync.WaitGroup
		wg.Add(1)
		go local.UpdateContainerComponentEnergyByRatioPowerModel(*exporter.ContainersMetrics, exporter.NodeMetrics, collector_metric.PKG, config.CoreUsageMetric, &wg)
		wg.Wait()

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
		fmt.Printf("Result:\n %s\n", body)

		// check pkg energy
		val, err := convertPromToValue(body, nodePackageEnergyMetric)
		Expect(err).NotTo(HaveOccurred())
		Expect(val).Should(BeEquivalentTo(0.005)) // J

		// check sample node energy
		val, err = convertPromToValue(body, nodeEnergyMetric)
		Expect(err).NotTo(HaveOccurred())
		Expect(val).To(Equal(0.01)) //J

		// check sample pod
		val, err = convertPromToValue(body, containerCPUCoreEnergyMetric)
		Expect(err).NotTo(HaveOccurred())
		// The pkg dynamic energy is 5mJ, the container cpu usage is 50%, so the dynamic energy is 2.5mJ = ~3mJ
		Expect(val).To(Equal(0.003)) //J
	})
})
