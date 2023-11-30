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
	"github.com/sustainable-computing-io/kepler/pkg/sensors/accelerator/qat"
	"k8s.io/klog/v2"
)

// UpdateNodeQATMetrics update QAT metrics from telemetry data
func UpdateNodeQATMetrics(nodeStats *stats.NodeStats) {
	// get available QAT devices
	devices := qat.GetQATs()
	if len(devices) != 0 {
		deviceUtil, err := qat.GetQATUtilization(devices)
		if err != nil {
			klog.V(5).Infoln(err)
		}
		for devID, sample := range deviceUtil {
			nodeStats.ResourceUsage[config.QATUtilization].SetDeltaStat(devID, sample.SampleCnt)
		}
	} else {
		klog.V(5).Infoln("Unable to find an available QAT. Please check the status of QAT again")
	}
}
