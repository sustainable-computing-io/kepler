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

	"github.com/sustainable-computing-io/kepler/pkg/collector/stats/types"
	"github.com/sustainable-computing-io/kepler/pkg/config"
	acc "github.com/sustainable-computing-io/kepler/pkg/sensors/accelerator"
	"k8s.io/klog/v2"
)

// metricSets stores different sets of metrics for energy and resource usage.
type metricSets struct {
	absEnergyMetrics  []string
	dynEnergyMetrics  []string
	idleEnergyMetrics []string
	bpfMetrics        []string
}

type Stats struct {
	ResourceUsage    map[string]types.UInt64StatCollection
	EnergyUsage      map[string]types.UInt64StatCollection
	availableMetrics *metricSets
}

// newMetricSets initializes and returns a new metricSets instance.
func newMetricSets() *metricSets {
	return &metricSets{
		absEnergyMetrics: []string{
			config.AbsEnergyInCore, config.AbsEnergyInDRAM, config.AbsEnergyInUnCore, config.AbsEnergyInPkg,
			config.AbsEnergyInGPU, config.AbsEnergyInOther, config.AbsEnergyInPlatform,
		},
		dynEnergyMetrics: []string{
			config.DynEnergyInCore, config.DynEnergyInDRAM, config.DynEnergyInUnCore, config.DynEnergyInPkg,
			config.DynEnergyInGPU, config.DynEnergyInOther, config.DynEnergyInPlatform,
		},
		idleEnergyMetrics: []string{
			config.IdleEnergyInCore, config.IdleEnergyInDRAM, config.IdleEnergyInUnCore, config.IdleEnergyInPkg,
			config.IdleEnergyInGPU, config.IdleEnergyInOther, config.IdleEnergyInPlatform,
		},
		bpfMetrics: AvailableBPFMetrics(),
	}
}

// NewStats creates a new Stats instance
func NewStats() *Stats {
	stats := &Stats{
		ResourceUsage:    make(map[string]types.UInt64StatCollection),
		EnergyUsage:      make(map[string]types.UInt64StatCollection),
		availableMetrics: newMetricSets(),
	}

	// Initialize the energy metrics in the map.
	energyMetrics := append([]string{}, stats.AbsEnergyMetrics()...)
	energyMetrics = append(energyMetrics, stats.DynEnergyMetrics()...)
	energyMetrics = append(energyMetrics, stats.IdleEnergyMetrics()...)
	for _, metricName := range energyMetrics {
		stats.EnergyUsage[metricName] = types.NewUInt64StatCollection()
	}

	// initialize the resource utilization metrics in the map
	resMetrics := append([]string{}, AvailableBPFMetrics()...)
	for _, metricName := range resMetrics {
		stats.ResourceUsage[metricName] = types.NewUInt64StatCollection()
	}

	if config.IsGPUEnabled() {
		if acc.GetActiveAcceleratorByType(config.GPU) != nil {
			stats.ResourceUsage[config.GPUComputeUtilization] = types.NewUInt64StatCollection()
			stats.ResourceUsage[config.GPUMemUtilization] = types.NewUInt64StatCollection()
			stats.ResourceUsage[config.IdleEnergyInGPU] = types.NewUInt64StatCollection()
		}
	}

	return stats
}

// ResetDeltaValues resets all current values to 0.
func (s *Stats) ResetDeltaValues() {
	for _, stat := range s.ResourceUsage {
		stat.ResetDeltaValues()
	}
	for metric, stat := range s.EnergyUsage {
		if strings.Contains(metric, "idle") {
			continue // Do not reset the idle power metrics.
		}
		stat.ResetDeltaValues()
	}
}

func (s *Stats) String() string {
	return fmt.Sprintf(
		"\tDyn ePkg (mJ): %s (eCore: %s eDram: %s eUncore: %s) eGPU (mJ): %s eOther (mJ): %s platform (mJ): %s \n"+
			"\tIdle ePkg (mJ): %s (eCore: %s eDram: %s eUncore: %s) eGPU (mJ): %s eOther (mJ): %s platform (mJ): %s \n"+
			"\tResUsage: %v\n",
		s.EnergyUsage[config.DynEnergyInPkg],
		s.EnergyUsage[config.DynEnergyInCore],
		s.EnergyUsage[config.DynEnergyInDRAM],
		s.EnergyUsage[config.DynEnergyInUnCore],
		s.EnergyUsage[config.DynEnergyInGPU],
		s.EnergyUsage[config.DynEnergyInOther],
		s.EnergyUsage[config.DynEnergyInPlatform],
		s.EnergyUsage[config.IdleEnergyInPkg],
		s.EnergyUsage[config.IdleEnergyInCore],
		s.EnergyUsage[config.IdleEnergyInDRAM],
		s.EnergyUsage[config.IdleEnergyInUnCore],
		s.EnergyUsage[config.IdleEnergyInGPU],
		s.EnergyUsage[config.IdleEnergyInOther],
		s.EnergyUsage[config.IdleEnergyInPlatform],
		s.ResourceUsage,
	)
}

