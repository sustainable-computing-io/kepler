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

type PowerMSR struct{}

func (PowerMSR) GetName() string {
	return "rapl-msr"
}

func (r *PowerMSR) IsSystemCollectionSupported() bool {
	return InitUnits() == nil
}

func (r *PowerMSR) GetAbsEnergyFromDram() (uint64, error) {
	return ReadAllPower(ReadDramPower)
}

func (r *PowerMSR) GetAbsEnergyFromCore() (uint64, error) {
	return ReadAllPower(ReadCorePower)
}

func (r *PowerMSR) GetAbsEnergyFromUncore() (uint64, error) {
	return ReadAllPower(ReadUncorePower)
}

func (r *PowerMSR) GetAbsEnergyFromPackage() (uint64, error) {
	return ReadAllPower(ReadPkgPower)
}

func (r *PowerMSR) GetAbsEnergyFromNodeComponents() map[int]NodeComponentsEnergy {
	return GetRAPLEnergyByMSR(ReadCorePower, ReadDramPower, ReadUncorePower, ReadPkgPower)
}

func (r *PowerMSR) StopPower() {
	CloseAllMSR()
}
