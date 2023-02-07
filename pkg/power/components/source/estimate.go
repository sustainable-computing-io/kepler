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

package source

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/jszwec/csvutil"
)

type PowerEstimate struct{}

var (
	powerDataPath = "/var/lib/kepler/data/power_data.csv" // obtained from https://github.com/cloud-carbon-footprint/cloud-carbon-coefficients/blob/main/output/coefficients-aws-use.csv
	dramRegex     = "^MemTotal:[\\s]+([0-9]+)"

	dramInGB                                                                 int
	cpuCores                                                                 = runtime.NumCPU()
	startTime                                                                = time.Now()
	perThreadMinPowerEstimate, perThreadMaxPowerEstimate, perGBPowerEstimate float64
)

type PowerEstimateData struct {
	Architecture string  `csv:"Architecture"`
	MinWatts     float64 `csv:"Min Watts"`
	MaxWatts     float64 `csv:"Max Watts"`
	PerGBWatts   float64 `csv:"GB/Chip"`
}

func getDram() (int, error) {
	b, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, err
	}
	re := regexp.MustCompile(dramRegex)
	matches := re.FindAllStringSubmatch(string(b), -1)
	if len(matches) > 0 {
		dram, err := strconv.Atoi(strings.TrimSpace(matches[0][1]))
		if err != nil {
			return 0, err
		}
		return dram / (1024 * 1024) /*kB to GB*/, nil
	}
	return 0, fmt.Errorf("no memory info found")
}

func getCPUPowerEstimate(cpu string) (perThreadMinPowerEstimate, perThreadMaxPowerEstimate, perGBPowerEstimate float64, err error) {
	file, _ := os.Open(powerDataPath)
	reader := csv.NewReader(file)

	dec, err := csvutil.NewDecoder(reader)
	if err != nil {
		return 0.0, 0.0, 0.0, err
	}

	for {
		var p PowerEstimateData
		if err := dec.Decode(&p); err == io.EOF {
			break
		}
		if p.Architecture == cpu {
			return p.MinWatts, p.MaxWatts, p.PerGBWatts, nil
		}
	}

	return 0.0, 0.0, 0.0, fmt.Errorf("no CPU power info found")
}

// If the Estimated Power is being used, it means that the system does not support Components Power Measurement
func (r *PowerEstimate) IsSystemCollectionSupported() bool {
	return false
}

func (r *PowerEstimate) StopPower() {
	startTime = time.Now()
}

func (r *PowerEstimate) GetEnergyFromDram() (uint64, error) {
	now := time.Now()
	diff := now.Sub(startTime)
	seconds := diff.Seconds()
	return uint64(float64(dramInGB)*perGBPowerEstimate*seconds) * 1000 / 3600, nil
}

func (r *PowerEstimate) GetEnergyFromCore() (uint64, error) {
	now := time.Now()
	diff := now.Sub(startTime)
	seconds := diff.Seconds()
	// TODO: use utilization
	return uint64(float64(cpuCores)*seconds*(perThreadMinPowerEstimate+perThreadMaxPowerEstimate)/2) * 1000 / 3600, nil
}

func (r *PowerEstimate) GetEnergyFromUncore() (uint64, error) {
	return 0, nil
}

func (r *PowerEstimate) GetEnergyFromPackage() (uint64, error) {
	return r.GetEnergyFromCore()
}

// No node components information, consider as 1 socket
func (r *PowerEstimate) GetNodeComponentsEnergy() map[int]NodeComponentsEnergy {
	coreEnergy, _ := r.GetEnergyFromCore()
	dramEnergy, _ := r.GetEnergyFromDram()
	componentsEnergies := make(map[int]NodeComponentsEnergy)
	componentsEnergies[0] = NodeComponentsEnergy{
		Core:   coreEnergy,
		DRAM:   dramEnergy,
		Uncore: 0,
		Pkg:    coreEnergy,
	}
	return componentsEnergies
}
