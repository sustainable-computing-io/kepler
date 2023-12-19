/*
Copyright 2023.

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
	uJTomJ                 = 1000
)

var (
	powerInputPath = ""
)

type ApmXgeneSysfs struct {
	currTime time.Time
}

func (ApmXgeneSysfs) GetName() string {
	return "ampere-xgene-hwmon"
}

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

func (r *ApmXgeneSysfs) GetAbsEnergyFromDram() (uint64, error) {
	return 0, nil
}

func (r *ApmXgeneSysfs) GetAbsEnergyFromCore() (uint64, error) {
	if r.currTime.IsZero() {
		r.currTime = time.Now()
		return 0, nil
	}
	now := time.Now()
	diff := now.Sub(r.currTime)
	seconds := diff.Seconds()
	r.currTime = now
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
	// per https://dri.freedesktop.org/docs/drm/hwmon/xgene-hwmon.html, the power is in uJ/s
	return uint64(power*seconds) / uJTomJ, nil
}

func (r *ApmXgeneSysfs) GetAbsEnergyFromUncore() (uint64, error) {
	return 0, nil
}

func (r *ApmXgeneSysfs) GetAbsEnergyFromPackage() (uint64, error) {
	return 0, nil
}

func (r *ApmXgeneSysfs) GetAbsEnergyFromNodeComponents() map[int]NodeComponentsEnergy {
	coreEnergy, _ := r.GetAbsEnergyFromCore()
	dramEnergy, _ := r.GetAbsEnergyFromDram()
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
