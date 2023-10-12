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

	"github.com/sustainable-computing-io/kepler/pkg/cgroup"
	"github.com/sustainable-computing-io/kepler/pkg/collector/metric/types"
	"github.com/sustainable-computing-io/kepler/pkg/config"
	"github.com/sustainable-computing-io/kepler/pkg/power/accelerator/gpu"
	"k8s.io/klog/v2"
)

var (
	// ContainerFloatFeatureNames holds the feature name of the container float collector_metric. This is specific for the machine-learning based models.
	ContainerFloatFeatureNames []string = []string{}
	// ContainerUintFeaturesNames holds the feature name of the container utint collector_metric. This is specific for the machine-learning based models.
	ContainerUintFeaturesNames []string = []string{}
	// ContainerFeaturesNames holds all the feature name of the container collector_metric. This is specific for the machine-learning based models.
	ContainerFeaturesNames []string = []string{}
)

type ContainerMetrics struct {
	ProcessMetrics

	CGroupPID     uint64
	PIDS          []uint64
	ContainerName string
	PodName       string
	Namespace     string
	ContainerID   string

	CurrProcesses int

	CgroupStatHandler cgroup.CCgroupStatHandler
	CgroupStatMap     map[string]*types.UInt64StatCollection
	// TODO: kubelet stat metrics is deprecated since it duplicates the cgroup metrics. We will remove it soon.
	KubeletStats map[string]*types.UInt64Stat
}

// NewContainerMetrics creates a new ContainerMetrics instance
func NewContainerMetrics(containerName, podName, podNamespace, containerID string) *ContainerMetrics {
	c := &ContainerMetrics{
		PodName:       podName,
		ContainerName: containerName,
		Namespace:     podNamespace,
		ContainerID:   containerID,
		ProcessMetrics: ProcessMetrics{
			BPFStats:             make(map[string]*types.UInt64Stat),
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
		},
		CgroupStatMap: make(map[string]*types.UInt64StatCollection),
		KubeletStats:  make(map[string]*types.UInt64Stat),
	}
	for _, metricName := range AvailableBPFSWCounters {
		c.BPFStats[metricName] = &types.UInt64Stat{}
	}
	for _, metricName := range AvailableBPFHWCounters {
		c.BPFStats[metricName] = &types.UInt64Stat{}
	}
	// TODO: transparently list the other metrics and do not initialize them when they are not supported, e.g. HC
	if gpu.IsGPUCollectionSupported() {
		c.BPFStats[config.GPUSMUtilization] = &types.UInt64Stat{}
		c.BPFStats[config.GPUMemUtilization] = &types.UInt64Stat{}
	}
	for _, metricName := range AvailableCGroupMetrics {
		c.CgroupStatMap[metricName] = &types.UInt64StatCollection{
			Stat: make(map[string]*types.UInt64Stat),
		}
	}
	for _, metricName := range AvailableKubeletMetrics {
		c.KubeletStats[metricName] = &types.UInt64Stat{}
	}
	return c
}

// ResetCurr reset all current value to 0
func (c *ContainerMetrics) ResetDeltaValues() {
	c.CurrProcesses = 0
	c.PIDS = c.PIDS[:0]
	for counterKey := range c.BPFStats {
		c.BPFStats[counterKey].ResetDeltaValues()
	}
	for cgroupFSKey := range c.CgroupStatMap {
		c.CgroupStatMap[cgroupFSKey].ResetDeltaValues()
	}
	for kubeletKey := range c.KubeletStats {
		c.KubeletStats[kubeletKey].ResetDeltaValues()
	}
	c.DynEnergyInCore.ResetDeltaValues()
	c.DynEnergyInDRAM.ResetDeltaValues()
	c.DynEnergyInUncore.ResetDeltaValues()
	c.DynEnergyInPkg.ResetDeltaValues()
	c.DynEnergyInOther.ResetDeltaValues()
	c.DynEnergyInGPU.ResetDeltaValues()
	c.DynEnergyInPlatform.ResetDeltaValues()
	c.IdleEnergyInCore.ResetDeltaValues()
	c.IdleEnergyInDRAM.ResetDeltaValues()
	c.IdleEnergyInUncore.ResetDeltaValues()
	c.IdleEnergyInPkg.ResetDeltaValues()
	c.IdleEnergyInOther.ResetDeltaValues()
	c.IdleEnergyInGPU.ResetDeltaValues()
	c.IdleEnergyInPlatform.ResetDeltaValues()
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
	if val, exists := c.BPFStats[metric]; exists {
		return val.Delta, val.Aggr, nil
	}
	if val, exists := c.CgroupStatMap[metric]; exists {
		return val.SumAllDeltaValues(), val.SumAllAggrValues(), nil
	}
	if val, exists := c.KubeletStats[metric]; exists {
		return val.Delta, val.Aggr, nil
	}
	klog.V(4).Infof("cannot extract: %s", metric)
	return 0, 0, fmt.Errorf("cannot extract: %s", metric)
}

