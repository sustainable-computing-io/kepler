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
	"strconv"
	"sync"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/sustainable-computing-io/kepler/pkg/bpfassets/attacher"
	collector_metric "github.com/sustainable-computing-io/kepler/pkg/collector/metric"
	"github.com/sustainable-computing-io/kepler/pkg/config"
)

const (
	namespace        = "kepler"
	miliJouleToJoule = 1000
)

var (
	// The energy stat metrics are meat to be only used by the model server for training purpose
	// These metris report the following labels, we have the full list here to make it more transparent
	// TODO: review these metrics, they might be deprecated
	NodeMetricsStatLabels = []string{
		"node_name",
		"cpu_architecture",
		"node_curr_cpu_time",
		"node_curr_cpu_cycles",
		"node_curr_cpu_instr",
		"node_curr_cache_miss",
		"node_curr_container_cpu_usage_seconds_total",
		"node_curr_container_memory_working_set_bytes",
		"node_curr_bytes_read",
		"node_curr_bytes_writes",
		"node_block_devices_used",
		"node_curr_energy_in_core_joule",
		"node_curr_energy_in_dram_joule",
		"node_curr_energy_in_gpu_joule",
		"node_curr_energy_in_other_joule",
		"node_curr_energy_in_pkg_joule",
		"node_curr_energy_in_uncore_joule"}
	podEnergyStatLabels = []string{
		"pod_name",
		"container_name",
		"pod_namespace",
		"command",
		"curr_cpu_time",
		"total_cpu_time",
		"curr_cpu_cycles",
		"total_cpu_cycles",
		"curr_cpu_instr",
		"total_cpu_instr",
		"curr_cache_miss",
		"total_cache_miss",
		"curr_container_cpu_usage_seconds_total",
		"total_container_cpu_usage_seconds_total",
		"curr_container_memory_working_set_bytes",
		"total_container_memory_working_set_bytes",
		"curr_bytes_read",
		"total_bytes_read",
		"curr_bytes_writes",
		"total_bytes_writes",
		"block_devices_used",
		"curr_irq_net_rx",
		"total_irq_net_rx",
		"curr_irq_net_tx",
		"total_irq_net_tx",
		"curr_irq_block",
		"total_irq_block"}
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

	// Old metric
	// TODO: remove these metrics in the next release. The dependent components must stop to use this.
	nodePackageMiliJoulesTotal *prometheus.Desc // deprecated
	// the NodeMetricsStat does not follow the prometheus metrics standard guideline, and this is only used by the model server.
	NodeMetricsStat *prometheus.Desc
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
	// TODO: review if we really need to expose this metric. cgroup also has some sortof cpuTime metric
	containerCPUTime *prometheus.Desc

	// IRQ metrics
	containerNetTxIRQTotal *prometheus.Desc
	containerNetRxIRQTotal *prometheus.Desc
	containerBlockIRQTotal *prometheus.Desc
}

// metric used by the model server to train the model
type PodDesc struct {
	// TODO: review if we need to remove this metric
	// the NodeMetricsStat does not follow the prometheus metrics standard guideline, and this is only used by the model server.
	podEnergyStat *prometheus.Desc
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

	// NodeMetrics holds all node energy and resource usage metrics
	NodeMetrics *collector_metric.NodeMetrics

	// ContainersMetrics holds all container energy and resource usage metrics
	ContainersMetrics *map[string]*collector_metric.ContainerMetrics

	// ProcessMetrics hold all process energy and resource usage metrics
	ProcessMetrics *map[uint64]*collector_metric.ProcessMetrics

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
	}
	exporter.newNodeMetrics()
	exporter.newContainerMetrics()
	exporter.newPodMetrics()
	exporter.newprocessMetrics()
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

	// Additional Node metrics (gauge)
	ch <- p.nodeDesc.NodeCPUFrequency

	// Old Node metric
	ch <- p.nodeDesc.nodePackageMiliJoulesTotal
	ch <- p.nodeDesc.NodeMetricsStat

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
	if config.ExposeCgroupMetrics {
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
	if config.ExposeKubeletMetrics {
		if len(collector_metric.AvailableKubeletMetrics) != 0 {
			ch <- p.containerDesc.containerKubeletCPUUsageTotal
			ch <- p.containerDesc.containerKubeletMemoryBytesTotal
			p.HaveKubletMetric = true
		} else {
			p.HaveKubletMetric = false
		}
	}

	// Old Node metric
	ch <- p.containerDesc.containerCPUTime
	ch <- p.podDesc.podEnergyStat

	// IRQ counter
	if config.ExposeIRQCounterMetrics {
		ch <- p.containerDesc.containerNetTxIRQTotal
		ch <- p.containerDesc.containerNetRxIRQTotal
		ch <- p.containerDesc.containerBlockIRQTotal
	}
	p.describeProcess(ch)
}

