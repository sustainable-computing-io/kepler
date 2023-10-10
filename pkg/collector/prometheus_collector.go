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

//nolint:dupl // process metrics should be here not in another package
package collector

import (
	"fmt"
	"sync"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/sustainable-computing-io/kepler/pkg/bpfassets/attacher"
	collector_metric "github.com/sustainable-computing-io/kepler/pkg/collector/metric"
	"github.com/sustainable-computing-io/kepler/pkg/config"
	"github.com/sustainable-computing-io/kepler/pkg/power/platform"
)

const (
	namespace        = "kepler"
	miliJouleToJoule = 1000
)

type NodeDesc struct {
	// Node metadata
	nodeInfo *prometheus.Desc

	// Energy (counter)
	nodeCoreJoulesTotal            *prometheus.Desc
	nodeUncoreJoulesTotal          *prometheus.Desc
	nodeDramJoulesTotal            *prometheus.Desc
	nodePackageJoulesTotal         *prometheus.Desc
	nodePlatformJoulesTotal        *prometheus.Desc
	nodeOtherComponentsJoulesTotal *prometheus.Desc
	nodeGPUJoulesTotal             *prometheus.Desc

	// Additional metrics (gauge)
	// TODO: review if we really need to expose this metric.
	NodeCPUFrequency *prometheus.Desc

	// QAT metrics
	NodeQATUtilization *prometheus.Desc
}

type ContainerDesc struct {
	// Energy (counter)
	containerCoreJoulesTotal            *prometheus.Desc
	containerUncoreJoulesTotal          *prometheus.Desc
	containerDramJoulesTotal            *prometheus.Desc
	containerPackageJoulesTotal         *prometheus.Desc
	containerOtherComponentsJoulesTotal *prometheus.Desc
	containerGPUJoulesTotal             *prometheus.Desc
	containerJoulesTotal                *prometheus.Desc

	// Hardware Counters (counter)
	containerCPUCyclesTotal *prometheus.Desc
	containerCPUInstrTotal  *prometheus.Desc
	containerCacheMissTotal *prometheus.Desc

	// cGroups (counter)
	containerCgroupCPUUsageUsTotal       *prometheus.Desc
	containerCgroupMemoryUsageBytesTotal *prometheus.Desc
	containerCgroupSystemCPUUsageUsTotal *prometheus.Desc
	containerCgroupUserCPUUsageUsTotal   *prometheus.Desc

	// General kubelet (counter)
	containerKubeletCPUUsageTotal    *prometheus.Desc
	containerKubeletMemoryBytesTotal *prometheus.Desc

	// Additional metrics (gauge)
	containerCPUTime      *prometheus.Desc
	containerPageCacheHit *prometheus.Desc

	// IRQ metrics
	containerNetTxIRQTotal *prometheus.Desc
	containerNetRxIRQTotal *prometheus.Desc
	containerBlockIRQTotal *prometheus.Desc
}

// metric used by the model server to train the model
type PodDesc struct {
	// Hardware Counters (counter)
	// The clever dashboard use the pod_cpu_instructions but should be updated later to container_cpu_instructions
	podCPUInstrTotal *prometheus.Desc
}

// PrometheusCollector holds the list of prometheus metrics for both node and pod context
type PrometheusCollector struct {
	nodeDesc      *NodeDesc
	containerDesc *ContainerDesc
	podDesc       *PodDesc
	processDesc   *processDesc
	vmDesc        *vmDesc

	// NodeMetrics holds all node energy and resource usage metrics
	NodeMetrics *collector_metric.NodeMetrics

	// ContainersMetrics holds all container energy and resource usage metrics
	ContainersMetrics *map[string]*collector_metric.ContainerMetrics

	// ProcessMetrics hold all process energy and resource usage metrics
	ProcessMetrics *map[uint64]*collector_metric.ProcessMetrics

	// VMMetrics hold all Virtual Machine energy and resource usage metrics
	VMMetrics *map[uint64]*collector_metric.VMMetrics

	// SamplePeriodSec the collector metric collection interval
	SamplePeriodSec float64

	// Lock to syncronize the collector update with prometheus exporter
	Mx sync.Mutex

	// Record whether we have KubletMetrics
	HaveKubletMetric bool

	// Record whether we have cGroupsMetrics
	HavecGroupsMetric bool
}

// NewPrometheusExporter create and initialize all the PrometheusCollector structures
func NewPrometheusExporter() *PrometheusCollector {
	exporter := PrometheusCollector{
		// prometheus metric descriptions
		nodeDesc:    &NodeDesc{},
		podDesc:     &PodDesc{},
		processDesc: &processDesc{},
		vmDesc:      &vmDesc{},
	}
	exporter.newNodeMetrics()
	exporter.newContainerMetrics()
	exporter.newPodMetrics()
	exporter.newprocessMetrics()
	exporter.newVMMetrics()
	return &exporter
}

