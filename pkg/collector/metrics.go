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
	"github.com/sustainable-computing-io/kepler/pkg/attacher"
	"github.com/sustainable-computing-io/kepler/pkg/cgroup"
	"github.com/sustainable-computing-io/kepler/pkg/model"
	"github.com/sustainable-computing-io/kepler/pkg/podlister"

	"k8s.io/klog/v2"
)

const (
	freqMetricLabel = "avg_cpu_frequency"

	// TO-DO: merge to cgroup stat
	ByteReadLabel    = "bytes_read"
	ByteWriteLabel   = "bytes_writes"
	blockDeviceLabel = "block_devices_used"

	CPUTimeLabel = "cpu_time"
)

var (
	FloatFeatures []string = []string{}

	availableCounters       []string = attacher.GetEnabledCounters()
	availableCgroupMetrics  []string = cgroup.GetAvailableCgroupMetrics()
	availableKubeletMetrics []string = podlister.GetAvailableKubeletMetrics()
	// TO-DO: merge to cgroup stat and remove hard-code metric list
	iostatMetrics []string = []string{ByteReadLabel, ByteWriteLabel}
	uintFeatures  []string
	features      []string
	metricNames   []string

	cpuInstrCounterEnabled = isCounterStatEnabled(attacher.CPUInstructionLabel)
)

func getUIntFeatures() []string {
	var metrics []string
	metrics = append(metrics, CPUTimeLabel)

	// counter metric
	metrics = append(metrics, availableCounters...)
	// cgroup metric
	metrics = append(metrics, availableCgroupMetrics...)
	// kubelet metric
	metrics = append(metrics, availableKubeletMetrics...)

	metrics = append(metrics, iostatMetrics...)

	klog.V(3).Infof("Available counter metrics: %v", availableCounters)
	klog.V(3).Infof("Available cgroup metrics: %v", availableCgroupMetrics)
	klog.V(3).Infof("Available kubelet metrics: %v", availableKubeletMetrics)

	return metrics
}

func SetEnabledMetrics() {
	availableCounters = attacher.GetEnabledCounters()

	uintFeatures = getUIntFeatures()
	features = append(features, FloatFeatures...)
	features = append(features, uintFeatures...)
	metricNames = getEstimatorMetrics()
}

func getPrometheusMetrics() []string {
	var labels []string
	for _, feature := range features {
		labels = append(labels, CurrPrefix+feature, AggrPrefix+feature)
	}
	if attacher.EnableCPUFreq {
		labels = append(labels, freqMetricLabel)
	}
	// TO-DO: remove this hard code metric
	labels = append(labels, blockDeviceLabel)
	return labels
}

func getEstimatorMetrics() []string {
	var names []string
	for _, feature := range features {
		names = append(names, CurrPrefix+feature)
	}
	// TO-DO: remove this hard code metric
	names = append(names, blockDeviceLabel)
	model.InitMetricIndexes(names)
	return names
}

func isCounterStatEnabled(label string) bool {
	for _, counter := range availableCounters {
		if counter == label {
			return true
		}
	}
	return false
}
