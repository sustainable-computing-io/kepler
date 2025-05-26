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

const (
	nodeRAPL      = "node"
	processRAPL   = "process"
	containerRAPL = "container"
	vmRAPL        = "vm"
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
	ready                 bool
	nodeJoulesDescriptors map[string]*prometheus.Desc
	nodeWattsDescriptors  map[string]*prometheus.Desc

	// Process power metrics
	processJoulesDescriptors  map[string]*prometheus.Desc
	processWattsDescriptors   map[string]*prometheus.Desc
	processCPUTimeDescriptors *prometheus.Desc

	// Container power metrics
	containerJoulesDescriptors map[string]*prometheus.Desc
	containerWattsDescriptors  map[string]*prometheus.Desc

	// Virtual Machine power metrics
	vmJoulesDescriptors map[string]*prometheus.Desc
	vmWattsDescriptors  map[string]*prometheus.Desc

	nodeEnergyZoneDescriptor *prometheus.Desc
}

// NewPowerCollector creates a collector that provides consistent metrics
// by fetching all data in a single snapshot during collection
func NewPowerCollector(monitor PowerDataProvider, logger *slog.Logger) *PowerCollector {
	c := &PowerCollector{
		pm:     monitor,
		logger: logger.With("collector", "power"),

		nodeJoulesDescriptors: make(map[string]*prometheus.Desc),
		nodeWattsDescriptors:  make(map[string]*prometheus.Desc),

		processJoulesDescriptors: make(map[string]*prometheus.Desc),
		processWattsDescriptors:  make(map[string]*prometheus.Desc),

		containerJoulesDescriptors: make(map[string]*prometheus.Desc),
		containerWattsDescriptors:  make(map[string]*prometheus.Desc),

		vmJoulesDescriptors: make(map[string]*prometheus.Desc),
		vmWattsDescriptors:  make(map[string]*prometheus.Desc),
	}

	go c.updateDescriptors()
	return c
}

// updateDescriptors creates metric descriptors based on available zones
func (c *PowerCollector) updateDescriptors() {
	<-c.pm.DataChannel()
	zoneNames := c.pm.ZoneNames() // must be thread-safe

	c.mutex.Lock() // for write
	defer c.mutex.Unlock()
	for _, name := range zoneNames {
		zoneName := SanitizeMetricName(name)

		//  node metric descriptors
		if _, exists := c.nodeJoulesDescriptors[zoneName]; !exists {
			c.nodeJoulesDescriptors[zoneName] = prometheus.NewDesc(
				prometheus.BuildFQName(namespace, nodeRAPL, zoneName+"_joules_total"),
				"Energy consumption in joules for RAPL zone "+zoneName,
				[]string{"path"},
				nil,
			)
		}

		if _, exists := c.nodeWattsDescriptors[zoneName]; !exists {
			c.nodeWattsDescriptors[zoneName] = prometheus.NewDesc(
				prometheus.BuildFQName(namespace, nodeRAPL, zoneName+"_watts"),
				"Power consumption in watts for RAPL zone "+zoneName,
				[]string{"path"},
				nil,
			)
		}

		// process metric descriptors
		if _, exists := c.processJoulesDescriptors[zoneName]; !exists {
			c.processJoulesDescriptors[zoneName] = prometheus.NewDesc(
				prometheus.BuildFQName(namespace, processRAPL, zoneName+"_joules_total"),
				"Energy consumption in joules for RAPL zone "+zoneName+" by process",
				[]string{"pid", "comm", "exe", "type", "container_id", "vm_id"},
				nil,
			)
		}

		if _, exists := c.processWattsDescriptors[zoneName]; !exists {
			c.processWattsDescriptors[zoneName] = prometheus.NewDesc(
				prometheus.BuildFQName(namespace, processRAPL, zoneName+"_watts"),
				"Power consumption in watts for RAPL zone "+zoneName+" by process",
				[]string{"pid", "comm", "exe", "type", "container_id", "vm_id"},
				nil,
			)
		}

		// container metric descriptors
		if _, exists := c.containerJoulesDescriptors[zoneName]; !exists {
			c.containerJoulesDescriptors[zoneName] = prometheus.NewDesc(
				prometheus.BuildFQName(namespace, containerRAPL, zoneName+"_joules_total"),
				"Energy consumption in joules for RAPL zone "+zoneName+" by container",
				[]string{"id", "name", "runtime"},
				nil,
			)
		}

		if _, exists := c.containerWattsDescriptors[zoneName]; !exists {
			c.containerWattsDescriptors[zoneName] = prometheus.NewDesc(
				prometheus.BuildFQName(namespace, containerRAPL, zoneName+"_watts"),
				"Power consumption in watts for RAPL zone "+zoneName+" by container",
				[]string{"id", "name", "runtime"},
				nil,
			)
		}

		// vm metric descriptors
		if _, exists := c.vmJoulesDescriptors[zoneName]; !exists {
			c.vmJoulesDescriptors[zoneName] = prometheus.NewDesc(
				prometheus.BuildFQName(namespace, vmRAPL, zoneName+"_joules_total"),
				"Energy consumption in joules for RAPL zone "+zoneName+" by vm",
				[]string{"id", "name", "hypervisor"},
				nil,
			)
		}

		if _, exists := c.vmWattsDescriptors[zoneName]; !exists {
			c.vmWattsDescriptors[zoneName] = prometheus.NewDesc(
				prometheus.BuildFQName(namespace, vmRAPL, zoneName+"_watts"),
				"Power consumption in watts for RAPL zone "+zoneName+" by vm",
				[]string{"id", "name", "hypervisor"},
				nil,
			)
		}
	}

	c.nodeEnergyZoneDescriptor = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, nodeRAPL, "energy_zone"),
		"Energy Zones from RAPL",
		[]string{"name", "index", "path"},
		nil,
	)

	c.processCPUTimeDescriptors = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, processRAPL, "cpu_seconds_total"),
		"Total user and system CPU time in seconds",
		[]string{"pid", "comm", "exe", "type", "container_id", "vm_id"},
		nil,
	)

	c.ready = true
}

