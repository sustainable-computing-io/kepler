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

package model

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

type Coeff struct {
	CPUTime     float64 `json:"cpu_time"`
	CPUCycle    float64 `json:"cpu_cycle"`
	CPUInstr    float64 `json:"cpu_instruction"`
	MemoryUsage float64 `json:"memory_usage"`
	CacheMisses float64 `json:"cache_misses"`
}

type RegressionModel struct {
	Core        float64 `json:"core"`
	Dram        float64 `json:"dram"`
	CPUTime     float64 `json:"cpu_time"`
	CPUCycle    float64 `json:"cpu_cycles"`
	CPUInstr    float64 `json:"cpu_instructions"`
	MemoryUsage float64 `json:"memory_usage"`
	CacheMisses float64 `json:"cache_misses"`
}

var (
	//TODO obtain the coeff via regression
	BareMetalCoeff = Coeff{
		CPUTime:     0.6,
		CPUCycle:    0.2,
		CPUInstr:    0.2,
		MemoryUsage: 0.5,
		CacheMisses: 0.5,
	}
	// if per counters are not avail on VMs, don't use them
	VMCoeff = Coeff{
		CPUTime:     1.0,
		CPUCycle:    0,
		CPUInstr:    0,
		MemoryUsage: 1.0,
		CacheMisses: 0,
	}
	RunTimeCoeff Coeff = BareMetalCoeff

	modelServerEndpoint string
)

func SetVMCoeff() {
	RunTimeCoeff = VMCoeff
}

func SetBMCoeff() {
	RunTimeCoeff = BareMetalCoeff
}

func SetRuntimeCoeff(coeff Coeff) {
	RunTimeCoeff = coeff

}
func SetModelServerEndpoint(ep string) {
	modelServerEndpoint = ep
}

func GetCoeffFromModelServer() (*Coeff, error) {
	if len(modelServerEndpoint) == 0 {
		return &RunTimeCoeff, nil
	}
	req, _ := http.NewRequest("GET", modelServerEndpoint, nil)

	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		fmt.Printf("failed to connect to %s: %v\n", modelServerEndpoint, err)
		return nil, err
	}

	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}
	coeff := Coeff{}
	err = json.Unmarshal(body, &coeff)
	if err != nil {
		return nil, fmt.Errorf("failed to parse response body: %v", err)
	}

	return &coeff, nil
}

func SendDataToModelServer(data *RegressionModel) {
	if len(modelServerEndpoint) == 0 {
		return
	}

	go func() {
		buf := new(bytes.Buffer)
		json.NewEncoder(buf).Encode(data)
		req, _ := http.NewRequest("POST", modelServerEndpoint, buf)

		client := &http.Client{}
		res, err := client.Do(req)
		if err != nil {
			fmt.Printf("failed to connect to %s: %v\n", modelServerEndpoint, err)
			return
		}

		defer res.Body.Close()
		fmt.Println("response Status:", res.Status)
		return
	}()
	return
}
