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

package model

type Coeff struct {
	CPUTime       float64
	CPUCycle      float64
	CPUInstr      float64
	MemBackground float64
	MemDynamic    float64
}

var (
	//TODO obtain the coeff via regression
	BareMetalCoeff = Coeff{
		CPUTime:       0.6,
		CPUCycle:      0.2,
		CPUInstr:      0.2,
		MemBackground: 0.5,
		MemDynamic:    0.5,
	}
	// if per counters are not avail on VMs, don't use them
	VMCoeff = Coeff{
		CPUTime:       1.0,
		CPUCycle:      0,
		CPUInstr:      0,
		MemBackground: 1.0,
		MemDynamic:    0,
	}
	RunTimeCoeff Coeff = BareMetalCoeff
)

func SetVMCoeff() {
	RunTimeCoeff = VMCoeff
}

func SetBMCoeff() {
	RunTimeCoeff = BareMetalCoeff
}
