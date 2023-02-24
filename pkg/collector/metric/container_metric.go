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

	"github.com/sustainable-computing-io/kepler/pkg/bpfassets/attacher"
	"github.com/sustainable-computing-io/kepler/pkg/config"
	"github.com/sustainable-computing-io/kepler/pkg/power/accelerator"
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
	ProcessMetrics

	CGroupPID     uint64
	PIDS          []uint64
	ContainerName string
	PodName       string
	Namespace     string

	CurrProcesses int
	Disks         int

	CgroupFSStats map[string]*UInt64StatCollection
	KubeletStats  map[string]*UInt64Stat
	BytesRead     *UInt64StatCollection
	BytesWrite    *UInt64StatCollection
}

// NewContainerMetrics creates a new ContainerMetrics instance
func NewContainerMetrics(containerName, podName, podNamespace string) *ContainerMetrics {
	c := &ContainerMetrics{
		PodName:       podName,
		ContainerName: containerName,
		Namespace:     podNamespace,
		ProcessMetrics: ProcessMetrics{
			CPUTime:            &UInt64Stat{},
			CounterStats:       make(map[string]*UInt64Stat),
			SoftIRQCount:       make([]UInt64Stat, config.MaxIRQ),
			DynEnergyInCore:    &UInt64Stat{},
			DynEnergyInDRAM:    &UInt64Stat{},
			DynEnergyInUncore:  &UInt64Stat{},
			DynEnergyInPkg:     &UInt64Stat{},
			DynEnergyInOther:   &UInt64Stat{},
			DynEnergyInGPU:     &UInt64Stat{},
			IdleEnergyInCore:   &UInt64Stat{},
			IdleEnergyInDRAM:   &UInt64Stat{},
			IdleEnergyInUncore: &UInt64Stat{},
			IdleEnergyInPkg:    &UInt64Stat{},
			IdleEnergyInOther:  &UInt64Stat{},
			IdleEnergyInGPU:    &UInt64Stat{},
		},
		CgroupFSStats: make(map[string]*UInt64StatCollection),
		KubeletStats:  make(map[string]*UInt64Stat),
		BytesRead: &UInt64StatCollection{
			Stat: make(map[string]*UInt64Stat),
		},
		BytesWrite: &UInt64StatCollection{
			Stat: make(map[string]*UInt64Stat),
		},
	}
	for _, metricName := range AvailableHWCounters {
		c.CounterStats[metricName] = &UInt64Stat{}
	}
	// TODO: transparently list the other metrics and do not initialize them when they are not supported, e.g. HC
	if accelerator.IsGPUCollectionSupported() {
		c.CounterStats[config.GPUSMUtilization] = &UInt64Stat{}
		c.CounterStats[config.GPUMemUtilization] = &UInt64Stat{}
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
func (c *ContainerMetrics) ResetDeltaValues() {
	c.CurrProcesses = 0
	c.CPUTime.ResetDeltaValues()
	for i := 0; i < config.MaxIRQ; i++ {
		c.SoftIRQCount[i].ResetDeltaValues()
	}
	for counterKey := range c.CounterStats {
		c.CounterStats[counterKey].ResetDeltaValues()
	}
	for cgroupFSKey := range c.CgroupFSStats {
		c.CgroupFSStats[cgroupFSKey].ResetDeltaValues()
	}
	c.BytesRead.ResetDeltaValues()
	c.BytesWrite.ResetDeltaValues()
	for kubeletKey := range c.KubeletStats {
		c.KubeletStats[kubeletKey].ResetDeltaValues()
	}
	c.DynEnergyInCore.ResetDeltaValues()
	c.DynEnergyInDRAM.ResetDeltaValues()
	c.DynEnergyInUncore.ResetDeltaValues()
	c.DynEnergyInPkg.ResetDeltaValues()
	c.DynEnergyInOther.ResetDeltaValues()
	c.DynEnergyInGPU.ResetDeltaValues()
	c.IdleEnergyInCore.ResetDeltaValues()
	c.IdleEnergyInDRAM.ResetDeltaValues()
	c.IdleEnergyInUncore.ResetDeltaValues()
	c.IdleEnergyInPkg.ResetDeltaValues()
	c.IdleEnergyInOther.ResetDeltaValues()
	c.IdleEnergyInGPU.ResetDeltaValues()
}

// SetLatestProcess set cgroupPID, PID, and command to the latest captured process
// NOTICE: can lose main container info for multi-container pod
func (c *ContainerMetrics) SetLatestProcess(cgroupPID, pid uint64, comm string) {
	c.CGroupPID = cgroupPID

	// TODO: review if we can remove the PIDS list as it's for GPU consumption and likely will remove this dependency
	notexist := true
	for _, v := range c.PIDS {
		if v == pid {
			notexist = false
		}
	}
	if notexist {
		c.PIDS = append(c.PIDS, pid)
	}

	c.Command = comm
}

// getFloatCurrAndAggrValue return curr, aggr float64 values of specific uint metric
func (c *ContainerMetrics) getFloatCurrAndAggrValue(metric string) (curr, aggr float64, err error) {
	// TO-ADD
	return 0, 0, nil
}

// getIntDeltaAndAggrValue return curr, aggr uint64 values of specific uint metric
func (c *ContainerMetrics) getIntDeltaAndAggrValue(metric string) (curr, aggr uint64, err error) {
	if val, exists := c.CounterStats[metric]; exists {
		return val.Delta, val.Aggr, nil
	}
	if val, exists := c.CgroupFSStats[metric]; exists {
		return val.SumAllDeltaValues(), val.SumAllAggrValues(), nil
	}
	if val, exists := c.KubeletStats[metric]; exists {
		return val.Delta, val.Aggr, nil
	}

	switch metric {
	// ebpf metrics
	case config.CPUTime:
		return c.CPUTime.Delta, c.CPUTime.Aggr, nil
	case config.IRQBlockLabel:
		return c.SoftIRQCount[attacher.IRQBlock].Delta, c.SoftIRQCount[attacher.IRQBlock].Aggr, nil
	case config.IRQNetTXLabel:
		return c.SoftIRQCount[attacher.IRQNetTX].Delta, c.SoftIRQCount[attacher.IRQNetTX].Aggr, nil
	case config.IRQNetRXLabel:
		return c.SoftIRQCount[attacher.IRQNetRX].Delta, c.SoftIRQCount[attacher.IRQNetRX].Aggr, nil
	// hardcode cgroup metrics
	// TO-DO: merge to cgroup stat
	case config.BlockDevicesIO:
		return uint64(c.Disks), uint64(c.Disks), nil
	case ByteReadLabel:
		return c.BytesRead.SumAllDeltaValues(), c.BytesRead.SumAllAggrValues(), nil
	case ByteWriteLabel:
		return c.BytesWrite.SumAllDeltaValues(), c.BytesWrite.SumAllAggrValues(), nil
	}

	klog.V(4).Infof("cannot extract: %s", metric)
	return 0, 0, fmt.Errorf("cannot extract: %s", metric)
}

// ToEstimatorValues return values regarding metricNames
func (c *ContainerMetrics) ToEstimatorValues() (values []float64) {
	for _, metric := range ContainerFloatFeatureNames {
		curr, _, _ := c.getFloatCurrAndAggrValue(metric)
		values = append(values, curr)
	}
	for _, metric := range ContainerUintFeaturesNames {
		curr, _, _ := c.getIntDeltaAndAggrValue(metric)
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

	if curr, aggr, err := c.getIntDeltaAndAggrValue(metric); err == nil {
		if currentValue {
			return strconv.FormatUint(curr, 10)
		}
		return strconv.FormatUint(aggr, 10)
	}
	if metric == "block_devices_used" {
		return strconv.FormatUint(uint64(c.Disks), 10)
	}
	if curr, aggr, err := c.getFloatCurrAndAggrValue(metric); err == nil {
		if currentValue {
			return fmt.Sprintf("%f", curr)
		}
		return fmt.Sprintf("%f", aggr)
	}
	klog.Errorf("cannot extract metric: %s", metric)
	return ""
}

func (c *ContainerMetrics) SumAllDynDeltaValues() uint64 {
	return c.DynEnergyInPkg.Delta + c.DynEnergyInGPU.Delta + c.DynEnergyInOther.Delta
}

func (c *ContainerMetrics) SumAllDynAggrValues() uint64 {
	return c.DynEnergyInPkg.Aggr + c.DynEnergyInGPU.Aggr + c.DynEnergyInOther.Aggr
}

func (c *ContainerMetrics) String() string {
	return fmt.Sprintf("energy from pod/container (%d active processes): name: %s/%s namespace: %s \n"+
		"\tcgrouppid: %d pid: %d comm: %s\n"+
		"\tDyn ePkg (mJ): %s (eCore: %s eDram: %s eUncore: %s) eGPU (mJ): %s eOther (mJ): %s \n"+
		"\tIdle ePkg (mJ): %s (eCore: %s eDram: %s eUncore: %s) eGPU (mJ): %s eOther (mJ): %s \n"+
		"\tCPUTime:  %d (%d)\n"+
		"\tNetTX IRQ: %d (%d)\n"+
		"\tNetRX IRQ: %d (%d)\n"+
		"\tBlock IRQ: %d (%d)\n"+
		"\tcounters: %v\n"+
		"\tcgroupfs: %v\n"+
		"\tkubelets: %v\n",
		c.CurrProcesses, c.PodName, c.ContainerName, c.Namespace,
		c.CGroupPID, c.PIDS, c.Command,
		c.DynEnergyInPkg, c.DynEnergyInCore, c.DynEnergyInDRAM, c.DynEnergyInUncore, c.DynEnergyInGPU, c.DynEnergyInOther,
		c.IdleEnergyInPkg, c.IdleEnergyInCore, c.IdleEnergyInDRAM, c.IdleEnergyInUncore, c.IdleEnergyInGPU, c.IdleEnergyInOther,
		c.CPUTime.Delta, c.CPUTime.Aggr,
		c.SoftIRQCount[attacher.IRQNetTX].Delta, c.SoftIRQCount[attacher.IRQNetTX].Aggr,
		c.SoftIRQCount[attacher.IRQNetRX].Delta, c.SoftIRQCount[attacher.IRQNetRX].Aggr,
		c.SoftIRQCount[attacher.IRQBlock].Delta, c.SoftIRQCount[attacher.IRQBlock].Aggr,
		c.CounterStats,
		c.CgroupFSStats,
		c.KubeletStats)
}
