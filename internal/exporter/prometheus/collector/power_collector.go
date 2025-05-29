// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package collector

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sustainable-computing-io/kepler/internal/monitor"
)

type PowerDataProvider = monitor.PowerDataProvider

// PowerCollector combines Node, Process, and Container collectors to ensure data consistency
// by fetching all data in a single atomic operation during collection
type PowerCollector struct {
	pm     PowerDataProvider
	logger *slog.Logger

	// Lock to ensure thread safety during collection
	mutex sync.RWMutex

	// Node power metrics
	ready                   bool
	nodeCPUJoulesDescriptor *prometheus.Desc
	nodeCPUWattsDescriptor  *prometheus.Desc

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
}

func joulesDesc(level, device string, labels []string) *prometheus.Desc {
	return prometheus.NewDesc(
		prometheus.BuildFQName(keplerNS, level, device+"_joules_total"),
		fmt.Sprintf("Energy consumption of %s at %s level in joules", device, level),
		labels, nil)
}

func wattsDesc(level, device string, labels []string) *prometheus.Desc {
	return prometheus.NewDesc(
		prometheus.BuildFQName(keplerNS, level, device+"_watts"),
		fmt.Sprintf("Power consumption of %s at %s level in watts", device, level),
		labels, nil)
}

func timeDesc(level, device string, labels []string) *prometheus.Desc {
	return prometheus.NewDesc(
		prometheus.BuildFQName(keplerNS, level, device+"_seconds_total"),
		fmt.Sprintf("Total user and system time of %s at %s level in seconds", device, level),
		labels, nil)
}