// Describe implements the prometheus.Collector interface
func (c *PowerCollector) Describe(ch chan<- *prometheus.Desc) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	if !c.ready {
		c.logger.Debug("Describe called before monitor is ready")
		return
	}

	// node
	ch <- c.nodeEnergyZoneDescriptor
	for _, desc := range c.nodeJoulesDescriptors {
		ch <- desc
	}
	for _, desc := range c.nodeWattsDescriptors {
		ch <- desc
	}

	// process
	ch <- c.processCPUTimeDescriptors
	for _, desc := range c.processJoulesDescriptors {
		ch <- desc
	}
	for _, desc := range c.processWattsDescriptors {
		ch <- desc
	}

	// containers
	for _, desc := range c.containerJoulesDescriptors {
		ch <- desc
	}
	for _, desc := range c.containerWattsDescriptors {
		ch <- desc
	}

	// vms
	for _, desc := range c.vmJoulesDescriptors {
		ch <- desc
	}
	for _, desc := range c.vmWattsDescriptors {
		ch <- desc
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
		zoneName := SanitizeMetricName(zone.Name())
		// ensure both joules and watts descriptors exist
		joulesDesc, exists := c.nodeJoulesDescriptors[zoneName]
		if !exists {
			continue
		}

		wattsDesc, exists := c.nodeWattsDescriptors[zoneName]
		if !exists {
			continue
		}

		path := zone.Path()
		ch <- prometheus.MustNewConstMetric(
			joulesDesc,
			prometheus.CounterValue,
			energy.Absolute.Joules(),
			path,
		)

		ch <- prometheus.MustNewConstMetric(
			wattsDesc,
			prometheus.GaugeValue,
			energy.Power.Watts(),
			path,
		)

		ch <- prometheus.MustNewConstMetric(
			c.nodeEnergyZoneDescriptor,
			prometheus.GaugeValue,
			1,
			zoneName,
			fmt.Sprintf("%d", zone.Index()),
			path,
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
			c.processCPUTimeDescriptors,
			prometheus.CounterValue,
			proc.CPUTotalTime,
			pidStr, proc.Comm, proc.Exe, string(proc.Type),
			proc.ContainerID, proc.VirtualMachineID,
		)

		for zone, usage := range proc.Zones {
			zoneName := SanitizeMetricName(zone.Name())

			// Skip if descriptor doesn't exist
			joulesDesc, exists := c.processJoulesDescriptors[zoneName]
			if !exists {
				continue
			}

			wattsDesc, exists := c.processWattsDescriptors[zoneName]
			if !exists {
				continue
			}

			ch <- prometheus.MustNewConstMetric(
				joulesDesc,
				prometheus.CounterValue,
				usage.Absolute.Joules(),
				pidStr, proc.Comm, proc.Exe, string(proc.Type),
				proc.ContainerID, proc.VirtualMachineID,
			)

			ch <- prometheus.MustNewConstMetric(
				wattsDesc,
				prometheus.GaugeValue,
				usage.Power.Watts(),
				pidStr, proc.Comm, proc.Exe, string(proc.Type),
				proc.ContainerID, proc.VirtualMachineID,
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
			zoneName := SanitizeMetricName(zone.Name())

			// Skip if descriptor doesn't exist
			joulesDesc, exists := c.containerJoulesDescriptors[zoneName]
			if !exists {
				continue
			}

			wattsDesc, exists := c.containerWattsDescriptors[zoneName]
			if !exists {
				continue
			}

			ch <- prometheus.MustNewConstMetric(
				joulesDesc,
				prometheus.CounterValue,
				usage.Absolute.Joules(),
				id, container.Name, string(container.Runtime),
			)

			ch <- prometheus.MustNewConstMetric(
				wattsDesc,
				prometheus.GaugeValue,
				usage.Power.Watts(),
				id, container.Name, string(container.Runtime),
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
			zoneName := SanitizeMetricName(zone.Name())

			// Skip if descriptor doesn't exist
			joulesDesc, exists := c.vmJoulesDescriptors[zoneName]
			if !exists {
				continue
			}

			wattsDesc, exists := c.vmWattsDescriptors[zoneName]
			if !exists {
				continue
			}

			ch <- prometheus.MustNewConstMetric(
				joulesDesc,
				prometheus.CounterValue,
				usage.Absolute.Joules(),
				id, vm.Name, string(vm.Hypervisor),
			)

			ch <- prometheus.MustNewConstMetric(
				wattsDesc,
				prometheus.GaugeValue,
				usage.Power.Watts(),
				id, vm.Name, string(vm.Hypervisor),
			)
		}
	}
}