func (p *PrometheusCollector) newNodeMetrics() {
	nodeInfo := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "node", "nodeInfo"),
		"Labeled node information",
		[]string{"cpu_architecture"}, nil,
	)
	// Energy (counter)
	// TODO: separate the energy consumption per CPU, including the label cpu
	nodeCoreJoulesTotal := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "node", "core_joules_total"),
		"Aggregated RAPL value in core in joules",
		[]string{"package", "instance", "source", "mode"}, nil,
	)
	nodeUncoreJoulesTotal := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "node", "uncore_joules_total"),
		"Aggregated RAPL value in uncore in joules",
		[]string{"package", "instance", "source", "mode"}, nil,
	)
	nodeDramJoulesTotal := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "node", "dram_joules_total"),
		"Aggregated RAPL value in dram in joules",
		[]string{"package", "instance", "source", "mode"}, nil,
	)
	nodePackageJoulesTotal := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "node", "package_joules_total"),
		"Aggregated RAPL value in package (socket) in joules",
		[]string{"package", "instance", "source", "mode"}, nil,
	)
	nodePlatformJoulesTotal := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "node", "platform_joules_total"),
		"Aggregated RAPL value in platform (entire node) in joules",
		[]string{"instance", "source", "mode"}, nil,
	)
	nodeOtherComponentsJoulesTotal := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "node", "other_host_components_joules_total"),
		"Aggregated RAPL value in other components (platform - package - dram) in joules",
		[]string{"instance", "mode"}, nil,
	)
	nodeGPUJoulesTotal := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "node", "gpu_joules_total"),
		"Current GPU value in joules",
		[]string{"index", "instance", "source", "mode"}, nil,
	)

	// Additional metrics (gauge)
	NodeCPUFrequency := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "node", "cpu_scaling_frequency_hertz"),
		"Current average cpu frequency in hertz",
		[]string{"cpu", "instance"}, nil,
	)

	// Old metrics
	nodePackageMiliJoulesTotal := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "node", "package_energy_millijoule"),
		"Aggregated RAPL value in package (socket) in milijoules (deprecated)",
		[]string{"instance", "pkg_id", "core", "dram", "uncore"}, nil,
	)
	NodeMetricsStat := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "node", "energy_stat"),
		"Several labeled node metrics",
		NodeMetricsStatLabels, nil,
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
		nodePackageMiliJoulesTotal:     nodePackageMiliJoulesTotal, // deprecated
		NodeMetricsStat:                NodeMetricsStat,
	}
}

