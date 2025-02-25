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

package stats

import (
	"k8s.io/klog/v2"

	"math"

	"github.com/sustainable-computing-io/kepler/pkg/config"
	acc "github.com/sustainable-computing-io/kepler/pkg/sensors/accelerator"
)

type Coordinate struct {
	X, Y float64
}

func percentDiff(a, b float64) float64 {
	if a == 0 || b == 0 {
		return 1.0
	}
	return (math.Abs(a-b) / math.Min(a, b))
}

func appendToSliceWithSizeRestriction(slice *[]float64, length int, value float64) {
	if len(*slice) >= length {
		//ic.result.history = append(ic.result.history[1:], ic.result.calculatedIdleEnergy)
		copy(*slice, (*slice)[1:])
		// After copying, the 0th element is removed and the 4th element is a duplicate.
		// Now we can replace the 4th element with new idleEnergy
		// pointers required to modify array
		(*slice)[length-1] = value
	} else {
		*slice = append(*slice, value)
	}

}

func getAverage(slice []float64) float64 {
	var sum float64 = 0
	for _, value := range slice {
		sum += value
	}
	return sum / float64(len(slice))
}

func checkSliceAllSame(slice []float64) bool {
	if len(slice) == 0 {
		return true
	}
	firstElem := slice[0]
	for _, value := range slice {
		if value != firstElem {
			return false
		}
	}
	return true
}

func GetProcessFeatureNames() []string {
	var metrics []string
	// bpf counter metrics
	metrics = append(metrics, AvailableBPFMetrics()...)
	klog.V(3).Infof("Available ebpf counters: %v", metrics)

	// gpu metric
	if config.IsGPUEnabled() {
		if acc.GetActiveAcceleratorByType(config.GPU) != nil {
			gpuMetrics := []string{config.GPUComputeUtilization, config.GPUMemUtilization}
			metrics = append(metrics, gpuMetrics...)
			klog.V(3).Infof("Available GPU metrics: %v", gpuMetrics)
		}
	}

	return metrics
}
