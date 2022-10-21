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

	"github.com/sustainable-computing-io/kepler/pkg/attacher"
	collector_metric "github.com/sustainable-computing-io/kepler/pkg/collector/metric"
	"github.com/sustainable-computing-io/kepler/pkg/config"

	"github.com/sustainable-computing-io/kepler/pkg/power/rapl/source"
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
		"block_devices_used"}
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

	// Hardware Counters (counter)
	containerCPUCyclesTotal *prometheus.Desc
	containerCPUInstrTotal  *prometheus.Desc
	containerCacheMissTotal *prometheus.Desc

	// Additional metrics (gauge)
	// TODO: review if we really need to expose this metric. cgroup also has some sortof cpuTime metric
	containerCPUTime *prometheus.Desc
}

// Old metric
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

	// TODO: fix me: these metrics should be in NodeMetrics structure
	NodePkgEnergy    *map[int]source.RAPLEnergy
	NodeCPUFrequency *map[int32]uint64

	// NodeMetrics holds all node energy and resource usage metrics
	NodeMetrics *collector_metric.NodeMetrics

	// ContainersMetrics holds all container energy and resource usage metrics
	ContainersMetrics *map[string]*collector_metric.ContainerMetrics

	// SamplePeriodSec the collector metric collection interval
	SamplePeriodSec float64

	// Lock to syncronize the collector update with prometheus exporter
	Mx sync.Mutex
}

// NewPrometheusExporter create and initialize all the PrometheusCollector structures
func NewPrometheusExporter() *PrometheusCollector {
	exporter := PrometheusCollector{
		// prometheus metric descriptions
		nodeDesc: &NodeDesc{},
		podDesc:  &PodDesc{},
	}
	exporter.newNodeMetrics()
	exporter.newContainerMetrics()
	exporter.newPodMetrics()
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

	// container Hardware Counters (counter)
	if collector_metric.CPUHardwareCounterEnabled {
		ch <- p.containerDesc.containerCPUCyclesTotal
		ch <- p.containerDesc.containerCPUInstrTotal
		ch <- p.containerDesc.containerCacheMissTotal
	}

	// Old Node metric
	ch <- p.containerDesc.containerCPUTime
	ch <- p.podDesc.podEnergyStat
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
		[]string{"package", "instance", "source"}, nil,
	)
	nodeUncoreJoulesTotal := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "node", "uncore_joules_total"),
		"Aggregated RAPL value in uncore in joules",
		[]string{"package", "instance", "source"}, nil,
	)
	nodeDramJoulesTotal := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "node", "dram_joules_total"),
		"Aggregated RAPL value in dram in joules",
		[]string{"package", "instance", "source"}, nil,
	)
	nodePackageJoulesTotal := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "node", "package_joules_total"),
		"Aggregated RAPL value in package (socket) in joules",
		[]string{"package", "instance", "source"}, nil,
	)
	nodePlatformJoulesTotal := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "node", "platform_joules_total"),
		"Aggregated RAPL value in platform (entire node) in joules",
		[]string{"instance", "source"}, nil,
	)
	nodeOtherComponentsJoulesTotal := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "node", "other_host_components_joules_total"),
		"Aggregated RAPL value in other components (platform - package - dram) in joules",
		[]string{"instance"}, nil,
	)
	nodeGPUJoulesTotal := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "node", "gpu_joules_total"),
		"Current GPU value in joules",
		[]string{"index", "instance", "source"}, nil,
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
		[]string{"pod_name", "container_name", "container_namespace", "command"}, nil,
	)
	containerUncoreJoulesTotal := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "container", "uncore_joules_total"),
		"Aggregated RAPL value in uncore in joules",
		[]string{"pod_name", "container_name", "container_namespace", "command"}, nil,
	)
	containerDramJoulesTotal := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "container", "dram_joules_total"),
		"Aggregated RAPL value in dram in joules",
		[]string{"pod_name", "container_name", "container_namespace", "command"}, nil,
	)
	containerPackageJoulesTotal := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "container", "package_joules_total"),
		"Aggregated RAPL value in package (socket) in joules",
		[]string{"pod_name", "container_name", "container_namespace", "command"}, nil,
	)
	containerOtherComponentsJoulesTotal := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "container", "other_host_components_joules_total"),
		"Aggregated value in other host components (platform - package - dram) in joules",
		[]string{"pod_name", "container_name", "container_namespace", "command"}, nil,
	)
	containerGPUJoulesTotal := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "container", "gpu_joules_total"),
		"Aggregated GPU value in joules",
		[]string{"pod_name", "container_name", "container_namespace", "command"}, nil,
	)

	// Hardware Counters (counter)
	containerCPUCyclesTotal := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "container", "cpu_cycles_total"),
		"Aggregated CPU cycle value",
		[]string{"pod_name", "container_name", "container_namespace", "command"}, nil,
	)
	containerCPUInstrTotal := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "container", "cpu_instructions_total"),
		"Aggregated CPU instruction value",
		[]string{"pod_name", "container_name", "container_namespace", "command"}, nil,
	)
	containerCacheMissTotal := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "container", "cache_miss_total"),
		"Aggregated cache miss value",
		[]string{"pod_name", "container_name", "container_namespace", "command"}, nil,
	)

	// Additional metrics (gauge)
	containerCPUTime := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "container", "cpu_cpu_time_us"),
		"Current CPU time per CPU",
		[]string{"pod_name", "container_name", "container_namespace", "cpu"}, nil,
	)

	p.containerDesc = &ContainerDesc{
		containerCoreJoulesTotal:            containerCoreJoulesTotal,
		containerUncoreJoulesTotal:          containerUncoreJoulesTotal,
		containerDramJoulesTotal:            containerDramJoulesTotal,
		containerPackageJoulesTotal:         containerPackageJoulesTotal,
		containerOtherComponentsJoulesTotal: containerOtherComponentsJoulesTotal,
		containerGPUJoulesTotal:             containerGPUJoulesTotal,
		containerCPUCyclesTotal:             containerCPUCyclesTotal,
		containerCPUInstrTotal:              containerCPUInstrTotal,
		containerCacheMissTotal:             containerCacheMissTotal,
		containerCPUTime:                    containerCPUTime,
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
	p.UpdateNodeMetrics(&wg, ch)
	p.UpdatePodMetrics(&wg, ch)
	wg.Wait()
}