func (p *PrometheusCollector) newContainerMetrics() {
	// Energy (counter)
	containerCoreJoulesTotal := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "container", "core_joules_total"),
		"Aggregated RAPL value in core in joules",
		[]string{"container_id", "pod_name", "container_name", "container_namespace", "command", "mode"}, nil,
	)
	containerUncoreJoulesTotal := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "container", "uncore_joules_total"),
		"Aggregated RAPL value in uncore in joules",
		[]string{"container_id", "pod_name", "container_name", "container_namespace", "command", "mode"}, nil,
	)
	containerDramJoulesTotal := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "container", "dram_joules_total"),
		"Aggregated RAPL value in dram in joules",
		[]string{"container_id", "pod_name", "container_name", "container_namespace", "command", "mode"}, nil,
	)
	containerPackageJoulesTotal := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "container", "package_joules_total"),
		"Aggregated RAPL value in package (socket) in joules",
		[]string{"container_id", "pod_name", "container_name", "container_namespace", "command", "mode"}, nil,
	)
	containerOtherComponentsJoulesTotal := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "container", "other_host_components_joules_total"),
		"Aggregated value in other host components (platform - package - dram) in joules",
		[]string{"container_id", "pod_name", "container_name", "container_namespace", "command", "mode"}, nil,
	)
	containerGPUJoulesTotal := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "container", "gpu_joules_total"),
		"Aggregated GPU value in joules",
		[]string{"container_id", "pod_name", "container_name", "container_namespace", "command", "mode"}, nil,
	)
	containerJoulesTotal := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "container", "joules_total"),
		"Aggregated RAPL Package + Uncore + DRAM + GPU + other host components (platform - package - dram) in joules",
		[]string{"container_id", "pod_name", "container_name", "container_namespace", "command", "mode"}, nil,
	)

	// Hardware Counters (counter)
	containerCPUCyclesTotal := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "container", "cpu_cycles_total"),
		"Aggregated CPU cycle value",
		[]string{"container_id", "pod_name", "container_name", "container_namespace", "command"}, nil,
	)
	containerCPUInstrTotal := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "container", "cpu_instructions_total"),
		"Aggregated CPU instruction value",
		[]string{"container_id", "pod_name", "container_name", "container_namespace", "command"}, nil,
	)
	containerCacheMissTotal := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "container", "cache_miss_total"),
		"Aggregated cache miss value",
		[]string{"container_id", "pod_name", "container_name", "container_namespace", "command"}, nil,
	)

	// cGroups Counters (counter)
	containerCgroupCPUUsageUsTotal := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "container", "cgroupfs_cpu_usage_us_total"),
		"Aggregated cpu usage obtained from cGroups",
		[]string{"container_id", "pod_name", "container_name", "container_namespace", "command"}, nil,
	)

	containerCgroupMemoryUsageBytesTotal := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "container", "cgroupfs_memory_usage_bytes_total"),
		"Aggregated memory bytes obtained from cGroups",
		[]string{"container_id", "pod_name", "container_name", "container_namespace", "command"}, nil,
	)

	containerCgroupSystemCPUUsageUsTotal := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "container", "cgroupfs_system_cpu_usage_us_total"),
		"Aggregated system cpu usage obtained from cGroups",
		[]string{"container_id", "pod_name", "container_name", "container_namespace", "command"}, nil,
	)

	containerCgroupUserCPUUsageUsTotal := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "container", "cgroupfs_user_cpu_usage_us_total"),
		"Aggregated user cpu usage obtained from cGroups",
		[]string{"container_id", "pod_name", "container_name", "container_namespace", "command"}, nil,
	)

	// Kubelet Counters (counter)
	containerKubeletCPUUsageTotal := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "container", "kubelet_cpu_usage_total"),
		"Aggregated cpu usage obtained from kubelet",
		[]string{"container_id", "pod_name", "container_name", "container_namespace", "command"}, nil,
	)

	containerKubeletMemoryBytesTotal := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "container", "kubelet_memory_bytes_total"),
		"Aggregated memory bytes obtained from kubelet",
		[]string{"container_id", "pod_name", "container_name", "container_namespace", "command"}, nil,
	)

	// Additional metrics (gauge)
	containerCPUTime := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "container", "bpf_cpu_time_us_total"),
		"Aggregated CPU time obtained from BPF",
		[]string{"container_id", "pod_name", "container_name", "container_namespace"}, nil,
	)

	// network irq metrics
	containerNetTxIRQTotal := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "container", "bpf_net_tx_irq_total"),
		"Aggregated network tx irq value obtained from BPF",
		[]string{"container_id", "pod_name", "container_name", "container_namespace"}, nil,
	)
	containerNetRxIRQTotal := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "container", "bpf_net_rx_irq_total"),
		"Aggregated network rx irq value obtained from BPF",
		[]string{"container_id", "pod_name", "container_name", "container_namespace"}, nil,
	)
	containerBlockIRQTotal := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "container", "bpf_block_irq_total"),
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
		containerNetTxIRQTotal:               containerNetTxIRQTotal,
		containerNetRxIRQTotal:               containerNetRxIRQTotal,
		containerBlockIRQTotal:               containerBlockIRQTotal,
	}
}

