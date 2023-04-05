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
package metric_test

import (
	"strconv"
	"testing"

	"github.com/sustainable-computing-io/kepler/pkg/collector/metric"
)

func benchmarkNtesting(b *testing.B, continerNumber int) {
	var containerMetrics map[string]*metric.ContainerMetrics
	nodeMetrics := metric.NewNodeMetrics()
	containerMetrics = make(map[string]*metric.ContainerMetrics)
	for i := 0; i < continerNumber; i++ {
		containerMetrics["container"+strconv.Itoa(i)] = createMockContainerMetrics("container"+strconv.Itoa(i), "podA", "test")
	}
	b.ReportAllocs()
	b.ResetTimer()
	nodeMetrics.AddNodeResUsageFromContainerResUsage(containerMetrics)
	b.StopTimer()
}

func BenchmarkAddNodeResUsageFromContainerResUsageWith1000Contianer(b *testing.B) {
	benchmarkNtesting(b, 1000)
}

func BenchmarkAddNodeResUsageFromContainerResUsageWith2000Contianer(b *testing.B) {
	benchmarkNtesting(b, 2000)
}

func BenchmarkAddNodeResUsageFromContainerResUsageWith5000Contianer(b *testing.B) {
	benchmarkNtesting(b, 5000)
}

func BenchmarkAddNodeResUsageFromContainerResUsageWith10000Contianer(b *testing.B) {
	benchmarkNtesting(b, 10000)
}

// see usageMetrics for the list of used metrics. For the sake of visibility we add all metrics, but only few of them will be used.
func createMockContainerMetrics(containerName, podName, namespace string) *metric.ContainerMetrics {
	containerMetrics := metric.NewContainerMetrics(containerName, podName, namespace, containerName)
	// bpf - cpu time
	_ = containerMetrics.CPUTime.AddNewDelta(10) // config.CPUTime
	return containerMetrics
}
