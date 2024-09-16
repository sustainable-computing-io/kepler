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

	"github.com/sustainable-computing-io/kepler/pkg/bpf"
	"github.com/sustainable-computing-io/kepler/pkg/config"
	acc "github.com/sustainable-computing-io/kepler/pkg/sensors/accelerator"
)

func GetProcessFeatureNames(bpfSupportedMetrics bpf.SupportedMetrics) []string {
	var metrics []string
	// bpf software counter metrics
	for counterKey := range bpfSupportedMetrics.SoftwareCounters {
		metrics = append(metrics, counterKey)
	}
	// bpf hardware counter metrics
	for counterKey := range bpfSupportedMetrics.HardwareCounters {
		metrics = append(metrics, counterKey)
	}
	klog.V(3).Infof("Available ebpf counters: %v", metrics)

	// gpu metric
	if config.EnabledGPU() {
		if acc.GetRegistry().ActiveAcceleratorByType(acc.GPU) != nil {
			gpuMetrics := []string{config.GPUComputeUtilization, config.GPUMemUtilization}
			metrics = append(metrics, gpuMetrics...)
			klog.V(3).Infof("Available GPU metrics: %v", gpuMetrics)
		}
	}

	return metrics
}
