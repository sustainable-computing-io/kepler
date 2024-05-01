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

package accelerator

import (
	"github.com/sustainable-computing-io/kepler/pkg/collector/stats"
	"github.com/sustainable-computing-io/kepler/pkg/config"
	acc "github.com/sustainable-computing-io/kepler/pkg/sensors/accelerator"
	"github.com/sustainable-computing-io/kepler/pkg/sensors/accelerator/device"
	"k8s.io/klog/v2"
)

// UpdateNodeQATMetrics update QAT metrics from telemetry data
func UpdateNodeQATMetrics(nodeStats *stats.NodeStats) {
	// get available QAT devices
	if devices, err := acc.GetActiveAcceleratorsByType("qat"); err == nil {
		for _, a := range devices {
			d := a.GetAccelerator()
			for _, q := range d.GetDevicesByName() {
				var deviceUtil map[any]any
				if deviceUtil, err = d.GetDeviceUtilizationStats(q); err != nil {
					klog.Infoln(err)
					return
				}
				for devID, sample := range deviceUtil {
					nodeStats.ResourceUsage[config.QATUtilization].SetDeltaStat(devID.(string), sample.(device.QATUtilizationSample).SampleCnt)
				}
			}
		}
	}
}
