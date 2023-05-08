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

func (c *Collector) createContainersMetricsIfNotExist(containerID string, cGroupID, pid uint64, withCGroupID bool) {
	if _, ok := c.ContainersMetrics[containerID]; !ok {
		var err error
		var podName, containerName string

		// the acceletator package does not get the cgroup ID, then we need to verify if we can call the cgroup with cGroupID or not
		podName, _ = cgroup.GetPodName(cGroupID, pid, withCGroupID)
		containerName, _ = cgroup.GetContainerName(cGroupID, pid, withCGroupID)

		namespace := c.systemProcessNamespace

		if containerName == c.systemProcessName {
			// if the systemProcess already exist in ContainersMetrics do not overwrite the data
			if _, exist := c.ContainersMetrics[containerID]; exist {
				return
			}
			containerID = c.systemProcessName
		} else {
			namespace, err = cgroup.GetPodNameSpace(cGroupID, pid, withCGroupID)
			if err != nil {
				klog.V(5).Infof("failed to find namespace for cGroup ID %v: %v", cGroupID, err)
				namespace = "unknown"
			}
		}

		c.ContainersMetrics[containerID] = collector_metric.NewContainerMetrics(containerName, podName, namespace, containerID)
	}
}

func (c *Collector) createProcessMetricsIfNotExist(pid uint64, command string) {
	if p, ok := c.ProcessMetrics[pid]; !ok {
		c.ProcessMetrics[pid] = collector_metric.NewProcessMetrics(pid, command)
	} else if p.Command == "" {
		p.Command = command
	}
}
