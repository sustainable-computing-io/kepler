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
	maxSockets = 4
)

func getEnergy(base string) (int, error) {
	energy := 0
	i := 0
	for i = 0; i < maxSockets; i++ {
		path := fmt.Sprintf(base, i, i)
		data, err := ioutil.ReadFile(path)
		if err != nil {
			maxSockets = i
			break
		}
		count, err := strconv.Atoi(strings.TrimSpace(string(data)))
		if err == nil {
			energy += count
		}
	}
	return energy, nil
}

func GetEnergyFromDram() (int, error) {
	return getEnergy(dramPath)
}

func GetEnergyFromCore() (int, error) {
	return getEnergy(corePath)
}

func GetEnergyFromUncore() (int, error) {
	return getEnergy(uncorePath)
}

func GetEnergyFromPackage() (int, error) {
	energy := 0
	i := 0
	for i = 0; i < maxSockets; i++ {
		path := fmt.Sprintf(packagePath, i)
		data, err := ioutil.ReadFile(path)
		if err != nil {
			maxSockets = i
			break
		}
		count, err := strconv.Atoi(strings.TrimSpace(string(data)))
		if err == nil {
			energy += count
		}
	}
	return energy, nil
}
