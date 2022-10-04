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
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/jszwec/csvutil"
	"k8s.io/klog/v2"
)

type PowerEstimate struct{}

var (
	cpuModelDataPath = "/var/lib/kepler/data/normalized_cpu_arch.csv"
	powerDataPath    = "/var/lib/kepler/data/power_data.csv" // obtained from https://github.com/cloud-carbon-footprint/cloud-carbon-coefficients/blob/main/output/coefficients-aws-use.csv
	dramRegex        = "^MemTotal:[\\s]+([0-9]+)"

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

type CPUModelData struct {
	Name         string `csv:"Name"`
	Architecture string `csv:"Architecture"`
}

func GetCPUArchitecture() (string, error) {
	// check if there is a CPU architecture override
	cpuArchOverride := os.Getenv("CPU_ARCH_OVERRIDE")
	if len(cpuArchOverride) > 0 {
		klog.V(2).Infof("cpu arch override: %v\n", cpuArchOverride)
		return cpuArchOverride, nil
	}
	output, err := exec.Command("archspec", "cpu").Output()
	if err != nil {
		return "", err
	}
	myCPUModel := strings.TrimSuffix(string(output), "\n")
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
		if strings.HasPrefix(myCPUModel, p.Name) {
			return p.Architecture, nil
		}
	}

	return "", fmt.Errorf("no CPU power model found for architecture %s", myCPUModel)
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
func (r *PowerEstimate) IsSupported() bool {
	cpu, err := GetCPUArchitecture()
	if err != nil {
		klog.V(2).Infof("no cpu info: %v\n", err)
		return false
	}
	dramInGB, err = getDram()
	if err != nil {
		klog.V(2).Infof("no dram info: %v\n", err)
		return false
	}
	perThreadMinPowerEstimate, perThreadMaxPowerEstimate, perGBPowerEstimate, err = getCPUPowerEstimate(cpu)
	startTime = time.Now()
	klog.V(4).Infof("cpu architecture %v, dram in GB %v\n", cpu, dramInGB)
	return err == nil
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

// No package information, consider as 1 package
func (r *PowerEstimate) GetRAPLEnergy() map[int]RAPLEnergy {
	coreEnergy, _ := r.GetEnergyFromCore()
	dramEnergy, _ := r.GetEnergyFromDram()
	packageEnergies := make(map[int]RAPLEnergy)
	packageEnergies[0] = RAPLEnergy{
		Core:   coreEnergy,
		DRAM:   dramEnergy,
		Uncore: 0,
		Pkg:    coreEnergy,
	}
	return packageEnergies
}
