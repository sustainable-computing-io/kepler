// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package collector

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sustainable-computing-io/kepler/config"
	"github.com/sustainable-computing-io/kepler/internal/monitor"
)

const nodeNameLabel = "node_name"

type PowerDataProvider = monitor.PowerDataProvider

// PowerCollector combines Node, Process, and Container collectors to ensure data consistency
// by fetching all data in a single atomic operation during collection
type PowerCollector struct {
	pm           PowerDataProvider
	logger       *slog.Logger
	metricsLevel config.Level

	// Lock to ensure thread safety during collection
	mutex sync.RWMutex

	// Node power metrics
	ready                   bool
	nodeCPUJoulesDescriptor *prometheus.Desc
	nodeCPUWattsDescriptor  *prometheus.Desc

	// Node power attribution as active and idle
	nodeCPUActiveWattsDesc  *prometheus.Desc
	nodeCPUActiveJoulesDesc *prometheus.Desc

	nodeCPUIdleWattsDesc  *prometheus.Desc
	nodeCPUIdleJoulesDesc *prometheus.Desc

	nodeCPUUsageRatioDescriptor *prometheus.Desc

	// Process power metrics
	processCPUJoulesDescriptor *prometheus.Desc
	processCPUWattsDescriptor  *prometheus.Desc
	processCPUTimeDescriptor   *prometheus.Desc

	// Container power metrics
	containerCPUJoulesDescriptor *prometheus.Desc
	containerCPUWattsDescriptor  *prometheus.Desc

	// Virtual Machine power metrics
	vmCPUJoulesDescriptor *prometheus.Desc
	vmCPUWattsDescriptor  *prometheus.Desc

	// Pod power metrics
	podCPUJoulesDescriptor *prometheus.Desc
	podCPUWattsDescriptor  *prometheus.Desc
}

func joulesDesc(level, device, nodeName string, labels []string) *prometheus.Desc {
	return prometheus.NewDesc(
		prometheus.BuildFQName(keplerNS, level, device+"_joules_total"),
		fmt.Sprintf("Energy consumption of %s at %s level in joules", device, level),
		labels, prometheus.Labels{nodeNameLabel: nodeName})
}

func wattsDesc(level, device, nodeName string, labels []string) *prometheus.Desc {
	return prometheus.NewDesc(
		prometheus.BuildFQName(keplerNS, level, device+"_watts"),
		fmt.Sprintf("Power consumption of %s at %s level in watts", device, level),
		labels, prometheus.Labels{nodeNameLabel: nodeName})
}

func deviceStateJoulesDesc(level, device, state, nodeName string, labels []string) *prometheus.Desc {
	return prometheus.NewDesc(
		prometheus.BuildFQName(keplerNS, level, fmt.Sprintf("%s_%s_joules_total", device, state)),
		fmt.Sprintf("Energy consumption of %s in %s state at %s level in joules", device, state, level),
		labels, prometheus.Labels{nodeNameLabel: nodeName})
}

func deviceStateWattsDesc(level, device, state, nodeName string, labels []string) *prometheus.Desc {
	return prometheus.NewDesc(
		prometheus.BuildFQName(keplerNS, level, fmt.Sprintf("%s_%s_watts", device, state)),
		fmt.Sprintf("Power consumption of %s in %s state at %s level in watts", device, state, level),
		labels, prometheus.Labels{nodeNameLabel: nodeName})
}

func timeDesc(level, device, nodeName string, labels []string) *prometheus.Desc {
	return prometheus.NewDesc(
		prometheus.BuildFQName(keplerNS, level, device+"_seconds_total"),
		fmt.Sprintf("Total user and system time of %s at %s level in seconds", device, level),
		labels, prometheus.Labels{nodeNameLabel: nodeName})
}

