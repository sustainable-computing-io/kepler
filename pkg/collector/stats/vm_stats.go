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

import "github.com/sustainable-computing-io/kepler/pkg/bpf"

type VMStats struct {
	Stats
	PID  uint64
	VMID string
}

// NewVMStats creates a new VMStats instance
func NewVMStats(pid uint64, vmID string, bpfSupportedMetrics bpf.SupportedMetrics) *VMStats {
	vm := &VMStats{
		PID:   pid,
		VMID:  vmID,
		Stats: *NewStats(bpfSupportedMetrics),
	}
	return vm
}

// ResetCurr reset all current value to 0
func (vm *VMStats) ResetDeltaValues() {
	vm.Stats.ResetDeltaValues()
}
