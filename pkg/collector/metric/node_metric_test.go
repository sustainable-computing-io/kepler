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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Test Node Metric", func() {

	nodeMap := make(map[string]float64)
	nodeMap["cgroupfs_memory_usage_bytes"] = 100
	nodeMap["cgroupfs_kernel_memory_usage_bytes"] = 200
	nodeMap["cgroupfs_tcp_memory_usage_bytes"] = 300

	It("Test sumUsage", func() {
		ne := NodeMetrics{
			Usage:          nodeMap,
			EnergyInCore:   &UInt64StatCollection{},
			EnergyInDRAM:   &UInt64StatCollection{},
			EnergyInUncore: &UInt64StatCollection{},
			EnergyInPkg:    &UInt64StatCollection{},
			EnergyInGPU:    &UInt64StatCollection{},
			EnergyInOther:  &UInt64StatCollection{},
			SensorEnergy:   &UInt64StatCollection{},
		}
		podMetricValues := make([][]float64, 0)
		podMetricValues = append(podMetricValues, []float64{100, 100, 100})
		nodeUsageValues, nodeUsageMap := ne.sumUsage(podMetricValues)
		Expect(len(nodeUsageMap)).To(Equal(1))
		v, ok := nodeUsageMap["cgroupfs_memory_usage_bytes"]
		Expect(ok).To(Equal(false))
		Expect(v).To(Equal(float64(0)))
		Expect(len(nodeUsageValues)).To(Equal(1))
		Expect(nodeUsageValues[0]).To(Equal(float64(100)))
	})

	It("Test GetPrometheusEnergyValue", func() {
		ne := NodeMetrics{
			Usage: nodeMap,
			EnergyInCore: &UInt64StatCollection{
				Stat: map[string]*UInt64Stat{
					"0": {
						Curr: 123,
					},
				},
			},
			EnergyInDRAM:   &UInt64StatCollection{},
			EnergyInUncore: &UInt64StatCollection{},
			EnergyInPkg:    &UInt64StatCollection{},
			EnergyInGPU:    &UInt64StatCollection{},
			EnergyInOther:  &UInt64StatCollection{},
			SensorEnergy:   &UInt64StatCollection{},
		}

		out := ne.GetPrometheusEnergyValue("core")
		Expect(out).To(Equal(uint64(123)))
	})
})