// UpdateNodeMetrics send node metrics to prometheus
func (p *PrometheusCollector) UpdateNodeMetrics(wg *sync.WaitGroup, ch chan<- prometheus.Metric) {
	// we start with the metrics that might have a longer loop, e.g. range the cpus
	wg.Add(1)
	go func() {
		defer wg.Done()
		// TODO: remove this metric if we don't need, reporting this can be an expensive process
		for cpuID, freq := range *p.NodeCPUFrequency {
			ch <- prometheus.MustNewConstMetric(
				p.nodeDesc.NodeCPUFrequency,
				prometheus.GaugeValue,
				float64(freq),
				fmt.Sprintf("%d", cpuID), collector_metric.NodeName,
			)
		}
		for pkgID, val := range *p.NodePkgEnergy {
			coreEnergy := strconv.FormatUint(val.Core, 10)
			dramEnergy := strconv.FormatUint(val.DRAM, 10)
			uncoreEnergy := strconv.FormatUint(val.Uncore, 10)
			ch <- prometheus.MustNewConstMetric(
				p.nodeDesc.nodePackageMiliJoulesTotal,
				prometheus.CounterValue,
				float64(val.Pkg),
				collector_metric.NodeName, strconv.Itoa(pkgID), coreEnergy, dramEnergy, uncoreEnergy,
			) // deprecated metric
		}
		NodeMetricsStatusLabelValues := []string{collector_metric.NodeName, collector_metric.NodeCPUArchitecture}
		for _, label := range NodeMetricsStatLabels[2:] {
			val := uint64(p.NodeMetrics.Usage[label])
			valStr := strconv.FormatUint(val, 10)
			NodeMetricsStatusLabelValues = append(NodeMetricsStatusLabelValues, valStr)
		}
		ch <- prometheus.MustNewConstMetric(
			p.nodeDesc.NodeMetricsStat,
			prometheus.CounterValue,
			(float64(p.NodeMetrics.SensorEnergy.Curr())/miliJouleToJoule)/p.SamplePeriodSec,
			NodeMetricsStatusLabelValues...,
		)
		ch <- prometheus.MustNewConstMetric(
			p.nodeDesc.nodeInfo,
			prometheus.CounterValue,
			1,
			collector_metric.NodeCPUArchitecture,
		)
		for pkgID, val := range p.NodeMetrics.EnergyInCore.Stat {
			// Node metrics in joules (counter)
			ch <- prometheus.MustNewConstMetric(
				p.nodeDesc.nodeCoreJoulesTotal,
				prometheus.CounterValue,
				float64(val.Aggr)/miliJouleToJoule,
				pkgID, collector_metric.NodeName, "rapl",
			)
		}
		for pkgID, val := range p.NodeMetrics.EnergyInUncore.Stat {
			ch <- prometheus.MustNewConstMetric(
				p.nodeDesc.nodeUncoreJoulesTotal,
				prometheus.CounterValue,
				float64(val.Aggr)/miliJouleToJoule,
				pkgID, collector_metric.NodeName, "rapl",
			)
		}
		for pkgID, val := range p.NodeMetrics.EnergyInDRAM.Stat {
			ch <- prometheus.MustNewConstMetric(
				p.nodeDesc.nodeDramJoulesTotal,
				prometheus.CounterValue,
				float64(val.Aggr)/miliJouleToJoule,
				pkgID, collector_metric.NodeName, "rapl",
			)
		}
		for pkgID, val := range p.NodeMetrics.EnergyInPkg.Stat {
			ch <- prometheus.MustNewConstMetric(
				p.nodeDesc.nodePackageJoulesTotal,
				prometheus.CounterValue,
				float64(val.Aggr)/miliJouleToJoule,
				pkgID, collector_metric.NodeName, "rapl",
			)
		}
		ch <- prometheus.MustNewConstMetric(
			p.nodeDesc.nodePlatformJoulesTotal,
			prometheus.CounterValue,
			float64(p.NodeMetrics.SensorEnergy.Aggr())/miliJouleToJoule,
			collector_metric.NodeName, "acpi",
		)
		ch <- prometheus.MustNewConstMetric(
			p.nodeDesc.nodeOtherComponentsJoulesTotal,
			prometheus.CounterValue,
			float64(p.NodeMetrics.EnergyInOther.Aggr())/miliJouleToJoule,
			collector_metric.NodeName,
		)
		if config.EnabledGPU {
			for gpuID, val := range p.NodeMetrics.EnergyInGPU.Stat {
				ch <- prometheus.MustNewConstMetric(
					p.nodeDesc.nodeGPUJoulesTotal,
					prometheus.CounterValue,
					float64(val.Aggr)/miliJouleToJoule,
					gpuID, collector_metric.NodeName, "nvidia",
				)
			}
		}
	}()
}