func (p *PrometheusCollector) newPodMetrics() {
	// Old metrics
	podEnergyStat := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "pod", "energy_stat"),
		"Several labeled pod metrics",
		podEnergyStatLabels, nil,
	)
	podCPUInstrTotal := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "pod", "cpu_instructions"),
		"Aggregated CPU instruction value (deprecated)",
		[]string{"pod_name", "container_name", "container_namespace", "command"}, nil,
	)

	p.podDesc = &PodDesc{
		podEnergyStat:    podEnergyStat,
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
		for pkgID, val := range p.NodeMetrics.TotalEnergyInPkg.Stat {
			coreEnergy := strconv.FormatUint(p.NodeMetrics.TotalEnergyInCore.Stat[pkgID].Delta, 10)
			dramEnergy := strconv.FormatUint(p.NodeMetrics.TotalEnergyInDRAM.Stat[pkgID].Delta, 10)
			uncoreEnergy := strconv.FormatUint(p.NodeMetrics.TotalEnergyInUncore.Stat[pkgID].Delta, 10)
			ch <- prometheus.MustNewConstMetric(
				p.nodeDesc.nodePackageMiliJoulesTotal,
				prometheus.CounterValue,
				float64(val.Delta),
				collector_metric.NodeName, pkgID, coreEnergy, dramEnergy, uncoreEnergy,
			) // deprecated metric
		}

		NodeMetricsStatusLabelValues := []string{collector_metric.NodeName, collector_metric.NodeCPUArchitecture}
		for _, label := range NodeMetricsStatLabels[2:] {
			val := uint64(p.NodeMetrics.ResourceUsage[label])
			valStr := strconv.FormatUint(val, 10)
			NodeMetricsStatusLabelValues = append(NodeMetricsStatusLabelValues, valStr)
		}
		ch <- prometheus.MustNewConstMetric(
			p.nodeDesc.NodeMetricsStat,
			prometheus.CounterValue,
			(float64(p.NodeMetrics.TotalEnergyInPlatform.SumAllDeltaValues())/miliJouleToJoule)/p.SamplePeriodSec,
			NodeMetricsStatusLabelValues...,
		)
		ch <- prometheus.MustNewConstMetric(
			p.nodeDesc.nodeInfo,
			prometheus.CounterValue,
			1,
			collector_metric.NodeCPUArchitecture,
		)
		// Node metrics in joules (counter)
		for pkgID := range p.NodeMetrics.TotalEnergyInCore.Stat {
			dynPower := (float64(p.NodeMetrics.GetAggrDynEnergyPerID(collector_metric.PKG, pkgID)) / miliJouleToJoule)
			ch <- prometheus.MustNewConstMetric(
				p.nodeDesc.nodePackageJoulesTotal,
				prometheus.CounterValue,
				dynPower,
				pkgID, collector_metric.NodeName, "rapl", "dynamic",
			)
			idlePower := (float64(p.NodeMetrics.GetAggrIdleEnergyPerID(collector_metric.PKG, pkgID)) / miliJouleToJoule)
			ch <- prometheus.MustNewConstMetric(
				p.nodeDesc.nodePackageJoulesTotal,
				prometheus.CounterValue,
				idlePower,
				pkgID, collector_metric.NodeName, "rapl", "idle",
			)

			dynPower = (float64(p.NodeMetrics.GetAggrDynEnergyPerID(collector_metric.CORE, pkgID)) / miliJouleToJoule)
			ch <- prometheus.MustNewConstMetric(
				p.nodeDesc.nodeCoreJoulesTotal,
				prometheus.CounterValue,
				dynPower,
				pkgID, collector_metric.NodeName, "rapl", "dynamic",
			)
			idlePower = (float64(p.NodeMetrics.GetAggrIdleEnergyPerID(collector_metric.CORE, pkgID)) / miliJouleToJoule)
			ch <- prometheus.MustNewConstMetric(
				p.nodeDesc.nodeCoreJoulesTotal,
				prometheus.CounterValue,
				idlePower,
				pkgID, collector_metric.NodeName, "rapl", "idle",
			)

			dynPower = (float64(p.NodeMetrics.GetAggrDynEnergyPerID(collector_metric.UNCORE, pkgID)) / miliJouleToJoule)
			ch <- prometheus.MustNewConstMetric(
				p.nodeDesc.nodeUncoreJoulesTotal,
				prometheus.CounterValue,
				dynPower,
				pkgID, collector_metric.NodeName, "rapl", "dynamic",
			)
			idlePower = (float64(p.NodeMetrics.GetAggrIdleEnergyPerID(collector_metric.UNCORE, pkgID)) / miliJouleToJoule)
			ch <- prometheus.MustNewConstMetric(
				p.nodeDesc.nodeUncoreJoulesTotal,
				prometheus.CounterValue,
				idlePower,
				pkgID, collector_metric.NodeName, "rapl", "idle",
			)
			dynPower = (float64(p.NodeMetrics.GetAggrDynEnergyPerID(collector_metric.DRAM, pkgID)) / miliJouleToJoule)
			ch <- prometheus.MustNewConstMetric(
				p.nodeDesc.nodeDramJoulesTotal,
				prometheus.CounterValue,
				dynPower,
				pkgID, collector_metric.NodeName, "rapl", "dynamic",
			)
			idlePower = (float64(p.NodeMetrics.GetAggrIdleEnergyPerID(collector_metric.DRAM, pkgID)) / miliJouleToJoule)
			ch <- prometheus.MustNewConstMetric(
				p.nodeDesc.nodeDramJoulesTotal,
				prometheus.CounterValue,
				idlePower,
				pkgID, collector_metric.NodeName, "rapl", "idle",
			)
		}

		dynPower := (float64(p.NodeMetrics.GetSumAggrDynEnergyFromAllSources(collector_metric.OTHER)) / miliJouleToJoule)
		ch <- prometheus.MustNewConstMetric(
			p.nodeDesc.nodeOtherComponentsJoulesTotal,
			prometheus.CounterValue,
			dynPower,
			collector_metric.NodeName, "dynamic",
		)
		idlePower := (float64(p.NodeMetrics.GetSumAggrDynEnergyFromAllSources(collector_metric.OTHER)) / miliJouleToJoule)
		ch <- prometheus.MustNewConstMetric(
			p.nodeDesc.nodeOtherComponentsJoulesTotal,
			prometheus.CounterValue,
			idlePower,
			collector_metric.NodeName, "idle",
		)

		dynPower = (float64(p.NodeMetrics.GetSumAggrDynEnergyFromAllSources(collector_metric.PLATFORM)) / miliJouleToJoule)
		ch <- prometheus.MustNewConstMetric(
			p.nodeDesc.nodePlatformJoulesTotal,
			prometheus.CounterValue,
			dynPower,
			collector_metric.NodeName, "acpi", "dynamic",
		)
		idlePower = (float64(p.NodeMetrics.GetSumAggrDynEnergyFromAllSources(collector_metric.PLATFORM)) / miliJouleToJoule)
		ch <- prometheus.MustNewConstMetric(
			p.nodeDesc.nodePlatformJoulesTotal,
			prometheus.CounterValue,
			idlePower,
			collector_metric.NodeName, "acpi", "idle",
		)

		if config.EnabledGPU {
			for gpuID := range p.NodeMetrics.TotalEnergyInGPU.Stat {
				dynPower = (float64(p.NodeMetrics.GetAggrDynEnergyPerID(collector_metric.GPU, gpuID)) / miliJouleToJoule)
				ch <- prometheus.MustNewConstMetric(
					p.nodeDesc.nodeGPUJoulesTotal,
					prometheus.CounterValue,
					dynPower,
					gpuID, collector_metric.NodeName, "nvidia", "dynamic",
				)
				idlePower = (float64(p.NodeMetrics.GetAggrIdleEnergyPerID(collector_metric.GPU, gpuID)) / miliJouleToJoule)
				ch <- prometheus.MustNewConstMetric(
					p.nodeDesc.nodeGPUJoulesTotal,
					prometheus.CounterValue,
					idlePower,
					gpuID, collector_metric.NodeName, "nvidia", "idle",
				)
			}
		}
	}()
}

