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

var (
	// this variable is used for unit and functional test purpose
	SystemCollectionSupported = false
)

type PowerDummy struct{}

func (r *PowerDummy) IsSystemCollectionSupported() bool {
	return SystemCollectionSupported
}

func (r *PowerDummy) StopPower() {
}

func (r *PowerDummy) GetEnergyFromDram() (uint64, error) {
	return 1, nil
}

func (r *PowerDummy) GetEnergyFromCore() (uint64, error) {
	return 5, nil
}

func (r *PowerDummy) GetEnergyFromUncore() (uint64, error) {
	return 0, nil
}

func (r *PowerDummy) GetEnergyFromPackage() (uint64, error) {
	return 8, nil
}

func (r *PowerDummy) GetNodeComponentsEnergy() map[int]NodeComponentsEnergy {
	componentsEnergies := make(map[int]NodeComponentsEnergy)
	machineSocketID := 0
	componentsEnergies[machineSocketID] = NodeComponentsEnergy{
		Pkg:  8,
		Core: 5,
		DRAM: 1,
	}
	return componentsEnergies
}
