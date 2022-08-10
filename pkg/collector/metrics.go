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
	"fmt"
	"log"
	"strconv"
)

const (
	FREQ_METRIC_LABEL = "avg_cpu_frequency"
	CURR_PREFIX = "curr_"
	AGGR_PREFIX = "total_"

	// TO-DO: merge to cgroup stat
	BYTE_READ_LABEL = "bytes_read"
	BYTE_WRITE_LABEL = "bytes_writes"
	BLOCK_DEVICE_LABEL = "block_devices_used"

	CPU_TIME_LABEL = "cpu_time"
	
)
// TO-DO: merge to cgroup stat and remove hard-code metric list
var IOSTAT_METRICS []string = []string {BYTE_READ_LABEL, BYTE_WRITE_LABEL}
var FLOAT_FEATURES []string = []string {CPU_TIME_LABEL}

func GetUIntFeatures() []string {
	var metrics []string
	// counter metric
	metrics = append(metrics, attacher.GetEnabledCounters()...)
	// cgroup metric
	metrics = append(metrics, cgroup.GetAvailableCgroupMetrics()...)
	metrics = append(metrics, IOSTAT_METRICS...)
	return metrics
}

func getCollectedLabels(floatFeatures, uintFeatures []string) []string {
	var labels []string
	features := append(floatFeatures, uintFeatures...)
	for _, feature := range features {
		labels = append(labels, CURR_PREFIX + feature)
		labels = append(labels, AGGR_PREFIX + feature)
	}
	if attacher.EnableCPUFreq {
		labels = append(labels, FREQ_METRIC_LABEL)
	}
	// TO-DO: remove this hard code metric 
	labels = append(labels, BLOCK_DEVICE_LABEL)
	return labels
}

func convertCollectedValues(floatFeatures, uintFeatures []string, podEnergy *PodEnergy) []string{
	var values []string

	for _, metric := range floatFeatures {
		curr, aggr, err := converFloatCurrAggr(metric, podEnergy)
		if err != nil {
			log.Printf("convertCollectedValues: %v", err)
		}
		values = append(values, fmt.Sprintf("%f", curr))
		values = append(values, fmt.Sprintf("%f", aggr))
	}

	for _, metric := range uintFeatures {
		curr, aggr, err := convertUIntCurrAggr(metric, podEnergy)
		if err != nil {
			log.Printf("convertCollectedValues: %v", err)
		}
		values = append(values, strconv.FormatUint(curr, 10))
		values = append(values, strconv.FormatUint(aggr, 10))
	}

	if attacher.EnableCPUFreq {
		avgFreq := fmt.Sprintf("%f", float64(podEnergy.AvgCPUFreq))
		values = append(values, avgFreq)
	}

	// TO-DO: remove this hard code metric 
	disks := fmt.Sprintf("%d", podEnergy.Disks)
	values = append(values, disks)
	return values
}

// convertUIntCurrAggr return curr, aggr values of specific uint metric
func convertUIntCurrAggr(metric string, podEnergy *PodEnergy) (uint64, uint64, error) {
	// cgroup metrics
	if statValue, exists := podEnergy.CgroupFSStats[metric]; exists {
		return statValue.Curr, statValue.Aggr, nil
	}
	// hardcode cgroup metrics
	// TO-DO: merge to cgroup stat
	if metric == BYTE_READ_LABEL {
		return podEnergy.CurrBytesRead, podEnergy.AggBytesRead, nil
	}
	if metric == BYTE_WRITE_LABEL {
		return podEnergy.CurrBytesWrite, podEnergy.AggBytesWrite, nil
	}
	// counter metrics
	switch metric {
	case attacher.CPU_CYCLE_LABEL:
		return podEnergy.CurrCacheMisses, podEnergy.AggCacheMisses, nil
	case attacher.CPU_INSTRUCTION_LABEL:
		return podEnergy.CurrCPUInstr, podEnergy.AggCPUInstr, nil
	case attacher.CACHE_MISS_LABEL:
		return podEnergy.CurrCacheMisses, podEnergy.AggCacheMisses, nil
	default:
		return 0, 0, fmt.Errorf("cannot extract metric %s", metric)
	}
}

func converFloatCurrAggr(metric string, podEnergy *PodEnergy) (float64, float64, error) {
	// TO-DO: remove hard code
	if metric == CPU_TIME_LABEL {
		return float64(podEnergy.CurrCPUTime), float64(podEnergy.AggCPUTime), nil
	}
	return 0, 0, fmt.Errorf("cannot extract metric %s", metric)
}




