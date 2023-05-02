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

type PowerHMC struct{}

func (r *PowerHMC) IsSystemCollectionSupported() bool {
	return false
}

func (r *PowerHMC) StopPower() {
}

func (r *PowerHMC) GetEnergyFromDram() (uint64, error) {
	return 0, nil
}

func (r *PowerHMC) GetEnergyFromCore() (uint64, error) {
	return 0, nil
}

func (r *PowerHMC) GetEnergyFromUncore() (uint64, error) {
	return 0, nil
}

func (r *PowerHMC) GetEnergyFromPackage() (uint64, error) {
	return 0, nil
}

func (r *PowerHMC) GetNodeComponentsEnergy() map[int]NodeComponentsEnergy {
	packageEnergies := make(map[int]NodeComponentsEnergy)
	return packageEnergies
}
