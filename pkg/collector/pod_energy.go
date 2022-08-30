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
package collector

import (
	"fmt"
	"github.com/sustainable-computing-io/kepler/pkg/attacher"
	"log"
	"strconv"
)

type PodEnergy struct {
	CGroupPID uint64
	PID       uint64
	PodName   string
	Namespace string
	Command   string

	AvgCPUFreq    float64
	CurrProcesses int
	Disks         int

	CPUTime *UInt64Stat

	CounterStats  map[string]*UInt64Stat
	CgroupFSStats map[string]*UInt64StatCollection
	KubeletStats  map[string]*UInt64Stat

	BytesRead  *UInt64StatCollection
	BytesWrite *UInt64StatCollection

	CurrCPUTimePerCPU map[uint32]uint64

	EnergyInCore   *UInt64Stat
	EnergyInDRAM   *UInt64Stat
	EnergyInUncore *UInt64Stat
	EnergyInPkg    *UInt64Stat
	EnergyInGPU    *UInt64Stat
	EnergyInOther  *UInt64Stat

	DynEnergy *UInt64Stat
}

// NewPodEnergy creates a new PodEnergy instance
func NewPodEnergy(podName, podNamespace string) *PodEnergy {
	v := &PodEnergy{
		PodName:       podName,
		Namespace:     podNamespace,
		CPUTime:       &UInt64Stat{},
		CounterStats:  make(map[string]*UInt64Stat),
		CgroupFSStats: make(map[string]*UInt64StatCollection),
		KubeletStats:  make(map[string]*UInt64Stat),
		BytesRead: &UInt64StatCollection{
			Stat: make(map[string]*UInt64Stat),
		},
		BytesWrite: &UInt64StatCollection{
			Stat: make(map[string]*UInt64Stat),
		},
		CurrCPUTimePerCPU: make(map[uint32]uint64),
		EnergyInCore:      &UInt64Stat{},
		EnergyInDRAM:      &UInt64Stat{},
		EnergyInUncore:    &UInt64Stat{},
		EnergyInPkg:       &UInt64Stat{},
		EnergyInOther:     &UInt64Stat{},
		EnergyInGPU:       &UInt64Stat{},
		DynEnergy:         &UInt64Stat{},
	}
	for _, metricName := range availableCounters {
		v.CounterStats[metricName] = &UInt64Stat{}
	}
	for _, metricName := range availableCgroupMetrics {
		v.CgroupFSStats[metricName] = &UInt64StatCollection{
			Stat: make(map[string]*UInt64Stat),
		}
	}
	for _, metricName := range availableKubeletMetrics {
		v.KubeletStats[metricName] = &UInt64Stat{}
	}
	return v
}

// ResetCurr reset all current value to 0
func (v *PodEnergy) ResetCurr() {
	v.CurrProcesses = 0
	v.CPUTime.ResetCurr()
	for counterKey, _ := range v.CounterStats {
		v.CounterStats[counterKey].ResetCurr()
	}
	for cgroupFSKey, _ := range v.CgroupFSStats {
		v.CgroupFSStats[cgroupFSKey].ResetCurr()
	}
	v.BytesRead.ResetCurr()
	v.BytesWrite.ResetCurr()
	for kubeletKey, _ := range v.KubeletStats {
		v.KubeletStats[kubeletKey].ResetCurr()
	}
	v.CurrCPUTimePerCPU = make(map[uint32]uint64)
	v.EnergyInCore.ResetCurr()
	v.EnergyInDRAM.ResetCurr()
	v.EnergyInUncore.ResetCurr()
	v.EnergyInPkg.ResetCurr()
	v.EnergyInOther.ResetCurr()
	v.EnergyInGPU.ResetCurr()
	v.DynEnergy.ResetCurr()
}

// SetLatestProcess set cgroupPID, PID, and command to the latest captured process
// NOTICE: can lose main container info for multi-container pod
func (v *PodEnergy) SetLatestProcess(cgroupPID, pid uint64, comm string) {
	v.CGroupPID = cgroupPID
	v.PID = pid
	v.Command = comm
}

// extractFloatCurrAggr return curr, aggr float64 values of specific uint metric
func (v *PodEnergy) extractFloatCurrAggr(metric string) (float64, float64) {
	// TO-ADD
	return 0, 0
}

