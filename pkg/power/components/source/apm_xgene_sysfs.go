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
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"k8s.io/klog/v2"
)

const (
	// sysfs path templates for Ampere Xgene hwmon
	powerLabelPathTemplate = "/sys/class/hwmon/hwmon*/power*_label"
	cpuPowerLabel          = "CPU power"
)

var (
	powerInputPath = ""
	currTime       = time.Now()
)

type ApmXgeneSysfs struct{}

func (r *ApmXgeneSysfs) IsSystemCollectionSupported() bool {
	labelFiles, err := filepath.Glob(powerLabelPathTemplate)
	if err != nil {
		return false
	}
	for _, labelFile := range labelFiles {
		var data []byte
		if data, err = os.ReadFile(labelFile); err != nil {
			continue
		}
		if strings.TrimSuffix(strings.TrimSpace(string(data)), "\n") == cpuPowerLabel {
			// replace the label file with the input file
			powerInputPath = strings.Replace(labelFile, "label", "input", 1)
			klog.V(1).Infof("Found power input file: %s", powerInputPath)
			return true
		}
	}
	return false
}

func (r *ApmXgeneSysfs) GetEnergyFromDram() (uint64, error) {
	return 0, nil
}

func (r *ApmXgeneSysfs) GetEnergyFromCore() (uint64, error) {
	now := time.Now()
	diff := now.Sub(currTime)
	seconds := diff.Seconds()
	// read from the power input file and convert file content to numbers
	var data []byte
	var err error
	if data, err = os.ReadFile(powerInputPath); err != nil {
		return 0, err
	}
	power, err := strconv.ParseFloat(strings.TrimSpace(string(data)), 64)
	if err != nil {
		return 0, err
	}
	return uint64(power * seconds), nil
}

func (r *ApmXgeneSysfs) GetEnergyFromUncore() (uint64, error) {
	return 0, nil
}

func (r *ApmXgeneSysfs) GetEnergyFromPackage() (uint64, error) {
	return 0, nil
}

func (r *ApmXgeneSysfs) GetNodeComponentsEnergy() map[int]NodeComponentsEnergy {
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

func (r *ApmXgeneSysfs) StopPower() {
}