// NewPowerCollector creates a collector that provides consistent metrics
// by fetching all data in a single snapshot during collection
func NewPowerCollector(monitor PowerDataProvider, logger *slog.Logger) *PowerCollector {
	const (
		// these labels should rename the same across all descriptors to ease querying
		zone   = "zone"
		cntrID = "container_id"
		vmID   = "vm_id"
	)

	c := &PowerCollector{
		pm:                      monitor,
		logger:                  logger.With("collector", "power"),
		nodeCPUJoulesDescriptor: joulesDesc("node", "cpu", []string{zone, "path"}),
		nodeCPUWattsDescriptor:  wattsDesc("node", "cpu", []string{zone, "path"}),

		processCPUJoulesDescriptor: joulesDesc("process", "cpu", []string{"pid", "comm", "exe", "type", cntrID, vmID, zone}),
		processCPUWattsDescriptor:  wattsDesc("process", "cpu", []string{"pid", "comm", "exe", "type", cntrID, vmID, zone}),
		processCPUTimeDescriptor:   timeDesc("process", "cpu", []string{"pid", "comm", "exe", "type", cntrID, vmID}),

		containerCPUJoulesDescriptor: joulesDesc("container", "cpu", []string{cntrID, "container_name", "runtime", zone}),
		containerCPUWattsDescriptor:  wattsDesc("container", "cpu", []string{cntrID, "container_name", "runtime", zone}),

		vmCPUJoulesDescriptor: joulesDesc("vm", "cpu", []string{vmID, "vm_name", "hypervisor", zone}),
		vmCPUWattsDescriptor:  wattsDesc("vm", "cpu", []string{vmID, "vm_name", "hypervisor", zone}),
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
	ch <- c.nodeCPUJoulesDescriptor
	ch <- c.nodeCPUWattsDescriptor

	// process
	ch <- c.processCPUJoulesDescriptor
	ch <- c.processCPUWattsDescriptor
	ch <- c.processCPUTimeDescriptor

	// container
	ch <- c.containerCPUJoulesDescriptor
	ch <- c.containerCPUWattsDescriptor
	// ch <- c.containerCPUTimeDescriptor // TODO: add conntainerCPUTimeDescriptor

	// vm
	ch <- c.vmCPUJoulesDescriptor
	ch <- c.vmCPUWattsDescriptor
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

	c.collectNodeMetrics(ch, snapshot.Node)
	c.collectProcessMetrics(ch, snapshot.Processes)
	c.collectContainerMetrics(ch, snapshot.Containers)
	c.collectVMMetrics(ch, snapshot.VirtualMachines)
}

// collectNodeMetrics collects node-level power metrics
func (c *PowerCollector) collectNodeMetrics(ch chan<- prometheus.Metric, node *monitor.Node) {
	c.mutex.RLock() // locking nodeJoulesDescriptors
	defer c.mutex.RUnlock()

	for zone, energy := range node.Zones {
		path := zone.Path()
		zoneName := fmt.Sprintf("%s-%d", zone.Name(), zone.Index())

		ch <- prometheus.MustNewConstMetric(
			c.nodeCPUJoulesDescriptor,
			prometheus.CounterValue,
			energy.Absolute.Joules(),
			zoneName, path,
		)

		ch <- prometheus.MustNewConstMetric(
			c.nodeCPUWattsDescriptor,
			prometheus.GaugeValue,
			energy.Power.Watts(),
			zoneName, path,
		)
	}
}

// collectProcessMetrics collects process-level power metrics
func (c *PowerCollector) collectProcessMetrics(ch chan<- prometheus.Metric, processes monitor.Processes) {
	if len(processes) == 0 {
		c.logger.Debug("No processes to export metrics for")
		return
	}

	// No need to lock, already done by the calling function
	for pid, proc := range processes {
		pidStr := fmt.Sprintf("%d", pid)

		ch <- prometheus.MustNewConstMetric(
			c.processCPUTimeDescriptor,
			prometheus.CounterValue,
			proc.CPUTotalTime,
			pidStr, proc.Comm, proc.Exe, string(proc.Type),
			proc.ContainerID, proc.VirtualMachineID,
		)

		for zone, usage := range proc.Zones {
			zoneName := fmt.Sprintf("%s-%d", zone.Name(), zone.Index())
			ch <- prometheus.MustNewConstMetric(
				c.processCPUJoulesDescriptor,
				prometheus.CounterValue,
				usage.Absolute.Joules(),
				pidStr, proc.Comm, proc.Exe, string(proc.Type),
				proc.ContainerID, proc.VirtualMachineID,
				zoneName,
			)

			ch <- prometheus.MustNewConstMetric(
				c.processCPUWattsDescriptor,
				prometheus.GaugeValue,
				usage.Power.Watts(),
				pidStr, proc.Comm, proc.Exe, string(proc.Type),
				proc.ContainerID, proc.VirtualMachineID,
				zoneName,
			)
		}
	}
}

// collectContainerMetrics collects container-level power metrics
func (c *PowerCollector) collectContainerMetrics(ch chan<- prometheus.Metric, containers monitor.Containers) {
	if len(containers) == 0 {
		c.logger.Debug("No containers to export metrics for")
		return
	}

	// No need to lock, already done by the calling function
	for id, container := range containers {
		for zone, usage := range container.Zones {
			zoneName := fmt.Sprintf("%s-%d", zone.Name(), zone.Index())

			ch <- prometheus.MustNewConstMetric(
				c.containerCPUJoulesDescriptor,
				prometheus.CounterValue,
				usage.Absolute.Joules(),
				id, container.Name, string(container.Runtime),
				zoneName,
			)

			ch <- prometheus.MustNewConstMetric(
				c.containerCPUWattsDescriptor,
				prometheus.GaugeValue,
				usage.Power.Watts(),
				id, container.Name, string(container.Runtime), zoneName,
			)
		}
	}
}

// collectVMMetrics collects vm-level power metrics
func (c *PowerCollector) collectVMMetrics(ch chan<- prometheus.Metric, vms monitor.VirtualMachines) {
	if len(vms) == 0 {
		c.logger.Debug("No vms to export metrics for")
		return
	}

	// No need to lock, already done by the calling function
	for id, vm := range vms {
		for zone, usage := range vm.Zones {
			zoneName := fmt.Sprintf("%s-%d", zone.Name(), zone.Index())
			ch <- prometheus.MustNewConstMetric(
				c.vmCPUJoulesDescriptor,
				prometheus.CounterValue,
				usage.Absolute.Joules(),
				id, vm.Name, string(vm.Hypervisor),
				zoneName,
			)

			ch <- prometheus.MustNewConstMetric(
				c.vmCPUWattsDescriptor,
				prometheus.GaugeValue,
				usage.Power.Watts(),
				id, vm.Name, string(vm.Hypervisor),
				zoneName,
			)
		}
	}
}