// UpdateDynEnergy calculates the dynamic energy.
func (s *Stats) UpdateDynEnergy() {
	for pkgID := range s.EnergyUsage[config.AbsEnergyInPkg] {
		s.CalcDynEnergy(config.AbsEnergyInPkg, config.IdleEnergyInPkg, config.DynEnergyInPkg, pkgID)
		s.CalcDynEnergy(config.AbsEnergyInCore, config.IdleEnergyInCore, config.DynEnergyInCore, pkgID)
		s.CalcDynEnergy(config.AbsEnergyInUnCore, config.IdleEnergyInUnCore, config.DynEnergyInUnCore, pkgID)
		s.CalcDynEnergy(config.AbsEnergyInDRAM, config.IdleEnergyInDRAM, config.DynEnergyInDRAM, pkgID)
	}
	for sensorID := range s.EnergyUsage[config.AbsEnergyInPlatform] {
		s.CalcDynEnergy(config.AbsEnergyInPlatform, config.IdleEnergyInPlatform, config.DynEnergyInPlatform, sensorID)
	}
	// GPU metric
	if config.IsGPUEnabled() {
		if acc.GetActiveAcceleratorByType(config.GPU) != nil {
			for gpuID := range s.EnergyUsage[config.AbsEnergyInGPU] {
				s.CalcDynEnergy(config.AbsEnergyInGPU, config.IdleEnergyInGPU, config.DynEnergyInGPU, gpuID)
			}
		}
	}
}

// CalcDynEnergy calculates the difference between the absolute and idle energy/power.
func (s *Stats) CalcDynEnergy(absM, idleM, dynM, id string) {
	if _, exist := s.EnergyUsage[absM][id]; !exist {
		return
	}
	totalPower := s.EnergyUsage[absM][id].GetDelta()
	klog.V(6).Infof("Absolute Energy stat: %v (%s)", s.EnergyUsage[absM], id)
	idlePower := uint64(0)
	if idleStat, found := s.EnergyUsage[idleM][id]; found {
		idlePower = idleStat.GetDelta()
		klog.V(6).Infof("Idle Energy stat: %v (%s)", s.EnergyUsage[idleM], id)
	}
	dynPower := calcDynEnergy(totalPower, idlePower)
	s.EnergyUsage[dynM].SetDeltaStat(id, dynPower)
	klog.V(6).Infof("Dynamic Energy stat: %v (%s)", s.EnergyUsage[dynM], id)
}

// calcDynEnergy calculates the dynamic energy.
func calcDynEnergy(totalE, idleE uint64) uint64 {
	if (totalE == 0) || (totalE < idleE) {
		return 0
	}
	return totalE - idleE
}

// normalize normalizes the value if required.
func normalize(val float64, shouldNormalize bool) float64 {
	if shouldNormalize {
		return val / float64(config.SamplePeriodSec())
	}
	return val
}

