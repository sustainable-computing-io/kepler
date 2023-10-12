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

package model

import (
	collector_metric "github.com/sustainable-computing-io/kepler/pkg/collector/metric"
)

// UpdateVMEnergy matches the VM metrics with the process metrics already computed
// TODO: remove data duplication, we don;t need to have a separate map for VM metrics, the processes metrics just need to have the VM ID label.
func UpdateVMEnergy(vmMetrics map[uint64]*collector_metric.VMMetrics, processMetrics map[uint64]*collector_metric.ProcessMetrics) {
	for _, vmmetrics := range vmMetrics {
		for _, procmetrics := range processMetrics {
			if procmetrics.PID != vmmetrics.PID {
				continue
			}

			vmmetrics.BPFStats = procmetrics.BPFStats

			vmmetrics.DynEnergyInCore = procmetrics.DynEnergyInCore
			vmmetrics.DynEnergyInDRAM = procmetrics.DynEnergyInDRAM
			vmmetrics.DynEnergyInUncore = procmetrics.DynEnergyInUncore
			vmmetrics.DynEnergyInPkg = procmetrics.DynEnergyInPkg
			vmmetrics.DynEnergyInGPU = procmetrics.DynEnergyInGPU
			vmmetrics.DynEnergyInOther = procmetrics.DynEnergyInOther
			vmmetrics.DynEnergyInPlatform = procmetrics.DynEnergyInPlatform

			vmmetrics.IdleEnergyInCore = procmetrics.IdleEnergyInCore
			vmmetrics.IdleEnergyInDRAM = procmetrics.IdleEnergyInDRAM
			vmmetrics.IdleEnergyInUncore = procmetrics.IdleEnergyInUncore
			vmmetrics.IdleEnergyInPkg = procmetrics.IdleEnergyInPkg
			vmmetrics.IdleEnergyInGPU = procmetrics.IdleEnergyInGPU
			vmmetrics.IdleEnergyInOther = procmetrics.IdleEnergyInOther
			vmmetrics.IdleEnergyInPlatform = procmetrics.IdleEnergyInPlatform
		}
	}
}
