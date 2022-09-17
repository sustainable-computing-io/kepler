package model

import (
	"encoding/json"
	"log"
	"math"
	"net"

	"github.com/sustainable-computing-io/kepler/pkg/config"
)

const (
	serveSocket = "/tmp/estimator.sock"
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
	totalCoreUsage := sumUsage[config.CoreUsageMetric]
	totalDRAMUsage := sumUsage[config.DRAMUsageMetric]
	totalUncoreUsage := sumUsage[config.UncoreUsageMetric]
	totalUsage := sumUsage[config.GeneralUsageMetric]

	// Package (PKG) domain measures the energy consumption of the entire socket, including the consumption of all the cores, integrated graphics and
	// also the "unknown" components such as last level caches and memory controllers
	pkgUnknownValue := totalPkgPower - totalCorePower - totalUncorePower

	// find ratio power
	for _, podMetricValue := range podMetricValues {
		coreValue := getRatio(podMetricValue, coreMetricIndex, totalCoreUsage, totalCorePower, podNumber)
		dramValue := getRatio(podMetricValue, dramMetricIndex, totalDRAMUsage, totalDRAMPower, podNumber)
		uncoreValue := getRatio(podMetricValue, uncoreMetricIndex, totalUncoreUsage, totalUncorePower, podNumber)
		unknownValue := getRatio(podMetricValue, generalMetricIndex, totalUsage, pkgUnknownValue, podNumber)
		pkgValue := coreValue + uncoreValue + unknownValue
		podCore = append(podCore, coreValue)
		podDRAM = append(podDRAM, dramValue)
		podUncore = append(podUncore, uncoreValue)
		podPkg = append(podPkg, pkgValue)
	}
	return
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
	powerRequestJSON, err := json.Marshal(powerRequest)
	if err != nil {
		log.Printf("marshal error: %v (%v)", err, powerRequest)
		return []float64{}
	}

	c, err := net.Dial("unix", serveSocket)
	if err != nil {
		log.Printf("dial error: %v", err)
		return []float64{}
	}
	defer c.Close()

	_, err = c.Write(powerRequestJSON)
	if err != nil {
		log.Printf("estimator write error: %v", err)
		return []float64{}
	}
	buf := make([]byte, 1024)
	n, err := c.Read(buf)
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
