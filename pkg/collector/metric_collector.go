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
	"os"
	"sync"
	"syscall"
	"time"

	"github.com/sustainable-computing-io/kepler/pkg/bpf"
	cgroup_api "github.com/sustainable-computing-io/kepler/pkg/cgroup"
	"github.com/sustainable-computing-io/kepler/pkg/collector/energy"
	"github.com/sustainable-computing-io/kepler/pkg/collector/resourceutilization/accelerator"
	bpf_collector "github.com/sustainable-computing-io/kepler/pkg/collector/resourceutilization/bpf"
	cgroup_collector "github.com/sustainable-computing-io/kepler/pkg/collector/resourceutilization/cgroup"
	"github.com/sustainable-computing-io/kepler/pkg/collector/stats"
	"github.com/sustainable-computing-io/kepler/pkg/config"
	"github.com/sustainable-computing-io/kepler/pkg/model"
	"github.com/sustainable-computing-io/kepler/pkg/sensors/accelerator/gpu"
	"github.com/sustainable-computing-io/kepler/pkg/sensors/accelerator/qat"
	"github.com/sustainable-computing-io/kepler/pkg/utils"

	"k8s.io/klog/v2"
)

const (
	maxInactiveContainers        = 10
	maxInactiveVM                = 3
	procPath              string = "/proc/%d/cgroup"
)

type Collector struct {
	// NodeStats holds all node energy and resource usage metrics
	NodeStats stats.NodeStats

	// ProcessStats hold all process energy and resource usage metrics
	ProcessStats map[uint64]*stats.ProcessStats

	// ContainerStats holds the aggregated processes metrics for all containers
	ContainerStats map[string]*stats.ContainerStats

	// VMStats holds the aggregated processes metrics for all virtual machines
	VMStats map[string]*stats.VMStats

	// bpfExporter handles gathering metrics from bpf probes
	bpfExporter bpf.Exporter
	// bpfSupportedMetrics holds the supported metrics by the bpf exporter
	bpfSupportedMetrics bpf.SupportedMetrics

	// metricsLock prevents new metrics being added while the collector is
	// updating the metrics
	metricsLock    sync.Mutex
	bpfMetricsChan chan []*bpf.ProcessBPFMetrics
}

func NewCollector(bpfExporter bpf.Exporter) *Collector {
	bpfSupportedMetrics := bpfExporter.SupportedMetrics()
	c := &Collector{
		NodeStats:           *stats.NewNodeStats(bpfSupportedMetrics),
		ContainerStats:      map[string]*stats.ContainerStats{},
		ProcessStats:        map[uint64]*stats.ProcessStats{},
		VMStats:             map[string]*stats.VMStats{},
		bpfExporter:         bpfExporter,
		bpfSupportedMetrics: bpfSupportedMetrics,
		// bpfMetricsChan is a channel to receive the metrics from the
		// bpf exporter. It's exporting once per second. The channel
		// is buffered to allow for the exporter to continue collecting
		// metrics while the collector is processing them.
		bpfMetricsChan: make(chan []*bpf.ProcessBPFMetrics, 100),
	}
	return c
}

func (c *Collector) Start(stop <-chan struct{}) error {
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				klog.Infof("Collector shutdown")
				return
			case r := <-c.bpfMetricsChan:
				c.metricsLock.Lock()
				bpf_collector.UpdateProcessBPFMetrics(r, c.bpfSupportedMetrics, c.ProcessStats)
				c.metricsLock.Unlock()
			}
		}
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		c.bpfExporter.Start(c.bpfMetricsChan, stop)
		klog.Infof("bpfexporter shutdown complete")
	}()
	wg.Wait()
	klog.Infof("Metric collector shutdown done")
	return nil
}

func (c *Collector) Initialize() error {
	if config.IsCgroupMetricsEnabled() {
		_, err := cgroup_api.Init()
		if err != nil && !config.EnableProcessStats {
			klog.V(5).Infoln(err)
			return err
		}
	}

	// For local estimator, there is endpoint provided, thus we should let
	// model component decide whether/how to init
	model.CreatePowerEstimatorModels(
		stats.GetProcessFeatureNames(c.bpfSupportedMetrics),
		stats.NodeMetadataFeatureNames,
		stats.NodeMetadataFeatureValues,
		c.bpfSupportedMetrics,
	)

	return nil
}

