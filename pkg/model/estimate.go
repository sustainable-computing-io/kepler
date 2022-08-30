package model

import (
	"encoding/json"
	"github.com/sustainable-computing-io/kepler/pkg/config"
	"log"
	"math"
	"net"
)

const (
	SERVE_SOCKET = "/tmp/estimator.sock"
)

var (
	coreMetricIndex    int = -1
	dramMetricIndex    int = -1
	uncoreMetricIndex  int = -1
	generalMetricIndex int = -1
)

type PowerRequest struct {
	ModelName       string      `json:"model_name"`
	MetricNames     []string    `json:"metrics"`
	PodMetricValues [][]float64 `json:"values"`
	CorePower       []float64   `json:"core_power"`
	DRAMPower       []float64   `json:"dram_power"`
	UncorePower     []float64   `json:"uncore_power"`
	PkgPower        []float64   `json:"pkg_power"`
	GPUPower        []float64   `json:"gpu_power"`
	SelectFilter    string      `json:"filter"`
}

type PowerResponse struct {
	Powers  []float64 `json:"powers"`
	Message string    `json:"msg"`
}

func InitMetricIndexes(metricNames []string) {
	for index, metricName := range metricNames {
		if metricName == config.CoreUsageMetric {
			coreMetricIndex = index
			log.Printf("set coreMetricIndex = %d", index)
		}
		if metricName == config.DRAMUsageMetric {
			dramMetricIndex = index
			log.Printf("set dramMetricIndex = %d", index)
		}
		if metricName == config.UncoreUsageMetric {
			uncoreMetricIndex = index
			log.Printf("set uncoreMetricIndex = %d", index)
		}
		if metricName == config.GeneralUsageMetric {
			generalMetricIndex = index
			log.Printf("set generalMetricIndex = %d", index)
		}
	}
}

func GetSumUsageMap(metricNames []string, podMetricValues [][]float64) (sumUsage map[string]float64) {
	sumUsage = make(map[string]float64)
	for i, metricName := range metricNames {
		sumUsage[metricName] = 0
		for _, podMetricValue := range podMetricValues {
			sumUsage[metricName] += podMetricValue[i]
		}
	}
	return
}

func GetSumDelta(corePower, dramPower, uncorePower, pkgPower, gpuPower []float64) (totalCorePower, totalDRAMPower, totalUncorePower, totalPkgPower, totalGPUPower uint64) {
	for i, val := range pkgPower {
		totalCorePower += uint64(corePower[i])
		totalDRAMPower += uint64(dramPower[i])
		totalUncorePower += uint64(uncorePower[i])
		totalPkgPower += uint64(val)
	}
	for _, val := range gpuPower {
		totalGPUPower += uint64(val)
	}
	return
}

func getRatio(podMetricValue []float64, metricIndex int, totalUsage float64, totalPower uint64, podNumber float64) uint64 {
	var power float64
	if metricIndex >= 0 && totalUsage > 0 {
		power = podMetricValue[metricIndex] / totalUsage * float64(totalPower)
	} else {
		power = float64(totalPower) / podNumber
	}
	return uint64(math.Ceil(power))
}

func GetPowerFromUsageRatio(podMetricValues [][]float64, totalCorePower, totalDRAMPower, totalUncorePower, totalPkgPower uint64, sumUsage map[string]float64) (podCore, podDRAM, podUncore, podPkg []uint64) {
	podNumber := float64(len(podMetricValues))
	totalCoreUsage := sumUsage[config.UncoreUsageMetric]
	totalDRAMUsage := sumUsage[config.DRAMUsageMetric]
	totalUncoreUsage := sumUsage[config.UncoreUsageMetric]
	totalUsage := sumUsage[config.GeneralUsageMetric]

	unknownValue := totalPkgPower - totalCorePower - totalDRAMPower - totalUncorePower

	// find ratio power
	for _, podMetricValue := range podMetricValues {
		coreValue := getRatio(podMetricValue, coreMetricIndex, totalCoreUsage, totalCorePower, podNumber)
		dramValue := getRatio(podMetricValue, dramMetricIndex, totalDRAMUsage, totalDRAMPower, podNumber)
		uncoreValue := getRatio(podMetricValue, uncoreMetricIndex, totalUncoreUsage, totalUncorePower, podNumber)
		unknownValue := getRatio(podMetricValue, generalMetricIndex, totalUsage, unknownValue, podNumber)
		pkgValue := coreValue + dramValue + uncoreValue + unknownValue
		podCore = append(podCore, coreValue)
		podDRAM = append(podDRAM, dramValue)
		podUncore = append(podUncore, uncoreValue)
		podPkg = append(podPkg, pkgValue)
	}
	return
}

// convert f64 to f32 for reducing communication cost
func f64Tof32(f64arr []float64) []float32 {
	var f32arr []float32
	for _, val := range f64arr {
		f32arr = append(f32arr, float32(val))
	}
	return f32arr
}

func GetDynamicPower(metricNames []string, podMetricValues [][]float64, corePower, dramPower, uncorePower, pkgPower, gpuPower []float64) []float64 {
	powerRequest := PowerRequest{
		ModelName:       config.EstimatorModel,
		MetricNames:     metricNames,
		PodMetricValues: podMetricValues,
		CorePower:       corePower,
		DRAMPower:       dramPower,
		UncorePower:     uncorePower,
		PkgPower:        pkgPower,
		GPUPower:        gpuPower,
		SelectFilter:    config.EstimatorSelectFilter,
	}
	powerRequestJson, err := json.Marshal(powerRequest)
	if err != nil {
		log.Printf("marshal error: %v (%v)", err, powerRequest)
		return []float64{}
	}

	c, err := net.Dial("unix", SERVE_SOCKET)
	if err != nil {
		log.Printf("dial error: %v", err)
		return []float64{}
	}
	defer c.Close()

	_, err = c.Write(powerRequestJson)
	if err != nil {
		log.Printf("estimator write error: %v", err)
		return []float64{}
	}
	buf := make([]byte, 1024)
	n, err := c.Read(buf[:])
	if err != nil {
		log.Printf("estimator read error: %v", err)
		return []float64{}
	}
	var powerResponse PowerResponse
	err = json.Unmarshal(buf[0:n], &powerResponse)
	if err != nil {
		log.Printf("estimator unmarshal error: %v (%s)", err, string(buf[0:n]))
		return []float64{}
	}
	if len(powerResponse.Powers) != len(podMetricValues) {
		log.Printf("fail to get pod power : %s", powerResponse.Message)
	}
	return powerResponse.Powers
}
