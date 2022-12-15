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

package metric

import (
	"github.com/sustainable-computing-io/kepler/pkg/bpfassets/attacher"
	"github.com/sustainable-computing-io/kepler/pkg/config"
	"github.com/sustainable-computing-io/kepler/pkg/power/accelerator"

	"k8s.io/klog/v2"
)

func getcontainerUintFeatureNames() []string {
	var metrics []string
	metrics = append(metrics, CPUTimeLabel)

	// counter metric
	metrics = append(metrics, AvailableCounters...)
	// cgroup metric
	metrics = append(metrics, AvailableCgroupMetrics...)
	// cgroup kubelet metric
	metrics = append(metrics, AvailableKubeletMetrics...)
	// cgroup I/O metric
	metrics = append(metrics, ContainerIOStatMetricsNames...)
	// gpu metric
	if config.EnabledGPU && accelerator.IsGPUCollectionSupported() {
		metrics = append(metrics, []string{config.GPUSMUtilization, config.GPUMemUtilization}...)
	}

	klog.V(3).Infof("Available counter metrics: %v", AvailableCounters)
	klog.V(3).Infof("Available cgroup metrics from cgroup: %v", AvailableCgroupMetrics)
	klog.V(3).Infof("Available cgroup metrics from kubelet: %v", AvailableKubeletMetrics)
	klog.V(3).Infof("Available I/O metrics: %v", ContainerIOStatMetricsNames)

	return metrics
}

func setEnabledMetrics() []string {
	ContainerFeaturesNames = []string{}
	AvailableCounters = attacher.GetEnabledCounters()

	ContainerUintFeaturesNames = getcontainerUintFeatureNames()
	ContainerFeaturesNames = append(ContainerFeaturesNames, ContainerFloatFeatureNames...)
	ContainerFeaturesNames = append(ContainerFeaturesNames, ContainerUintFeaturesNames...)
	ContainerMetricNames = getEstimatorMetrics()
	return ContainerMetricNames
}

func getPrometheusMetrics() []string {
	var labels []string
	for _, feature := range ContainerFeaturesNames {
		labels = append(labels, CurrPrefix+feature, AggrPrefix+feature)
	}
	// TO-DO: remove this hard code metric
	labels = append(labels, blockDeviceLabel)
	return labels
}

func getEstimatorMetrics() []string {
	var metricNames []string
	metricNames = append(metricNames, ContainerFeaturesNames...)
	// TO-DO: remove this hard code metric
	metricNames = append(metricNames, blockDeviceLabel)

	return metricNames
}

func isCounterStatEnabled(label string) bool {
	for _, counter := range AvailableCounters {
		if counter == label {
			return true
		}
	}
	return false
}