// updatePodMetrics send pod metrics to prometheus
func (p *PrometheusCollector) updatePodMetrics(wg *sync.WaitGroup, ch chan<- prometheus.Metric) {
	const commandLenLimit = 10
	for _, container := range *p.ContainersMetrics {
		wg.Add(1)
		go func(container *collector_metric.ContainerMetrics) {
			defer wg.Done()
			containerCommand := container.Command
			if len(containerCommand) > commandLenLimit {
				containerCommand = container.Command[:commandLenLimit]
			}
			// TODO: After removing this metric in the next release, we need to refactor and remove the ToPrometheusValues function
			podEnergyStatusLabelValues := []string{container.PodName, container.ContainerName, container.Namespace, containerCommand}
			for _, label := range podEnergyStatLabels[4:] {
				val := container.ToPrometheusValue(label)
				podEnergyStatusLabelValues = append(podEnergyStatusLabelValues, val)
			}
			ch <- prometheus.MustNewConstMetric(
				p.podDesc.podEnergyStat,
				prometheus.GaugeValue,
				float64(container.SumAllDynDeltaValues()),
				podEnergyStatusLabelValues...,
			)
			ch <- prometheus.MustNewConstMetric(
				p.containerDesc.containerCPUTime,
				prometheus.CounterValue,
				float64(container.CPUTime.Aggr),
				container.ContainerID, container.PodName, container.ContainerName, container.Namespace,
			)
			ch <- prometheus.MustNewConstMetric(
				p.containerDesc.containerCoreJoulesTotal,
				prometheus.CounterValue,
				float64(container.DynEnergyInCore.Aggr)/miliJouleToJoule,
				container.ContainerID, container.PodName, container.ContainerName, container.Namespace, containerCommand, "dynamic",
			)
			ch <- prometheus.MustNewConstMetric(
				p.containerDesc.containerCoreJoulesTotal,
				prometheus.CounterValue,
				float64(container.IdleEnergyInCore.Aggr)/miliJouleToJoule,
				container.ContainerID, container.PodName, container.ContainerName, container.Namespace, containerCommand, "idle",
			)
			ch <- prometheus.MustNewConstMetric(
				p.containerDesc.containerUncoreJoulesTotal,
				prometheus.CounterValue,
				float64(container.DynEnergyInUncore.Aggr)/miliJouleToJoule,
				container.ContainerID, container.PodName, container.ContainerName, container.Namespace, containerCommand, "dynamic",
			)
			ch <- prometheus.MustNewConstMetric(
				p.containerDesc.containerUncoreJoulesTotal,
				prometheus.CounterValue,
				float64(container.IdleEnergyInUncore.Aggr)/miliJouleToJoule,
				container.ContainerID, container.PodName, container.ContainerName, container.Namespace, containerCommand, "idle",
			)
			ch <- prometheus.MustNewConstMetric(
				p.containerDesc.containerDramJoulesTotal,
				prometheus.CounterValue,
				float64(container.DynEnergyInDRAM.Aggr)/miliJouleToJoule,
				container.ContainerID, container.PodName, container.ContainerName, container.Namespace, containerCommand, "dynamic",
			)
			ch <- prometheus.MustNewConstMetric(
				p.containerDesc.containerDramJoulesTotal,
				prometheus.CounterValue,
				float64(container.IdleEnergyInDRAM.Aggr)/miliJouleToJoule,
				container.ContainerID, container.PodName, container.ContainerName, container.Namespace, containerCommand, "idle",
			)
			ch <- prometheus.MustNewConstMetric(
				p.containerDesc.containerPackageJoulesTotal,
				prometheus.CounterValue,
				float64(container.DynEnergyInPkg.Aggr)/miliJouleToJoule,
				container.ContainerID, container.PodName, container.ContainerName, container.Namespace, containerCommand, "dynamic",
			)
			ch <- prometheus.MustNewConstMetric(
				p.containerDesc.containerPackageJoulesTotal,
				prometheus.CounterValue,
				float64(container.IdleEnergyInPkg.Aggr)/miliJouleToJoule,
				container.ContainerID, container.PodName, container.ContainerName, container.Namespace, containerCommand, "idle",
			)
			ch <- prometheus.MustNewConstMetric(
				p.containerDesc.containerOtherComponentsJoulesTotal,
				prometheus.CounterValue,
				float64(container.IdleEnergyInOther.Aggr)/miliJouleToJoule,
				container.ContainerID, container.PodName, container.ContainerName, container.Namespace, containerCommand, "idle",
			)
			if config.EnabledGPU {
				if container.DynEnergyInGPU.Aggr > 0 {
					ch <- prometheus.MustNewConstMetric(
						p.containerDesc.containerGPUJoulesTotal,
						prometheus.CounterValue,
						float64(container.DynEnergyInGPU.Aggr)/miliJouleToJoule,
						container.ContainerID, container.PodName, container.ContainerName, container.Namespace, containerCommand, "dynamic",
					)
				}
				if container.IdleEnergyInGPU.Aggr > 0 {
					ch <- prometheus.MustNewConstMetric(
						p.containerDesc.containerGPUJoulesTotal,
						prometheus.CounterValue,
						float64(container.IdleEnergyInGPU.Aggr)/miliJouleToJoule,
						container.ContainerID, container.PodName, container.ContainerName, container.Namespace, containerCommand, "idle",
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
				container.ContainerID, container.PodName, container.ContainerName, container.Namespace, containerCommand, "dynamic",
			)
			ch <- prometheus.MustNewConstMetric(
				p.containerDesc.containerJoulesTotal,
				prometheus.CounterValue,
				(float64(container.IdleEnergyInPkg.Aggr)/miliJouleToJoule +
					float64(container.IdleEnergyInUncore.Aggr)/miliJouleToJoule +
					float64(container.IdleEnergyInDRAM.Aggr)/miliJouleToJoule +
					float64(container.IdleEnergyInGPU.Aggr)/miliJouleToJoule +
					float64(container.IdleEnergyInOther.Aggr)/miliJouleToJoule),
				container.ContainerID, container.PodName, container.ContainerName, container.Namespace, containerCommand, "idle",
			)
			if config.ExposeHardwareCounterMetrics && collector_metric.CPUHardwareCounterEnabled {
				if container.CounterStats[attacher.CPUCycleLabel] != nil {
					ch <- prometheus.MustNewConstMetric(
						p.containerDesc.containerCPUCyclesTotal,
						prometheus.CounterValue,
						float64(container.CounterStats[attacher.CPUCycleLabel].Aggr),
						container.ContainerID, container.PodName, container.ContainerName, container.Namespace, containerCommand,
					)
				}
				if container.CounterStats[attacher.CPUInstructionLabel] != nil {
					ch <- prometheus.MustNewConstMetric(
						p.containerDesc.containerCPUInstrTotal,
						prometheus.CounterValue,
						float64(container.CounterStats[attacher.CPUInstructionLabel].Aggr),
						container.ContainerID, container.PodName, container.ContainerName, container.Namespace, containerCommand,
					)
				}
				if container.CounterStats[attacher.CacheMissLabel] != nil {
					ch <- prometheus.MustNewConstMetric(
						p.containerDesc.containerCacheMissTotal,
						prometheus.CounterValue,
						float64(container.CounterStats[attacher.CacheMissLabel].Aggr),
						container.ContainerID, container.PodName, container.ContainerName, container.Namespace, containerCommand,
					)
				}
			}

			if config.ExposeCgroupMetrics && p.HavecGroupsMetric {
				ch <- prometheus.MustNewConstMetric(
					p.containerDesc.containerCgroupCPUUsageUsTotal,
					prometheus.CounterValue,
					float64(container.CgroupStatMap[config.CgroupfsCPU].SumAllAggrValues()),
					container.ContainerID, container.PodName, container.ContainerName, container.Namespace, containerCommand,
				)
				ch <- prometheus.MustNewConstMetric(
					p.containerDesc.containerCgroupMemoryUsageBytesTotal,
					prometheus.CounterValue,
					float64(container.CgroupStatMap[config.CgroupfsMemory].SumAllAggrValues()),
					container.ContainerID, container.PodName, container.ContainerName, container.Namespace, containerCommand,
				)
				ch <- prometheus.MustNewConstMetric(
					p.containerDesc.containerCgroupSystemCPUUsageUsTotal,
					prometheus.CounterValue,
					float64(container.CgroupStatMap[config.CgroupfsSystemCPU].SumAllAggrValues()),
					container.ContainerID, container.PodName, container.ContainerName, container.Namespace, containerCommand,
				)
				ch <- prometheus.MustNewConstMetric(
					p.containerDesc.containerCgroupUserCPUUsageUsTotal,
					prometheus.CounterValue,
					float64(container.CgroupStatMap[config.CgroupfsUserCPU].SumAllAggrValues()),
					container.ContainerID, container.PodName, container.ContainerName, container.Namespace, containerCommand,
				)
			}

			if config.ExposeKubeletMetrics && p.HaveKubletMetric {
				ch <- prometheus.MustNewConstMetric(
					p.containerDesc.containerKubeletCPUUsageTotal,
					prometheus.CounterValue,
					float64(container.KubeletStats[config.KubeletContainerCPU].Aggr),
					container.ContainerID, container.PodName, container.ContainerName, container.Namespace, containerCommand,
				)
				ch <- prometheus.MustNewConstMetric(
					p.containerDesc.containerKubeletMemoryBytesTotal,
					prometheus.CounterValue,
					float64(container.KubeletStats[config.KubeletContainerMemory].Aggr),
					container.ContainerID, container.PodName, container.ContainerName, container.Namespace, containerCommand,
				)
			}

			if config.ExposeIRQCounterMetrics {
				ch <- prometheus.MustNewConstMetric(
					p.containerDesc.containerNetTxIRQTotal,
					prometheus.CounterValue,
					float64(container.SoftIRQCount[attacher.IRQNetTX].Aggr),
					container.ContainerID, container.PodName, container.ContainerName, container.Namespace,
				)
				ch <- prometheus.MustNewConstMetric(
					p.containerDesc.containerNetRxIRQTotal,
					prometheus.CounterValue,
					float64(container.SoftIRQCount[attacher.IRQNetRX].Aggr),
					container.ContainerID, container.PodName, container.ContainerName, container.Namespace,
				)
				ch <- prometheus.MustNewConstMetric(
					p.containerDesc.containerBlockIRQTotal,
					prometheus.CounterValue,
					float64(container.SoftIRQCount[attacher.IRQBlock].Aggr),
					container.ContainerID, container.PodName, container.ContainerName, container.Namespace,
				)
			}
		}(container)
	}
}
