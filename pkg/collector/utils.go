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

	"github.com/sustainable-computing-io/kepler/pkg/kubernetes"
)

// this function is only called with the watcher delayed to sync and update the container info or if the watcher is not enabled
func (c *Collector) createContainersMetricsIfNotExist(containerID string, cGroupID, pid uint64, withCGroupID bool) {
	if _, ok := c.ContainersMetrics[containerID]; !ok {
		// We feel the info with generic values because the watcher will eventually update it.
		podName := c.systemProcessName
		containerName := c.systemProcessName
		namespace := c.systemProcessNamespace

		// In case the pod watcher is not enabled, we need to retrieve the information about the
		// pod and container from the kubelet API. However, we prefer to use the watcher approach
		// as accessing the kubelet API might be restricted in certain systems.
		// Additionally, the code that fetches the information from the kubelet API utilizes cache
		// for performance reasons. Therefore, if the kubelet API delay the information of the
		// containerID (which occasionally occurs), the container will be wrongly identified for its entire lifetime.
		if !kubernetes.IsWatcherEnabled {
			podName, _ = cgroup.GetPodName(cGroupID, pid, withCGroupID)
			containerName, _ = cgroup.GetContainerName(cGroupID, pid, withCGroupID)
			if containerName == c.systemProcessName {
				containerID = c.systemProcessName
			} else {
				var err error
				namespace, err = cgroup.GetPodNameSpace(cGroupID, pid, withCGroupID)
				if err != nil {
					klog.V(5).Infof("failed to find namespace for cGroup ID %v: %v", cGroupID, err)
					namespace = "unknown"
				}
			}
		}
		c.ContainersMetrics[containerID] = collector_metric.NewContainerMetrics(
			podName, containerName, namespace, containerID)
	}
}

func (c *Collector) createProcessMetricsIfNotExist(pid uint64, command string) {
	if p, ok := c.ProcessMetrics[pid]; !ok {
		c.ProcessMetrics[pid] = collector_metric.NewProcessMetrics(pid, command)
	} else if p.Command == "" {
		p.Command = command
	}
}
