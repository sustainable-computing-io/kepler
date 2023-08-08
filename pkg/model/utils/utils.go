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

package utils

import (
	"github.com/sustainable-computing-io/kepler/pkg/power/components/source"
)

const (
	jouleToMiliJoule = 1000
)

// GetComponentPower called by getPodComponentPowers to check if component key is present in powers response and fills with single 0
func GetComponentPower(powers map[string][]float64, componentKey string, index int) uint64 {
	values := powers[componentKey]
	if index >= len(values) {
		return 0
	} else {
		return uint64(values[index] * jouleToMiliJoule)
	}
}

// FillNodeComponentsPower fills missing component (pkg or core) power
func FillNodeComponentsPower(pkgPower, corePower, uncorePower, dramPower uint64) source.NodeComponentsEnergy {
	if pkgPower < corePower+uncorePower {
		pkgPower = corePower + uncorePower
	}
	if corePower == 0 {
		corePower = pkgPower - uncorePower
	}
	return source.NodeComponentsEnergy{
		Core:   corePower,
		Uncore: uncorePower,
		DRAM:   dramPower,
		Pkg:    pkgPower,
	}
}
