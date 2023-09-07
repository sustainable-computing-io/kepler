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

package metric

import (
	"github.com/sustainable-computing-io/kepler/pkg/collector/metric/types"
	"github.com/sustainable-computing-io/kepler/pkg/config"
	"github.com/sustainable-computing-io/kepler/pkg/power/accelerator/gpu"
)

var (
	// VMMetricNames holds the list of names of the vm metric
	VMMetricNames []string
	// VMFloatFeatureNames holds the feature name of the vm float collector_metric. This is specific for the machine-learning based models.
	VMFloatFeatureNames []string = []string{}
	// VMUintFeaturesNames holds the feature name of the vm utint collector_metric. This is specific for the machine-learning based models.
	VMUintFeaturesNames []string
	// VMFeaturesNames holds all the feature name of the vm collector_metric. This is specific for the machine-learning based models.
	VMFeaturesNames []string
)

type VMMetrics struct {
	PID          uint64
	Name         string
	CounterStats map[string]*types.UInt64Stat
	// ebpf metrics
	CPUTime             *types.UInt64Stat
	SoftIRQCount        []types.UInt64Stat
	DynEnergyInCore     *types.UInt64Stat
	DynEnergyInDRAM     *types.UInt64Stat
	DynEnergyInUncore   *types.UInt64Stat
	DynEnergyInPkg      *types.UInt64Stat
	DynEnergyInGPU      *types.UInt64Stat
	DynEnergyInOther    *types.UInt64Stat
	DynEnergyInPlatform *types.UInt64Stat

	IdleEnergyInCore     *types.UInt64Stat
	IdleEnergyInDRAM     *types.UInt64Stat
	IdleEnergyInUncore   *types.UInt64Stat
	IdleEnergyInPkg      *types.UInt64Stat
	IdleEnergyInGPU      *types.UInt64Stat
	IdleEnergyInOther    *types.UInt64Stat
	IdleEnergyInPlatform *types.UInt64Stat
}

// NewVMMetrics creates a new VMMetrics instance
func NewVMMetrics(pid uint64, name string) *VMMetrics {
	p := &VMMetrics{
		PID:                  pid,
		Name:                 name,
		CPUTime:              &types.UInt64Stat{},
		CounterStats:         make(map[string]*types.UInt64Stat),
		SoftIRQCount:         make([]types.UInt64Stat, config.MaxIRQ),
		DynEnergyInCore:      &types.UInt64Stat{},
		DynEnergyInDRAM:      &types.UInt64Stat{},
		DynEnergyInUncore:    &types.UInt64Stat{},
		DynEnergyInPkg:       &types.UInt64Stat{},
		DynEnergyInOther:     &types.UInt64Stat{},
		DynEnergyInGPU:       &types.UInt64Stat{},
		DynEnergyInPlatform:  &types.UInt64Stat{},
		IdleEnergyInCore:     &types.UInt64Stat{},
		IdleEnergyInDRAM:     &types.UInt64Stat{},
		IdleEnergyInUncore:   &types.UInt64Stat{},
		IdleEnergyInPkg:      &types.UInt64Stat{},
		IdleEnergyInOther:    &types.UInt64Stat{},
		IdleEnergyInGPU:      &types.UInt64Stat{},
		IdleEnergyInPlatform: &types.UInt64Stat{},
	}

	for _, metricName := range AvailableHWCounters {
		p.CounterStats[metricName] = &types.UInt64Stat{}
	}
	// TODO: transparently list the other metrics and do not initialize them when they are not supported, e.g. HC
	if gpu.IsGPUCollectionSupported() {
		p.CounterStats[config.GPUSMUtilization] = &types.UInt64Stat{}
		p.CounterStats[config.GPUMemUtilization] = &types.UInt64Stat{}
	}
	return p
}

// ResetCurr reset all current value to 0
func (p *VMMetrics) ResetDeltaValues() {
	p.CPUTime.ResetDeltaValues()
	for counterKey := range p.CounterStats {
		p.CounterStats[counterKey].ResetDeltaValues()
	}
	for i := 0; i < config.MaxIRQ; i++ {
		p.SoftIRQCount[i].ResetDeltaValues()
	}
	p.DynEnergyInCore.ResetDeltaValues()
	p.DynEnergyInDRAM.ResetDeltaValues()
	p.DynEnergyInUncore.ResetDeltaValues()
	p.DynEnergyInPkg.ResetDeltaValues()
	p.DynEnergyInOther.ResetDeltaValues()
	p.DynEnergyInGPU.ResetDeltaValues()
	p.DynEnergyInPlatform.ResetDeltaValues()
	p.IdleEnergyInCore.ResetDeltaValues()
	p.IdleEnergyInDRAM.ResetDeltaValues()
	p.IdleEnergyInUncore.ResetDeltaValues()
	p.IdleEnergyInPkg.ResetDeltaValues()
	p.IdleEnergyInOther.ResetDeltaValues()
	p.IdleEnergyInGPU.ResetDeltaValues()
	p.IdleEnergyInPlatform.ResetDeltaValues()
}