// updatePodMetrics send pod metrics to prometheus
func (p *PrometheusCollector) UpdatePodMetrics(wg *sync.WaitGroup, ch chan<- prometheus.Metric) {
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
				float64(container.Curr()),
				podEnergyStatusLabelValues...,
			)
			for cpu, cpuTime := range container.CurrCPUTimePerCPU {
				ch <- prometheus.MustNewConstMetric(
					p.containerDesc.containerCPUTime,
					prometheus.GaugeValue,
					float64(cpuTime),
					container.PodName, container.ContainerName, container.Namespace, strconv.Itoa(int(cpu)),
				)
			}
			ch <- prometheus.MustNewConstMetric(
				p.containerDesc.containerCoreJoulesTotal,
				prometheus.CounterValue,
				float64(container.EnergyInCore.Aggr)/miliJouleToJoule,
				container.PodName, container.ContainerName, container.Namespace, containerCommand,
			)
			ch <- prometheus.MustNewConstMetric(
				p.containerDesc.containerUncoreJoulesTotal,
				prometheus.CounterValue,
				float64(container.EnergyInUncore.Aggr)/miliJouleToJoule,
				container.PodName, container.ContainerName, container.Namespace, containerCommand,
			)
			ch <- prometheus.MustNewConstMetric(
				p.containerDesc.containerDramJoulesTotal,
				prometheus.CounterValue,
				float64(container.EnergyInDRAM.Aggr)/miliJouleToJoule,
				container.PodName, container.ContainerName, container.Namespace, containerCommand,
			)
			ch <- prometheus.MustNewConstMetric(
				p.containerDesc.containerPackageJoulesTotal,
				prometheus.CounterValue,
				float64(container.EnergyInPkg.Aggr)/miliJouleToJoule,
				container.PodName, container.ContainerName, container.Namespace, containerCommand,
			)
			ch <- prometheus.MustNewConstMetric(
				p.containerDesc.containerOtherComponentsJoulesTotal,
				prometheus.CounterValue,
				float64(container.EnergyInOther.Aggr)/miliJouleToJoule,
				container.PodName, container.ContainerName, container.Namespace, containerCommand,
			)
			if config.EnabledGPU {
				ch <- prometheus.MustNewConstMetric(
					p.containerDesc.containerGPUJoulesTotal,
					prometheus.CounterValue,
					float64(container.EnergyInGPU.Aggr)/miliJouleToJoule,
					container.PodName, container.ContainerName, container.Namespace, containerCommand,
				)
			}
			if collector_metric.CPUHardwareCounterEnabled {
				if container.CounterStats[attacher.CPUCycleLable] != nil {
					ch <- prometheus.MustNewConstMetric(
						p.containerDesc.containerCPUCyclesTotal,
						prometheus.CounterValue,
						float64(container.CounterStats[attacher.CPUCycleLable].Aggr),
						container.PodName, container.ContainerName, container.Namespace, containerCommand,
					)
				}
				if container.CounterStats[attacher.CPUInstructionLabel] != nil {
					ch <- prometheus.MustNewConstMetric(
						p.containerDesc.containerCPUInstrTotal,
						prometheus.CounterValue,
						float64(container.CounterStats[attacher.CPUInstructionLabel].Aggr),
						container.PodName, container.ContainerName, container.Namespace, containerCommand,
					)
				}
				if container.CounterStats[attacher.CacheMissLabel] != nil {
					ch <- prometheus.MustNewConstMetric(
						p.containerDesc.containerCacheMissTotal,
						prometheus.CounterValue,
						float64(container.CounterStats[attacher.CacheMissLabel].Aggr),
						container.PodName, container.ContainerName, container.Namespace, containerCommand,
					)
				}
			}
		}(container)
	}
}