// Describe implements the prometheus.Collector interface
func (p *PrometheusCollector) Describe(ch chan<- *prometheus.Desc) {
	// Node metadata
	ch <- p.nodeDesc.nodeInfo

	// Node Energy (counter)
	ch <- p.nodeDesc.nodeCoreJoulesTotal
	ch <- p.nodeDesc.nodeUncoreJoulesTotal
	ch <- p.nodeDesc.nodeDramJoulesTotal
	ch <- p.nodeDesc.nodePackageJoulesTotal
	ch <- p.nodeDesc.nodePlatformJoulesTotal
	ch <- p.nodeDesc.nodeOtherComponentsJoulesTotal

	if config.EnabledGPU {
		ch <- p.nodeDesc.nodeGPUJoulesTotal
	}

	// Additional Node metrics from QAT
	if config.EnabledQAT {
		ch <- p.nodeDesc.NodeQATUtilization
	}

	// Additional Node metrics (gauge)
	ch <- p.nodeDesc.NodeCPUFrequency

	// Container Energy (counter)
	ch <- p.containerDesc.containerCoreJoulesTotal
	ch <- p.containerDesc.containerUncoreJoulesTotal
	ch <- p.containerDesc.containerDramJoulesTotal
	ch <- p.containerDesc.containerPackageJoulesTotal
	ch <- p.containerDesc.containerOtherComponentsJoulesTotal
	if config.EnabledGPU {
		ch <- p.containerDesc.containerGPUJoulesTotal
	}
	ch <- p.containerDesc.containerJoulesTotal

	// container Hardware Counters (counter)
	if config.ExposeHardwareCounterMetrics && collector_metric.CPUHardwareCounterEnabled {
		ch <- p.containerDesc.containerCPUCyclesTotal
		ch <- p.containerDesc.containerCPUInstrTotal
		ch <- p.containerDesc.containerCacheMissTotal
	}

	// container cGroups Counters (counter)
	if config.IsCgroupMetricsEnabled() {
		if len(collector_metric.AvailableCGroupMetrics) != 0 {
			ch <- p.containerDesc.containerCgroupCPUUsageUsTotal
			ch <- p.containerDesc.containerCgroupMemoryUsageBytesTotal
			ch <- p.containerDesc.containerCgroupSystemCPUUsageUsTotal
			ch <- p.containerDesc.containerCgroupUserCPUUsageUsTotal
			p.HavecGroupsMetric = true
		} else {
			p.HavecGroupsMetric = false
		}
	}

	// container Kubelet Counters (counter)
	if config.IsKubeletMetricsEnabled() {
		if len(collector_metric.AvailableKubeletMetrics) != 0 {
			ch <- p.containerDesc.containerKubeletCPUUsageTotal
			ch <- p.containerDesc.containerKubeletMemoryBytesTotal
			p.HaveKubletMetric = true
		} else {
			p.HaveKubletMetric = false
		}
	}

	ch <- p.containerDesc.containerCPUTime
	ch <- p.containerDesc.containerPageCacheHit

	// IRQ counter
	if config.IsIRQCounterMetricsEnabled() {
		ch <- p.containerDesc.containerNetTxIRQTotal
		ch <- p.containerDesc.containerNetRxIRQTotal
		ch <- p.containerDesc.containerBlockIRQTotal
	}
	p.describeProcess(ch)
	p.describeVM(ch)
}