// NewPowerCollector creates a collector that provides consistent metrics
// by fetching all data in a single snapshot during collection
func NewPowerCollector(monitor PowerDataProvider, nodeName string, logger *slog.Logger, metricsLevel config.Level) *PowerCollector {
	const (
		// these labels should remain the same across all descriptors to ease querying
		zone   = "zone"
		cntrID = "container_id"
		vmID   = "vm_id"
		podID  = "pod_id"
	)

	c := &PowerCollector{
		pm:           monitor,
		logger:       logger.With("collector", "power"),
		metricsLevel: metricsLevel,

		nodeCPUJoulesDescriptor: joulesDesc("node", "cpu", nodeName, []string{zone, "path"}),
		nodeCPUWattsDescriptor:  wattsDesc("node", "cpu", nodeName, []string{zone, "path"}),

		nodeCPUActiveJoulesDesc: deviceStateJoulesDesc("node", "cpu", "active", nodeName, []string{zone, "path"}),
		nodeCPUIdleJoulesDesc:   deviceStateJoulesDesc("node", "cpu", "idle", nodeName, []string{zone, "path"}),

		nodeCPUActiveWattsDesc: deviceStateWattsDesc("node", "cpu", "active", nodeName, []string{zone, "path"}),
		nodeCPUIdleWattsDesc:   deviceStateWattsDesc("node", "cpu", "idle", nodeName, []string{zone, "path"}),

		nodeCPUUsageRatioDescriptor: prometheus.NewDesc(
			prometheus.BuildFQName(keplerNS, "node", "cpu_usage_ratio"),
			"CPU usage ratio of a node (value between 0.0 and 1.0)",
			nil, prometheus.Labels{nodeNameLabel: nodeName}),

		processCPUJoulesDescriptor: joulesDesc("process", "cpu", nodeName, []string{"pid", "comm", "exe", "type", "state", cntrID, vmID, zone}),
		processCPUWattsDescriptor:  wattsDesc("process", "cpu", nodeName, []string{"pid", "comm", "exe", "type", "state", cntrID, vmID, zone}),
		processCPUTimeDescriptor:   timeDesc("process", "cpu", nodeName, []string{"pid", "comm", "exe", "type", cntrID, vmID}),

		containerCPUJoulesDescriptor: joulesDesc("container", "cpu", nodeName, []string{cntrID, "container_name", "runtime", "state", zone, podID}),
		containerCPUWattsDescriptor:  wattsDesc("container", "cpu", nodeName, []string{cntrID, "container_name", "runtime", "state", zone, podID}),

		vmCPUJoulesDescriptor: joulesDesc("vm", "cpu", nodeName, []string{vmID, "vm_name", "hypervisor", "state", zone}),
		vmCPUWattsDescriptor:  wattsDesc("vm", "cpu", nodeName, []string{vmID, "vm_name", "hypervisor", "state", zone}),

		podCPUJoulesDescriptor: joulesDesc("pod", "cpu", nodeName, []string{podID, "pod_name", "pod_namespace", "state", zone}),
		podCPUWattsDescriptor:  wattsDesc("pod", "cpu", nodeName, []string{podID, "pod_name", "pod_namespace", "state", zone}),
	}

	go c.waitForData()

	return c
}

func (c *PowerCollector) waitForData() {
	<-c.pm.DataChannel()
	c.mutex.Lock()
	c.ready = true
	c.mutex.Unlock()
}

// Describe implements the prometheus.Collector interface
func (c *PowerCollector) Describe(ch chan<- *prometheus.Desc) {
	// node
	if c.metricsLevel.IsNodeEnabled() {
		ch <- c.nodeCPUJoulesDescriptor
		ch <- c.nodeCPUWattsDescriptor
		ch <- c.nodeCPUUsageRatioDescriptor
		// node cpu active
		ch <- c.nodeCPUActiveJoulesDesc
		ch <- c.nodeCPUActiveWattsDesc
		// node cpu idle
		ch <- c.nodeCPUIdleJoulesDesc
		ch <- c.nodeCPUIdleWattsDesc
	}

	// process
	if c.metricsLevel.IsProcessEnabled() {
		ch <- c.processCPUJoulesDescriptor
		ch <- c.processCPUWattsDescriptor
		ch <- c.processCPUTimeDescriptor
	}

	// container
	if c.metricsLevel.IsContainerEnabled() {
		ch <- c.containerCPUJoulesDescriptor
		ch <- c.containerCPUWattsDescriptor
		// ch <- c.containerCPUTimeDescriptor // TODO: add conntainerCPUTimeDescriptor
	}

	// vm
	if c.metricsLevel.IsVMEnabled() {
		ch <- c.vmCPUJoulesDescriptor
		ch <- c.vmCPUWattsDescriptor
	}

	// pod
	if c.metricsLevel.IsPodEnabled() {
		ch <- c.podCPUJoulesDescriptor
		ch <- c.podCPUWattsDescriptor
	}
}

