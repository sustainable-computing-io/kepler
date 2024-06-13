/*
Copyright 2023.

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

package utils

import (
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sustainable-computing-io/kepler/pkg/bpf"
	"github.com/sustainable-computing-io/kepler/pkg/collector/stats"
	"github.com/sustainable-computing-io/kepler/pkg/config"
	"github.com/sustainable-computing-io/kepler/pkg/metrics/consts"
	"github.com/sustainable-computing-io/kepler/pkg/metrics/metricfactory"
	"github.com/sustainable-computing-io/kepler/pkg/model/utils"
	acc "github.com/sustainable-computing-io/kepler/pkg/sensors/accelerator"
	"k8s.io/klog/v2"
)

var JouleMillijouleConversionFactor = utils.JouleMillijouleConversionFactor

func CollectEnergyMetrics(ch chan<- prometheus.Metric, instance interface{}, collectors map[string]metricfactory.PromMetric) {
	// collect the dynamic energy metrics
	for i, collectorName := range consts.EnergyMetricNames {
		if collectorName == config.GPU && !config.EnabledGPU {
			continue
		}
		collectEnergy(ch, instance, consts.DynEnergyMetricNames[i], "dynamic", collectors[collectorName])
		// idle power is not enabled by default on VMs, since it is the host idle power and was not split among all running VMs
		if config.IsIdlePowerEnabled() {
			collectEnergy(ch, instance, consts.IdleEnergyMetricNames[i], "idle", collectors[collectorName])
		}
	}
}

func CollectResUtilizationMetrics(ch chan<- prometheus.Metric, instance interface{}, collectors map[string]metricfactory.PromMetric, bpfSupportedMetrics bpf.SupportedMetrics) {
	// collect the BPF Software Counters
	for collectorName := range bpfSupportedMetrics.SoftwareCounters {
		CollectResUtil(ch, instance, collectorName, collectors[collectorName])
	}
	for collectorName := range bpfSupportedMetrics.HardwareCounters {
		CollectResUtil(ch, instance, collectorName, collectors[collectorName])
	}
	if config.EnabledGPU {
		if _, err := acc.Registry().ActiveAcceleratorsByType(acc.GPU); err == nil {
			for _, collectorName := range consts.GPUMetricNames {
				CollectResUtil(ch, instance, collectorName, collectors[collectorName])
			}
		}
	}
}

func collect(ch chan<- prometheus.Metric, collector metricfactory.PromMetric, value float64, labelValues []string) {
	ch <- collector.MustMetric(value, labelValues...)
}

func collectEnergy(ch chan<- prometheus.Metric, instance interface{}, metricName, mode string, collector metricfactory.PromMetric) {
	var value float64
	var labelValues []string
	switch v := instance.(type) {
	case *stats.ContainerStats:
		container := instance.(*stats.ContainerStats)
		value = float64(container.EnergyUsage[metricName].SumAllAggrValues()) / JouleMillijouleConversionFactor
		labelValues = []string{container.ContainerID, container.PodName, container.ContainerName, container.Namespace, mode}
		collect(ch, collector, value, labelValues)

	case *stats.ProcessStats:
		process := instance.(*stats.ProcessStats)
		value = float64(process.EnergyUsage[metricName].SumAllAggrValues()) / JouleMillijouleConversionFactor
		labelValues = []string{strconv.FormatUint(process.PID, 10), process.ContainerID, process.VMID, process.Command, mode}
		collect(ch, collector, value, labelValues)

	case *stats.VMStats:
		vm := instance.(*stats.VMStats)
		value = float64(vm.EnergyUsage[metricName].SumAllAggrValues()) / JouleMillijouleConversionFactor
		labelValues = []string{vm.VMID, mode}
		collect(ch, collector, value, labelValues)

	// only node metrics report metrics per device, process, container and VM reports the aggregation
	case *stats.NodeStats:
		node := instance.(*stats.NodeStats)
		if _, exist := node.EnergyUsage[metricName]; exist {
			for deviceID, utilization := range node.EnergyUsage[metricName].Stat {
				value = float64(utilization.Aggr) / JouleMillijouleConversionFactor
				labelValues = []string{deviceID, stats.NodeName, mode}
				collect(ch, collector, value, labelValues)
			}
		}

	default:
		klog.Errorf("Type %T is not known!\n", v)
	}
}

func convertUnit(metricName string, val uint64) float64 {
	if metricName == config.CPUTime {
		// convert microseconds to miliseconds
		return float64(val) / 1000.0
	}
	return float64(val)
}

func CollectResUtil(ch chan<- prometheus.Metric, instance interface{}, metricName string, collector metricfactory.PromMetric) {
	var value float64
	var labelValues []string
	switch v := instance.(type) {
	case *stats.ContainerStats:
		container := instance.(*stats.ContainerStats)
		// special case for GPU devices, the metrics are reported per device
		isGPUMetric := false
		for _, m := range consts.GPUMetricNames {
			if metricName == m {
				isGPUMetric = true
				break
			}
		}
		if isGPUMetric {
			for deviceID, utilization := range container.ResourceUsage[metricName].Stat {
				value = convertUnit(metricName, utilization.Aggr)
				labelValues = []string{container.ContainerID, container.PodName, container.ContainerName, container.Namespace, deviceID}
				collect(ch, collector, value, labelValues)
			}
		} else {
			if _, exist := container.ResourceUsage[metricName]; !exist {
				klog.Errorf("ContainerStats %s does not have metric %s\n", container.ContainerID, metricName)
				return
			}
			value = convertUnit(metricName, container.ResourceUsage[metricName].SumAllAggrValues())
			labelValues = []string{container.ContainerID, container.PodName, container.ContainerName, container.Namespace}
			collect(ch, collector, value, labelValues)
		}

	case *stats.ProcessStats:
		process := instance.(*stats.ProcessStats)
		value = convertUnit(metricName, process.ResourceUsage[metricName].SumAllAggrValues())
		labelValues = []string{strconv.FormatUint(process.PID, 10), process.ContainerID, process.VMID, process.Command}
		collect(ch, collector, value, labelValues)

	case *stats.VMStats:
		vm := instance.(*stats.VMStats)
		value = convertUnit(metricName, vm.ResourceUsage[metricName].SumAllAggrValues())
		labelValues = []string{vm.VMID}
		collect(ch, collector, value, labelValues)

	// only node metrics report metrics per device, process, container and VM reports the aggregation
	case *stats.NodeStats:
		node := instance.(*stats.NodeStats)
		if _, exist := node.ResourceUsage[metricName]; exist {
			for deviceID, utilization := range node.ResourceUsage[metricName].Stat {
				value = convertUnit(metricName, utilization.Aggr)
				labelValues = []string{deviceID, stats.NodeName}
				collect(ch, collector, value, labelValues)
			}
		}

	default:
		klog.Errorf("Type %T is not supported.\n", v)
	}
}
