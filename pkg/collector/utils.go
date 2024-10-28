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
	"github.com/sustainable-computing-io/kepler/pkg/collector/stats"
	"github.com/sustainable-computing-io/kepler/pkg/utils"

	"github.com/sustainable-computing-io/kepler/pkg/kubernetes"
)

// this function is only called with the watcher delayed to sync and update the container info or if the watcher is not enabled
func (c *Collector) createContainerStatsIfNotExist(containerID string, cGroupID, pid uint64, withCGroupID bool) {
	if _, ok := c.ContainerStats[containerID]; !ok {
		// In case the pod watcher is not enabled, we need to retrieve the information about the
		// pod and container from the kubelet API. However, we prefer to use the watcher approach
		// as accessing the kubelet API might be restricted in certain systems.
		// Additionally, the code that fetches the information from the kubelet API utilizes cache
		// for performance reasons. Therefore, if the kubelet API delay the information of the
		// containerID (which occasionally occurs), the container will be wrongly identified for its entire lifetime.
		if !kubernetes.IsWatcherEnabled {
			info, _ := cgroup.GetContainerInfo(cGroupID, pid, withCGroupID)
			c.ContainerStats[containerID] = stats.NewContainerStats(
				info.ContainerName, info.PodName, info.Namespace, containerID)
		} else {
			name := utils.SystemProcessName
			namespace := utils.SystemProcessNamespace
			if cGroupID == 1 {
				// some kernel processes have cgroup id equal 1 or 0
				name = utils.KernelProcessName
				namespace = utils.KernelProcessNamespace
			}
			// We feel the info with generic values because the watcher will eventually update it.
			c.ContainerStats[containerID] = stats.NewContainerStats(
				name, name, namespace, containerID)
		}
	} else {
		// TODO set only the most resource intensive PID for the container
		c.ContainerStats[containerID].SetLatestProcess(pid)
	}
}