// Update updates the node and container energy and resource usage metrics
func (c *Collector) Update() {
	start := time.Now()

	// Block eBPF updates from coming in
	c.metricsLock.Lock()
	defer c.metricsLock.Unlock()
	// reset the previous collected value because not all process will have new data
	// that is, a process that was inactive will not have any update but we need to set its metrics to 0
	c.resetDeltaValue()

	// collect process resource utilization and aggregate it per node, container and VMs
	c.updateResourceUtilizationMetrics()

	// collect node power and estimate process power
	c.UpdateEnergyUtilizationMetrics()

	c.printDebugMetrics()
	c.resetBpfDeltaValue()
	klog.V(5).Infof("Collector Update elapsed time: %s", time.Since(start))
}

// resetDeltaValue resets existing podEnergy previous curr value
func (c *Collector) resetDeltaValue() {
	c.NodeStats.ResetDeltaValues()
	if config.IsExposeContainerStatsEnabled() {
		for _, v := range c.ContainerStats {
			v.ResetDeltaValues()
		}
	}
	if config.IsExposeVMStatsEnabled() {
		for _, v := range c.VMStats {
			v.ResetDeltaValues()
		}
	}
}

// reset any processStats delta values *AFTER* collection
// given that we're streaming these stats and updating as we go
func (c *Collector) resetBpfDeltaValue() {
	for _, v := range c.ProcessStats {
		v.ResetDeltaValues()
	}
}

func (c *Collector) UpdateEnergyUtilizationMetrics() {
	c.UpdateNodeEnergyUtilizationMetrics()
	c.UpdateProcessEnergyUtilizationMetrics()
	// aggregate the process metrics per container and/or VMs
	c.AggregateProcessEnergyUtilizationMetrics()
}

// UpdateEnergyUtilizationMetrics collects real-time node's resource power utilization
// if there is no real-time power meter, use the container's resource usage metrics to estimate the node's resource power
func (c *Collector) UpdateNodeEnergyUtilizationMetrics() {
	energy.UpdateNodeEnergyMetrics(&c.NodeStats)
}

// UpdateProcessEnergyUtilizationMetrics estimates the process energy consumption using its resource utilization and the node components energy consumption
func (c *Collector) UpdateProcessEnergyUtilizationMetrics() {
	energy.UpdateProcessEnergy(c.ProcessStats, &c.NodeStats)
}

func (c *Collector) updateResourceUtilizationMetrics() {
	var wg sync.WaitGroup
	wg.Add(2)
	go c.updateNodeResourceUtilizationMetrics(&wg)
	go c.updateProcessResourceUtilizationMetrics(&wg)
	wg.Wait()
	// aggregate processes' resource utilization metrics to containers, virtual machines and nodes
	c.AggregateProcessResourceUtilizationMetrics()
	// update the deprecated cgroup metrics. Note that we only call this function after all process metrics were aggregated per container
	c.updateContainerResourceUtilizationMetrics()
}

// updateNodeAvgCPUFrequencyFromEBPF updates the average CPU frequency in each core
func (c *Collector) updateNodeAvgCPUFrequencyFromEBPF() {

}

// update the node metrics that are not related to aggregated resource utilization of processes
func (c *Collector) updateNodeResourceUtilizationMetrics(wg *sync.WaitGroup) {
	defer wg.Done()
	if config.IsExposeQATMetricsEnabled() && qat.IsQATCollectionSupported() {
		accelerator.UpdateNodeQATMetrics(stats.NewNodeStats(c.bpfSupportedMetrics))
	}
	if config.ExposeCPUFrequencyMetrics {
		c.updateNodeAvgCPUFrequencyFromEBPF()
	}
}

func (c *Collector) updateProcessResourceUtilizationMetrics(wg *sync.WaitGroup) {
	defer wg.Done()
	if config.EnabledGPU && gpu.IsGPUCollectionSupported() {
		accelerator.UpdateProcessGPUUtilizationMetrics(c.ProcessStats, c.bpfSupportedMetrics)
	}
}

// this is only for cgroup metrics, as these metrics are deprecated we might remove thi in the future
func (c *Collector) updateContainerResourceUtilizationMetrics() {
	if config.IsExposeContainerStatsEnabled() {
		if config.IsCgroupMetricsEnabled() {
			// collect cgroup metrics from cgroup api
			cgroup_collector.UpdateContainerCgroupMetrics(c.ContainerStats)
		}
	}
}

