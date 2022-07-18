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
	"encoding/csv"
	"fmt"
	"io"
	"os"

	"github.com/sustainable-computing-io/kepler/pkg/power/rapl/source"

	"github.com/jszwec/csvutil"
)

type Coeff struct {
	Architecture  string  `csv:"architecture"`
	CPUTime       float64 `csv:"cpu_time"`
	CPUCycle      float64 `csv:"cpu_cycle"`
	CPUInstr      float64 `csv:"cpu_instruction"`
	MemoryUsage   float64 `csv:"memory_usage"`
	CacheMisses   float64 `csv:"cache_misses"`
	InterceptCore float64 `csv:"intercept_core"`
	InterceptDram float64 `csv:"intercept_dram"`
}

var (
	//obtained the coeff via regression
	BareMetalCoeff = Coeff{
		CPUTime:       0.0,
		CPUCycle:      0.0000005268224465,
		CPUInstr:      0.0000005484982329,
		InterceptCore: 152121.0472,

		MemoryUsage:   0.0,
		CacheMisses:   0.000004112383656,
		InterceptDram: -23.70284983,
	}
	// if per counters are not avail on VMs, don't use them
	VMCoeff = Coeff{
		CPUTime:     1.0,
		CPUCycle:    0,
		CPUInstr:    0,
		MemoryUsage: 1.0,
		CacheMisses: 0,
	}
	RunTimeCoeff Coeff

	modelServerEndpoint string

	powerModelPath = "/var/lib/kepler/data/power_model.csv"
)

func SetVMCoeff() {
	RunTimeCoeff = VMCoeff
}

func SetBMCoeff() {
	// use the default one if no model found
	RunTimeCoeff = BareMetalCoeff

	arch, err := source.GetCPUArchitecture()
	if err == nil {
		cpuArch := arch
		file, err := os.Open(powerModelPath)
		if err == nil {
			reader := csv.NewReader(file)

			dec, err := csvutil.NewDecoder(reader)
			if err == nil {
				for {
					var p Coeff
					if err := dec.Decode(&p); err == io.EOF {
						break
					}
					if p.Architecture == cpuArch {
						fmt.Printf("use model %v\n", p)
						RunTimeCoeff = p
					}
				}
			}
		}
	}
}

func SetRuntimeCoeff(coeff Coeff) {
	RunTimeCoeff = coeff

}

func SetModelServerEndpoint(ep string) {
	modelServerEndpoint = ep
}

/*
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
*/