// ToEstimatorValues returns values for the specified metric names, normalized if required.
// The metrics can be related to resource utilization or power consumption.
// Since Kepler collects metrics at intervals of SamplePeriodSec, which is greater than 1 second,
// and the power models are trained to estimate power in 1 second interval. It is necessary to
// normalize the resource utilization by the SamplePeriodSec. This is important because the power
// curve can be different for higher or lower resource usage within 1 second interval.
func (s *Stats) ToEstimatorValues(featuresName []string, shouldNormalize bool) []float64 {
	featureValues := []float64{}
	for _, feature := range featuresName {
		// Verify all metrics that are part of the node resource usage metrics.
		if value, exists := s.ResourceUsage[feature]; exists {
			featureValues = append(featureValues, normalize(float64(value.SumAllDeltaValues()), shouldNormalize))
			continue
		}
		// Some features are not related to resource utilization, such as power metrics.
		switch feature {
		case config.GeneralUsageMetric(): // Is an empty string for UNCORE and OTHER resource usage.
			featureValues = append(featureValues, 0)

		case config.DynEnergyInPkg: // For dynamic PKG power consumption.
			value := normalize(float64(s.EnergyUsage[config.DynEnergyInPkg].SumAllDeltaValues()), shouldNormalize)
			featureValues = append(featureValues, value)

		case config.DynEnergyInCore: // For dynamic CORE power consumption.
			value := normalize(float64(s.EnergyUsage[config.DynEnergyInCore].SumAllDeltaValues()), shouldNormalize)
			featureValues = append(featureValues, value)

		case config.DynEnergyInDRAM: // For dynamic DRAM power consumption.
			value := normalize(float64(s.EnergyUsage[config.DynEnergyInDRAM].SumAllDeltaValues()), shouldNormalize)
			featureValues = append(featureValues, value)

		case config.DynEnergyInUnCore: // For dynamic UNCORE power consumption.
			value := normalize(float64(s.EnergyUsage[config.DynEnergyInUnCore].SumAllDeltaValues()), shouldNormalize)
			featureValues = append(featureValues, value)

		case config.DynEnergyInOther: // For dynamic OTHER power consumption.
			value := normalize(float64(s.EnergyUsage[config.DynEnergyInOther].SumAllDeltaValues()), shouldNormalize)
			featureValues = append(featureValues, value)

		case config.DynEnergyInPlatform: // For dynamic PLATFORM power consumption.
			value := normalize(float64(s.EnergyUsage[config.DynEnergyInPlatform].SumAllDeltaValues()), shouldNormalize)
			featureValues = append(featureValues, value)

		case config.DynEnergyInGPU: // For dynamic GPU power consumption.
			value := normalize(float64(s.EnergyUsage[config.DynEnergyInGPU].SumAllDeltaValues()), shouldNormalize)
			featureValues = append(featureValues, value)

		case config.IdleEnergyInPkg: // For idle PKG power consumption.
			value := normalize(float64(s.EnergyUsage[config.IdleEnergyInPkg].SumAllDeltaValues()), shouldNormalize)
			featureValues = append(featureValues, value)

		case config.IdleEnergyInCore: // For idle CORE power consumption.
			value := normalize(float64(s.EnergyUsage[config.IdleEnergyInCore].SumAllDeltaValues()), shouldNormalize)
			featureValues = append(featureValues, value)

		case config.IdleEnergyInDRAM: // For idle DRAM power consumption.
			value := normalize(float64(s.EnergyUsage[config.IdleEnergyInDRAM].SumAllDeltaValues()), shouldNormalize)
			featureValues = append(featureValues, value)

		case config.IdleEnergyInUnCore: // For idle UNCORE power consumption.
			value := normalize(float64(s.EnergyUsage[config.IdleEnergyInUnCore].SumAllDeltaValues()), shouldNormalize)
			featureValues = append(featureValues, value)

		case config.IdleEnergyInOther: // For idle OTHER power consumption.
			value := normalize(float64(s.EnergyUsage[config.IdleEnergyInOther].SumAllDeltaValues()), shouldNormalize)
			featureValues = append(featureValues, value)

		case config.IdleEnergyInPlatform: // For idle PLATFORM power consumption.
			value := normalize(float64(s.EnergyUsage[config.IdleEnergyInPlatform].SumAllDeltaValues()), shouldNormalize)
			featureValues = append(featureValues, value)

		case config.IdleEnergyInGPU: // For idle GPU power consumption.
			value := normalize(float64(s.EnergyUsage[config.IdleEnergyInGPU].SumAllDeltaValues()), shouldNormalize)
			featureValues = append(featureValues, value)

		default:
			klog.V(10).Infof("Unknown node feature: %s, adding 0 value", feature)
			featureValues = append(featureValues, 0)
		}
	}
	return featureValues
}

func (s *Stats) AbsEnergyMetrics() []string {
	return s.availableMetrics.absEnergyMetrics
}

func (s *Stats) DynEnergyMetrics() []string {
	return s.availableMetrics.dynEnergyMetrics
}

func (s *Stats) IdleEnergyMetrics() []string {
	return s.availableMetrics.idleEnergyMetrics
}

func (s *Stats) BPFMetrics() []string {
	return s.availableMetrics.bpfMetrics
}

func AvailableBPFMetrics() []string {
	metrics := append(config.BPFHwCounters(), config.BPFSwCounters()...)
	return metrics
}