func (p *PrometheusCollector) newNodeMetrics() {
	nodeInfo := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "node", "info"),
		"Labeled node information",
		[]string{"cpu_architecture"}, nil,
	)
	// Energy (counter)
	// TODO: separate the energy consumption per CPU, including the label cpu
	nodeCoreJoulesTotal := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "node", addSuffix(collector_metric.CORE, config.AggregatedEnergySuffix)),
		"Aggregated RAPL value in core in joules",
		[]string{"package", "instance", "source", "mode"}, nil,
	)
	nodeUncoreJoulesTotal := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "node", addSuffix(collector_metric.UNCORE, config.AggregatedEnergySuffix)),
		"Aggregated RAPL value in uncore in joules",
		[]string{"package", "instance", "source", "mode"}, nil,
	)
	nodeDramJoulesTotal := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "node", addSuffix(collector_metric.DRAM, config.AggregatedEnergySuffix)),
		"Aggregated RAPL value in dram in joules",
		[]string{"package", "instance", "source", "mode"}, nil,
	)
	nodePackageJoulesTotal := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "node", addSuffix(collector_metric.PKG, config.AggregatedEnergySuffix)),
		"Aggregated RAPL value in package (socket) in joules",
		[]string{"package", "instance", "source", "mode"}, nil,
	)
	nodePlatformJoulesTotal := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "node", addSuffix(collector_metric.PLATFORM, config.AggregatedEnergySuffix)),
		"Aggregated RAPL value in platform (entire node) in joules",
		[]string{"instance", "source", "mode"}, nil,
	)
	nodeOtherComponentsJoulesTotal := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "node", addSuffix(collector_metric.OTHER, config.AggregatedEnergySuffix)),
		"Aggregated RAPL value in other components (platform - package - dram) in joules",
		[]string{"instance", "mode"}, nil,
	)
	nodeGPUJoulesTotal := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "node", addSuffix(collector_metric.GPU, config.AggregatedEnergySuffix)),
		"Current GPU value in joules",
		[]string{"index", "instance", "source", "mode"}, nil,
	)

	// Additional metrics from QAT
	NodeQATUtilization := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "node", "accelerator_intel_qat"),
		"Current Intel QAT Utilization",
		[]string{"qatDevID", "instance", "source", "type"}, nil,
	)

	// Additional metrics (gauge)
	NodeCPUFrequency := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "node", "cpu_scaling_frequency_hertz"),
		"Current average cpu frequency in hertz",
		[]string{"cpu", "instance"}, nil,
	)

	p.nodeDesc = &NodeDesc{
		nodeInfo:                       nodeInfo,
		nodeCoreJoulesTotal:            nodeCoreJoulesTotal,
		nodeUncoreJoulesTotal:          nodeUncoreJoulesTotal,
		nodeDramJoulesTotal:            nodeDramJoulesTotal,
		nodePackageJoulesTotal:         nodePackageJoulesTotal,
		nodePlatformJoulesTotal:        nodePlatformJoulesTotal,
		nodeOtherComponentsJoulesTotal: nodeOtherComponentsJoulesTotal,
		nodeGPUJoulesTotal:             nodeGPUJoulesTotal,
		NodeCPUFrequency:               NodeCPUFrequency,
		NodeQATUtilization:             NodeQATUtilization,
	}
}