// AggregateProcessResourceUtilizationMetrics aggregates processes' resource utilization metrics to containers, virtual machines and nodes
func (c *Collector) AggregateProcessResourceUtilizationMetrics() {
	foundContainer := make(map[string]bool)
	foundVM := make(map[string]bool)
	for _, process := range c.ProcessStats {
		if process.IdleCounter > 0 {
			// if the process metrics were not updated for multiple interations, very if the process still exist, otherwise delete it from the map
			c.handleIdlingProcess(process)
		}
		for metricName, resource := range process.ResourceUsage {
			for id := range resource.Stat {
				delta := resource.Stat[id].GetDelta() // currently the process metrics are single socket

				// aggregate metrics per container
				if config.IsExposeContainerStatsEnabled() {
					if process.ContainerID != "" {
						c.createContainerStatsIfNotExist(process.ContainerID, process.CGroupID, process.PID, config.EnabledEBPFCgroupID)
						c.ContainerStats[process.ContainerID].ResourceUsage[metricName].AddDeltaStat(id, delta)
						foundContainer[process.ContainerID] = true
					}
				}

				// aggregate metrics per Virtual Machine
				if config.IsExposeVMStatsEnabled() {
					if process.VMID != "" {
						if _, ok := c.VMStats[process.VMID]; !ok {
							c.VMStats[process.VMID] = stats.NewVMStats(process.PID, process.VMID, c.bpfSupportedMetrics)
						}
						c.VMStats[process.VMID].ResourceUsage[metricName].AddDeltaStat(id, delta)
						foundVM[process.VMID] = true
					}
				}

				// aggregate metrics from all process to represent the node resource utilization
				c.NodeStats.ResourceUsage[metricName].AddDeltaStat(id, delta)
			}
		}
	}

	// clean up the cache
	// TODO: improve the removal of deleted containers from ContainerStats. Currently we verify the maxInactiveContainers using the found map
	if config.IsExposeContainerStatsEnabled() {
		c.handleInactiveContainers(foundContainer)
	}
	if config.IsExposeVMStatsEnabled() {
		c.handleInactiveVM(foundVM)
	}
}

// handleInactiveProcesses
func (c *Collector) handleIdlingProcess(pStat *stats.ProcessStats) {
	proc, _ := os.FindProcess(int(pStat.PID))
	err := proc.Signal(syscall.Signal(0))
	if err != nil {
		// delete if the process does not exist anymore
		delete(c.ProcessStats, pStat.PID)
		return
	}
}

// handleInactiveContainers
func (c *Collector) handleInactiveContainers(foundContainer map[string]bool) {
	numOfInactive := len(c.ContainerStats) - len(foundContainer)
	if numOfInactive > maxInactiveContainers {
		aliveContainers, err := cgroup_api.GetAliveContainers()
		if err != nil {
			klog.V(5).Infoln(err)
			return
		}
		for containerID := range c.ContainerStats {
			if containerID == utils.SystemProcessName || containerID == utils.KernelProcessName {
				continue
			}
			if _, found := aliveContainers[containerID]; !found {
				delete(c.ContainerStats, containerID)
			}
		}
	}
}

// handleInactiveVirtualMachine
func (c *Collector) handleInactiveVM(foundVM map[string]bool) {
	numOfInactive := len(c.VMStats) - len(foundVM)
	if numOfInactive > maxInactiveVM {
		for vmID := range c.VMStats {
			if _, found := foundVM[vmID]; !found {
				delete(c.VMStats, vmID)
			}
		}
	}
}

// AggregateProcessEnergyUtilizationMetrics aggregates processes' utilization metrics to containers and virtual machines
func (c *Collector) AggregateProcessEnergyUtilizationMetrics() {
	for _, process := range c.ProcessStats {
		for metricName, stat := range process.EnergyUsage {
			for id := range stat.Stat {
				delta := stat.Stat[id].GetDelta() // currently the process metrics are single socket

				// aggregate metrics per container
				if config.IsExposeContainerStatsEnabled() {
					if process.ContainerID != "" {
						c.createContainerStatsIfNotExist(process.ContainerID, process.CGroupID, process.PID, config.EnabledEBPFCgroupID)
						c.ContainerStats[process.ContainerID].EnergyUsage[metricName].AddDeltaStat(id, delta)
					}
				}

				// aggregate metrics per Virtual Machine
				if config.IsExposeVMStatsEnabled() {
					if process.VMID != "" {
						if _, ok := c.VMStats[process.VMID]; !ok {
							c.VMStats[process.VMID] = stats.NewVMStats(process.PID, process.VMID, c.bpfSupportedMetrics)
						}
						c.VMStats[process.VMID].EnergyUsage[metricName].AddDeltaStat(id, delta)
					}
				}
			}
		}
	}
}

func (c *Collector) printDebugMetrics() {
	// check the log verbosity level before iterating in all container
	if klog.V(3).Enabled() {
		if config.IsExposeContainerStatsEnabled() {
			for _, v := range c.ContainerStats {
				klog.V(3).Infoln(v)
			}
		}
		klog.V(3).Infoln(c.NodeStats.String())
	}
}
