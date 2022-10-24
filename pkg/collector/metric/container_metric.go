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

package metric

import (
	"fmt"
	"strconv"
	"strings"

	"k8s.io/klog/v2"
)

var (
	// ContainerMetricNames holds the list of names of the container metric
	ContainerMetricNames []string
	// ContainerFloatFeatureNames holds the feature name of the container float collector_metric. This is specific for the machine-learning based models.
	ContainerFloatFeatureNames []string = []string{}
	// ContainerIOStatMetricsNames holds the cgroup IO metric name
	ContainerIOStatMetricsNames []string = []string{ByteReadLabel, ByteWriteLabel}
	// ContainerUintFeaturesNames holds the feature name of the container utint collector_metric. This is specific for the machine-learning based models.
	ContainerUintFeaturesNames []string
	// ContainerFeaturesNames holds all the feature name of the container collector_metric. This is specific for the machine-learning based models.
	ContainerFeaturesNames []string
)

type ContainerMetrics struct {
	CGroupPID     uint64
	PIDS          []uint64
	ContainerName string
	PodName       string
	Namespace     string
	// TODO: we should consider deprecate the command information
	Command string

	AvgCPUFreq    float64
	CurrProcesses int
	Disks         int

	CPUTime *UInt64Stat

	CounterStats  map[string]*UInt64Stat
	CgroupFSStats map[string]*UInt64StatCollection
	KubeletStats  map[string]*UInt64Stat
	GPUStats      map[string]*UInt64Stat

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

// NewContainerMetrics creates a new ContainerMetrics instance
func NewContainerMetrics(containerName, podName, podNamespace string) *ContainerMetrics {
	c := &ContainerMetrics{
		PodName:       podName,
		ContainerName: containerName,
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
	for _, metricName := range AvailableCounters {
		c.CounterStats[metricName] = &UInt64Stat{}
	}
	for _, metricName := range AvailableCgroupMetrics {
		c.CgroupFSStats[metricName] = &UInt64StatCollection{
			Stat: make(map[string]*UInt64Stat),
		}
	}
	for _, metricName := range AvailableKubeletMetrics {
		c.KubeletStats[metricName] = &UInt64Stat{}
	}
	return c
}

// ResetCurr reset all current value to 0
func (c *ContainerMetrics) ResetCurr() {
	c.CurrProcesses = 0
	c.CPUTime.ResetCurr()
	for counterKey := range c.CounterStats {
		c.CounterStats[counterKey].ResetCurr()
	}
	for cgroupFSKey := range c.CgroupFSStats {
		c.CgroupFSStats[cgroupFSKey].ResetCurr()
	}
	c.BytesRead.ResetCurr()
	c.BytesWrite.ResetCurr()
	for kubeletKey := range c.KubeletStats {
		c.KubeletStats[kubeletKey].ResetCurr()
	}
	c.CurrCPUTimePerCPU = make(map[uint32]uint64)
	c.EnergyInCore.ResetCurr()
	c.EnergyInDRAM.ResetCurr()
	c.EnergyInUncore.ResetCurr()
	c.EnergyInPkg.ResetCurr()
	c.EnergyInOther.ResetCurr()
	c.EnergyInGPU.ResetCurr()
	c.DynEnergy.ResetCurr()
}

// SetLatestProcess set cgroupPID, PID, and command to the latest captured process
// NOTICE: can lose main container info for multi-container pod
func (c *ContainerMetrics) SetLatestProcess(cgroupPID, pid uint64, comm string) {
	c.CGroupPID = cgroupPID
	c.PIDS = append(c.PIDS, pid)
	c.Command = comm
}

// extractFloatCurrAggr return curr, aggr float64 values of specific uint metric
func (c *ContainerMetrics) extractFloatCurrAggr(metric string) (curr, aggr float64, err error) {
	// TO-ADD
	return 0, 0, nil
}

// extractUIntCurrAggr return curr, aggr uint64 values of specific uint metric
func (c *ContainerMetrics) extractUIntCurrAggr(metric string) (curr, aggr uint64, err error) {
	if val, exists := c.CounterStats[metric]; exists {
		return val.Curr, val.Aggr, nil
	}
	if val, exists := c.CgroupFSStats[metric]; exists {
		return val.Curr(), val.Aggr(), nil
	}
	if val, exists := c.KubeletStats[metric]; exists {
		return val.Curr, val.Aggr, nil
	}

	switch metric {
	case CPUTimeLabel:
		return c.CPUTime.Curr, c.CPUTime.Aggr, nil
	// hardcode cgroup metrics
	// TO-DO: merge to cgroup stat
	case ByteReadLabel:
		return c.BytesRead.Curr(), c.BytesRead.Aggr(), nil
	case ByteWriteLabel:
		return c.BytesWrite.Curr(), c.BytesWrite.Aggr(), nil
	}

	klog.V(4).Infof("cannot extract: %s", metric)
	return 0, 0, fmt.Errorf("cannot extract: %s", metric)
}

// ToEstimatorValues return values regarding metricNames
func (c *ContainerMetrics) ToEstimatorValues() (values []float64) {
	for _, metric := range ContainerFloatFeatureNames {
		curr, _, _ := c.extractFloatCurrAggr(metric)
		values = append(values, curr)
	}
	for _, metric := range ContainerUintFeaturesNames {
		curr, _, _ := c.extractUIntCurrAggr(metric)
		values = append(values, float64(curr))
	}
	// TO-DO: remove this hard code metric
	values = append(values, float64(c.Disks))
	return
}

// GetBasicValues return basic label balues
func (c *ContainerMetrics) GetBasicValues() []string {
	command := c.Command
	if len(command) > 10 {
		command = command[:10]
	}
	return []string{c.PodName, c.Namespace, command}
}

// ToPrometheusValue return the value regarding metric label
func (c *ContainerMetrics) ToPrometheusValue(metric string) string {
	currentValue := false
	if strings.Contains(metric, "curr_") {
		currentValue = true
		metric = strings.ReplaceAll(metric, "curr_", "")
	}
	metric = strings.ReplaceAll(metric, "total_", "")

	if curr, aggr, err := c.extractUIntCurrAggr(metric); err == nil {
		if currentValue {
			return strconv.FormatUint(curr, 10)
		}
		return strconv.FormatUint(aggr, 10)
	}
	if metric == "block_devices_used" {
		return strconv.FormatUint(uint64(c.Disks), 10)
	}
	if metric == "avg_cpu_frequency" {
		return fmt.Sprintf("%f", c.AvgCPUFreq)
	}
	if curr, aggr, err := c.extractFloatCurrAggr(metric); err == nil {
		if currentValue {
			return fmt.Sprintf("%f", curr)
		}
		return fmt.Sprintf("%f", aggr)
	}
	klog.Errorf("cannot extract metric: %s", metric)
	return ""
}

func (c *ContainerMetrics) GetPrometheusEnergyValue(ekey string, curr bool) float64 {
	var val *UInt64Stat
	switch ekey {
	case "core":
		val = c.EnergyInCore
	case "dram":
		val = c.EnergyInDRAM
	case "uncore":
		val = c.EnergyInUncore
	case "pkg":
		val = c.EnergyInPkg
	case "gpu":
		val = c.EnergyInGPU
	case "other":
		val = c.EnergyInOther
	}
	if curr {
		return float64(val.Curr)
	}
	return float64(val.Aggr)
}

func (c *ContainerMetrics) Curr() uint64 {
	return c.EnergyInPkg.Curr + c.EnergyInGPU.Curr + c.EnergyInOther.Curr
}

func (c *ContainerMetrics) Aggr() uint64 {
	return c.EnergyInPkg.Aggr + c.EnergyInGPU.Aggr + c.EnergyInOther.Aggr
}

func (c *ContainerMetrics) String() string {
	return fmt.Sprintf("energy from pod (%d processes): name: %s namespace: %s \n"+
		"\tcgrouppid: %d pid: %d comm: %s\n"+
		"\tePkg (mJ): %s (eCore: %s eDram: %s eUncore: %s) eGPU (mJ): %s eOther (mJ): %s \n"+
		"\teDyn (mJ): %s \n"+
		"\tavgFreq: %.2f\n"+
		"\tCPUTime:  %d (%d)\n"+
		"\tcounters: %v\n"+
		"\tcgroupfs: %v\n"+
		"\tkubelets: %v\n",
		c.CurrProcesses, c.PodName, c.Namespace,
		c.CGroupPID, c.PIDS, c.Command,
		c.EnergyInPkg, c.EnergyInCore, c.EnergyInDRAM, c.EnergyInUncore, c.EnergyInOther, c.EnergyInGPU,
		c.DynEnergy,
		c.AvgCPUFreq/1000, /*MHZ*/
		c.CPUTime.Curr, c.CPUTime.Aggr,
		c.CounterStats,
		c.CgroupFSStats,
		c.KubeletStats)
}
