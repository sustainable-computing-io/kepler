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

package metrics

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
	"github.com/sustainable-computing-io/kepler/pkg/bpf"
	"github.com/sustainable-computing-io/kepler/pkg/collector"
	"github.com/sustainable-computing-io/kepler/pkg/collector/stats"
	"github.com/sustainable-computing-io/kepler/pkg/model"

	acc "github.com/sustainable-computing-io/kepler/pkg/sensors/accelerator"
	"github.com/sustainable-computing-io/kepler/pkg/sensors/components"
	"github.com/sustainable-computing-io/kepler/pkg/sensors/platform"
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

var _ = Describe("Test Prometheus Collector Unit", func() {
	It("Init and Run", func() {
		// we need to disable the system real time power metrics for testing since we add mock values or use power model estimator
		components.SetIsSystemCollectionSupported(false)
		platform.SetIsSystemCollectionSupported(false)
		if gpus, err := acc.GetActiveAcceleratorsByType("gpu"); err == nil {
			for _, a := range gpus {
				err := a.GetAccelerator().Init() // create structure instances that will be accessed to create a containerMetric
				Expect(err).NotTo(HaveOccurred())
			}
		}
		stats.SetMockedCollectorMetrics()
		processStats := stats.CreateMockedProcessStats(2)
		nodeStats := stats.CreateMockedNodeStats()

		bpfExporter := bpf.NewMockExporter(bpf.DefaultSupportedMetrics())
		metricCollector := collector.NewCollector(bpfExporter)
		metricCollector.ProcessStats = processStats
		metricCollector.NodeStats = nodeStats
		// aggregate processes' resource utilization metrics to containers, virtual machines and nodes
		metricCollector.AggregateProcessResourceUtilizationMetrics()

		// the collector and prometheusExporter share structures and collections
		bpfSupportedMetrics := bpfExporter.SupportedMetrics()
		exporter := NewPrometheusExporter(bpfSupportedMetrics)
		exporter.NewProcessCollector(metricCollector.ProcessStats)
		exporter.NewContainerCollector(metricCollector.ContainerStats)
		exporter.NewVMCollector(metricCollector.VMStats)
		exporter.NewNodeCollector(&metricCollector.NodeStats)

		nodeStats.UpdateDynEnergy()

		model.CreatePowerEstimatorModels(stats.GetProcessFeatureNames(bpfSupportedMetrics),
			stats.NodeMetadataFeatureNames,
			stats.NodeMetadataFeatureValues,
			bpfSupportedMetrics)
		model.UpdateProcessEnergy(processStats, &nodeStats)

		// get metrics from prometheus
		err := prometheus.Register(exporter.ProcessStatsCollector)
		Expect(err).NotTo(HaveOccurred())
		err = prometheus.Register(exporter.ContainerStatsCollector)
		Expect(err).NotTo(HaveOccurred())
		err = prometheus.Register(exporter.VMStatsCollector)
		Expect(err).NotTo(HaveOccurred())
		err = prometheus.Register(exporter.NodeStatsCollector)
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
