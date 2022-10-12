package collector

import (
	"io"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sustainable-computing-io/kepler/pkg/power/rapl/source"
)

const (
	CPUUsageTotalKey = "cgroupfs_cpu_usage_us"
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

var _ = Describe("Test Collector Unit", func() {
	It("Init and Run", func() {
		newCollector, err := New()
		setPodStatProm()
		Expect(err).NotTo(HaveOccurred())
		err = prometheus.Register(newCollector)
		Expect(err).NotTo(HaveOccurred())
		req, _ := http.NewRequest("GET", "", http.NoBody)
		res := httptest.NewRecorder()
		handler := promhttp.Handler()
		handler.ServeHTTP(res, req)
		body, _ := io.ReadAll(res.Body)
		Expect(len(body)).Should(BeNumerically(">", 0))

		regStr := fmt.Sprintf(`%s{[^{}]*}`, podEnergyStatMetric)
		r := regexp.MustCompile(regStr)
		match := r.FindString(string(body))
		Expect(match).To(Equal(""))

		v := NewPodEnergy("podA", "default")
		v.EnergyInCore = &UInt64Stat{
			Curr: 10,
			Aggr: 10,
		}
		v.CgroupFSStats = map[string]*UInt64StatCollection{
			CPUUsageTotalKey: {
				Stat: map[string]*UInt64Stat{
					"cA": {
						Curr: SampleCurr,
						Aggr: SampleAggr,
					},
				},
			},
		}
		cpuFrequency = map[int32]uint64{
			0: SampleFreq,
		}
		podEnergy = map[string]*PodEnergy{
			"a": v,
		}
		// initial
		sensorEnergy = map[string]float64{
			"sensor0": sampleNodeEnergy,
		}
		pkgEnergy = map[int]source.RAPLEnergy{
			0: {
				Pkg: samplePkgEnergy,
			},
		}
		nodeEnergy.SetValues(sensorEnergy, pkgEnergy, 0, [][]float64{})
		sensorEnergy = map[string]float64{
			"sensor0": sampleNodeEnergy * 2,
		}
		pkgEnergy = map[int]source.RAPLEnergy{
			0: {
				Pkg: samplePkgEnergy * 2,
			},
		}
		nodeEnergy.SetValues(sensorEnergy, pkgEnergy, 0, [][]float64{})

		res = httptest.NewRecorder()
		handler = promhttp.Handler()
		handler.ServeHTTP(res, req)
		body, _ = io.ReadAll(res.Body)
		Expect(len(body)).Should(BeNumerically(">", 0))
		fmt.Printf("Result:\n %s\n", body)

		// check sample pod energy stat
		response := convertPromMetricToMap(body, podEnergyStatMetric)
		if len(availableCgroupMetrics) > 0 {
			currSample, found := response["curr_"+CPUUsageTotalKey]
			Expect(found).To(Equal(true))
			Expect(currSample).To(Equal(fmt.Sprintf("%d", SampleCurr)))
			aggrSample, found := response["total_"+CPUUsageTotalKey]
			Expect(found).To(Equal(true))
			Expect(aggrSample).To(Equal(fmt.Sprintf("%d", SampleAggr)))
		}
		// check sample node energy
		val, err := convertPromToValue(body, nodeEnergyMetric)
		Expect(err).NotTo(HaveOccurred())
		Expect(val).To(Equal(int(2 * sampleNodeEnergy / 1000))) //J
		val, err = convertPromToValue(body, nodeEnergyStatMetric)
		Expect(err).NotTo(HaveOccurred())
		Expect(val).Should(BeEquivalentTo(int(sampleNodeEnergy / 1000))) // J
		val, err = convertPromToValue(body, nodeLabelPrefix+CurrPrefix+EnergyLabels["pkg"]+JSuffix)
		Expect(err).NotTo(HaveOccurred())
		Expect(val).Should(BeEquivalentTo(int(samplePkgEnergy / 1000))) // J
		// check pkg energy
		val, err = convertPromToValue(body, pkgEnergyMetric)
		Expect(err).NotTo(HaveOccurred())
		Expect(val).Should(BeEquivalentTo(int(2 * samplePkgEnergy))) // mJ
		// check sample frequency
		val, err = convertPromToValue(body, freqMetric)
		Expect(err).NotTo(HaveOccurred())
		Expect(val).To(Equal(int(SampleFreq)))
	})
})
