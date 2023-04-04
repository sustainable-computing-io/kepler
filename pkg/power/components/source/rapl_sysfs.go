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
	"os"
	"strconv"
	"strings"

	"k8s.io/klog/v2"
)

const (
	// sysfs path templates
	packageNamePathTemplate = "/sys/class/powercap/intel-rapl/intel-rapl:%d/"
	eventNamePathTemplate   = "/sys/class/powercap/intel-rapl/intel-rapl:%d/intel-rapl:%d:%d/"
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
	}
	return energy, fmt.Errorf("could not read RAPL energy for %s", event)
}

func readEventEnergy(eventName string) map[string]uint64 {
	energy := map[string]uint64{}
	for pkID, subTree := range eventPaths {
		for event, path := range subTree {
			if strings.Index(event, eventName) != 0 {
				continue
			}
			var e uint64
			var err error
			var data []byte

			if data, err = os.ReadFile(path + energyFile); err != nil {
				klog.V(3).Infoln(err)
				continue
			}
			if e, err = strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64); err != nil {
				klog.V(3).Infoln(err)
				continue
			}
			e /= 1000 /*mJ*/
			energy[pkID] = e
		}
	}
	return energy
}

type PowerSysfs struct{}

func (r *PowerSysfs) IsSystemCollectionSupported() bool {
	path := fmt.Sprintf(packageNamePathTemplate, 0)
	_, err := os.ReadFile(path + energyFile)
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

func (r *PowerSysfs) GetNodeComponentsEnergy() map[int]NodeComponentsEnergy {
	packageEnergies := make(map[int]NodeComponentsEnergy)

	pkgEnergies := readEventEnergy(packageEvent)
	coreEnergies := readEventEnergy(coreEvent)
	dramEnergies := readEventEnergy(dramEvent)
	uncoreEnergies := readEventEnergy(uncoreEvent)

	for pkgID, pkgEnergy := range pkgEnergies {
		coreEnergy := coreEnergies[pkgID]
		dramEnergy := dramEnergies[pkgID]
		uncoreEnergy := uncoreEnergies[pkgID]
		splits := strings.Split(pkgID, "-")
		i, _ := strconv.Atoi(splits[len(splits)-1])
		packageEnergies[i] = NodeComponentsEnergy{
			Core:   coreEnergy,
			DRAM:   dramEnergy,
			Uncore: uncoreEnergy,
			Pkg:    pkgEnergy,
		}
	}

	return packageEnergies
}

func (r *PowerSysfs) StopPower() {
}