func (c *PowerCollector) isReady() bool {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.ready
}

// Collect implements the prometheus.Collector interface
func (c *PowerCollector) Collect(ch chan<- prometheus.Metric) {
	if !c.isReady() {
		c.logger.Debug("Collect called before monitor is ready")
		return
	}

	started := time.Now()
	c.logger.Info("Collecting unified power data")
	defer func() {
		c.logger.Info("Collected unified power data", "duration", time.Since(started))
	}()

	snapshot, err := c.pm.Snapshot() // snapshot is thread-safe
	if err != nil {
		c.logger.Error("Failed to collect power data", "error", err)
		return
	}

	if c.metricsLevel.IsNodeEnabled() {
		c.collectNodeMetrics(ch, snapshot.Node)
	}

	if c.metricsLevel.IsProcessEnabled() {
		c.collectProcessMetrics(ch, "running", snapshot.Processes)
		c.collectProcessMetrics(ch, "terminated", snapshot.TerminatedProcesses)
	}

	if c.metricsLevel.IsContainerEnabled() {
		c.collectContainerMetrics(ch, "running", snapshot.Containers)
		c.collectContainerMetrics(ch, "terminated", snapshot.TerminatedContainers)
	}

	if c.metricsLevel.IsVMEnabled() {
		c.collectVMMetrics(ch, "running", snapshot.VirtualMachines)
		c.collectVMMetrics(ch, "terminated", snapshot.TerminatedVirtualMachines)
	}

	if c.metricsLevel.IsPodEnabled() {
		c.collectPodMetrics(ch, "running", snapshot.Pods)
		c.collectPodMetrics(ch, "terminated", snapshot.TerminatedPods)
	}
}

// collectNodeMetrics collects node-level power metrics
func (c *PowerCollector) collectNodeMetrics(ch chan<- prometheus.Metric, node *monitor.Node) {
	c.mutex.RLock() // locking nodeJoulesDescriptors
	defer c.mutex.RUnlock()

	ch <- prometheus.MustNewConstMetric(
		c.nodeCPUUsageRatioDescriptor,
		prometheus.GaugeValue,
		node.UsageRatio,
	)
	for zone, energy := range node.Zones {
		path := zone.Path()
		zoneName := zone.Name()

		// joules
		ch <- prometheus.MustNewConstMetric(
			c.nodeCPUJoulesDescriptor,
			prometheus.CounterValue,
			energy.EnergyTotal.Joules(),
			zoneName, path,
		)

		ch <- prometheus.MustNewConstMetric(
			c.nodeCPUActiveJoulesDesc,
			prometheus.CounterValue,
			energy.ActiveEnergyTotal.Joules(),
			zoneName, path,
		)

		ch <- prometheus.MustNewConstMetric(
			c.nodeCPUIdleJoulesDesc,
			prometheus.CounterValue,
			energy.IdleEnergyTotal.Joules(),
			zoneName, path,
		)

		// watts
		ch <- prometheus.MustNewConstMetric(
			c.nodeCPUWattsDescriptor,
			prometheus.GaugeValue,
			energy.Power.Watts(),
			zoneName, path,
		)
		ch <- prometheus.MustNewConstMetric(
			c.nodeCPUActiveWattsDesc,
			prometheus.GaugeValue,
			energy.ActivePower.Watts(),
			zoneName, path,
		)
		ch <- prometheus.MustNewConstMetric(
			c.nodeCPUIdleWattsDesc,
			prometheus.GaugeValue,
			energy.IdlePower.Watts(),
			zoneName, path,
		)

	}
}

