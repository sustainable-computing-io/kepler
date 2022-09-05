package collector

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"encoding/json"
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sustainable-computing-io/kepler/pkg/power/rapl/source"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
)

const (
	CPU_USAGE_TOTAL_KEY = "cgroupfs_cpu_usage_us"
	SAMPLE_CURR         = 100
	SAMPLE_AGGR         = 1000
	SAMPLE_NODE_ENERGY  = 20000 //mJ
	SAMPLE_PKG_ENERGY   = 1000  //mJ
	SAMPLE_FREQ         = 100000
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
	json.Unmarshal([]byte(match), &response)
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

var _ = Describe("Test Collector Unit", func() {
	It("Init and Run", func() {
		newCollector, err := New()
		Expect(err).NotTo(HaveOccurred())
		err = prometheus.Register(newCollector)
		Expect(err).NotTo(HaveOccurred())
		req, _ := http.NewRequest("GET", "", nil)
		res := httptest.NewRecorder()
		handler := http.Handler(promhttp.Handler())
		handler.ServeHTTP(res, req)
		body, _ := ioutil.ReadAll(res.Body)
		Expect(len(body)).Should(BeNumerically(">", 0))

		regStr := fmt.Sprintf(`%s{[^{}]*}`, POD_ENERGY_STAT_METRIC)
		r := regexp.MustCompile(regStr)
		match := r.FindString(string(body))
		Expect(match).To(Equal(""))

		v := NewPodEnergy("podA", "default")
		v.EnergyInCore = &UInt64Stat{
			Curr: 10,
			Aggr: 10,
		}
		v.CgroupFSStats = map[string]*UInt64StatCollection{
			CPU_USAGE_TOTAL_KEY: &UInt64StatCollection{
				Stat: map[string]*UInt64Stat{
					"cA": &UInt64Stat{
						Curr: SAMPLE_CURR,
						Aggr: SAMPLE_AGGR,
					},
				},
			},
		}
		cpuFrequency = map[int32]uint64{
			0: SAMPLE_FREQ,
		}
		podEnergy = map[string]*PodEnergy{
			"a": v,
		}
		// initial
		sensorEnergy = map[string]float64{
			"sensor0": SAMPLE_NODE_ENERGY,
		}
		pkgEnergy = map[int]source.PackageEnergy{
			0: source.PackageEnergy{
				Pkg: SAMPLE_PKG_ENERGY,
			},
		}
		nodeEnergy.SetValues(sensorEnergy, pkgEnergy, 0, map[string]float64{})
		sensorEnergy = map[string]float64{
			"sensor0": SAMPLE_NODE_ENERGY * 2,
		}
		pkgEnergy = map[int]source.PackageEnergy{
			0: source.PackageEnergy{
				Pkg: SAMPLE_PKG_ENERGY * 2,
			},
		}
		nodeEnergy.SetValues(sensorEnergy, pkgEnergy, 0, map[string]float64{})

		res = httptest.NewRecorder()
		handler = http.Handler(promhttp.Handler())
		handler.ServeHTTP(res, req)
		body, _ = ioutil.ReadAll(res.Body)
		Expect(len(body)).Should(BeNumerically(">", 0))
		fmt.Printf("Result:\n %s\n", body)

		// check sample pod energy stat
		response := convertPromMetricToMap(body, POD_ENERGY_STAT_METRIC)
		if len(availableCgroupMetrics) > 0 {
			currSample, found := response["curr_"+CPU_USAGE_TOTAL_KEY]
			Expect(found).To(Equal(true))
			Expect(currSample).To(Equal(fmt.Sprintf("%d", SAMPLE_CURR)))
			aggrSample, found := response["total_"+CPU_USAGE_TOTAL_KEY]
			Expect(found).To(Equal(true))
			Expect(aggrSample).To(Equal(fmt.Sprintf("%d", SAMPLE_AGGR)))
		}
		// check sample node energy
		val, err := convertPromToValue(body, NODE_ENERGY_METRIC)
		Expect(err).NotTo(HaveOccurred())
		Expect(val).To(Equal(int(2 * SAMPLE_NODE_ENERGY / 1000))) //J
		val, err = convertPromToValue(body, NODE_ENERGY_STAT_METRRIC)
		Expect(err).NotTo(HaveOccurred())
		Expect(val).Should(BeEquivalentTo(int(SAMPLE_NODE_ENERGY / 1000))) // J
		val, err = convertPromToValue(body, NODE_LABEL_PREFIX+CURR_PREFIX+ENERGY_LABELS["pkg"]+J_SUFFIX)
		Expect(err).NotTo(HaveOccurred())
		Expect(val).Should(BeEquivalentTo(int(SAMPLE_PKG_ENERGY / 1000))) // J
		// check pkg energy
		val, err = convertPromToValue(body, PACKAGE_ENERGY_METRIC)
		Expect(err).NotTo(HaveOccurred())
		Expect(val).Should(BeEquivalentTo(int(2 * SAMPLE_PKG_ENERGY))) // mJ
		// check sample frequency
		val, err = convertPromToValue(body, FREQ_METRIC)
		Expect(err).NotTo(HaveOccurred())
		Expect(val).To(Equal(int(SAMPLE_FREQ)))
	})
})
