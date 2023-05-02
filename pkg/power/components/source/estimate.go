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
	"runtime"
	"time"
)

type PowerEstimate struct{}

var (
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
