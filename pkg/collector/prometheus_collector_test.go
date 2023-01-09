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
	"github.com/sustainable-computing-io/kepler/pkg/power/accelerator"
	"github.com/sustainable-computing-io/kepler/pkg/power/components/source"
)

const (
	nodeEnergyMetric             = "kepler_node_platform_joules_total"
	nodePackageEnergyMetric      = "kepler_node_package_joules_total"
	containerCPUCoreEnergyMetric = "kepler_container_package_joules_total"

	SampleCurr       = 100
	SampleAggr       = 1000
	sampleNodeEnergy = 20000 // mJ
	samplePkgEnergy  = 1000  // mJ
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
	if accelerator.IsGPUCollectionSupported() {
		err := accelerator.Init() // create structure instances that will be accessed to create a containerMetric
		Expect(err).NotTo(HaveOccurred())
	}
	exporter := NewPrometheusExporter()
	exporter.NodeCPUFrequency = &map[int32]uint64{}
	exporter.NodeMetrics = collector_metric.NewNodeMetrics()
	exporter.ContainersMetrics = &map[string]*collector_metric.ContainerMetrics{}
	exporter.SamplePeriodSec = 3.0
	return exporter
}

var _ = Describe("Test Prometheus Collector Unit", func() {
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
		(*exporter.ContainersMetrics)["containerA"] = collector_metric.NewContainerMetrics("containerA", "podA", "test")
		err = (*exporter.ContainersMetrics)["containerA"].EnergyInPkg.AddNewCurr(SampleCurr * 1000)
		Expect(err).NotTo(HaveOccurred())

		// add node mock values
		componentsEnergies := make(map[int]source.NodeComponentsEnergy)
		componentsEnergies[0] = source.NodeComponentsEnergy{
			Pkg: samplePkgEnergy,
		}
		exporter.NodeMetrics.AddNodeComponentsEnergy(componentsEnergies)
		nodePlatformEnergy := map[string]float64{}
		nodePlatformEnergy["sensor0"] = sampleNodeEnergy
		exporter.NodeMetrics.AddLastestPlatformEnergy(nodePlatformEnergy) // must be higher than components energy

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

		// check sample pod
		val, err = convertPromToValue(body, containerCPUCoreEnergyMetric)
		Expect(err).NotTo(HaveOccurred())
		Expect(val).To(Equal(int(SampleCurr)))
	})
})
