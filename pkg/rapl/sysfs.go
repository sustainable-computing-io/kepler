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

package rapl

import (
	"fmt"
	"io/ioutil"
	"strconv"
	"strings"
)

const (
	corePath    = "/sys/class/powercap/intel-rapl/intel-rapl:%d/intel-rapl:%d:0/energy_uj"
	dramPath    = "/sys/class/powercap/intel-rapl/intel-rapl:%d/intel-rapl:%d:1/energy_uj"
	uncorePath  = "/sys/class/powercap/intel-rapl/intel-rapl:%d/intel-rapl:%d:2/energy_uj"
	packagePath = "/sys/class/powercap/intel-rapl/intel-rapl:%d/energy_uj"
)

var (
	maxSockets = 128
)

func getEnergy(base string) (uint64, error) {
	energy := uint64(0)
	i := 0
	for i = 0; i < maxSockets; i++ {
		path := fmt.Sprintf(base, i, i)
		data, err := ioutil.ReadFile(path)
		if err != nil {
			maxSockets = i
			break
		}
		count, err := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
		if err == nil {
			energy += count / 1000 /*mJ*/
		}
	}
	return energy, nil
}

type raplSysfs struct{}

func (r *raplSysfs) IsSupported() bool {
	path := fmt.Sprintf(corePath, 0, 0)
	_, err := ioutil.ReadFile(path)
	return err == nil
}

func (r *raplSysfs) GetEnergyFromDram() (uint64, error) {
	return getEnergy(dramPath)
}

func (r *raplSysfs) GetEnergyFromCore() (uint64, error) {
	return getEnergy(corePath)
}

func (r *raplSysfs) GetEnergyFromUncore() (uint64, error) {
	return getEnergy(uncorePath)
}

func (r *raplSysfs) GetEnergyFromPackage() (uint64, error) {
	energy := uint64(0)
	i := 0
	for i = 0; i < maxSockets; i++ {
		path := fmt.Sprintf(packagePath, i)
		data, err := ioutil.ReadFile(path)
		if err != nil {
			maxSockets = i
			break
		}
		count, err := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
		if err == nil {
			energy += count / 1000 /*mJ*/
		}
	}
	return energy, nil
}

func (r *raplSysfs) StopRAPL() {
}
