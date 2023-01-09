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
	"github.com/sustainable-computing-io/kepler/pkg/cgroup"
	collector_metric "github.com/sustainable-computing-io/kepler/pkg/collector/metric"

	"k8s.io/klog/v2"
)

// updateCgroupMetrics adds container-level cgroup data
func (c *Collector) updateCgroupMetrics() {
	klog.V(5).Infof("overall cgroup stats %v", cgroup.SliceHandlerInstance)

	for containerID := range c.ContainersMetrics {
		cgroup.TryInitStatReaders(containerID)
		cgroupFSStandardStats := cgroup.GetStandardStat(containerID)
		for cgroupFSKey, cgroupFSValue := range cgroupFSStandardStats {
			readVal := cgroupFSValue.(uint64)
			if _, ok := c.ContainersMetrics[containerID].CgroupFSStats[cgroupFSKey]; ok {
				c.ContainersMetrics[containerID].CgroupFSStats[cgroupFSKey].AddAggrStat(containerID, readVal)
			}
		}
	}
}

// updateKubeletMetrics adds kubelet data (resident mem)
func (c *Collector) updateKubeletMetrics() {
	if len(collector_metric.AvailableKubeletMetrics) == 2 {
		containerCPU, containerMem, _, _, _ := cgroup.GetContainerMetrics()
		klog.V(5).Infof("Kubelet Read: %v, %v\n", containerCPU, containerMem)
		for _, c := range c.ContainersMetrics {
			k := c.Namespace + "/" + c.PodName + "/" + c.ContainerName
			readCPU := uint64(containerCPU[k])
			readMem := uint64(containerMem[k])
			cpuMetricName := collector_metric.AvailableKubeletMetrics[0]
			memMetricName := collector_metric.AvailableKubeletMetrics[1]
			if err := c.KubeletStats[cpuMetricName].SetNewAggr(readCPU); err != nil {
				klog.V(5).Infoln(err)
			}
			if err := c.KubeletStats[memMetricName].SetNewAggr(readMem); err != nil {
				klog.V(5).Infoln(err)
			}
		}
	}
}
