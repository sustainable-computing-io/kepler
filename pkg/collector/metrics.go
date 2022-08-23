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
	"github.com/sustainable-computing-io/kepler/pkg/pod_lister"

	"log"
)

const (
	FREQ_METRIC_LABEL = "avg_cpu_frequency"

	// TO-DO: merge to cgroup stat
	BYTE_READ_LABEL    = "bytes_read"
	BYTE_WRITE_LABEL   = "bytes_writes"
	BLOCK_DEVICE_LABEL = "block_devices_used"

	CPU_TIME_LABEL = "cpu_time"
)

var (
	FLOAT_FEATURES []string = []string{}

	availableCounters       []string = attacher.GetEnabledCounters()
	availableCgroupMetrics  []string = cgroup.GetAvailableCgroupMetrics()
	availableKubeletMetrics []string = pod_lister.GetAvailableKubeletMetrics()
	// TO-DO: merge to cgroup stat and remove hard-code metric list
	IOSTAT_METRICS []string = []string{BYTE_READ_LABEL, BYTE_WRITE_LABEL}
	uintFeatures   []string = getUIntFeatures()
	features       []string = append(FLOAT_FEATURES, uintFeatures...)
	metricNames    []string = getEstimatorMetrics()
)

func getUIntFeatures() []string {
	var metrics []string
	metrics = append(metrics, CPU_TIME_LABEL)
	// counter metric
	metrics = append(metrics, availableCounters...)
	// cgroup metric
	metrics = append(metrics, availableCgroupMetrics...)
	// kubelet metric
	metrics = append(metrics, availableKubeletMetrics...)
	metrics = append(metrics, IOSTAT_METRICS...)
	log.Printf("Available counter metrics: %v", availableCounters)
	log.Printf("Available cgroup metrics: %v", availableCgroupMetrics)
	log.Printf("Available kubelet metrics: %v", availableKubeletMetrics)
	return metrics
}

func getPrometheusMetrics() []string {
	var labels []string
	for _, feature := range features {
		labels = append(labels, CURR_PREFIX+feature)
		labels = append(labels, AGGR_PREFIX+feature)
	}
	if attacher.EnableCPUFreq {
		labels = append(labels, FREQ_METRIC_LABEL)
	}
	// TO-DO: remove this hard code metric
	labels = append(labels, BLOCK_DEVICE_LABEL)
	return labels
}

func getEstimatorMetrics() []string {
	var metricNames []string
	for _, feature := range features {
		metricNames = append(metricNames, CURR_PREFIX+feature)
	}
	// TO-DO: remove this hard code metric
	metricNames = append(metricNames, BLOCK_DEVICE_LABEL)
	model.InitMetricIndexes(metricNames)
	return metricNames
}