// extractUIntCurrAggr return curr, aggr uint64 values of specific uint metric
func (v *PodEnergy) extractUIntCurrAggr(metric string) (uint64, uint64) {
	if val, exists := v.CounterStats[metric]; exists {
		return val.Curr, val.Aggr
	}
	if val, exists := v.CgroupFSStats[metric]; exists {
		return val.Curr(), val.Aggr()
	}
	if val, exists := v.KubeletStats[metric]; exists {
		return val.Curr, val.Aggr
	}

	switch metric {
	case CPU_TIME_LABEL:
		return v.CPUTime.Curr, v.CPUTime.Aggr
	// hardcode cgroup metrics
	// TO-DO: merge to cgroup stat
	case BYTE_READ_LABEL:
		return v.BytesRead.Curr(), v.BytesRead.Aggr()
	case BYTE_WRITE_LABEL:
		return v.BytesWrite.Curr(), v.BytesWrite.Aggr()
	}

	log.Printf("cannot extract: %s", metric)
	return 0, 0
}

// ToEstimatorValues return values regarding metricNames
func (v *PodEnergy) ToEstimatorValues() (values []float64) {
	for _, metric := range FLOAT_FEATURES {
		curr, _ := v.extractFloatCurrAggr(metric)
		values = append(values, float64(curr))
	}
	for _, metric := range uintFeatures {
		curr, _ := v.extractUIntCurrAggr(metric)
		values = append(values, float64(curr))
	}
	// TO-DO: remove this hard code metric
	values = append(values, float64(v.Disks))
	return
}

// GetBasicValues return basic label balues
func (v *PodEnergy) GetBasicValues() []string {
	command := fmt.Sprintf("%s", v.Command)
	if len(command) > 10 {
		command = command[:10]
	}
	return []string{v.PodName, v.Namespace, command}
}

// ToPrometheusValues return values regarding podEnergyLabels
func (v *PodEnergy) ToPrometheusValues() []string {
	valuesInStr := v.GetBasicValues()
	for _, metric := range FLOAT_FEATURES {
		curr, aggr := v.extractFloatCurrAggr(metric)
		valuesInStr = append(valuesInStr, fmt.Sprintf("%f", curr))
		valuesInStr = append(valuesInStr, fmt.Sprintf("%f", aggr))
	}
	for _, metric := range uintFeatures {
		curr, aggr := v.extractUIntCurrAggr(metric)
		valuesInStr = append(valuesInStr, strconv.FormatUint(curr, 10))
		valuesInStr = append(valuesInStr, strconv.FormatUint(aggr, 10))
	}
	if attacher.EnableCPUFreq {
		valuesInStr = append(valuesInStr, fmt.Sprintf("%f", v.AvgCPUFreq))
	}
	// TO-DO: remove this hard code metric
	valuesInStr = append(valuesInStr, strconv.FormatUint(uint64(v.Disks), 10))
	return valuesInStr
}

func (v *PodEnergy) GetPrometheusEnergyValue(ekey string, curr bool) float64 {
	var val *UInt64Stat
	switch ekey {
	case "core":
		val = v.EnergyInCore
	case "dram":
		val = v.EnergyInDRAM
	case "uncore":
		val = v.EnergyInUncore
	case "pkg":
		val = v.EnergyInPkg
	case "gpu":
		val = v.EnergyInGPU
	case "other":
		val = v.EnergyInOther
	}
	if curr {
		return float64(val.Curr)
	}
	return float64(val.Aggr)
}

func (v *PodEnergy) Curr() uint64 {
	return v.EnergyInPkg.Curr + v.EnergyInGPU.Curr + v.EnergyInOther.Curr
}

func (v *PodEnergy) Aggr() uint64 {
	return v.EnergyInPkg.Aggr + v.EnergyInGPU.Aggr + v.EnergyInOther.Aggr
}

func (v PodEnergy) String() string {
	return fmt.Sprintf("energy from pod (%d processes): name: %s namespace: %s \n"+
		"\tcgrouppid: %d pid: %d comm: %s\n"+
		"\tePkg (mJ): %s (eCore: %s eDram: %s eUncore: %s) eGPU (mJ): %s eOther (mJ): %s \n"+
		"\teDyn (mJ): %s \n"+
		"\tavgFreq: %.2f\n"+
		"\tCPUTime:  %d (%d)\n"+
		"\tcounters: %v\n"+
		"\tcgroupfs: %v\n"+
		"\tkubelets: %v\n",
		v.CurrProcesses, v.PodName, v.Namespace,
		v.CGroupPID, v.PID, v.Command,
		v.EnergyInPkg, v.EnergyInCore, v.EnergyInDRAM, v.EnergyInUncore, v.EnergyInOther, v.EnergyInGPU,
		v.DynEnergy,
		v.AvgCPUFreq/1000, /*MHZ*/
		v.CPUTime.Curr, v.CPUTime.Aggr,
		v.CounterStats,
		v.CgroupFSStats,
		v.KubeletStats)
}
