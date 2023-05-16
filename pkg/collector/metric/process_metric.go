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
	"fmt"

	"github.com/sustainable-computing-io/kepler/pkg/bpfassets/attacher"
	"github.com/sustainable-computing-io/kepler/pkg/collector/metric/types"
	"github.com/sustainable-computing-io/kepler/pkg/config"
	"github.com/sustainable-computing-io/kepler/pkg/power/accelerator"
	"k8s.io/klog/v2"
)

var (
	// ProcessMetricNames holds the list of names of the container metric
	ProcessMetricNames []string
	// ProcessFloatFeatureNames holds the feature name of the container float collector_metric. This is specific for the machine-learning based models.
	ProcessFloatFeatureNames []string = []string{}
	// ProcessUintFeaturesNames holds the feature name of the container utint collector_metric. This is specific for the machine-learning based models.
	ProcessUintFeaturesNames []string
	// ProcessFeaturesNames holds all the feature name of the container collector_metric. This is specific for the machine-learning based models.
	ProcessFeaturesNames []string
)

type ProcessMetrics struct {
	PID          uint64
	Command      string
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

// NewProcessMetrics creates a new ProcessMetrics instance
func NewProcessMetrics(pid uint64, command string) *ProcessMetrics {
	p := &ProcessMetrics{
		PID:                  pid,
		Command:              command,
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
	if accelerator.IsGPUCollectionSupported() {
		p.CounterStats[config.GPUSMUtilization] = &types.UInt64Stat{}
		p.CounterStats[config.GPUMemUtilization] = &types.UInt64Stat{}
	}
	return p
}

// ResetCurr reset all current value to 0
func (p *ProcessMetrics) ResetDeltaValues() {
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

// getFloatCurrAndAggrValue return curr, aggr float64 values of specific uint metric
func (p *ProcessMetrics) getFloatCurrAndAggrValue(metric string) (curr, aggr float64, err error) {
	// TO-ADD
	return 0, 0, nil
}

// getIntDeltaAndAggrValue return curr, aggr uint64 values of specific uint metric
func (p *ProcessMetrics) getIntDeltaAndAggrValue(metric string) (curr, aggr uint64, err error) {
	if val, exists := p.CounterStats[metric]; exists {
		return val.Delta, val.Aggr, nil
	}

	switch metric {
	// ebpf metrics
	case config.CPUTime:
		return p.CPUTime.Delta, p.CPUTime.Aggr, nil
	case config.IRQBlockLabel:
		return p.SoftIRQCount[attacher.IRQBlock].Delta, p.SoftIRQCount[attacher.IRQBlock].Aggr, nil
	case config.IRQNetTXLabel:
		return p.SoftIRQCount[attacher.IRQNetTX].Delta, p.SoftIRQCount[attacher.IRQNetTX].Aggr, nil
	case config.IRQNetRXLabel:
		return p.SoftIRQCount[attacher.IRQNetRX].Delta, p.SoftIRQCount[attacher.IRQNetRX].Aggr, nil
	}
	klog.V(4).Infof("cannot extract: %s", metric)
	return 0, 0, fmt.Errorf("cannot extract: %s", metric)
}

// ToEstimatorValues return values regarding metricNames
func (p *ProcessMetrics) ToEstimatorValues() (values []float64) {
	for _, metric := range ContainerFloatFeatureNames {
		curr, _, _ := p.getFloatCurrAndAggrValue(metric)
		values = append(values, curr)
	}
	for _, metric := range ContainerUintFeaturesNames {
		curr, _, _ := p.getIntDeltaAndAggrValue(metric)
		values = append(values, float64(curr))
	}
	return
}

func (p *ProcessMetrics) SumAllDynDeltaValues() uint64 {
	return p.DynEnergyInPkg.Delta + p.DynEnergyInGPU.Delta + p.DynEnergyInOther.Delta
}

func (p *ProcessMetrics) SumAllDynAggrValues() uint64 {
	return p.DynEnergyInPkg.Aggr + p.DynEnergyInGPU.Aggr + p.DynEnergyInOther.Aggr
}

func (p *ProcessMetrics) String() string {
	return fmt.Sprintf("energy from process pid: %d comm: %s\n"+
		"\tDyn ePkg (mJ): %s (eCore: %s eDram: %s eUncore: %s) eGPU (mJ): %s eOther (mJ): %s \n"+
		"\tIdle ePkg (mJ): %s (eCore: %s eDram: %s eUncore: %s) eGPU (mJ): %s eOther (mJ): %s \n"+
		"\tCPUTime:  %d (%d)\n"+
		"\tcounters: %v\n",
		p.PID, p.Command,
		p.DynEnergyInPkg, p.DynEnergyInCore, p.DynEnergyInDRAM, p.DynEnergyInUncore, p.DynEnergyInGPU, p.DynEnergyInOther,
		p.IdleEnergyInPkg, p.IdleEnergyInCore, p.IdleEnergyInDRAM, p.IdleEnergyInUncore, p.IdleEnergyInGPU, p.IdleEnergyInOther,
		p.CPUTime.Delta, p.CPUTime.Aggr,
		p.CounterStats)
}

func (p *ProcessMetrics) GetDynEnergyStat(component string) (energyStat *types.UInt64Stat) {
	switch component {
	case PKG:
		return p.DynEnergyInPkg
	case CORE:
		return p.DynEnergyInCore
	case DRAM:
		return p.DynEnergyInDRAM
	case UNCORE:
		return p.DynEnergyInUncore
	case GPU:
		return p.DynEnergyInGPU
	case OTHER:
		return p.DynEnergyInOther
	case PLATFORM:
		return p.DynEnergyInPlatform
	default:
		klog.Fatalf("DynEnergy component type %s is unknown\n", component)
	}
	return
}

func (p *ProcessMetrics) GetIdleEnergyStat(component string) (energyStat *types.UInt64Stat) {
	switch component {
	case PKG:
		return p.IdleEnergyInPkg
	case CORE:
		return p.IdleEnergyInCore
	case DRAM:
		return p.IdleEnergyInDRAM
	case UNCORE:
		return p.IdleEnergyInUncore
	case GPU:
		return p.IdleEnergyInGPU
	case OTHER:
		return p.IdleEnergyInOther
	case PLATFORM:
		return p.IdleEnergyInPlatform
	default:
		klog.Fatalf("IdleEnergy component type %s is unknown\n", component)
	}
	return
}
