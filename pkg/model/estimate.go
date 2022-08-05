package model

import (
	"encoding/json"
	"log"
	"net"
	// "os"
)

const (
	SERVE_SOCKET = "/tmp/estimator.sock"
)

type PowerRequest struct {
	ModelName       string      `json:"model_name"`
	MetricNames     []string    `json:"metrics"`
	PodMetricValues [][]float32 `json:"values"`
	CorePower       []float32   `json:"core_power"`
	DRAMPower       []float32   `json:"dram_power"`
	GPUPower        []float32   `json:"gpu_power"`
	OtherPower      []float32   `json:"other_power"`
}

type PowerResponse struct {
	Powers  []float32 `json:"powers"`
	Message string    `json:"msg"`
}

func GetPower(modelName string, metricNames []string, podMetricValues [][]float32, corePower, dramPower, gpuPower, otherPower []float32) []float32 {
	powerRequest := PowerRequest{
		ModelName:       modelName,
		MetricNames:     metricNames,
		PodMetricValues: podMetricValues,
		CorePower:       corePower,
		DRAMPower:       dramPower,
		GPUPower:        gpuPower,
		OtherPower:      otherPower,
	}
	powerRequestJson, err := json.Marshal(powerRequest)

	c, err := net.Dial("unix", SERVE_SOCKET)
	if err != nil {
		log.Printf("dial error: %v", err)
		return []float32{}
	}
	defer c.Close()

	if err != nil {
		log.Printf("marshal error: %v", err)
		return []float32{}
	}
	_, err = c.Write(powerRequestJson)
	if err != nil {
		log.Printf("estimator write error: %v", err)
		return []float32{}
	}
	buf := make([]byte, 1024)
	n, err := c.Read(buf[:])
	if err != nil {
		log.Printf("estimator read error: %v", err)
		return []float32{}
	}
	var powerResponse PowerResponse
	err = json.Unmarshal(buf[0:n], &powerResponse)
	if err != nil {
		log.Printf("estimator unmarshal error: %v", err)
		return []float32{}
	}
	if len(powerResponse.Powers) != len(podMetricValues) {
		log.Printf(powerResponse.Message)
	}
	return powerResponse.Powers
}
