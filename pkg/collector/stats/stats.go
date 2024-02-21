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

import (
	"fmt"
	"strings"

	"github.com/sustainable-computing-io/kepler/pkg/bpfassets/attacher"
	"github.com/sustainable-computing-io/kepler/pkg/collector/stats/types"
	"github.com/sustainable-computing-io/kepler/pkg/config"
	"github.com/sustainable-computing-io/kepler/pkg/sensors/accelerator/gpu"
	"k8s.io/klog/v2"
)

var (
	// AvailableBPFSWCounters holds a list of eBPF counters that might be collected
	AvailableBPFSWCounters []string
	// AvailableBPFHWCounters holds a list of hardware counters that might be collected
	AvailableBPFHWCounters []string
	// AvailableCGroupMetrics holds a list of cgroup metrics exposed by the cgroup that might be collected
	AvailableCGroupMetrics []string
	// AvailableAbsEnergyMetrics holds a list of absolute energy metrics
	AvailableAbsEnergyMetrics []string
	// AvailableDynEnergyMetrics holds a list of dynamic energy metrics
	AvailableDynEnergyMetrics []string
	// AvailableIdleEnergyMetrics holds a list of idle energy metrics
	AvailableIdleEnergyMetrics []string
	// CPUHardwareCounterEnabled defined if hardware counters should be accounted and exported
	CPUHardwareCounterEnabled = false
)

type Stats struct {
	ResourceUsage map[string]*types.UInt64StatCollection
	EnergyUsage   map[string]*types.UInt64StatCollection
}

// NewStats creates a new Stats instance
func NewStats() *Stats {
	m := &Stats{
		ResourceUsage: make(map[string]*types.UInt64StatCollection),
		EnergyUsage:   make(map[string]*types.UInt64StatCollection),
	}

	// initialize the energy metrics in the map
	energyMetrics := []string{}
	energyMetrics = append(energyMetrics, AvailableDynEnergyMetrics...)
	energyMetrics = append(energyMetrics, AvailableAbsEnergyMetrics...)
	energyMetrics = append(energyMetrics, AvailableIdleEnergyMetrics...)
	for _, metricName := range energyMetrics {
		m.EnergyUsage[metricName] = types.NewUInt64StatCollection()
	}

	// initialize the resource utilization metrics in the map
	resMetrics := []string{}
	resMetrics = append(resMetrics, AvailableBPFHWCounters...)
	resMetrics = append(resMetrics, AvailableBPFSWCounters...)
	// CGroup metrics are deprecated, it will be removed in the future
	resMetrics = append(resMetrics, AvailableCGroupMetrics...)
	for _, metricName := range resMetrics {
		m.ResourceUsage[metricName] = types.NewUInt64StatCollection()
	}

	if config.EnabledGPU && gpu.IsGPUCollectionSupported() {
		m.ResourceUsage[config.GPUComputeUtilization] = types.NewUInt64StatCollection()
		m.ResourceUsage[config.GPUMemUtilization] = types.NewUInt64StatCollection()
	}

	if config.IsExposeQATMetricsEnabled() {
		m.ResourceUsage[config.QATUtilization] = types.NewUInt64StatCollection()
	}

	if config.IsExposeCPUFrequencyMetricsEnabled() && attacher.HardwareCountersEnabled {
		m.ResourceUsage[config.CPUFrequency] = types.NewUInt64StatCollection()
	}

	return m
}

// ResetDeltaValues reset all current value to 0
func (m *Stats) ResetDeltaValues() {
	for _, stat := range m.ResourceUsage {
		stat.ResetDeltaValues()
	}
	for metric, stat := range m.EnergyUsage {
		if strings.Contains(metric, "idle") {
			continue // do not reset the idle power metrics
		}
		stat.ResetDeltaValues()
	}
}

