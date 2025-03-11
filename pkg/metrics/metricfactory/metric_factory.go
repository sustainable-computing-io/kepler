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
	"fmt"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sustainable-computing-io/kepler/pkg/bpf"
	"github.com/sustainable-computing-io/kepler/pkg/config"
	"github.com/sustainable-computing-io/kepler/pkg/metrics/consts"
	modeltypes "github.com/sustainable-computing-io/kepler/pkg/model/types"
	acc "github.com/sustainable-computing-io/kepler/pkg/sensors/accelerator"
	"github.com/sustainable-computing-io/kepler/pkg/sensors/components"
	"github.com/sustainable-computing-io/kepler/pkg/sensors/platform"
	"k8s.io/klog/v2"
)

func EnergyMetricsPromDesc(context string) (descriptions map[string]*prometheus.Desc) {
	descriptions = make(map[string]*prometheus.Desc)
	allComponents := fmt.Sprintf("%s%s%s%s", config.CORE, config.DRAM, config.UNCORE, config.PKG)
	for _, name := range consts.EnergyMetricNames {
		// set the default source to trained power model
		source := modeltypes.TrainedPowerModelSource
		if strings.Contains(name, config.GPU) {
			if gpu := acc.GetActiveAcceleratorByType(config.GPU); gpu != nil {
				source = gpu.Device().Name()
			}
		} else if strings.Contains(name, config.PLATFORM) && platform.IsSystemCollectionSupported() {
			source = platform.GetSourceName()
		} else if strings.Contains(allComponents, name) && components.IsSystemCollectionSupported() {
			// TODO: need to update condition when we have more type of energy metric such as network, disk.
			source = components.GetSourceName()
		}
		// check if trained power model in use here
		if source != modeltypes.TrainedPowerModelSource || !config.DisablePowerModels() {
			descriptions[name] = energyMetricsPromDesc(context, name, source)
		}
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

func HCMetricsPromDesc(context string, bpfSupportedMetrics bpf.SupportedMetrics) (descriptions map[string]*prometheus.Desc) {
	descriptions = make(map[string]*prometheus.Desc)
	for name := range bpfSupportedMetrics.HardwareCounters {
		descriptions[name] = resMetricsPromDesc(context, name, "bpf")
	}
	return descriptions
}

func SCMetricsPromDesc(context string, bpfSupportedMetrics bpf.SupportedMetrics) (descriptions map[string]*prometheus.Desc) {
	descriptions = make(map[string]*prometheus.Desc)
	for name := range bpfSupportedMetrics.SoftwareCounters {
		descriptions[name] = resMetricsPromDesc(context, name, "bpf")
	}
	return descriptions
}

func GPUUsageMetricsPromDesc(context string) (descriptions map[string]*prometheus.Desc) {
	descriptions = make(map[string]*prometheus.Desc)
	if config.IsGPUEnabled() {
		if gpu := acc.GetActiveAcceleratorByType(config.GPU); gpu != nil {
			for _, name := range consts.GPUMetricNames {
				descriptions[name] = resMetricsPromDesc(context, name, gpu.Device().Name())
			}
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
