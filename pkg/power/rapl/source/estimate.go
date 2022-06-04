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
	"io/ioutil"
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
	powerDataPath    = "/var/lib/kepler/data/power_data.csv" // obtained from https://github.com/cloud-carbon-footprint/cloud-carbon-coefficients/blob/main/output/coefficients-aws-use.csv
	cpuModelDataPath = "/var/lib/kepler/data/cpu_model.csv"
	dramInGB         int
	cpuCores         = runtime.NumCPU()

	startTime = time.Now()

	perThreadMinPowerEstimate, perThreadMaxPowerEstimate, perGBPowerEstimate float64

	cpuModelRegex = []string{
		"(.)*Intel(.)*( [-a-zA-Z0-9]+[0-9]+[A-Z]* )", // Intel, e.g. "model name      : Intel(R) Core(TM) i7-8750H CPU @ 2.20GHz". This is seen on KVM
		"(.)*Intel(.)*( [-a-zA-Z0-9]+[0-9]+[A-Z]*)",  // Intel, e.g. "model name      : 12th Gen Intel(R) Core(TM) i7-12700H". This is seen on Hyper-V
	}
	dramRegex = "^MemTotal:[\\s]+([0-9]+)"
)

type PowerEstimateData struct {
	Architecture string  `csv:"Architecture"`
	MinWatts     float64 `csv:"Min Watts"`
	MaxWatts     float64 `csv:"Max Watts"`
	PerGBWatts   float64 `csv:"GB/Chip"`
}

type CPUModelData struct {
	CPUModel     string `csv:"Model"`
	Architecture string `csv:"Architecture"`
}

func getCPUModel() (string, error) {
	b, err := ioutil.ReadFile("/proc/cpuinfo")
	if err != nil {
		return "", err
	}
	for _, r := range cpuModelRegex {
		re := regexp.MustCompile(r)
		matches := re.FindStringSubmatch(string(b))
		l := len(matches)
		if l > 0 {
			return strings.TrimSpace(matches[l-1]), nil
		}
	}
	return "", fmt.Errorf("no CPU architecture found")
}

func GetCPUArchitecture() (string, error) {
	myCPUModel, err := getCPUModel()
	if err != nil {
		return "", err
	}
	file, err := os.Open(cpuModelDataPath)
	if err != nil {
		return "", err
	}
	reader := csv.NewReader(file)

	dec, err := csvutil.NewDecoder(reader)
	if err != nil {
		return "", err
	}

	for {
		var p CPUModelData
		if err := dec.Decode(&p); err == io.EOF {
			break
		}
		if p.CPUModel == myCPUModel {
			return p.Architecture, nil
		}
	}

	return "", fmt.Errorf("no CPU power model found for architecture %s", myCPUModel)
}

func getDram() (int, error) {
	b, err := ioutil.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, err
	}
	re := regexp.MustCompile(dramRegex)
	matches := re.FindAllStringSubmatch(string(b), -1)
	if len(matches) > 0 {
		dram, err := strconv.Atoi(strings.TrimSpace(string(matches[0][1])))
		if err != nil {
			return 0, err
		}
		return dram / (1024 * 1024) /*kB to GB*/, nil
	}
	return 0, fmt.Errorf("no memory info found")
}

func getCPUPowerEstimate(cpu string) (float64, float64, float64, error) {
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
func (r *PowerEstimate) IsSupported() bool {
	cpu, err := GetCPUArchitecture()
	if err != nil {
		fmt.Printf("no cpu info: %v\n", err)
		return false
	}
	dramInGB, err = getDram()
	if err != nil {
		fmt.Printf("no dram info: %v\n", err)
		return false
	}
	perThreadMinPowerEstimate, perThreadMaxPowerEstimate, perGBPowerEstimate, err = getCPUPowerEstimate(cpu)
	startTime = time.Now()
	fmt.Printf("cpu architecture %v, dram in GB %v\n", cpu, dramInGB)
	return err == nil
}

func (r *PowerEstimate) StopPower() {
	startTime = time.Now()
}

func (r *PowerEstimate) GetEnergyFromDram() (uint64, error) {
	now := time.Now()
	diff := now.Sub(startTime)
	seconds := diff.Seconds()
	return uint64(float64(dramInGB)*perGBPowerEstimate*seconds) * 1000, nil
}

func (r *PowerEstimate) GetEnergyFromCore() (uint64, error) {
	now := time.Now()
	diff := now.Sub(startTime)
	seconds := diff.Seconds()
	//TODO use utilization
	return uint64(float64(cpuCores)*seconds*(perThreadMinPowerEstimate+perThreadMaxPowerEstimate)/2) * 1000, nil
}

func (r *PowerEstimate) GetEnergyFromUncore() (uint64, error) {
	return 0, nil
}

func (r *PowerEstimate) GetEnergyFromPackage() (uint64, error) {
	return 0, nil
}