func (m *Stats) String() string {
	return fmt.Sprintf(
		"\tDyn ePkg (mJ): %s (eCore: %s eDram: %s eUncore: %s) eGPU (mJ): %s eOther (mJ): %s platform (mJ): %s \n"+
			"\tIdle ePkg (mJ): %s (eCore: %s eDram: %s eUncore: %s) eGPU (mJ): %s eOther (mJ): %s platform (mJ): %s \n"+
			"\tResUsage: %v\n",
		m.EnergyUsage[config.DynEnergyInPkg],
		m.EnergyUsage[config.DynEnergyInCore],
		m.EnergyUsage[config.DynEnergyInDRAM],
		m.EnergyUsage[config.DynEnergyInUnCore],
		m.EnergyUsage[config.DynEnergyInGPU],
		m.EnergyUsage[config.DynEnergyInOther],
		m.EnergyUsage[config.DynEnergyInPlatform],
		m.EnergyUsage[config.IdleEnergyInPkg],
		m.EnergyUsage[config.IdleEnergyInCore],
		m.EnergyUsage[config.IdleEnergyInDRAM],
		m.EnergyUsage[config.IdleEnergyInUnCore],
		m.EnergyUsage[config.IdleEnergyInGPU],
		m.EnergyUsage[config.IdleEnergyInOther],
		m.EnergyUsage[config.IdleEnergyInPlatform],
		m.ResourceUsage)
}

// UpdateDynEnergy calculates the dynamic energy
func (m *Stats) UpdateDynEnergy() {
	for pkgID := range m.EnergyUsage[config.AbsEnergyInPkg].Stat {
		m.CalcDynEnergy(config.AbsEnergyInPkg, config.IdleEnergyInPkg, config.DynEnergyInPkg, pkgID)
		m.CalcDynEnergy(config.AbsEnergyInCore, config.IdleEnergyInCore, config.DynEnergyInCore, pkgID)
		m.CalcDynEnergy(config.AbsEnergyInUnCore, config.IdleEnergyInUnCore, config.DynEnergyInUnCore, pkgID)
		m.CalcDynEnergy(config.AbsEnergyInDRAM, config.IdleEnergyInDRAM, config.DynEnergyInDRAM, pkgID)
	}
	for sensorID := range m.EnergyUsage[config.AbsEnergyInPlatform].Stat {
		m.CalcDynEnergy(config.AbsEnergyInPlatform, config.IdleEnergyInPlatform, config.DynEnergyInPlatform, sensorID)
	}
	// gpu metric
	if config.EnabledGPU && gpu.IsGPUCollectionSupported() {
		for gpuID := range m.EnergyUsage[config.AbsEnergyInGPU].Stat {
			m.CalcDynEnergy(config.AbsEnergyInGPU, config.IdleEnergyInGPU, config.DynEnergyInGPU, gpuID)
		}
	}
}

// CalcDynEnergy calculate the difference between the absolute and idle energy/power
func (m *Stats) CalcDynEnergy(absM, idleM, dynM, id string) {
	if _, exist := m.EnergyUsage[absM].Stat[id]; !exist {
		return
	}
	totalPower := m.EnergyUsage[absM].Stat[id].Delta
	klog.V(6).Infof("Absolute Energy stat: %v (%s)", m.EnergyUsage[absM].Stat, id)
	idlePower := uint64(0)
	if idleStat, found := m.EnergyUsage[idleM].Stat[id]; found {
		idlePower = idleStat.Delta
	}
	dynPower := calcDynEnergy(totalPower, idlePower)
	m.EnergyUsage[dynM].SetDeltaStat(id, dynPower)
}

func calcDynEnergy(totalE, idleE uint64) uint64 {
	if (totalE == 0) || (idleE == 0) || (totalE < idleE) {
		return 0
	}
	return totalE - idleE
}

func normalize(val float64, shouldNormalize bool) float64 {
	if shouldNormalize {
		return val / float64(config.SamplePeriodSec)
	}
	return val
}

