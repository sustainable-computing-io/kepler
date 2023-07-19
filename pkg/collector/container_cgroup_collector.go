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
	"strings"

	"github.com/sustainable-computing-io/kepler/pkg/cgroup"
	collector_metric "github.com/sustainable-computing-io/kepler/pkg/collector/metric"

	"k8s.io/klog/v2"
)

// updateCgroupMetrics adds container-level cgroup data
func (c *Collector) updateCgroupMetrics() {
	for key := range c.ContainersMetrics {
		if c.ContainersMetrics[key].PID == 0 {
			klog.V(3).Infof("PID 0 does not have cgroup metrics since there is no /proc/0/cgroup")
			continue
		}
		if c.ContainersMetrics[key].CgroupStatHandler == nil {
			handler, err := cgroup.NewCGroupStatManager(int(c.ContainersMetrics[key].PID))
			if err != nil {
				klog.V(3).Infof("Error: could not start cgroup stat handler for PID %d: %v", c.ContainersMetrics[key].PID, err)
				if key != c.systemProcessName {
					// if cgroup manager does not exist, it means that the container was deleted
					delete(c.ContainersMetrics, key)
				}
				continue
			}
			c.ContainersMetrics[key].CgroupStatHandler = handler
		}
		if err := c.ContainersMetrics[key].UpdateCgroupMetrics(); err != nil {
			// if the cgroup metrics of a container does not exist, it means that the container was deleted
			if key != c.systemProcessName && strings.Contains(err.Error(), "cgroup deleted") {
				delete(c.ContainersMetrics, key)
				klog.V(1).Infof("Container/Pod %s/%s was removed from the map because the cgroup was deleted",
					c.ContainersMetrics[key].ContainerName, c.ContainersMetrics[key].PodName)
			}
		}
	}
}

// updateKubeletMetrics adds kubelet data (resident mem)
func (c *Collector) updateKubeletMetrics() {
	if len(collector_metric.AvailableKubeletMetrics) == 2 {
		containerCPU, containerMem, _ := cgroup.GetContainerMetrics()
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
