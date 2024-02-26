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

package metricfactory

import (
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sustainable-computing-io/kepler/pkg/config"
	"github.com/sustainable-computing-io/kepler/pkg/metrics/consts"
	modeltypes "github.com/sustainable-computing-io/kepler/pkg/model/types"
	"github.com/sustainable-computing-io/kepler/pkg/sensors/accelerator/gpu"
	"github.com/sustainable-computing-io/kepler/pkg/sensors/components"
	"github.com/sustainable-computing-io/kepler/pkg/sensors/platform"
	"k8s.io/klog/v2"
)

func EnergyMetricsPromDesc(context string) (descriptions map[string]*prometheus.Desc) {
	descriptions = make(map[string]*prometheus.Desc)
	for _, name := range consts.EnergyMetricNames {
		// set the default source to trained power model
		source := modeltypes.TrainedPowerModelSource
		if strings.Contains(name, config.GPU) {
			source = modeltypes.GPUEnergySource
		} else if strings.Contains(name, config.PLATFORM) && platform.IsSystemCollectionSupported() {
			source = modeltypes.PlatformEnergySource
		} else if components.IsSystemCollectionSupported() {
			// TODO: need to update condition when we have more type of energy metric such as network, disk.
			source = modeltypes.ComponentEnergySource
		}
		descriptions[name] = energyMetricsPromDesc(context, name, source)
	}
	return descriptions
}

func energyMetricsPromDesc(context, name, source string) (desc *prometheus.Desc) {
	var labels []string
	switch context {
	case "process":
		labels = consts.ProcessEnergyLabels
	case "container":
		labels = consts.ContainerEnergyLabels
	case "vm":
		labels = consts.VMEnergyLabels
	case "node":
		labels = consts.NodeEnergyLabels
	default:
		klog.Errorf("Unexpected prometheus context: %s", context)
		return
	}
	return MetricsPromDesc(context, name, consts.EnergyMetricNameSuffix, source, labels)
}

func HCMetricsPromDesc(context string) (descriptions map[string]*prometheus.Desc) {
	descriptions = make(map[string]*prometheus.Desc)
	if config.IsHCMetricsEnabled() {
		for _, name := range consts.HCMetricNames {
			descriptions[name] = resMetricsPromDesc(context, name, "bpf")
		}
	}
	return descriptions
}

func SCMetricsPromDesc(context string) (descriptions map[string]*prometheus.Desc) {
	descriptions = make(map[string]*prometheus.Desc)
	for _, name := range consts.SCMetricNames {
		descriptions[name] = resMetricsPromDesc(context, name, "bpf")
	}
	return descriptions
}

func IRQMetricsPromDesc(context string) (descriptions map[string]*prometheus.Desc) {
	descriptions = make(map[string]*prometheus.Desc)
	if config.IsIRQCounterMetricsEnabled() {
		for _, name := range consts.IRQMetricNames {
			descriptions[name] = resMetricsPromDesc(context, name, "bpf")
		}
	}
	return descriptions
}

func CGroupMetricsPromDesc(context string) (descriptions map[string]*prometheus.Desc) {
	descriptions = make(map[string]*prometheus.Desc)
	if config.IsCgroupMetricsEnabled() {
		for _, name := range consts.CGroupMetricNames {
			descriptions[name] = resMetricsPromDesc(context, name, "cgroup")
		}
	}
	return descriptions
}

func QATMetricsPromDesc(context string) (descriptions map[string]*prometheus.Desc) {
	descriptions = make(map[string]*prometheus.Desc)
	if config.IsExposeQATMetricsEnabled() {
		name := config.QATUtilization
		descriptions[name] = resMetricsPromDesc(context, name, "intel_qat")
	}
	return descriptions
}

func NodeCPUFrequencyMetricsPromDesc(context string) (descriptions map[string]*prometheus.Desc) {
	descriptions = make(map[string]*prometheus.Desc)
	if config.IsExposeCPUFrequencyMetricsEnabled() {
		name := config.CPUFrequency
		descriptions[name] = resMetricsPromDesc(context, name, "")
	}
	return descriptions
}

func GPUUsageMetricsPromDesc(context string) (descriptions map[string]*prometheus.Desc) {
	descriptions = make(map[string]*prometheus.Desc)
	if config.EnabledGPU && gpu.IsGPUCollectionSupported() {
		for _, name := range consts.GPUMetricNames {
			descriptions[name] = resMetricsPromDesc(context, name, "nvidia-nvml")
		}
	}
	return descriptions
}

func resMetricsPromDesc(context, name, source string) (desc *prometheus.Desc) {
	var labels []string
	switch context {
	case "process":
		labels = consts.ProcessResUtilLabels
	case "container":
		labels = consts.ContainerResUtilLabels
	case "vm":
		labels = consts.VMResUtilLabels
	case "node":
		labels = consts.NodeResUtilLabels
	default:
		klog.Errorf("Unexpected prometheus context: %s", context)
		return
	}
	// if this is a GPU metric, we need to add the GPU ID label
	for _, gpuMetric := range consts.GPUMetricNames {
		if name == gpuMetric {
			labels = append(labels, consts.GPUResUtilLabels...)
		}
	}
	return MetricsPromDesc(context, name, consts.UsageMetricNameSuffix, source, labels)
}

func MetricsPromDesc(context, name, suffix, source string, labels []string) (desc *prometheus.Desc) {
	return prometheus.NewDesc(
		prometheus.BuildFQName(consts.MetricsNamespace, context, name+suffix),
		"Aggregated value in "+name+" value from "+source,
		labels,
		prometheus.Labels{"source": source},
	)
}