func (p *PrometheusCollector) newContainerMetrics() {
	// Energy (counter)
	containerCoreJoulesTotal := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "container", addSuffix(collector_metric.CORE, config.AggregatedEnergySuffix)),
		"Aggregated RAPL value in core in joules",
		[]string{"container_id", "pod_name", "container_name", "container_namespace", "mode"}, nil,
	)
	containerUncoreJoulesTotal := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "container", addSuffix(collector_metric.UNCORE, config.AggregatedEnergySuffix)),
		"Aggregated RAPL value in uncore in joules",
		[]string{"container_id", "pod_name", "container_name", "container_namespace", "mode"}, nil,
	)
	containerDramJoulesTotal := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "container", addSuffix(collector_metric.DRAM, config.AggregatedEnergySuffix)),
		"Aggregated RAPL value in dram in joules",
		[]string{"container_id", "pod_name", "container_name", "container_namespace", "mode"}, nil,
	)
	containerPackageJoulesTotal := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "container", addSuffix(collector_metric.PKG, config.AggregatedEnergySuffix)),
		"Aggregated RAPL value in package (socket) in joules",
		[]string{"container_id", "pod_name", "container_name", "container_namespace", "mode"}, nil,
	)
	containerOtherComponentsJoulesTotal := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "container", addSuffix(collector_metric.OTHER, config.AggregatedEnergySuffix)),
		"Aggregated value in other host components (platform - package - dram) in joules",
		[]string{"container_id", "pod_name", "container_name", "container_namespace", "mode"}, nil,
	)
	containerGPUJoulesTotal := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "container", addSuffix(collector_metric.GPU, config.AggregatedEnergySuffix)),
		"Aggregated GPU value in joules",
		[]string{"container_id", "pod_name", "container_name", "container_namespace", "mode"}, nil,
	)
	containerJoulesTotal := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "container", config.AggregatedEnergySuffix),
		"Aggregated RAPL Package + Uncore + DRAM + GPU + other host components (platform - package - dram) in joules",
		[]string{"container_id", "pod_name", "container_name", "container_namespace", "mode"}, nil,
	)

	// Hardware Counters (counter)
	containerCPUCyclesTotal := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "container", addSuffix(config.CPUCycle, config.AggregatedUsageSuffix)),
		"Aggregated CPU cycle value",
		[]string{"container_id", "pod_name", "container_name", "container_namespace"}, nil,
	)
	containerCPUInstrTotal := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "container", addSuffix(config.CPUInstruction, config.AggregatedUsageSuffix)),
		"Aggregated CPU instruction value",
		[]string{"container_id", "pod_name", "container_name", "container_namespace"}, nil,
	)
	containerCacheMissTotal := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "container", addSuffix(config.CacheMiss, config.AggregatedUsageSuffix)),
		"Aggregated cache miss value",
		[]string{"container_id", "pod_name", "container_name", "container_namespace"}, nil,
	)

	// cGroups Counters (counter)
	containerCgroupCPUUsageUsTotal := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "container", addSuffix(config.CgroupfsCPU, config.AggregatedUsageSuffix)),
		"Aggregated cpu usage obtained from cGroups",
		[]string{"container_id", "pod_name", "container_name", "container_namespace"}, nil,
	)

	containerCgroupMemoryUsageBytesTotal := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "container", addSuffix(config.CgroupfsMemory, config.AggregatedUsageSuffix)),
		"Aggregated memory bytes obtained from cGroups",
		[]string{"container_id", "pod_name", "container_name", "container_namespace"}, nil,
	)

	containerCgroupSystemCPUUsageUsTotal := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "container", addSuffix(config.CgroupfsSystemCPU, config.AggregatedUsageSuffix)),
		"Aggregated system cpu usage obtained from cGroups",
		[]string{"container_id", "pod_name", "container_name", "container_namespace"}, nil,
	)

	containerCgroupUserCPUUsageUsTotal := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "container", addSuffix(config.CgroupfsUserCPU, config.AggregatedUsageSuffix)),
		"Aggregated user cpu usage obtained from cGroups",
		[]string{"container_id", "pod_name", "container_name", "container_namespace"}, nil,
	)

	// Kubelet Counters (counter)
	containerKubeletCPUUsageTotal := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "container", addSuffix(config.KubeletCPUUsage, config.AggregatedUsageSuffix)),
		"Aggregated cpu usage obtained from kubelet",
		[]string{"container_id", "pod_name", "container_name", "container_namespace"}, nil,
	)

	containerKubeletMemoryBytesTotal := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "container", addSuffix(config.KubeletMemoryUsage, config.AggregatedUsageSuffix)),
		"Aggregated memory bytes obtained from kubelet",
		[]string{"container_id", "pod_name", "container_name", "container_namespace"}, nil,
	)

	// Additional metrics (counter)
	containerCPUTime := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "container", addSuffix(config.CPUTime, config.AggregatedUsageSuffix)),
		"Aggregated CPU time obtained from BPF",
		[]string{"container_id", "pod_name", "container_name", "container_namespace"}, nil,
	)
	containerPageCacheHit := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "container", addSuffix(config.PageCacheHit, config.AggregatedUsageSuffix)),
		"Aggregated Page Cache Hit obtained from BPF",
		[]string{"container_id", "pod_name", "container_name", "container_namespace"}, nil,
	)

	// network irq metrics
	containerNetTxIRQTotal := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "container", addSuffix(config.IRQNetTXLabel, config.AggregatedUsageSuffix)),
		"Aggregated network tx irq value obtained from BPF",
		[]string{"container_id", "pod_name", "container_name", "container_namespace"}, nil,
	)
	containerNetRxIRQTotal := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "container", addSuffix(config.IRQNetRXLabel, config.AggregatedUsageSuffix)),
		"Aggregated network rx irq value obtained from BPF",
		[]string{"container_id", "pod_name", "container_name", "container_namespace"}, nil,
	)
	containerBlockIRQTotal := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "container", addSuffix(config.IRQBlockLabel, config.AggregatedUsageSuffix)),
		"Aggregated block irq value obtained from BPF",
		[]string{"container_id", "pod_name", "container_name", "container_namespace"}, nil,
	)

	p.containerDesc = &ContainerDesc{
		containerCoreJoulesTotal:             containerCoreJoulesTotal,
		containerUncoreJoulesTotal:           containerUncoreJoulesTotal,
		containerDramJoulesTotal:             containerDramJoulesTotal,
		containerPackageJoulesTotal:          containerPackageJoulesTotal,
		containerOtherComponentsJoulesTotal:  containerOtherComponentsJoulesTotal,
		containerGPUJoulesTotal:              containerGPUJoulesTotal,
		containerJoulesTotal:                 containerJoulesTotal,
		containerCPUCyclesTotal:              containerCPUCyclesTotal,
		containerCPUInstrTotal:               containerCPUInstrTotal,
		containerCacheMissTotal:              containerCacheMissTotal,
		containerCgroupCPUUsageUsTotal:       containerCgroupCPUUsageUsTotal,
		containerCgroupMemoryUsageBytesTotal: containerCgroupMemoryUsageBytesTotal,
		containerCgroupSystemCPUUsageUsTotal: containerCgroupSystemCPUUsageUsTotal,
		containerCgroupUserCPUUsageUsTotal:   containerCgroupUserCPUUsageUsTotal,
		containerKubeletMemoryBytesTotal:     containerKubeletMemoryBytesTotal,
		containerKubeletCPUUsageTotal:        containerKubeletCPUUsageTotal,
		containerCPUTime:                     containerCPUTime,
		containerPageCacheHit:                containerPageCacheHit,
		containerNetTxIRQTotal:               containerNetTxIRQTotal,
		containerNetRxIRQTotal:               containerNetRxIRQTotal,
		containerBlockIRQTotal:               containerBlockIRQTotal,
	}
}

