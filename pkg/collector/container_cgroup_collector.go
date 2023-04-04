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
	for key := range c.ContainersMetrics {
		if c.ContainersMetrics[key].PID == 0 {
			klog.V(3).Infof("PID 0 does not have cgroup metrics since there is no /proc/0/cgroup")
			continue
		}
		if c.ContainersMetrics[key].CgroupStatHandler == nil {
			handler, err := cgroup.NewCGroupStatHandler(int(c.ContainersMetrics[key].PID))
			if err != nil {
				klog.V(3).Infoln("Error: could not start cgroup stat handler for PID:", c.ContainersMetrics[key].PID)
				continue
			}
			c.ContainersMetrics[key].CgroupStatHandler = handler
		}

		// we need to check again if CgroupStatHandler is not nil because on darwin OS the handler will always be nil
		if c.ContainersMetrics[key].CgroupStatHandler != nil {
			cGroupMetrics, err := c.ContainersMetrics[key].CgroupStatHandler.GetCGroupStat()
			if err != nil {
				klog.V(3).Infoln("Error: could not red cgroup manager for PID:", c.ContainersMetrics[key].PID)
				continue
			}
			// cGroupMetrics contains all cgroups metrics listed in the config/types.go
			for metricName, value := range cGroupMetrics {
				if _, ok := c.ContainersMetrics[key].CgroupStatMap[metricName]; ok {
					c.ContainersMetrics[key].CgroupStatMap[metricName].SetAggrStat(key, value)
				} else {
					klog.Infoln(metricName, " does not exist for container", key)
				}
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
