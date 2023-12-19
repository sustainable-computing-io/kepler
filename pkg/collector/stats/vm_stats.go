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

package stats

var (
	// VMMetricNames holds the list of names of the vm metric
	VMMetricNames []string
	// VMFloatFeatureNames holds the feature name of the vm float stats. This is specific for the machine-learning based models.
	VMFloatFeatureNames []string = []string{}
	// VMUintFeaturesNames holds the feature name of the vm utint stats. This is specific for the machine-learning based models.
	VMUintFeaturesNames []string
	// VMFeaturesNames holds all the feature name of the vm stats. This is specific for the machine-learning based models.
	VMFeaturesNames []string
)

type VMStats struct {
	Stats
	PID  uint64
	VMID string
}

// NewVMStats creates a new VMStats instance
func NewVMStats(pid uint64, vmID string) *VMStats {
	vm := &VMStats{
		PID:   pid,
		VMID:  vmID,
		Stats: *NewStats(),
	}
	return vm
}

// ResetCurr reset all current value to 0
func (vm *VMStats) ResetDeltaValues() {
	vm.Stats.ResetDeltaValues()
}