func (p *PrometheusCollector) newPodMetrics() {
	podCPUInstrTotal := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "pod", "cpu_instructions"),
		"Aggregated CPU instruction value (deprecated)",
		[]string{"pod_name", "container_name", "container_namespace"}, nil,
	)

	p.podDesc = &PodDesc{
		podCPUInstrTotal: podCPUInstrTotal,
	}
}

// Collect implements the prometheus.Collector interface
func (p *PrometheusCollector) Collect(ch chan<- prometheus.Metric) {
	p.Mx.Lock()
	defer p.Mx.Unlock()
	wg := sync.WaitGroup{}
	p.updateNodeMetrics(&wg, ch)
	p.updatePodMetrics(&wg, ch)
	p.updateProcessMetrics(&wg, ch)
	p.updateVMMetrics(&wg, ch)
	wg.Wait()
}

// updateNodeMetrics send node metrics to prometheus
func (p *PrometheusCollector) updateNodeMetrics(wg *sync.WaitGroup, ch chan<- prometheus.Metric) {
	// we start with the metrics that might have a longer loop, e.g. range the cpus
	wg.Add(1)
	go func() {
		defer wg.Done()
		// TODO: remove this metric if we don't need, reporting this can be an expensive process
		for cpuID, freq := range p.NodeMetrics.CPUFrequency {
			ch <- prometheus.MustNewConstMetric(
				p.nodeDesc.NodeCPUFrequency,
				prometheus.GaugeValue,
				float64(freq),
				fmt.Sprintf("%d", cpuID), collector_metric.NodeName,
			)
		}

		ch <- prometheus.MustNewConstMetric(
			p.nodeDesc.nodeInfo,
			prometheus.CounterValue,
			1,
			collector_metric.NodeCPUArchitecture,
		)
		// Node metrics in joules (counter)
		for pkgID := range p.NodeMetrics.AbsEnergyInCore.Stat {
			dynPower := (float64(p.NodeMetrics.GetAggrDynEnergyPerID(collector_metric.PKG, pkgID)) / miliJouleToJoule)
			ch <- prometheus.MustNewConstMetric(
				p.nodeDesc.nodePackageJoulesTotal,
				prometheus.CounterValue,
				dynPower,
				pkgID, collector_metric.NodeName, "rapl", "dynamic",
			)
			dynPower = (float64(p.NodeMetrics.GetAggrDynEnergyPerID(collector_metric.CORE, pkgID)) / miliJouleToJoule)
			ch <- prometheus.MustNewConstMetric(
				p.nodeDesc.nodeCoreJoulesTotal,
				prometheus.CounterValue,
				dynPower,
				pkgID, collector_metric.NodeName, "rapl", "dynamic",
			)
			dynPower = (float64(p.NodeMetrics.GetAggrDynEnergyPerID(collector_metric.UNCORE, pkgID)) / miliJouleToJoule)
			ch <- prometheus.MustNewConstMetric(
				p.nodeDesc.nodeUncoreJoulesTotal,
				prometheus.CounterValue,
				dynPower,
				pkgID, collector_metric.NodeName, "rapl", "dynamic",
			)
			dynPower = (float64(p.NodeMetrics.GetAggrDynEnergyPerID(collector_metric.DRAM, pkgID)) / miliJouleToJoule)
			ch <- prometheus.MustNewConstMetric(
				p.nodeDesc.nodeDramJoulesTotal,
				prometheus.CounterValue,
				dynPower,
				pkgID, collector_metric.NodeName, "rapl", "dynamic",
			)

			if config.IsIdlePowerEnabled() {
				idlePower := (float64(p.NodeMetrics.GetAggrIdleEnergyPerID(collector_metric.PKG, pkgID)) / miliJouleToJoule)
				ch <- prometheus.MustNewConstMetric(
					p.nodeDesc.nodePackageJoulesTotal,
					prometheus.CounterValue,
					idlePower,
					pkgID, collector_metric.NodeName, "rapl", "idle",
				)

				idlePower = (float64(p.NodeMetrics.GetAggrIdleEnergyPerID(collector_metric.CORE, pkgID)) / miliJouleToJoule)
				ch <- prometheus.MustNewConstMetric(
					p.nodeDesc.nodeCoreJoulesTotal,
					prometheus.CounterValue,
					idlePower,
					pkgID, collector_metric.NodeName, "rapl", "idle",
				)

				idlePower = (float64(p.NodeMetrics.GetAggrIdleEnergyPerID(collector_metric.UNCORE, pkgID)) / miliJouleToJoule)
				ch <- prometheus.MustNewConstMetric(
					p.nodeDesc.nodeUncoreJoulesTotal,
					prometheus.CounterValue,
					idlePower,
					pkgID, collector_metric.NodeName, "rapl", "idle",
				)

				idlePower = (float64(p.NodeMetrics.GetAggrIdleEnergyPerID(collector_metric.DRAM, pkgID)) / miliJouleToJoule)
				ch <- prometheus.MustNewConstMetric(
					p.nodeDesc.nodeDramJoulesTotal,
					prometheus.CounterValue,
					idlePower,
					pkgID, collector_metric.NodeName, "rapl", "idle",
				)
			}
		}

		dynPower := (float64(p.NodeMetrics.GetSumAggrDynEnergyFromAllSources(collector_metric.OTHER)) / miliJouleToJoule)
		ch <- prometheus.MustNewConstMetric(
			p.nodeDesc.nodeOtherComponentsJoulesTotal,
			prometheus.CounterValue,
			dynPower,
			collector_metric.NodeName, "dynamic",
		)
		powerSource := platform.GetPowerSource()
		dynPower = (float64(p.NodeMetrics.GetSumAggrDynEnergyFromAllSources(collector_metric.PLATFORM)) / miliJouleToJoule)
		ch <- prometheus.MustNewConstMetric(
			p.nodeDesc.nodePlatformJoulesTotal,
			prometheus.CounterValue,
			dynPower,
			collector_metric.NodeName, powerSource, "dynamic",
		)

		if config.IsIdlePowerEnabled() {
			idlePower := (float64(p.NodeMetrics.GetSumAggrIdleEnergyFromAllSources(collector_metric.OTHER)) / miliJouleToJoule)
			ch <- prometheus.MustNewConstMetric(
				p.nodeDesc.nodeOtherComponentsJoulesTotal,
				prometheus.CounterValue,
				idlePower,
				collector_metric.NodeName, "idle",
			)
			idlePower = (float64(p.NodeMetrics.GetSumAggrIdleEnergyFromAllSources(collector_metric.PLATFORM)) / miliJouleToJoule)
			ch <- prometheus.MustNewConstMetric(
				p.nodeDesc.nodePlatformJoulesTotal,
				prometheus.CounterValue,
				idlePower,
				collector_metric.NodeName, powerSource, "idle",
			)
		}

		if config.EnabledGPU {
			for gpuID := range p.NodeMetrics.AbsEnergyInGPU.Stat {
				dynPower = (float64(p.NodeMetrics.GetAggrDynEnergyPerID(collector_metric.GPU, gpuID)) / miliJouleToJoule)
				ch <- prometheus.MustNewConstMetric(
					p.nodeDesc.nodeGPUJoulesTotal,
					prometheus.CounterValue,
					dynPower,
					gpuID, collector_metric.NodeName, "nvidia", "dynamic",
				)
				// We do not verify if IsIdlePowerEnabled is enabled for GPUs because pre-trained power models does not estimate GPU power.
				idlePower := (float64(p.NodeMetrics.GetAggrIdleEnergyPerID(collector_metric.GPU, gpuID)) / miliJouleToJoule)
				ch <- prometheus.MustNewConstMetric(
					p.nodeDesc.nodeGPUJoulesTotal,
					prometheus.CounterValue,
					idlePower,
					gpuID, collector_metric.NodeName, "nvidia", "idle",
				)
			}
		}

		if config.EnabledQAT {
			for device, util := range p.NodeMetrics.QATUtilization {
				ch <- prometheus.MustNewConstMetric(
					p.nodeDesc.NodeQATUtilization,
					prometheus.CounterValue,
					float64(util.Latency),
					device, collector_metric.NodeName, "intel-qat", "latency",
				)
			}
		}
	}()
}