// collectProcessMetrics collects process-level power metrics
func (c *PowerCollector) collectProcessMetrics(ch chan<- prometheus.Metric, state string, processes monitor.Processes) {
	if len(processes) == 0 {
		c.logger.Debug("No processes to export metrics", "state", state)
		return
	}

	// No need to lock, already done by the calling function
	for pid, proc := range processes {

		ch <- prometheus.MustNewConstMetric(
			c.processCPUTimeDescriptor,
			prometheus.CounterValue,
			proc.CPUTotalTime,
			pid, proc.Comm, proc.Exe, string(proc.Type),
			proc.ContainerID, proc.VirtualMachineID,
		)

		for zone, usage := range proc.Zones {
			zoneName := zone.Name()
			ch <- prometheus.MustNewConstMetric(
				c.processCPUJoulesDescriptor,
				prometheus.CounterValue,
				usage.EnergyTotal.Joules(),
				pid, proc.Comm, proc.Exe, string(proc.Type), state,
				proc.ContainerID, proc.VirtualMachineID,
				zoneName,
			)

			ch <- prometheus.MustNewConstMetric(
				c.processCPUWattsDescriptor,
				prometheus.GaugeValue,
				usage.Power.Watts(),
				pid, proc.Comm, proc.Exe, string(proc.Type), state,
				proc.ContainerID, proc.VirtualMachineID,
				zoneName,
			)
		}
	}
}

// collectContainerMetrics collects container-level power metrics
func (c *PowerCollector) collectContainerMetrics(ch chan<- prometheus.Metric, state string, containers monitor.Containers) {
	if len(containers) == 0 {
		c.logger.Debug("No containers to export metrics for", "state", state)
		return
	}

	// No need to lock, already done by the calling function
	for id, container := range containers {
		for zone, usage := range container.Zones {
			zoneName := zone.Name()

			ch <- prometheus.MustNewConstMetric(
				c.containerCPUJoulesDescriptor,
				prometheus.CounterValue,
				usage.EnergyTotal.Joules(),
				id, container.Name, string(container.Runtime), state,
				zoneName,
				container.PodID,
			)

			ch <- prometheus.MustNewConstMetric(
				c.containerCPUWattsDescriptor,
				prometheus.GaugeValue,
				usage.Power.Watts(),
				id, container.Name, string(container.Runtime), state,
				zoneName,
				container.PodID,
			)
		}
	}
}

// collectVMMetrics collects vm-level power metrics
func (c *PowerCollector) collectVMMetrics(ch chan<- prometheus.Metric, state string, vms monitor.VirtualMachines) {
	if len(vms) == 0 {
		c.logger.Debug("No vms to export metrics for", "state", state)
		return
	}

	// No need to lock, already done by the calling function
	for id, vm := range vms {
		for zone, usage := range vm.Zones {
			zoneName := zone.Name()
			ch <- prometheus.MustNewConstMetric(
				c.vmCPUJoulesDescriptor,
				prometheus.CounterValue,
				usage.EnergyTotal.Joules(),
				id, vm.Name, string(vm.Hypervisor), state,
				zoneName,
			)

			ch <- prometheus.MustNewConstMetric(
				c.vmCPUWattsDescriptor,
				prometheus.GaugeValue,
				usage.Power.Watts(),
				id, vm.Name, string(vm.Hypervisor), state,
				zoneName,
			)
		}
	}
}

func (c *PowerCollector) collectPodMetrics(ch chan<- prometheus.Metric, state string, pods monitor.Pods) {
	if len(pods) == 0 {
		c.logger.Debug("No pods to export metrics", "state", state)
		return
	}

	// No need to lock, already done by the calling function
	for id, pod := range pods {
		for zone, usage := range pod.Zones {
			zoneName := zone.Name()
			ch <- prometheus.MustNewConstMetric(
				c.podCPUJoulesDescriptor,
				prometheus.CounterValue,
				usage.EnergyTotal.Joules(),
				id, pod.Name, pod.Namespace, state,
				zoneName,
			)

			ch <- prometheus.MustNewConstMetric(
				c.podCPUWattsDescriptor,
				prometheus.GaugeValue,
				usage.Power.Watts(),
				id, pod.Name, pod.Namespace, state,
				zoneName,
			)
		}
	}
}
