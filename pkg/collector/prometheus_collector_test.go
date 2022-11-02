package collector

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"encoding/json"
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
	"github.com/sustainable-computing-io/kepler/pkg/power/rapl/source"
)

const (
	nodeEnergyMetric             = "kepler_node_platform_joules_total"
	nodefreqMetric               = "kepler_node_cpu_scaling_frequency_hertz"
	nodePackageEnergyMetric      = "kepler_node_package_joules_total"
	containerCPUCoreEnergyMetric = "kepler_container_package_joules_total"

	SampleCurr       = 100
	SampleAggr       = 1000
	sampleNodeEnergy = 20000 // mJ
	samplePkgEnergy  = 1000  // mJ
	SampleFreq       = 100000
)

func convertPromMetricToMap(body []byte, metric string) map[string]string {
	regStr := fmt.Sprintf(`%s{[^{}]*}`, metric)
	r := regexp.MustCompile(regStr)
	match := r.FindString(string(body))
	match = strings.Replace(match, metric, "", 1)
	match = strings.ReplaceAll(match, "=", `"=`)
	match = strings.ReplaceAll(match, ",", `,"`)
	match = strings.ReplaceAll(match, "{", `{"`)
	match = strings.ReplaceAll(match, "=", `:`)
	var response map[string]string
	if err := json.Unmarshal([]byte(match), &response); err != nil {
		fmt.Println(err)
	}
	return response
}

func convertPromToValue(body []byte, metric string) (int, error) {
	regStr := fmt.Sprintf(`%s{[^{}]*} [0-9]+`, metric)
	r := regexp.MustCompile(regStr)
	match := r.FindString(string(body))
	splits := strings.Split(match, " ")
	fmt.Println(splits, regStr)
	return strconv.Atoi(splits[1])
}

func newMockPrometheusExporter() *PrometheusCollector {
	exporter := NewPrometheusExporter()
	exporter.NodePkgEnergy = &map[int]source.RAPLEnergy{}
	exporter.NodeCPUFrequency = &map[int32]uint64{}
	exporter.NodeMetrics = collector_metric.NewNodeMetrics()
	exporter.ContainersMetrics = &map[string]*collector_metric.ContainerMetrics{}
	exporter.SamplePeriodSec = 3.0
	return exporter
}

var _ = Describe("Test Collector Unit", func() {
	It("Init and Run", func() {
		exporter := newMockPrometheusExporter()
		err := prometheus.Register(exporter)
		Expect(err).NotTo(HaveOccurred())

		// check if prometheus is replying
		req, _ := http.NewRequest("GET", "", http.NoBody)
		res := httptest.NewRecorder()
		handler := promhttp.Handler()
		handler.ServeHTTP(res, req)
		body, _ := io.ReadAll(res.Body)
		Expect(len(body)).Should(BeNumerically(">", 0))

		// add container mock values
		containerA := collector_metric.NewContainerMetrics("containerA", "podA", "default")
		err = containerA.EnergyInPkg.AddNewCurr(SampleCurr * 1000)
		Expect(err).NotTo(HaveOccurred())

		// add node mock values
		(*exporter.ContainersMetrics)["containerA"] = containerA
		(*exporter.NodeCPUFrequency)[0] = SampleFreq
		(*exporter.NodePkgEnergy)[0] = source.RAPLEnergy{Pkg: samplePkgEnergy}
		nodeSensorEnergy := map[string]float64{
			"sensor0": sampleNodeEnergy,
		}
		gpuEnergy := []uint32{uint32(sampleNodeEnergy)}
		exporter.NodeMetrics.SetValues(nodeSensorEnergy, *exporter.NodePkgEnergy, gpuEnergy, [][]float64{})

		// get metrics from prometheus
		res = httptest.NewRecorder()
		handler = promhttp.Handler()
		handler.ServeHTTP(res, req)
		body, _ = io.ReadAll(res.Body)
		Expect(len(body)).Should(BeNumerically(">", 0))
		fmt.Printf("Result:\n %s\n", body)

		// check sample node energy
		val, err := convertPromToValue(body, nodeEnergyMetric)
		Expect(err).NotTo(HaveOccurred())
		Expect(val).To(Equal(int(sampleNodeEnergy / 1000))) //J

		// check pkg energy
		val, err = convertPromToValue(body, nodePackageEnergyMetric)
		Expect(err).NotTo(HaveOccurred())
		Expect(val).Should(BeEquivalentTo(int(samplePkgEnergy / 1000))) // J

		// check sample frequency
		val, err = convertPromToValue(body, nodefreqMetric)
		Expect(err).NotTo(HaveOccurred())
		Expect(val).To(Equal(int(SampleFreq)))

		// check sample pod
		val, err = convertPromToValue(body, containerCPUCoreEnergyMetric)
		Expect(err).NotTo(HaveOccurred())
		Expect(val).To(Equal(int(SampleCurr)))
	})
})