// updatePodMetrics send pod metrics to prometheus
func (p *PrometheusCollector) updatePodMetrics(wg *sync.WaitGroup, ch chan<- prometheus.Metric) {
	for _, container := range *p.ContainersMetrics {
		wg.Add(1)
		go func(container *collector_metric.ContainerMetrics) {
			defer wg.Done()
			if container.BPFStats[config.CPUTime] != nil {
				ch <- prometheus.MustNewConstMetric(
					p.containerDesc.containerCPUTime,
					prometheus.CounterValue,
					float64(container.BPFStats[config.CPUTime].Aggr),
					container.ContainerID, container.PodName, container.ContainerName, container.Namespace,
				)
			}
			if container.BPFStats[config.PageCacheHit] != nil {
				ch <- prometheus.MustNewConstMetric(
					p.containerDesc.containerPageCacheHit,
					prometheus.CounterValue,
					float64(container.BPFStats[config.PageCacheHit].Aggr),
					container.ContainerID, container.PodName, container.ContainerName, container.Namespace,
				)
			}
			ch <- prometheus.MustNewConstMetric(
				p.containerDesc.containerCoreJoulesTotal,
				prometheus.CounterValue,
				float64(container.DynEnergyInCore.Aggr)/miliJouleToJoule,
				container.ContainerID, container.PodName, container.ContainerName, container.Namespace, "dynamic",
			)
			ch <- prometheus.MustNewConstMetric(
				p.containerDesc.containerUncoreJoulesTotal,
				prometheus.CounterValue,
				float64(container.DynEnergyInUncore.Aggr)/miliJouleToJoule,
				container.ContainerID, container.PodName, container.ContainerName, container.Namespace, "dynamic",
			)
			ch <- prometheus.MustNewConstMetric(
				p.containerDesc.containerDramJoulesTotal,
				prometheus.CounterValue,
				float64(container.DynEnergyInDRAM.Aggr)/miliJouleToJoule,
				container.ContainerID, container.PodName, container.ContainerName, container.Namespace, "dynamic",
			)
			ch <- prometheus.MustNewConstMetric(
				p.containerDesc.containerPackageJoulesTotal,
				prometheus.CounterValue,
				float64(container.DynEnergyInPkg.Aggr)/miliJouleToJoule,
				container.ContainerID, container.PodName, container.ContainerName, container.Namespace, "dynamic",
			)
			ch <- prometheus.MustNewConstMetric(
				p.containerDesc.containerOtherComponentsJoulesTotal,
				prometheus.CounterValue,
				float64(container.DynEnergyInOther.Aggr)/miliJouleToJoule,
				container.ContainerID, container.PodName, container.ContainerName, container.Namespace, "dynamic",
			)
			if config.IsIdlePowerEnabled() {
				ch <- prometheus.MustNewConstMetric(
					p.containerDesc.containerCoreJoulesTotal,
					prometheus.CounterValue,
					float64(container.IdleEnergyInCore.Aggr)/miliJouleToJoule,
					container.ContainerID, container.PodName, container.ContainerName, container.Namespace, "idle",
				)
				ch <- prometheus.MustNewConstMetric(
					p.containerDesc.containerUncoreJoulesTotal,
					prometheus.CounterValue,
					float64(container.IdleEnergyInUncore.Aggr)/miliJouleToJoule,
					container.ContainerID, container.PodName, container.ContainerName, container.Namespace, "idle",
				)
				ch <- prometheus.MustNewConstMetric(
					p.containerDesc.containerDramJoulesTotal,
					prometheus.CounterValue,
					float64(container.IdleEnergyInDRAM.Aggr)/miliJouleToJoule,
					container.ContainerID, container.PodName, container.ContainerName, container.Namespace, "idle",
				)
				ch <- prometheus.MustNewConstMetric(
					p.containerDesc.containerPackageJoulesTotal,
					prometheus.CounterValue,
					float64(container.IdleEnergyInPkg.Aggr)/miliJouleToJoule,
					container.ContainerID, container.PodName, container.ContainerName, container.Namespace, "idle",
				)
				ch <- prometheus.MustNewConstMetric(
					p.containerDesc.containerOtherComponentsJoulesTotal,
					prometheus.CounterValue,
					float64(container.IdleEnergyInOther.Aggr)/miliJouleToJoule,
					container.ContainerID, container.PodName, container.ContainerName, container.Namespace, "idle",
				)
			}
			if config.EnabledGPU {
				if container.DynEnergyInGPU.Aggr > 0 {
					ch <- prometheus.MustNewConstMetric(
						p.containerDesc.containerGPUJoulesTotal,
						prometheus.CounterValue,
						float64(container.DynEnergyInGPU.Aggr)/miliJouleToJoule,
						container.ContainerID, container.PodName, container.ContainerName, container.Namespace, "dynamic",
					)
				}
				if container.IdleEnergyInGPU.Aggr > 0 {
					// We do not verify if IsIdlePowerEnabled is enabled for GPUs because pre-trained power models does not estimate GPU power.
					ch <- prometheus.MustNewConstMetric(
						p.containerDesc.containerGPUJoulesTotal,
						prometheus.CounterValue,
						float64(container.IdleEnergyInGPU.Aggr)/miliJouleToJoule,
						container.ContainerID, container.PodName, container.ContainerName, container.Namespace, "idle",
					)
				}
			}
			ch <- prometheus.MustNewConstMetric(
				p.containerDesc.containerJoulesTotal,
				prometheus.CounterValue,
				(float64(container.DynEnergyInPkg.Aggr)/miliJouleToJoule +
					float64(container.DynEnergyInUncore.Aggr)/miliJouleToJoule +
					float64(container.DynEnergyInDRAM.Aggr)/miliJouleToJoule +
					float64(container.DynEnergyInGPU.Aggr)/miliJouleToJoule +
					float64(container.DynEnergyInOther.Aggr)/miliJouleToJoule),
				container.ContainerID, container.PodName, container.ContainerName, container.Namespace, "dynamic",
			)
			if config.IsIdlePowerEnabled() {
				ch <- prometheus.MustNewConstMetric(
					p.containerDesc.containerJoulesTotal,
					prometheus.CounterValue,
					(float64(container.IdleEnergyInPkg.Aggr)/miliJouleToJoule +
						float64(container.IdleEnergyInUncore.Aggr)/miliJouleToJoule +
						float64(container.IdleEnergyInDRAM.Aggr)/miliJouleToJoule +
						float64(container.IdleEnergyInGPU.Aggr)/miliJouleToJoule +
						float64(container.IdleEnergyInOther.Aggr)/miliJouleToJoule),
					container.ContainerID, container.PodName, container.ContainerName, container.Namespace, "idle",
				)
			}
			if config.ExposeHardwareCounterMetrics && collector_metric.CPUHardwareCounterEnabled {
				if container.BPFStats[attacher.CPUCycleLabel] != nil {
					ch <- prometheus.MustNewConstMetric(
						p.containerDesc.containerCPUCyclesTotal,
						prometheus.CounterValue,
						float64(container.BPFStats[attacher.CPUCycleLabel].Aggr),
						container.ContainerID, container.PodName, container.ContainerName, container.Namespace,
					)
				}
				if container.BPFStats[attacher.CPUInstructionLabel] != nil {
					ch <- prometheus.MustNewConstMetric(
						p.containerDesc.containerCPUInstrTotal,
						prometheus.CounterValue,
						float64(container.BPFStats[attacher.CPUInstructionLabel].Aggr),
						container.ContainerID, container.PodName, container.ContainerName, container.Namespace,
					)
				}
				if container.BPFStats[attacher.CacheMissLabel] != nil {
					ch <- prometheus.MustNewConstMetric(
						p.containerDesc.containerCacheMissTotal,
						prometheus.CounterValue,
						float64(container.BPFStats[attacher.CacheMissLabel].Aggr),
						container.ContainerID, container.PodName, container.ContainerName, container.Namespace,
					)
				}
			}
			if config.ExposeIRQCounterMetrics {
				if container.BPFStats[config.IRQNetTXLabel] != nil {
					ch <- prometheus.MustNewConstMetric(
						p.containerDesc.containerNetTxIRQTotal,
						prometheus.CounterValue,
						float64(container.BPFStats[config.IRQNetTXLabel].Aggr),
						container.ContainerID, container.PodName, container.ContainerName, container.Namespace,
					)
				}
				if container.BPFStats[config.IRQNetRXLabel] != nil {
					ch <- prometheus.MustNewConstMetric(
						p.containerDesc.containerNetRxIRQTotal,
						prometheus.CounterValue,
						float64(container.BPFStats[config.IRQNetRXLabel].Aggr),
						container.ContainerID, container.PodName, container.ContainerName, container.Namespace,
					)
				}
				if container.BPFStats[config.IRQBlockLabel] != nil {
					ch <- prometheus.MustNewConstMetric(
						p.containerDesc.containerBlockIRQTotal,
						prometheus.CounterValue,
						float64(container.BPFStats[config.IRQBlockLabel].Aggr),
						container.ContainerID, container.PodName, container.ContainerName, container.Namespace,
					)
				}
			}

			if config.ExposeCgroupMetrics && p.HavecGroupsMetric {
				ch <- prometheus.MustNewConstMetric(
					p.containerDesc.containerCgroupCPUUsageUsTotal,
					prometheus.CounterValue,
					float64(container.CgroupStatMap[config.CgroupfsCPU].SumAllAggrValues()),
					container.ContainerID, container.PodName, container.ContainerName, container.Namespace,
				)
				ch <- prometheus.MustNewConstMetric(
					p.containerDesc.containerCgroupMemoryUsageBytesTotal,
					prometheus.CounterValue,
					float64(container.CgroupStatMap[config.CgroupfsMemory].SumAllAggrValues()),
					container.ContainerID, container.PodName, container.ContainerName, container.Namespace,
				)
				ch <- prometheus.MustNewConstMetric(
					p.containerDesc.containerCgroupSystemCPUUsageUsTotal,
					prometheus.CounterValue,
					float64(container.CgroupStatMap[config.CgroupfsSystemCPU].SumAllAggrValues()),
					container.ContainerID, container.PodName, container.ContainerName, container.Namespace,
				)
				ch <- prometheus.MustNewConstMetric(
					p.containerDesc.containerCgroupUserCPUUsageUsTotal,
					prometheus.CounterValue,
					float64(container.CgroupStatMap[config.CgroupfsUserCPU].SumAllAggrValues()),
					container.ContainerID, container.PodName, container.ContainerName, container.Namespace,
				)
			}

			if config.ExposeKubeletMetrics && p.HaveKubletMetric {
				ch <- prometheus.MustNewConstMetric(
					p.containerDesc.containerKubeletCPUUsageTotal,
					prometheus.CounterValue,
					float64(container.KubeletStats[config.KubeletCPUUsage].Aggr),
					container.ContainerID, container.PodName, container.ContainerName, container.Namespace,
				)
				ch <- prometheus.MustNewConstMetric(
					p.containerDesc.containerKubeletMemoryBytesTotal,
					prometheus.CounterValue,
					float64(container.KubeletStats[config.KubeletMemoryUsage].Aggr),
					container.ContainerID, container.PodName, container.ContainerName, container.Namespace,
				)
			}
		}(container)
	}
}