// ToEstimatorValues return values regarding metricNames.
// Since Kepler collects metrics at intervals of SamplePeriodSec, which is greater than 1 second, and the power models are trained to estimate power in 1 second interval.
// It is necessary to normalize the resource utilization by the SamplePeriodSec. Note that this is important because the power curve can be different for higher or lower resource usage within 1 second interval.
func (c *ContainerMetrics) ToEstimatorValues(featuresName []string, shouldNormalize bool) (values []float64) {
	for _, metric := range featuresName {
		curr, _, _ := c.getIntDeltaAndAggrValue(metric)
		values = append(values, normalize(float64(curr), shouldNormalize))
	}
	return
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
		"\tcgrouppid: %d pid: %d comm: %s containerid:%s\n"+
		"\tDyn ePkg (mJ): %s (eCore: %s eDram: %s eUncore: %s) eGPU (mJ): %s eOther (mJ): %s \n"+
		"\tIdle ePkg (mJ): %s (eCore: %s eDram: %s eUncore: %s) eGPU (mJ): %s eOther (mJ): %s \n"+
		"\tcounters: %v\n"+
		"\tcgroupfs: %v\n"+
		"\tkubelets: %v\n",
		c.CurrProcesses, c.PodName, c.ContainerName, c.Namespace,
		c.CGroupPID, c.PIDS, c.Command, c.ContainerID,
		c.DynEnergyInPkg, c.DynEnergyInCore, c.DynEnergyInDRAM, c.DynEnergyInUncore, c.DynEnergyInGPU, c.DynEnergyInOther,
		c.IdleEnergyInPkg, c.IdleEnergyInCore, c.IdleEnergyInDRAM, c.IdleEnergyInUncore, c.IdleEnergyInGPU, c.IdleEnergyInOther,
		c.BPFStats,
		c.CgroupStatMap,
		c.KubeletStats)
}

func (c *ContainerMetrics) UpdateCgroupMetrics() error {
	if c.CgroupStatHandler == nil {
		return nil
	}
	err := c.CgroupStatHandler.SetCGroupStat(c.ContainerID, c.CgroupStatMap)
	if err != nil {
		klog.V(3).Infof("Error reading cgroup stats for container %s (%s): %v", c.ContainerName, c.ContainerID, err)
	}
	return err
}

func (c *ContainerMetrics) GetDynEnergyStat(component string) (energyStat *types.UInt64Stat) {
	if c == nil {
		return
	}
	switch component {
	case PKG:
		return c.DynEnergyInPkg
	case CORE:
		return c.DynEnergyInCore
	case DRAM:
		return c.DynEnergyInDRAM
	case UNCORE:
		return c.DynEnergyInUncore
	case GPU:
		return c.DynEnergyInGPU
	case OTHER:
		return c.DynEnergyInOther
	case PLATFORM:
		return c.DynEnergyInPlatform
	default:
		klog.Fatalf("DynEnergy component type %s is unknown\n", component)
	}
	return
}

func (c *ContainerMetrics) GetIdleEnergyStat(component string) (energyStat *types.UInt64Stat) {
	switch component {
	case PKG:
		return c.IdleEnergyInPkg
	case CORE:
		return c.IdleEnergyInCore
	case DRAM:
		return c.IdleEnergyInDRAM
	case UNCORE:
		return c.IdleEnergyInUncore
	case GPU:
		return c.IdleEnergyInGPU
	case OTHER:
		return c.IdleEnergyInOther
	case PLATFORM:
		return c.IdleEnergyInPlatform
	default:
		klog.Fatalf("IdleEnergy component type %s is unknown\n", component)
	}
	return
}
