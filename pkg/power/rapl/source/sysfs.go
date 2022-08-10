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
	"fmt"
	"io/ioutil"
	"log"
	"strconv"
	"strings"
)

const (
	// sysfs path templates
	numPkgPathTemplate      = "/sys/devices/system/cpu/cpu%d/topology/physical_package_id"
	packageNamePathTemplate = "/sys/class/powercap/intel-rapl/intel-rapl:%d/"
	eventNamePathTemplate   = "/sys/class/powercap/intel-rapl/intel-rapl:%d/intel-rapl:%d:%d/"
	cpuInfoPath             = "/proc/cpuinfo"
	energyFile              = "energy_uj"

	// RAPL number of events (core, dram and uncore)
	numRAPLEvents = 3

	// RAPL events
	dramEvent    = "dram"
	coreEvent    = "core"
	uncoreEvent  = "uncore"
	packageEvent = "package"
)

var (
	eventPaths map[string]map[string]string
)

func init() {
	eventPaths = map[string]map[string]string{}
	detectEventPaths()
}

// getEnergy returns the sum of the energy consumption of all sockets for a given event
func getEnergy(event string) (uint64, error) {
	energy := uint64(0)
	if hasEvent(event) {
		energyMap := readEventEnergy(event)
		for _, e := range energyMap {
			energy += e
		}
		return energy, nil
	} else {
		switch event {
		case coreEvent:
			packageEnergy := readEventEnergy(packageEvent)
			dramEnergy := readEventEnergy(dramEvent)
			for id, e := range packageEnergy {
				energy += e - dramEnergy[id]
			}
			return energy, nil

		case dramEvent:
			packageEnergy := readEventEnergy(packageEvent)
			coreEnergy := readEventEnergy(coreEvent)
			for id, e := range packageEnergy {
				energy += e - coreEnergy[id]
			}
			return energy, nil
		}
	}

	return energy, fmt.Errorf("could not read RAPL energy for %s", event)
}

func readEventEnergy(eventName string) map[string]uint64 {
	energy := map[string]uint64{}
	for pkId, subTree := range eventPaths {
		for event, path := range subTree {
			if strings.Index(event, eventName) == 0 {
				var e uint64
				var err error
				var data []byte

				if data, err = ioutil.ReadFile(path + energyFile); err != nil {
					log.Println(err)
					continue
				}
				if e, err = strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64); err != nil {
					log.Println(err)
					continue
				}
				e /= 1000 /*mJ*/
				energy[pkId] = e
			}
		}
	}
	return energy
}

type PowerSysfs struct{}

func (r *PowerSysfs) IsSupported() bool {
	path := fmt.Sprintf(packageNamePathTemplate, 0)
	_, err := ioutil.ReadFile(path + energyFile)
	return err == nil
}

func (r *PowerSysfs) GetEnergyFromDram() (uint64, error) {
	return getEnergy(dramEvent)
}

func (r *PowerSysfs) GetEnergyFromCore() (uint64, error) {
	return getEnergy(coreEvent)
}

func (r *PowerSysfs) GetEnergyFromUncore() (uint64, error) {
	return getEnergy(uncoreEvent)
}

func (r *PowerSysfs) GetEnergyFromPackage() (uint64, error) {
	return getEnergy(packageEvent)
}

func (r *PowerSysfs) GetPackageEnergy() map[int]PackageEnergy {
	packageEnergies := make(map[int]PackageEnergy)

	pkgEnergies := readEventEnergy(packageEvent)
	coreEnergies := readEventEnergy(coreEvent)
	dramEnergies := readEventEnergy(dramEvent)
	uncoreEnergies := readEventEnergy(uncoreEvent)
	for pkgId, pkgEnergy := range pkgEnergies {
		coreEnergy, _ := coreEnergies[pkgId]
		dramEnergy, _ := dramEnergies[pkgId]
		uncoreEnergy, _ := uncoreEnergies[pkgId]
		splits := strings.Split(pkgId, "-")
		i, _ := strconv.Atoi(splits[len(splits)-1])
		packageEnergies[i] = PackageEnergy{
			Core: coreEnergy,
			DRAM: dramEnergy,
			Uncore: uncoreEnergy,
			Pkg: pkgEnergy,
		}
	}

	return packageEnergies
}

func (r *PowerSysfs) StopPower() {
}