// ToEstimatorValues return values regarding metricNames.
// The metrics can be related to resource utilization or power consumption.
// Since Kepler collects metrics at intervals of SamplePeriodSec, which is greater than 1 second, and the power models are trained to estimate power in 1 second interval,
// it is necessary to normalize the resource utilization by the SamplePeriodSec. Note that this is important because the power curve can be different for higher or lower resource usage within 1 second interval.
func (m *Stats) ToEstimatorValues(featuresName []string, shouldNormalize bool) []float64 {
	featureValues := []float64{}
	for _, feature := range featuresName {
		// verify all metrics that are part of the node resource usage metrics
		if value, exists := m.ResourceUsage[feature]; exists {
			featureValues = append(featureValues, normalize(float64(value.SumAllDeltaValues()), shouldNormalize))
			continue
		}
		// some features are not related to resource utilization, such as power metrics
		switch feature {
		case config.GeneralUsageMetric: // is an empty string for UNCORE and OTHER resource usage
			featureValues = append(featureValues, 0)

		case config.DynEnergyInPkg: // for dynamic PKG power consumption
			value := normalize(float64(m.EnergyUsage[config.DynEnergyInPkg].SumAllDeltaValues()), shouldNormalize)
			featureValues = append(featureValues, value)

		case config.DynEnergyInCore: // for dynamic CORE power consumption
			value := normalize(float64(m.EnergyUsage[config.DynEnergyInCore].SumAllDeltaValues()), shouldNormalize)
			featureValues = append(featureValues, value)

		case config.DynEnergyInDRAM: // for dynamic PKG power consumption
			value := normalize(float64(m.EnergyUsage[config.DynEnergyInDRAM].SumAllDeltaValues()), shouldNormalize)
			featureValues = append(featureValues, value)

		case config.DynEnergyInUnCore: // for dynamic UNCORE power consumption
			value := normalize(float64(m.EnergyUsage[config.DynEnergyInUnCore].SumAllDeltaValues()), shouldNormalize)
			featureValues = append(featureValues, value)

		case config.DynEnergyInOther: // for dynamic OTHER power consumption
			value := normalize(float64(m.EnergyUsage[config.DynEnergyInOther].SumAllDeltaValues()), shouldNormalize)
			featureValues = append(featureValues, value)

		case config.DynEnergyInPlatform: // for dynamic PLATFORM power consumption
			value := normalize(float64(m.EnergyUsage[config.DynEnergyInPlatform].SumAllDeltaValues()), shouldNormalize)
			featureValues = append(featureValues, value)

		case config.DynEnergyInGPU: // for dynamic GPU power consumption
			value := normalize(float64(m.EnergyUsage[config.DynEnergyInGPU].SumAllDeltaValues()), shouldNormalize)
			featureValues = append(featureValues, value)

		case config.IdleEnergyInPkg: // for idle PKG power consumption
			value := normalize(float64(m.EnergyUsage[config.IdleEnergyInPkg].SumAllDeltaValues()), shouldNormalize)
			featureValues = append(featureValues, value)

		case config.IdleEnergyInCore: // for idle CORE power consumption
			value := normalize(float64(m.EnergyUsage[config.IdleEnergyInCore].SumAllDeltaValues()), shouldNormalize)
			featureValues = append(featureValues, value)

		case config.IdleEnergyInDRAM: // for idle PKG power consumption
			value := normalize(float64(m.EnergyUsage[config.IdleEnergyInDRAM].SumAllDeltaValues()), shouldNormalize)
			featureValues = append(featureValues, value)

		case config.IdleEnergyInUnCore: // for idle UNCORE power consumption
			value := normalize(float64(m.EnergyUsage[config.IdleEnergyInUnCore].SumAllDeltaValues()), shouldNormalize)
			featureValues = append(featureValues, value)

		case config.IdleEnergyInOther: // for idle OTHER power consumption
			value := normalize(float64(m.EnergyUsage[config.IdleEnergyInOther].SumAllDeltaValues()), shouldNormalize)
			featureValues = append(featureValues, value)

		case config.IdleEnergyInPlatform: // for idle PLATFORM power consumption
			value := normalize(float64(m.EnergyUsage[config.IdleEnergyInPlatform].SumAllDeltaValues()), shouldNormalize)
			featureValues = append(featureValues, value)

		case config.IdleEnergyInGPU: // for idle GPU power consumption
			value := normalize(float64(m.EnergyUsage[config.IdleEnergyInGPU].SumAllDeltaValues()), shouldNormalize)
			featureValues = append(featureValues, value)

		default:
			klog.V(5).Infof("Unknown node feature: %s, adding 0 value", feature)
			featureValues = append(featureValues, 0)
		}
	}
	return featureValues
}
